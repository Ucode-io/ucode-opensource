package v1

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// googleLeadStandardColumns are the built-in Google lead form column ids offered
// to the UI when building a field mapping. Custom form questions carry their own
// ids and are entered by the user directly.
var googleLeadStandardColumns = []models.GoogleLeadColumn{
	{ColumnID: "FULL_NAME", Label: "Full name"},
	{ColumnID: "FIRST_NAME", Label: "First name"},
	{ColumnID: "LAST_NAME", Label: "Last name"},
	{ColumnID: "EMAIL", Label: "Email"},
	{ColumnID: "PHONE_NUMBER", Label: "Phone number"},
	{ColumnID: "WORK_EMAIL", Label: "Work email"},
	{ColumnID: "WORK_PHONE", Label: "Work phone"},
	{ColumnID: "COMPANY_NAME", Label: "Company name"},
	{ColumnID: "JOB_TITLE", Label: "Job title"},
	{ColumnID: "STREET_ADDRESS", Label: "Street address"},
	{ColumnID: "CITY", Label: "City"},
	{ColumnID: "REGION", Label: "Region"},
	{ColumnID: "POSTAL_CODE", Label: "Postal code"},
	{ColumnID: "COUNTRY", Label: "Country"},
}

// GoogleLeadsColumns returns the standard Google column ids for the mapping UI.
func (h *HandlerV1) GoogleLeadsColumns(c *gin.Context) {
	h.HandleResponse(c, status_http.OK, gin.H{"columns": googleLeadStandardColumns})
}

// GoogleWebhookReceive ingests a lead pushed by Google. Google retries on any
// non-2xx, so every path returns 200; the lead itself is processed out of band.
func (h *HandlerV1) GoogleWebhookReceive(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	var event models.GoogleLeadWebhookEvent
	if err = json.Unmarshal(body, &event); err != nil {
		h.log.Error("google lead webhook: invalid payload", logger.Error(err))
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	// Ack first, ingest out of band; Google expects a fast response.
	c.JSON(http.StatusOK, gin.H{"ok": true})

	if strings.TrimSpace(event.GoogleKey) == "" {
		h.log.Warn("google lead webhook: missing google_key")
		return
	}

	go h.processGoogleLeadEvent(event)
}

// processGoogleLeadEvent runs in its own goroutine after the webhook is acked,
// owning a fresh context and recovering from panics so a malformed payload never
// takes the gateway down. A google_key may be wired to several project
// environments (fan-out), so every match receives the lead.
func (h *HandlerV1) processGoogleLeadEvent(event models.GoogleLeadWebhookEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("google lead: recovered from panic", logger.Any("panic", r))
		}
	}()

	ctx := context.Background()

	list, err := h.companyServices.Resource().GetProjectResourcesByExternalId(
		ctx, &pb.GetByExternalIdRequest{
			ExternalId: event.GoogleKey,
			Type:       pb.ResourceType_GOOGLE_LEADS,
		},
	)
	if err != nil {
		h.log.Error("google lead: resolve resources failed", logger.Error(err))
		return
	}

	resources := list.GetResources()
	if len(resources) == 0 {
		h.log.Warn("google lead: no project mapped for key",
			logger.String("form_id", event.FormID.String()),
		)
		return
	}

	values := googleLeadValues(event.UserColumnData)

	for _, resource := range resources {
		credentials := resource.GetSettings().GetGoogleLeads()
		if credentials == nil {
			h.log.Warn("google lead: resource has no google_leads settings, skipping",
				logger.String("resource_id", resource.GetId()),
				logger.String("project_id", resource.GetProjectId()),
			)
			continue
		}

		// google_key resolves the resource, but verify it again in constant time:
		// it is the shared secret that authenticates the request.
		if subtle.ConstantTimeCompare(
			[]byte(credentials.GetGoogleKey()), []byte(event.GoogleKey),
		) != 1 {
			h.log.Warn("google lead: key mismatch", logger.String("project_id", resource.GetProjectId()))
			continue
		}

		// An optional form_id pins the resource to one form; empty accepts any.
		if formID := credentials.GetFormId(); formID != "" && formID != event.FormID.String() {
			h.log.Info("google lead: form not mapped, skipping",
				logger.String("form_id", event.FormID.String()),
				logger.String("project_id", resource.GetProjectId()),
			)
			continue
		}

		if err := h.writeGoogleLead(ctx, resource, credentials, values, event); err != nil {
			h.log.Error("google lead: write failed", logger.Error(err),
				logger.String("project_id", resource.GetProjectId()),
				logger.String("table_slug", credentials.GetTableSlug()),
			)
		}
	}
}

func (h *HandlerV1) writeGoogleLead(ctx context.Context, resource *pb.ProjectResource, credentials *pb.GoogleLeadsCredentials, values map[string]string, event models.GoogleLeadWebhookEvent) error {
	data := map[string]any{}

	for _, field := range credentials.GetFields() {
		raw := values[field.GetLeadColumn()]
		if raw == "" {
			if field.GetRequired() {
				h.log.Warn("google lead: required column missing, skipping lead",
					logger.String("lead_column", field.GetLeadColumn()),
					logger.String("lead_id", event.LeadID),
				)
				return nil
			}
			continue
		}
		data[field.GetTableField()] = raw
	}

	if len(data) == 0 {
		h.log.Info("google lead: no mapped fields produced data, skipping",
			logger.String("lead_id", event.LeadID),
			logger.String("table_slug", credentials.GetTableSlug()),
		)
		return nil
	}

	// Dedup: Google may retry delivery and resend test leads with the same id, so
	// derive a stable guid from lead_id. Fall back to a random guid when lead_id is
	// absent, otherwise distinct leads would collapse into one row.
	if event.LeadID != "" {
		data["guid"] = uuid.NewSHA1(uuid.NameSpaceOID, []byte("google-lead:"+event.LeadID)).String()
	} else {
		data["guid"] = uuid.NewString()
	}

	resourceModel, err := h.companyServices.ServiceResource().GetSingle(ctx, &pb.GetSingleServiceResourceReq{
		ProjectId:     resource.GetProjectId(),
		EnvironmentId: resource.GetEnvironmentId(),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		return err
	}

	services, err := h.GetProjectSrvc(ctx, resource.GetProjectId(), resourceModel.NodeType)
	if err != nil {
		return err
	}

	structData, err := helper.ConvertMapToStruct(data)
	if err != nil {
		return err
	}

	// Postgres and Mongo projects are served by different builder services, so the
	// write must be routed by the project's resource type (Mongo uses the legacy
	// ObjectBuilder, Postgres the new Go object builder).
	switch resourceModel.GetResourceType() {
	case pb.ResourceType_POSTGRESQL:
		_, err = services.GoObjectBuilderService().Items().Create(ctx, &nb.CommonMessage{
			TableSlug: credentials.GetTableSlug(),
			Data:      structData,
			ProjectId: resourceModel.GetResourceEnvironmentId(),
		})
	default:
		_, err = services.GetBuilderServiceByType(resourceModel.GetNodeType()).ObjectBuilder().Create(ctx, &obs.CommonMessage{
			TableSlug: credentials.GetTableSlug(),
			Data:      structData,
			ProjectId: resourceModel.GetResourceEnvironmentId(),
		})
	}
	if err != nil {
		// A duplicate guid means this lead was already ingested (Google retries and
		// resends test leads). Dedup is expected, not a failure.
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			h.log.Info("google lead: duplicate, already ingested",
				logger.String("lead_id", event.LeadID),
				logger.String("table_slug", credentials.GetTableSlug()),
			)
			return nil
		}
		return err
	}

	h.log.Info("google lead: written",
		logger.String("project_id", resource.GetProjectId()),
		logger.String("table_slug", credentials.GetTableSlug()),
		logger.String("lead_id", event.LeadID),
	)
	return nil
}

// googleLeadValues flattens user_column_data into column_id -> value.
func googleLeadValues(columns []models.GoogleLeadColumnData) map[string]string {
	values := make(map[string]string, len(columns))
	for _, column := range columns {
		values[column.ColumnID] = column.StringValue
	}
	return values
}

// GoogleLeadsCreate provisions an integration: it generates a google_key, stores
// the field mapping, and returns the key and webhook URL for the user to paste
// into the Google Ads lead form settings.
func (h *HandlerV1) GoogleLeadsCreate(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	var req models.GoogleLeadsCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	googleKey, err := generateGoogleKey()
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	credentials := &pb.GoogleLeadsCredentials{
		GoogleKey:   googleKey,
		FormId:      strings.TrimSpace(req.FormId),
		FormName:    strings.TrimSpace(req.FormName),
		TableSlug:   strings.TrimSpace(req.TableSlug),
		Status:      config.GoogleLeadsStatusActive,
		ConnectedAt: time.Now().UTC().Format(time.RFC3339),
		Fields:      googleFieldsToProto(req.Fields),
	}

	name := credentials.GetFormName()
	if name == "" {
		name = "Google Lead Form"
	}

	resource, err := h.companyServices.Resource().UpsertProjectResource(
		c.Request.Context(), &pb.AddResourceToProjectRequest{
			Name:          name,
			ProjectId:     state.ProjectId,
			EnvironmentId: state.EnvironmentId,
			Type:          pb.ResourceType_GOOGLE_LEADS,
			ExternalId:    googleKey,
			Settings:      &pb.Settings{GoogleLeads: credentials},
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, gin.H{
		"resource_id": resource.GetId(),
		"google_key":  googleKey,
		"webhook_url": h.baseConf.GoogleLeadsWebhookURL,
	})
}

// GoogleLeadsSaveMapping updates the table/field mapping of an existing
// integration, preserving its google_key.
func (h *HandlerV1) GoogleLeadsSaveMapping(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	resourceID := strings.TrimSpace(c.Param("id"))
	if resourceID == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "resource id is required")
		return
	}

	var req models.GoogleLeadsMappingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resource, err := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), &pb.PrimaryKeyProjectResource{
		Id:            resourceID,
		ProjectId:     state.ProjectId,
		EnvironmentId: state.EnvironmentId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	credentials := resource.GetSettings().GetGoogleLeads()
	if credentials == nil {
		h.HandleResponse(c, status_http.BadRequest, "google leads integration is not configured for this resource")
		return
	}

	credentials.FormId = strings.TrimSpace(req.FormId)
	credentials.FormName = strings.TrimSpace(req.FormName)
	credentials.TableSlug = strings.TrimSpace(req.TableSlug)
	credentials.Fields = googleFieldsToProto(req.Fields)

	name := resource.GetName()
	if name == "" {
		name = credentials.GetFormName()
	}

	if _, err := h.companyServices.Resource().UpdateProjectResource(c.Request.Context(), &pb.ProjectResource{
		Id:            resource.GetId(),
		ProjectId:     state.ProjectId,
		EnvironmentId: state.EnvironmentId,
		Name:          name,
		Type:          pb.ResourceType_GOOGLE_LEADS.String(),
		ResourceType:  int32(pb.ResourceType_GOOGLE_LEADS),
		ExternalId:    credentials.GetGoogleKey(),
		Settings:      &pb.Settings{GoogleLeads: credentials},
	}); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, gin.H{"resource_id": resource.GetId()})
}

func (h *HandlerV1) GoogleLeadsIntegration(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	list, err := h.companyServices.Resource().GetProjectResourceList(
		c.Request.Context(),
		&pb.GetProjectResourceListRequest{
			ProjectId:     state.ProjectId,
			EnvironmentId: state.EnvironmentId,
			Type:          pb.ResourceType_GOOGLE_LEADS,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	statuses := make([]models.GoogleLeadsIntegrationStatus, 0, len(list.GetResources()))

	for _, resource := range list.GetResources() {
		status := models.GoogleLeadsIntegrationStatus{
			Connected:  true,
			ResourceId: resource.GetId(),
			WebhookURL: h.baseConf.GoogleLeadsWebhookURL,
		}

		if credentials := resource.GetSettings().GetGoogleLeads(); credentials != nil {
			status.GoogleKey = credentials.GetGoogleKey()
			status.FormId = credentials.GetFormId()
			status.FormName = credentials.GetFormName()
			status.TableSlug = credentials.GetTableSlug()
			status.Status = credentials.GetStatus()
			status.ConnectedAt = credentials.GetConnectedAt()
			status.Fields = googleFieldsFromProto(credentials.GetFields())
		}

		statuses = append(statuses, status)
	}

	h.HandleResponse(c, status_http.OK, gin.H{"connected": len(statuses) > 0, "integrations": statuses})
}

func (h *HandlerV1) GoogleLeadsDisconnect(c *gin.Context) {
	state, ok := h.authContext(c)
	if !ok {
		return
	}

	resourceID := strings.TrimSpace(c.Param("id"))
	if resourceID == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "resource id is required")
		return
	}

	if _, err := h.companyServices.Resource().DeleteProjectResource(c.Request.Context(), &pb.PrimaryKeyProjectResource{
		Id:            resourceID,
		ProjectId:     state.ProjectId,
		EnvironmentId: state.EnvironmentId,
	}); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, gin.H{"resource_id": resourceID})
}

// generateGoogleKey returns a 24-byte random hex secret (48 chars) used both to
// authenticate the webhook and to route a lead to its project_resource. Google's
// lead form Key field caps at 50 characters, so the length must stay under it.
func generateGoogleKey() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func googleFieldsToProto(fields []models.GoogleLeadFieldMapping) []*pb.GoogleLeadFieldMapping {
	result := make([]*pb.GoogleLeadFieldMapping, 0, len(fields))
	for _, field := range fields {
		result = append(result, &pb.GoogleLeadFieldMapping{
			LeadColumn: field.LeadColumn,
			TableField: field.TableField,
			Required:   field.Required,
		})
	}
	return result
}

func googleFieldsFromProto(fields []*pb.GoogleLeadFieldMapping) []models.GoogleLeadFieldMapping {
	result := make([]models.GoogleLeadFieldMapping, 0, len(fields))
	for _, field := range fields {
		result = append(result, models.GoogleLeadFieldMapping{
			LeadColumn: field.GetLeadColumn(),
			TableField: field.GetTableField(),
			Required:   field.GetRequired(),
		})
	}
	return result
}

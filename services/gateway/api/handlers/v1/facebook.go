package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
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

func (h *HandlerV1) authContext(c *gin.Context) (models.FacebookOAuthState, bool) {
	projectID, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return models.FacebookOAuthState{}, false
	}

	environmentID, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return models.FacebookOAuthState{}, false
	}

	userID := ""
	if value, ok := c.Get("user_id"); ok {
		userID, _ = value.(string)
	}

	return models.FacebookOAuthState{
		ProjectId:     projectID.(string),
		EnvironmentId: environmentID.(string),
		UserId:        userID,
	}, true
}

func (h *HandlerV1) FacebookWebhookVerify(c *gin.Context) {
	var (
		mode      = c.Query("hub.mode")
		token     = c.Query("hub.verify_token")
		challenge = c.Query("hub.challenge")
	)

	if mode == "subscribe" && token == h.baseConf.FacebookWebhookVerifyToken && token != "" {
		c.String(http.StatusOK, challenge)
		return
	}

	c.String(http.StatusForbidden, "verification failed")
}

func (h *HandlerV1) FacebookWebhookReceive(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	if !h.verifyFacebookSignature(c.GetHeader(config.FacebookSignatureHeader), body) {
		h.log.Warn("facebook webhook: signature verification failed")
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	var event models.FacebookLeadWebhookEvent

	if err = json.Unmarshal(body, &event); err != nil {
		h.log.Error("facebook webhook: invalid payload", logger.Error(err))
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}

	// Meta requires a fast ack (~20s); acknowledge first, ingest out of band.
	c.JSON(http.StatusOK, gin.H{"ok": true})

	go h.processFacebookLeadEvent(event)
}

// processFacebookLeadEvent runs in its own goroutine after the webhook is acked,
// so it owns a fresh context and recovers from panics to never take the gateway
// down on a malformed payload. A Page may be wired to several project
// environments (fan-out), so every match receives the lead.
func (h *HandlerV1) processFacebookLeadEvent(event models.FacebookLeadWebhookEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("facebook lead: recovered from panic", logger.Any("panic", r))
		}
	}()

	ctx := context.Background()
	for _, entry := range event.Entry {
		for _, change := range entry.Changes {
			if change.Field != config.FacebookWebhookFieldLead {
				continue
			}
			h.ingestFacebookLead(ctx, entry.ID, change.Value)
		}
	}
}

func (h *HandlerV1) ingestFacebookLead(ctx context.Context, pageID string, value models.FacebookLeadChangeValue) {
	if value.PageID != "" {
		pageID = value.PageID
	}

	list, err := h.companyServices.Resource().GetProjectResourcesByExternalId(
		ctx, &pb.GetByExternalIdRequest{
			ExternalId: pageID,
			Type:       pb.ResourceType_META_LEADS,
		},
	)
	if err != nil {
		h.log.Error("facebook lead: resolve resources failed", logger.Error(err))
		return
	}

	var (
		resources = list.GetResources()
		lead      *models.FacebookLead
	)

	if len(resources) == 0 {
		h.log.Warn("facebook lead: no project mapped for page",
			logger.String("page_id", pageID),
			logger.String("form_id", value.FormID),
		)
		return
	}

	for _, resource := range resources {
		credentials := resource.GetSettings().GetFacebookLeads()
		if credentials == nil {
			continue
		}

		// All forms of a connected page are captured automatically — no per-form
		// mapping gate. The lead is fetched once and written to every project
		// mapped to this page (fan-out).
		if lead == nil {
			fetched, err := h.facebookFetchLead(ctx, value.LeadgenID, credentials.GetPageAccessToken())
			if err != nil {
				h.log.Error("facebook lead: fetch failed", logger.Error(err),
					logger.String("leadgen_id", value.LeadgenID))
				return
			}
			lead = &fetched
		}

		if err := h.writeProfessionalCRMLead(ctx, resource, *lead, value); err != nil {
			h.log.Error("facebook lead: write failed", logger.Error(err),
				logger.String("project_id", resource.GetProjectId()),
				logger.String("form_id", value.FormID),
			)
		}
	}
}

func (h *HandlerV1) writeFacebookLead(ctx context.Context, resource *pb.ProjectResource, mapping *pb.FacebookLeadFormMapping, lead models.FacebookLead, value models.FacebookLeadChangeValue) error {
	var (
		values = facebookLeadValues(lead.FieldData)
		data   = map[string]any{}
	)

	for _, field := range mapping.GetFields() {
		raw, present := values[strings.ToLower(field.GetLeadField())]
		if !present || raw == "" {
			if field.GetRequired() {
				return fmt.Errorf("required lead field %q is missing", field.GetLeadField())
			}
			continue
		}
		data[field.GetTableField()] = raw
	}

	if len(data) == 0 {
		h.log.Info("facebook lead: no mapped fields produced data, skipping",
			logger.String("leadgen_id", value.LeadgenID),
			logger.String("table_slug", mapping.GetTableSlug()),
		)
		return nil
	}

	data["guid"] = uuid.NewSHA1(uuid.NameSpaceOID, []byte("facebook-lead:"+value.LeadgenID)).String()

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
			TableSlug: mapping.GetTableSlug(),
			Data:      structData,
			ProjectId: resourceModel.GetResourceEnvironmentId(),
		})
	default:
		_, err = services.GetBuilderServiceByType(resourceModel.GetNodeType()).ObjectBuilder().Create(ctx, &obs.CommonMessage{
			TableSlug: mapping.GetTableSlug(),
			Data:      structData,
			ProjectId: resourceModel.GetResourceEnvironmentId(),
		})
	}
	if err != nil {
		// A duplicate guid means this lead was already ingested (Meta retries the
		// webhook). Dedup is expected, not a failure.
		if st, ok := status.FromError(err); ok && st.Code() == codes.AlreadyExists {
			h.log.Info("facebook lead: duplicate, already ingested",
				logger.String("leadgen_id", value.LeadgenID),
				logger.String("table_slug", mapping.GetTableSlug()),
			)
			return nil
		}
		return err
	}

	h.log.Info("facebook lead: written",
		logger.String("project_id", resource.GetProjectId()),
		logger.String("table_slug", mapping.GetTableSlug()),
		logger.String("leadgen_id", value.LeadgenID),
	)
	return nil
}

func facebookLeadValues(fieldData []models.FacebookFieldData) map[string]string {
	values := make(map[string]string, len(fieldData))

	for _, field := range fieldData {
		if len(field.Values) > 0 {
			values[strings.ToLower(field.Name)] = field.Values[0]
		}
	}
	return values
}

func (h *HandlerV1) verifyFacebookSignature(header string, body []byte) bool {
	secrets := append([]string{h.baseConf.FacebookAppSecret}, h.baseConf.FacebookLegacyAppSecrets...)
	return verifyMetaWebhookSignature(header, config.FacebookSignaturePrefix, body, secrets...)
}

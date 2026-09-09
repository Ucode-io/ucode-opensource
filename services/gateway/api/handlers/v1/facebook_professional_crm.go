package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

const crmSourcePrefix = "Facebook: "

// crmMapping describes, per project, where a Facebook lead is written: which
// tables hold contacts/deals/forms, which relation fields link them, and which
// pipeline/stage a new lead lands in. It is resolved per resource (see
// resolveCRMMapping) so the integration is NOT bound to one project's schema —
// the defaults below only seed a project that has not configured its own.
type crmMapping struct {
	ContactsTable  string
	DealsTable     string
	LeadFormsTable string

	DealContactField string // relation field on deals → contacts
	DealFormField    string // relation field on deals → lead_forms

	PipelineField string
	PipelineValue string
	StageField    string
	StageValue    string
	SourceField   string

	// PipelineStageField is a per-pipeline scalar stage column (ProfessionalCrm
	// stores the Sales Project pipeline's stage in `pipeline_sales_project`); it
	// is set to StageValue. StartDateField gets the lead's arrival time. Both are
	// optional — left blank they are simply not written.
	PipelineStageField string
	StartDateField     string

	// FormAcceptField is the boolean column on the lead_forms table that gates
	// ingestion under the allowlist policy: a lead is written only when its form's
	// row has this column set truthy. New/un-enabled forms are ignored (but their
	// row is still created, disabled, so the user can enable it in the CRM).
	FormAcceptField string
}

// defaultCRMMapping seeds the amoCRM-style flow for a project that has not
// stored its own mapping yet. These match the ProfessionalCrm template's
// schema but are only a fallback, never the sole source of truth.
func defaultCRMMapping() crmMapping {
	return crmMapping{
		ContactsTable:      "contacts",
		DealsTable:         "deals",
		LeadFormsTable:     "lead_forms",
		DealContactField:   "contacts_id",
		DealFormField:      "lead_forms_id",
		PipelineField:      "pipeline",
		PipelineValue:      "Sales Project",
		StageField:         "stage",
		StageValue:         "Новая заявка",
		SourceField:        "source",
		PipelineStageField: "pipeline_sales_project",
		StartDateField:     "start_date",
		FormAcceptField:    "accept_leads",
	}
}

// resolveCRMMapping returns the mapping for a resource, overlaying any
// per-project configuration stored on it over the defaults. This is the single
// seam through which each connected project chooses its own tables/pipeline/
// stage — nothing downstream references a hardcoded slug.
func (h *HandlerV1) resolveCRMMapping(resource *pb.ProjectResource) crmMapping {
	return overlayCRMMapping(defaultCRMMapping(), resource.GetSettings().GetFacebookLeads().GetCrmMapping())
}

// overlayCRMMapping applies the project's stored overrides over a base mapping.
// Only non-blank stored fields win, so a partially-filled mapping still works.
func overlayCRMMapping(m crmMapping, stored *pb.FacebookCrmMapping) crmMapping {
	if stored == nil {
		return m
	}
	setIfNotBlank(&m.ContactsTable, stored.GetContactsTable())
	setIfNotBlank(&m.DealsTable, stored.GetDealsTable())
	setIfNotBlank(&m.LeadFormsTable, stored.GetLeadFormsTable())
	setIfNotBlank(&m.DealContactField, stored.GetDealContactField())
	setIfNotBlank(&m.DealFormField, stored.GetDealFormField())
	setIfNotBlank(&m.PipelineField, stored.GetPipelineField())
	setIfNotBlank(&m.PipelineValue, stored.GetPipelineValue())
	setIfNotBlank(&m.StageField, stored.GetStageField())
	setIfNotBlank(&m.StageValue, stored.GetStageValue())
	setIfNotBlank(&m.SourceField, stored.GetSourceField())
	return m
}

func setIfNotBlank(dst *string, v string) {
	if v = strings.TrimSpace(v); v != "" {
		*dst = v
	}
}

// crmMappingToResponse renders a resolved mapping as the API shape.
func crmMappingToResponse(m crmMapping) models.FacebookCrmMapping {
	return models.FacebookCrmMapping{
		ContactsTable:    m.ContactsTable,
		DealsTable:       m.DealsTable,
		LeadFormsTable:   m.LeadFormsTable,
		DealContactField: m.DealContactField,
		DealFormField:    m.DealFormField,
		PipelineField:    m.PipelineField,
		PipelineValue:    m.PipelineValue,
		StageField:       m.StageField,
		StageValue:       m.StageValue,
		SourceField:      m.SourceField,
	}
}

// crmMappingRequestToProto stores the raw values the project supplied (trimmed);
// blanks are kept blank so resolveCRMMapping falls back to defaults per field.
func crmMappingRequestToProto(req models.FacebookCrmMapping) *pb.FacebookCrmMapping {
	return &pb.FacebookCrmMapping{
		ContactsTable:    strings.TrimSpace(req.ContactsTable),
		DealsTable:       strings.TrimSpace(req.DealsTable),
		LeadFormsTable:   strings.TrimSpace(req.LeadFormsTable),
		DealContactField: strings.TrimSpace(req.DealContactField),
		DealFormField:    strings.TrimSpace(req.DealFormField),
		PipelineField:    strings.TrimSpace(req.PipelineField),
		PipelineValue:    strings.TrimSpace(req.PipelineValue),
		StageField:       strings.TrimSpace(req.StageField),
		StageValue:       strings.TrimSpace(req.StageValue),
		SourceField:      strings.TrimSpace(req.SourceField),
	}
}

// crmLeadFields is the normalized person/contact data extracted from a Facebook
// lead's field_data, independent of how the form named its questions.
type crmLeadFields struct {
	firstName   string
	lastName    string
	email       string
	phone       string
	createdTime string // lead's created_time from Graph (ISO8601)
}

func (f crmLeadFields) hasContactIdentity() bool {
	return f.firstName != "" || f.lastName != "" || f.email != "" || f.phone != ""
}

func (f crmLeadFields) fullName() string {
	return strings.TrimSpace(strings.TrimSpace(f.firstName) + " " + strings.TrimSpace(f.lastName))
}

// writeProfessionalCRMLead performs the two-table amoCRM write for one project
// resource: find-or-create the contact, find-or-create the source form, then
// create the deal linking both. Only Postgres-backed projects are supported
// (ProfessionalCrm is Postgres); other backends are skipped rather than written
// with a wrong shape.
func (h *HandlerV1) writeProfessionalCRMLead(ctx context.Context, resource *pb.ProjectResource, lead models.FacebookLead, value models.FacebookLeadChangeValue) error {
	svc, resourceEnvID, err := h.resolveProjectBuilder(ctx, resource.GetProjectId(), resource.GetEnvironmentId())
	if err != nil {
		return err
	}

	mapping := h.resolveCRMMapping(resource)
	fields := professionalCRMLeadFields(lead.FieldData)
	fields.createdTime = lead.CreatedTime

	// Resolve the source form FIRST so the allowlist gate runs before any
	// contact/deal is written. A form that is not enabled (accept_leads=false, or
	// no row yet) is skipped entirely — its row is still created disabled so the
	// user can find and enable it in the CRM. If the lookup itself fails we
	// fail-open (accept) rather than silently drop a lead on a transient error.
	formGUID, formName, accepted, err := h.crmFindOrCreateLeadForm(ctx, svc, resourceEnvID, mapping, resource, value.FormID)
	if err != nil {
		h.log.Warn("facebook lead: resolve source form failed, accepting anyway: " + err.Error())
	} else if !accepted {
		h.log.Info("facebook lead: form not enabled, skipping",
			logger.String("project_id", resource.GetProjectId()),
			logger.String("form_id", value.FormID),
		)
		return nil
	}

	contactGUID, err := h.crmFindOrCreateContact(ctx, svc, resourceEnvID, mapping, fields)
	if err != nil {
		return fmt.Errorf("find-or-create contact: %w", err)
	}

	return h.crmCreateDeal(ctx, svc, resourceEnvID, mapping, contactGUID, formGUID, formName, fields, value)
}

// resolveProjectBuilder returns the builder service handle and resource-env id
// for a project's Postgres builder DB — the shared entry point for reading and
// writing a project's contacts/deals/lead_forms.
func (h *HandlerV1) resolveProjectBuilder(ctx context.Context, projectID, environmentID string) (services.ServiceManagerI, string, error) {
	resourceModel, err := h.companyServices.ServiceResource().GetSingle(ctx, &pb.GetSingleServiceResourceReq{
		ProjectId:     projectID,
		EnvironmentId: environmentID,
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		return nil, "", err
	}
	if resourceModel.GetResourceType() != pb.ResourceType_POSTGRESQL {
		return nil, "", fmt.Errorf("facebook leads support postgres projects only, got %s", resourceModel.GetResourceType())
	}
	svc, err := h.GetProjectSrvc(ctx, projectID, resourceModel.NodeType)
	if err != nil {
		return nil, "", err
	}
	return svc, resourceModel.GetResourceEnvironmentId(), nil
}

// facebookSyncPageForms best-effort pre-populates a project's lead_forms table
// with every current form of a page, so the source list is visible before any
// lead arrives. It is intentionally non-fatal: if the lead_forms table does not
// exist yet, ingestion still creates rows lazily, so a failure here is only a
// missing pre-fill, never a blocked connect.
func (h *HandlerV1) facebookSyncPageForms(ctx context.Context, state models.FacebookOAuthState, page models.FacebookPage) {
	resource, err := h.findFacebookResource(ctx, state.ProjectId, state.EnvironmentId, page.ID)
	if err != nil || resource == nil {
		return
	}

	svc, resourceEnvID, err := h.resolveProjectBuilder(ctx, state.ProjectId, state.EnvironmentId)
	if err != nil {
		h.log.Warn("facebook sync forms: resolve builder failed: " + err.Error())
		return
	}
	mapping := h.resolveCRMMapping(resource)

	forms, err := h.facebookListForms(ctx, page.ID, page.AccessToken)
	if err != nil {
		h.log.Warn("facebook sync forms: list forms failed: " + err.Error())
		return
	}

	for _, form := range forms {
		if row, err := h.crmLookupRow(ctx, svc, resourceEnvID, mapping.LeadFormsTable, "form_id", form.ID); err != nil {
			// The table is likely absent; stop rather than retry per form.
			h.log.Warn("facebook sync forms: lookup failed (lead_forms table missing?): " + err.Error())
			return
		} else if row != nil {
			continue
		}

		payload := map[string]any{
			"guid":      uuid.NewString(),
			"form_id":   form.ID,
			"name":      form.Name,
			"status":    form.Status,
			"page_id":   page.ID,
			"page_name": page.Name,
		}
		// Allowlist default: pre-synced forms start disabled; the user enables the
		// ones they want leads from (toggling accept_leads in the CRM).
		if mapping.FormAcceptField != "" {
			payload[mapping.FormAcceptField] = false
		}
		if err := h.crmCreateItem(ctx, svc, resourceEnvID, mapping.LeadFormsTable, payload); err != nil {
			h.log.Warn("facebook sync forms: create lead_forms row failed (lead_forms table missing?): " + err.Error())
			return
		}
	}
}

// crmFindOrCreateContact deduplicates by phone OR email (контроль дублей). On a
// match it backfills any missing name/phone/email and reuses the row; otherwise
// it creates a new contact with a caller-generated guid so the deal can link to
// it without a second round-trip.
func (h *HandlerV1) crmFindOrCreateContact(ctx context.Context, svc services.ServiceManagerI, resourceEnvID string, m crmMapping, f crmLeadFields) (string, error) {
	if !f.hasContactIdentity() {
		return "", nil
	}

	if f.phone != "" {
		if row, err := h.crmLookupRow(ctx, svc, resourceEnvID, m.ContactsTable, "phone", f.phone); err != nil {
			return "", err
		} else if row != nil {
			return h.crmBackfillContact(ctx, svc, resourceEnvID, m, row, f), nil
		}
	}
	if f.email != "" {
		if row, err := h.crmLookupRow(ctx, svc, resourceEnvID, m.ContactsTable, "email", f.email); err != nil {
			return "", err
		} else if row != nil {
			return h.crmBackfillContact(ctx, svc, resourceEnvID, m, row, f), nil
		}
	}

	guid := uuid.NewString()
	payload := map[string]any{"guid": guid}
	if f.firstName != "" {
		payload["first_name"] = f.firstName
	}
	if f.lastName != "" {
		payload["last_name"] = f.lastName
	}
	if f.email != "" {
		payload["email"] = f.email
		payload["emails"] = []string{f.email}
	}
	if f.phone != "" {
		payload["phone"] = f.phone
		payload["phones"] = []string{f.phone}
	}

	if err := h.crmCreateItem(ctx, svc, resourceEnvID, m.ContactsTable, payload); err != nil {
		return "", err
	}
	return guid, nil
}

// crmBackfillContact fills fields that are empty on an existing contact from the
// incoming lead (never overwrites existing values), then returns its guid.
func (h *HandlerV1) crmBackfillContact(ctx context.Context, svc services.ServiceManagerI, resourceEnvID string, m crmMapping, row map[string]any, f crmLeadFields) string {
	guid, _ := row["guid"].(string)
	if guid == "" {
		return ""
	}

	update := map[string]any{"guid": guid}
	if isBlank(row["first_name"]) && f.firstName != "" {
		update["first_name"] = f.firstName
	}
	if isBlank(row["last_name"]) && f.lastName != "" {
		update["last_name"] = f.lastName
	}
	if isBlank(row["email"]) && f.email != "" {
		update["email"] = f.email
	}
	if isBlank(row["phone"]) && f.phone != "" {
		update["phone"] = f.phone
	}

	if len(update) > 1 {
		if _, err := svc.GoObjectBuilderService().Items().Update(ctx, &nb.CommonMessage{
			TableSlug: m.ContactsTable,
			Data:      mustStruct(update),
			ProjectId: resourceEnvID,
		}); err != nil {
			// Backfill is a nicety; a failed update must not drop the lead.
			h.log.Warn("facebook lead: contact backfill failed: " + err.Error())
		}
	}
	return guid
}

// crmFindOrCreateLeadForm resolves the lead_forms row for a Facebook form,
// creating it (with the form name fetched from Graph) when it does not yet
// exist so future forms are visible in the CRM. Returns (guid, name, accepted):
// accepted reflects the allowlist gate — an existing row's accept_leads column,
// and false for a freshly-created row (new forms start disabled and must be
// enabled by the user before their leads are captured).
func (h *HandlerV1) crmFindOrCreateLeadForm(ctx context.Context, svc services.ServiceManagerI, resourceEnvID string, m crmMapping, resource *pb.ProjectResource, formID string) (string, string, bool, error) {
	formID = strings.TrimSpace(formID)
	if formID == "" {
		return "", "", false, nil
	}

	if row, err := h.crmLookupRow(ctx, svc, resourceEnvID, m.LeadFormsTable, "form_id", formID); err != nil {
		return "", "", false, err
	} else if row != nil {
		guid, _ := row["guid"].(string)
		name, _ := row["name"].(string)
		return guid, name, crmFormAccepted(row, m.FormAcceptField), nil
	}

	credentials := resource.GetSettings().GetFacebookLeads()
	name := ""
	if credentials != nil {
		if form, err := h.facebookFormQuestions(ctx, formID, credentials.GetPageAccessToken()); err == nil {
			name = form.Name
		}
	}

	guid := uuid.NewString()
	payload := map[string]any{
		"guid":    guid,
		"form_id": formID,
		"status":  "active",
	}
	// New forms are disabled by default under the allowlist policy — the row is
	// created only so the user can see the form and switch it on.
	if m.FormAcceptField != "" {
		payload[m.FormAcceptField] = false
	}
	if name != "" {
		payload["name"] = name
	}
	if credentials != nil {
		if credentials.GetPageId() != "" {
			payload["page_id"] = credentials.GetPageId()
		}
		if credentials.GetPageName() != "" {
			payload["page_name"] = credentials.GetPageName()
		}
	}

	if err := h.crmCreateItem(ctx, svc, resourceEnvID, m.LeadFormsTable, payload); err != nil {
		return "", name, false, err
	}
	return guid, name, false, nil
}

// crmFormAccepted reports whether a lead_forms row is enabled to capture leads.
// The accept flag may come back from object-builder as a real bool or as a
// string ("true"/"1"/"yes"/"on"), so both are handled. A blank/absent column
// means not enabled (allowlist default).
func crmFormAccepted(row map[string]any, field string) bool {
	if field == "" {
		return true
	}
	switch v := row[field].(type) {
	case bool:
		return v
	case string:
		switch strings.ToLower(strings.TrimSpace(v)) {
		case "true", "t", "1", "yes", "on":
			return true
		}
	}
	return false
}

// crmCreateDeal writes the deal linking the contact and the source form, placed
// in the agreed entry pipeline/stage, deduped by leadgen id so Meta webhook
// retries do not create duplicate deals.
func (h *HandlerV1) crmCreateDeal(ctx context.Context, svc services.ServiceManagerI, resourceEnvID string, m crmMapping, contactGUID, formGUID, formName string, f crmLeadFields, value models.FacebookLeadChangeValue) error {
	name := f.fullName()
	if name == "" {
		name = formName
	}
	if name == "" {
		name = "Facebook lead"
	}

	source := crmSourcePrefix + formName
	if formName == "" {
		source = strings.TrimSuffix(crmSourcePrefix, ": ")
	}

	payload := map[string]any{
		// Deterministic guid from the leadgen id makes retries idempotent.
		"guid":          uuid.NewSHA1(uuid.NameSpaceOID, []byte("facebook-lead:"+value.LeadgenID)).String(),
		"name":          name,
		m.PipelineField: []string{m.PipelineValue},
		m.StageField:    []string{m.StageValue},
		m.SourceField:   []string{source},
	}
	// Per-pipeline scalar stage column (e.g. pipeline_sales_project) holds the
	// same entry-stage value so the deal lands in the right board column.
	if m.PipelineStageField != "" {
		payload[m.PipelineStageField] = m.StageValue
	}
	// start_date = when the lead came in (falls back to now if unparseable).
	if m.StartDateField != "" {
		ts := parseFacebookTime(f.createdTime)
		if ts == 0 {
			ts = time.Now().Unix()
		}
		payload[m.StartDateField] = time.Unix(ts, 0).UTC().Format(time.RFC3339)
	}
	if contactGUID != "" && m.DealContactField != "" {
		payload[m.DealContactField] = contactGUID
	}
	if formGUID != "" && m.DealFormField != "" {
		payload[m.DealFormField] = formGUID
	}
	if f.email != "" {
		payload["email"] = f.email
	}

	if err := h.crmCreateItem(ctx, svc, resourceEnvID, m.DealsTable, payload); err != nil {
		if isAlreadyExists(err) {
			h.log.Info("facebook lead: deal already exists, skipping duplicate",
				logger.String("leadgen_id", value.LeadgenID))
			return nil
		}
		return err
	}

	h.log.Info("facebook lead: deal written",
		logger.String("leadgen_id", value.LeadgenID),
		logger.String("form_id", value.FormID),
	)
	return nil
}

// crmLookupRow returns the first row of table where field == value, or nil.
func (h *HandlerV1) crmLookupRow(ctx context.Context, svc services.ServiceManagerI, resourceEnvID, table, field, value string) (map[string]any, error) {
	resp, err := svc.GoObjectBuilderService().ObjectBuilder().GetList2(ctx, &nb.CommonMessage{
		TableSlug: table,
		Data:      mustStruct(map[string]any{field: value, "limit": 1, "offset": 0}),
		ProjectId: resourceEnvID,
	})
	if err != nil {
		return nil, err
	}
	return crmFirstRow(resp.GetData()), nil
}

func (h *HandlerV1) crmCreateItem(ctx context.Context, svc services.ServiceManagerI, resourceEnvID, table string, payload map[string]any) error {
	data, err := helper.ConvertMapToStruct(payload)
	if err != nil {
		return err
	}
	_, err = svc.GoObjectBuilderService().Items().Create(ctx, &nb.CommonMessage{
		TableSlug: table,
		Data:      data,
		ProjectId: resourceEnvID,
	})
	return err
}

// professionalCRMLeadFields maps a Facebook lead's field_data to normalized
// contact fields, tolerating the common field keys (full_name vs first/last,
// phone vs phone_number) so it works across forms without per-form config.
func professionalCRMLeadFields(fieldData []models.FacebookFieldData) crmLeadFields {
	values := facebookLeadValues(fieldData)

	f := crmLeadFields{
		firstName: strings.TrimSpace(values["first_name"]),
		lastName:  strings.TrimSpace(values["last_name"]),
		email:     strings.ToLower(strings.TrimSpace(values["email"])),
		phone:     strings.TrimSpace(fbFirstNonEmpty(values["phone_number"], values["phone"])),
	}

	if f.firstName == "" && f.lastName == "" {
		if full := strings.TrimSpace(values["full_name"]); full != "" {
			parts := strings.SplitN(full, " ", 2)
			f.firstName = parts[0]
			if len(parts) > 1 {
				f.lastName = strings.TrimSpace(parts[1])
			}
		}
	}
	return f
}

func crmFirstRow(data *structpb.Struct) map[string]any {
	if data == nil {
		return nil
	}
	rows, ok := data.AsMap()["response"].([]any)
	if !ok || len(rows) == 0 {
		return nil
	}
	row, _ := rows[0].(map[string]any)
	return row
}

func mustStruct(m map[string]any) *structpb.Struct {
	s, _ := helper.ConvertMapToStruct(m)
	return s
}

func fbFirstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func isBlank(v any) bool {
	s, ok := v.(string)
	return !ok || strings.TrimSpace(s) == ""
}

// isAlreadyExists reports whether err is a gRPC AlreadyExists — a duplicate guid
// from a Meta webhook retry, which is expected and not a failure.
func isAlreadyExists(err error) bool {
	st, ok := status.FromError(err)
	return ok && st.Code() == codes.AlreadyExists
}

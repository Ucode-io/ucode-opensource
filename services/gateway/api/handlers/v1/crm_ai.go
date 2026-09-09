package v1

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/openai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/gin-gonic/gin"
	go_redis "github.com/go-redis/redis/v8"
	"github.com/google/uuid"
)

const (
	crmAssistantTimeout       = 150 * time.Second
	crmAssistantSetupTimeout  = 10 * time.Second
	crmPendingActionTTL       = 10 * time.Minute
	crmPreferencesTTL         = 365 * 24 * time.Hour
	crmMaxImages              = 3
	crmMaxImageDecodedBytes   = 6 << 20
	crmMaxPreferenceFieldKeys = 200
)

type storedCRMAction struct {
	UserID        string               `json:"user_id"`
	ProjectID     string               `json:"project_id"`
	EnvironmentID string               `json:"environment_id"`
	Action        models.PendingAction `json:"action"`
}

func (h *HandlerV1) CreateCRMAssistantMessage(c *gin.Context) {
	var req models.CRMAssistantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid request body")
		return
	}
	if strings.TrimSpace(req.Message) == "" && len(req.Images) == 0 {
		h.HandleResponse(c, status_http.BadRequest, "message or image is required")
		return
	}
	if err := validateCRMImages(req.Images); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), crmAssistantTimeout)
	defer cancel()

	serviceCtx, serviceCancel := context.WithTimeout(ctx, crmAssistantSetupTimeout)
	service, resourceEnvID, err := h.getAiChatServicesWithContext(c, serviceCtx)
	serviceCancel()
	if err != nil {
		return
	}
	userID, err := h.getAiChatUserID(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "user identity is required")
		return
	}

	projectID := c.GetString("project_id")
	processor := newChatProcessor(
		h,
		service,
		h.baseConf,
		"crm-ai-"+uuid.NewString(),
		projectID,
		resourceEnvID,
		projectID,
		userID,
		"",
		"",
		c.GetHeader("Authorization"),
	)
	billingCtx, billingCancel := context.WithTimeout(ctx, crmAssistantSetupTimeout)
	if project, projectErr := h.companyServices.Project().GetById(billingCtx, &pb.GetProjectByIdRequest{ProjectId: projectID}); projectErr == nil {
		processor.companyId = project.GetCompanyId()
		processor.fareId = project.GetFareId()
	}
	processor.initTokenBudget(billingCtx)
	billingCancel()

	agent := openai.NewOpenAIAgent(h.baseConf, processor)
	result, err := processor.runCRMAssistantFlow(ctx, agent, req)
	if err != nil {
		h.log.Error("crm ai: request failed", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, "Mani AI so‘rovni bajara olmadi")
		return
	}

	response := models.CRMAssistantResponse{
		Reply:         strings.TrimSpace(result.reply),
		ClientActions: result.clientActions,
	}
	if response.Reply == "" && len(response.ClientActions) > 0 {
		response.Reply = "Tayyor. Kartochka maydonlari sozlandi."
	}
	if result.pendingAction != nil {
		proposal, proposalErr := h.storeCRMAction(ctx, c, userID, result.pendingAction)
		if proposalErr != nil {
			h.log.Error("crm ai: pending action store failed", logger.Error(proposalErr))
			h.HandleResponse(c, status_http.ServiceUnavailable, "Tasdiqlash amalini vaqtincha saqlab bo‘lmadi")
			return
		}
		response.PendingAction = proposal
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) ConfirmCRMAssistantAction(c *gin.Context) {
	var req models.CRMConfirmActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid request body")
		return
	}
	if h.centralRedis == nil {
		h.HandleResponse(c, status_http.ServiceUnavailable, "CRM action storage is unavailable")
		return
	}

	userID, err := h.getAiChatUserID(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "user identity is required")
		return
	}
	key := crmActionKey(c.Param("action-id"))
	storedJSON, err := h.centralRedis.Get(c.Request.Context(), key).Bytes()
	if err == go_redis.Nil {
		h.HandleResponse(c, status_http.NotFound, "action expired or was not found")
		return
	}
	if err != nil {
		h.HandleResponse(c, status_http.ServiceUnavailable, "CRM action storage is unavailable")
		return
	}

	var stored storedCRMAction
	if err := json.Unmarshal(storedJSON, &stored); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "stored action is invalid")
		return
	}
	if stored.UserID != userID || stored.ProjectID != c.GetString("project_id") || stored.EnvironmentID != c.GetString("environment_id") {
		h.HandleResponse(c, status_http.Forbidden, "action does not belong to this user")
		return
	}

	if !req.Confirmed {
		_, _ = h.centralRedis.Del(c.Request.Context(), key).Result()
		reply := stored.Action.CancelMessage
		if strings.TrimSpace(reply) == "" {
			reply = "Amal bekor qilindi."
		}
		h.HandleResponse(c, status_http.OK, models.CRMAssistantResponse{Reply: reply})
		return
	}
	if stored.Action.Action == "client_pipeline" {
		pipelineAction, decodeErr := decodeStoredCRMPipelineAction(stored.Action.Data)
		if decodeErr != nil {
			h.log.Error("crm ai: stored pipeline action is invalid", logger.Error(decodeErr))
			h.HandleResponse(c, status_http.InternalServerError, "stored pipeline action is invalid")
			return
		}
		_, _ = h.centralRedis.Del(c.Request.Context(), key).Result()
		reply := stored.Action.SuccessMessage
		if strings.TrimSpace(reply) == "" {
			reply = "Pipeline va bosqichlar yangilandi."
		}
		h.HandleResponse(c, status_http.OK, models.CRMAssistantResponse{
			Reply: reply,
			ClientActions: []models.CRMClientAction{{
				Type:           "manage_pipeline",
				Table:          "deals",
				PipelineAction: pipelineAction,
			}},
		})
		return
	}
	if stored.Action.Action == "client_record" {
		recordAction, decodeErr := decodeStoredCRMRecordAction(stored.Action.Data)
		if decodeErr != nil {
			h.log.Error("crm ai: stored record action is invalid", logger.Error(decodeErr))
			h.HandleResponse(c, status_http.InternalServerError, "stored record action is invalid")
			return
		}
		_, _ = h.centralRedis.Del(c.Request.Context(), key).Result()
		reply := stored.Action.SuccessMessage
		if strings.TrimSpace(reply) == "" {
			reply = "CRM yozuvi yangilandi."
		}
		h.HandleResponse(c, status_http.OK, models.CRMAssistantResponse{
			Reply: reply,
			ClientActions: []models.CRMClientAction{{
				Type:         "manage_record",
				Table:        recordAction.Table,
				RecordAction: recordAction,
			}},
		})
		return
	}
	if stored.Action.Action == "client_batch" {
		batchAction, decodeErr := decodeStoredCRMBatchAction(stored.Action.Data)
		if decodeErr != nil {
			h.log.Error("crm ai: stored batch action is invalid", logger.Error(decodeErr))
			h.HandleResponse(c, status_http.InternalServerError, "stored batch action is invalid")
			return
		}
		_, _ = h.centralRedis.Del(c.Request.Context(), key).Result()
		reply := stored.Action.SuccessMessage
		if strings.TrimSpace(reply) == "" {
			reply = "Pipeline yaratildi va deal ma’lumotlari import qilindi."
		}
		h.HandleResponse(c, status_http.OK, models.CRMAssistantResponse{
			Reply: reply,
			ClientActions: []models.CRMClientAction{{
				Type: "manage_batch", Table: "deals", BatchAction: batchAction,
			}},
		})
		return
	}

	service, resourceEnvID, err := h.getAiChatServices(c)
	if err != nil {
		return
	}
	if stored.Action.ResourceEnvID != resourceEnvID {
		h.HandleResponse(c, status_http.Forbidden, "action resource changed")
		return
	}
	if _, err = executeMutation(c.Request.Context(), &stored.Action, service); err != nil {
		h.log.Error("crm ai: confirmed action failed", logger.Error(err))
		h.HandleResponse(c, status_http.InternalServerError, "Tasdiqlangan amal bajarilmadi")
		return
	}
	_, _ = h.centralRedis.Del(c.Request.Context(), key).Result()
	reply := stored.Action.SuccessMessage
	if strings.TrimSpace(reply) == "" {
		reply = "Amal bajarildi."
	}
	h.HandleResponse(c, status_http.OK, models.CRMAssistantResponse{
		Reply:         reply,
		ClientActions: []models.CRMClientAction{{Type: "refresh_crm_data", Table: "*"}},
	})
}

func (h *HandlerV1) GetCRMFieldPreferences(c *gin.Context) {
	userID, ok := h.resolveCRMPreferenceRequest(c)
	if !ok {
		return
	}
	if h.centralRedis == nil {
		h.HandleResponse(c, status_http.ServiceUnavailable, "CRM preferences storage is unavailable")
		return
	}

	table := c.Param("table")
	value, err := h.centralRedis.Get(c.Request.Context(), crmPreferenceKey(c.GetString("project_id"), userID, table)).Bytes()
	if err == go_redis.Nil {
		h.HandleResponse(c, status_http.OK, models.CRMFieldPreferencesResponse{Table: table})
		return
	}
	if err != nil {
		h.HandleResponse(c, status_http.ServiceUnavailable, "CRM preferences storage is unavailable")
		return
	}

	var preferences models.CRMFieldPreferences
	if err := json.Unmarshal(value, &preferences); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "stored preferences are invalid")
		return
	}
	h.HandleResponse(c, status_http.OK, models.CRMFieldPreferencesResponse{
		Found:        true,
		Table:        table,
		HiddenFields: preferences.HiddenFields,
		FieldOrder:   preferences.FieldOrder,
	})
}

func (h *HandlerV1) UpdateCRMFieldPreferences(c *gin.Context) {
	userID, ok := h.resolveCRMPreferenceRequest(c)
	if !ok {
		return
	}
	if h.centralRedis == nil {
		h.HandleResponse(c, status_http.ServiceUnavailable, "CRM preferences storage is unavailable")
		return
	}

	var preferences models.CRMFieldPreferences
	if err := c.ShouldBindJSON(&preferences); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid request body")
		return
	}
	preferences.HiddenFields = normalizePreferenceFields(preferences.HiddenFields)
	preferences.FieldOrder = normalizePreferenceFields(preferences.FieldOrder)
	if len(preferences.HiddenFields) > crmMaxPreferenceFieldKeys || len(preferences.FieldOrder) > crmMaxPreferenceFieldKeys {
		h.HandleResponse(c, status_http.BadRequest, "too many field keys")
		return
	}

	encoded, err := json.Marshal(preferences)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "failed to encode preferences")
		return
	}
	key := crmPreferenceKey(c.GetString("project_id"), userID, c.Param("table"))
	if err = h.centralRedis.Set(c.Request.Context(), key, encoded, crmPreferencesTTL).Err(); err != nil {
		h.HandleResponse(c, status_http.ServiceUnavailable, "CRM preferences storage is unavailable")
		return
	}
	h.HandleResponse(c, status_http.OK, preferences)
}

func (h *HandlerV1) storeCRMAction(
	ctx context.Context,
	c *gin.Context,
	userID string,
	action *models.PendingAction,
) (*models.CRMActionProposal, error) {
	if h.centralRedis == nil {
		return nil, fmt.Errorf("central redis is unavailable")
	}
	id := uuid.NewString()
	stored := storedCRMAction{
		UserID:        userID,
		ProjectID:     c.GetString("project_id"),
		EnvironmentID: c.GetString("environment_id"),
		Action:        *action,
	}
	encoded, err := json.Marshal(stored)
	if err != nil {
		return nil, err
	}
	if err = h.centralRedis.Set(ctx, crmActionKey(id), encoded, crmPendingActionTTL).Err(); err != nil {
		return nil, err
	}
	return &models.CRMActionProposal{
		ID:          id,
		Description: action.ConfirmationPrompt,
		Kind:        action.Action,
	}, nil
}

func (h *HandlerV1) resolveCRMPreferenceRequest(c *gin.Context) (string, bool) {
	table := c.Param("table")
	if !isCRMPreferenceSlug(table) {
		h.HandleResponse(c, status_http.BadRequest, "invalid table slug")
		return "", false
	}
	userID, err := h.getAiChatUserID(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "user identity is required")
		return "", false
	}
	return userID, true
}

func validateCRMImages(images []string) error {
	if len(images) > crmMaxImages {
		return fmt.Errorf("maximum %d screenshots are allowed", crmMaxImages)
	}
	for _, image := range images {
		header, encoded, ok := strings.Cut(image, ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			return fmt.Errorf("screenshots must be base64 image data")
		}
		mimeType := strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		switch mimeType {
		case "image/png", "image/jpeg", "image/webp":
		default:
			return fmt.Errorf("unsupported screenshot type %q", mimeType)
		}
		if base64.StdEncoding.DecodedLen(len(encoded)) > crmMaxImageDecodedBytes {
			return fmt.Errorf("each screenshot must be at most 6 MB")
		}
		if _, err := base64.StdEncoding.DecodeString(encoded); err != nil {
			return fmt.Errorf("invalid screenshot data")
		}
	}
	return nil
}

func normalizePreferenceFields(fields []string) []string {
	seen := make(map[string]struct{}, len(fields))
	result := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.TrimSpace(field)
		if !isCRMPreferenceSlug(field) {
			continue
		}
		if _, exists := seen[field]; exists {
			continue
		}
		seen[field] = struct{}{}
		result = append(result, field)
	}
	return result
}

func isCRMPreferenceSlug(value string) bool {
	if value == "" || len(value) > 100 {
		return false
	}
	for _, char := range value {
		if (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') && (char < '0' || char > '9') && char != '_' && char != '-' {
			return false
		}
	}
	return true
}

func crmActionKey(id string) string {
	return "crm-ai:action:" + id
}

func crmPreferenceKey(projectID, userID, table string) string {
	return fmt.Sprintf("crm-ai:preferences:%s:%s:%s", projectID, userID, table)
}

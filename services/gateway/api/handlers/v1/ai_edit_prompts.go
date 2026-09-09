package v1

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"

	chatprompts "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const maxAIEditPromptBytes = 128 * 1024

func (h *HandlerV1) editPromptStore() aiEditPromptStore {
	if h.aiEditPromptStore == nil {
		return unavailableAIEditPromptStore{}
	}
	return h.aiEditPromptStore
}

func (h *HandlerV1) GetAIEditPrompts(c *gin.Context) {
	service, resourceEnvID, err := h.getAiChatServices(c)
	if err != nil {
		return
	}
	h.respondWithAIEditPrompts(c, service, resourceEnvID)
}

func (h *HandlerV1) UpsertAIEditPrompt(c *gin.Context) {
	service, resourceEnvID, err := h.getAiChatServices(c)
	if err != nil {
		return
	}
	h.upsertAIEditPrompt(c, service, resourceEnvID)
}

func (h *HandlerV1) DeleteAIEditPrompt(c *gin.Context) {
	service, resourceEnvID, err := h.getAiChatServices(c)
	if err != nil {
		return
	}
	h.deleteAIEditPrompt(c, service, resourceEnvID)
}

// GetMcpProjectAIEditPrompts and its siblings serve the same prompt settings,
// but scoped to an MCP project: the builder database is the generated child
// project's one, resolved server-side from the trusted MCP row. This is the
// same database the AI edit pipeline reads overrides from, so saves through
// these routes always take effect regardless of the caller's header context.
func (h *HandlerV1) GetMcpProjectAIEditPrompts(c *gin.Context) {
	service, resourceEnvID, ok := h.resolveMcpChildPromptServices(c)
	if !ok {
		return
	}
	h.respondWithAIEditPrompts(c, service, resourceEnvID)
}

func (h *HandlerV1) UpsertMcpProjectAIEditPrompt(c *gin.Context) {
	service, resourceEnvID, ok := h.resolveMcpChildPromptServices(c)
	if !ok {
		return
	}
	h.upsertAIEditPrompt(c, service, resourceEnvID)
}

func (h *HandlerV1) DeleteMcpProjectAIEditPrompt(c *gin.Context) {
	service, resourceEnvID, ok := h.resolveMcpChildPromptServices(c)
	if !ok {
		return
	}
	h.deleteAIEditPrompt(c, service, resourceEnvID)
}

// resolveMcpChildPromptServices loads the MCP project from the caller's
// current project context and resolves the generated child project's builder
// service. It writes the HTTP error response itself on failure.
func (h *HandlerV1) resolveMcpChildPromptServices(c *gin.Context) (services.ServiceManagerI, string, bool) {
	mcpProjectID := c.Param("mcp_project_id")
	if !util.IsValidUUID(mcpProjectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "mcp_project_id is not a valid uuid")
		return nil, "", false
	}

	service, resourceEnvID, err := h.getAiChatServices(c)
	if err != nil {
		return nil, "", false
	}

	mcpProject, err := service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		c.Request.Context(), &pbo.McpProjectId{
			ResourceEnvId: resourceEnvID,
			Id:            mcpProjectID,
			WithoutFiles:  true,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to get mcp project: %v", err))
		return nil, "", false
	}

	childProjectID := strings.TrimSpace(mcpProject.GetUcodeProjectId())
	childEnvironmentID := strings.TrimSpace(mcpProject.GetEnvironmentId())
	if childProjectID == "" || childEnvironmentID == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "mcp project has no generated child project; AI edit prompts are unavailable")
		return nil, "", false
	}

	childService, childResourceEnvID, err := h.getBuilderService(c.Request.Context(), childProjectID, childEnvironmentID)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to resolve generated project services: %v", err))
		return nil, "", false
	}

	return childService, childResourceEnvID, true
}

func (h *HandlerV1) respondWithAIEditPrompts(c *gin.Context, service services.ServiceManagerI, resourceEnvID string) {
	prompts, err := h.editPromptStore().GetAll(c.Request.Context(), service, resourceEnvID)
	storageAvailable := true
	if err != nil {
		if !isAIEditPromptStorageUnavailable(err) {
			h.handleAIEditPromptError(c, err)
			return
		}
		prompts = nil
		storageAvailable = false
	}

	h.HandleResponse(c, status_http.OK, buildAIEditPromptSettings(prompts, storageAvailable))
}

func (h *HandlerV1) upsertAIEditPrompt(c *gin.Context, service services.ServiceManagerI, resourceEnvID string) {
	promptKind, err := parseAIEditPromptKind(c.Param("prompt_kind"))
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	var request models.UpsertAIEditPromptRequest
	if err = c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if strings.TrimSpace(request.Content) == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "content is required")
		return
	}
	if len(request.Content) > maxAIEditPromptBytes {
		h.HandleResponse(c, status_http.InvalidArgument, fmt.Sprintf("content must not exceed %d bytes", maxAIEditPromptBytes))
		return
	}
	if request.ExpectedRevision < 0 {
		h.HandleResponse(c, status_http.InvalidArgument, "expected_revision must not be negative")
		return
	}

	userID, err := h.getAiChatUserID(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "authenticated user id is required")
		return
	}

	prompt, err := h.editPromptStore().Upsert(
		c.Request.Context(),
		service,
		resourceEnvID,
		models.AIEditPrompt{
			PromptKind:      promptKind,
			Content:         request.Content,
			UpdatedByUserID: userID,
		},
		request.ExpectedRevision,
	)
	if err != nil {
		h.handleAIEditPromptError(c, err)
		return
	}

	h.HandleResponse(c, status_http.OK, buildAIEditPromptSetting(promptKind, &prompt))
}

func (h *HandlerV1) deleteAIEditPrompt(c *gin.Context, service services.ServiceManagerI, resourceEnvID string) {
	promptKind, err := parseAIEditPromptKind(c.Param("prompt_kind"))
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	expectedRevision, err := parseExpectedRevision(c.Query("expected_revision"))
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	if err = h.editPromptStore().Delete(c.Request.Context(), service, resourceEnvID, promptKind, expectedRevision); err != nil {
		h.handleAIEditPromptError(c, err)
		return
	}

	h.HandleResponse(c, status_http.OK, buildAIEditPromptSetting(promptKind, nil))
}

// loadAIEditPromptOverrides resolves the generated child project's builder DB
// from the trusted MCP mapping, then loads only the two supported custom prompt
// bodies. Prompt storage is optional during rollout: every read failure falls
// back to the compiled defaults and must never block an otherwise valid edit.
func (h *HandlerV1) loadAIEditPromptOverrides(
	ctx context.Context,
	childProjectID string,
	childEnvironmentID string,
) models.EditPromptOverrides {
	childProjectID = strings.TrimSpace(childProjectID)
	childEnvironmentID = strings.TrimSpace(childEnvironmentID)
	if childProjectID == "" || childEnvironmentID == "" {
		return models.EditPromptOverrides{}
	}

	childService, childResourceEnvID, err := h.getBuilderService(ctx, childProjectID, childEnvironmentID)
	if err != nil {
		log.Printf("[AI EDIT PROMPTS] child project prompt storage unavailable; using compiled defaults: %v", err)
		return models.EditPromptOverrides{}
	}
	return h.readAIEditPromptOverrides(ctx, childService, childResourceEnvID)
}

func (h *HandlerV1) readAIEditPromptOverrides(ctx context.Context, service services.ServiceManagerI, resourceEnvID string) models.EditPromptOverrides {
	prompts, err := h.editPromptStore().GetAll(ctx, service, resourceEnvID)
	if err != nil {
		log.Printf("[AI EDIT PROMPTS] prompt lookup failed; using compiled defaults: %v", err)
		return models.EditPromptOverrides{}
	}

	var overrides models.EditPromptOverrides
	for _, prompt := range prompts {
		switch prompt.PromptKind {
		case models.AIEditPromptKindCodeEditor:
			overrides.CodeEditor = prompt.Content
		case models.AIEditPromptKindVisualEditor:
			overrides.VisualEditor = prompt.Content
		}
	}
	return overrides
}

func parseAIEditPromptKind(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case models.AIEditPromptKindCodeEditor, models.AIEditPromptKindVisualEditor:
		return value, nil
	default:
		return "", fmt.Errorf("prompt_kind must be %q or %q", models.AIEditPromptKindCodeEditor, models.AIEditPromptKindVisualEditor)
	}
}

func parseExpectedRevision(value string) (int64, error) {
	if value == "" {
		return 0, nil
	}
	revision, err := strconv.ParseInt(value, 10, 64)
	if err != nil || revision < 0 {
		return 0, fmt.Errorf("expected_revision must be a non-negative integer")
	}
	return revision, nil
}

func buildAIEditPromptSettings(customPrompts []models.AIEditPrompt, storageAvailable bool) models.AIEditPromptSettingsResponse {
	customByKind := make(map[string]*models.AIEditPrompt, len(customPrompts))
	for i := range customPrompts {
		prompt := &customPrompts[i]
		if _, err := parseAIEditPromptKind(prompt.PromptKind); err == nil {
			customByKind[prompt.PromptKind] = prompt
		}
	}

	return models.AIEditPromptSettingsResponse{
		Prompts: []models.AIEditPromptSetting{
			buildAIEditPromptSetting(models.AIEditPromptKindCodeEditor, customByKind[models.AIEditPromptKindCodeEditor]),
			buildAIEditPromptSetting(models.AIEditPromptKindVisualEditor, customByKind[models.AIEditPromptKindVisualEditor]),
		},
		StorageAvailable: storageAvailable,
	}
}

func buildAIEditPromptSetting(promptKind string, customPrompt *models.AIEditPrompt) models.AIEditPromptSetting {
	defaultContent := defaultAIEditPromptContent(promptKind)
	setting := models.AIEditPromptSetting{
		PromptKind:     promptKind,
		Content:        defaultContent,
		DefaultContent: defaultContent,
		Source:         models.AIEditPromptSourceDefault,
	}
	if customPrompt == nil {
		return setting
	}

	customContent := customPrompt.Content
	setting.Content = customContent
	setting.CustomContent = &customContent
	setting.Source = models.AIEditPromptSourceCustom
	setting.Revision = customPrompt.Revision
	setting.UpdatedByUserID = customPrompt.UpdatedByUserID
	setting.CreatedAt = customPrompt.CreatedAt
	setting.UpdatedAt = customPrompt.UpdatedAt
	return setting
}

func defaultAIEditPromptContent(promptKind string) string {
	switch promptKind {
	case models.AIEditPromptKindCodeEditor:
		return chatprompts.PromptCodeEditor
	case models.AIEditPromptKindVisualEditor:
		return chatprompts.PromptVisualEdit
	default:
		return ""
	}
}

func isAIEditPromptStorageUnavailable(err error) bool {
	code := status.Code(err)
	return code == codes.Unimplemented || code == codes.FailedPrecondition
}

func (h *HandlerV1) handleAIEditPromptError(c *gin.Context, err error) {
	switch status.Code(err) {
	case codes.InvalidArgument:
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
	case codes.NotFound:
		h.HandleResponse(c, status_http.NotFound, err.Error())
	case codes.Aborted, codes.AlreadyExists:
		h.HandleResponse(c, status_http.Conflict, err.Error())
	case codes.Unimplemented, codes.FailedPrecondition:
		h.HandleResponse(c, status_http.ServiceUnavailable, "AI edit prompt storage is unavailable")
	default:
		h.HandleResponse(c, status_http.GRPCError, err.Error())
	}
}

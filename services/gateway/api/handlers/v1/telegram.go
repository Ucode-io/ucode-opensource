package v1

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	telegramManagedSessionPrefix  = "telegram:managed-session:"
	telegramManagedNoncePrefix    = "telegram:managed-nonce:"
	telegramManagedUserPrefix     = "telegram:managed-user:"
	telegramManagedUsernamePrefix = "telegram:managed-username:"
	telegramManagedSessionTTL     = 30 * time.Minute

	telegramContactsTable    = "ugen_telegram_contacts"
	telegramMessagesTable    = "ugen_telegram_messages"
	telegramAttachmentsTable = "ugen_telegram_attachments"
	telegramUpdatesTable     = "ugen_telegram_updates"
)

var telegramIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

type telegramChatMappingRequest struct {
	TableID             string `json:"table_id"`
	TableSlug           string `json:"table_slug"`
	TelegramChatIDField string `json:"telegram_chat_id_field"`
	TelegramUserIDField string `json:"telegram_user_id_field"`
	DisplayNameField    string `json:"display_name_field"`
	UsernameField       string `json:"username_field"`
	LastMessageField    string `json:"last_message_field"`
	LastMessageAtField  string `json:"last_message_at_field"`
	UnreadCountField    string `json:"unread_count_field"`
	StatusField         string `json:"status_field"`
}

func (r telegramChatMappingRequest) toProto() *pb.TelegramChatMapping {
	return &pb.TelegramChatMapping{
		TableId:             strings.TrimSpace(r.TableID),
		TableSlug:           strings.TrimSpace(r.TableSlug),
		TelegramChatIdField: strings.TrimSpace(r.TelegramChatIDField),
		TelegramUserIdField: strings.TrimSpace(r.TelegramUserIDField),
		DisplayNameField:    strings.TrimSpace(r.DisplayNameField),
		UsernameField:       strings.TrimSpace(r.UsernameField),
		LastMessageField:    strings.TrimSpace(r.LastMessageField),
		LastMessageAtField:  strings.TrimSpace(r.LastMessageAtField),
		UnreadCountField:    strings.TrimSpace(r.UnreadCountField),
		StatusField:         strings.TrimSpace(r.StatusField),
	}
}

type telegramManagedSessionRequest struct {
	DisplayName       string                     `json:"display_name"`
	SuggestedUsername string                     `json:"suggested_username"`
	Mapping           telegramChatMappingRequest `json:"mapping"`
}

type telegramConnectExistingRequest struct {
	BotToken string                     `json:"bot_token"`
	Mapping  telegramChatMappingRequest `json:"mapping"`
}

type telegramManagedSession struct {
	ID                string                  `json:"id"`
	Nonce             string                  `json:"nonce"`
	Status            string                  `json:"status"`
	ErrorMessage      string                  `json:"error_message,omitempty"`
	McpProjectID      string                  `json:"mcp_project_id"`
	ChildProjectID    string                  `json:"child_project_id"`
	ChildEnvironment  string                  `json:"child_environment_id"`
	InitiatorUserID   string                  `json:"initiator_user_id,omitempty"`
	TelegramUserID    string                  `json:"telegram_user_id,omitempty"`
	DisplayName       string                  `json:"display_name"`
	SuggestedUsername string                  `json:"suggested_username"`
	BotUsername       string                  `json:"bot_username,omitempty"`
	Mapping           *pb.TelegramChatMapping `json:"mapping"`
	ExpiresAt         time.Time               `json:"expires_at"`
}

type telegramManagedSessionResponse struct {
	ID                 string    `json:"id"`
	Status             string    `json:"status"`
	ManagerBotUsername string    `json:"manager_bot_username"`
	StartLink          string    `json:"start_link"`
	CreateLink         string    `json:"create_link"`
	DisplayName        string    `json:"display_name"`
	SuggestedUsername  string    `json:"suggested_username"`
	BotUsername        string    `json:"bot_username,omitempty"`
	ErrorMessage       string    `json:"error_message,omitempty"`
	ExpiresAt          time.Time `json:"expires_at"`
}

type telegramIntegrationResponse struct {
	ResourceID  string                  `json:"resource_id"`
	BotID       string                  `json:"bot_id"`
	BotUsername string                  `json:"bot_username"`
	Status      string                  `json:"status"`
	Mapping     *pb.TelegramChatMapping `json:"mapping,omitempty"`
	BotURL      string                  `json:"bot_url,omitempty"`
}

type telegramMappingFieldOption struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	IsUnique bool   `json:"is_unique"`
}

type telegramMappingTableOption struct {
	ID     string                       `json:"id"`
	Slug   string                       `json:"slug"`
	Label  string                       `json:"label"`
	Fields []telegramMappingFieldOption `json:"fields"`
}

type telegramProjectTarget struct {
	ProjectID     string
	EnvironmentID string
	ResourceEnvID string
	NodeType      string
	Services      services.ServiceManagerI
	McpProjectID  string
}

type telegramSecret struct {
	BotToken      string `json:"bot_token"`
	WebhookSecret string `json:"webhook_secret"`
}

type telegramAPIClient struct {
	token string
	http  *http.Client
}

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	Description string          `json:"description"`
}

type telegramBotUser struct {
	ID       int64  `json:"id"`
	Username string `json:"username"`
}

type telegramFile struct {
	FileID   string `json:"file_id"`
	FilePath string `json:"file_path"`
}

type telegramUpdate struct {
	UpdateID      int64            `json:"update_id"`
	Message       *telegramMessage `json:"message"`
	EditedMessage *telegramMessage `json:"edited_message"`
	ManagedBot    *telegramManaged `json:"managed_bot"`
	MyChatMember  json.RawMessage  `json:"my_chat_member"`
}

type telegramManaged struct {
	User telegramBotUser `json:"user"`
	Bot  telegramBotUser `json:"bot"`
}

type telegramMessage struct {
	MessageID int64 `json:"message_id"`
	Date      int64 `json:"date"`
	Chat      struct {
		ID        int64  `json:"id"`
		Type      string `json:"type"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	} `json:"chat"`
	From struct {
		ID        int64  `json:"id"`
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Username  string `json:"username"`
	} `json:"from"`
	Text     string          `json:"text"`
	Caption  string          `json:"caption"`
	Photo    json.RawMessage `json:"photo"`
	Document json.RawMessage `json:"document"`
	Video    json.RawMessage `json:"video"`
	Voice    json.RawMessage `json:"voice"`
	Audio    json.RawMessage `json:"audio"`
	Sticker  json.RawMessage `json:"sticker"`
}

func newTelegramAPIClient(token string) *telegramAPIClient {
	return &telegramAPIClient{
		token: strings.TrimSpace(token),
		http:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (t *telegramAPIClient) call(ctx context.Context, method string, payload any, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	endpoint := fmt.Sprintf("https://api.telegram.org/bot%s/%s", t.token, method)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := t.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var envelope telegramAPIResponse
	if err = json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if !envelope.OK {
		if envelope.Description == "" {
			envelope.Description = resp.Status
		}
		return errors.New(envelope.Description)
	}
	if result == nil || len(envelope.Result) == 0 {
		return nil
	}
	return json.Unmarshal(envelope.Result, result)
}

func (t *telegramAPIClient) getMe(ctx context.Context) (telegramBotUser, error) {
	var bot telegramBotUser
	err := t.call(ctx, "getMe", map[string]any{}, &bot)
	return bot, err
}

func (t *telegramAPIClient) setWebhook(ctx context.Context, webhookURL, secret string, allowedUpdates []string) error {
	return t.call(ctx, "setWebhook", map[string]any{
		"url":             webhookURL,
		"secret_token":    secret,
		"allowed_updates": allowedUpdates,
	}, nil)
}

func (t *telegramAPIClient) deleteWebhook(ctx context.Context) error {
	return t.call(ctx, "deleteWebhook", map[string]any{"drop_pending_updates": false}, nil)
}

func (t *telegramAPIClient) sendMessage(ctx context.Context, chatID string, text string) (telegramMessage, error) {
	return t.sendMessageWithMarkup(ctx, chatID, text, nil)
}

func (t *telegramAPIClient) sendMessageWithMarkup(ctx context.Context, chatID string, text string, replyMarkup any) (telegramMessage, error) {
	var message telegramMessage
	payload := map[string]any{"chat_id": chatID, "text": text}
	if replyMarkup != nil {
		payload["reply_markup"] = replyMarkup
	}
	err := t.call(ctx, "sendMessage", payload, &message)
	return message, err
}

func (t *telegramAPIClient) getManagedBotToken(ctx context.Context, userID string, replace bool) (string, error) {
	method := "getManagedBotToken"
	if replace {
		method = "replaceManagedBotToken"
	}
	var result json.RawMessage
	if err := t.call(ctx, method, map[string]any{"user_id": userID}, &result); err != nil {
		return "", err
	}

	var token string
	if err := json.Unmarshal(result, &token); err == nil && token != "" {
		return token, nil
	}
	var envelope struct {
		Token    string `json:"token"`
		BotToken string `json:"bot_token"`
	}
	if err := json.Unmarshal(result, &envelope); err != nil {
		return "", err
	}
	if envelope.Token != "" {
		return envelope.Token, nil
	}
	if envelope.BotToken != "" {
		return envelope.BotToken, nil
	}
	return "", errors.New("telegram managed bot token is empty")
}

func (t *telegramAPIClient) getFile(ctx context.Context, fileID string) (telegramFile, error) {
	var file telegramFile
	err := t.call(ctx, "getFile", map[string]any{"file_id": fileID}, &file)
	return file, err
}

func (t *telegramAPIClient) downloadFile(ctx context.Context, filePath string) (*http.Response, error) {
	if strings.TrimSpace(filePath) == "" || strings.Contains(filePath, "..") {
		return nil, errors.New("telegram file path is invalid")
	}
	endpoint := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", t.token, strings.TrimLeft(filePath, "/"))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	resp, err := t.http.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		resp.Body.Close()
		return nil, fmt.Errorf("telegram file download failed: %s", resp.Status)
	}
	return resp, nil
}

func (h *HandlerV1) telegramManagerConfigured() bool {
	return strings.TrimSpace(h.baseConf.TelegramManagerBotToken) != "" &&
		strings.TrimSpace(h.baseConf.TelegramManagerBotUsername) != "" &&
		strings.TrimSpace(h.baseConf.TelegramManagerWebhookSecret) != "" &&
		strings.TrimSpace(h.baseConf.TelegramWebhookBaseURL) != ""
}

func (h *HandlerV1) CreateTelegramManagedSession(c *gin.Context) {
	if !h.telegramManagerConfigured() {
		h.HandleResponse(c, status_http.BadRequest, "telegram manager bot is not configured; use connect-existing")
		return
	}

	var request telegramManagedSessionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	mcpProjectID := c.Param("mcp_project_id")
	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, mcpProjectID)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	mapping, err := h.prepareTelegramMapping(c.Request.Context(), target, request.Mapping.toProto())
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if err = h.ensureTelegramSystemTables(c.Request.Context(), target, mapping); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	nonce, err := telegramRandomToken(16)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	displayName := strings.TrimSpace(request.DisplayName)
	if displayName == "" {
		displayName = "Support"
	}
	if len([]rune(displayName)) > 64 {
		displayName = string([]rune(displayName)[:64])
	}

	session := telegramManagedSession{
		ID:                uuid.NewString(),
		Nonce:             nonce,
		Status:            "awaiting_link",
		McpProjectID:      target.McpProjectID,
		ChildProjectID:    target.ProjectID,
		ChildEnvironment:  target.EnvironmentID,
		DisplayName:       displayName,
		SuggestedUsername: normalizeTelegramBotUsername(request.SuggestedUsername, displayName),
		Mapping:           mapping,
		ExpiresAt:         time.Now().Add(telegramManagedSessionTTL),
	}
	if value, ok := c.Get("user_id"); ok {
		session.InitiatorUserID, _ = value.(string)
	}

	if err = h.storeTelegramManagedSession(c.Request.Context(), &session); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, h.telegramSessionResponse(&session))
}

func (h *HandlerV1) GetTelegramManagedSession(c *gin.Context) {
	mcpProjectID := c.Param("mcp_project_id")
	if _, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, mcpProjectID); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	session, err := h.loadTelegramManagedSession(c.Request.Context(), c.Param("session_id"))
	if err != nil || session.McpProjectID != mcpProjectID {
		h.HandleResponse(c, status_http.NotFound, "telegram managed session not found")
		return
	}
	if time.Now().After(session.ExpiresAt) && (session.Status == "awaiting_link" || session.Status == "awaiting_creation") {
		session.Status = "expired"
		_ = h.storeTelegramManagedSession(c.Request.Context(), session)
	}
	h.HandleResponse(c, status_http.OK, h.telegramSessionResponse(session))
}

func (h *HandlerV1) ConnectExistingTelegramBot(c *gin.Context) {
	var request telegramConnectExistingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.BotToken) == "" {
		h.HandleResponse(c, status_http.BadRequest, "bot_token is required")
		return
	}

	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	mapping, err := h.prepareTelegramMapping(c.Request.Context(), target, request.Mapping.toProto())
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if err = h.ensureTelegramSystemTables(c.Request.Context(), target, mapping); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	bot, err := newTelegramAPIClient(request.BotToken).getMe(c.Request.Context())
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, fmt.Sprintf("invalid telegram bot token: %v", err))
		return
	}
	resource, err := h.connectTelegramBot(c.Request.Context(), target, mapping, request.BotToken, bot)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.Created, telegramResourceResponse(resource))
}

func (h *HandlerV1) GetTelegramIntegration(c *gin.Context) {
	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}
	h.HandleResponse(c, status_http.OK, telegramResourceResponse(resource))
}

// GetTelegramMappingOptions returns only the child generated project's tables
// and fields. Ugen uses it for the mapping step instead of trying to read the
// main MCP database as if it were the generated project's database.
func (h *HandlerV1) GetTelegramMappingOptions(c *gin.Context) {
	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	tables, err := target.Services.GoObjectBuilderService().Table().GetAll(c.Request.Context(), &pbo.GetAllTablesRequest{ProjectId: target.ResourceEnvID, Limit: 1000})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	options := make([]telegramMappingTableOption, 0, len(tables.GetTables()))
	for _, table := range tables.GetTables() {
		if strings.HasPrefix(table.GetSlug(), "ugen_telegram_") {
			continue
		}
		fields, fieldErr := target.Services.GoObjectBuilderService().Field().GetAll(c.Request.Context(), &pbo.GetAllFieldsRequest{TableId: table.GetId(), ProjectId: target.ResourceEnvID, Limit: 500})
		if fieldErr != nil {
			h.HandleResponse(c, status_http.GRPCError, fieldErr.Error())
			return
		}
		option := telegramMappingTableOption{ID: table.GetId(), Slug: table.GetSlug(), Label: table.GetLabel(), Fields: make([]telegramMappingFieldOption, 0, len(fields.GetFields()))}
		for _, field := range fields.GetFields() {
			option.Fields = append(option.Fields, telegramMappingFieldOption{ID: field.GetId(), Slug: field.GetSlug(), Label: field.GetLabel(), Type: field.GetType(), IsUnique: field.GetUnique()})
		}
		options = append(options, option)
	}
	h.HandleResponse(c, status_http.OK, gin.H{"tables": options})
}

// GetTelegramInboxPrompt produces the bounded system task consumed by the
// existing Ugen AI Chat pipeline. The token, webhook secret and Telegram file
// paths are intentionally not part of this contract.
func (h *HandlerV1) GetTelegramInboxPrompt(c *gin.Context) {
	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}
	telegram := resource.GetSettings().GetTelegram()
	h.HandleResponse(c, status_http.OK, gin.H{
		"status": resource.GetSettings().GetTelegram().GetStatus(),
		"prompt": telegramInboxPrompt(telegram.GetMapping()),
	})
}

// MarkTelegramInboxReady is called by Ugen only after the existing AI Chat
// request has completed successfully. It is deliberately retry-safe.
func (h *HandlerV1) MarkTelegramInboxReady(c *gin.Context) {
	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}
	telegram := resource.GetSettings().GetTelegram()
	telegram.Status = "connected"
	if _, err = h.companyServices.Resource().UpdateProjectResource(c.Request.Context(), &pb.ProjectResource{
		Id: resource.GetId(), ProjectId: target.ProjectID, EnvironmentId: target.EnvironmentID,
		Name: resource.GetName(), Type: pb.ResourceType_TELEGRAM.String(), ResourceType: int32(pb.ResourceType_TELEGRAM),
		Settings: resource.GetSettings(),
	}); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, telegramResourceResponse(resource))
}

func (h *HandlerV1) DisconnectTelegramIntegration(c *gin.Context) {
	target, err := h.resolveTelegramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}

	secret, err := h.loadTelegramSecret(resource)
	if err == nil && secret.BotToken != "" {
		_ = newTelegramAPIClient(secret.BotToken).deleteWebhook(c.Request.Context())
	}
	if _, err = h.companyServices.Resource().DeleteProjectResource(c.Request.Context(), &pb.PrimaryKeyProjectResource{
		Id:            resource.GetId(),
		ProjectId:     target.ProjectID,
		EnvironmentId: target.EnvironmentID,
	}); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.NoContent, gin.H{})
}

func (h *HandlerV1) TelegramManagerWebhook(c *gin.Context) {
	if !h.telegramManagerConfigured() || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Telegram-Bot-Api-Secret-Token")), []byte(h.baseConf.TelegramManagerWebhookSecret)) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}

	var update telegramUpdate
	if err := c.ShouldBindJSON(&update); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	if update.ManagedBot != nil {
		h.handleTelegramManagedBotUpdate(c.Request.Context(), update.ManagedBot)
		c.Status(http.StatusOK)
		return
	}
	if update.Message != nil && strings.HasPrefix(strings.TrimSpace(update.Message.Text), "/start") {
		h.handleTelegramManagerStart(c.Request.Context(), update.Message)
	}
	c.Status(http.StatusOK)
}

func (h *HandlerV1) TelegramProjectWebhook(c *gin.Context) {
	target, err := h.resolveTelegramProjectTarget(c.Request.Context(), c.Param("project_id"), c.Param("environment_id"))
	if err != nil {
		c.Status(http.StatusNotFound)
		return
	}
	resource, err := h.getTelegramResourceByID(c.Request.Context(), target, c.Param("resource_id"))
	if err != nil || resource.GetSettings().GetTelegram() == nil {
		c.Status(http.StatusNotFound)
		return
	}
	secret, err := h.loadTelegramSecret(resource)
	if err != nil || subtle.ConstantTimeCompare([]byte(c.GetHeader("X-Telegram-Bot-Api-Secret-Token")), []byte(secret.WebhookSecret)) != 1 {
		c.Status(http.StatusUnauthorized)
		return
	}

	raw, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	var update telegramUpdate
	if err = json.Unmarshal(raw, &update); err != nil {
		c.Status(http.StatusBadRequest)
		return
	}
	message := update.Message
	if message == nil {
		message = update.EditedMessage
	}
	if message == nil || message.Chat.Type != "private" {
		c.Status(http.StatusOK)
		return
	}
	if err = h.persistTelegramIncomingMessage(c.Request.Context(), target, resource.GetSettings().GetTelegram().GetMapping(), update, message, string(raw)); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.Status(http.StatusOK)
}

func (h *HandlerV1) ListTelegramChats(c *gin.Context) {
	if !h.requireTelegramInboxAccess(c) {
		return
	}
	target, err := h.resolveTelegramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}
	mapping := resource.GetSettings().GetTelegram().GetMapping()
	rows, err := h.listTelegramChatRows(c.Request.Context(), target, mapping, c.Query("search"))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, gin.H{"chats": rows})
}

func (h *HandlerV1) ListTelegramMessages(c *gin.Context) {
	if !h.requireTelegramInboxAccess(c) {
		return
	}
	target, err := h.resolveTelegramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(c.Param("chat_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "chat_id is invalid")
		return
	}
	mapping, err := h.telegramMapping(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, err.Error())
		return
	}
	relationField, err := telegramMessageChatRelationField(mapping)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	rows, err := h.executeTelegramSQL(c.Request.Context(), target,
		`SELECT m.guid, m.`+telegramQuoteIdentifier(relationField)+` AS chat_record_id, m.telegram_message_id, m.direction, m.message_type, m.text, m.sent_at, m.delivery_status, (SELECT COUNT(*) FROM "`+telegramAttachmentsTable+`" a WHERE a.message_key = m.telegram_message_key AND a.deleted_at IS NULL) AS attachment_count FROM "`+telegramMessagesTable+`" m WHERE m.`+telegramQuoteIdentifier(relationField)+` = $1 AND m.deleted_at IS NULL ORDER BY m.sent_at ASC`,
		[]string{c.Param("chat_id")})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, gin.H{"messages": rows})
}

func (h *HandlerV1) ListTelegramMessageAttachments(c *gin.Context) {
	if !h.requireTelegramInboxAccess(c) {
		return
	}
	target, err := h.resolveTelegramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(c.Param("message_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "message_id is invalid")
		return
	}
	rows, err := h.executeTelegramSQL(c.Request.Context(), target, `SELECT a.guid, a.kind, a.file_name FROM "`+telegramAttachmentsTable+`" a JOIN "`+telegramMessagesTable+`" m ON m.telegram_message_key = a.message_key WHERE m.guid = $1 AND a.deleted_at IS NULL AND m.deleted_at IS NULL ORDER BY a.created_at ASC`, []string{c.Param("message_id")})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	attachments := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		attachment := row.AsMap()
		attachment["download_url"] = fmt.Sprintf("/v1/telegram/messages/%s/attachments/%s", c.Param("message_id"), attachment["guid"])
		attachments = append(attachments, attachment)
	}
	h.HandleResponse(c, status_http.OK, gin.H{"attachments": attachments})
}

func (h *HandlerV1) ProxyTelegramAttachment(c *gin.Context) {
	if !h.requireTelegramInboxAccess(c) {
		return
	}
	target, err := h.resolveTelegramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(c.Param("message_id")) || !util.IsValidUUID(c.Param("attachment_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "message_id or attachment_id is invalid")
		return
	}
	rows, err := h.executeTelegramSQL(c.Request.Context(), target, `SELECT a.file_id, a.file_name FROM "`+telegramAttachmentsTable+`" a JOIN "`+telegramMessagesTable+`" m ON m.telegram_message_key = a.message_key WHERE m.guid = $1 AND a.guid = $2 AND a.deleted_at IS NULL AND m.deleted_at IS NULL LIMIT 1`, []string{c.Param("message_id"), c.Param("attachment_id")})
	if err != nil || len(rows) == 0 {
		h.HandleResponse(c, status_http.NotFound, "telegram attachment not found")
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}
	secret, err := h.loadTelegramSecret(resource)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	attachment := rows[0].AsMap()
	file, err := newTelegramAPIClient(secret.BotToken).getFile(c.Request.Context(), fmt.Sprint(attachment["file_id"]))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, "telegram attachment is unavailable")
		return
	}
	response, err := newTelegramAPIClient(secret.BotToken).downloadFile(c.Request.Context(), file.FilePath)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	defer response.Body.Close()
	contentType := response.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileName := strings.ReplaceAll(fmt.Sprint(attachment["file_name"]), `"`, "")
	if fileName != "" {
		c.Header("Content-Disposition", `inline; filename="`+fileName+`"`)
	}
	c.DataFromReader(http.StatusOK, response.ContentLength, contentType, response.Body, nil)
}

func (h *HandlerV1) SendTelegramMessage(c *gin.Context) {
	if !h.requireTelegramInboxAccess(c) {
		return
	}
	var request struct {
		Text string `json:"text"`
	}
	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if strings.TrimSpace(request.Text) == "" || !util.IsValidUUID(c.Param("chat_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "chat_id and text are required")
		return
	}
	target, err := h.resolveTelegramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getTelegramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "telegram integration is not connected")
		return
	}
	mapping := resource.GetSettings().GetTelegram().GetMapping()
	chatID, err := h.telegramChatIDForRecord(c.Request.Context(), target, mapping, c.Param("chat_id"))
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, err.Error())
		return
	}
	secret, err := h.loadTelegramSecret(resource)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	message, err := newTelegramAPIClient(secret.BotToken).sendMessage(c.Request.Context(), chatID, request.Text)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if err = h.persistTelegramMessage(c.Request.Context(), target, c.Param("chat_id"), message, "outbound", "sent", ""); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.Created, gin.H{"message_id": message.MessageID})
}

func (h *HandlerV1) resolveTelegramMcpTarget(ctx context.Context, c *gin.Context, mcpProjectID string) (*telegramProjectTarget, error) {
	if !util.IsValidUUID(mcpProjectID) {
		return nil, errors.New("mcp_project_id is invalid")
	}
	mainProjectValue, ok := c.Get("project_id")
	mainProjectID, isProjectID := mainProjectValue.(string)
	if !ok || !isProjectID || !util.IsValidUUID(mainProjectID) {
		return nil, errors.New("project id is invalid")
	}
	mainEnvironmentValue, ok := c.Get("environment_id")
	mainEnvironmentID, isEnvironmentID := mainEnvironmentValue.(string)
	if !ok || !isEnvironmentID || !util.IsValidUUID(mainEnvironmentID) {
		return nil, errors.New("environment id is invalid")
	}
	mainResource, err := h.companyServices.ServiceResource().GetSingle(ctx, &pb.GetSingleServiceResourceReq{
		ProjectId: mainProjectID, EnvironmentId: mainEnvironmentID, ServiceType: pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		return nil, fmt.Errorf("get main builder resource: %w", err)
	}
	mainServices, err := h.GetProjectSrvc(ctx, mainProjectID, mainResource.GetNodeType())
	if err != nil {
		return nil, err
	}
	mcp, err := mainServices.GoObjectBuilderService().McpProject().GetMcpProjectFiles(ctx, &pbo.McpProjectId{ResourceEnvId: mainResource.GetResourceEnvironmentId(), Id: mcpProjectID})
	if err != nil {
		return nil, fmt.Errorf("get mcp project: %w", err)
	}
	if !util.IsValidUUID(mcp.GetUcodeProjectId()) || !util.IsValidUUID(mcp.GetEnvironmentId()) {
		return nil, errors.New("mcp project has no generated project context")
	}

	target, err := h.resolveTelegramProjectTarget(ctx, mcp.GetUcodeProjectId(), mcp.GetEnvironmentId())
	if err != nil {
		return nil, err
	}
	target.McpProjectID = mcpProjectID
	return target, nil
}

func (h *HandlerV1) resolveTelegramProjectTarget(ctx context.Context, projectID, environmentID string) (*telegramProjectTarget, error) {
	if !util.IsValidUUID(projectID) || !util.IsValidUUID(environmentID) {
		return nil, errors.New("telegram target project or environment is invalid")
	}
	resource, err := h.companyServices.ServiceResource().GetSingle(ctx, &pb.GetSingleServiceResourceReq{
		ProjectId: projectID, EnvironmentId: environmentID, ServiceType: pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		return nil, fmt.Errorf("get generated project builder resource: %w", err)
	}
	services, err := h.GetProjectSrvc(ctx, projectID, resource.GetNodeType())
	if err != nil {
		return nil, err
	}
	return &telegramProjectTarget{ProjectID: projectID, EnvironmentID: environmentID, ResourceEnvID: resource.GetResourceEnvironmentId(), NodeType: resource.GetNodeType(), Services: services}, nil
}

func (h *HandlerV1) resolveTelegramCurrentTarget(ctx context.Context, c *gin.Context) (*telegramProjectTarget, error) {
	projectID, ok := c.Get("project_id")
	if !ok {
		return nil, errors.New("project id is missing")
	}
	environmentID, ok := c.Get("environment_id")
	if !ok {
		return nil, errors.New("environment id is missing")
	}
	return h.resolveTelegramProjectTarget(ctx, fmt.Sprint(projectID), fmt.Sprint(environmentID))
}

func (h *HandlerV1) requireTelegramInboxAccess(c *gin.Context) bool {
	authHeader := strings.ToLower(strings.TrimSpace(c.GetHeader("Authorization")))
	if strings.HasPrefix(authHeader, "bearer ") {
		if _, err := h.GetAuthInfo(c); err != nil {
			return false
		}
		return true
	}
	if authHeader == "api-key" || strings.HasPrefix(authHeader, "api-key ") {
		if _, ok := c.Get("auth"); !ok {
			h.HandleResponse(c, status_http.Forbidden, "telegram inbox requires an authenticated api key")
			return false
		}
		if _, ok := c.Get("project_id"); !ok {
			h.HandleResponse(c, status_http.Forbidden, "telegram inbox requires project context")
			return false
		}
		if _, ok := c.Get("environment_id"); !ok {
			h.HandleResponse(c, status_http.Forbidden, "telegram inbox requires environment context")
			return false
		}
		return true
	}
	h.HandleResponse(c, status_http.Forbidden, "telegram inbox requires an authenticated user session or api key")
	return false
}

func (h *HandlerV1) getTelegramResource(ctx context.Context, target *telegramProjectTarget) (*pb.ProjectResource, error) {
	resources, err := h.companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId: target.ProjectID, EnvironmentId: target.EnvironmentID, Type: pb.ResourceType_TELEGRAM,
	})
	if err != nil {
		return nil, err
	}
	for _, resource := range resources.GetResources() {
		if resource.GetSettings() != nil && resource.GetSettings().GetTelegram() != nil {
			return resource, nil
		}
	}
	return nil, errors.New("telegram resource not found")
}

func (h *HandlerV1) getTelegramResourceByID(ctx context.Context, target *telegramProjectTarget, resourceID string) (*pb.ProjectResource, error) {
	if !util.IsValidUUID(resourceID) {
		return nil, errors.New("telegram resource id is invalid")
	}
	resource, err := h.companyServices.Resource().GetSingleProjectResouece(ctx, &pb.PrimaryKeyProjectResource{Id: resourceID, ProjectId: target.ProjectID, EnvironmentId: target.EnvironmentID})
	if err != nil {
		return nil, err
	}
	if resource.GetSettings() == nil || resource.GetSettings().GetTelegram() == nil {
		return nil, errors.New("telegram resource settings are missing")
	}
	return resource, nil
}

func (h *HandlerV1) telegramMapping(ctx context.Context, target *telegramProjectTarget) (*pb.TelegramChatMapping, error) {
	resource, err := h.getTelegramResource(ctx, target)
	if err != nil {
		return nil, err
	}
	mapping := resource.GetSettings().GetTelegram().GetMapping()
	if mapping == nil {
		return nil, errors.New("telegram mapping is missing")
	}
	if err = h.applyTelegramDefaultMappingFields(ctx, target, mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

func (h *HandlerV1) prepareTelegramMapping(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping) (*pb.TelegramChatMapping, error) {
	if mapping == nil || !util.IsValidUUID(mapping.GetTableId()) {
		return nil, errors.New("mapping.table_id is required")
	}
	table, err := target.Services.GoObjectBuilderService().Table().GetByID(ctx, &pbo.TablePrimaryKey{Id: mapping.GetTableId(), ProjectId: target.ResourceEnvID})
	if err != nil {
		return nil, fmt.Errorf("get mapped chat table: %w", err)
	}
	if table.GetSlug() == "" {
		return nil, errors.New("mapped chat table has no slug")
	}
	if strings.HasPrefix(table.GetSlug(), "ugen_telegram_") {
		return nil, errors.New("a Telegram system table cannot be used as the mapped chat table")
	}
	if mapping.GetTableSlug() != "" && mapping.GetTableSlug() != table.GetSlug() {
		return nil, errors.New("mapping.table_slug does not match table_id")
	}
	mapping.TableSlug = table.GetSlug()
	if mapping.GetTelegramChatIdField() == "" {
		mapping.TelegramChatIdField = "telegram_chat_id"
	}
	normalizeTelegramMappingFields(mapping)

	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{TableId: table.GetId(), ProjectId: target.ResourceEnvID, Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("get mapped chat table fields: %w", err)
	}
	known := make(map[string]*pbo.Field, len(fields.GetFields()))
	for _, field := range fields.GetFields() {
		known[field.GetSlug()] = field
	}
	applyTelegramDefaultMappingFieldsFromKnown(mapping, known)

	if !telegramIdentifierPattern.MatchString(mapping.GetTelegramChatIdField()) {
		return nil, fmt.Errorf("invalid mapped field %q", mapping.GetTelegramChatIdField())
	}
	if field := known[mapping.GetTelegramChatIdField()]; field != nil && !field.GetUnique() {
		return nil, fmt.Errorf("mapped telegram_chat_id field %q must be unique", mapping.GetTelegramChatIdField())
	}
	if known[mapping.GetTelegramChatIdField()] == nil {
		attrs, attrErr := helperFunc.ConvertMapToStruct(map[string]any{"label_en": "Telegram chat ID", "telegram_system": true})
		if attrErr != nil {
			return nil, attrErr
		}
		if _, err = target.Services.GoObjectBuilderService().Field().Create(ctx, &pbo.CreateFieldRequest{
			Id: uuid.NewString(), TableId: table.GetId(), ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			Slug: mapping.GetTelegramChatIdField(), Label: "Telegram chat ID", Type: "SINGLE_LINE", Unique: true,
			IsVisible: false, Attributes: attrs,
		}); err != nil {
			return nil, fmt.Errorf("create telegram mapping field %s: %w", mapping.GetTelegramChatIdField(), err)
		}
		known[mapping.GetTelegramChatIdField()] = &pbo.Field{Slug: mapping.GetTelegramChatIdField(), Type: "SINGLE_LINE", Unique: true}
	}
	if err = h.ensureTelegramMappedOptionalFields(ctx, target, table.GetId(), known, mapping); err != nil {
		return nil, err
	}
	if field := mapping.GetLastMessageAtField(); field != "" && known[field].GetType() != "DATE_TIME" {
		return nil, fmt.Errorf("mapped last_message_at field %q must use DATE_TIME", field)
	}
	if field := mapping.GetUnreadCountField(); field != "" && known[field].GetType() != "NUMBER" {
		return nil, fmt.Errorf("mapped unread_count field %q must use NUMBER", field)
	}
	return mapping, nil
}

func (h *HandlerV1) ensureTelegramMappedOptionalFields(ctx context.Context, target *telegramProjectTarget, tableID string, known map[string]*pbo.Field, mapping *pb.TelegramChatMapping) error {
	definitions := map[string]telegramSystemField{
		mapping.GetTelegramUserIdField(): {"telegram_user_id", "Telegram user ID", "SINGLE_LINE", false},
		mapping.GetDisplayNameField():    {"customer_name", "Customer name", "SINGLE_LINE", false},
		mapping.GetUsernameField():       {"telegram_username", "Telegram username", "SINGLE_LINE", false},
		mapping.GetLastMessageField():    {"last_message", "Last message", "MULTI_LINE", false},
		mapping.GetLastMessageAtField():  {"last_message_at", "Last message at", "DATE_TIME", false},
		mapping.GetUnreadCountField():    {"unread_count", "Unread count", "NUMBER", false},
		mapping.GetStatusField():         {"status", "Status", "SINGLE_LINE", false},
	}

	for _, field := range []string{
		mapping.GetTelegramUserIdField(),
		mapping.GetDisplayNameField(),
		mapping.GetUsernameField(),
		mapping.GetLastMessageField(),
		mapping.GetLastMessageAtField(),
		mapping.GetUnreadCountField(),
		mapping.GetStatusField(),
	} {
		if field == "" {
			continue
		}
		if !telegramIdentifierPattern.MatchString(field) {
			return fmt.Errorf("invalid mapped optional field %q", field)
		}
		if known[field] != nil {
			continue
		}
		definition, ok := definitions[field]
		if !ok || definition.slug == "" {
			return fmt.Errorf("mapped optional field %q does not exist in table", field)
		}
		attrs, attrErr := helperFunc.ConvertMapToStruct(map[string]any{"label_en": definition.label, "telegram_system": true})
		if attrErr != nil {
			return attrErr
		}
		if _, err := target.Services.GoObjectBuilderService().Field().Create(ctx, &pbo.CreateFieldRequest{
			Id: uuid.NewString(), TableId: tableID, ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			Slug: field, Label: definition.label, Type: definition.typeID, Unique: definition.unique,
			IsVisible: false, Attributes: attrs,
		}); err != nil {
			return fmt.Errorf("create telegram mapping field %s: %w", field, err)
		}
		known[field] = &pbo.Field{Slug: field, Type: definition.typeID, Unique: definition.unique}
	}
	return nil
}

func (h *HandlerV1) applyTelegramDefaultMappingFields(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping) error {
	if mapping == nil || !util.IsValidUUID(mapping.GetTableId()) {
		return nil
	}
	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{TableId: mapping.GetTableId(), ProjectId: target.ResourceEnvID, Limit: 500})
	if err != nil {
		return fmt.Errorf("get telegram mapped table fields: %w", err)
	}
	known := make(map[string]*pbo.Field, len(fields.GetFields()))
	for _, field := range fields.GetFields() {
		known[field.GetSlug()] = field
	}
	applyTelegramDefaultMappingFieldsFromKnown(mapping, known)
	return nil
}

func applyTelegramDefaultMappingFieldsFromKnown(mapping *pb.TelegramChatMapping, known map[string]*pbo.Field) {
	if mapping == nil || len(known) == 0 {
		return
	}
	if mapping.TelegramUserIdField == "" {
		mapping.TelegramUserIdField = telegramFirstExistingField(known, "", "telegram_user_id")
	}
	if mapping.DisplayNameField == "" {
		mapping.DisplayNameField = telegramFirstExistingField(known, "", "customer_name", "display_name", "name")
	}
	if mapping.UsernameField == "" {
		mapping.UsernameField = telegramFirstExistingField(known, "", "telegram_username", "username")
	}
	if mapping.LastMessageField == "" {
		mapping.LastMessageField = telegramFirstExistingField(known, "", "last_message")
	}
	if mapping.LastMessageAtField == "" {
		mapping.LastMessageAtField = telegramFirstExistingField(known, "DATE_TIME", "last_message_at")
	}
	if mapping.UnreadCountField == "" {
		mapping.UnreadCountField = telegramFirstExistingField(known, "NUMBER", "unread_count")
	}
	if mapping.StatusField == "" {
		mapping.StatusField = telegramFirstExistingField(known, "", "status")
	}
}

func normalizeTelegramMappingFields(mapping *pb.TelegramChatMapping) {
	if mapping == nil {
		return
	}
	if mapping.GetLastMessageField() == mapping.GetLastMessageAtField() && mapping.GetLastMessageAtField() == "last_message_at" {
		mapping.LastMessageField = "last_message"
	}
}

func telegramFirstExistingField(known map[string]*pbo.Field, requiredType string, slugs ...string) string {
	for _, slug := range slugs {
		field := known[slug]
		if field == nil {
			continue
		}
		if requiredType != "" && field.GetType() != requiredType {
			continue
		}
		return slug
	}
	return ""
}

func (h *HandlerV1) ensureTelegramSystemTables(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping) error {
	tables, err := target.Services.GoObjectBuilderService().Table().GetAll(ctx, &pbo.GetAllTablesRequest{ProjectId: target.ResourceEnvID, Limit: 1000})
	if err != nil {
		return err
	}
	existing := make(map[string]*pbo.Table, len(tables.GetTables()))
	for _, table := range tables.GetTables() {
		existing[table.GetSlug()] = table
	}
	definitions := []struct {
		slug   string
		label  string
		fields []*pbo.CreateFieldsRequest
	}{
		{telegramContactsTable, "Ugen Telegram Contacts", telegramSystemFields([]telegramSystemField{
			{"telegram_user_id", "Telegram user ID", "SINGLE_LINE", true}, {"display_name", "Display name", "SINGLE_LINE", false}, {"username", "Username", "SINGLE_LINE", false},
		})},
		{telegramMessagesTable, "Ugen Telegram Messages", telegramSystemFields([]telegramSystemField{
			{"telegram_message_key", "Telegram message key", "SINGLE_LINE", true}, {"telegram_message_id", "Telegram message ID", "SINGLE_LINE", false}, {"direction", "Direction", "SINGLE_LINE", false}, {"message_type", "Message type", "SINGLE_LINE", false}, {"text", "Text", "MULTI_LINE", false}, {"raw_payload", "Raw payload", "MULTI_LINE", false}, {"sent_at", "Sent at", "DATE_TIME", false}, {"delivery_status", "Delivery status", "SINGLE_LINE", false},
		})},
		{telegramAttachmentsTable, "Ugen Telegram Attachments", telegramSystemFields([]telegramSystemField{
			{"message_key", "Message key", "SINGLE_LINE", false}, {"telegram_attachment_key", "Telegram attachment key", "SINGLE_LINE", true}, {"file_id", "Telegram file ID", "SINGLE_LINE", false}, {"kind", "Attachment type", "SINGLE_LINE", false}, {"file_name", "File name", "SINGLE_LINE", false}, {"raw_payload", "Raw payload", "MULTI_LINE", false},
		})},
		{telegramUpdatesTable, "Ugen Telegram Updates", telegramSystemFields([]telegramSystemField{
			{"update_id", "Telegram update ID", "SINGLE_LINE", true}, {"update_status", "Update status", "SINGLE_LINE", false},
		})},
	}
	for _, definition := range definitions {
		if table := existing[definition.slug]; table != nil {
			if err = h.ensureTelegramSystemFields(ctx, target, table.GetId(), definition.fields); err != nil {
				return fmt.Errorf("ensure telegram system table %s fields: %w", definition.slug, err)
			}
			continue
		}
		attrs, attrErr := helperFunc.ConvertMapToStruct(map[string]any{"telegram_system": true, "label_en": definition.label})
		if attrErr != nil {
			return attrErr
		}
		if _, err = target.Services.GoObjectBuilderService().Table().Create(ctx, &pbo.CreateTableRequest{
			Id: uuid.NewString(), Label: definition.label, Slug: definition.slug, Fields: definition.fields,
			ShowInMenu: false, ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			UcodeProjectId: target.ProjectID, Attributes: attrs, SoftDelete: true,
		}); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return fmt.Errorf("create telegram system table %s: %w", definition.slug, err)
		}
	}
	return h.ensureTelegramMessageRelation(ctx, target, mapping)
}

func (h *HandlerV1) ensureTelegramSystemFields(ctx context.Context, target *telegramProjectTarget, tableID string, definitions []*pbo.CreateFieldsRequest) error {
	if !util.IsValidUUID(tableID) {
		return errors.New("telegram system table id is invalid")
	}
	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{TableId: tableID, ProjectId: target.ResourceEnvID, Limit: 500})
	if err != nil {
		return err
	}
	existing := make(map[string]bool, len(fields.GetFields()))
	for _, field := range fields.GetFields() {
		existing[field.GetSlug()] = true
	}
	for _, definition := range definitions {
		if existing[definition.GetSlug()] {
			continue
		}
		if _, err = target.Services.GoObjectBuilderService().Field().Create(ctx, &pbo.CreateFieldRequest{
			Id: definition.GetId(), TableId: tableID, ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			Slug: definition.GetSlug(), Label: definition.GetLabel(), Type: definition.GetType(), Unique: definition.GetUnique(),
			IsVisible: definition.GetIsVisible(), Attributes: definition.GetAttributes(),
		}); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "already exists") {
				continue
			}
			return err
		}
	}
	return nil
}

func (h *HandlerV1) ensureTelegramMessageRelation(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping) error {
	relationField, err := telegramMessageChatRelationField(mapping)
	if err != nil {
		return err
	}
	relations, err := target.Services.GoObjectBuilderService().Relation().GetAll(ctx, &pbo.GetAllRelationsRequest{
		TableSlug: telegramMessagesTable, ProjectId: target.ResourceEnvID, Limit: 500,
	})
	if err != nil {
		return fmt.Errorf("get telegram message relations: %w", err)
	}
	for _, relation := range relations.GetRelations() {
		if relation.GetTableFrom().GetSlug() == telegramMessagesTable && relation.GetTableTo().GetSlug() == mapping.GetTableSlug() && relation.GetRelationFieldSlug() == relationField {
			return nil
		}
	}
	viewFieldID, err := h.telegramMessageRelationViewFieldID(ctx, target)
	if err != nil {
		return fmt.Errorf("get telegram message field for relation: %w", err)
	}
	attributes, err := helperFunc.ConvertMapToStruct(map[string]any{
		"label_en": "Telegram chat", "label_to_en": "Telegram messages", "telegram_system": true,
	})
	if err != nil {
		return err
	}
	if _, err = target.Services.GoObjectBuilderService().Relation().Create(ctx, &pbo.CreateRelationRequest{
		Id: uuid.NewString(), TableFrom: telegramMessagesTable, TableTo: mapping.GetTableSlug(), Type: "Many2One",
		RelationTableSlug: mapping.GetTableSlug(), RelationFieldSlug: relationField,
		RelationFieldId: uuid.NewString(), RelationToFieldId: uuid.NewString(),
		ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID, ViewFields: []string{viewFieldID}, Attributes: attributes,
	}); err != nil {
		return fmt.Errorf("create telegram message relation: %w", err)
	}
	return nil
}

func (h *HandlerV1) telegramMessageRelationViewFieldID(ctx context.Context, target *telegramProjectTarget) (string, error) {
	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{
		TableSlug: telegramMessagesTable,
		ProjectId: target.ResourceEnvID,
		Limit:     500,
	})
	if err != nil {
		return "", err
	}
	if len(fields.GetFields()) == 0 {
		return "", errors.New("telegram message table has no fields")
	}
	preferredSlugs := map[string]struct{}{"text": {}, "telegram_message_id": {}}
	for _, slug := range []string{"text", "telegram_message_id"} {
		for _, field := range fields.GetFields() {
			if field.GetSlug() == slug && field.GetId() != "" {
				return field.GetId(), nil
			}
		}
	}
	for _, field := range fields.GetFields() {
		if _, preferred := preferredSlugs[field.GetSlug()]; preferred {
			continue
		}
		if field.GetId() != "" {
			return field.GetId(), nil
		}
	}
	return "", errors.New("telegram message table fields have no ids")
}

type telegramSystemField struct {
	slug   string
	label  string
	typeID string
	unique bool
}

func telegramSystemFields(definitions []telegramSystemField) []*pbo.CreateFieldsRequest {
	fields := make([]*pbo.CreateFieldsRequest, 0, len(definitions))
	for _, definition := range definitions {
		attrs, _ := structpb.NewStruct(map[string]any{"telegram_system": true, "label_en": definition.label})
		fields = append(fields, &pbo.CreateFieldsRequest{Id: uuid.NewString(), Slug: definition.slug, Label: definition.label, Type: definition.typeID, Unique: definition.unique, IsVisible: false, Attributes: attrs})
	}
	return fields
}

func (h *HandlerV1) connectTelegramBot(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping, token string, bot telegramBotUser) (*pb.ProjectResource, error) {
	if bot.ID == 0 || strings.TrimSpace(bot.Username) == "" {
		return nil, errors.New("telegram bot must have an id and username")
	}
	if _, err := h.getTelegramResource(ctx, target); err == nil {
		return nil, errors.New("telegram integration is already connected; disconnect it before replacing the bot")
	}

	webhookSecret, err := telegramRandomToken(24)
	if err != nil {
		return nil, err
	}
	secret, err := structpb.NewStruct(map[string]any{"bot_token": strings.TrimSpace(token), "webhook_secret": webhookSecret})
	if err != nil {
		return nil, fmt.Errorf("prepare telegram credentials: %w", err)
	}
	settings := &pb.Settings{Telegram: &pb.TelegramCredentials{
		BotId: fmt.Sprint(bot.ID), BotUsername: bot.Username, Status: "pending_ui", Mapping: mapping,
	}}

	resource, err := h.companyServices.Resource().AddResourceToProject(ctx, &pb.AddResourceToProjectRequest{
		Name: "Telegram Support", ProjectId: target.ProjectID, EnvironmentId: target.EnvironmentID, Type: pb.ResourceType_TELEGRAM,
		Settings: settings,
		Secret:   secret,
	})
	if err != nil {
		return nil, fmt.Errorf("save telegram resource: %w", err)
	}

	webhookURL := fmt.Sprintf("%s/v1/telegram/webhook/%s/%s/%s", strings.TrimRight(h.baseConf.TelegramWebhookBaseURL, "/"), target.ProjectID, target.EnvironmentID, resource.GetId())
	if err = newTelegramAPIClient(token).setWebhook(ctx, webhookURL, webhookSecret, []string{"message"}); err != nil {
		_, _ = h.companyServices.Resource().DeleteProjectResource(ctx, &pb.PrimaryKeyProjectResource{Id: resource.GetId(), ProjectId: target.ProjectID, EnvironmentId: target.EnvironmentID})
		return nil, fmt.Errorf("set telegram webhook: %w", err)
	}
	resource.Settings = settings
	resource.Secret = nil
	return resource, nil
}

func (h *HandlerV1) storeTelegramManagedSession(ctx context.Context, session *telegramManagedSession) error {
	if h.centralRedis == nil {
		return errors.New("central redis is not configured")
	}
	body, err := json.Marshal(session)
	if err != nil {
		return err
	}
	ttl := time.Until(session.ExpiresAt)
	if ttl <= 0 {
		return errors.New("telegram managed session is expired")
	}
	if err = h.centralRedis.Set(ctx, telegramManagedSessionPrefix+session.ID, body, ttl).Err(); err != nil {
		return err
	}
	if err = h.centralRedis.Set(ctx, telegramManagedNoncePrefix+session.Nonce, session.ID, ttl).Err(); err != nil {
		return err
	}
	if username := telegramManagedUsernameKey(session.SuggestedUsername); username != "" {
		if err = h.centralRedis.Set(ctx, telegramManagedUsernamePrefix+username, session.ID, ttl).Err(); err != nil {
			return err
		}
	}
	return nil
}

func (h *HandlerV1) loadTelegramManagedSession(ctx context.Context, id string) (*telegramManagedSession, error) {
	if h.centralRedis == nil || !util.IsValidUUID(id) {
		return nil, errors.New("telegram managed session not found")
	}
	body, err := h.centralRedis.Get(ctx, telegramManagedSessionPrefix+id).Bytes()
	if err != nil {
		return nil, err
	}
	var session telegramManagedSession
	if err = json.Unmarshal(body, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (h *HandlerV1) telegramSessionResponse(session *telegramManagedSession) telegramManagedSessionResponse {
	startLink := fmt.Sprintf("https://t.me/%s?start=mb_%s", h.baseConf.TelegramManagerBotUsername, session.Nonce)
	createLink := fmt.Sprintf("https://t.me/newbot/%s/%s?name=%s", h.baseConf.TelegramManagerBotUsername, session.SuggestedUsername, url.QueryEscape(session.DisplayName))
	return telegramManagedSessionResponse{ID: session.ID, Status: session.Status, ManagerBotUsername: h.baseConf.TelegramManagerBotUsername, StartLink: startLink, CreateLink: createLink, DisplayName: session.DisplayName, SuggestedUsername: session.SuggestedUsername, BotUsername: session.BotUsername, ErrorMessage: session.ErrorMessage, ExpiresAt: session.ExpiresAt}
}

func (h *HandlerV1) handleTelegramManagerStart(ctx context.Context, message *telegramMessage) {
	payload := strings.TrimSpace(message.Text)
	if !strings.HasPrefix(payload, "/start") {
		return
	}
	payload = strings.TrimSpace(strings.TrimPrefix(payload, "/start"))
	if strings.HasPrefix(payload, "@") {
		if parts := strings.Fields(payload); len(parts) > 1 {
			payload = parts[1]
		} else {
			return
		}
	}
	if !strings.HasPrefix(payload, "mb_") {
		return
	}
	nonce := strings.TrimPrefix(payload, "mb_")
	if nonce == "" || h.centralRedis == nil {
		return
	}
	sessionID, err := h.centralRedis.Get(ctx, telegramManagedNoncePrefix+nonce).Result()
	if err != nil {
		return
	}
	session, err := h.loadTelegramManagedSession(ctx, sessionID)
	if err != nil || session.Nonce != nonce || time.Now().After(session.ExpiresAt) || session.Status != "awaiting_link" {
		return
	}

	userKey := telegramManagedUserPrefix + fmt.Sprint(message.From.ID)
	if existingSessionID, getErr := h.centralRedis.Get(ctx, userKey).Result(); getErr == nil && existingSessionID != session.ID {
		if existing, loadErr := h.loadTelegramManagedSession(ctx, existingSessionID); loadErr == nil && existing.Status == "awaiting_creation" {
			_, _ = newTelegramAPIClient(h.baseConf.TelegramManagerBotToken).sendMessage(ctx, fmt.Sprint(message.Chat.ID), "Сначала завершите создание предыдущего бота или дождитесь истечения сессии.")
			return
		}
	}

	session.TelegramUserID = fmt.Sprint(message.From.ID)
	session.Status = "awaiting_creation"
	if h.storeTelegramManagedSession(ctx, session) != nil {
		return
	}
	_ = h.centralRedis.Set(ctx, userKey, session.ID, time.Until(session.ExpiresAt)).Err()
	createLink := h.telegramSessionResponse(session).CreateLink
	_, _ = newTelegramAPIClient(h.baseConf.TelegramManagerBotToken).sendMessageWithMarkup(ctx, fmt.Sprint(message.Chat.ID), "Создайте бота поддержки для проекта.", map[string]any{
		"inline_keyboard": [][]map[string]string{{{"text": "Создать Telegram-бота", "url": createLink}}},
	})
}

func (h *HandlerV1) handleTelegramManagedBotUpdate(ctx context.Context, managed *telegramManaged) {
	if h.centralRedis == nil || managed.Bot.ID == 0 {
		return
	}
	sessionID, err := h.telegramManagedSessionIDForBotUpdate(ctx, managed)
	if err != nil {
		return
	}
	session, err := h.loadTelegramManagedSession(ctx, sessionID)
	if err != nil || time.Now().After(session.ExpiresAt) {
		return
	}
	if session.Status != "awaiting_creation" && session.Status != "awaiting_link" {
		return
	}
	if managed.User.ID != 0 {
		session.TelegramUserID = fmt.Sprint(managed.User.ID)
	}

	manager := newTelegramAPIClient(h.baseConf.TelegramManagerBotToken)
	token, err := manager.getManagedBotToken(ctx, fmt.Sprint(managed.Bot.ID), false)
	if err != nil {
		token, err = manager.getManagedBotToken(ctx, fmt.Sprint(managed.Bot.ID), true)
	}
	if err != nil {
		session.Status, session.ErrorMessage = "error", "could not obtain managed bot token"
		_ = h.storeTelegramManagedSession(ctx, session)
		return
	}

	target, err := h.resolveTelegramProjectTarget(ctx, session.ChildProjectID, session.ChildEnvironment)
	if err == nil {
		_, err = h.connectTelegramBot(ctx, target, session.Mapping, token, managed.Bot)
	}
	if err != nil {
		session.Status, session.ErrorMessage = "error", truncateTelegramError(err)
		_ = h.storeTelegramManagedSession(ctx, session)
		return
	}
	session.Status, session.BotUsername, session.ErrorMessage = "completed", managed.Bot.Username, ""
	_ = h.storeTelegramManagedSession(ctx, session)
}

func (h *HandlerV1) telegramManagedSessionIDForBotUpdate(ctx context.Context, managed *telegramManaged) (string, error) {
	if h.centralRedis == nil || managed == nil {
		return "", errors.New("telegram managed session not found")
	}
	if username := telegramManagedUsernameKey(managed.Bot.Username); username != "" {
		if sessionID, err := h.centralRedis.Get(ctx, telegramManagedUsernamePrefix+username).Result(); err == nil && sessionID != "" {
			return sessionID, nil
		}
	}
	if managed.User.ID != 0 {
		if sessionID, err := h.centralRedis.Get(ctx, telegramManagedUserPrefix+fmt.Sprint(managed.User.ID)).Result(); err == nil && sessionID != "" {
			return sessionID, nil
		}
	}
	return "", errors.New("telegram managed session not found")
}

func (h *HandlerV1) persistTelegramIncomingMessage(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping, update telegramUpdate, message *telegramMessage, raw string) error {
	if mapping == nil {
		return errors.New("telegram mapping is missing")
	}
	processed, err := h.executeTelegramSQL(ctx, target, `SELECT guid, update_status FROM "`+telegramUpdatesTable+`" WHERE update_id = $1 LIMIT 1`, []string{fmt.Sprint(update.UpdateID)})
	if err != nil {
		return err
	}
	if len(processed) > 0 {
		status := fmt.Sprint(processed[0].AsMap()["update_status"])
		if status == "completed" {
			return nil
		}
		return errors.New("telegram update is already being processed")
	}
	if err = h.createTelegramItem(ctx, target, telegramUpdatesTable, map[string]any{"update_id": fmt.Sprint(update.UpdateID), "update_status": "processing"}); err != nil {
		// A concurrent webhook claimed the same update. Telegram will retry and
		// then see the completed marker if the other request succeeds.
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return errors.New("telegram update is already being processed")
		}
		return err
	}
	completed := false
	defer func() {
		if !completed {
			// The update row is a short transaction-like claim. If processing
			// fails, remove it so Telegram's retry can finish the write.
			_, _ = h.executeTelegramSQL(context.Background(), target, `DELETE FROM "`+telegramUpdatesTable+`" WHERE update_id = $1`, []string{fmt.Sprint(update.UpdateID)})
		}
	}()

	if err = h.ensureTelegramContact(ctx, target, message); err != nil {
		return err
	}
	chatRecordID, err := h.upsertTelegramChat(ctx, target, mapping, message)
	if err != nil {
		return err
	}
	if err = h.persistTelegramMessage(ctx, target, chatRecordID, *message, "inbound", "received", raw); err != nil {
		return err
	}
	if _, err = h.executeTelegramSQL(ctx, target, `UPDATE "`+telegramUpdatesTable+`" SET update_status = $1 WHERE update_id = $2`, []string{"completed", fmt.Sprint(update.UpdateID)}); err != nil {
		return err
	}
	completed = true
	return nil
}

func (h *HandlerV1) ensureTelegramContact(ctx context.Context, target *telegramProjectTarget, message *telegramMessage) error {
	userID := fmt.Sprint(message.From.ID)
	rows, err := h.executeTelegramSQL(ctx, target, `SELECT guid FROM "`+telegramContactsTable+`" WHERE telegram_user_id = $1 LIMIT 1`, []string{userID})
	if err != nil || len(rows) > 0 {
		return err
	}
	return h.createTelegramItem(ctx, target, telegramContactsTable, map[string]any{
		"telegram_user_id": userID,
		"display_name":     telegramDisplayName(message),
		"username":         message.From.Username,
	})
}

func (h *HandlerV1) upsertTelegramChat(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping, message *telegramMessage) (string, error) {
	if !telegramIdentifierPattern.MatchString(mapping.GetTableSlug()) || !telegramIdentifierPattern.MatchString(mapping.GetTelegramChatIdField()) {
		return "", errors.New("telegram mapping contains invalid table or chat id field")
	}
	chatID := fmt.Sprint(message.Chat.ID)
	query := fmt.Sprintf(`SELECT guid FROM %s WHERE %s = $1 AND deleted_at IS NULL LIMIT 1`, telegramQuoteIdentifier(mapping.GetTableSlug()), telegramQuoteIdentifier(mapping.GetTelegramChatIdField()))
	rows, err := h.executeTelegramSQL(ctx, target, query, []string{chatID})
	if err != nil {
		return "", err
	}
	if len(rows) > 0 {
		guid := fmt.Sprint(rows[0].AsMap()["guid"])
		messageKey := fmt.Sprintf("%s:%d", guid, message.MessageID)
		existingMessage, queryErr := h.executeTelegramSQL(ctx, target, `SELECT guid FROM "`+telegramMessagesTable+`" WHERE telegram_message_key = $1 AND deleted_at IS NULL LIMIT 1`, []string{messageKey})
		if queryErr != nil {
			return "", queryErr
		}
		if len(existingMessage) == 0 {
			if err = h.updateTelegramChatSummary(ctx, target, mapping, guid, message); err != nil {
				return "", err
			}
		}
		return guid, nil
	}

	data := map[string]any{mapping.GetTelegramChatIdField(): chatID}
	telegramPutMappedValue(data, mapping.GetTelegramUserIdField(), fmt.Sprint(message.From.ID))
	telegramPutMappedValue(data, mapping.GetDisplayNameField(), telegramDisplayName(message))
	telegramPutMappedValue(data, mapping.GetUsernameField(), message.From.Username)
	telegramPutMappedValue(data, mapping.GetLastMessageField(), telegramMessageText(message))
	telegramPutMappedValue(data, mapping.GetLastMessageAtField(), time.Unix(message.Date, 0).UTC().Format(time.RFC3339))
	telegramPutMappedValue(data, mapping.GetUnreadCountField(), 1)
	if mapping.GetStatusField() != "" {
		telegramPutMappedValue(data, mapping.GetStatusField(), "open")
	}
	result, err := h.createTelegramItemWithResponse(ctx, target, mapping.GetTableSlug(), data)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(result["guid"]), nil
}

func (h *HandlerV1) updateTelegramChatSummary(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping, guid string, message *telegramMessage) error {
	setClauses := make([]string, 0, 7)
	params := make([]string, 0, 8)
	if field := mapping.GetTelegramUserIdField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, fmt.Sprint(message.From.ID))
	}
	if field := mapping.GetDisplayNameField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, telegramDisplayName(message))
	}
	if field := mapping.GetUsernameField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, message.From.Username)
	}
	if field := mapping.GetLastMessageField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, telegramMessageText(message))
	}
	if field := mapping.GetLastMessageAtField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, time.Unix(message.Date, 0).UTC().Format(time.RFC3339))
	}
	if field := mapping.GetUnreadCountField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+" = COALESCE("+telegramQuoteIdentifier(field)+", 0) + 1")
	}
	if field := mapping.GetStatusField(); field != "" {
		setClauses = append(setClauses, telegramQuoteIdentifier(field)+fmt.Sprintf(" = COALESCE(NULLIF(%s::text, ''), $%d)", telegramQuoteIdentifier(field), len(params)+1))
		params = append(params, "open")
	}
	if len(setClauses) == 0 {
		return nil
	}
	params = append(params, guid)
	_, err := h.executeTelegramSQL(ctx, target, fmt.Sprintf(`UPDATE %s SET %s WHERE guid = $%d`, telegramQuoteIdentifier(mapping.GetTableSlug()), strings.Join(setClauses, ", "), len(params)), params)
	return err
}

func (h *HandlerV1) persistTelegramMessage(ctx context.Context, target *telegramProjectTarget, chatRecordID string, message telegramMessage, direction, deliveryStatus, raw string) error {
	if raw == "" {
		encoded, err := json.Marshal(message)
		if err != nil {
			return err
		}
		raw = string(encoded)
	}
	messageKey := fmt.Sprintf("%s:%d", chatRecordID, message.MessageID)
	mapping, err := h.telegramMapping(ctx, target)
	if err != nil {
		return err
	}
	relationField, err := telegramMessageChatRelationField(mapping)
	if err != nil {
		return err
	}
	data := map[string]any{
		relationField:          chatRecordID,
		"telegram_message_key": messageKey,
		"telegram_message_id":  fmt.Sprint(message.MessageID),
		"direction":            direction,
		"message_type":         telegramMessageType(&message),
		"text":                 telegramMessageText(&message),
		"raw_payload":          raw,
		"sent_at":              time.Unix(message.Date, 0).UTC().Format(time.RFC3339),
		"delivery_status":      deliveryStatus,
	}
	if err := h.createTelegramItem(ctx, target, telegramMessagesTable, data); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return h.persistTelegramAttachments(ctx, target, messageKey, &message)
		}
		return err
	}
	return h.persistTelegramAttachments(ctx, target, messageKey, &message)
}

func (h *HandlerV1) persistTelegramAttachments(ctx context.Context, target *telegramProjectTarget, messageKey string, message *telegramMessage) error {
	attachments := []struct {
		kind string
		raw  json.RawMessage
	}{
		{"photo", message.Photo}, {"document", message.Document}, {"video", message.Video}, {"voice", message.Voice}, {"audio", message.Audio}, {"sticker", message.Sticker},
	}
	for _, attachment := range attachments {
		if len(attachment.raw) == 0 || string(attachment.raw) == "null" {
			continue
		}
		fileID, fileName := telegramAttachmentMetadata(attachment.raw)
		if fileID == "" {
			continue
		}
		if err := h.createTelegramItem(ctx, target, telegramAttachmentsTable, map[string]any{
			"message_key": messageKey, "telegram_attachment_key": messageKey + ":" + fileID, "file_id": fileID, "kind": attachment.kind, "file_name": fileName, "raw_payload": string(attachment.raw),
		}); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
				continue
			}
			return err
		}
	}
	return nil
}

func (h *HandlerV1) createTelegramItem(ctx context.Context, target *telegramProjectTarget, tableSlug string, data map[string]any) error {
	_, err := h.createTelegramItemWithResponse(ctx, target, tableSlug, data)
	return err
}

func (h *HandlerV1) createTelegramItemWithResponse(ctx context.Context, target *telegramProjectTarget, tableSlug string, data map[string]any) (map[string]any, error) {
	body, err := structpb.NewStruct(data)
	if err != nil {
		return nil, err
	}
	result, err := target.Services.GoObjectBuilderService().Items().Create(ctx, &pbo.CommonMessage{TableSlug: tableSlug, ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID, CompanyProjectId: target.ProjectID, Data: body})
	if err != nil {
		return nil, err
	}
	return result.GetData().AsMap(), nil
}

func (h *HandlerV1) executeTelegramSQL(ctx context.Context, target *telegramProjectTarget, sql string, params []string) ([]*structpb.Struct, error) {
	result, err := target.Services.GoObjectBuilderService().ObjectBuilder().ExecuteSQL(ctx, &pbo.ExecuteSQLRequest{ResourceEnvId: target.ResourceEnvID, Sql: sql, Params: params})
	if err != nil {
		return nil, err
	}
	if result.GetError() != "" {
		return nil, errors.New(result.GetError())
	}
	return result.GetRows(), nil
}

func (h *HandlerV1) listTelegramChatRows(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping, search string) ([]map[string]any, error) {
	if mapping == nil || !telegramIdentifierPattern.MatchString(mapping.GetTableSlug()) {
		return nil, errors.New("telegram mapping is missing")
	}
	columns := []string{"guid"}
	for alias, field := range map[string]string{
		"telegram_chat_id": mapping.GetTelegramChatIdField(), "display_name": mapping.GetDisplayNameField(), "username": mapping.GetUsernameField(), "last_message": mapping.GetLastMessageField(), "last_message_at": mapping.GetLastMessageAtField(), "unread_count": mapping.GetUnreadCountField(), "status": mapping.GetStatusField(),
	} {
		if field != "" {
			columns = append(columns, fmt.Sprintf("%s AS %s", telegramQuoteIdentifier(field), telegramQuoteIdentifier(alias)))
		}
	}
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE deleted_at IS NULL AND %s IS NOT NULL AND COALESCE(%s::text, '') <> ''`,
		strings.Join(columns, ", "),
		telegramQuoteIdentifier(mapping.GetTableSlug()),
		telegramQuoteIdentifier(mapping.GetTelegramChatIdField()),
		telegramQuoteIdentifier(mapping.GetTelegramChatIdField()),
	)
	params := []string{}
	if strings.TrimSpace(search) != "" && mapping.GetDisplayNameField() != "" {
		params = append(params, ".*"+regexp.QuoteMeta(strings.TrimSpace(search))+".*")
		query += fmt.Sprintf(" AND %s ~* $1", telegramQuoteIdentifier(mapping.GetDisplayNameField()))
	}
	if mapping.GetLastMessageAtField() != "" {
		query += " ORDER BY " + telegramQuoteIdentifier(mapping.GetLastMessageAtField()) + " DESC NULLS LAST"
	}
	rows, err := h.executeTelegramSQL(ctx, target, query+" LIMIT 100", params)
	if err != nil {
		return nil, err
	}
	response := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		response = append(response, row.AsMap())
	}
	return response, nil
}

func (h *HandlerV1) telegramChatIDForRecord(ctx context.Context, target *telegramProjectTarget, mapping *pb.TelegramChatMapping, recordID string) (string, error) {
	if mapping == nil {
		return "", errors.New("telegram mapping is missing")
	}
	rows, err := h.executeTelegramSQL(ctx, target, fmt.Sprintf(`SELECT %s AS telegram_chat_id FROM %s WHERE guid = $1 AND deleted_at IS NULL LIMIT 1`, telegramQuoteIdentifier(mapping.GetTelegramChatIdField()), telegramQuoteIdentifier(mapping.GetTableSlug())), []string{recordID})
	if err != nil || len(rows) == 0 {
		return "", errors.New("telegram chat was not found")
	}
	chatID := fmt.Sprint(rows[0].AsMap()["telegram_chat_id"])
	if chatID == "" {
		return "", errors.New("telegram chat id is missing")
	}
	return chatID, nil
}

func (h *HandlerV1) loadTelegramSecret(resource *pb.ProjectResource) (telegramSecret, error) {
	if resource == nil {
		return telegramSecret{}, errors.New("telegram resource is missing")
	}
	if resource.GetSecret() != nil {
		secret := telegramSecret{BotToken: telegramSecretString(resource.GetSecret().AsMap(), "bot_token"), WebhookSecret: telegramSecretString(resource.GetSecret().AsMap(), "webhook_secret")}
		if secret.BotToken != "" && secret.WebhookSecret != "" {
			return secret, nil
		}
	}
	return telegramSecret{}, errors.New("telegram credentials are missing")
}

func telegramSecretString(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func telegramResourceResponse(resource *pb.ProjectResource) telegramIntegrationResponse {
	telegram := resource.GetSettings().GetTelegram()
	response := telegramIntegrationResponse{ResourceID: resource.GetId(), BotID: telegram.GetBotId(), BotUsername: telegram.GetBotUsername(), Status: telegram.GetStatus(), Mapping: telegram.GetMapping()}
	if telegram.GetBotUsername() != "" {
		response.BotURL = "https://t.me/" + strings.TrimPrefix(telegram.GetBotUsername(), "@")
	}
	return response
}

func telegramInboxPrompt(mapping *pb.TelegramChatMapping) string {
	if mapping == nil {
		return ""
	}
	return fmt.Sprintf(`SYSTEM TASK — add Telegram Inbox to the current generated admin panel.

First inspect the current project_files and preserve its existing framework, navigation, auth conventions and visual language. Do not replace the app or introduce a separate application.

Add a menu item and route named "Support → Telegram Inbox". Implement a responsive three-pane inbox:
1. chats list with search, display name, username, last message, unread count and status;
2. selected conversation with history, attachments and a text reply composer;
3. a details panel that displays mapped fields from the selected chat record.

The backend contract (all responses are wrapped in {data: ...}) is:
- GET /v1/telegram/chats?search=... -> data.chats
- GET /v1/telegram/chats/{chat_record_guid}/messages -> data.messages
- GET /v1/telegram/messages/{message_guid}/attachments -> data.attachments
- GET /v1/telegram/messages/{message_guid}/attachments/{attachment_guid} streams the attachment through the authorised gateway
- POST /v1/telegram/chats/{chat_record_guid}/messages with {"text":"..."} sends an answer.

Use the app's existing authenticated API client. Never ask for, store or render a Telegram bot token, webhook secret, raw Telegram file URL or raw_payload. Restrict the page using the app's existing owner/administrator/operator permission mechanism; a customer must not see it.

Mapped chat table: %q. Its Telegram fields are chat_id=%q, user_id=%q, display_name=%q, username=%q, last_message=%q, last_message_at=%q, unread_count=%q, status=%q. The chat record guid is the identifier passed to Inbox APIs. The message history is maintained by the backend in ugen_telegram_messages; do not create client-side message tables.

Make the change in the existing frontend files, then leave the build in a working state.`,
		mapping.GetTableSlug(),
		mapping.GetTelegramChatIdField(),
		mapping.GetTelegramUserIdField(),
		mapping.GetDisplayNameField(),
		mapping.GetUsernameField(),
		mapping.GetLastMessageField(),
		mapping.GetLastMessageAtField(),
		mapping.GetUnreadCountField(),
		mapping.GetStatusField(),
	)
}

func telegramRandomToken(bytesLen int) (string, error) {
	bytes := make([]byte, bytesLen)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func normalizeTelegramBotUsername(input, displayName string) string {
	username := strings.ToLower(strings.TrimSpace(input))
	if username == "" {
		username = strings.ToLower(displayName)
	}
	username = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' {
			return r
		}
		return -1
	}, username)
	if username == "" {
		username = "ugen_" + uuid.NewString()[:8]
	}
	if username[0] < 'a' || username[0] > 'z' {
		username = "b_" + username
	}
	if !strings.HasSuffix(username, "bot") {
		username += "_bot"
	}
	if len(username) > 32 {
		username = strings.TrimSuffix(username[:28], "_") + "_bot"
	}
	return username
}

func telegramManagedUsernameKey(username string) string {
	username = strings.TrimPrefix(strings.TrimSpace(username), "@")
	username = strings.ToLower(username)
	if username == "" {
		return ""
	}
	return username
}

func telegramQuoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func telegramPutMappedValue(data map[string]any, field string, value any) {
	if field != "" {
		data[field] = value
	}
}

func telegramDisplayName(message *telegramMessage) string {
	return strings.TrimSpace(strings.TrimSpace(message.From.FirstName) + " " + strings.TrimSpace(message.From.LastName))
}

func telegramMessageText(message *telegramMessage) string {
	if strings.TrimSpace(message.Text) != "" {
		return message.Text
	}
	return message.Caption
}

func telegramMessageType(message *telegramMessage) string {
	for _, candidate := range []struct {
		kind string
		raw  json.RawMessage
	}{{"photo", message.Photo}, {"document", message.Document}, {"video", message.Video}, {"voice", message.Voice}, {"audio", message.Audio}, {"sticker", message.Sticker}} {
		if len(candidate.raw) > 0 && string(candidate.raw) != "null" {
			return candidate.kind
		}
	}
	return "text"
}

func telegramMessageChatRelationField(mapping *pb.TelegramChatMapping) (string, error) {
	if mapping == nil || !telegramIdentifierPattern.MatchString(mapping.GetTableSlug()) {
		return "", errors.New("telegram mapping table is invalid")
	}
	field := mapping.GetTableSlug() + "_id"
	if !telegramIdentifierPattern.MatchString(field) {
		return "", errors.New("telegram message relation field is invalid")
	}
	return field, nil
}

func telegramAttachmentMetadata(raw json.RawMessage) (string, string) {
	var single struct {
		FileID   string `json:"file_id"`
		FileName string `json:"file_name"`
	}
	if json.Unmarshal(raw, &single) == nil && single.FileID != "" {
		return single.FileID, single.FileName
	}
	var photos []struct {
		FileID string `json:"file_id"`
	}
	if json.Unmarshal(raw, &photos) == nil && len(photos) > 0 {
		return photos[len(photos)-1].FileID, ""
	}
	return "", ""
}

func truncateTelegramError(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	if len(value) > 500 {
		return value[:500]
	}
	return value
}

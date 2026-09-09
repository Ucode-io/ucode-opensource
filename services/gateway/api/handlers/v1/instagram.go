package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	instagramContactsTable    = "ugen_instagram_contacts"
	instagramMessagesTable    = "ugen_instagram_messages"
	instagramAttachmentsTable = "ugen_instagram_attachments"
	instagramUpdatesTable     = "ugen_instagram_updates"
)

var (
	instagramIdentifierPattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)
	instagramHTTPClient        = &http.Client{Timeout: 20 * time.Second}
)

type instagramProjectTarget = telegramProjectTarget

type instagramChatMappingRequest struct {
	TableID              string `json:"table_id"`
	TableSlug            string `json:"table_slug"`
	InstagramUserIDField string `json:"instagram_user_id_field"`
	UsernameField        string `json:"username_field"`
	DisplayNameField     string `json:"display_name_field"`
	ProfilePictureField  string `json:"profile_picture_field"`
	LastMessageField     string `json:"last_message_field"`
	LastMessageAtField   string `json:"last_message_at_field"`
	UnreadCountField     string `json:"unread_count_field"`
	StatusField          string `json:"status_field"`
	ConversationIDField  string `json:"conversation_id_field"`
}

func (r instagramChatMappingRequest) toProto() *pb.InstagramChatMapping {
	return &pb.InstagramChatMapping{
		TableId:              strings.TrimSpace(r.TableID),
		TableSlug:            strings.TrimSpace(r.TableSlug),
		InstagramUserIdField: strings.TrimSpace(r.InstagramUserIDField),
		UsernameField:        strings.TrimSpace(r.UsernameField),
		DisplayNameField:     strings.TrimSpace(r.DisplayNameField),
		ProfilePictureField:  strings.TrimSpace(r.ProfilePictureField),
		LastMessageField:     strings.TrimSpace(r.LastMessageField),
		LastMessageAtField:   strings.TrimSpace(r.LastMessageAtField),
		UnreadCountField:     strings.TrimSpace(r.UnreadCountField),
		StatusField:          strings.TrimSpace(r.StatusField),
		ConversationIdField:  strings.TrimSpace(r.ConversationIDField),
	}
}

type instagramMappingFieldOption struct {
	ID       string `json:"id"`
	Slug     string `json:"slug"`
	Label    string `json:"label"`
	Type     string `json:"type"`
	IsUnique bool   `json:"is_unique"`
}

type instagramMappingTableOption struct {
	ID     string                        `json:"id"`
	Slug   string                        `json:"slug"`
	Label  string                        `json:"label"`
	Fields []instagramMappingFieldOption `json:"fields"`
}

type instagramOAuthState struct {
	McpProjectID   string                   `json:"mcp_project_id"`
	ProjectID      string                   `json:"project_id"`
	EnvironmentID  string                   `json:"environment_id"`
	UserID         string                   `json:"user_id,omitempty"`
	RedirectURL    string                   `json:"redirect_url,omitempty"`
	Mapping        *pb.InstagramChatMapping `json:"mapping"`
	RequestedAtUTC string                   `json:"requested_at_utc"`
}

type instagramIntegrationResponse struct {
	ResourceID        string                   `json:"resource_id"`
	IgID              string                   `json:"ig_id"`
	Username          string                   `json:"username"`
	AccountType       string                   `json:"account_type"`
	ProfilePictureURL string                   `json:"profile_picture_url,omitempty"`
	Status            string                   `json:"status"`
	ConnectedAt       string                   `json:"connected_at,omitempty"`
	Mapping           *pb.InstagramChatMapping `json:"mapping,omitempty"`
	ProfileURL        string                   `json:"profile_url,omitempty"`
}

type instagramSecret struct {
	AccessToken    string `json:"instagram_user_access_token"`
	TokenExpiresAt string `json:"token_expires_at"`
}

type instagramTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	UserID      int64  `json:"user_id"`
}

type instagramUserProfile struct {
	ID                string `json:"id"`
	UserID            string `json:"user_id"`
	Username          string `json:"username"`
	AccountType       string `json:"account_type"`
	ProfilePictureURL string `json:"profile_picture_url"`
}

type instagramAPIError struct {
	Message   string `json:"message"`
	Type      string `json:"type"`
	Code      int    `json:"code"`
	FbtraceID string `json:"fbtrace_id"`
}

func (e *instagramAPIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("instagram graph error %d (%s): %s", e.Code, e.Type, e.Message)
}

type instagramWebhookEvent struct {
	Object string                  `json:"object"`
	Entry  []instagramWebhookEntry `json:"entry"`
}

type instagramWebhookEntry struct {
	ID        string                    `json:"id"`
	Time      int64                     `json:"time"`
	Messaging []instagramMessagingEvent `json:"messaging"`
	Changes   []json.RawMessage         `json:"changes"`
}

type instagramMessagingEvent struct {
	Sender      instagramWebhookUser `json:"sender"`
	Recipient   instagramWebhookUser `json:"recipient"`
	Timestamp   int64                `json:"timestamp"`
	Message     *instagramMessage    `json:"message"`
	MessageEdit json.RawMessage      `json:"message_edit"`
	Postback    *instagramPostback   `json:"postback"`
	Reaction    *instagramReaction   `json:"reaction"`
	Read        *instagramRead       `json:"read"`
	Optin       json.RawMessage      `json:"optin"`
	Referral    json.RawMessage      `json:"referral"`
	Raw         map[string]any       `json:"-"`
}

func (e *instagramMessagingEvent) UnmarshalJSON(data []byte) error {
	type alias instagramMessagingEvent
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*e = instagramMessagingEvent(decoded)
	_ = json.Unmarshal(data, &e.Raw)
	return nil
}

type instagramWebhookUser struct {
	ID string `json:"id"`
}

type instagramMessage struct {
	Mid         string                `json:"mid"`
	Text        string                `json:"text"`
	IsEcho      bool                  `json:"is_echo"`
	Attachments []instagramAttachment `json:"attachments"`
}

type instagramAttachment struct {
	Type    string                     `json:"type"`
	Payload instagramAttachmentPayload `json:"payload"`
}

type instagramAttachmentPayload struct {
	URL       string `json:"url"`
	ID        string `json:"id"`
	StickerID string `json:"sticker_id"`
}

type instagramPostback struct {
	Mid     string          `json:"mid"`
	Title   string          `json:"title"`
	Payload string          `json:"payload"`
	Raw     json.RawMessage `json:"-"`
}

type instagramReaction struct {
	Mid      string `json:"mid"`
	Action   string `json:"action"`
	Reaction string `json:"reaction"`
}

type instagramRead struct {
	Mid       string `json:"mid"`
	Watermark int64  `json:"watermark"`
}

type instagramSendMessageResponse struct {
	RecipientID string `json:"recipient_id"`
	MessageID   string `json:"message_id"`
}

func (h *HandlerV1) GetInstagramMappingOptions(c *gin.Context) {
	target, err := h.resolveInstagramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	tables, err := target.Services.GoObjectBuilderService().Table().GetAll(c.Request.Context(), &pbo.GetAllTablesRequest{ProjectId: target.ResourceEnvID, Limit: 1000})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	options := make([]instagramMappingTableOption, 0, len(tables.GetTables()))
	for _, table := range tables.GetTables() {
		if strings.HasPrefix(table.GetSlug(), "ugen_instagram_") {
			continue
		}
		fields, fieldErr := target.Services.GoObjectBuilderService().Field().GetAll(c.Request.Context(), &pbo.GetAllFieldsRequest{TableId: table.GetId(), ProjectId: target.ResourceEnvID, Limit: 500})
		if fieldErr != nil {
			h.HandleResponse(c, status_http.GRPCError, fieldErr.Error())
			return
		}
		option := instagramMappingTableOption{ID: table.GetId(), Slug: table.GetSlug(), Label: table.GetLabel(), Fields: make([]instagramMappingFieldOption, 0, len(fields.GetFields()))}
		for _, field := range fields.GetFields() {
			option.Fields = append(option.Fields, instagramMappingFieldOption{ID: field.GetId(), Slug: field.GetSlug(), Label: field.GetLabel(), Type: field.GetType(), IsUnique: field.GetUnique()})
		}
		options = append(options, option)
	}
	h.HandleResponse(c, status_http.OK, gin.H{"tables": options})
}

func (h *HandlerV1) InstagramConnect(c *gin.Context) {
	if h.baseConf.InstagramClientID == "" || h.baseConf.InstagramClientSecret == "" || h.baseConf.InstagramRedirectURI == "" {
		h.HandleResponse(c, status_http.BadRequest, "instagram integration is not configured")
		return
	}
	if h.centralRedis == nil {
		h.HandleResponse(c, status_http.BadRequest, "central redis is not configured")
		return
	}

	target, err := h.resolveInstagramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	mapping, err := h.prepareInstagramMapping(c.Request.Context(), target, instagramMappingFromQuery(c).toProto())
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if err = h.ensureInstagramSystemTables(c.Request.Context(), target, mapping); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	stateToken, err := generateOAuthState()
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}
	state := instagramOAuthState{
		McpProjectID:   target.McpProjectID,
		ProjectID:      target.ProjectID,
		EnvironmentID:  target.EnvironmentID,
		RedirectURL:    resolveInstagramRedirectURL(c),
		Mapping:        mapping,
		RequestedAtUTC: time.Now().UTC().Format(time.RFC3339),
	}
	if value, ok := c.Get("user_id"); ok {
		state.UserID, _ = value.(string)
	}
	body, err := json.Marshal(state)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}
	if err = h.centralRedis.Set(c.Request.Context(), config.InstagramOAuthStatePrefix+stateToken, body, config.InstagramOAuthStateTTL).Err(); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	query := url.Values{
		"client_id":     {h.baseConf.InstagramClientID},
		"redirect_uri":  {h.baseConf.InstagramRedirectURI},
		"response_type": {"code"},
		"scope":         {config.InstagramOAuthScopes},
		"state":         {stateToken},
	}
	h.HandleResponse(c, status_http.OK, gin.H{"auth_url": h.baseConf.InstagramOAuthAuthorizeURL + "?" + query.Encode()})
}

func (h *HandlerV1) InstagramCallback(c *gin.Context) {
	var (
		code       = strings.TrimSpace(c.Query("code"))
		stateToken = strings.TrimSpace(c.Query("state"))
		metaErr    = strings.TrimSpace(c.Query("error"))
	)

	state, stateErr := h.getAndDeleteInstagramState(c.Request.Context(), stateToken)
	if metaErr != "" {
		detail := c.Query("error_description")
		if detail == "" {
			detail = c.Query("error_reason")
		}
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "instagram_" + metaErr, detail: detail, state: state})
		return
	}
	if stateErr != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "invalid_state", detail: stateErr.Error()})
		return
	}
	if code == "" {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "missing_code", state: state})
		return
	}

	shortToken, err := h.exchangeInstagramCode(c.Request.Context(), code)
	if err != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "code_exchange_failed", detail: err.Error(), state: state})
		return
	}
	longToken, err := h.exchangeInstagramLongLivedToken(c.Request.Context(), shortToken.AccessToken)
	if err != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "long_lived_exchange_failed", detail: err.Error(), state: state})
		return
	}
	profile, err := h.instagramFetchProfile(c.Request.Context(), longToken.AccessToken)
	if err != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "profile_fetch_failed", detail: err.Error(), state: state})
		return
	}
	igID := strings.TrimSpace(profile.UserID)
	if igID == "" {
		igID = strings.TrimSpace(profile.ID)
	}
	if igID == "" {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "missing_ig_id", detail: "instagram profile did not include user_id", state: state})
		return
	}

	target, err := h.resolveInstagramProjectTarget(c.Request.Context(), state.ProjectID, state.EnvironmentID)
	if err != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "target_resolve_failed", detail: err.Error(), state: state})
		return
	}
	if existing, existingErr := h.getInstagramResource(c.Request.Context(), target); existingErr == nil && existing.GetExternalId() != "" && existing.GetExternalId() != igID {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "instagram_already_connected", detail: "disconnect the existing Instagram account before replacing it", state: state})
		return
	}
	if err = h.instagramSubscribeAccount(c.Request.Context(), igID, longToken.AccessToken); err != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "webhook_subscribe_failed", detail: err.Error(), state: state})
		return
	}
	if _, err = h.connectInstagramAccount(c.Request.Context(), target, state.Mapping, profile, longToken, state.UserID); err != nil {
		h.redirectInstagramOAuth(c, instagramOAuthOutcome{reason: "resource_save_failed", detail: err.Error(), state: state})
		return
	}
	h.redirectInstagramOAuth(c, instagramOAuthOutcome{success: true, state: state})
}

func (h *HandlerV1) GetInstagramIntegration(c *gin.Context) {
	target, err := h.resolveInstagramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getInstagramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "instagram integration is not connected")
		return
	}
	h.HandleResponse(c, status_http.OK, instagramResourceResponse(resource))
}

func (h *HandlerV1) GetInstagramInboxPrompt(c *gin.Context) {
	target, err := h.resolveInstagramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getInstagramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "instagram integration is not connected")
		return
	}
	instagram := resource.GetSettings().GetInstagram()
	h.HandleResponse(c, status_http.OK, gin.H{
		"status": instagram.GetStatus(),
		"prompt": instagramInboxPrompt(instagram.GetMapping()),
	})
}

func (h *HandlerV1) MarkInstagramInboxReady(c *gin.Context) {
	target, err := h.resolveInstagramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getInstagramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "instagram integration is not connected")
		return
	}
	instagram := resource.GetSettings().GetInstagram()
	instagram.Status = config.InstagramStatusConnected
	if _, err = h.companyServices.Resource().UpdateProjectResource(c.Request.Context(), &pb.ProjectResource{
		Id:            resource.GetId(),
		ProjectId:     target.ProjectID,
		EnvironmentId: target.EnvironmentID,
		Name:          resource.GetName(),
		Type:          pb.ResourceType_INSTAGRAM.String(),
		ResourceType:  int32(pb.ResourceType_INSTAGRAM),
		ExternalId:    resource.GetExternalId(),
		Settings:      resource.GetSettings(),
	}); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, instagramResourceResponse(resource))
}

func (h *HandlerV1) DisconnectInstagramIntegration(c *gin.Context) {
	target, err := h.resolveInstagramMcpTarget(c.Request.Context(), c, c.Param("mcp_project_id"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getInstagramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "instagram integration is not connected")
		return
	}
	if secret, secretErr := h.loadInstagramSecret(resource); secretErr == nil && secret.AccessToken != "" && resource.GetExternalId() != "" {
		if unsubscribeErr := h.instagramUnsubscribeAccount(c.Request.Context(), resource.GetExternalId(), secret.AccessToken); unsubscribeErr != nil {
			h.log.Warn("instagram disconnect: unsubscribe failed: " + unsubscribeErr.Error())
		}
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

func (h *HandlerV1) InstagramWebhookVerify(c *gin.Context) {
	mode := c.Query("hub.mode")
	token := c.Query("hub.verify_token")
	challenge := c.Query("hub.challenge")
	if mode == "subscribe" && token != "" && token == h.baseConf.InstagramWebhookVerifyToken {
		c.String(http.StatusOK, challenge)
		return
	}
	c.String(http.StatusForbidden, "verification failed")
}

func (h *HandlerV1) InstagramWebhookReceive(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	if !h.verifyInstagramSignature(c.GetHeader(config.InstagramSignatureHeader), body) {
		h.log.Warn("instagram webhook: signature verification failed")
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	var event instagramWebhookEvent
	if err = json.Unmarshal(body, &event); err != nil {
		h.log.Error("instagram webhook: invalid payload", logger.Error(err))
		c.JSON(http.StatusOK, gin.H{"ok": true})
		return
	}
	h.log.Info("instagram webhook: accepted",
		logger.String("object", event.Object),
		logger.Int("entries", len(event.Entry)),
	)
	c.JSON(http.StatusOK, gin.H{"ok": true})
	go h.processInstagramWebhookEvent(event)
}

func (h *HandlerV1) ListInstagramChats(c *gin.Context) {
	if !h.requireInstagramInboxAccess(c) {
		return
	}
	target, err := h.resolveInstagramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getInstagramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "instagram integration is not connected")
		return
	}
	rows, err := h.listInstagramChatRows(c.Request.Context(), target, resource.GetSettings().GetInstagram().GetMapping(), c.Query("search"))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, gin.H{"chats": rows})
}

func (h *HandlerV1) ListInstagramMessages(c *gin.Context) {
	if !h.requireInstagramInboxAccess(c) {
		return
	}
	target, err := h.resolveInstagramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(c.Param("chat_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "chat_id is invalid")
		return
	}
	mapping, err := h.instagramMapping(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, err.Error())
		return
	}
	relationField, err := instagramMessageChatRelationField(mapping)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	rows, err := h.executeInstagramSQL(c.Request.Context(), target,
		`SELECT m.guid, m.`+instagramQuoteIdentifier(relationField)+` AS chat_record_id, m.instagram_message_id, m.direction, m.message_type, m.text, m.sent_at, m.delivery_status, (SELECT COUNT(*) FROM "`+instagramAttachmentsTable+`" a WHERE a.message_key = m.instagram_message_key AND a.deleted_at IS NULL) AS attachment_count FROM "`+instagramMessagesTable+`" m WHERE m.`+instagramQuoteIdentifier(relationField)+` = $1 AND m.deleted_at IS NULL ORDER BY m.sent_at ASC`,
		[]string{c.Param("chat_id")})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.OK, gin.H{"messages": rows})
}

func (h *HandlerV1) ListInstagramMessageAttachments(c *gin.Context) {
	if !h.requireInstagramInboxAccess(c) {
		return
	}
	target, err := h.resolveInstagramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(c.Param("message_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "message_id is invalid")
		return
	}
	rows, err := h.executeInstagramSQL(c.Request.Context(), target, `SELECT a.guid, a.kind, a.file_name FROM "`+instagramAttachmentsTable+`" a JOIN "`+instagramMessagesTable+`" m ON m.instagram_message_key = a.message_key WHERE m.guid = $1 AND a.deleted_at IS NULL AND m.deleted_at IS NULL ORDER BY a.created_at ASC`, []string{c.Param("message_id")})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	attachments := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		attachment := row.AsMap()
		attachment["download_url"] = fmt.Sprintf("/v1/instagram/messages/%s/attachments/%s", c.Param("message_id"), attachment["guid"])
		attachments = append(attachments, attachment)
	}
	h.HandleResponse(c, status_http.OK, gin.H{"attachments": attachments})
}

func (h *HandlerV1) ProxyInstagramAttachment(c *gin.Context) {
	if !h.requireInstagramInboxAccess(c) {
		return
	}
	target, err := h.resolveInstagramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(c.Param("message_id")) || !util.IsValidUUID(c.Param("attachment_id")) {
		h.HandleResponse(c, status_http.InvalidArgument, "message_id or attachment_id is invalid")
		return
	}
	rows, err := h.executeInstagramSQL(c.Request.Context(), target, `SELECT a.media_url, a.file_name FROM "`+instagramAttachmentsTable+`" a JOIN "`+instagramMessagesTable+`" m ON m.instagram_message_key = a.message_key WHERE m.guid = $1 AND a.guid = $2 AND a.deleted_at IS NULL AND m.deleted_at IS NULL LIMIT 1`, []string{c.Param("message_id"), c.Param("attachment_id")})
	if err != nil || len(rows) == 0 {
		h.HandleResponse(c, status_http.NotFound, "instagram attachment not found")
		return
	}
	attachment := rows[0].AsMap()
	mediaURL := strings.TrimSpace(fmt.Sprint(attachment["media_url"]))
	if mediaURL == "" {
		h.HandleResponse(c, status_http.NotFound, "instagram attachment has no media url")
		return
	}
	parsed, err := url.Parse(mediaURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		h.HandleResponse(c, status_http.BadRequest, "instagram attachment url is invalid")
		return
	}
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, mediaURL, nil)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resp, err := instagramHTTPClient.Do(req)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		h.HandleResponse(c, status_http.GRPCError, "instagram attachment is unavailable")
		return
	}
	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileName := strings.ReplaceAll(fmt.Sprint(attachment["file_name"]), `"`, "")
	if fileName != "" {
		c.Header("Content-Disposition", `inline; filename="`+fileName+`"`)
	}
	c.DataFromReader(http.StatusOK, resp.ContentLength, contentType, resp.Body, nil)
}

func (h *HandlerV1) SendInstagramMessage(c *gin.Context) {
	if !h.requireInstagramInboxAccess(c) {
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
	target, err := h.resolveInstagramCurrentTarget(c.Request.Context(), c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	resource, err := h.getInstagramResource(c.Request.Context(), target)
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, "instagram integration is not connected")
		return
	}
	mapping := resource.GetSettings().GetInstagram().GetMapping()
	recipientID, err := h.instagramUserIDForRecord(c.Request.Context(), target, mapping, c.Param("chat_id"))
	if err != nil {
		h.HandleResponse(c, status_http.NotFound, err.Error())
		return
	}
	secret, err := h.loadInstagramSecret(resource)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	message, err := h.instagramSendText(c.Request.Context(), resource.GetExternalId(), secret.AccessToken, recipientID, request.Text)
	if err != nil {
		_ = h.persistInstagramOutboundError(c.Request.Context(), target, c.Param("chat_id"), request.Text, err)
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if err = h.persistInstagramOutboundMessage(c.Request.Context(), target, c.Param("chat_id"), message.MessageID, request.Text, "sent", ""); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	h.HandleResponse(c, status_http.Created, gin.H{"message_id": message.MessageID})
}

func (h *HandlerV1) resolveInstagramMcpTarget(ctx context.Context, c *gin.Context, mcpProjectID string) (*instagramProjectTarget, error) {
	return h.resolveTelegramMcpTarget(ctx, c, mcpProjectID)
}

func (h *HandlerV1) resolveInstagramProjectTarget(ctx context.Context, projectID, environmentID string) (*instagramProjectTarget, error) {
	return h.resolveTelegramProjectTarget(ctx, projectID, environmentID)
}

func (h *HandlerV1) resolveInstagramCurrentTarget(ctx context.Context, c *gin.Context) (*instagramProjectTarget, error) {
	return h.resolveTelegramCurrentTarget(ctx, c)
}

func (h *HandlerV1) requireInstagramInboxAccess(c *gin.Context) bool {
	authHeader := strings.ToLower(strings.TrimSpace(c.GetHeader("Authorization")))
	if strings.HasPrefix(authHeader, "bearer ") {
		if _, err := h.GetAuthInfo(c); err != nil {
			return false
		}
		return true
	}
	if authHeader == "api-key" || strings.HasPrefix(authHeader, "api-key ") {
		if _, ok := c.Get("auth"); !ok {
			h.HandleResponse(c, status_http.Forbidden, "instagram inbox requires an authenticated api key")
			return false
		}
		if _, ok := c.Get("project_id"); !ok {
			h.HandleResponse(c, status_http.Forbidden, "instagram inbox requires project context")
			return false
		}
		if _, ok := c.Get("environment_id"); !ok {
			h.HandleResponse(c, status_http.Forbidden, "instagram inbox requires environment context")
			return false
		}
		return true
	}
	h.HandleResponse(c, status_http.Forbidden, "instagram inbox requires an authenticated user session or api key")
	return false
}

func (h *HandlerV1) getInstagramResource(ctx context.Context, target *instagramProjectTarget) (*pb.ProjectResource, error) {
	resources, err := h.companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId: target.ProjectID, EnvironmentId: target.EnvironmentID, Type: pb.ResourceType_INSTAGRAM,
	})
	if err != nil {
		return nil, err
	}
	for _, resource := range resources.GetResources() {
		if resource.GetSettings() != nil && resource.GetSettings().GetInstagram() != nil {
			return resource, nil
		}
	}
	return nil, errors.New("instagram resource not found")
}

func (h *HandlerV1) instagramMapping(ctx context.Context, target *instagramProjectTarget) (*pb.InstagramChatMapping, error) {
	resource, err := h.getInstagramResource(ctx, target)
	if err != nil {
		return nil, err
	}
	mapping := resource.GetSettings().GetInstagram().GetMapping()
	if mapping == nil {
		return nil, errors.New("instagram mapping is missing")
	}
	if err = h.applyInstagramDefaultMappingFields(ctx, target, mapping); err != nil {
		return nil, err
	}
	return mapping, nil
}

func (h *HandlerV1) prepareInstagramMapping(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping) (*pb.InstagramChatMapping, error) {
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
	if strings.HasPrefix(table.GetSlug(), "ugen_instagram_") {
		return nil, errors.New("an Instagram system table cannot be used as the mapped chat table")
	}
	if mapping.GetTableSlug() != "" && mapping.GetTableSlug() != table.GetSlug() {
		return nil, errors.New("mapping.table_slug does not match table_id")
	}
	mapping.TableSlug = table.GetSlug()
	if mapping.GetInstagramUserIdField() == "" {
		mapping.InstagramUserIdField = "instagram_user_id"
	}

	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{TableId: table.GetId(), ProjectId: target.ResourceEnvID, Limit: 500})
	if err != nil {
		return nil, fmt.Errorf("get mapped chat table fields: %w", err)
	}
	known := make(map[string]*pbo.Field, len(fields.GetFields()))
	for _, field := range fields.GetFields() {
		known[field.GetSlug()] = field
	}
	applyInstagramDefaultMappingFieldsFromKnown(mapping, known)

	if !instagramIdentifierPattern.MatchString(mapping.GetInstagramUserIdField()) {
		return nil, fmt.Errorf("invalid mapped field %q", mapping.GetInstagramUserIdField())
	}
	if field := known[mapping.GetInstagramUserIdField()]; field != nil && !field.GetUnique() {
		return nil, fmt.Errorf("mapped instagram_user_id field %q must be unique", mapping.GetInstagramUserIdField())
	}
	if known[mapping.GetInstagramUserIdField()] == nil {
		attrs, attrErr := helperFunc.ConvertMapToStruct(map[string]any{"label_en": "Instagram user ID", "instagram_system": true})
		if attrErr != nil {
			return nil, attrErr
		}
		if _, err = target.Services.GoObjectBuilderService().Field().Create(ctx, &pbo.CreateFieldRequest{
			Id: uuid.NewString(), TableId: table.GetId(), ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			Slug: mapping.GetInstagramUserIdField(), Label: "Instagram user ID", Type: "SINGLE_LINE", Unique: true,
			IsVisible: false, Attributes: attrs,
		}); err != nil {
			return nil, fmt.Errorf("create instagram mapping field %s: %w", mapping.GetInstagramUserIdField(), err)
		}
		known[mapping.GetInstagramUserIdField()] = &pbo.Field{Slug: mapping.GetInstagramUserIdField(), Type: "SINGLE_LINE", Unique: true}
	}
	if err = h.ensureInstagramMappedOptionalFields(ctx, target, table.GetId(), known, mapping); err != nil {
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

func (h *HandlerV1) ensureInstagramMappedOptionalFields(ctx context.Context, target *instagramProjectTarget, tableID string, known map[string]*pbo.Field, mapping *pb.InstagramChatMapping) error {
	definitions := map[string]telegramSystemField{
		mapping.GetUsernameField():       {"instagram_username", "Instagram username", "SINGLE_LINE", false},
		mapping.GetDisplayNameField():    {"customer_name", "Customer name", "SINGLE_LINE", false},
		mapping.GetProfilePictureField(): {"profile_picture_url", "Profile picture URL", "SINGLE_LINE", false},
		mapping.GetLastMessageField():    {"last_message", "Last message", "MULTI_LINE", false},
		mapping.GetLastMessageAtField():  {"last_message_at", "Last message at", "DATE_TIME", false},
		mapping.GetUnreadCountField():    {"unread_count", "Unread count", "NUMBER", false},
		mapping.GetStatusField():         {"status", "Status", "SINGLE_LINE", false},
		mapping.GetConversationIdField(): {"instagram_conversation_id", "Instagram conversation ID", "SINGLE_LINE", false},
	}
	for _, field := range []string{
		mapping.GetUsernameField(),
		mapping.GetDisplayNameField(),
		mapping.GetProfilePictureField(),
		mapping.GetLastMessageField(),
		mapping.GetLastMessageAtField(),
		mapping.GetUnreadCountField(),
		mapping.GetStatusField(),
		mapping.GetConversationIdField(),
	} {
		if field == "" {
			continue
		}
		if !instagramIdentifierPattern.MatchString(field) {
			return fmt.Errorf("invalid mapped optional field %q", field)
		}
		if known[field] != nil {
			continue
		}
		definition, ok := definitions[field]
		if !ok || definition.slug == "" {
			return fmt.Errorf("mapped optional field %q does not exist in table", field)
		}
		attrs, attrErr := helperFunc.ConvertMapToStruct(map[string]any{"label_en": definition.label, "instagram_system": true})
		if attrErr != nil {
			return attrErr
		}
		if _, err := target.Services.GoObjectBuilderService().Field().Create(ctx, &pbo.CreateFieldRequest{
			Id: uuid.NewString(), TableId: tableID, ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			Slug: field, Label: definition.label, Type: definition.typeID, Unique: definition.unique,
			IsVisible: false, Attributes: attrs,
		}); err != nil {
			return fmt.Errorf("create instagram mapping field %s: %w", field, err)
		}
		known[field] = &pbo.Field{Slug: field, Type: definition.typeID, Unique: definition.unique}
	}
	return nil
}

func (h *HandlerV1) applyInstagramDefaultMappingFields(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping) error {
	if mapping == nil || !util.IsValidUUID(mapping.GetTableId()) {
		return nil
	}
	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{TableId: mapping.GetTableId(), ProjectId: target.ResourceEnvID, Limit: 500})
	if err != nil {
		return fmt.Errorf("get instagram mapped table fields: %w", err)
	}
	known := make(map[string]*pbo.Field, len(fields.GetFields()))
	for _, field := range fields.GetFields() {
		known[field.GetSlug()] = field
	}
	applyInstagramDefaultMappingFieldsFromKnown(mapping, known)
	return nil
}

func applyInstagramDefaultMappingFieldsFromKnown(mapping *pb.InstagramChatMapping, known map[string]*pbo.Field) {
	if mapping == nil || len(known) == 0 {
		return
	}
	if mapping.UsernameField == "" {
		mapping.UsernameField = telegramFirstExistingField(known, "", "instagram_username", "username")
	}
	if mapping.DisplayNameField == "" {
		mapping.DisplayNameField = telegramFirstExistingField(known, "", "customer_name", "display_name", "name")
	}
	if mapping.ProfilePictureField == "" {
		mapping.ProfilePictureField = telegramFirstExistingField(known, "", "profile_picture_url", "avatar_url", "photo_url")
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
	if mapping.ConversationIdField == "" {
		mapping.ConversationIdField = telegramFirstExistingField(known, "", "instagram_conversation_id", "conversation_id")
	}
}

func (h *HandlerV1) ensureInstagramSystemTables(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping) error {
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
		{instagramContactsTable, "Ugen Instagram Contacts", instagramSystemFields([]telegramSystemField{
			{"instagram_user_id", "Instagram user ID", "SINGLE_LINE", true}, {"username", "Username", "SINGLE_LINE", false}, {"display_name", "Display name", "SINGLE_LINE", false}, {"profile_picture_url", "Profile picture URL", "SINGLE_LINE", false}, {"raw_payload", "Raw payload", "MULTI_LINE", false},
		})},
		{instagramMessagesTable, "Ugen Instagram Messages", instagramSystemFields([]telegramSystemField{
			{"instagram_message_key", "Instagram message key", "SINGLE_LINE", true}, {"instagram_message_id", "Instagram message ID", "SINGLE_LINE", false}, {"direction", "Direction", "SINGLE_LINE", false}, {"message_type", "Message type", "SINGLE_LINE", false}, {"text", "Text", "MULTI_LINE", false}, {"raw_payload", "Raw payload", "MULTI_LINE", false}, {"sent_at", "Sent at", "DATE_TIME", false}, {"delivery_status", "Delivery status", "SINGLE_LINE", false},
		})},
		{instagramAttachmentsTable, "Ugen Instagram Attachments", instagramSystemFields([]telegramSystemField{
			{"message_key", "Message key", "SINGLE_LINE", false}, {"instagram_attachment_key", "Instagram attachment key", "SINGLE_LINE", true}, {"media_url", "Media URL", "SINGLE_LINE", false}, {"media_id", "Media ID", "SINGLE_LINE", false}, {"kind", "Attachment type", "SINGLE_LINE", false}, {"file_name", "File name", "SINGLE_LINE", false}, {"raw_payload", "Raw payload", "MULTI_LINE", false},
		})},
		{instagramUpdatesTable, "Ugen Instagram Updates", instagramSystemFields([]telegramSystemField{
			{"event_id", "Instagram event ID", "SINGLE_LINE", true}, {"event_status", "Event status", "SINGLE_LINE", false},
		})},
	}
	for _, definition := range definitions {
		if table := existing[definition.slug]; table != nil {
			if err = h.ensureInstagramSystemFields(ctx, target, table.GetId(), definition.fields); err != nil {
				return fmt.Errorf("ensure instagram system table %s fields: %w", definition.slug, err)
			}
			continue
		}
		attrs, attrErr := helperFunc.ConvertMapToStruct(map[string]any{"instagram_system": true, "label_en": definition.label})
		if attrErr != nil {
			return attrErr
		}
		if _, err = target.Services.GoObjectBuilderService().Table().Create(ctx, &pbo.CreateTableRequest{
			Id: uuid.NewString(), Label: definition.label, Slug: definition.slug, Fields: definition.fields,
			ShowInMenu: false, ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID,
			UcodeProjectId: target.ProjectID, Attributes: attrs, SoftDelete: true,
		}); err != nil {
			if isDuplicateError(err) {
				continue
			}
			return fmt.Errorf("create instagram system table %s: %w", definition.slug, err)
		}
	}
	return h.ensureInstagramMessageRelation(ctx, target, mapping)
}

func (h *HandlerV1) ensureInstagramSystemFields(ctx context.Context, target *instagramProjectTarget, tableID string, definitions []*pbo.CreateFieldsRequest) error {
	if !util.IsValidUUID(tableID) {
		return errors.New("instagram system table id is invalid")
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
			if isDuplicateError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (h *HandlerV1) ensureInstagramMessageRelation(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping) error {
	relationField, err := instagramMessageChatRelationField(mapping)
	if err != nil {
		return err
	}
	relations, err := target.Services.GoObjectBuilderService().Relation().GetAll(ctx, &pbo.GetAllRelationsRequest{
		TableSlug: instagramMessagesTable, ProjectId: target.ResourceEnvID, Limit: 500,
	})
	if err != nil {
		return fmt.Errorf("get instagram message relations: %w", err)
	}
	for _, relation := range relations.GetRelations() {
		if relation.GetTableFrom().GetSlug() == instagramMessagesTable && relation.GetTableTo().GetSlug() == mapping.GetTableSlug() && relation.GetRelationFieldSlug() == relationField {
			return nil
		}
	}
	viewFieldID, err := h.instagramMessageRelationViewFieldID(ctx, target)
	if err != nil {
		return fmt.Errorf("get instagram message field for relation: %w", err)
	}
	attributes, err := helperFunc.ConvertMapToStruct(map[string]any{
		"label_en": "Instagram chat", "label_to_en": "Instagram messages", "instagram_system": true,
	})
	if err != nil {
		return err
	}
	if _, err = target.Services.GoObjectBuilderService().Relation().Create(ctx, &pbo.CreateRelationRequest{
		Id: uuid.NewString(), TableFrom: instagramMessagesTable, TableTo: mapping.GetTableSlug(), Type: "Many2One",
		RelationTableSlug: mapping.GetTableSlug(), RelationFieldSlug: relationField,
		RelationFieldId: uuid.NewString(), RelationToFieldId: uuid.NewString(),
		ProjectId: target.ResourceEnvID, EnvId: target.EnvironmentID, ViewFields: []string{viewFieldID}, Attributes: attributes,
	}); err != nil {
		return fmt.Errorf("create instagram message relation: %w", err)
	}
	return nil
}

func (h *HandlerV1) instagramMessageRelationViewFieldID(ctx context.Context, target *instagramProjectTarget) (string, error) {
	fields, err := target.Services.GoObjectBuilderService().Field().GetAll(ctx, &pbo.GetAllFieldsRequest{
		TableSlug: instagramMessagesTable,
		ProjectId: target.ResourceEnvID,
		Limit:     500,
	})
	if err != nil {
		return "", err
	}
	if len(fields.GetFields()) == 0 {
		return "", errors.New("instagram message table has no fields")
	}
	preferredSlugs := map[string]struct{}{"text": {}, "instagram_message_id": {}}
	for _, slug := range []string{"text", "instagram_message_id"} {
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
	return "", errors.New("instagram message table fields have no ids")
}

func (h *HandlerV1) connectInstagramAccount(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, profile instagramUserProfile, token instagramTokenResponse, connectedUserID string) (*pb.ProjectResource, error) {
	igID := strings.TrimSpace(profile.UserID)
	if igID == "" {
		igID = strings.TrimSpace(profile.ID)
	}
	if igID == "" || strings.TrimSpace(token.AccessToken) == "" {
		return nil, errors.New("instagram account id and access token are required")
	}
	expiresAt := ""
	if token.ExpiresIn > 0 {
		expiresAt = time.Now().UTC().Add(time.Duration(token.ExpiresIn) * time.Second).Format(time.RFC3339)
	}
	secret, err := structpb.NewStruct(map[string]any{"instagram_user_access_token": strings.TrimSpace(token.AccessToken), "token_expires_at": expiresAt})
	if err != nil {
		return nil, fmt.Errorf("prepare instagram credentials: %w", err)
	}
	settings := &pb.Settings{Instagram: &pb.InstagramCredentials{
		IgId:              igID,
		Username:          profile.Username,
		AccountType:       profile.AccountType,
		ProfilePictureUrl: profile.ProfilePictureURL,
		Status:            config.InstagramStatusPendingUI,
		ConnectedUserId:   strings.TrimSpace(connectedUserID),
		ConnectedAt:       time.Now().UTC().Format(time.RFC3339),
		Mapping:           mapping,
	}}
	if mapping != nil {
		settings.Instagram.Mapping = mapping
	}
	resource, err := h.companyServices.Resource().UpsertProjectResource(ctx, &pb.AddResourceToProjectRequest{
		Name:          config.InstagramIntegrationName,
		ProjectId:     target.ProjectID,
		EnvironmentId: target.EnvironmentID,
		Type:          pb.ResourceType_INSTAGRAM,
		ExternalId:    igID,
		Settings:      settings,
		Secret:        secret,
	})
	if err != nil {
		return nil, fmt.Errorf("save instagram resource: %w", err)
	}
	resource.Settings = settings
	resource.Secret = nil
	return resource, nil
}

func (h *HandlerV1) processInstagramWebhookEvent(event instagramWebhookEvent) {
	defer func() {
		if r := recover(); r != nil {
			h.log.Error("instagram webhook: recovered from panic", logger.Any("panic", r))
		}
	}()
	ctx := context.Background()
	for _, entry := range event.Entry {
		h.log.Info("instagram webhook: processing entry",
			logger.String("entry_id", entry.ID),
			logger.Int("messaging_count", len(entry.Messaging)),
			logger.Int("changes_count", len(entry.Changes)),
		)
		resources, resolvedIgID, err := h.instagramResourcesForWebhookEntry(ctx, entry)
		if err != nil {
			h.log.Error("instagram webhook: resolve resource failed", logger.Error(err), logger.String("ig_id", resolvedIgID))
			continue
		}
		if len(resources.GetResources()) == 0 {
			h.log.Warn("instagram webhook: no project mapped for account", logger.String("ig_id", resolvedIgID))
			continue
		}
		for _, resource := range resources.GetResources() {
			target, err := h.resolveInstagramProjectTarget(ctx, resource.GetProjectId(), resource.GetEnvironmentId())
			if err != nil {
				h.log.Error("instagram webhook: resolve target failed", logger.Error(err), logger.String("project_id", resource.GetProjectId()))
				continue
			}
			if resource.GetSettings() == nil || resource.GetSettings().GetInstagram() == nil || resource.GetSettings().GetInstagram().GetMapping() == nil {
				h.log.Error("instagram webhook: resource mapping missing",
					logger.String("resource_id", resource.GetId()),
					logger.String("project_id", resource.GetProjectId()),
				)
				continue
			}
			mapping := resource.GetSettings().GetInstagram().GetMapping()
			for _, messageEvent := range entry.Messaging {
				if len(messageEvent.MessageEdit) > 0 {
					raw, _ := json.Marshal(messageEvent)
					updated, updateErr := h.persistInstagramMessageEdit(ctx, target, messageEvent, string(raw))
					if updateErr != nil {
						h.log.Error("instagram webhook: persist message edit failed",
							logger.Error(updateErr),
							logger.String("project_id", resource.GetProjectId()),
						)
						continue
					}
					if !updated {
						if instagramSenderID(messageEvent) == "" {
							h.log.Info("instagram webhook: message edit skipped, original message not found",
								logger.String("ig_id", resolvedIgID),
								logger.String("message_id", instagramMessageEditID(messageEvent)),
								logger.String("project_id", resource.GetProjectId()),
								logger.Any("message_edit_keys", instagramRawKeys(instagramMessageEditMap(messageEvent))),
							)
							continue
						}
						if err = h.persistInstagramIncomingEvent(ctx, target, mapping, messageEvent, string(raw)); err != nil {
							h.log.Error("instagram webhook: persist message edit as message failed",
								logger.Error(err),
								logger.String("project_id", resource.GetProjectId()),
								logger.Any("message_edit_keys", instagramRawKeys(instagramMessageEditMap(messageEvent))),
							)
							continue
						}
						h.log.Info("instagram webhook: message edit persisted as message",
							logger.String("ig_id", resolvedIgID),
							logger.String("message_id", instagramMessageEditID(messageEvent)),
							logger.String("sender_id", instagramSenderID(messageEvent)),
							logger.String("project_id", resource.GetProjectId()),
						)
						continue
					}
					h.log.Info("instagram webhook: message edit persisted",
						logger.String("ig_id", resolvedIgID),
						logger.String("message_id", instagramMessageEditID(messageEvent)),
						logger.String("project_id", resource.GetProjectId()),
					)
					continue
				}
				if messageEvent.Message != nil && messageEvent.Message.IsEcho {
					h.log.Info("instagram webhook: echo ignored",
						logger.String("ig_id", resolvedIgID),
						logger.String("message_id", messageEvent.Message.Mid),
					)
					continue
				}
				if !instagramEventIsPersistable(messageEvent) {
					h.log.Info("instagram webhook: event ignored",
						logger.String("ig_id", resolvedIgID),
						logger.String("event_type", instagramEventType(messageEvent)),
						logger.String("sender_id", instagramSenderID(messageEvent)),
						logger.String("recipient_id", messageEvent.Recipient.ID),
						logger.Any("event_keys", instagramRawKeys(messageEvent.Raw)),
					)
					continue
				}
				raw, _ := json.Marshal(messageEvent)
				if err = h.persistInstagramIncomingEvent(ctx, target, mapping, messageEvent, string(raw)); err != nil {
					h.log.Error("instagram webhook: persist event failed", logger.Error(err), logger.String("project_id", resource.GetProjectId()))
					continue
				}
				h.log.Info("instagram webhook: event persisted",
					logger.String("ig_id", resolvedIgID),
					logger.String("event_id", instagramEventKey(messageEvent)),
					logger.String("event_type", instagramEventType(messageEvent)),
					logger.String("sender_id", instagramSenderID(messageEvent)),
					logger.String("project_id", resource.GetProjectId()),
				)
			}
		}
	}
}

func (h *HandlerV1) instagramResourcesForWebhookEntry(ctx context.Context, entry instagramWebhookEntry) (*pb.ListProjectResource, string, error) {
	candidates := make([]string, 0, 1+len(entry.Messaging))
	addCandidate := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		for _, existing := range candidates {
			if existing == value {
				return
			}
		}
		candidates = append(candidates, value)
	}
	addCandidate(entry.ID)
	for _, event := range entry.Messaging {
		addCandidate(event.Recipient.ID)
		addCandidate(instagramRecipientID(event))
	}
	for _, candidate := range candidates {
		resources, err := h.companyServices.Resource().GetProjectResourcesByExternalId(ctx, &pb.GetByExternalIdRequest{
			ExternalId: candidate,
			Type:       pb.ResourceType_INSTAGRAM,
		})
		if err != nil {
			return nil, candidate, err
		}
		if len(resources.GetResources()) > 0 {
			return resources, candidate, nil
		}
	}
	return &pb.ListProjectResource{}, strings.Join(candidates, ","), nil
}

func (h *HandlerV1) persistInstagramIncomingEvent(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, event instagramMessagingEvent, raw string) error {
	if mapping == nil {
		return errors.New("instagram mapping is missing")
	}
	eventID := instagramEventKey(event)
	if eventID == "" {
		return nil
	}
	processed, err := h.executeInstagramSQL(ctx, target, `SELECT guid, event_status FROM "`+instagramUpdatesTable+`" WHERE event_id = $1 LIMIT 1`, []string{eventID})
	if err != nil {
		return err
	}
	if len(processed) > 0 {
		status := fmt.Sprint(processed[0].AsMap()["event_status"])
		if status == "completed" {
			return nil
		}
		return errors.New("instagram event is already being processed")
	}
	if err = h.createInstagramItem(ctx, target, instagramUpdatesTable, map[string]any{"event_id": eventID, "event_status": "processing"}); err != nil {
		if isDuplicateError(err) {
			return errors.New("instagram event is already being processed")
		}
		return err
	}
	completed := false
	defer func() {
		if !completed {
			_, _ = h.executeInstagramSQL(context.Background(), target, `DELETE FROM "`+instagramUpdatesTable+`" WHERE event_id = $1`, []string{eventID})
		}
	}()
	if err = h.ensureInstagramContact(ctx, target, event, raw); err != nil {
		return err
	}
	chatRecordID, err := h.upsertInstagramChat(ctx, target, mapping, event)
	if err != nil {
		return err
	}
	if err = h.persistInstagramMessage(ctx, target, mapping, chatRecordID, event, "inbound", "received", raw); err != nil {
		return err
	}
	if _, err = h.executeInstagramSQL(ctx, target, `UPDATE "`+instagramUpdatesTable+`" SET event_status = $1 WHERE event_id = $2`, []string{"completed", eventID}); err != nil {
		return err
	}
	completed = true
	return nil
}

func (h *HandlerV1) ensureInstagramContact(ctx context.Context, target *instagramProjectTarget, event instagramMessagingEvent, raw string) error {
	userID := instagramSenderID(event)
	if userID == "" {
		return nil
	}
	rows, err := h.executeInstagramSQL(ctx, target, `SELECT guid FROM "`+instagramContactsTable+`" WHERE instagram_user_id = $1 LIMIT 1`, []string{userID})
	if err != nil || len(rows) > 0 {
		return err
	}
	return h.createInstagramItem(ctx, target, instagramContactsTable, map[string]any{
		"instagram_user_id": userID,
		"display_name":      instagramDisplayName(event),
		"raw_payload":       raw,
	})
}

func (h *HandlerV1) upsertInstagramChat(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, event instagramMessagingEvent) (string, error) {
	if !instagramIdentifierPattern.MatchString(mapping.GetTableSlug()) || !instagramIdentifierPattern.MatchString(mapping.GetInstagramUserIdField()) {
		return "", errors.New("instagram mapping contains invalid table or user id field")
	}
	userID := instagramSenderID(event)
	if userID == "" {
		return "", errors.New("instagram sender id is missing")
	}
	query := fmt.Sprintf(`SELECT guid FROM %s WHERE %s = $1 AND deleted_at IS NULL LIMIT 1`, instagramQuoteIdentifier(mapping.GetTableSlug()), instagramQuoteIdentifier(mapping.GetInstagramUserIdField()))
	rows, err := h.executeInstagramSQL(ctx, target, query, []string{userID})
	if err != nil {
		return "", err
	}
	if len(rows) > 0 {
		guid := fmt.Sprint(rows[0].AsMap()["guid"])
		messageKey := fmt.Sprintf("%s:%s", guid, instagramMessageID(event))
		existingMessage, queryErr := h.executeInstagramSQL(ctx, target, `SELECT guid FROM "`+instagramMessagesTable+`" WHERE instagram_message_key = $1 AND deleted_at IS NULL LIMIT 1`, []string{messageKey})
		if queryErr != nil {
			return "", queryErr
		}
		if len(existingMessage) == 0 {
			if err = h.updateInstagramChatSummary(ctx, target, mapping, guid, event); err != nil {
				return "", err
			}
		}
		return guid, nil
	}
	data := map[string]any{mapping.GetInstagramUserIdField(): userID}
	instagramPutMappedValue(data, mapping.GetUsernameField(), "")
	instagramPutMappedValue(data, mapping.GetDisplayNameField(), instagramDisplayName(event))
	instagramPutMappedValue(data, mapping.GetLastMessageField(), instagramMessageText(event))
	instagramPutMappedValue(data, mapping.GetLastMessageAtField(), instagramEventTime(event).UTC().Format(time.RFC3339))
	instagramPutMappedValue(data, mapping.GetUnreadCountField(), 1)
	instagramPutMappedValue(data, mapping.GetConversationIdField(), userID)
	if mapping.GetStatusField() != "" {
		instagramPutMappedValue(data, mapping.GetStatusField(), "open")
	}
	result, err := h.createInstagramItemWithResponse(ctx, target, mapping.GetTableSlug(), data)
	if err != nil {
		return "", err
	}
	return fmt.Sprint(result["guid"]), nil
}

func (h *HandlerV1) updateInstagramChatSummary(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, guid string, event instagramMessagingEvent) error {
	setClauses := make([]string, 0, 7)
	params := make([]string, 0, 8)
	if field := mapping.GetDisplayNameField(); field != "" {
		setClauses = append(setClauses, instagramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, instagramDisplayName(event))
	}
	if field := mapping.GetLastMessageField(); field != "" {
		setClauses = append(setClauses, instagramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, instagramMessageText(event))
	}
	if field := mapping.GetLastMessageAtField(); field != "" {
		setClauses = append(setClauses, instagramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, instagramEventTime(event).UTC().Format(time.RFC3339))
	}
	if field := mapping.GetUnreadCountField(); field != "" {
		setClauses = append(setClauses, instagramQuoteIdentifier(field)+" = COALESCE("+instagramQuoteIdentifier(field)+", 0) + 1")
	}
	if field := mapping.GetConversationIdField(); field != "" {
		setClauses = append(setClauses, instagramQuoteIdentifier(field)+fmt.Sprintf(" = $%d", len(params)+1))
		params = append(params, instagramSenderID(event))
	}
	if field := mapping.GetStatusField(); field != "" {
		setClauses = append(setClauses, instagramQuoteIdentifier(field)+fmt.Sprintf(" = COALESCE(NULLIF(%s::text, ''), $%d)", instagramQuoteIdentifier(field), len(params)+1))
		params = append(params, "open")
	}
	if len(setClauses) == 0 {
		return nil
	}
	params = append(params, guid)
	_, err := h.executeInstagramSQL(ctx, target, fmt.Sprintf(`UPDATE %s SET %s WHERE guid = $%d`, instagramQuoteIdentifier(mapping.GetTableSlug()), strings.Join(setClauses, ", "), len(params)), params)
	return err
}

func (h *HandlerV1) persistInstagramMessage(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, chatRecordID string, event instagramMessagingEvent, direction, deliveryStatus, raw string) error {
	if raw == "" {
		encoded, err := json.Marshal(event)
		if err != nil {
			return err
		}
		raw = string(encoded)
	}
	messageID := instagramMessageID(event)
	if messageID == "" {
		messageID = instagramEventKey(event)
	}
	messageKey := fmt.Sprintf("%s:%s", chatRecordID, messageID)
	relationField, err := instagramMessageChatRelationField(mapping)
	if err != nil {
		return err
	}
	data := map[string]any{
		relationField:           chatRecordID,
		"instagram_message_key": messageKey,
		"instagram_message_id":  messageID,
		"direction":             direction,
		"message_type":          instagramMessageType(event),
		"text":                  instagramMessageText(event),
		"raw_payload":           raw,
		"sent_at":               instagramEventTime(event).UTC().Format(time.RFC3339),
		"delivery_status":       deliveryStatus,
	}
	if err := h.createInstagramItem(ctx, target, instagramMessagesTable, data); err != nil {
		if isDuplicateError(err) {
			return h.persistInstagramAttachments(ctx, target, messageKey, event)
		}
		return err
	}
	return h.persistInstagramAttachments(ctx, target, messageKey, event)
}

func (h *HandlerV1) persistInstagramAttachments(ctx context.Context, target *instagramProjectTarget, messageKey string, event instagramMessagingEvent) error {
	if event.Message == nil {
		return nil
	}
	for idx, attachment := range event.Message.Attachments {
		mediaURL := strings.TrimSpace(attachment.Payload.URL)
		mediaID := strings.TrimSpace(attachment.Payload.ID)
		if mediaID == "" {
			mediaID = strings.TrimSpace(attachment.Payload.StickerID)
		}
		if mediaURL == "" && mediaID == "" {
			continue
		}
		raw, _ := json.Marshal(attachment)
		attachmentKey := fmt.Sprintf("%s:%d:%s:%s", messageKey, idx, attachment.Type, mediaID)
		if err := h.createInstagramItem(ctx, target, instagramAttachmentsTable, map[string]any{
			"message_key":              messageKey,
			"instagram_attachment_key": attachmentKey,
			"media_url":                mediaURL,
			"media_id":                 mediaID,
			"kind":                     attachment.Type,
			"file_name":                instagramAttachmentFileName(mediaURL),
			"raw_payload":              string(raw),
		}); err != nil {
			if isDuplicateError(err) {
				continue
			}
			return err
		}
	}
	return nil
}

func (h *HandlerV1) persistInstagramMessageEdit(ctx context.Context, target *instagramProjectTarget, event instagramMessagingEvent, raw string) (bool, error) {
	messageID := instagramMessageEditID(event)
	if strings.TrimSpace(messageID) == "" {
		return false, errors.New("instagram message edit id is missing")
	}
	text := instagramMessageEditText(event)
	setClauses := []string{"raw_payload = $1"}
	params := []string{raw}
	if strings.TrimSpace(text) != "" {
		setClauses = append(setClauses, fmt.Sprintf("text = $%d", len(params)+1))
		params = append(params, text)
	}
	params = append(params, messageID)
	rows, err := h.executeInstagramSQL(ctx, target,
		fmt.Sprintf(`UPDATE "%s" SET %s WHERE instagram_message_id = $%d AND deleted_at IS NULL RETURNING guid`, instagramMessagesTable, strings.Join(setClauses, ", "), len(params)),
		params,
	)
	if err != nil {
		return false, err
	}
	return len(rows) > 0, nil
}

func (h *HandlerV1) persistInstagramOutboundMessage(ctx context.Context, target *instagramProjectTarget, chatRecordID, messageID, text, deliveryStatus, raw string) error {
	mapping, err := h.instagramMapping(ctx, target)
	if err != nil {
		return err
	}
	event := instagramMessagingEvent{
		Sender:    instagramWebhookUser{ID: "operator"},
		Timestamp: time.Now().UnixMilli(),
		Message:   &instagramMessage{Mid: messageID, Text: text},
	}
	return h.persistInstagramMessage(ctx, target, mapping, chatRecordID, event, "outbound", deliveryStatus, raw)
}

func (h *HandlerV1) persistInstagramOutboundError(ctx context.Context, target *instagramProjectTarget, chatRecordID, text string, sendErr error) error {
	return h.persistInstagramOutboundMessage(ctx, target, chatRecordID, "error:"+uuid.NewString(), text, "error", truncateInstagramError(sendErr))
}

func (h *HandlerV1) createInstagramItem(ctx context.Context, target *instagramProjectTarget, tableSlug string, data map[string]any) error {
	_, err := h.createInstagramItemWithResponse(ctx, target, tableSlug, data)
	return err
}

func (h *HandlerV1) createInstagramItemWithResponse(ctx context.Context, target *instagramProjectTarget, tableSlug string, data map[string]any) (map[string]any, error) {
	return h.createTelegramItemWithResponse(ctx, target, tableSlug, data)
}

func (h *HandlerV1) executeInstagramSQL(ctx context.Context, target *instagramProjectTarget, sql string, params []string) ([]*structpb.Struct, error) {
	return h.executeTelegramSQL(ctx, target, sql, params)
}

func (h *HandlerV1) listInstagramChatRows(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, search string) ([]map[string]any, error) {
	if mapping == nil || !instagramIdentifierPattern.MatchString(mapping.GetTableSlug()) {
		return nil, errors.New("instagram mapping is missing")
	}
	columns := []string{"guid"}
	for alias, field := range map[string]string{
		"instagram_user_id": mapping.GetInstagramUserIdField(), "display_name": mapping.GetDisplayNameField(), "username": mapping.GetUsernameField(), "profile_picture_url": mapping.GetProfilePictureField(), "last_message": mapping.GetLastMessageField(), "last_message_at": mapping.GetLastMessageAtField(), "unread_count": mapping.GetUnreadCountField(), "status": mapping.GetStatusField(), "conversation_id": mapping.GetConversationIdField(),
	} {
		if field != "" {
			columns = append(columns, fmt.Sprintf("%s AS %s", instagramQuoteIdentifier(field), instagramQuoteIdentifier(alias)))
		}
	}
	query := fmt.Sprintf(`SELECT %s FROM %s WHERE deleted_at IS NULL AND %s IS NOT NULL AND COALESCE(%s::text, '') <> ''`,
		strings.Join(columns, ", "),
		instagramQuoteIdentifier(mapping.GetTableSlug()),
		instagramQuoteIdentifier(mapping.GetInstagramUserIdField()),
		instagramQuoteIdentifier(mapping.GetInstagramUserIdField()),
	)
	params := []string{}
	if strings.TrimSpace(search) != "" && mapping.GetDisplayNameField() != "" {
		params = append(params, ".*"+regexp.QuoteMeta(strings.TrimSpace(search))+".*")
		query += fmt.Sprintf(" AND %s ~* $1", instagramQuoteIdentifier(mapping.GetDisplayNameField()))
	}
	if mapping.GetLastMessageAtField() != "" {
		query += " ORDER BY " + instagramQuoteIdentifier(mapping.GetLastMessageAtField()) + " DESC NULLS LAST"
	}
	rows, err := h.executeInstagramSQL(ctx, target, query+" LIMIT 100", params)
	if err != nil {
		return nil, err
	}
	response := make([]map[string]any, 0, len(rows))
	for _, row := range rows {
		response = append(response, row.AsMap())
	}
	return response, nil
}

func (h *HandlerV1) instagramUserIDForRecord(ctx context.Context, target *instagramProjectTarget, mapping *pb.InstagramChatMapping, recordID string) (string, error) {
	if mapping == nil {
		return "", errors.New("instagram mapping is missing")
	}
	rows, err := h.executeInstagramSQL(ctx, target, fmt.Sprintf(`SELECT %s AS instagram_user_id FROM %s WHERE guid = $1 AND deleted_at IS NULL LIMIT 1`, instagramQuoteIdentifier(mapping.GetInstagramUserIdField()), instagramQuoteIdentifier(mapping.GetTableSlug())), []string{recordID})
	if err != nil || len(rows) == 0 {
		return "", errors.New("instagram chat was not found")
	}
	userID := fmt.Sprint(rows[0].AsMap()["instagram_user_id"])
	if userID == "" {
		return "", errors.New("instagram user id is missing")
	}
	return userID, nil
}

func (h *HandlerV1) loadInstagramSecret(resource *pb.ProjectResource) (instagramSecret, error) {
	if resource == nil {
		return instagramSecret{}, errors.New("instagram resource is missing")
	}
	if resource.GetSecret() != nil {
		secret := instagramSecret{
			AccessToken:    secretString(resource.GetSecret().AsMap(), "instagram_user_access_token"),
			TokenExpiresAt: secretString(resource.GetSecret().AsMap(), "token_expires_at"),
		}
		if secret.AccessToken != "" {
			return secret, nil
		}
	}
	return instagramSecret{}, errors.New("instagram credentials are missing")
}

func secretString(data map[string]any, key string) string {
	value, ok := data[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func (h *HandlerV1) instagramGraphURL(path string) string {
	return fmt.Sprintf("%s/%s/%s", h.baseConf.InstagramGraphBaseURL, h.baseConf.InstagramGraphVersion, strings.TrimPrefix(path, "/"))
}

func (h *HandlerV1) instagramGraphRootURL(path string) string {
	return fmt.Sprintf("%s/%s", h.baseConf.InstagramGraphBaseURL, strings.TrimPrefix(path, "/"))
}

func (h *HandlerV1) instagramGraphGet(ctx context.Context, path string, query url.Values, out any) error {
	return h.instagramGraphGetURL(ctx, h.instagramGraphURL(path), query, out)
}

func (h *HandlerV1) instagramGraphGetRoot(ctx context.Context, path string, query url.Values, out any) error {
	return h.instagramGraphGetURL(ctx, h.instagramGraphRootURL(path), query, out)
}

func (h *HandlerV1) instagramGraphGetURL(ctx context.Context, endpoint string, query url.Values, out any) error {
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	return instagramGraphDo(req, out)
}

func (h *HandlerV1) instagramGraphPost(ctx context.Context, path string, form url.Values, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.instagramGraphURL(path), strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return instagramGraphDo(req, out)
}

func (h *HandlerV1) instagramGraphDelete(ctx context.Context, path string, query url.Values, out any) error {
	endpoint := h.instagramGraphURL(path)
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return err
	}
	return instagramGraphDo(req, out)
}

func (h *HandlerV1) instagramGraphJSONPost(ctx context.Context, path, accessToken string, payload any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.instagramGraphURL(path), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)
	return instagramGraphDo(req, out)
}

func instagramGraphDo(req *http.Request, out any) error {
	resp, err := instagramHTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("instagram graph api call: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read instagram graph response: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		var wrap struct {
			Error *instagramAPIError `json:"error"`
		}
		if json.Unmarshal(body, &wrap) == nil && wrap.Error != nil {
			return wrap.Error
		}
		return fmt.Errorf("instagram graph api %d: %s", resp.StatusCode, string(body))
	}
	if out == nil {
		return nil
	}
	if err = json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parse instagram graph response: %w", err)
	}
	return nil
}

func (h *HandlerV1) exchangeInstagramCode(ctx context.Context, code string) (instagramTokenResponse, error) {
	form := url.Values{
		"client_id":     {h.baseConf.InstagramClientID},
		"client_secret": {h.baseConf.InstagramClientSecret},
		"grant_type":    {"authorization_code"},
		"redirect_uri":  {h.baseConf.InstagramRedirectURI},
		"code":          {code},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, h.baseConf.InstagramOAuthAccessTokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return instagramTokenResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	var token instagramTokenResponse
	if err = instagramGraphDo(req, &token); err != nil {
		return instagramTokenResponse{}, err
	}
	if token.AccessToken == "" {
		return instagramTokenResponse{}, errors.New("empty access token in code exchange")
	}
	return token, nil
}

func (h *HandlerV1) exchangeInstagramLongLivedToken(ctx context.Context, shortLived string) (instagramTokenResponse, error) {
	var token instagramTokenResponse
	err := h.instagramGraphGetRoot(ctx, "access_token", url.Values{
		"grant_type":    {"ig_exchange_token"},
		"client_secret": {h.baseConf.InstagramClientSecret},
		"access_token":  {shortLived},
	}, &token)
	if err != nil {
		return instagramTokenResponse{}, err
	}
	if token.AccessToken == "" {
		return instagramTokenResponse{}, errors.New("empty access token in long-lived exchange")
	}
	return token, nil
}

func (h *HandlerV1) instagramFetchProfile(ctx context.Context, accessToken string) (instagramUserProfile, error) {
	var profile instagramUserProfile
	err := h.instagramGraphGet(ctx, "me", url.Values{
		"fields":       {"id,user_id,username,account_type,profile_picture_url"},
		"access_token": {accessToken},
	}, &profile)
	return profile, err
}

func (h *HandlerV1) instagramSubscribeAccount(ctx context.Context, igID, accessToken string) error {
	var result struct {
		Success bool `json:"success"`
	}
	return h.instagramGraphPost(ctx, igID+"/subscribed_apps", url.Values{
		"subscribed_fields": {config.InstagramSubscribedFields},
		"access_token":      {accessToken},
	}, &result)
}

func (h *HandlerV1) instagramUnsubscribeAccount(ctx context.Context, igID, accessToken string) error {
	return h.instagramGraphDelete(ctx, igID+"/subscribed_apps", url.Values{"access_token": {accessToken}}, nil)
}

func (h *HandlerV1) instagramSendText(ctx context.Context, igID, accessToken, recipientID, text string) (instagramSendMessageResponse, error) {
	var response instagramSendMessageResponse
	payload := map[string]any{
		"messaging_type": "RESPONSE",
		"recipient":      map[string]string{"id": recipientID},
		"message":        map[string]string{"text": text},
	}
	err := h.instagramGraphJSONPost(ctx, igID+"/messages", accessToken, payload, &response)
	return response, err
}

type instagramOAuthOutcome struct {
	success bool
	reason  string
	detail  string
	state   instagramOAuthState
}

func (h *HandlerV1) redirectInstagramOAuth(c *gin.Context, out instagramOAuthOutcome) {
	base := out.state.RedirectURL
	if base == "" {
		if out.success {
			base = h.baseConf.InstagramFrontendSuccessURL
		} else {
			base = h.baseConf.InstagramFrontendErrorURL
		}
	}
	if base == "" {
		base = "/"
	}
	base = applyInstagramRedirectPlaceholders(base, out.state)
	u, err := url.Parse(base)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, base)
		return
	}
	q := u.Query()
	if out.success {
		q.Set("instagram", "success")
	} else {
		q.Set("instagram", "error")
		if out.reason != "" {
			q.Set("reason", out.reason)
		}
		if out.detail != "" {
			q.Set("detail", out.detail)
		}
	}
	if out.state.ProjectID != "" {
		q.Set("project_id", out.state.ProjectID)
	}
	if out.state.EnvironmentID != "" {
		q.Set("environment_id", out.state.EnvironmentID)
	}
	if out.state.McpProjectID != "" {
		q.Set("mcp_project_id", out.state.McpProjectID)
	}
	u.RawQuery = q.Encode()
	c.Redirect(http.StatusTemporaryRedirect, u.String())
}

func (h *HandlerV1) getAndDeleteInstagramState(ctx context.Context, state string) (instagramOAuthState, error) {
	if h.centralRedis == nil || strings.TrimSpace(state) == "" {
		return instagramOAuthState{}, errors.New("instagram oauth state is missing")
	}
	key := config.InstagramOAuthStatePrefix + state
	body, err := h.centralRedis.Get(ctx, key).Bytes()
	if err != nil {
		return instagramOAuthState{}, err
	}
	_ = h.centralRedis.Del(ctx, key).Err()
	var payload instagramOAuthState
	if err = json.Unmarshal(body, &payload); err != nil {
		return instagramOAuthState{}, err
	}
	return payload, nil
}

func resolveInstagramRedirectURL(c *gin.Context) string {
	if raw := strings.TrimSpace(c.Query("redirect_uri")); raw != "" {
		return raw
	}
	return strings.TrimSpace(c.GetHeader("Referer"))
}

func applyInstagramRedirectPlaceholders(target string, state instagramOAuthState) string {
	replacer := strings.NewReplacer(
		"{project_id}", url.PathEscape(state.ProjectID),
		":project_id", url.PathEscape(state.ProjectID),
		"YOUR_PROJECT_ID", url.PathEscape(state.ProjectID),
		"{environment_id}", url.PathEscape(state.EnvironmentID),
		":environment_id", url.PathEscape(state.EnvironmentID),
		"YOUR_ENVIRONMENT_ID", url.PathEscape(state.EnvironmentID),
		"{mcp_project_id}", url.PathEscape(state.McpProjectID),
		":mcp_project_id", url.PathEscape(state.McpProjectID),
		"YOUR_MCP_PROJECT_ID", url.PathEscape(state.McpProjectID),
	)
	return replacer.Replace(target)
}

func instagramMappingFromQuery(c *gin.Context) instagramChatMappingRequest {
	return instagramChatMappingRequest{
		TableID:              c.Query("table_id"),
		TableSlug:            c.Query("table_slug"),
		InstagramUserIDField: c.DefaultQuery("instagram_user_id_field", "instagram_user_id"),
		UsernameField:        c.Query("username_field"),
		DisplayNameField:     c.Query("display_name_field"),
		ProfilePictureField:  c.Query("profile_picture_field"),
		LastMessageField:     c.Query("last_message_field"),
		LastMessageAtField:   c.Query("last_message_at_field"),
		UnreadCountField:     c.Query("unread_count_field"),
		StatusField:          c.Query("status_field"),
		ConversationIDField:  c.Query("conversation_id_field"),
	}
}

func instagramResourceResponse(resource *pb.ProjectResource) instagramIntegrationResponse {
	instagram := resource.GetSettings().GetInstagram()
	response := instagramIntegrationResponse{
		ResourceID:        resource.GetId(),
		IgID:              instagram.GetIgId(),
		Username:          instagram.GetUsername(),
		AccountType:       instagram.GetAccountType(),
		ProfilePictureURL: instagram.GetProfilePictureUrl(),
		Status:            instagram.GetStatus(),
		ConnectedAt:       instagram.GetConnectedAt(),
		Mapping:           instagram.GetMapping(),
	}
	if instagram.GetUsername() != "" {
		response.ProfileURL = "https://instagram.com/" + strings.TrimPrefix(instagram.GetUsername(), "@")
	}
	return response
}

func instagramInboxPrompt(mapping *pb.InstagramChatMapping) string {
	if mapping == nil {
		return ""
	}
	return fmt.Sprintf(`SYSTEM TASK - add Instagram Inbox to the current generated admin panel.

First inspect the current project_files and preserve its existing framework, navigation, auth conventions and visual language. Do not replace the app or introduce a separate application.

Add a menu item and route named "Support → Instagram Inbox". Implement a responsive three-pane inbox:
1. chats list with search, display name, username, profile image, last message, unread count and status;
2. selected conversation with history, attachments and a text reply composer;
3. a details panel that displays mapped fields from the selected chat record.

The backend contract (all responses are wrapped in {data: ...}) is:
- GET /v1/instagram/chats?search=... -> data.chats
- GET /v1/instagram/chats/{chat_record_guid}/messages -> data.messages
- GET /v1/instagram/messages/{message_guid}/attachments -> data.attachments
- GET /v1/instagram/messages/{message_guid}/attachments/{attachment_guid} streams the attachment through the authorised gateway
- POST /v1/instagram/chats/{chat_record_guid}/messages with {"text":"..."} sends an answer.

Use the app's existing authenticated API client. Never ask for, store or render an Instagram access token, raw Instagram media URL or raw_payload. Restrict the page using the app's existing owner/administrator/operator permission mechanism; a customer must not see it.

Mapped chat table: %q. Its Instagram fields are user_id=%q, username=%q, display_name=%q, profile_picture=%q, last_message=%q, last_message_at=%q, unread_count=%q, status=%q, conversation_id=%q. The chat record guid is the identifier passed to Inbox APIs. The message history is maintained by the backend in ugen_instagram_messages; do not create client-side message tables.

Make the change in the existing frontend files, then leave the build in a working state.`,
		mapping.GetTableSlug(),
		mapping.GetInstagramUserIdField(),
		mapping.GetUsernameField(),
		mapping.GetDisplayNameField(),
		mapping.GetProfilePictureField(),
		mapping.GetLastMessageField(),
		mapping.GetLastMessageAtField(),
		mapping.GetUnreadCountField(),
		mapping.GetStatusField(),
		mapping.GetConversationIdField(),
	)
}

func instagramSystemFields(definitions []telegramSystemField) []*pbo.CreateFieldsRequest {
	fields := make([]*pbo.CreateFieldsRequest, 0, len(definitions))
	for _, definition := range definitions {
		attrs, _ := structpb.NewStruct(map[string]any{"instagram_system": true, "label_en": definition.label})
		fields = append(fields, &pbo.CreateFieldsRequest{Id: uuid.NewString(), Slug: definition.slug, Label: definition.label, Type: definition.typeID, Unique: definition.unique, IsVisible: false, Attributes: attrs})
	}
	return fields
}

func instagramQuoteIdentifier(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func instagramPutMappedValue(data map[string]any, field string, value any) {
	if field != "" {
		data[field] = value
	}
}

func instagramMessageChatRelationField(mapping *pb.InstagramChatMapping) (string, error) {
	if mapping == nil || !instagramIdentifierPattern.MatchString(mapping.GetTableSlug()) {
		return "", errors.New("instagram mapping table is invalid")
	}
	field := mapping.GetTableSlug() + "_id"
	if !instagramIdentifierPattern.MatchString(field) {
		return "", errors.New("instagram message relation field is invalid")
	}
	return field, nil
}

func instagramEventIsPersistable(event instagramMessagingEvent) bool {
	return event.Message != nil || event.Postback != nil || event.Reaction != nil
}

func instagramEventType(event instagramMessagingEvent) string {
	switch {
	case event.Message != nil:
		if event.Message.IsEcho {
			return "message_echo"
		}
		return "message"
	case len(event.MessageEdit) > 0:
		return "message_edit"
	case event.Postback != nil:
		return "postback"
	case event.Reaction != nil:
		return "reaction"
	case event.Read != nil:
		return "read"
	case len(event.Optin) > 0:
		return "optin"
	case len(event.Referral) > 0:
		return "referral"
	default:
		return "unknown"
	}
}

func instagramRawKeys(raw map[string]any) []string {
	if len(raw) == 0 {
		return nil
	}
	keys := make([]string, 0, len(raw))
	for key := range raw {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func instagramEventKey(event instagramMessagingEvent) string {
	if event.Message != nil && event.Message.Mid != "" {
		return "message:" + event.Message.Mid
	}
	if id := instagramMessageEditID(event); id != "" {
		return "message_edit:" + id
	}
	if event.Postback != nil && event.Postback.Mid != "" {
		return "postback:" + event.Postback.Mid
	}
	if event.Reaction != nil && event.Reaction.Mid != "" {
		return fmt.Sprintf("reaction:%s:%s:%d", event.Reaction.Mid, instagramSenderID(event), event.Timestamp)
	}
	return fmt.Sprintf("event:%s:%d", instagramSenderID(event), event.Timestamp)
}

func instagramMessageID(event instagramMessagingEvent) string {
	if event.Message != nil && event.Message.Mid != "" {
		return event.Message.Mid
	}
	if id := instagramMessageEditID(event); id != "" {
		return id
	}
	if event.Postback != nil && event.Postback.Mid != "" {
		return event.Postback.Mid
	}
	if event.Reaction != nil && event.Reaction.Mid != "" {
		return event.Reaction.Mid
	}
	return instagramEventKey(event)
}

func instagramMessageEditID(event instagramMessagingEvent) string {
	if len(event.MessageEdit) == 0 {
		return ""
	}
	payload := instagramMessageEditMap(event)
	if value := instagramStringFromMap(payload, "mid", "message_id", "id"); value != "" {
		return value
	}
	if nested, ok := payload["message"].(map[string]any); ok {
		return instagramStringFromMap(nested, "mid", "message_id", "id")
	}
	return ""
}

func instagramMessageEditText(event instagramMessagingEvent) string {
	if len(event.MessageEdit) == 0 {
		return ""
	}
	payload := instagramMessageEditMap(event)
	if value := instagramStringFromMap(payload, "text"); value != "" {
		return value
	}
	if nested, ok := payload["message"].(map[string]any); ok {
		return instagramStringFromMap(nested, "text")
	}
	return ""
}

func instagramMessageEditMap(event instagramMessagingEvent) map[string]any {
	payload := map[string]any{}
	_ = json.Unmarshal(event.MessageEdit, &payload)
	return payload
}

func instagramStringFromMap(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key]; ok && value != nil {
			text := strings.TrimSpace(fmt.Sprint(value))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func instagramNestedInstagramID(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		value, ok := payload[key]
		if !ok || value == nil {
			continue
		}
		switch typed := value.(type) {
		case map[string]any:
			if id := instagramStringFromMap(typed, "id", "igsid", "user_id"); id != "" {
				return id
			}
		default:
			text := strings.TrimSpace(fmt.Sprint(typed))
			if text != "" && text != "<nil>" {
				return text
			}
		}
	}
	return ""
}

func instagramSenderID(event instagramMessagingEvent) string {
	if value := strings.TrimSpace(event.Sender.ID); value != "" {
		return value
	}
	if len(event.MessageEdit) == 0 {
		return ""
	}
	payload := instagramMessageEditMap(event)
	if value := instagramNestedInstagramID(payload, "sender", "from", "user"); value != "" {
		return value
	}
	if nested, ok := payload["message"].(map[string]any); ok {
		return instagramNestedInstagramID(nested, "sender", "from", "user")
	}
	return ""
}

func instagramRecipientID(event instagramMessagingEvent) string {
	if value := strings.TrimSpace(event.Recipient.ID); value != "" {
		return value
	}
	if len(event.MessageEdit) == 0 {
		return ""
	}
	payload := instagramMessageEditMap(event)
	if value := instagramNestedInstagramID(payload, "recipient", "to"); value != "" {
		return value
	}
	if nested, ok := payload["message"].(map[string]any); ok {
		return instagramNestedInstagramID(nested, "recipient", "to")
	}
	return ""
}

func instagramDisplayName(event instagramMessagingEvent) string {
	if sender := instagramSenderID(event); sender != "" {
		return sender
	}
	return "Instagram user"
}

func instagramMessageText(event instagramMessagingEvent) string {
	if event.Message != nil {
		if strings.TrimSpace(event.Message.Text) != "" {
			return event.Message.Text
		}
		if len(event.Message.Attachments) > 0 {
			return "[" + instagramMessageType(event) + "]"
		}
	}
	if event.Postback != nil {
		if event.Postback.Title != "" {
			return event.Postback.Title
		}
		return event.Postback.Payload
	}
	if event.Reaction != nil {
		return strings.TrimSpace(event.Reaction.Action + " " + event.Reaction.Reaction)
	}
	if text := instagramMessageEditText(event); text != "" {
		return text
	}
	return ""
}

func instagramMessageType(event instagramMessagingEvent) string {
	if len(event.MessageEdit) > 0 {
		return "message_edit"
	}
	if event.Postback != nil {
		return "postback"
	}
	if event.Reaction != nil {
		return "reaction"
	}
	if event.Message != nil && len(event.Message.Attachments) > 0 {
		return event.Message.Attachments[0].Type
	}
	return "text"
}

func instagramEventTime(event instagramMessagingEvent) time.Time {
	ts := event.Timestamp
	if ts <= 0 {
		return time.Now().UTC()
	}
	if ts > 1000000000000 {
		return time.UnixMilli(ts)
	}
	return time.Unix(ts, 0)
}

func instagramAttachmentFileName(mediaURL string) string {
	parsed, err := url.Parse(mediaURL)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

func (h *HandlerV1) verifyInstagramSignature(header string, body []byte) bool {
	secrets := append([]string{h.baseConf.InstagramClientSecret}, h.baseConf.InstagramLegacyClientSecrets...)
	return verifyMetaWebhookSignature(header, config.InstagramSignaturePrefix, body, secrets...)
}

func isDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "duplicate") || strings.Contains(lower, "unique") || strings.Contains(lower, "already exists")
}

func truncateInstagramError(err error) string {
	if err == nil {
		return ""
	}
	value := err.Error()
	if len(value) > 500 {
		return value[:500]
	}
	return value
}

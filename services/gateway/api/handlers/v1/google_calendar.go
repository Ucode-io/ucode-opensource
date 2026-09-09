package v1

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/googlecalendar"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
)

const (
	googleCalendarOAuthStatePrefix = "google-calendar-oauth-state:"
	googleCalendarOAuthStateTTL    = 10 * time.Minute

	// Lightweight frontend pages the popup lands on so it can postMessage its
	// opener and self-close (mirrors the GitHub/Bitbucket /oauth/success flow),
	// instead of surfacing a full dashboard.
	googleCalendarSuccessPath = "/settings/google-calendar-success"
	googleCalendarErrorPath   = "/settings/google-calendar-error"
)

type googleCalendarOAuthState struct {
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
	McpProjectID  string `json:"mcp_project_id,omitempty"`
	UserID        string `json:"user_id"`
	// FrontendOrigin is the origin (scheme://host) of the app that opened the
	// OAuth popup, captured from the connect request. The callback redirects the
	// popup back here so it closes on the same app that started the flow.
	FrontendOrigin string `json:"frontend_origin,omitempty"`
}

type googleCalendarMappingRequest struct {
	TableID          string `json:"table_id"`
	TableSlug        string `json:"table_slug"`
	TitleField       string `json:"title_field"`
	StartField       string `json:"start_field"`
	EndField         string `json:"end_field"`
	DescriptionField string `json:"description_field"`
	LocationField    string `json:"location_field"`
	AttendeesField   string `json:"attendees_field"`
	StatusField      string `json:"status_field"`
}

func (r googleCalendarMappingRequest) toProto() *pb.GoogleCalendarMapping {
	return &pb.GoogleCalendarMapping{
		TableSlug:        strings.TrimSpace(r.TableSlug),
		TitleField:       strings.TrimSpace(r.TitleField),
		StartField:       strings.TrimSpace(r.StartField),
		EndField:         strings.TrimSpace(r.EndField),
		DescriptionField: strings.TrimSpace(r.DescriptionField),
		LocationField:    strings.TrimSpace(r.LocationField),
		AttendeesField:   strings.TrimSpace(r.AttendeesField),
		StatusField:      strings.TrimSpace(r.StatusField),
	}
}

// GoogleCalendarConnect godoc
// @Security ApiKeyAuth
// @ID google_calendar_connect
// @Router /v1/google-calendar/connect [GET]
// @Summary Initiate Google Calendar OAuth
// @Tags Google Calendar Integration
// @Success 200 {object} status_http.Response{data=map[string]string}
func (h *HandlerV1) GoogleCalendarConnect(c *gin.Context) {
	projectID, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentID, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	mcpProjectID := strings.TrimSpace(c.Query("mcp_project_id"))
	if mcpProjectID != "" && !util.IsValidUUID(mcpProjectID) {
		h.HandleResponse(c, status_http.InvalidArgument, "mcp_project_id is an invalid uuid")
		return
	}

	userID := ""
	if value, ok := c.Get("user_id"); ok {
		userID, _ = value.(string)
	}

	// Captured from the browser-set Origin/Referer of this connect XHR so the
	// callback can return the popup to the same app that opened it.
	frontendOrigin := resolveGoogleCalendarFrontendOrigin(c)

	oauthConfig, err := googlecalendar.NewOAuthConfig(h.googleCalendarConfig())
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	state, err := newGoogleCalendarOAuthState()
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	payload := googleCalendarOAuthState{
		ProjectID:      projectID.(string),
		EnvironmentID:  environmentID.(string),
		McpProjectID:   mcpProjectID,
		UserID:         userID,
		FrontendOrigin: frontendOrigin,
	}
	if err := h.storeGoogleCalendarOAuthState(c.Request.Context(), state, payload); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	authURL := oauthConfig.AuthCodeURL(
		state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
		oauth2.SetAuthURLParam("include_granted_scopes", "true"),
	)

	if c.Query("redirect") == "true" {
		c.Redirect(http.StatusTemporaryRedirect, authURL)
		return
	}

	c.PureJSON(status_http.OK.Code, status_http.Response{
		Status:        status_http.OK.Status,
		Description:   status_http.OK.Description,
		Data:          gin.H{"auth_url": authURL},
		CustomMessage: status_http.OK.CustomMessage,
	})
}

// GoogleCalendarCallback godoc
// @ID google_calendar_callback
// @Router /v1/google-calendar/callback [GET]
// @Summary Google Calendar OAuth Callback
// @Tags Google Calendar Integration
// @Param code query string true "Authorization code from Google"
// @Param state query string true "CSRF state token"
// @Success 307 "Redirect to frontend"
func (h *HandlerV1) GoogleCalendarCallback(c *gin.Context) {
	if googleErr := c.Query("error"); googleErr != "" {
		h.redirectGoogleCalendarOAuth(c, false, googleErr)
		return
	}

	code := c.Query("code")
	stateToken := c.Query("state")
	if code == "" || stateToken == "" {
		h.redirectGoogleCalendarOAuth(c, false, "missing_code_or_state")
		return
	}

	state, err := h.popGoogleCalendarOAuthState(c.Request.Context(), stateToken)
	if err != nil {
		h.redirectGoogleCalendarOAuth(c, false, "invalid_state")
		return
	}

	oauthConfig, err := googlecalendar.NewOAuthConfig(h.googleCalendarConfig())
	if err != nil {
		h.redirectGoogleCalendarOAuth(c, false, "oauth_config_error", state)
		return
	}

	token, err := oauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		h.redirectGoogleCalendarOAuth(c, false, "token_exchange_failed", state)
		return
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		h.redirectGoogleCalendarOAuth(c, false, "refresh_token_missing", state)
		return
	}

	credentials, err := googlecalendar.CredentialsFromRefreshToken(token.RefreshToken)
	if err != nil {
		h.redirectGoogleCalendarOAuth(c, false, "oauth_credentials_error", state)
		return
	}

	if err := h.upsertGoogleCalendarResource(c.Request.Context(), state, &pb.Settings{GoogleCalendar: credentials}); err != nil {
		h.redirectGoogleCalendarOAuth(c, false, "resource_save_failed", state)
		return
	}

	h.redirectGoogleCalendarOAuth(c, true, "", state)
}

// UpdateGoogleCalendarMapping godoc
// @Security ApiKeyAuth
// @ID google_calendar_update_mapping
// @Router /v1/google-calendar/mapping [PUT]
// @Summary Save Google Calendar table field mapping
// @Tags Google Calendar Integration
func (h *HandlerV1) UpdateGoogleCalendarMapping(c *gin.Context) {
	var request googleCalendarMappingRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	mapping := request.toProto()

	projectID, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}
	environmentID, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	resource, credentials, configured, err := googlecalendar.GetConfiguredResource(c.Request.Context(), h.companyServices, projectID.(string), environmentID.(string))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("get configured google calendar resource: %w", err).Error())
		return
	}
	if !configured || credentials == nil || strings.TrimSpace(credentials.GetRefreshToken()) == "" {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/google-calendar/connect to connect Google Calendar")
		return
	}
	if !util.IsValidUUID(resource.GetId()) {
		h.HandleResponse(c, status_http.InternalServerError, "google calendar project resource id is empty or invalid")
		return
	}

	builderResource, err := h.companyServices.ServiceResource().GetSingle(c.Request.Context(), &pb.GetSingleServiceResourceReq{
		ProjectId:     projectID.(string),
		EnvironmentId: environmentID.(string),
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("get builder service resource: %w", err).Error())
		return
	}
	if !util.IsValidUUID(builderResource.GetResourceEnvironmentId()) {
		h.HandleResponse(c, status_http.InternalServerError, "builder resource environment id is empty or invalid")
		return
	}
	services, err := h.GetProjectSrvc(c.Request.Context(), projectID.(string), builderResource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("get project object-builder service: %w", err).Error())
		return
	}
	table, err := googlecalendar.ResolveTable(c.Request.Context(), services, builderResource.ResourceEnvironmentId, request.TableID, mapping.GetTableSlug())
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("resolve google calendar mapping table: %w", err).Error())
		return
	}
	mapping.TableSlug = table.GetSlug()
	if err := googlecalendar.EnsureHiddenFieldsForTable(c.Request.Context(), services, builderResource.ResourceEnvironmentId, table, mapping); err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("ensure google calendar hidden fields: %w", err).Error())
		return
	}

	credentials.Mapping = mapping
	credentials.SyncDirection = googlecalendar.SyncDirection
	credentials.CalendarId = googlecalendar.DefaultCalendarID
	update := &pb.ProjectResource{
		Id:            resource.GetId(),
		ProjectId:     projectID.(string),
		EnvironmentId: environmentID.(string),
		Name:          resource.GetName(),
		Type:          pb.ResourceType_GOOGLE_CALENDAR.String(),
		ResourceType:  int32(pb.ResourceType_GOOGLE_CALENDAR),
		Settings:      sanitizeGoogleCalendarSettingsForStorage(&pb.Settings{GoogleCalendar: credentials}),
	}
	if update.Name == "" {
		update.Name = "Google Calendar"
	}
	if _, err := h.companyServices.Resource().UpdateProjectResource(c.Request.Context(), update); err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("update google calendar project resource: %w", err).Error())
		return
	}

	sanitizeGoogleCalendarResourceForResponse(update)
	h.HandleResponse(c, status_http.OK, update)
}

// GetGoogleCalendarMapping godoc
// @Security ApiKeyAuth
// @ID google_calendar_get_mapping
// @Router /v1/google-calendar/mapping [GET]
// @Summary Get Google Calendar table field mapping
// @Tags Google Calendar Integration
func (h *HandlerV1) GetGoogleCalendarMapping(c *gin.Context) {
	projectID, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}
	environmentID, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentID.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}
	resource, _, configured, err := googlecalendar.GetConfiguredResource(c.Request.Context(), h.companyServices, projectID.(string), environmentID.(string))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	if !configured {
		h.HandleResponse(c, status_http.OK, gin.H{"mapping": nil})
		return
	}
	sanitizeGoogleCalendarResourceForResponse(resource)
	h.HandleResponse(c, status_http.OK, resource)
}

func (h *HandlerV1) googleCalendarConfig() googlecalendar.Config {
	return googlecalendar.Config{
		ClientID:     h.baseConf.GoogleCalendarClientID,
		ClientSecret: h.baseConf.GoogleCalendarClientSecret,
		RedirectURI:  h.baseConf.GoogleCalendarRedirectURI,
	}
}

func (h *HandlerV1) upsertGoogleCalendarResource(ctx context.Context, state googleCalendarOAuthState, settings *pb.Settings) error {
	list, err := h.companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          pb.ResourceType_GOOGLE_CALENDAR,
	})
	if err != nil {
		return err
	}
	if len(list.GetResources()) > 1 {
		return errors.New("multiple google calendar resources configured for project environment")
	}

	if len(list.GetResources()) == 1 {
		current := list.GetResources()[0]
		if !util.IsValidUUID(current.GetId()) {
			return errors.New("google calendar project resource id is empty or invalid")
		}
		name := current.GetName()
		if name == "" {
			name = "Google Calendar"
		}
		var currentCalendar *pb.GoogleCalendarCredentials
		if current.GetSettings() != nil {
			currentCalendar = current.GetSettings().GetGoogleCalendar()
		}
		requestCalendar := settings.GetGoogleCalendar()
		if currentCalendar != nil && currentCalendar.GetMapping() != nil {
			requestCalendar.Mapping = currentCalendar.GetMapping()
		}
		_, err = h.companyServices.Resource().UpdateProjectResource(ctx, &pb.ProjectResource{
			Id:            current.GetId(),
			ProjectId:     state.ProjectID,
			EnvironmentId: state.EnvironmentID,
			Name:          name,
			Type:          pb.ResourceType_GOOGLE_CALENDAR.String(),
			ResourceType:  int32(pb.ResourceType_GOOGLE_CALENDAR),
			Settings:      sanitizeGoogleCalendarSettingsForStorage(settings),
		})
		return err
	}

	_, err = h.companyServices.Resource().AddResourceToProject(ctx, &pb.AddResourceToProjectRequest{
		Name:          "Google Calendar",
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          pb.ResourceType_GOOGLE_CALENDAR,
		Settings:      sanitizeGoogleCalendarSettingsForStorage(settings),
	})
	return err
}

func (h *HandlerV1) storeGoogleCalendarOAuthState(ctx context.Context, state string, payload googleCalendarOAuthState) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	key := googleCalendarOAuthStatePrefix + state
	if h.centralRedis != nil {
		return h.centralRedis.Set(ctx, key, body, googleCalendarOAuthStateTTL).Err()
	}
	if h.cache != nil {
		h.cache.Add(key, body, googleCalendarOAuthStateTTL)
		return nil
	}
	return errors.New("oauth state storage is not configured")
}

func (h *HandlerV1) popGoogleCalendarOAuthState(ctx context.Context, state string) (googleCalendarOAuthState, error) {
	var payload googleCalendarOAuthState
	key := googleCalendarOAuthStatePrefix + state

	var body []byte
	if h.centralRedis != nil {
		value, err := h.centralRedis.Get(ctx, key).Result()
		if err != nil {
			return payload, err
		}
		body = []byte(value)
		_ = h.centralRedis.Del(ctx, key).Err()
	} else if h.cache != nil {
		value, ok := h.cache.Get(key)
		if !ok {
			return payload, errors.New("oauth state not found")
		}
		body = value
	} else {
		return payload, errors.New("oauth state storage is not configured")
	}

	if err := json.Unmarshal(body, &payload); err != nil {
		return payload, err
	}
	if payload.ProjectID == "" || payload.EnvironmentID == "" {
		return payload, errors.New("oauth state payload is incomplete")
	}
	return payload, nil
}

func (h *HandlerV1) redirectGoogleCalendarOAuth(c *gin.Context, success bool, reason string, states ...googleCalendarOAuthState) {
	var state googleCalendarOAuthState
	if len(states) > 0 {
		state = states[0]
	}

	// Preferred path: send the popup back to the frontend origin that opened it
	// (captured at connect time). It lands on the lightweight
	// google-calendar-success/error page on the SAME app, which postMessages its
	// opener and self-closes — exactly like the GitHub/Bitbucket flow. This
	// avoids surfacing an unrelated dashboard when the configured frontend URL
	// points at another origin or at the project page.
	if origin := strings.TrimSpace(state.FrontendOrigin); origin != "" {
		if target := buildGoogleCalendarOriginRedirect(origin, success, reason, state); target != "" {
			c.Redirect(http.StatusTemporaryRedirect, target)
			return
		}
	}

	// Fallback: the origin could not be determined — use the configured frontend
	// URL as before.
	target := h.baseConf.GoogleCalendarFrontendErrorURL
	if success {
		target = h.baseConf.GoogleCalendarFrontendSuccessURL
	}
	target = strings.TrimSpace(target)
	hasConfiguredTarget := target != ""
	if target == "" {
		if success {
			target = "/?google_calendar=success"
		} else {
			target = "/?google_calendar=error"
		}
	}

	if len(states) > 0 {
		target = applyGoogleCalendarRedirectPlaceholders(target, state)
	}

	if hasConfiguredTarget {
		c.Redirect(http.StatusTemporaryRedirect, target)
		return
	}

	u, err := url.Parse(target)
	if err == nil {
		q := u.Query()
		if success {
			q.Set("google_calendar", "success")
		} else {
			q.Set("google_calendar", "error")
			if reason != "" {
				q.Set("reason", reason)
			}
		}
		if state.ProjectID != "" {
			q.Set("project_id", state.ProjectID)
		}
		if state.EnvironmentID != "" {
			q.Set("environment_id", state.EnvironmentID)
		}
		if state.McpProjectID != "" {
			q.Set("mcp_project_id", state.McpProjectID)
		}
		u.RawQuery = q.Encode()
		target = u.String()
	}

	c.Redirect(http.StatusTemporaryRedirect, target)
}

// resolveGoogleCalendarFrontendOrigin derives the origin (scheme://host) of the
// app that initiated the OAuth flow from the browser-set Origin header, falling
// back to the Referer. Both are set by the browser on the connect XHR, so they
// identify the real caller and are safe to redirect back to (not user-spoofable
// for a victim's session). Returns "" when neither yields a valid http(s) origin.
func resolveGoogleCalendarFrontendOrigin(c *gin.Context) string {
	origin := strings.TrimSpace(c.GetHeader("Origin"))
	if origin == "" {
		if ref := strings.TrimSpace(c.Request.Referer()); ref != "" {
			if u, err := url.Parse(ref); err == nil && u.Scheme != "" && u.Host != "" {
				origin = u.Scheme + "://" + u.Host
			}
		}
	}
	if origin == "" {
		return ""
	}
	u, err := url.Parse(origin)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

// buildGoogleCalendarOriginRedirect builds the popup redirect URL on the captured
// frontend origin, pointing at the lightweight success/error close-page and
// carrying the OAuth result + ids as query params. Returns "" if the URL can't
// be built so the caller can fall back to the configured target.
func buildGoogleCalendarOriginRedirect(origin string, success bool, reason string, state googleCalendarOAuthState) string {
	path := googleCalendarErrorPath
	if success {
		path = googleCalendarSuccessPath
	}

	u, err := url.Parse(strings.TrimRight(origin, "/") + path)
	if err != nil {
		return ""
	}

	q := u.Query()
	if success {
		q.Set("google_calendar", "success")
	} else {
		q.Set("google_calendar", "error")
		if reason != "" {
			q.Set("reason", reason)
		}
	}
	if state.ProjectID != "" {
		q.Set("project_id", state.ProjectID)
	}
	if state.EnvironmentID != "" {
		q.Set("environment_id", state.EnvironmentID)
	}
	if state.McpProjectID != "" {
		q.Set("mcp_project_id", state.McpProjectID)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func applyGoogleCalendarRedirectPlaceholders(target string, state googleCalendarOAuthState) string {
	replacer := strings.NewReplacer(
		"{project_id}", url.PathEscape(state.ProjectID),
		":project_id", url.PathEscape(state.ProjectID),
		"{environment_id}", url.PathEscape(state.EnvironmentID),
		":environment_id", url.PathEscape(state.EnvironmentID),
		"{mcp_project_id}", url.PathEscape(state.McpProjectID),
		":mcp_project_id", url.PathEscape(state.McpProjectID),
	)
	return replacer.Replace(target)
}

func newGoogleCalendarOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

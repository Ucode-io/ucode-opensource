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

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/fileupload"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	gatewayConfig "github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"golang.org/x/oauth2"
)

const (
	googleDriveOAuthStatePrefix = "google-drive-oauth-state:"
	googleDriveOAuthStateTTL    = 10 * time.Minute
	googleDriveFolderMenuType   = "GOOGLE_DRIVE_FOLDER"

	// Lightweight frontend pages the popup lands on so it can postMessage its
	// opener and self-close (mirrors the GitHub/Bitbucket /oauth/success flow),
	// instead of surfacing a full dashboard.
	googleDriveSuccessPath = "/settings/google-drive-success"
	googleDriveErrorPath   = "/settings/google-drive-error"
)

type googleDriveOAuthState struct {
	ProjectID     string `json:"project_id"`
	EnvironmentID string `json:"environment_id"`
	McpProjectID  string `json:"mcp_project_id,omitempty"`
	UserID        string `json:"user_id"`
	// FrontendOrigin is the origin (scheme://host) of the app that opened the
	// OAuth popup, captured from the connect request. The callback redirects the
	// popup back here so it closes on the same app that started the flow.
	FrontendOrigin string `json:"frontend_origin,omitempty"`
}

// GoogleDriveConnect godoc
// @Security ApiKeyAuth
// @ID google_drive_connect
// @Router /v1/google-drive/connect [GET]
// @Summary Initiate Google Drive OAuth
// @Tags Google Drive Integration
// @Success 200 {object} status_http.Response{data=map[string]string}
func (h *HandlerV1) GoogleDriveConnect(c *gin.Context) {
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
	frontendOrigin := resolveGoogleDriveFrontendOrigin(c)

	oauthConfig, err := fileupload.NewOAuthConfig(h.googleDriveConfig())
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if err := h.ensureGoogleDriveFolderMenuForExistingResource(c.Request.Context(), googleDriveOAuthState{
		ProjectID:     projectID.(string),
		EnvironmentID: environmentID.(string),
		McpProjectID:  mcpProjectID,
		UserID:        userID,
	}); err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	state, err := newGoogleDriveOAuthState()
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	if err := h.storeGoogleDriveOAuthState(c.Request.Context(), state, googleDriveOAuthState{
		ProjectID:      projectID.(string),
		EnvironmentID:  environmentID.(string),
		McpProjectID:   mcpProjectID,
		UserID:         userID,
		FrontendOrigin: frontendOrigin,
	}); err != nil {
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

// GoogleDriveCallback godoc
// @ID google_drive_callback
// @Router /v1/google-drive/callback [GET]
// @Summary Google Drive OAuth Callback
// @Tags Google Drive Integration
// @Param code query string true "Authorization code from Google"
// @Param state query string true "CSRF state token"
// @Success 307 "Redirect to frontend"
func (h *HandlerV1) GoogleDriveCallback(c *gin.Context) {
	if googleErr := c.Query("error"); googleErr != "" {
		h.redirectGoogleDriveOAuth(c, false, googleErr)
		return
	}

	code := c.Query("code")
	stateToken := c.Query("state")
	if code == "" || stateToken == "" {
		h.redirectGoogleDriveOAuth(c, false, "missing_code_or_state")
		return
	}

	state, err := h.popGoogleDriveOAuthState(c.Request.Context(), stateToken)
	if err != nil {
		h.redirectGoogleDriveOAuth(c, false, "invalid_state")
		return
	}

	config := h.googleDriveConfig()
	oauthConfig, err := fileupload.NewOAuthConfig(config)
	if err != nil {
		h.redirectGoogleDriveOAuth(c, false, "oauth_config_error", state)
		return
	}

	token, err := oauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		h.redirectGoogleDriveOAuth(c, false, "token_exchange_failed", state)
		return
	}
	if strings.TrimSpace(token.RefreshToken) == "" {
		h.redirectGoogleDriveOAuth(c, false, "refresh_token_missing", state)
		return
	}

	project, err := h.companyServices.Project().GetById(c.Request.Context(), &pb.GetProjectByIdRequest{
		ProjectId: state.ProjectID,
	})
	if err != nil {
		h.redirectGoogleDriveOAuth(c, false, "project_not_found", state)
		return
	}

	credentials, err := fileupload.OAuthCredentialsFromRefreshToken(config, token.RefreshToken)
	if err != nil {
		h.redirectGoogleDriveOAuth(c, false, "oauth_credentials_error", state)
		return
	}

	uploader := fileupload.NewGoogleDriveUploader(h.companyServices.Resource(), config)
	folder, err := uploader.ProvisionFolder(c.Request.Context(), credentials, fileupload.GoogleDriveCreateFolderRequest{
		Name: fileupload.ProjectFolderName(project.GetTitle(), state.ProjectID),
	})
	if err != nil {
		h.redirectGoogleDriveOAuth(c, false, "folder_create_failed", state)
		return
	}

	settings := &pb.Settings{
		GoogleDrive: &pb.GoogleDriveCredentials{
			AuthType:     "oauth",
			FolderId:     folder.GetFolderID(),
			Visibility:   googleDriveVisibility(config.Visibility),
			RefreshToken: strings.TrimSpace(token.RefreshToken),
		},
	}

	if err := h.upsertGoogleDriveResource(c.Request.Context(), state, settings); err != nil {
		h.redirectGoogleDriveOAuth(c, false, "resource_save_failed", state)
		return
	}

	if err := h.ensureGoogleDriveFolderMenu(c.Request.Context(), state, folder.GetFolderID()); err != nil {
		h.redirectGoogleDriveOAuth(c, false, "menu_create_failed", state)
		return
	}

	h.redirectGoogleDriveOAuth(c, true, "", state)
}

func (h *HandlerV1) googleDriveConfig() fileupload.GoogleDriveConfig {
	return fileupload.GoogleDriveConfig{
		ClientID:           h.baseConf.GoogleDriveClientID,
		ClientSecret:       h.baseConf.GoogleDriveClientSecret,
		RedirectURI:        h.baseConf.GoogleDriveRedirectURI,
		ServiceAccountJSON: h.baseConf.GoogleDriveServiceAccountJSON,
		ParentFolderID:     h.baseConf.GoogleDriveParentFolderID,
		Visibility:         h.baseConf.GoogleDriveVisibility,
	}
}

func (h *HandlerV1) upsertGoogleDriveResource(ctx context.Context, state googleDriveOAuthState, settings *pb.Settings) error {
	list, err := h.companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          pb.ResourceType_GOOGLE_DRIVE,
	})
	if err != nil {
		return err
	}
	if len(list.GetResources()) > 1 {
		return errors.New("multiple google drive resources configured for project environment")
	}

	if len(list.GetResources()) == 1 {
		current := list.GetResources()[0]
		name := current.GetName()
		if name == "" {
			name = "Google Drive"
		}
		_, err = h.companyServices.Resource().UpdateProjectResource(ctx, &pb.ProjectResource{
			Id:            current.GetId(),
			ProjectId:     state.ProjectID,
			EnvironmentId: state.EnvironmentID,
			Name:          name,
			Type:          pb.ResourceType_GOOGLE_DRIVE.String(),
			ResourceType:  int32(pb.ResourceType_GOOGLE_DRIVE),
			Settings:      settings,
		})
		return err
	}

	_, err = h.companyServices.Resource().AddResourceToProject(ctx, &pb.AddResourceToProjectRequest{
		Name:          "Google Drive",
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          pb.ResourceType_GOOGLE_DRIVE,
		Settings:      settings,
	})
	return err
}

func (h *HandlerV1) ensureGoogleDriveFolderMenuForExistingResource(ctx context.Context, state googleDriveOAuthState) error {
	list, err := h.companyServices.Resource().GetProjectResourceList(ctx, &pb.GetProjectResourceListRequest{
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		Type:          pb.ResourceType_GOOGLE_DRIVE,
	})
	if err != nil {
		if fileupload.IsUnsupportedGoogleDriveResourceTypeError(err) {
			return nil
		}
		return err
	}
	if len(list.GetResources()) == 0 {
		return nil
	}
	if len(list.GetResources()) > 1 {
		return errors.New("multiple google drive resources configured for project environment")
	}

	settings := list.GetResources()[0].GetSettings()
	if settings == nil || settings.GetGoogleDrive() == nil {
		return nil
	}

	folderID := strings.TrimSpace(settings.GetGoogleDrive().GetFolderId())
	if folderID == "" {
		return nil
	}

	return h.ensureGoogleDriveFolderMenu(ctx, state, folderID)
}

func (h *HandlerV1) ensureGoogleDriveFolderMenu(ctx context.Context, state googleDriveOAuthState, folderID string) error {
	resource, err := h.companyServices.ServiceResource().GetSingle(ctx, &pb.GetSingleServiceResourceReq{
		ProjectId:     state.ProjectID,
		EnvironmentId: state.EnvironmentID,
		ServiceType:   pb.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		return err
	}

	services, err := h.GetProjectSrvc(ctx, state.ProjectID, resource.NodeType)
	if err != nil {
		return err
	}

	attributes, err := helperFunc.ConvertMapToStruct(map[string]any{
		"label_en":  "Google Drive",
		"label_ru":  "Google Drive",
		"path":      fileupload.DriveStorageName,
		"storage":   fileupload.DriveStorageName,
		"folder_id": folderID,
	})
	if err != nil {
		return err
	}

	menuID := googleDriveFolderMenuID(state.ProjectID, state.EnvironmentID)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		menuClient := services.GetBuilderServiceByType(resource.NodeType).Menu()
		if _, err := menuClient.GetByID(ctx, &obs.MenuPrimaryKey{
			Id:        menuID,
			ProjectId: resource.ResourceEnvironmentId,
		}); err == nil {
			return nil
		}

		_, err = menuClient.Create(ctx, &obs.CreateMenuRequest{
			Id:         menuID,
			Label:      "Google Drive",
			Icon:       "",
			ParentId:   gatewayConfig.MainFolderID,
			Type:       googleDriveFolderMenuType,
			ProjectId:  resource.ResourceEnvironmentId,
			NewRouter:  true,
			Attributes: attributes,
		})
		return err

	case pb.ResourceType_POSTGRESQL:
		menuClient := services.GoObjectBuilderService().Menu()
		if _, err := menuClient.GetByID(ctx, &nb.MenuPrimaryKey{
			Id:        menuID,
			ProjectId: resource.ResourceEnvironmentId,
		}); err == nil {
			return nil
		}

		_, err = menuClient.Create(ctx, &nb.CreateMenuRequest{
			Id:         menuID,
			Label:      "Google Drive",
			Icon:       "",
			ParentId:   gatewayConfig.MainFolderID,
			Type:       googleDriveFolderMenuType,
			ProjectId:  resource.ResourceEnvironmentId,
			NewRouter:  true,
			Attributes: attributes,
		})
		return err
	}

	return nil
}

func googleDriveFolderMenuID(projectID, environmentID string) string {
	return uuid.NewSHA1(uuid.NameSpaceOID, []byte("google-drive-folder:"+projectID+":"+environmentID)).String()
}

func (h *HandlerV1) storeGoogleDriveOAuthState(ctx context.Context, state string, payload googleDriveOAuthState) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	key := googleDriveOAuthStatePrefix + state
	if h.centralRedis != nil {
		return h.centralRedis.Set(ctx, key, body, googleDriveOAuthStateTTL).Err()
	}
	if h.cache != nil {
		h.cache.Add(key, body, googleDriveOAuthStateTTL)
		return nil
	}
	return errors.New("oauth state storage is not configured")
}

func (h *HandlerV1) popGoogleDriveOAuthState(ctx context.Context, state string) (googleDriveOAuthState, error) {
	var payload googleDriveOAuthState
	key := googleDriveOAuthStatePrefix + state

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

func (h *HandlerV1) redirectGoogleDriveOAuth(c *gin.Context, success bool, reason string, states ...googleDriveOAuthState) {
	var state googleDriveOAuthState
	if len(states) > 0 {
		state = states[0]
	}

	// Preferred path: send the popup back to the frontend origin that opened it
	// (captured at connect time). It lands on the lightweight
	// google-drive-success/error page on the SAME app, which postMessages its
	// opener and self-closes — exactly like the GitHub/Bitbucket flow. This
	// avoids surfacing an unrelated dashboard when the configured frontend URL
	// points at another origin or at the project page.
	if origin := strings.TrimSpace(state.FrontendOrigin); origin != "" {
		if target := buildGoogleDriveOriginRedirect(origin, success, reason, state); target != "" {
			c.Redirect(http.StatusTemporaryRedirect, target)
			return
		}
	}

	// Fallback: the origin could not be determined — use the configured frontend
	// URL as before.
	target := h.baseConf.GoogleDriveFrontendErrorURL
	if success {
		target = h.baseConf.GoogleDriveFrontendSuccessURL
	}
	target = strings.TrimSpace(target)
	hasConfiguredTarget := target != ""
	if target == "" {
		if success {
			target = "/?google_drive=success"
		} else {
			target = "/?google_drive=error"
		}
	}

	if len(states) > 0 {
		target = applyGoogleDriveRedirectPlaceholders(target, state)
	}

	if hasConfiguredTarget {
		c.Redirect(http.StatusTemporaryRedirect, target)
		return
	}

	u, err := url.Parse(target)
	if err == nil {
		q := u.Query()
		if success {
			q.Set("google_drive", "success")
		} else {
			q.Set("google_drive", "error")
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

// resolveGoogleDriveFrontendOrigin derives the origin (scheme://host) of the app
// that initiated the OAuth flow from the browser-set Origin header, falling back
// to the Referer. Both are set by the browser on the connect XHR, so they
// identify the real caller and are safe to redirect back to (not user-spoofable
// for a victim's session). Returns "" when neither yields a valid http(s) origin.
func resolveGoogleDriveFrontendOrigin(c *gin.Context) string {
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

// buildGoogleDriveOriginRedirect builds the popup redirect URL on the captured
// frontend origin, pointing at the lightweight success/error close-page and
// carrying the OAuth result + ids as query params. Returns "" if the URL can't
// be built so the caller can fall back to the configured target.
func buildGoogleDriveOriginRedirect(origin string, success bool, reason string, state googleDriveOAuthState) string {
	path := googleDriveErrorPath
	if success {
		path = googleDriveSuccessPath
	}

	u, err := url.Parse(strings.TrimRight(origin, "/") + path)
	if err != nil {
		return ""
	}

	q := u.Query()
	if success {
		q.Set("google_drive", "success")
	} else {
		q.Set("google_drive", "error")
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

func applyGoogleDriveRedirectPlaceholders(target string, state googleDriveOAuthState) string {
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

func newGoogleDriveOAuthState() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generate oauth state: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(bytes), nil
}

func googleDriveVisibility(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "private"
	}
	return value
}

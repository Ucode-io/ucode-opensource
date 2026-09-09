package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ExtGitlabConnect proxies to the function service to initiate external GitLab OAuth.
//
// @Security ApiKeyAuth
// @ID ext_gitlab_connect
// @Router /v1/ext-gitlab/connect [GET]
// @Summary Initiate external GitLab OAuth
// @Tags External GitLab Integration
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) ExtGitlabConnect(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// ExtGitlabCallback proxies the external GitLab OAuth callback to the function service.
// This endpoint is public — GitLab redirects the user here after authorization.
//
// @ID ext_gitlab_callback
// @Router /v1/ext-gitlab/callback [GET]
// @Summary External GitLab OAuth Callback
// @Tags External GitLab Integration
// @Param code  query string true "Authorization code from GitLab"
// @Param state query string true "CSRF state token"
// @Success 307 "Redirect to frontend"
func (h *HandlerV1) ExtGitlabCallback(c *gin.Context) {
	target := h.baseConf.GoFunctionServiceHost + h.baseConf.GoFunctionServiceHTTPPort
	err := h.MakeProxy(c, target, c.Request.URL.Path)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?reason=proxy_error")
	}
}

// ExtGitlabGetIntegration proxies to the function service to get the stored external GitLab integration.
//
// @Security ApiKeyAuth
// @ID ext_gitlab_get_integration
// @Router /v1/ext-gitlab/integration [GET]
// @Summary Get External GitLab Integration
// @Tags External GitLab Integration
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
func (h *HandlerV1) ExtGitlabGetIntegration(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// ExtGitlabValidateToken proxies to the function service to validate the stored external GitLab token.
//
// @Security ApiKeyAuth
// @ID ext_gitlab_validate_token
// @Router /v1/ext-gitlab/integration/validate [GET]
// @Summary Validate stored external GitLab token
// @Tags External GitLab Integration
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]any
func (h *HandlerV1) ExtGitlabValidateToken(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// ExtGitlabDeleteIntegration proxies to the function service to delete an external GitLab integration.
//
// @Security ApiKeyAuth
// @ID ext_gitlab_delete_integration
// @Router /v1/ext-gitlab/integration/{id} [DELETE]
// @Summary Delete External GitLab Integration
// @Tags External GitLab Integration
// @Param id path string true "Integration ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) ExtGitlabDeleteIntegration(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// ExtGitlabSyncMicrofrontend proxies to the function service to sync a microfrontend to external GitLab.
//
// @Security ApiKeyAuth
// @ID ext_gitlab_sync_microfrontend
// @Router /v2/functions/micro-frontend/ext-gitlab-sync [POST]
// @Summary Sync microfrontend to external GitLab
// @Description Creates an external GitLab mirror repo, pushes current u-gen files, and registers a webhook.
// @Tags External GitLab Integration
// @Accept  json
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) ExtGitlabSyncMicrofrontend(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

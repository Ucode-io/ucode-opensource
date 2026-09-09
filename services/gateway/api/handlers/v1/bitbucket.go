package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// BitbucketConnect proxies to the function service to initiate Bitbucket OAuth.
//
// @Security ApiKeyAuth
// @ID bitbucket_connect
// @Router /v1/bitbucket/connect [GET]
// @Summary Initiate Bitbucket OAuth
// @Tags Bitbucket Integration
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) BitbucketConnect(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// BitbucketCallback proxies the Bitbucket OAuth callback to the function service.
// This endpoint is public — Bitbucket redirects the user here after authorization.
//
// @ID bitbucket_callback
// @Router /v1/bitbucket/callback [GET]
// @Summary Bitbucket OAuth Callback
// @Tags Bitbucket Integration
// @Param code  query string true "Authorization code from Bitbucket"
// @Param state query string true "CSRF state token"
// @Success 307 "Redirect to frontend"
func (h *HandlerV1) BitbucketCallback(c *gin.Context) {
	target := h.baseConf.GoFunctionServiceHost + h.baseConf.GoFunctionServiceHTTPPort
	err := h.MakeProxy(c, target, c.Request.URL.Path)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?reason=proxy_error")
	}
}

// BitbucketGetIntegration proxies to the function service to get the stored Bitbucket integration.
//
// @Security ApiKeyAuth
// @ID bitbucket_get_integration
// @Router /v1/bitbucket/integration [GET]
// @Summary Get Bitbucket Integration
// @Tags Bitbucket Integration
// @Success 200 {object} map[string]any
// @Failure 404 {object} map[string]any
func (h *HandlerV1) BitbucketGetIntegration(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// BitbucketValidateToken proxies to the function service to validate the stored Bitbucket token.
//
// @Security ApiKeyAuth
// @ID bitbucket_validate_token
// @Router /v1/bitbucket/integration/validate [GET]
// @Summary Validate stored Bitbucket token
// @Tags Bitbucket Integration
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]any
func (h *HandlerV1) BitbucketValidateToken(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// BitbucketWorkspaces proxies to the function service to list accessible Bitbucket workspaces.
//
// @Security ApiKeyAuth
// @ID bitbucket_workspaces
// @Router /v1/bitbucket/workspaces [GET]
// @Summary List Bitbucket Workspaces
// @Tags Bitbucket Integration
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) BitbucketWorkspaces(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// BitbucketDeleteIntegration proxies to the function service to delete a Bitbucket integration.
//
// @Security ApiKeyAuth
// @ID bitbucket_delete_integration
// @Router /v1/bitbucket/integration/{id} [DELETE]
// @Summary Delete Bitbucket Integration
// @Tags Bitbucket Integration
// @Param id path string true "Integration ID"
// @Success 200 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) BitbucketDeleteIntegration(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// BitbucketSyncMicrofrontend proxies to the function service to sync a microfrontend to Bitbucket.
//
// @Security ApiKeyAuth
// @ID bitbucket_sync_microfrontend
// @Router /v2/functions/micro-frontend/bitbucket-sync [POST]
// @Summary Sync microfrontend to Bitbucket
// @Description Creates a Bitbucket mirror repo, pushes current u-gen files, and registers a webhook.
// @Tags Bitbucket Integration
// @Accept  json
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) BitbucketSyncMicrofrontend(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

package v1

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GithubConnect proxies to the function service to initiate GitHub OAuth.
//
// @Security ApiKeyAuth
// @ID github_connect
// @Router /v1/github/connect [GET]
// @Summary Initiate GitHub OAuth
// @Tags GitHub Integration
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) GithubConnect(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GithubCallback proxies the GitHub OAuth callback to the function service.
// This endpoint is public — GitHub redirects the user here after authorization.
//
// @ID github_callback
// @Router /v1/github/callback [GET]
// @Summary GitHub OAuth Callback
// @Tags GitHub Integration
// @Param code  query string true "Authorization code from GitHub"
// @Param state query string true "CSRF state token"
// @Success 307 "Redirect to frontend"
func (h *HandlerV1) GithubCallback(c *gin.Context) {
	target := h.baseConf.GoFunctionServiceHost + h.baseConf.GoFunctionServiceHTTPPort
	err := h.MakeProxy(c, target, c.Request.URL.Path)
	if err != nil {
		c.Redirect(http.StatusTemporaryRedirect, "/?reason=proxy_error")
	}
}

// GithubGetIntegration proxies to the function service to get the stored GitHub integration.
//
// @Security ApiKeyAuth
// @ID github_get_integration
// @Router /v1/github/integration [GET]
// @Summary Get GitHub Integration
// @Tags GitHub Integration
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
func (h *HandlerV1) GithubGetIntegration(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GithubValidateToken proxies to the function service to validate the stored GitHub token.
//
// @Security ApiKeyAuth
// @ID github_validate_token
// @Router /v1/github/integration/validate [GET]
// @Summary Validate stored GitHub token
// @Tags GitHub Integration
// @Success 200 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Failure 404 {object} map[string]any
func (h *HandlerV1) GithubValidateToken(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GithubDeleteIntegration proxies to the function service to delete a GitHub integration.
//
// @Security ApiKeyAuth
// @ID github_delete_integration
// @Router /v1/github/integration/{id} [DELETE]
// @Summary Delete GitHub Integration
// @Tags GitHub Integration
// @Param id path string true "Integration ID"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) GithubDeleteIntegration(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GithubCreateRepo proxies to the function service to create a GitHub repository.
//
// @Security ApiKeyAuth
// @ID github_create_repo
// @Router /v1/github/repo [POST]
// @Summary Create GitHub Repository
// @Tags GitHub Integration
// @Accept  json
// @Produce json
// @Success 201 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
func (h *HandlerV1) GithubCreateRepo(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GithubGetRepoList proxies to the function service to list GitHub repositories.
//
// @Security ApiKeyAuth
// @ID github_list_repos
// @Router /v1/github/repos [GET]
// @Summary List GitHub Repositories
// @Tags GitHub Integration
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 404 {object} map[string]any
func (h *HandlerV1) GithubGetRepoList(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GithubSyncMicrofrontend proxies to the function service to sync a microfrontend to GitHub.
//
// @Security ApiKeyAuth
// @ID github_sync_microfrontend
// @Router /v2/functions/micro-frontend/github-sync [POST]
// @Summary Sync microfrontend to GitHub
// @Description Creates a GitHub mirror repo, pushes current u-gen files, and registers a webhook.
// @Tags GitHub Integration
// @Accept  json
// @Produce json
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 500 {object} map[string]any
func (h *HandlerV1) GithubSyncMicrofrontend(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

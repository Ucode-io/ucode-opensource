package v2

import (
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
)

// Gitlab godoc
// @ID gitlab_get_branches
// @Router /gitlab/branches [GET]
// @Summary Gitlab Branches
// @Description Gitlab Branches
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param token query string true "token"
// @Param project_id query string true "project_id"
// @Success 201 {object} status_http.Response{data=object} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabGetBranches(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// Gitlab godoc
// @ID gitlab_get_repos
// @Router /gitlab/repos [GET]
// @Summary Gitlab Repo
// @Description Gitlab Repo
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Success 201 {object} status_http.Response{data=object} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabGetRepos(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// Gitlab godoc
// @ID gitlab_get_user
// @Router /gitlab/user [GET]
// @Summary Gitlab User
// @Description Gitlab User
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param token query string false "token"
// @Success 201 {object} status_http.Response{data=object} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabGetUser(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// Gitlab godoc
// @ID gitlab_login
// @Router /gitlab/login [GET]
// @Summary Gitlab Login
// @Description Gitlab Login
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param code query string false "code"
// @Success 201 {object} status_http.Response{data=string} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabLogin(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GitlabGetTree godoc
// @Security ApiKeyAuth
// @ID gitlab_get_tree
// @Router /gitlab/tree [GET]
// @Summary Gitlab Get Repository Tree
// @Description Get all files and folders in a GitLab repository recursively
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param branch query string false "branch name (default: master)"
// @Success 200 {object} status_http.Response{data=[]models.GitlabTreeItem} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabGetTree(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GitlabGetFile godoc
// @Security ApiKeyAuth
// @ID gitlab_get_file
// @Router /gitlab/file [GET]
// @Summary Gitlab Get File Content
// @Description Get file content from a GitLab repository
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param file_path query string true "file path in the repository"
// @Param branch query string false "branch name (default: master)"
// @Success 200 {object} status_http.Response{data=string} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabGetFile(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GitlabUpdateFile godoc
// @Security ApiKeyAuth
// @ID gitlab_update_file
// @Router /gitlab/file [PUT]
// @Summary Gitlab Update File Content
// @Description Update (commit and push) file content to a GitLab repository
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param body body models.GitlabUpdateFileRequest true "GitlabUpdateFileRequest"
// @Success 200 {object} status_http.Response{data=string} "Data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabUpdateFile(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// GitlabGetPipelineStatus godoc
// @Security ApiKeyAuth
// @ID gitlab_get_pipeline_status
// @Router /gitlab/pipeline [GET]
// @Summary Gitlab Get Pipeline Status
// @Description Get the latest pipeline status for a GitLab repository
// @Tags Gitlab
// @Accept json
// @Produce json
// @Param project_id query string true "gitlab numeric project id"
// @Param branch query string false "branch name (default: master)"
// @Success 200 {object} status_http.Response{data=string} "Data"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GitlabGetPipelineStatus(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

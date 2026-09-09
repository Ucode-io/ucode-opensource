package v2

import (
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
)

//// Github godoc
//// @ID github_get_branches
//// @Router /github/branches [GET]
//// @Summary Github Branches
//// @Description Github Branches
//// @Tags Github
//// @Accept json
//// @Produce json
//// @Param token query string false "token"
//// @Param username query string false "username"
//// @Success 201 {object} status_http.Response{data=models.GithubRepo} "Data"
//// @Response 400 {object} status_http.Response{data=string} "Bad Request"
//// @Failure 500 {object} status_http.Response{data=string} "Server Error"
//func (h *HandlerV2) GithubGetBranches(c *gin.Context) {
//	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
//}
//
//// Github godoc
//// @ID github_get_repos
//// @Router /github/repos [GET]
//// @Summary Github Repo
//// @Description Github Repo
//// @Tags Github
//// @Accept json
//// @Produce json
//// @Param token query string false "token"
//// @Param username query string false "username"
//// @Success 201 {object} status_http.Response{data=models.GithubRepo} "Data"
//// @Response 400 {object} status_http.Response{data=string} "Bad Request"
//// @Failure 500 {object} status_http.Response{data=string} "Server Error"
//func (h *HandlerV2) GithubGetRepos(c *gin.Context) {
//	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
//}
//
//// Github godoc
//// @ID github_get_user
//// @Router /github/user [GET]
//// @Summary Github User
//// @Description Github User
//// @Tags Github
//// @Accept json
//// @Produce json
//// @Param token query string false "token"
//// @Success 201 {object} status_http.Response{data=models.GithubUser} "Data"
//// @Response 400 {object} status_http.Response{data=string} "Bad Request"
//// @Failure 500 {object} status_http.Response{data=string} "Server Error"
//func (h *HandlerV2) GithubGetUser(c *gin.Context) {
//	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
//}
//
//// Github godoc
//// @ID github_login
//// @Router /github/login [GET]
//// @Summary Github Login
//// @Description Github Login
//// @Tags Github
//// @Accept json
//// @Produce json
//// @Param code query number false "code"
//// @Success 201 {object} status_http.Response{data=string} "Data"
//// @Response 400 {object} status_http.Response{data=string} "Bad Request"
//// @Failure 500 {object} status_http.Response{data=string} "Server Error"
//func (h *HandlerV2) GithubLogin(c *gin.Context) {
//	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
//}
//
//func (h *HandlerV2) CreateWebhook(c *gin.Context) {
//	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
//}
//
//func (h *HandlerV2) HandleWebhook(c *gin.Context) {
//	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
//}

package v2

import (
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	_ "github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
)

// Grafana godoc
// @Security ApiKeyAuth
// @ID grafana_function_logs
// @Router /v2/grafana/loki [POST]
// @Summary Grafana Function Logs
// @Description Grafana Function Logs
// @Tags Grafana
// @Accept json
// @Produce json
// @Param Grafana body models.GetGrafanaFunctionLogRequest true "GetGrafanaFunctionLogRequest"
// @Success 200 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetGrafanaFunctionLogs(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

// Grafana godoc
// @Security ApiKeyAuth
// @ID grafana_function_list
// @Router /v2/grafana/function [POST]
// @Summary Grafana Function List
// @Description Grafana Function List
// @Tags Grafana
// @Accept json
// @Produce json
// @Param start query string true "start"
// @Param end query string true "end"
// @Success 200 {object} status_http.Response{data=string} "Success"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetGrafanaFunctionList(c *gin.Context) {
	_ = h.MakeProxy(c, h.baseConf.GoFunctionServiceHost+h.baseConf.GoFunctionServiceHTTPPort, c.Request.URL.Path)
}

package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
)

// Ping godoc
// @ID ping
// @Router /ping [GET]
// @Summary returns "pong" message
// @Description this returns "pong" messsage to show service is working
// @Accept json
// @Produce json
// @Param service query string false "service"
// @Param environment_id query string false "environment_id"
// @Param project_id query string false "project_id"
// @Success 200 {object} status_http.Response{data=string} "Response data"
// @Failure 500 {object} status_http.Response{}
func (h *HandlerV1) Ping(c *gin.Context) {
	var (
		service       = c.Query("service")
		projectId     = c.Query("project_id")
		environmentId = c.Query("environment_id")
		limit         = 10
		// offset        = 0
	)

	switch service {
	case "company_service":
		_, err := h.companyServices.CompanyPing().Ping(c.Request.Context(), &pb.PingRequest{})
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	case "auth_service":
		_, err := h.authService.AuthPing().Ping(c.Request.Context(), &auth_service.PingRequest{})
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

	case "object_builder_service":
		resource, err := h.companyServices.ServiceResource().GetSingle(
			c.Request.Context(), &pb.GetSingleServiceResourceReq{
				ProjectId:     projectId,
				EnvironmentId: environmentId,
				ServiceType:   pb.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		services, err := h.GetProjectSrvc(c.Request.Context(), projectId, resource.NodeType)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		_, err = services.GetBuilderServiceByType(resource.NodeType).Function().GetList(
			c.Request.Context(), &obs.GetAllFunctionsRequest{
				Search:    c.DefaultQuery("search", ""),
				Limit:     int32(limit),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

	case "function_service":
	}

	h.HandleResponse(c, status_http.OK, "pong")
}

func (h *HandlerV1) GetConfig(c *gin.Context) {
	switch h.baseConf.Environment {
	case config.DebugMode:
		h.HandleResponse(c, status_http.OK, h.baseConf)
		return
	case config.TestMode:
		h.HandleResponse(c, status_http.OK, h.baseConf)
		return
	case config.ReleaseMode:
		h.HandleResponse(c, status_http.OK, "private data")
		return
	}

	h.HandleResponse(c, status_http.BadEnvironment, "wrong environment value passed")
}

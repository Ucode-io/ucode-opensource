package handlers

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"

	"github.com/gin-gonic/gin"
)

// GetAllResourceEnvironments godoc
// @ID get_environments
// @Router /v2/resource-environment [GET]
// @Summary Get Environments
// @Description Get Environments
// @Tags V2_Environment
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Success 200 {object} http.Response{data=pb.GetListConfiguredResourceEnvironmentRes} "Environment data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetAllResourceEnvironments(c *gin.Context) {
	var (
		res *pb.GetListConfiguredResourceEnvironmentRes = &pb.GetListConfiguredResourceEnvironmentRes{
			Data: []*pb.GetListConfiguredResourceEnvironmentResResourceEnvironment{},
		}
	)
	
	resp, err := h.services.ResourceService().GetListConfiguredResourceEnvironment(
		c.Request.Context(),
		&pb.GetListConfiguredResourceEnvironmentReq{
			ProjectId: c.DefaultQuery("project-id", ""),
		},
	)

	for _, item := range resp.GetData() {
		if item.GetServiceType() == int32(pb.ServiceType_BUILDER_SERVICE.Number()) {
			res.Data = append(res.Data, item)
		}
	}

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, res)
}

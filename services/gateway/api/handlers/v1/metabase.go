package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"

	"github.com/gin-gonic/gin"
)

func (h *HandlerV1) GetMetabaseDashboards(c *gin.Context) {
	var request pb.GetMetabaseDashboardsRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Visualization().GetMetabaseDashboards(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) GetMetabasePublicUrl(c *gin.Context) {
	var request pb.GetMetabasePublicUrlRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Visualization().GetMetabasePublicUrl(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

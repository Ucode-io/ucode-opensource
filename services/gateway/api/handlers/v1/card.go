package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

func (h *HandlerV1) GetVerifyCode(c *gin.Context) {
	var request pb.GetVerifyCodeRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	request.ProjectId = projectId.(string)
	resp, err := h.companyServices.Billing().GetVerifyCode(c, &request)
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) Verify(c *gin.Context) {
	var request pb.VerifyRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	request.ProjectId = projectId.(string)
	resp, err := h.companyServices.Billing().Verify(c, &request)
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

func (h *HandlerV1) GetAllProjectCards(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	response, err := h.companyServices.Billing().ListProjectCards(c, &pb.ListRequest{
		Offset:    int32(offset),
		Limit:     int32(limit),
		ProjectId: projectId.(string),
	})
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) ReceiptPay(c *gin.Context) {
	var request pb.ReceiptPayRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	userId, _ := c.Get("user_id")

	request.ProjectId = projectId.(string)
	request.UserId = userId.(string)
	response, err := h.companyServices.Billing().ReceiptPay(c, &request)
	if err != nil {
		h.handleError(c, status_http.GRPCError, err)
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) DeleteProjectCard(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, config.ErrIdValid)
		return
	}

	response, err := h.companyServices.Billing().DeleteProjectCard(c, &pb.PrimaryKey{Id: id})
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	h.HandleResponse(c, status_http.NoContent, response)
}

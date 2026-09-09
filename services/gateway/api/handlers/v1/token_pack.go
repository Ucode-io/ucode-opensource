package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// projectCompanyID resolves the company a project belongs to. The token-pack pool
// is company-scoped, but requests authenticate at the project level, so every
// pack endpoint maps project_id -> company_id before calling company-service.
func (h *HandlerV1) projectCompanyID(c *gin.Context, projectId string) (string, error) {
	project, err := h.companyServices.Project().GetById(c, &pb.GetProjectByIdRequest{ProjectId: projectId})
	if err != nil {
		return "", err
	}
	return project.GetCompanyId(), nil
}

// ListTokenPacks godoc
// @Security ApiKeyAuth
// @ID list-token-packs
// @Router /v1/token-pack [GET]
// @Summary List token packs
// @Description List the token-pack catalog. Pass only_active=true to hide inactive packs (user-facing store).
// @Tags TokenPack
// @Accept json
// @Produce json
// @Param only_active query bool false "only active packs"
// @Param product_type query string false "ucode | ugen"
// @Success 200 {object} status_http.Response{data=pb.ListTokenPacksResponse} "Token packs"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ListTokenPacks(c *gin.Context) {
	response, err := h.companyServices.Billing().ListTokenPacks(c, &pb.ListTokenPacksRequest{
		ProductType: c.Query("product_type"),
		OnlyActive:  cast.ToBool(c.Query("only_active")),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// GetTokenPackBalance godoc
// @Security ApiKeyAuth
// @ID get-token-pack-balance
// @Router /v1/token-pack/balance [GET]
// @Summary Get company token-pack balance
// @Description Remaining add-on token-pack tokens for the authenticated project's company.
// @Tags TokenPack
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response{data=pb.GetTokenPackBalanceResponse} "Balance"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetTokenPackBalance(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(cast.ToString(projectId)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	companyId, err := h.projectCompanyID(c, cast.ToString(projectId))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	response, err := h.companyServices.Billing().GetTokenPackBalance(c, &pb.GetTokenPackBalanceRequest{CompanyId: companyId})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// PurchaseTokenPack godoc
// @Security ApiKeyAuth
// @ID purchase-token-pack
// @Router /v1/token-pack/purchase [POST]
// @Summary Purchase a token pack
// @Description Buys a token pack: charges the project's balance and credits the company token-pack pool.
// @Tags TokenPack
// @Accept json
// @Produce json
// @Param body body models.PurchaseTokenPackBody true "pack to buy"
// @Success 200 {object} status_http.Response{data=pb.PurchaseTokenPackResponse} "Purchase result"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) PurchaseTokenPack(c *gin.Context) {
	var body struct {
		PackId string `json:"pack_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(body.PackId) {
		h.HandleResponse(c, status_http.BadRequest, "invalid pack_id")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(cast.ToString(projectId)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}
	userId, _ := c.Get("user_id")

	companyId, err := h.projectCompanyID(c, cast.ToString(projectId))
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	response, err := h.companyServices.Billing().PurchaseTokenPack(c, &pb.PurchaseTokenPackRequest{
		CompanyId: companyId,
		ProjectId: cast.ToString(projectId),
		PackId:    body.PackId,
		UserId:    cast.ToString(userId),
	})
	if err != nil {
		// Map company-service domain errors to distinct HTTP codes so the UI can
		// tell "no funds" (402) from "pack gone" (404) from a real server fault.
		st, _ := status.FromError(err)
		switch st.Code() {
		case codes.NotFound:
			h.HandleResponse(c, status_http.NotFound, st.Message())
		case codes.FailedPrecondition:
			h.HandleResponse(c, status_http.PaymentRequired, st.Message())
		case codes.InvalidArgument:
			h.HandleResponse(c, status_http.InvalidArgument, st.Message())
		default:
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		}
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// CreateTokenPack godoc
// @Security ApiKeyAuth
// @ID create-token-pack
// @Router /v1/token-pack [POST]
// @Summary Create token pack (admin)
// @Description Creates a token-pack catalog entry.
// @Tags TokenPack
// @Accept json
// @Produce json
// @Param body body pb.CreateTokenPackRequest true "token pack"
// @Success 201 {object} status_http.Response{data=pb.TokenPack} "Created pack"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateTokenPack(c *gin.Context) {
	var request pb.CreateTokenPackRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().CreateTokenPack(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// UpdateTokenPack godoc
// @Security ApiKeyAuth
// @ID update-token-pack
// @Router /v1/token-pack [PUT]
// @Summary Update token pack (admin)
// @Description Updates a token-pack catalog entry (full replace).
// @Tags TokenPack
// @Accept json
// @Produce json
// @Param body body pb.TokenPack true "token pack"
// @Success 200 {object} status_http.Response{data=pb.TokenPack} "Updated pack"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateTokenPack(c *gin.Context) {
	var request pb.TokenPack
	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	if !util.IsValidUUID(request.GetId()) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	response, err := h.companyServices.Billing().UpdateTokenPack(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// DeleteTokenPack godoc
// @Security ApiKeyAuth
// @ID delete-token-pack
// @Router /v1/token-pack/{id} [DELETE]
// @Summary Delete token pack (admin)
// @Description Soft-deletes a token-pack catalog entry.
// @Tags TokenPack
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteTokenPack(c *gin.Context) {
	id := c.Param("id")
	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	response, err := h.companyServices.Billing().DeleteTokenPack(c, &pb.PrimaryKey{Id: id})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, response)
}
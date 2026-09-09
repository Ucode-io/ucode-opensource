package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// CreateFare godoc
// @Security ApiKeyAuth
// @ID create-fare
// @Router /v1/fare [POST]
// @Summary Create fare
// @Description Create fare
// @Tags Billing
// @Accept json
// @Produce json
// @Param billing body pb.CreateFareRequest true "FareCreateRequest"
// @Success 201 {object} status_http.Response{data=pb.Fare} "Fare data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateFare(c *gin.Context) {
	var request pb.CreateFareRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().CreateFare(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// ListFares godoc
// @Security ApiKeyAuth
// @ID list-fares
// @Router /v1/fare [GET]
// @Summary List fares
// @Description List fares
// @Tags Billing
// @Accept json
// @Produce json
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Param product_type query string false "product_type"
// @Success 200 {object} status_http.Response{data=pb.Fare} "Fares data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllFares(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	productType := c.Query("product_type")

	response, err := h.companyServices.Billing().ListFares(c, &pb.ListRequest{
		Offset:      int32(offset),
		Limit:       int32(limit),
		ProjectId:   projectId,
		ProductType: productType,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// GetFare godoc
// @Security ApiKeyAuth
// @ID get-fare
// @Router /v1/fare/{id} [GET]
// @Summary Get fare
// @Description Get fare
// @Tags Billing
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=pb.Fare} "Fare data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetFare(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	response, err := h.companyServices.Billing().GetFare(c, &pb.PrimaryKey{Id: id, ProjectId: projectId.(string)})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// UpdateFare godoc
// @Security ApiKeyAuth
// @ID update-fare
// @Router /v1/fare [PUT]
// @Summary Update fare
// @Description Update fare
// @Tags Billing
// @Accept json
// @Produce json
// @Param billing body pb.Fare true "Fare"
// @Success 200 {object} status_http.Response{data=pb.Fare} "Fare data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateFare(c *gin.Context) {
	var request pb.Fare

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().UpdateFare(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// DeleteFare godoc
// @Security ApiKeyAuth
// @ID delete-fare
// @Router /v1/fare/{id} [DELETE]
// @Summary Delete fare
// @Description Delete fare
// @Tags Billing
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteFare(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	response, err := h.companyServices.Billing().DeleteFare(c, &pb.PrimaryKey{Id: id})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, response)
}

// CreateFareItem godoc
// @Security ApiKeyAuth
// @ID create-fare-item
// @Router /v1/fare/item [POST]
// @Summary Create fare item
// @Description Create fare item
// @Tags Billing
// @Accept json
// @Produce json
// @Param billing body pb.CreateFareItemRequest true "FareItemCreateRequest"
// @Success 201 {object} status_http.Response{data=pb.FareItem} "FareItem data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateFareItem(c *gin.Context) {
	var request pb.CreateFareItemRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().CreateFareItem(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// ListFareItems godoc
// @Security ApiKeyAuth
// @ID list-fare-items
// @Router /v1/fare/item [GET]
// @Summary List fare items
// @Description List fare items
// @Tags Billing
// @Accept json
// @Produce json
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Success 200 {object} status_http.Response{data=pb.FareItem} "FareItems data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllFareItem(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().ListFareItems(c, &pb.ListRequest{
		Offset: int32(offset),
		Limit:  int32(limit),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// GetFareItem godoc
// @Security ApiKeyAuth
// @ID get-fare-item
// @Router /v1/fare/item/{id} [GET]
// @Summary Get fare item
// @Description Get fare item
// @Tags Billing
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=pb.FareItem} "FareItem data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetFareItem(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	response, err := h.companyServices.Billing().GetFareItem(c, &pb.PrimaryKey{Id: id})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// UpdateFareItem godoc
// @Security ApiKeyAuth
// @ID update-fare-item
// @Router /v1/fare/item [PUT]
// @Summary Update fare item
// @Description Update fare item
// @Tags Billing
// @Accept json
// @Produce json
// @Param FareItem body pb.FareItem true "FareItem"
// @Success 200 {object} status_http.Response{data=pb.FareItem} "FareItem data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateFareItem(c *gin.Context) {
	var request pb.FareItem

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().UpdateFareItem(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// DeleteFareItem godoc
// @Security ApiKeyAuth
// @ID delete-fare-item
// @Router /v1/fare/item/{id} [DELETE]
// @Summary Delete fare item
// @Description Delete fare item
// @Tags Billing
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteFareItem(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	response, err := h.companyServices.Billing().DeleteFareItem(c, &pb.PrimaryKey{Id: id})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, response)
}

// CreateTransaction godoc
// @Security ApiKeyAuth
// @ID create-transaction
// @Router /v1/transaction [POST]
// @Summary Create transaction
// @Description Create transaction
// @Tags Billing
// @Accept json
// @Produce json
// @Param transaction body pb.CreateTransactionRequest true "TransactionCreateRequest"
// @Success 201 {object} status_http.Response{data=pb.Transaction} "Transaction data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateTransaction(c *gin.Context) {
	var request pb.CreateTransactionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().CreateTransaction(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// GetTransaction godoc
// @Security ApiKeyAuth
// @ID get-transaction
// @Router /v1/transaction/{id} [GET]
// @Summary Get transaction
// @Description Get transaction
// @Tags Billing
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=pb.Transaction} "Transaction data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetTransaction(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.BadRequest, "invalid id")
		return
	}

	response, err := h.companyServices.Billing().GetTransaction(c, &pb.PrimaryKey{Id: id})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ListTransactions godoc
// @Security ApiKeyAuth
// @ID list-transactions
// @Router /v1/transaction [GET]
// @Summary List transactions
// @Description List transactions
// @Tags Billing
// @Accept json
// @Produce json
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Success 200 {object} status_http.Response{data=pb.Transaction} "Transactions data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllTransactions(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var (
		request = &pb.ListRequest{
			Offset: int32(offset),
			Limit:  int32(limit),
		}

		isAll = c.Query("all")
	)

	if isAll != "true" {
		request.ProjectId = projectId.(string)
	}

	response, err := h.companyServices.Billing().ListTransactions(c, request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// UpdateTransaction godoc
// @Security ApiKeyAuth
// @ID update-transaction
// @Router /v1/transaction [PUT]
// @Summary Update transaction
// @Description Update transaction
// @Tags Billing
// @Accept json
// @Produce json
// @Param transaction body pb.Transaction true "Transaction"
// @Success 200 {object} status_http.Response{data=pb.Transaction} "Transaction data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateTransaction(c *gin.Context) {
	var request pb.Transaction

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().UpdateTransaction(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// CreateFare godoc
// @Security ApiKeyAuth
// @ID calculate_price
// @Router /v1/fare/calculate-price [POST]
// @Summary Calculate price
// @Description Calculate price
// @Tags Billing
// @Accept json
// @Produce json
// @Param billing body pb.CalculatePriceRequest true "CalculatePriceRequest"
// @Success 201 {object} status_http.Response{data=pb.CalculatePriceResponse} "CalculatePriceResponse data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CalculatePrice(c *gin.Context) {
	var request pb.CalculatePriceRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().CalculatePrice(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// ListDiscounts godoc
// @Security ApiKeyAuth
// @ID list-discounts
// @Router /v1/discounts [GET]
// @Summary List discounts
// @Description List discounts
// @Tags Billing
// @Accept json
// @Produce json
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Success 201 {object} status_http.Response{data=pb.ListDiscountsResponse} "Fare data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ListDiscounts(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response, err := h.companyServices.Billing().ListDiscounts(c, &pb.ListRequest{
		Offset:    int32(offset),
		Limit:     int32(limit),
		ProjectId: projectId.(string),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ListBillingPeriods godoc
// @Security ApiKeyAuth
// @Router /v1/billing-periods [GET]
// @Summary List billing periods
// @Description List Ugen billing periods
// @Tags Subscription
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response{data=pb.ListBillingPeriodsResponse} "Billing periods"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ListBillingPeriods(c *gin.Context) {
	productType := c.DefaultQuery("product_type", "ugen")

	response, err := h.companyServices.Billing().ListBillingPeriods(c, &pb.ListBillingPeriodsRequest{
		ProductType: productType,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// UpdateSubscriptionEndDate godoc
// @Security ApiKeyAuth
// @Router /v1/subscription [PUT]
// @Summary Update subscription date
// @Description Update subscription date
// @Tags Subscription
// @Accept json
// @Produce json
// @Success 201 {object} status_http.Response{data=pb.ListDiscountsResponse} "Fare data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateSubscriptionEndDate(c *gin.Context) {
	var request pb.UpdateSubscriptionEndDateReq

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if !util.IsValidUUID(request.ProjectId) {
		h.HandleResponse(c, status_http.InvalidArgument, "project_id is an invalid uuid")
		return
	}

	response, err := h.companyServices.Billing().UpdateSubscriptionEndDate(c, &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// GetCurrentSubscription godoc
// @Security ApiKeyAuth
// @Router /v1/subscription/current [GET]
// @Summary Get current subscription
// @Description Get current active or pending downgrade subscription for the authenticated project
// @Tags Subscription
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response{data=pb.Subscription} "Subscription"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetCurrentSubscription(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	response, err := h.companyServices.Billing().GetSubscription(c, &pb.PrimaryKey{
		ProjectId: projectId.(string),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// CancelSubscription godoc
// @Security ApiKeyAuth
// @Router /v1/subscription/cancel [PATCH]
// @Summary Cancel subscription at period end
// @Description Schedule active Ugen subscription cancellation at current period end
// @Tags Subscription
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response{data=pb.Subscription} "Subscription"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CancelSubscription(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	response, err := h.companyServices.Billing().CancelSubscription(c, &pb.CancelSubscriptionRequest{
		ProjectId: projectId.(string),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// GetProjectBillingStatus godoc
// @Security ApiKeyAuth
// @Router /v1/billing/status [GET]
// @Summary Get project billing status
// @Description Return the project's billing status and, when the next renewal is within 10 days and the balance cannot cover it, a low-balance warning with the shortfall
// @Tags Billing
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response{data=pb.GetProjectBillingStatusResponse} "Project billing status"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetProjectBillingStatus(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	response, err := h.companyServices.Billing().GetProjectBillingStatus(c, &pb.GetProjectBillingStatusRequest{
		ProjectId: projectId.(string),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

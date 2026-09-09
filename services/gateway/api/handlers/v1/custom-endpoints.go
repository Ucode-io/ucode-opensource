package v1

import (
	"encoding/json"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// ─── ExecQuery (старый raw exec — не трогаем) ─────────────────────────────────

func (h *HandlerV1) ExecQuery(c *gin.Context) {
	var request pbo.ExecuteSQLRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	request.ResourceEnvId = resourceEnvId

	response, err := service.GoObjectBuilderService().ObjectBuilder().ExecuteSQL(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, map[string]any{
		"rows":          response.GetRows(),
		"rows_affected": response.GetRowsAffected(),
		"types":         response.GetTypes(),
	})
}

// ─── CreateCustomEndpoint ─────────────────────────────────────────────────────

func (h *HandlerV1) CreateCustomEndpoint(c *gin.Context) {
	var request pbo.CreateCustomEndpointRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	request.ResourceEnvId = resourceEnvId

	response, err := service.GoObjectBuilderService().CustomEndpoint().Create(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// ─── UpdateCustomEndpoint ─────────────────────────────────────────────────────

func (h *HandlerV1) UpdateCustomEndpoint(c *gin.Context) {
	var request pbo.CustomEndpoint

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	request.ResourceEnvId = resourceEnvId
	request.Id = c.Param("id")

	response, err := service.GoObjectBuilderService().CustomEndpoint().Update(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ─── GetAllCustomEndpoints ────────────────────────────────────────────────────

func (h *HandlerV1) GetAllCustomEndpoints(c *gin.Context) {
	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomEndpoint().GetAll(
		c.Request.Context(),
		&pbo.GetCustomEndpointListRequest{
			ResourceEnvId:  resourceEnvId,
			Search:         c.Query("search"),
			Limit:          cast.ToUint32(c.DefaultQuery("limit", "20")),
			Offset:         cast.ToUint32(c.DefaultQuery("offset", "0")),
			OrderBy:        c.Query("order_by"),
			OrderDirection: c.Query("order_direction"),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ─── GetCustomEndpointById ────────────────────────────────────────────────────

func (h *HandlerV1) GetCustomEndpointById(c *gin.Context) {
	id := c.Param("id")

	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomEndpoint().GetById(
		c.Request.Context(),
		&pbo.CustomEndpointId{
			ResourceEnvId: resourceEnvId,
			Id:            id,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ─── DeleteCustomEndpoint ─────────────────────────────────────────────────────

func (h *HandlerV1) DeleteCustomEndpoint(c *gin.Context) {
	id := c.Param("id")

	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomEndpoint().Delete(
		c.Request.Context(),
		&pbo.CustomEndpointId{
			ResourceEnvId: resourceEnvId,
			Id:            id,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ─── RunCustomEndpoint ────────────────────────────────────────────────────────
// Логика:
//  1. Достать endpoint по id → получить SQL + in_transaction
//  2. Принять params из body: {"params": ["val1", "val2"]}
//  3. Вызвать ExecuteSQL с сохранённым SQL
//  4. Вернуть результат

type runCustomEndpointRequest struct {
	Params map[string]string `json:"params"`
}

func (h *HandlerV1) RunCustomEndpoint(c *gin.Context) {
	id := c.Param("id")

	// 1. Получить сервис и resource_env_id
	service, resourceEnvId, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	// 2. Получить params из body
	var body runCustomEndpointRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		// params не обязательны
		body.Params = make(map[string]string)
	}

	if body.Params == nil {
		body.Params = make(map[string]string)
	}

	// 3. Выполнить SQL одной gRPC операцией
	response, err := service.GoObjectBuilderService().CustomEndpoint().Run(
		c.Request.Context(),
		&pbo.RunCustomEndpointRequest{
			ResourceEnvId: resourceEnvId,
			Id:            id,
			Params:        body.Params,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if response.GetError() != "" {
		h.HandleResponse(c, status_http.InvalidArgument, response.GetError())
		return
	}

	// Return data directly if it's a valid JSON
	var rawData any
	if err := json.Unmarshal(response.GetData(), &rawData); err == nil {
		h.HandleResponse(c, status_http.OK, rawData)
	} else {
		h.HandleResponse(c, status_http.OK, response)
	}
}

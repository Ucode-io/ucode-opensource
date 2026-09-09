package handlers

import (
	"errors"
	status "github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
)

// CreateApiKey godoc
// @ID create_api_keys
// @Router /v2/api-key/{project-id} [POST]
// @Summary Create ApiKey
// @Description Create ApiKey
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Param api-key body auth_service.CreateReq true "ApiKeyReqBody"
// @Success 201 {object} http.Response{data=auth_service.CreateRes} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) CreateApiKey(c *gin.Context) {
	var apiKey auth_service.CreateReq

	projectId, ok := c.Get("project_id")
	if !ok {
		h.handleResponse(c, status.InvalidArgument, errors.New("project_id is required"))
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok {
		h.handleResponse(c, status.InvalidArgument, errors.New("environment_id is required"))
		return
	}

	err := c.ShouldBindJSON(&apiKey)
	if err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	apiKey.ProjectId = projectId.(string)
	apiKey.EnvironmentId = environmentId.(string)

	res, err := h.services.ApiKeysService().Create(
		c.Request.Context(),
		&apiKey,
	)

	if err != nil {
		if st, ok := grpcstatus.FromError(err); ok && st.Code() == codes.ResourceExhausted {
			h.handleResponse(c, status.PaymentRequired, models.PaymentRequiredData{
				Type: "payment_required",
				Code: "api_key_limit",
				Unit: "api_keys",
			})
			return
		}
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.Created, res)
}

// UpdateApiKey godoc
// @ID update_api_keys
// @Router /v2/api-key/{project-id}/{id} [PUT]
// @Summary Update ApiKey
// @Description Update ApiKey
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Param id path string true "id"
// @Param api-key body auth_service.UpdateReq true "ApiKeyReqBody"
// @Success 201 {object} http.Response{data=auth_service.UpdateRes} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UpdateApiKey(c *gin.Context) {
	var apiKey auth_service.UpdateReq

	err := c.ShouldBindJSON(&apiKey)
	if err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	apiKey.Id = c.Param("id")

	res, err := h.services.ApiKeysService().Update(
		c.Request.Context(),
		&apiKey,
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, res)
}

// GetApiKey godoc
// @ID get_api_key
// @Router /v2/api-key/{project-id}/{id} [GET]
// @Summary Get ApiKey
// @Description Get ApiKey
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Param project-id path string true "project-id"
// @Success 201 {object} http.Response{data=auth_service.GetRes} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetApiKey(c *gin.Context) {

	res, err := h.services.ApiKeysService().Get(
		c.Request.Context(),
		&auth_service.GetReq{
			Id: c.Param("id"),
		},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, res)
}

// GetListApiKeys godoc
// @ID get_list_api_key
// @Router /v2/api-key/{project-id} [GET]
// @Summary Get List ApiKey
// @Description Get List ApiKey
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Param environment-id query string false "environment-id"
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Param search query string false "search"
// @Param client_type_id query string false "client_type_id"
// @Param role_id query string false "role_id"
// @Success 201 {object} http.Response{data=auth_service.GetListRes} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetListApiKeys(c *gin.Context) {

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok {
		h.handleResponse(c, status.InvalidArgument, errors.New("project_id is required"))
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok {
		h.handleResponse(c, status.InvalidArgument, errors.New("environment_id is required"))
		return
	}

	res, err := h.services.ApiKeysService().GetList(
		c.Request.Context(),
		&auth_service.GetListReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			Offset:        int32(offset),
			Limit:         int32(limit),
			Search:        c.Query("search"),
			ClientTypeId:  c.DefaultQuery("client_type_id", ""),
			RoleId:        c.DefaultQuery("role_id", ""),
		},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, res)
}

// DeleteApiKeys godoc
// @ID delete_api_keys
// @Router /v2/api-key/{project-id}/{id} [DELETE]
// @Summary Delete ApiKeys
// @Description Delete ApiKeys
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Param id path string true "id"
// @Success 201 {object} http.Response{data=auth_service.DeleteRes} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteApiKeys(c *gin.Context) {

	res, err := h.services.ApiKeysService().Delete(
		c.Request.Context(),
		&auth_service.DeleteReq{Id: c.Param("id")},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, res)
}

// GenerateApiKeyToken godoc
// @ID generate_api_key_token
// @Router /v2/api-key/generate-token [POST]
// @Summary Generate Api Key Token
// @Description Generate Api Key Token
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param api-key body auth_service.GenerateApiTokenReq true "ApiKeyReqBody"
// @Success 201 {object} http.Response{data=auth_service.GenerateApiTokenRes} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GenerateApiKeyToken(c *gin.Context) {
	var apiKey auth_service.GenerateApiTokenReq

	err := c.ShouldBindJSON(&apiKey)
	if err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	res, err := h.services.ApiKeysService().GenerateApiToken(
		c.Request.Context(),
		&apiKey,
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, res)
}

// RefreshApiKeyToken godoc
// @ID refresh_api_key_token
// @Router /v2/api-key/refresh-token [POST]
// @Summary Refresh Api Key Token
// @Description Refresh Api Key Token
// @Tags V2_ApiKey
// @Accept json
// @Produce json
// @Param api-key body auth_service.RefreshApiTokenReq true "ApiKeyReqBody"
// @Success 201 {object} http.Response{data=auth_service.RefreshApiTokenReq} "ApiKey data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) RefreshApiKeyToken(c *gin.Context) {
	var apiKey auth_service.RefreshApiTokenReq

	err := c.ShouldBindJSON(&apiKey)
	if err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	res, err := h.services.ApiKeysService().RefreshApiToken(
		c.Request.Context(),
		&apiKey,
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, res)
}

// GetClientPlatformList godoc
// @ID get_client_platform_list
// @Router /v2/client-platform [GET]
// @Summary Get ClientPlatform List
// @Description  Get ClientPlatform List
// @Tags ClientPlatform
// @Accept json
// @Produce json
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Param search query string false "search"
// @Success 200 {object} http.Response{data=auth_service.GetClientPlatformListResponse} "GetClientPlatformListResponseBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetClientPlatformList(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	resp, err := h.services.ClientService().GetClientPlatformList(
		c.Request.Context(),
		&auth_service.GetClientPlatformListRequest{
			Limit:  int32(limit),
			Offset: int32(offset),
			Search: c.Query("search"),
		},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, resp)
}

// GetListClientTokens godoc
// @ID get_list_client_tokens
// @Router /v2/api-key/{project-id}/tokens [GET]
// @Summary Get List Clint Tokens
// @Description Get List Clint Tokens
// @Tags V2_ApiKey_Token
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Param client-id query string false "client-id"
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Success 201 {object} http.Response{data=auth_service.ListClientTokenResponse} "Tokens data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) ListClientTokens(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, status.InvalidArgument, err.Error())
		return
	}

	resp, err := h.services.ApiKeysService().ListClientToken(
		c.Request.Context(), &auth_service.ListClientTokenRequest{
			ClientId: c.Query("client_id"),
			Offset:   int32(offset),
			Limit:    int32(limit),
		},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.OK, resp)
}

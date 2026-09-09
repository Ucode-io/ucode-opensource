package handlers

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	pbCompany "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/gin-gonic/gin"
	"github.com/saidamir98/udevs_pkg/util"
)

// V2CreateConnection godoc
// @ID create_connection_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/connection [POST]
// @Summary Create Connection
// @Description Create Connection
// @Tags V2_Connection
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param connection body models.CreateConnectionRequest true "CreateConnectionRequestBody"
// @Success 201 {object} http.Response{data=models.CommonMessage} "Connection data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2CreateConnection(c *gin.Context) {
	var (
		connection models.CreateConnectionRequest
		resp       *obs.CommonMessage
	)

	err := c.ShouldBindJSON(&connection)
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(),
		&pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertRequestToSturct(connection)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	connection.ProjectId = resource.ResourceEnvironmentId

	services, err := h.GetProjectSrvc(
		c,
		projectId,
		resource.NodeType,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	// This is create connection by client type id
	switch resource.ResourceType {
	case 1:
		resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).Create(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: "connections",
				ProjectId: connection.ProjectId,
				Data:      structData,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.Created, resp)
	case 3:
		resp, err := services.GoItemService().Create(
			c.Request.Context(),
			&nobs.CommonMessage{
				TableSlug: "connections",
				ProjectId: connection.ProjectId,
				Data:      structData,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.Created, resp)
	}
}

// V2UpdateConnection godoc
// @ID update_connection_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/connection [PUT]
// @Summary Update Connection
// @Description Update Connection
// @Tags V2_Connection
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param connection body models.Connection true "UpdateConnectionRequestBody"
// @Success 201 {object} http.Response{data=models.CommonMessage} "Connection data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2UpdateConnection(c *gin.Context) {
	var (
		connection models.Connection
		resp       *obs.CommonMessage
	)

	if err := c.ShouldBindJSON(&connection); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertRequestToSturct(connection)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	connection.ProjectId = resource.ResourceEnvironmentId

	services, err := h.GetProjectSrvc(c, projectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	// This is create connection by client type id
	switch resource.ResourceType {
	case 1:
		resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).Update(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: "connections",
				ProjectId: connection.ProjectId,
				Data:      structData,
			},
		)
	case 3:
		resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).Update(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: "connections",
				ProjectId: connection.ProjectId,
				Data:      structData,
			},
		)
	}
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// V2GetConnectionList godoc
// @ID get_connection_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/connection [GET]
// @Summary Get Connection List
// @Description  Get Connection List
// @Tags V2_Connection
// @Accept json
// @Produce json
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Param project-id query string false "project-id"
// @Param user-id query string true "user-id"
// @Param client_type_id query string true "client_type_id"
// @Success 200 {object} http.Response{data=models.CommonMessage} "Connection data"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetConnectionList(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	// this is get list connection list from object builder
	services, err := h.GetProjectSrvc(c, projectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resp := &obs.CommonMessage{}
	switch resource.ResourceType {
	case 1:
		structData, err := helper.ConvertMapToStruct(map[string]any{
			"limit":          limit,
			"offset":         offset,
			"client_type_id": c.Query("client_type_id"),
		})
		if err != nil {
			h.handleResponse(c, http.InvalidArgument, err.Error())
			return
		}

		resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).GetList(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: "connections",
				ProjectId: resource.ResourceEnvironmentId,
				Data:      structData,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	case 3:
		structData, err := helper.ConvertMapToStruct(map[string]any{
			"limit":                     limit,
			"offset":                    offset,
			"client_type_id_from_token": c.Query("client_type_id"),
		})
		if err != nil {
			h.handleResponse(c, http.InvalidArgument, err.Error())
			return
		}
		result2, err := services.GoObjectBuilderService().GetList(
			c.Request.Context(),
			&nobs.CommonMessage{
				TableSlug: "connections",
				ProjectId: resource.ResourceEnvironmentId,
				Data:      structData,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		resp.Data = result2.Data
	}

	response, ok := resp.Data.AsMap()["response"].([]any)
	responseWithOptions := make([]any, 0, len(response))
	if ok && c.Query("user-id") != "" {
		switch resource.ResourceType {
		case pbCompany.ResourceType_MONGODB:
			for _, v := range response {
				if res, ok := v.(map[string]any); ok {
					if guid, ok := res["guid"].(string); ok {
						options, err := services.GetLoginServiceByType(resource.NodeType).GetConnetionOptions(
							c.Request.Context(),
							&obs.GetConnetionOptionsRequest{
								ConnectionId:          guid,
								ResourceEnvironmentId: resource.ResourceEnvironmentId,
								UserId:                c.Query("user-id"),
							},
						)
						if err != nil {
							continue
						}
						res["options"] = options.Data.AsMap()["response"]
					}
					v = res
				}
				responseWithOptions = append(responseWithOptions, v)
			}
		case pbCompany.ResourceType_POSTGRESQL:
			for _, v := range response {
				if res, ok := v.(map[string]any); ok {
					if guid, ok := res["guid"].(string); ok {
						options, err := services.GoLoginService().GetConnetionOptions(
							c.Request.Context(),
							&nobs.GetConnetionOptionsRequest{
								ConnectionId:          guid,
								ResourceEnvironmentId: resource.ResourceEnvironmentId,
								UserId:                c.Query("user-id"),
								ProjectId:             resource.ProjectId,
							},
						)
						if err != nil {
							continue
						}
						res["options"] = options.Data.AsMap()["response"]
					}
					v = res
				}
				responseWithOptions = append(responseWithOptions, v)
			}
		}
	}

	if len(responseWithOptions) <= 0 {
		if res, ok := resp.Data.AsMap()["response"].([]any); ok {
			responseWithOptions = res
		} else {
			responseWithOptions = []any{}
		}
	}
	h.handleResponse(c, http.OK, map[string]any{
		"data": map[string]any{
			"response": responseWithOptions,
			"count":    resp.Data.AsMap()["count"],
		},
	},
	)
}

// V2GetConnectionByID godoc
// @ID get_connection_by_id_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/connection/{connection_id} [GET]
// @Summary Get Connection By ID
// @Description Get Connection By ID
// @Tags V2_Connection
// @Accept json
// @Produce json
// @Param connection_id path string true "connection_id"
// @Param project-id query string true "project-id"
// @Success 200 {object} http.Response{data=models.CommonMessage} "ConnectionBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetConnectionByID(c *gin.Context) {
	var (
		err          error
		connectionId = c.Param("connection_id")
	)

	if !util.IsValidUUID(connectionId) {
		h.handleResponse(c, http.InvalidArgument, "connection id is an invalid uuid")
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(),
		&pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(map[string]any{"id": c.Param("connection_id")})
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, projectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	var resp *obs.CommonMessage
	switch resource.ResourceType {
	case 1:
		resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).GetSingle(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: "connections",
				ProjectId: resource.ResourceEnvironmentId,
				Data:      structData,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.OK, resp)
	case 3:
		resp, err := services.GoItemService().GetSingle(
			c.Request.Context(),
			&nobs.CommonMessage{
				TableSlug: "connections",
				ProjectId: resource.ResourceEnvironmentId,
				Data:      structData,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.OK, resp)
	}
}

// V2DeleteConnection godoc
// @ID delete_connection
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/connection/{connection_id} [DELETE]
// @Summary Delete Connection
// @Description Get Connection
// @Tags V2_Connection
// @Accept json
// @Produce json
// @Param connection_id path string true "connection_id"
// @Param project-id query string true "project-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2DeleteConnection(c *gin.Context) {
	var (
		connectionId = c.Param("connection_id")
		err          error
	)

	if !util.IsValidUUID(connectionId) {
		h.handleResponse(c, http.InvalidArgument, "connection id is an invalid uuid")
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(map[string]any{"id": connectionId})
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, projectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	var resp *obs.CommonMessage
	switch resource.ResourceType {
	case 1:
		resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).Delete(
			c.Request.Context(),
			&obs.CommonMessage{
				TableSlug: "connections",
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.NoContent, resp)
	case 3:
		resp, err := services.GoItemService().Delete(
			c.Request.Context(),
			&nobs.CommonMessage{
				TableSlug: "connections",
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.NoContent, resp)
	}
}

// GetConnectionOptions godoc
// @ID get_connection_options
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/get-connection-options/{connection_id}/{user_id} [GET]
// @Summary Get Connection Options
// @Description Get Connection Options
// @Tags V2_Connection
// @Accept json
// @Produce json
// @Param connection_id path string true "connection_id"
// @Param user_id path string true "user_id"
// @Param project-id query string true "project-id"
// @Success 200 {object} http.Response{data=models.CommonMessage} "ConnectionOptionsBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetConnectionOptions(c *gin.Context) {
	var (
		connectionId = c.Param("connection_id")
		err          error
	)

	if !util.IsValidUUID(connectionId) {
		h.handleResponse(c, http.InvalidArgument, "connection id is an invalid uuid")
		return
	}

	userId := c.Param("user_id")

	if !util.IsValidUUID(userId) {
		h.handleResponse(c, http.InvalidArgument, "user id is an invalid uuid")
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, projectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	var resp *obs.GetConnectionOptionsResponse
	switch resource.ResourceType {
	case 1:
		resp, err = services.GetLoginServiceByType(resource.NodeType).GetConnetionOptions(
			c.Request.Context(),
			&obs.GetConnetionOptionsRequest{
				ConnectionId:          connectionId,
				ResourceEnvironmentId: resource.ResourceEnvironmentId,
				UserId:                userId,
				NodeType:              resource.NodeType,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, http.OK, resp)
}

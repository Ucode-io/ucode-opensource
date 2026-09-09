package v1

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// GetResource godoc
// @Security ApiKeyAuth
// @ID get_resource_id
// @Router /v1/company/project/resource/{resource_id} [GET]
// @Summary Get Resource by id
// @Description Get Resource by id
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param resource_id path string true "resource_id"
// @Success 200 {object} status_http.Response{data=company_service.ResourceWithoutPassword} "Resource data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetResource(c *gin.Context) {

	resp, err := h.companyServices.Resource().GetResource(
		c.Request.Context(),
		&company_service.GetResourceRequest{
			Id: c.Param("resource_id"),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// AddProjectResource godoc
// @Security ApiKeyAuth
// @ID add_project_resource
// @Router /v1/company/project/resource [POST]
// @Summary Add ProjectResource
// @Description Add ProjectResource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param ProjectResource body company_service.AddResourceRequest true "ProjectResourceAddRequest"
// @Success 201 {object} status_http.Response{data=company_service.AddResourceResponse} "ProjectResource data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) AddProjectResource(c *gin.Context) {
	var company company_service.AddResourceRequest

	err := c.ShouldBindJSON(&company)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if company.ServiceType == company_service.ServiceType_NOT_SPECIFIED {
		switch company.ResourceType {
		case company_service.ResourceType_MONGODB:
			company.ServiceType = company_service.ServiceType_BUILDER_SERVICE
		case company_service.ResourceType_CLICKHOUSE:
			company.ServiceType = company_service.ServiceType_ANALYTICS_SERVICE
		case company_service.ResourceType_POSTGRESQL:
			company.ServiceType = company_service.ServiceType_BUILDER_SERVICE
		default:
			err := errors.New("err resource type not supported yet")
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	resp, err := h.companyServices.Resource().AddResource(
		c.Request.Context(),
		&company,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// ConfigureProjectResource godoc
// @Security ApiKeyAuth
// @ID configure_project_resource
// @Router /v1/company/project/configure-resource [POST]
// @Summary Configure ProjectResource
// @Description Configure ProjectResource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param ProjectResource body company_service.ConfigureResourceRequest true "ProjectResourceConfigureRequest"
// @Success 201 {object} status_http.Response{data=company_service.ConfigureResourceResponse} "ProjectResource data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ConfigureProjectResource(c *gin.Context) {
	var company company_service.ConfigureResourceRequest

	err := c.ShouldBindJSON(&company)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if company.ServiceType == company_service.ServiceType_NOT_SPECIFIED {
		switch company.ResourceType {
		case company_service.ResourceType_MONGODB:
			company.ServiceType = company_service.ServiceType_BUILDER_SERVICE
		case company_service.ResourceType_CLICKHOUSE:
			company.ServiceType = company_service.ServiceType_ANALYTICS_SERVICE
		case company_service.ResourceType_POSTGRESQL:
			company.ServiceType = company_service.ServiceType_BUILDER_SERVICE
		default:
			err := errors.New("err resource type not supported yet")
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	resp, err := h.companyServices.Resource().ConfigureResource(
		c.Request.Context(),
		&company,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// CreateProjectResource godoc
// @Security ApiKeyAuth
// @ID create_project_resource
// @Router /v1/company/project/create-resource [POST]
// @Summary Create ProjectResource
// @Description Create ProjectResource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param ProjectResource body company_service.CreateResourceReq true "ProjectResourceCreateRequest"
// @Success 201 {object} status_http.Response{data=company_service.CreateResourceRes} "ProjectResource data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateProjectResource(c *gin.Context) {
	var company company_service.CreateResourceReq

	err := c.ShouldBindJSON(&company)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Resource().CreateResource(
		c.Request.Context(),
		&company,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// RemoveProjectResource godoc
// @Security ApiKeyAuth
// @ID remove_project_resource
// @Router /v1/company/project/resource [DELETE]
// @Summary Remove ProjectResource
// @Description Remove ProjectResource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param ProjectResource body company_service.RemoveResourceRequest true "ProjectResourceRemoveRequest"
// @Success 201 {object} status_http.Response{data=company_service.EmptyProto} "ProjectResource data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) RemoveProjectResource(c *gin.Context) {
	var company company_service.RemoveResourceRequest

	err := c.ShouldBindJSON(&company)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Resource().RemoveResource(
		c.Request.Context(),
		&company,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// UpdateResource godoc
// @Security ApiKeyAuth
// @ID put_resource_id
// @Router /v1/company/project/resource/{resource_id} [PUT]
// @Summary Update Resource by id
// @Description Update Resource by id
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param UpdateResourceRequestBody body company_service.UpdateResourceRequest  true "UpdateResourceRequestBody"
// @Success 200 {object} status_http.Response{data=company_service.ResourceWithoutPassword} "Resource data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateResource(c *gin.Context) {
	var resource company_service.UpdateResourceRequest

	err := c.ShouldBindJSON(&resource)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Resource().UpdateResource(
		c.Request.Context(),
		&resource,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetResourceList godoc
// @Security ApiKeyAuth
// @ID get_resource_list
// @Router /v1/company/project/resource [GET]
// @Summary Get all companies
// @Description Get all companies
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param filters query company_service.GetResourceListRequest true "filters"
// @Success 200 {object} status_http.Response{data=company_service.GetResourceListResponse} "Resource data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetResourceList(c *gin.Context) {
	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok {
		h.HandleResponse(c, status_http.InvalidArgument, errors.New("project_id is required"))
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok {
		h.HandleResponse(c, status_http.InvalidArgument, errors.New("environment_id is required"))
		return
	}

	resp, err := h.companyServices.Resource().GetResourceList(
		c.Request.Context(),
		&company_service.GetResourceListRequest{
			Limit:         int32(limit),
			Offset:        int32(offset),
			Search:        c.DefaultQuery("search", ""),
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// ReconnectProjectResource godoc
// @Security ApiKeyAuth
// @ID reconnect_project_resource
// @Router /v1/company/project/resource/reconnect [POST]
// @Summary Reconnect ProjectResource
// @Description Reconnect ProjectResource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param ProjectResource body company_service.ReconnectResourceRequest true "ProjectResourceReconnectRequest"
// @Success 201 {object} status_http.Response{data=company_service.ReconnectResourceRes} "ProjectResource data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ReconnectProjectResource(c *gin.Context) {
	var company company_service.ReconnectResourceRequest

	err := c.ShouldBindJSON(&company)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	company.ProjectId = projectId.(string)

	if len(c.Query("project-id")) > 0 {
		company.ProjectId = c.Query("project-id")
	}

	resp, err := h.companyServices.Resource().ReconnectResource(
		c.Request.Context(),
		&company,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// GetResourceEnvironment godoc
// @Security ApiKeyAuth
// @ID get_resource_environment_id
// @Router /v1/company/project/resource-environment/{resource_id} [GET]
// @Summary Get Resource Environment by id
// @Description Get Resource Environment by id
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param resource_id path string true "resource_id"
// @Success 200 {object} status_http.Response{data=company_service.ResourceWithoutPassword} "Resource data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetResourceEnvironment(c *gin.Context) {

	resp, err := h.companyServices.Resource().GetResourceByResEnvironId(
		c.Request.Context(),
		&company_service.GetResourceRequest{
			Id: c.Param("resource_id"),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetServiceResources godoc
// @Security ApiKeyAuth
// @ID get_service_resources
// @Router /v1/company/project/resource-default [GET]
// @Summary Get Service Resource
// @Description Get Service Resource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Success 200 {object} status_http.Response{data=company_service.GetServiceResourcesRes} "Resource data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetServiceResources(c *gin.Context) {

	environmentId, ok := c.Get("environment_id")
	if !ok {
		h.HandleResponse(c, status_http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	res, err := h.companyServices.Resource().GetServiceResources(c.Request.Context(), &company_service.GetServiceResourcesReq{
		ProjectId:     c.DefaultQuery("project-id", ""),
		EnvironmentId: environmentId.(string),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

// SetDefaultResource godoc
// @Security ApiKeyAuth
// @ID set_default_resource
// @Router /v1/company/project/resource-default [PUT]
// @Summary Set Default Resource
// @Description Set Default Resource
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param data body company_service.SetDefaultResourceReq true "data"
// @Success 200 {object} status_http.Response{data=company_service.SetDefaultResourceRes} "Resource data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) SetDefaultResource(c *gin.Context) {
	var (
		req company_service.SetDefaultResourceReq
	)

	err := c.ShouldBindJSON(&req)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok {
		h.HandleResponse(c, status_http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	res, err := h.companyServices.Resource().SetDefaultResource(c.Request.Context(), &company_service.SetDefaultResourceReq{
		ProjectId:     c.DefaultQuery("project-id", ""),
		EnvironmentId: environmentId.(string),
		ServiceType:   req.GetServiceType(),
		ResourceId:    req.GetResourceId(),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

// GetByIdAirbyte godoc
// @Security ApiKeyAuth
// @ID get_by_id_airbyte
// @Router /v1/company/airbyte/{id} [GET]
// @Summary GetById Airbyte
// @Description GetById Airbyte
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 201 {object} status_http.Response{data=company_service.Airbyte} "Airbyte data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetByIdAirbyte(c *gin.Context) {
	resp, err := h.companyServices.AirByte().GetById(
		c.Request.Context(),
		&company_service.GetAirbyteByIdRequest{Id: c.Param("id")},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetListAirbyte godoc
// @Security ApiKeyAuth
// @ID get_list_airbyte
// @Router /v1/company/airbyte [POST]
// @Summary GetList Airbyte
// @Description GetList Airbyte
// @Tags Company Resource
// @Accept json
// @Produce json
// @Param Airbyte body models.AirByteRequest true "GetListAirbyteRequest"
// @Success 201 {object} status_http.Response{data=company_service.GetListAirbyteResponse} "Airbyte data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListAirbyte(c *gin.Context) {
	var airbyte models.AirByteRequest

	if err := c.ShouldBindJSON(&airbyte); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.AirByte().GetList(c.Request.Context(), &airbyte.Data)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

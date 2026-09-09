package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// CreateLanguage godoc
// @Security ApiKeyAuth
// @ID create-language
// @Router /v1/language [POST]
// @Summary Create language
// @Description Create language
// @Tags Language
// @Accept json
// @Produce json
// @Param language body nb.CreateLanguageRequest true "LanguageCreateRequest"
// @Success 201 {object} status_http.Response{data=nb.Language} "Language data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateLanguage(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
		createLanguage        = &obs.CreateLanguageRequest{}
	)

	err := c.ShouldBindJSON(&createLanguage)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType
	createLanguage.ProjectId = resourceEnvironmentId

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Language().Create(
			c, createLanguage,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.Created, resp)
	case pb.ResourceType_POSTGRESQL:
		createLanguagePg := nb.CreateLanguageRequest{}

		if err = helper.MarshalToStruct(&createLanguage, &createLanguagePg); err != nil {
			h.handleError(c, status_http.InternalServerError, err)
			return
		}
		resp, err := services.GoObjectBuilderService().Language().Create(
			c, &createLanguagePg,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.Created, resp)
	}
}

// GetByIdLanguage godoc
// @Security ApiKeyAuth
// @ID get-language
// @Router /v1/language/{id} [GET]
// @Summary Get language
// @Description Get language
// @Tags Language
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=nb.Language} "Language data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetByIdLanguage(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
		languageId            string
	)

	languageId = c.Param("id")
	if languageId == "" {
		h.HandleResponse(c, status_http.BadRequest, "language id is required")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Language().GetById(
			c, &obs.PrimaryKey{
				Id:        languageId,
				ProjectId: resourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Language().GetById(
			c, &nb.PrimaryKey{
				Id:        languageId,
				ProjectId: resourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// GetListLanguage godoc
// @Security ApiKeyAuth
// @ID list-languages
// @Router /v1/language [GET]
// @Summary List languages
// @Description List languages
// @Tags Language
// @Accept json
// @Produce json
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Success 200 {object} status_http.Response{data=nb.Language} "Languages data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListLanguage(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
	)

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return
	}

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParamWithoutDefault(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	search := c.DefaultQuery("search", "")

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Language().GetList(
			c, &obs.GetListLanguagesRequest{
				Offset:    int32(offset),
				Limit:     int32(limit),
				ProjectId: resourceEnvironmentId,
				Search:    search,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Language().GetList(
			c, &nb.GetListLanguagesRequest{
				Offset:    int32(offset),
				Limit:     int32(limit),
				ProjectId: resourceEnvironmentId,
				Search:    search,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// UpdateLanguage godoc
// @Security ApiKeyAuth
// @ID update-language
// @Router /v1/language [PUT]
// @Summary Update language
// @Description Update language
// @Tags Language
// @Accept json
// @Produce json
// @Param language body nb.Language true "Language"
// @Success 200 {object} status_http.Response{data=nb.Language} "Language data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateLanguage(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
		language              = &obs.UpdateLanguageRequest{}
	)

	err := c.ShouldBindJSON(&language)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType
	language.ProjectId = resourceEnvironmentId

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Language().Update(
			c, language,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		languagePg := nb.UpdateLanguageRequest{}

		if err = helper.MarshalToStruct(&language, &languagePg); err != nil {
			h.handleError(c, status_http.InternalServerError, err)
			return
		}
		resp, err := services.GoObjectBuilderService().Language().Update(
			c, &languagePg,
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// DeleteLanguage godoc
// @Security ApiKeyAuth
// @ID delete-language
// @Router /v1/language/{id} [DELETE]
// @Summary Delete language
// @Description Delete language
// @Tags Language
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteLanguage(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
		languageId            string
	)

	languageId = c.Param("id")
	if languageId == "" {
		h.HandleResponse(c, status_http.BadRequest, "language id is required")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Language().Delete(
			c, &obs.PrimaryKey{
				Id:        languageId,
				ProjectId: resourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Language().Delete(
			c, &nb.PrimaryKey{
				Id:        languageId,
				ProjectId: resourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err)
			return
		}
		h.HandleResponse(c, status_http.NoContent, resp)
	}
}

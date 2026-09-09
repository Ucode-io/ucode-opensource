package v1

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// CreateHtmlTemplate godoc
// @Security ApiKeyAuth
// @ID create_html_template
// @Router /v1/html-template [POST]
// @Summary Create htmlTemplate
// @Description Create htmlTemplate
// @Tags HtmlTemplate
// @Accept json
// @Produce json
// @Param html_template body obs.CreateHtmlTemplateRequest true "CreateHtmlTemplateRequestBody"
// @Success 201 {object} status_http.Response{data=obs.HtmlTemplate} "Html Template data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateHtmlTemplate(c *gin.Context) {
	var htmlTemplate obs.CreateHtmlTemplateRequest

	err := c.ShouldBindJSON(&htmlTemplate)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	htmlTemplate.ProjectId = resource.ResourceEnvironmentId

	resp, err := services.GetBuilderServiceByType(resource.NodeType).HtmlTemplate().Create(
		c.Request.Context(),
		&htmlTemplate,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// GetSingleHtmlTemplate godoc
// @Security ApiKeyAuth
// @ID get_html_template_by_id
// @Router /v1/html-template/{html_template_id} [GET]
// @Summary Get single html template
// @Description Get single html template
// @Tags HtmlTemplate
// @Accept json
// @Produce json
// @Param html_template_id path string true "html_template_id"
// @Success 200 {object} status_http.Response{data=obs.HtmlTemplate} "HtmlTemplateBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleHtmlTemplate(c *gin.Context) {
	htmlTemplateID := c.Param("html_template_id")
	if !util.IsValidUUID(htmlTemplateID) {
		h.HandleResponse(c, status_http.InvalidArgument, "html template id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).HtmlTemplate().GetSingle(
		c.Request.Context(),
		&obs.HtmlTemplatePrimaryKey{
			Id:        htmlTemplateID,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// UpdateHtmlTemplate godoc
// @Security ApiKeyAuth
// @ID update_html_template
// @Router /v1/html-template [PUT]
// @Summary Update html template
// @Description Update html template
// @Tags HtmlTemplate
// @Accept json
// @Produce json
// @Param html_template body obs.HtmlTemplate true "UpdateHtmlTemplateRequestBody"
// @Success 200 {object} status_http.Response{data=obs.HtmlTemplate} "HtmlTemplate data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateHtmlTemplate(c *gin.Context) {
	var htmlTemplate obs.HtmlTemplate

	err := c.ShouldBindJSON(&htmlTemplate)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	htmlTemplate.ProjectId = resource.ResourceEnvironmentId

	resp, err := services.GetBuilderServiceByType(resource.NodeType).HtmlTemplate().Update(
		c.Request.Context(),
		&htmlTemplate,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteHtmlTemplate godoc
// @Security ApiKeyAuth
// @ID delete_html_template_id
// @Router /v1/html-template/{html_template_id} [DELETE]
// @Summary Delete html template
// @Description Delete html template
// @Tags HtmlTemplate
// @Accept json
// @Produce json
// @Param html_template_id path string true "html_template_id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteHtmlTemplate(c *gin.Context) {
	htmlTemplateID := c.Param("html_template_id")

	if !util.IsValidUUID(htmlTemplateID) {
		h.HandleResponse(c, status_http.InvalidArgument, "html template id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).HtmlTemplate().Delete(
		c.Request.Context(),
		&obs.HtmlTemplatePrimaryKey{
			Id:        htmlTemplateID,
			ProjectId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, resp)
}

// GetHtmlTemplateList godoc
// @Security ApiKeyAuth
// @ID get_html_template_list
// @Router /v1/html-template [GET]
// @Summary Get html template list
// @Description Get html template list
// @Tags HtmlTemplate
// @Accept json
// @Produce json
// @Param filters query obs.GetAllHtmlTemplateRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetAllHtmlTemplateResponse} "HtmlTemplateBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetHtmlTemplateList(c *gin.Context) {

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GetBuilderServiceByType(resource.NodeType).HtmlTemplate().GetList(
		c.Request.Context(),
		&obs.GetAllHtmlTemplateRequest{
			TableSlug: c.Query("table_slug"),
			ProjectId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

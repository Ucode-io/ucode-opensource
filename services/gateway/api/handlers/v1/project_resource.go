package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// AddResourceToProject godoc
// @Security ApiKeyAuth
// @ID add_resource_to_project
// @Router /v2/company/project/resource [POST]
// @Summary Add rosource to project
// @Description Add rosource to project
// @Tags Project resource
// @Accept json
// @Produce json
// @Param data body pb.AddResourceToProjectRequest true "AddResourceToProjectRequest"
// @Success 201 {object} status_http.Response{data=pb.ProjectResource} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) AddResourceToProject(c *gin.Context) {
	var (
		request = &pb.AddResourceToProjectRequest{}
		resp    = &pb.ProjectResource{}
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Secret = nil

	resp, err := h.companyServices.Resource().AddResourceToProject(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	resp.Secret = nil

	h.HandleResponse(c, status_http.Created, resp)
}

// UpdateProjectResource godoc
// @Security ApiKeyAuth
// @ID update_project_resource
// @Router /v2/company/project/resource [PUT]
// @Summary Update Project resource
// @Description update Project resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param Company body pb.ProjectResource  true "ProjectResource"
// @Success 200 {object} status_http.Response{data=pb.Empty} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateProjectResource(c *gin.Context) {
	var (
		request = &pb.ProjectResource{}
		resp    = &pb.Empty{}
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	userId, _ := c.Get("user_id")

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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Secret = nil

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE",
			UserInfo:     cast.ToString(userId),
			Request:      projectResourceForLog(request),
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.companyServices.Resource().UpdateProjectResource(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetListProjectResource godoc
// @Security ApiKeyAuth
// @ID get_list_project_resource
// @Router /v2/company/project/resource [GET]
// @Summary Get list project resource
// @Description Get list project resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param type query string false "type"
// @Success 200 {object} status_http.Response{data=pb.ListProjectResource} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListProjectResourceList(c *gin.Context) {
	var request = &pb.GetProjectResourceListRequest{}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)

	if c.DefaultQuery("type", "") != "" && pb.ResourceType(pb.ResourceType_value[c.DefaultQuery("type", "")]) != 0 {
		request.Type = pb.ResourceType(pb.ResourceType_value[c.DefaultQuery("type", "")])
	}

	resp, err := h.companyServices.Resource().GetProjectResourceList(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for _, resource := range resp.GetResources() {
		resource.Secret = nil
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetProjectResourceByID godoc
// @Security ApiKeyAuth
// @ID get_single_project_resource
// @Router /v2/company/project/resource/{id} [GET]
// @Summary Get single variable resource
// @Description Get single variable resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=pb.ProjectResource} "ProjectResource"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleProjectResource(c *gin.Context) {
	request := &pb.PrimaryKeyProjectResource{}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Id = c.Param("id")

	resp, err := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp.Secret = nil

	h.HandleResponse(c, status_http.OK, resp)
}

// DeletevariableResource godoc
// @Security ApiKeyAuth
// @ID delete_project_resource
// @Router /v2/company/project/resource/{id} [DELETE]
// @Summary Delete variable resource
// @Description Delete variable resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteProjectResource(c *gin.Context) {
	var (
		request = &pb.PrimaryKeyProjectResource{}
		resp    = &pb.Empty{}
	)

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
		return
	}

	userId, _ := c.Get("user_id")

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Id = c.Param("id")

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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE",
			UserInfo:     cast.ToString(userId),
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.companyServices.Resource().DeleteProjectResource(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, nil)
}

// projectResourceForLog returns a copy safe to write to logs: the secret is
// dropped so credentials never reach the log sink.
func projectResourceForLog(resource *pb.ProjectResource) *pb.ProjectResource {
	if resource == nil {
		return nil
	}
	safe := *resource
	safe.Secret = nil
	return &safe
}

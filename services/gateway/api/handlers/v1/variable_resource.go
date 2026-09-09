package v1

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// AddDataToVariableResource godoc
// @Security ApiKeyAuth
// @ID add_data_to_variable_resource
// @Router /v1/company/project/resource-variable [POST]
// @Summary Add data to variable resource
// @Description Add data to variable resource
// @Tags Variable resource
// @Accept json
// @Produce json
// @Param data body pb.CreateVariableResourceRequest true "CreateVariableResourceRequest"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) AddDataToVariableResource(c *gin.Context) {
	var request = &pb.CreateVariableResourceRequest{}

	err := c.ShouldBindJSON(&request)
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
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)

	resp, err := h.companyServices.Resource().CreateVariableResource(
		c.Request.Context(),
		request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// UpdateVariableResource godoc
// @Security ApiKeyAuth
// @ID update_variable_resource
// @Router /v1/company/project/resource-variable [PUT]
// @Summary Update variable resource
// @Description update variable resource
// @Tags Variable resource
// @Accept json
// @Produce json
// @Param Company body pb.UpdateVariableResourceRequest  true "VariableResource"
// @Success 200 {object} status_http.Response{data=pb.Empty} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateVariableResource(c *gin.Context) {
	var request = &pb.UpdateVariableResourceRequest{}

	err := c.ShouldBindJSON(&request)
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
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)

	resp, err := h.companyServices.Resource().UpdateVariableResource(
		c.Request.Context(),
		request,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetListVariableResource godoc
// @Security ApiKeyAuth
// @ID get_list_variable_resource
// @Router /v1/company/project/resource-variable/{project-resource-id} [GET]
// @Summary Get list variable resource
// @Description Get list variable resource
// @Tags Variable resource
// @Accept json
// @Produce json
// @Param project-resource-id path string true "project-resource-id"
// @Success 200 {object} status_http.Response{data=pb.GetVariableResourceListResponse} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListVariableResource(c *gin.Context) {

	request := &pb.GetVariableResourceListRequest{}

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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.ProjectResourceId = c.Param("project-resource-id")

	resp, err := h.companyServices.Resource().GetVariableResourceList(
		c.Request.Context(),
		request,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetVariableByIdOrKey godoc
// @Security ApiKeyAuth
// @ID get_single_variable_resource
// @Router /v1/company/project/resource-variable/single [GET]
// @Summary Get single variable resource
// @Description Get single variable resource
// @Tags Variable resource
// @Accept json
// @Produce json
// @Param id query string false "id"
// @Param key query string false "key"
// @Success 200 {object} status_http.Response{data=pb.VariableResource} "VariableResource"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleVariableResource(c *gin.Context) {

	request := &pb.PrimaryKeyVariableResource{}

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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Key = c.DefaultQuery("key", "")
	request.Id = c.DefaultQuery("id", "")

	resp, err := h.companyServices.Resource().GetSingleVariableResource(
		c.Request.Context(),
		request,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeletevariableResource godoc
// @Security ApiKeyAuth
// @ID delete_variable_resource
// @Router /v1/company/project/resource-variable/{id} [DELETE]
// @Summary Delete variable resource
// @Description Delete variable resource
// @Tags Variable resource
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=pb.Empty} "VariableResource"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteVariableResource(c *gin.Context) {

	request := &pb.PrimaryKeyVariableResource{}

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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Id = c.Param("id")

	resp, err := h.companyServices.Resource().DeleteVariableResource(
		c.Request.Context(),
		request,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

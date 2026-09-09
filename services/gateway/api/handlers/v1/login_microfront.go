package v1

import (
	"errors"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// BindLoginMicroFrontToProject godoc
// @Security ApiKeyAuth
// @ID bind_login_micro_front_to_project
// @Router /v1/login-microfront [POST]
// @Summary Bind login microfrotn to project
// @Description Bind login microfrotn to project
// @Tags Project login microfront
// @Accept json
// @Produce json
// @Param data body pb.ProjectLoginMicroFrontend true "ProjectLoginMicroFrontend"
// @Success 201 {object} status_http.Response{data=pb.ProjectLoginMicroFrontend} "ProjectLoginMicroFrontend"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) BindLoginMicroFrontToProject(c *gin.Context) {
	var (
		data pb.ProjectLoginMicroFrontend
		//resourceEnvironment *obs.ResourceEnvironment
	)

	err := c.ShouldBindJSON(&data)
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

	data.ProjectId = projectId.(string)
	data.EnvironmentId = environmentId.(string)

	res, err := h.companyServices.Project().CreateProjectLoginMicroFront(
		c.Request.Context(),
		&data,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, res)
}

// UpdateLoginMicroFront godoc
// @Security ApiKeyAuth
// @ID update_login_microfront
// @Router /v1/login-microfront [PUT]
// @Summary Update Login MicroFront Project
// @Description Update Login MicroFront Project
// @Tags Project login microfront
// @Accept json
// @Produce json
// @Param Company body pb.ProjectLoginMicroFrontend  true "ProjectLoginMicroFrontend"
// @Success 200 {object} status_http.Response{data=pb.ProjectLoginMicroFrontend} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateLoginMicroFrontProject(c *gin.Context) {
	var req pb.ProjectLoginMicroFrontend

	err := c.ShouldBindJSON(&req)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Project().UpdateProjectLoginMicroFront(
		c.Request.Context(),
		&req,
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetLoginMicroFrontBySubdomain godoc
// @Security ApiKeyAuth
// @ID get_login_microfront_by_subdomain
// @Router /v1/login-microfront [GET]
// @Summary Get Project By Id
// @Description Get Project By Id
// @Tags Project login microfront
// @Accept json
// @Produce json
// @Param subdomain query string false "subdomain"
// @Param project-id query string false "project-id"
// @Success 200 {object} status_http.Response{data=models.MicrofrontForLoginPage} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetLoginMicroFrontBySubdomain(c *gin.Context) {
	subdomain := c.DefaultQuery("subdomain", "")

	if subdomain == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "subdomain or project-id is required")
		return
	}

	resp, err := h.companyServices.Project().GetProjectLoginMicroFront(
		c.Request.Context(),
		&pb.GetProjectLoginMicroFrontRequest{
			Subdomain: subdomain,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resp.ProjectId == "" || resp.EnvironmentId == "" {
		h.HandleResponse(c, status_http.OK, models.MicrofrontForLoginPage{})
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     resp.ProjectId,
			EnvironmentId: resp.EnvironmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		resp.ProjectId,
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	functionResp := &obs.Function{}
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		functionResp, err = services.GetBuilderServiceByType(resource.NodeType).Function().GetSingle(c.Request.Context(), &obs.FunctionPrimaryKey{
			Id:        resp.MicrofrontId,
			ProjectId: resource.ResourceEnvironmentId,
		})
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		function, err := services.GoObjectBuilderService().Function().GetSingle(c.Request.Context(), &nobs.FunctionPrimaryKey{
			Id:        resp.MicrofrontId,
			ProjectId: resource.ResourceEnvironmentId,
		})
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(function, &functionResp); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, models.MicrofrontForLoginPage{
		Function:      functionResp,
		Id:            resp.GetId(),
		MicrofrontId:  resp.GetMicrofrontId(),
		ProjectId:     resp.GetProjectId(),
		Subdomain:     resp.GetSubdomain(),
		EnvironmentId: resp.GetEnvironmentId(),
	})
}

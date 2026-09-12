package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// CreateEnvironment godoc
// @Security ApiKeyAuth
// @ID create_environment
// @Router /v1/environment [POST]
// @Summary Create environment
// @Description Create environment
// @Tags Environment
// @Accept json
// @Produce json
// @Param environment body pb.CreateEnvironmentRequest true "CreateEnvironmentRequestBody"
// @Success 201 {object} status_http.Response{data=pb.Environment} "Environment data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateEnvironment(c *gin.Context) {
	var (
		environmentRequest pb.CreateEnvironmentRequest
		resp               = &pb.Environment{}
	)

	if err := c.ShouldBindJSON(&environmentRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	tokenInfo, err := h.GetAuthAdminInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
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

	environmentRequest.RoleId = tokenInfo.GetRoleId()
	environmentRequest.UserId = tokenInfo.GetUserIdAuth()
	environmentRequest.ClientTypeId = tokenInfo.GetClientTypeId()

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "CREATE",
			UserInfo:     cast.ToString(userId),
			Request:      &environmentRequest,
			TableSlug:    "ENVIRONMENT",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Request = resp
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.companyServices.Environment().CreateV2(c.Request.Context(), &environmentRequest)
	if err != nil {
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// GetSingleEnvironment godoc
// @Security ApiKeyAuth
// @ID get_environment_by_id
// @Router /v1/environment/{environment_id} [GET]
// @Summary Get single environment
// @Description Get single environment
// @Tags Environment
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Success 200 {object} status_http.Response{data=pb.Environment} "EnvironmentBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleEnvironment(c *gin.Context) {
	environmentID := c.Param("environment_id")

	if !util.IsValidUUID(environmentID) {
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
		return
	}

	resp, err := h.companyServices.Environment().GetById(c.Request.Context(), &pb.EnvironmentPrimaryKey{Id: environmentID})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// UpdateEnvironment godoc
// @Security ApiKeyAuth
// @ID update_environment
// @Router /v1/environment [PUT]
// @Summary Update environment
// @Description Update environment
// @Tags Environment
// @Accept json
// @Produce json
// @Param environment body models.Environment true "UpdateEnvironmentRequestBody"
// @Success 200 {object} status_http.Response{data=pb.Environment} "Environment data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateEnvironment(c *gin.Context) {
	var (
		environment models.Environment
		resp        = &pb.Environment{}
	)

	if err := c.ShouldBindJSON(&environment); err != nil {
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
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id | not valid")
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

	var (
		updateEnvironment = &pb.Environment{
			Id:           environment.Id,
			ProjectId:    environment.ProjectId,
			Name:         environment.Name,
			DisplayColor: environment.DisplayColor,
			Description:  environment.Description,
		}
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE",
			UserInfo:     cast.ToString(userId),
			Request:      &updateEnvironment,
			TableSlug:    "ENVIRONMENT",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.companyServices.Environment().Update(c.Request.Context(), updateEnvironment)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteEnvironment godoc
// @Security ApiKeyAuth
// @ID delete_environment
// @Router /v1/environment/{environment_id} [DELETE]
// @Summary Delete environment
// @Description Delete environment
// @Tags Environment
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteEnvironment(c *gin.Context) {
	var (
		resp          = &pb.Empty{}
		environmentID = c.Param("environment_id")
	)

	if !util.IsValidUUID(environmentID) {
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
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

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE",
			UserInfo:     cast.ToString(userId),
			TableSlug:    "ENVIRONMENT",
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

	resp, err = h.companyServices.Environment().Delete(c.Request.Context(), &pb.EnvironmentPrimaryKey{Id: environmentID})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, nil)
}

// GetAllEnvironments godoc
// @Security ApiKeyAuth
// @ID get_environment_list
// @Router /v1/environment [GET]
// @Summary Get environment list
// @Description Get environment list
// @Tags Environment
// @Accept json
// @Produce json
// @Param filters query pb.GetEnvironmentListRequest true "filters"
// @Success 200 {object} status_http.Response{data=pb.GetEnvironmentListResponse} "EnvironmentBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllEnvironments(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	authInfo, err := h.GetAuthAdminInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}

	resp, err := h.companyServices.Environment().GetList(
		c.Request.Context(), &pb.GetEnvironmentListRequest{
			Offset:         int32(offset),
			Limit:          int32(limit),
			Search:         c.Query("search"),
			ProjectId:      c.Query("project_id"),
			UserId:         authInfo.GetUserIdAuth(),
			WithClientType: c.DefaultQuery("with_client_type", "true") == "true",
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

package handlers

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"

	"github.com/google/uuid"
	"github.com/saidamir98/udevs_pkg/util"
	"github.com/spf13/cast"

	"github.com/gin-gonic/gin"
)

// CreateProject godoc
// @ID create_project
// @Router /project [POST]
// @Summary Create Project
// @Description Create Project
// @Tags Project
// @Accept json
// @Produce json
// @Param user_id query string false "user_id"
// @Param project body company_service.CreateProjectRequest true "CreateProjectRequestBody"
// @Success 201 {object} http.Response{data=company_service.CreateProjectResponse} "Project data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) CreateProject(c *gin.Context) {
	var (
		project company_service.CreateProjectRequest
		resp    *company_service.CreateProjectResponse
	)

	if err := c.ShouldBindJSON(&project); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.ProjectServiceClient().Create(
		c.Request.Context(),
		&project,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	_, err = h.services.UserService().AddUserToProject(
		c.Request.Context(),
		&pb.AddUserToProjectReq{
			UserId:    c.Query("user_id"),
			CompanyId: project.GetCompanyId(),
			ProjectId: resp.GetProjectId(),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// GetProjectList godoc
// @ID get_project_list
// @Router /project [GET]
// @Summary Get Project List
// @Description  Get Project List
// @Tags Project
// @Accept json
// @Produce json
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Param search query string false "search"
// @Success 200 {object} http.Response{data=company_service.GetProjectListResponse} "GetProjectListResponseBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetProjectList(c *gin.Context) {
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

	resp, err := h.services.ProjectServiceClient().GetList(
		c.Request.Context(), &company_service.GetProjectListRequest{
			Limit:  int32(limit),
			Offset: int32(offset),
			Search: c.Query("search"),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// GetProjectByID godoc
// @ID get_project_by_id
// @Router /project/{project-id} [GET]
// @Summary Get Project By ID
// @Description Get Project By ID
// @Tags Project
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Success 200 {object} http.Response{data=company_service.Project} "ProjectBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetProjectByID(c *gin.Context) {
	projectID := c.Param("project-id")

	if !util.IsValidUUID(projectID) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	resp, err := h.services.ProjectServiceClient().GetById(
		c.Request.Context(),
		&company_service.GetProjectByIdRequest{
			ProjectId: projectID,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// UpdateProject godoc
// @ID update_project
// @Router /project [PUT]
// @Summary Update Project
// @Description Update Project
// @Tags Project
// @Accept json
// @Produce json
// @Param project body company_service.Project true "UpdateProjectRequestBody"
// @Success 200 {object} http.Response{data=company_service.Project} "Project data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UpdateProject(c *gin.Context) {
	var project company_service.Project

	if err := c.ShouldBindJSON(&project); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.ProjectServiceClient().Update(
		c.Request.Context(), &project,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// DeleteProject godoc
// @ID delete_project
// @Router /project/{project-id} [DELETE]
// @Summary Delete Project
// @Description Get Project
// @Tags Project
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteProject(c *gin.Context) {
	projectID := c.Param("project-id")

	if !util.IsValidUUID(projectID) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	resp, err := h.services.ProjectServiceClient().Delete(
		c.Request.Context(), &company_service.DeleteProjectRequest{
			ProjectId: projectID,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

func (h *Handler) Emqx(c *gin.Context) {
	project := make(map[string]any)

	if err := c.ShouldBindJSON(&project); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	clientId := cast.ToString(project["client_id"])
	if _, err := uuid.Parse(clientId); err != nil {
		c.JSON(400, map[string]any{"result": "deny"})
		return
	}

	resp, err := h.services.ProjectServiceClient().GetById(
		c.Request.Context(),
		&company_service.GetProjectByIdRequest{
			ProjectId: clientId,
		},
	)
	if err != nil {
		c.JSON(500, map[string]any{"result": "deny"})
		return
	}

	if config.IsProjectStatusBlocking(resp.Status) {
		c.JSON(500, map[string]any{"result": "deny"})
		return
	}

	c.JSON(200, map[string]any{
		"result": "allow", // "allow" | "deny" | "ignore"
	})
}

func (h *Handler) Custom(c *gin.Context) {
	project := make(map[string]any)

	if err := c.ShouldBindJSON(&project); err != nil {
		h.handleResponse(c, http.Unauthorized, err.Error())
		return
	}

	c.JSON(200, map[string]any{
		"X-Hasura-User-Id": "bba3dddc-5f20-449c-8ec8-37bef283c766",
		"X-Hasura-Role":    "admin",
	})
}

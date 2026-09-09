package v1

import (
	"errors"
	"strconv"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
)

// GetCompanyProjectById godoc
// @Security ApiKeyAuth
// @ID get_company_project_id
// @Router /v1/company-project/{project_id} [GET]
// @Summary Get Project By Id
// @Description Get Project By Id
// @Tags Company Project
// @Accept json
// @Produce json
// @Param project_id path string true "project_id"
// @Param company_id query string false "company_id"
// @Success 200 {object} status_http.Response{data=company_service.Project} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetCompanyProjectById(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok && !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "project id is required")
		return
	}

	resp, err := h.companyServices.Project().GetById(
		c.Request.Context(),
		&company_service.GetProjectByIdRequest{
			ProjectId: projectId.(string),
			CompanyId: c.Query("company_id"),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetCompanyProjectList godoc
// @Security ApiKeyAuth
// @ID get_project_list
// @Router /v1/company-project [GET]
// @Summary Get all projects
// @Description Get all projects
// @Tags Company Project
// @Accept json
// @Produce json
// @Param filters query company_service.GetProjectListRequest true "filters"
// @Success 200 {object} status_http.Response{data=company_service.GetProjectListResponse} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetCompanyProjectList(c *gin.Context) {
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

	authInfo, _ := h.GetAuthAdminInfo(c)
	userProjects, err := h.authService.User().GetUserProjects(c.Request.Context(), &auth_service.UserPrimaryKey{
		Id: authInfo.GetUserIdAuth(),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var projectIds []string
	for _, userProject := range userProjects.GetCompanies() {
		if userProject.GetId() == c.Query("company_id") {
			projectIds = userProject.GetProjectIds()
		}
	}

	resp, err := h.companyServices.Project().GetList(
		c.Request.Context(),
		&company_service.GetProjectListRequest{
			Limit:     int32(limit),
			Offset:    int32(offset),
			Search:    c.Query("search"),
			CompanyId: c.Query("company_id"),
		},
	)

	projectsMap := make(map[string]*company_service.Project)
	for _, project := range resp.GetProjects() {
		projectsMap[project.GetProjectId()] = project
	}

	var availableProjects = make([]*company_service.Project, 0, len(projectIds))

	for _, id := range projectIds {
		if val, ok := projectsMap[id]; ok {
			availableProjects = append(availableProjects, val)
		}
	}

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	resp.Projects = availableProjects

	h.HandleResponse(c, status_http.OK, resp)
}

// UpdateCompanyProject godoc
// @Security ApiKeyAuth
// @ID update_project
// @Router /v1/company-project/{project_id} [PUT]
// @Summary Update Project
// @Description Update Project
// @Tags Company Project
// @Accept json
// @Produce json
// @Param project_id path string true "project_id"
// @Param Company body company_service.Project  true "CompanyProjectCreateRequest"
// @Success 200 {object} status_http.Response{data=company_service.Project} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateCompanyProject(c *gin.Context) {
	var (
		project   company_service.Project
		projectId = c.Param("project_id")
	)

	project.ProjectId = projectId

	err := c.ShouldBindJSON(&project)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Project().Update(c.Request.Context(), &project)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteCompanyProject godoc
// @Security ApiKeyAuth
// @ID delete_project
// @Router /v1/company-project/{project_id} [DELETE]
// @Summary Delete Project
// @Description Delete Project
// @Tags Company Project
// @Accept json
// @Produce json
// @Param project_id path string true "project_id"
// @Success 204 {object} status_http.Response{data=string} "Data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteCompanyProject(c *gin.Context) {
	projectId := c.Param("project_id")

	resp, err := h.companyServices.Project().Delete(
		c.Request.Context(),
		&company_service.DeleteProjectRequest{
			ProjectId: projectId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, resp)
}

// GetCompanyProjectList godoc
// @Security ApiKeyAuth
// @ID get_companies_id_projects_list
// @Router /v1/companies/{company_id}/projects [GET]
// @Summary Get all projects
// @Description Get all projects
// @Tags Company Project
// @Accept json
// @Produce json
// @Param company_id path string true "company_id"
// @Param filters query company_service.GetProjectListRequest true "filters"
// @Success 200 {object} status_http.Response{data=company_service.GetProjectListResponse} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ListCompanyProjects(c *gin.Context) {
	companyId := c.Param("company_id")
	if companyId == "" {
		h.HandleResponse(c, status_http.BadRequest, "company_id is required")
		return
	}

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

	resp, err := h.companyServices.Project().GetList(c.Request.Context(),
		&company_service.GetProjectListRequest{
			Limit:     int32(limit),
			Offset:    int32(offset),
			Search:    c.Query("search"),
			CompanyId: companyId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) AttachFareToProject(c *gin.Context) {
	var (
		data company_service.AttachFareRequest
	)

	if err := c.ShouldBindJSON(&data); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}
	data.ProjectId = projectId.(string)

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "error getting environment id")
		return
	}

	resp, err := h.companyServices.Project().AttachFare(c.Request.Context(), &data)
	if err != nil {
		h.handleError(c, status_http.GRPCError, err)
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetAllSettings godoc
// @Security ApiKeyAuth
// @ID get_list_setting
// @Router /v1/project/setting [GET]
// @Summary Get List settings
// @Description Get List settings
// @Tags Company Project
// @Accept json
// @Produce json
// @Param project-id query string false "project-id"
// @Param search query string false "search"
// @Param limit query string false "limit"
// @Param offset query string false "offset"
// @Param type query string false "type"
// @Success 200 {object} status_http.Response{data=obs.Setting} "Setting"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllSettings(c *gin.Context) {
	var (
		//resourceEnvironment *obs.ResourceEnvironment
		settingType obs.SettingType
	)

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "100"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	offset, err := strconv.Atoi(c.DefaultQuery("offset", "0"))
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	stype := c.DefaultQuery("type", "LANGUAGE")

	switch strings.ToUpper(stype) {
	case "LANGUAGE":
		settingType = obs.SettingType_LANGUAGE
	case "CURRENCY":
		settingType = obs.SettingType_CURRENCY
	case "TIMEZONE":
		settingType = obs.SettingType_TIMEZONE
	default:
		err = errors.New("not valid type")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	projectId := c.DefaultQuery("project-id", "")

	res, err := h.companyServices.Project().GetListSetting(
		c.Request.Context(),
		&obs.GetListSettingReq{
			Type:      settingType,
			ProjectId: projectId,
			Search:    c.DefaultQuery("search", ""),
			Limit:     int32(limit),
			Offset:    int32(offset),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, res)
}

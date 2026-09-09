package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"sync"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"

	"github.com/gin-gonic/gin"
)

// GetUgenCompanyProjects godoc
// @Security ApiKeyAuth
// @ID get_ugen_company_projects
// @Router /v1/ugen/company-projects [GET]
// @Summary Get Ugen Company Projects
// @Description Returns projects for a given company the user has access to, each with its default environment ID (Production preferred, any otherwise).
// @Tags Ugen
// @Produce json
// @Param company_id query string true "company_id"
// @Success 200 {object} status_http.Response{data=[]models.UgenProjectResp}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 401 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *HandlerV1) GetUgenCompanyProjects(c *gin.Context) {
	var ctx = c.Request.Context()

	companyId := c.Query("company_id")
	if companyId == "" {
		h.HandleResponse(c, status_http.BadRequest, "company_id is required")
		return
	}

	authInfo, err := h.adminAuthInfo(c)
	if err != nil {
		return
	}
	userId := authInfo.GetUserIdAuth()
	if userId == "" {
		h.HandleResponse(c, status_http.Unauthorized, "unauthorized")
		return
	}

	userProjects, err := h.authService.User().GetUserProjects(ctx, &auth.UserPrimaryKey{Id: userId})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var projectIds []string
	for _, uc := range userProjects.GetCompanies() {
		if uc.GetId() == companyId {
			projectIds = uc.GetProjectIds()
			break
		}
	}

	var (
		result   = make([]models.UgenProjectResp, 0, len(projectIds))
		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	resultSlice := make([]models.UgenProjectResp, len(projectIds))

	for i, projId := range projectIds {
		wg.Add(1)
		go func(idx int, projId string) {
			defer wg.Done()

			info, err := h.companyServices.Project().GetById(
				ctx, &pb.GetProjectByIdRequest{
					ProjectId: projId,
					CompanyId: companyId,
				},
			)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			ugenStatus, _ := h.companyServices.Project().GetProjectUgenStatus(
				ctx, &pb.GetProjectUgenStatusRequest{
					ProjectId: projId,
					CompanyId: companyId,
				},
			)

			// Try Production first
			envList, _ := h.companyServices.Environment().GetList(
				ctx, &pb.GetEnvironmentListRequest{
					ProjectId: projId,
					Search:    "Production",
					Limit:     1,
				},
			)

			var envId string
			if len(envList.GetEnvironments()) > 0 {
				envId = envList.GetEnvironments()[0].GetId()
			} else {
				// Fallback: get any environment
				anyEnv, _ := h.companyServices.Environment().GetList(
					ctx, &pb.GetEnvironmentListRequest{
						ProjectId: projId,
						Limit:     1,
					},
				)
				if len(anyEnv.GetEnvironments()) > 0 {
					envId = anyEnv.GetEnvironments()[0].GetId()
				}
			}

			resultSlice[idx] = models.UgenProjectResp{
				Id:            info.GetProjectId(),
				Title:         info.GetTitle(),
				Logo:          info.GetLogo(),
				IsUgen:        ugenStatus.GetIsUgen(),
				EnvironmentId: envId,
			}
		}(i, projId)
	}

	wg.Wait()

	if firstErr != nil {
		h.HandleResponse(c, status_http.GRPCError, firstErr.Error())
		return
	}

	for _, p := range resultSlice {
		if p.Id != "" {
			result = append(result, p)
		}
	}

	h.HandleResponse(c, status_http.OK, result)
}

// GetUgenUserProjects godoc
// @Security ApiKeyAuth
// @ID get_ugen_user_projects
// @Router /v1/ugen/user-projects [GET]
// @Summary Get Ugen User Projects
// @Description Returns companies and their projects the user has access to.
// @Description If a company has an is_ugen=true project, only that project is returned (personal fork).
// @Tags Ugen
// @Produce json
// @Success 200 {object} status_http.Response{data=models.UgenUserProjectsResp}
// @Failure 401 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *HandlerV1) GetUgenUserProjects(c *gin.Context) {
	var ctx = c.Request.Context()

	authInfo, err := h.adminAuthInfo(c)
	if err != nil {
		return
	}
	userId := authInfo.GetUserIdAuth()
	if userId == "" {
		h.HandleResponse(c, status_http.Unauthorized, "unauthorized")
		return
	}

	userProjects, err := h.authService.User().GetUserProjects(ctx, &auth.UserPrimaryKey{Id: userId})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		companies = userProjects.GetCompanies()
		result    = models.UgenUserProjectsResp{
			Companies: make([]models.UgenCompanyResp, len(companies)),
		}

		wg       sync.WaitGroup
		mu       sync.Mutex
		firstErr error
	)

	for i, uc := range companies {
		wg.Add(1)
		go func(idx int, uc *auth.UserCompany) {
			defer wg.Done()

			companyResp, err := h.companyServices.Company().GetById(ctx, &pb.GetCompanyByIdRequest{Id: uc.GetId()})
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = err
				}
				mu.Unlock()
				return
			}

			var (
				company = companyResp.GetCompany()

				projectIds = uc.GetProjectIds()
				projects   = make([]models.UgenProjectData, len(projectIds))

				projWg sync.WaitGroup
			)

			for j, projId := range projectIds {
				projWg.Add(1)
				go func(jdx int, projId string) {
					defer projWg.Done()

					info, err := h.companyServices.Project().GetById(
						ctx, &pb.GetProjectByIdRequest{
							ProjectId: projId,
							CompanyId: uc.GetId(),
						},
					)
					if err != nil {
						return
					}

					ugenStatus, _ := h.companyServices.Project().GetProjectUgenStatus(
						ctx, &pb.GetProjectUgenStatusRequest{
							ProjectId: projId,
							CompanyId: uc.GetId(),
						},
					)

					envList, _ := h.companyServices.Environment().GetList(
						ctx, &pb.GetEnvironmentListRequest{
							ProjectId: projId,
							Search:    "Production",
							Limit:     1,
						},
					)

					var envId string
					if envList != nil && len(envList.GetEnvironments()) > 0 {
						envId = envList.GetEnvironments()[0].GetId()
					}

					projects[jdx] = models.UgenProjectData{
						Id:            info.GetProjectId(),
						Title:         info.GetTitle(),
						Logo:          info.GetLogo(),
						IsUgen:        ugenStatus.GetIsUgen(),
						EnvironmentId: envId,
					}
				}(j, projId)
			}
			projWg.Wait()

			ugenIdx := -1

			for k, p := range projects {
				if p.IsUgen {
					ugenIdx = k
					break
				}
			}

			companyOut := models.UgenCompanyResp{
				Id:              company.GetId(),
				Name:            company.GetName(),
				Logo:            company.GetLogo(),
				HasPersonalFork: ugenIdx >= 0,
				Projects:        make([]models.UgenProjectResp, 0, len(projects)),
			}

			if ugenIdx >= 0 {
				p := projects[ugenIdx]
				companyOut.Projects = append(companyOut.Projects, models.UgenProjectResp{
					Id:            p.Id,
					Title:         p.Title,
					Logo:          p.Logo,
					IsUgen:        true,
					EnvironmentId: p.EnvironmentId,
				})
			} else {
				for _, p := range projects {
					if p.Id == "" {
						continue
					}
					companyOut.Projects = append(companyOut.Projects, models.UgenProjectResp{
						Id:            p.Id,
						Title:         p.Title,
						Logo:          p.Logo,
						IsUgen:        false,
						EnvironmentId: p.EnvironmentId,
					})
				}
			}

			result.Companies[idx] = companyOut
		}(i, uc)
	}

	wg.Wait()

	if firstErr != nil {
		h.HandleResponse(c, status_http.GRPCError, firstErr.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, result)
}

// ListUgenProjects godoc
// @Security ApiKeyAuth
// @ID list_ugen_projects
// @Router /v1/ugen/projects [GET]
// @Summary List all is_ugen projects (admin)
// @Description Returns a paginated list of all is_ugen projects across companies. Each item includes a default environment ID (Production preferred, first-created otherwise) and the project count of its company.
// @Tags Ugen
// @Produce json
// @Param limit query int false "limit"
// @Param offset query int false "offset"
// @Param search query string false "search by project title"
// @Success 200 {object} status_http.Response{data=models.ListUgenProjectsResp}
// @Failure 400 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *HandlerV1) ListUgenProjects(c *gin.Context) {

	userId, ok := c.Get("user_id")
	if !ok {
		h.HandleResponse(c, status_http.BadRequest, "user_id is required")
		return
	}

	if userId != config.UgenSuperAdminUserId {
		h.HandleResponse(c, status_http.Forbidden, "permission denied")
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid limit")
		return
	}
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid offset")
		return
	}

	resp, err := h.companyServices.Project().ListUgenProjects(c.Request.Context(), &pb.ListUgenProjectsRequest{
		Limit:  int32(limit),
		Offset: int32(offset),
		Search: c.Query("search"),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// ExportUgenProjects godoc
// @Security ApiKeyAuth
// @ID export_ugen_projects
// @Router /v1/ugen/projects/export [GET]
// @Summary Export all is_ugen projects to Excel (admin)
// @Description Generates an Excel file of all is_ugen projects and returns its download URL.
// @Tags Ugen
// @Produce json
// @Param search query string false "search by project title"
// @Success 200 {object} status_http.Response{data=company_service.ExportUgenProjectsResponse}
// @Failure 403 {object} status_http.Response{data=string}
// @Failure 500 {object} status_http.Response{data=string}
func (h *HandlerV1) ExportUgenProjects(c *gin.Context) {

	userId, ok := c.Get("user_id")
	if !ok {
		h.HandleResponse(c, status_http.BadRequest, "user_id is required")
		return
	}

	if userId != config.UgenSuperAdminUserId {
		h.HandleResponse(c, status_http.Forbidden, "permission denied")
		return
	}

	resp, err := h.companyServices.Project().ExportUgenProjects(c.Request.Context(), &pb.ExportUgenProjectsRequest{
		Search: c.Query("search"),
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

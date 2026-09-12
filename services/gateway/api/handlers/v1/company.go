package v1

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

// CreateCompany godoc
// @Security ApiKeyAuth
// @ID create_company
// @Router /v1/company [POST]
// @Summary Create Company
// @Description Create Company
// @Tags Company
// @Accept json
// @Produce json
// @Param Company body models.CompanyCreateRequest true "CompanyCreateRequest"
// @Success 201 {object} status_http.Response{data=pb.Company} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateCompany(c *gin.Context) {
	var (
		company models.CompanyCreateRequest
	)

	if err := c.ShouldBindJSON(&company); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	authInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	companyPKey, err := h.companyServices.Company().Create(
		c.Request.Context(), &pb.CreateCompanyRequest{
			Title:   company.Name,
			OwnerId: authInfo.GetUserIdAuth(),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	project, err := h.companyServices.Project().Create(
		c.Request.Context(), &pb.CreateProjectRequest{
			CompanyId:    companyPKey.GetId(),
			K8SNamespace: "cp-region-type-id",
			Title:        company.Name,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	_, err = h.companyServices.Environment().CreateV2(
		c.Request.Context(), &pb.CreateEnvironmentRequest{
			CompanyId:    companyPKey.GetId(),
			ProjectId:    project.ProjectId,
			UserId:       authInfo.GetUserIdAuth(),
			ClientTypeId: authInfo.GetClientTypeId(),
			RoleId:       authInfo.GetRoleId(),
			Name:         "Production",
			DisplayColor: "#00FF00",
			Description:  "Production Environment",
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, &pb.Company{
		Id:      companyPKey.GetId(),
		Name:    company.Name,
		OwnerId: authInfo.GetUserId(),
	})
}

// GetCompanyByID godoc
// @Security ApiKeyAuth
// @ID get_company_by_id
// @Router /v1/company/{company_id} [GET]
// @Summary Get Company by id
// @Description Get Company by id
// @Tags Company
// @Accept json
// @Produce json
// @Param company_id path string true "company_id"
// @Success 200 {object} status_http.Response{data=pb.GetCompanyByIdResponse} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetCompanyByID(c *gin.Context) {
	var companyId = c.Param("company_id")

	resp, err := h.companyServices.Company().GetById(
		c.Request.Context(), &pb.GetCompanyByIdRequest{
			Id: companyId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) GetCompanyList(c *gin.Context) {
	userProjects, err := h.authService.User().GetUserProjects(c.Request.Context(), &auth_service.UserPrimaryKey{
		Id: c.Query("owner_id"),
	})

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	companies := make([]*pb.Company, 0, len(userProjects.GetCompanies()))
	for _, company := range userProjects.GetCompanies() {
		companyFromService, err := h.companyServices.Company().GetById(
			c.Request.Context(), &pb.GetCompanyByIdRequest{Id: company.GetId()},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		companies = append(companies, companyFromService.Company)
	}

	h.HandleResponse(c, status_http.OK, &pb.GetComanyListResponse{
		Count:     int32(len(companies)),
		Companies: companies,
	})
}

func (h *HandlerV1) ListCompanies(c *gin.Context) {
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

	companies, err := h.companyServices.Company().GetList(
		c.Request.Context(), &pb.GetCompanyListRequest{
			Limit:  int32(limit),
			Offset: int32(offset),
			Search: c.Query("search"),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, companies)
}

func (h *HandlerV1) GetCompanyListWithProjects(c *gin.Context) {
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

	resp, err := h.companyServices.Company().GetListWithProjects(
		c.Request.Context(), &pb.GetListWithProjectsRequest{
			Limit:    int32(limit),
			Offset:   int32(offset),
			Search:   c.DefaultQuery("search", ""),
			ComanyId: c.DefaultQuery("company_id", ""),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// UpdateCompany godoc
// @Security ApiKeyAuth
// @ID update_company
// @Router /v1/company/{company_id} [PUT]
// @Summary Update company
// @Description Update company
// @Tags Company
// @Accept json
// @Produce json
// @Param company_id path string true "company_id"
// @Param Company body models.CompanyCreateRequest  true "CompanyCreateRequest"
// @Success 200 {object} status_http.Response{data=pb.Company} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateCompany(c *gin.Context) {
	var companyId = c.Param("company_id")

	if !util.IsValidUUID(companyId) {
		h.HandleResponse(c, status_http.BadRequest, errors.New("uuid invalid!!! : "+companyId))
		return
	}

	var company models.CompanyCreateRequest

	if err := c.ShouldBindJSON(&company); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Company().Update(
		c.Request.Context(), &pb.Company{
			Id:          companyId,
			Name:        company.Name,
			Logo:        company.Logo,
			Description: company.Description,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteCompany godoc
// @Security ApiKeyAuth
// @ID delete_company
// @Router /v1/company/{company_id} [DELETE]
// @Summary Delete Company
// @Description Delete Company
// @Tags Company
// @Accept json
// @Produce json
// @Param company_id path string true "company_id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteCompany(c *gin.Context) {
	var companyId = c.Param("company_id")

	resp, err := h.companyServices.Company().Delete(
		c.Request.Context(), &pb.DeleteCompanyRequest{Id: companyId},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, resp)
}

// CreateCompanyProject godoc
// @Security ApiKeyAuth
// @ID create_project
// @Router /v1/company-project [POST]
// @Summary Create Company
// @Description Create Company
// @Tags Company Project
// @Accept json
// @Produce json
// @Param Project body pb.CreateProjectRequest true "CompanyProjectCreateRequest"
// @Success 201 {object} status_http.Response{data=pb.CreateProjectResponse} "Project data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateCompanyProject(c *gin.Context) {
	var project pb.CreateProjectRequest

	if err := c.ShouldBindJSON(&project); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	resp, err := h.companyServices.Project().Create(
		c.Request.Context(), &pb.CreateProjectRequest{
			Title:        project.Title,
			K8SNamespace: project.K8SNamespace,
			CompanyId:    project.CompanyId,
			FareId:       project.FareId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	authInfo, err := h.adminAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	_, err = h.companyServices.Environment().CreateV2(
		c.Request.Context(), &pb.CreateEnvironmentRequest{
			CompanyId:    project.CompanyId,
			ProjectId:    resp.ProjectId,
			UserId:       authInfo.GetUserIdAuth(),
			ClientTypeId: authInfo.GetClientTypeId(),
			RoleId:       authInfo.GetRoleId(),
			Name:         "Production",
			DisplayColor: "#00FF00",
			Description:  "Production Environment",
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

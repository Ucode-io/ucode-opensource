package handlers

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"

	pba "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/saidamir98/udevs_pkg/util"

	"github.com/gin-gonic/gin"
)

// RegisterCompany godoc
// @ID create_company
// @Router /company [POST]
// @Summary Register Company
// @Description Register Company
// @Tags Company
// @Accept json
// @Produce json
// @Param company body pba.RegisterCompanyRequest true "RegisterCompanyRequestBody"
// @Success 201 {object} http.Response{data=pba.CompanyPrimaryKey} "Company data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) RegisterCompany(c *gin.Context) {
	var company pba.RegisterCompanyRequest

	if err := c.ShouldBindJSON(&company); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	// No billing in the open-source build, so no plan to put a company on.
	company.FareId = ""

	resp, err := h.services.CompanyService().Register(
		c.Request.Context(), &company,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// UpdateCompany godoc
// @ID update_company
// @Router /company [PUT]
// @Summary Update Company
// @Description Update Company
// @Tags Company
// @Accept json
// @Produce json
// @Param company body pba.UpdateCompanyRequest true "UpdateCompanyRequestBody"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UpdateCompany(c *gin.Context) {
	var company pba.UpdateCompanyRequest

	if err := c.ShouldBindJSON(&company); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.CompanyService().Update(
		c.Request.Context(), &company,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// RemoveCompany godoc
// @ID remove_company
// @Router /company/{company-id} [DELETE]
// @Summary Remove Company
// @Description Get Company
// @Tags Company
// @Accept json
// @Produce json
// @Param company-id path string true "company-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) RemoveCompany(c *gin.Context) {
	companyID := c.Param("company-id")

	if !util.IsValidUUID(companyID) {
		h.handleResponse(c, http.InvalidArgument, "company id is an invalid uuid")
		return
	}

	resp, err := h.services.CompanyService().Remove(
		c.Request.Context(), &pba.CompanyPrimaryKey{
			Id: companyID,
		},
	)

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

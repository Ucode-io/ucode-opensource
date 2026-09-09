package v2

import (
	"context"
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
)

// ExcelReader godoc
// @Security ApiKeyAuth
// @ID excel_fields
// @Router /v2/collections/{collection}/import/fields/{id} [GET]
// @Summary Get excel writer
// @Description Get excel writer
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id query string true "id"
// @Success 200 {object} status_http.Response{data=obs.ExcelReadResponse} "ExcelReadResponse"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) ExcelReader(c *gin.Context) {
	excelId := c.Param("id")
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
		resource.GetProjectId(),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var res *obs.ExcelReadResponse

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		res, err = services.GetBuilderServiceByType(resource.NodeType).Excel().ExcelRead(
			context.Background(),
			&obs.ExcelReadRequest{
				Id:        excelId,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}

	h.HandleResponse(c, status_http.OK, res)
}

// ImportData godoc
// @Security ApiKeyAuth
// @ID import_data
// @Router /v2/collections/{collection}/import/{id} [POST]
// @Summary Post excel writer
// @Description Post excel writer
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Param type query string false "type"
// @Param table body models.ExcelToDbRequest true "ImportDataRequest"
// @Success 200 {object} status_http.Response{data=models.ExcelToDbResponse} "ExcelToDbResponse"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) ImportData(c *gin.Context) {
	var excelRequest models.ExcelToDbRequest

	err := c.ShouldBindJSON(&excelRequest)
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
		resource.GetProjectId(),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	excelRequest.Data["company_service_project_id"] = resource.GetProjectId()
	excelRequest.Data["company_service_environment_id"] = resource.GetEnvironmentId()

	data, err := helper.ConvertMapToStruct(excelRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = services.GetBuilderServiceByType(resource.NodeType).Excel().ExcelToDb(
			context.Background(),
			&obs.ExcelToDbRequest{
				Id:        c.Param("id"),
				TableSlug: excelRequest.TableSlug,
				Data:      data,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}

	h.HandleResponse(c, status_http.Created, models.ExcelToDbResponse{
		Message: "Success!",
	})
}

package v1

import (
	"encoding/json"
	hHelper "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"

	"github.com/gin-gonic/gin"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/spf13/cast"
)

// ExcelReader godoc
// @Security ApiKeyAuth
// @ID excel_reader
// @Router /v1/excel/{excel_id} [GET]
// @Summary Get excel writer
// @Description Get excel writer
// @Tags Excel
// @Accept json
// @Produce json
// @Param excel_id path string true "excel_id"
// @Success 200 {object} status_http.Response{data=obs.ExcelReadResponse} "ExcelReadResponse"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ExcelReader(c *gin.Context) {
	var excelId = c.Param("excel_id")

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

	var res *obs.ExcelReadResponse

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		res, err = services.GetBuilderServiceByType(resource.NodeType).Excel().ExcelRead(
			c.Request.Context(), &obs.ExcelReadRequest{
				Id:        excelId,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, res)
	case pb.ResourceType_POSTGRESQL:
		res, err := services.GoObjectBuilderService().Excel().ExcelRead(
			c.Request.Context(), &nb.ExcelReadRequest{
				Id:        excelId,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, res)
	}
}

// ExcelToDb godoc
// @Security ApiKeyAuth
// @ID excel_to_db
// @Router /v1/excel/excel_to_db/{excel_id} [POST]
// @Summary Post excel writer
// @Description Post excel writer
// @Tags Excel
// @Accept json
// @Produce json
// @Param excel_id path string true "excel_id"
// @Param table body models.ExcelToDbRequest true "ExcelToDbRequest"
// @Success 200 {object} status_http.Response{data=models.ExcelToDbResponse} "ExcelToDbResponse"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) ExcelToDb(c *gin.Context) {
	var (
		excelRequest models.ExcelToDbRequest
	)

	if err := c.ShouldBindJSON(&excelRequest); err != nil {
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
	excelRequest.Data["company_service_project_id"] = resource.GetProjectId()
	excelRequest.Data["company_service_environment_id"] = resource.GetEnvironmentId()

	data, err := helper.ConvertMapToStruct(excelRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	beforeActions, afterActions, getEventsErr := hHelper.GetListCustomEvents(models.GetListCustomEventsStruct{
		TableSlug: excelRequest.TableSlug,
		Method:    "EXCEL_IMPORT",
		Resource:  resource,
	}, c, h)
	if getEventsErr != nil {
		h.HandleResponse(c, status_http.InvalidArgument, getEventsErr.Error())
		return
	}

	objectData := map[string]any{
		"bucket": resource.ResourceEnvironmentId,
	}

	actionRequest := models.DoInvokeFunctionStruct{
		Services:  services.GoObjectBuilderService(),
		TableSlug: excelRequest.TableSlug,
		Method:    "EXCEL_IMPORT",
		Resource:  resource,
	}

	if len(beforeActions) > 0 {
		actionRequest.ObjectData = objectData
		actionRequest.CustomEvents = beforeActions
		actionRequest.ActionType = "BEFORE"
		if _, err = hHelper.DoInvokeFunction(actionRequest, c, h); err != nil {
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	var logReq = &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: "EXCEL",
		ActionType:   "IMPORT EXCEL",
		UserInfo:     cast.ToString(userId),
		Request:      &excelRequest,
		TableSlug:    excelRequest.TableSlug,
	}

	defer func() {
		go h.versionHistoryGo(c, logReq)
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = services.GetBuilderServiceByType(resource.NodeType).Excel().ExcelToDb(
			c.Request.Context(), &obs.ExcelToDbRequest{
				Id:        c.Param("excel_id"),
				TableSlug: excelRequest.TableSlug,
				Data:      data,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Excel().ExcelToDb(
			c.Request.Context(), &nb.ExcelToDbRequest{
				Id:        c.Param("excel_id"),
				TableSlug: excelRequest.TableSlug,
				Data:      data,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		rowsMap, err := convert[[]*structpb.Struct, []map[string]any](resp.Rows)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
		objectData["rows"] = rowsMap
	}

	if len(afterActions) > 0 {
		actionRequest.ObjectData = objectData
		actionRequest.CustomEvents = afterActions
		actionRequest.ActionType = "AFTER"
		if _, err := hHelper.DoInvokeFunction(actionRequest, c, h); err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.InvalidArgument, err.Error())
			return
		}
	}

	h.HandleResponse(c, status_http.OK, models.ExcelToDbResponse{Message: "Success"})
}

func Convert[T any, U any](in T) (U, error) {
	var out U

	data, err := json.Marshal(in)
	if err != nil {
		return out, err
	}

	err = json.Unmarshal(data, &out)
	return out, err
}

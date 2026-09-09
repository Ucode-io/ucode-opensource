package v2

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/xuri/excelize/v2"
)

// GetByUDVersionHistory godoc
// @Security ApiKeyAuth
// @ID get_version_history_by_id
// @Router /v2/version/history/{environment_id}/{id} [GET]
// @Summary Get single version history
// @Description Get single version history
// @Tags VersionHistory
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=obs.VersionHistory} "VersionHistoryBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetVersionHistoryByID(c *gin.Context) {
	id := c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "version history id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, errors.New("error getting environment id | not valid"))
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).VersionHistory().GetByID(
			c.Request.Context(),
			&obs.VersionHistoryPrimaryKey{
				Id:        id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().VersionHistory().GetByID(
			c.Request.Context(),
			&nb.VersionHistoryPrimaryKey{
				Id:        id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}

}

// GetAllVersionHistory godoc
// @Security ApiKeyAuth
// @ID get_all_version_history
// @Router /v2/version/history/{environment_id} [GET]
// @Summary Get version history list
// @Description Get version history list
// @Tags VersionHistory
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Param filters query obs.GetAllRquest true "filters"
// @Success 200 {object} status_http.Response{data=obs.ListVersionHistory} "ViewBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllVersionHistory(c *gin.Context) {

	var (
		fromDate, toDate string
		orderby          bool
	)
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

	apiKey := c.Query("api_key")
	actionType := c.Query("action_type")
	collection := c.Query("collection")
	userInfo := c.Query("user_info")

	if c.Query("from_date") != "" {
		formatFromDate, err := time.Parse("2006-01-02", c.Query("from_date"))
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		fromDate = formatFromDate.Format("2006-01-02")
	}

	if c.Query("to_date") != "" {
		formatToDate, err := time.Parse("2006-01-02", c.Query("to_date"))
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		toDate = formatToDate.Format("2006-01-02")
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	currEnvironmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(currEnvironmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: currEnvironmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	tip := strings.ToUpper(c.Query("type"))
	if tip == "UP" {
		orderby = false
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).VersionHistory().GatAll(c.Request.Context(),
			&obs.GetAllRquest{
				Type:       tip,
				ProjectId:  resource.ResourceEnvironmentId,
				EnvId:      currEnvironmentId.(string),
				ApiKey:     apiKey,
				Offset:     int32(offset),
				Limit:      int32(limit),
				FromDate:   fromDate,
				ToDate:     toDate,
				OrderBy:    orderby,
				ActionType: actionType,
				Collection: collection,
				UserInfo:   userInfo,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().VersionHistory().GatAll(
			c.Request.Context(),
			&nb.GetAllRquest{
				Type:       tip,
				ProjectId:  resource.ResourceEnvironmentId,
				EnvId:      currEnvironmentId.(string),
				ApiKey:     apiKey,
				Offset:     int32(offset),
				Limit:      int32(limit),
				FromDate:   fromDate,
				ToDate:     toDate,
				OrderBy:    orderby,
				ActionType: actionType,
				Collection: collection,
				UserInfo:   userInfo,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}

}

// UpdateListVersionHistory godoc
// @Security ApiKeyAuth
// @ID update_version_history
// @Router /v2/version/history/{environment_id} [PUT]
// @Summary Update version history
// @Description Update version history
// @Tags VersionHistory
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Param view body obs.UsedForEnvRequest true "UpdateViewRequestBody"
// @Success 200 {object} status_http.Response{data=obs.UsedForEnvRequest} "UsedForEnvRequest data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateVersionHistory(c *gin.Context) {
	var (
		view obs.UsedForEnvRequest
	)

	err := c.ShouldBindJSON(&view)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId := c.Param("environment_id")
	if !util.IsValidUUID(environmentId) {
		err := errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: environmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	view.ProjectId = resource.ResourceEnvironmentId

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = services.GetBuilderServiceByType(resource.NodeType).VersionHistory().Update(
			c.Request.Context(),
			&view,
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// resp, err = services.PostgresBuilderService().View().Update(
		// 	c.Request.Context(),
		// 	&view,
		// )

		// if err != nil {
		// 	h.HandleResponse(c, status_http.GRPCError, err.Error())
		// 	return
		// }
	}

	h.HandleResponse(c, status_http.OK, nil)
}

// VersionHistoryExcelDownload godoc
// @Security ApiKeyAuth
// @ID version_history_excel_download
// @Router /v2/version/history/{environment_id}/excel [GET]
// @Summary Get version history list in excel format
// @Description Get version history list in excel format
// @Tags VersionHistory
// @Accept json
// @Produce json
// @Param environment_id path string true "environment_id"
// @Param filters query obs.GetAllRquest true "filters"
// @Success 200 {object} status_http.Response{data=object} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) VersionHistoryExcelDownload(c *gin.Context) {
	var (
		response         = map[string]string{}
		fileName         = fmt.Sprintf("report_%d.xlsx", time.Now().Unix())
		fromDate, toDate string
		orderby          bool
		resp             *obs.ListVersionHistory
	)
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

	apiKey := c.Query("api_key")
	actionType := c.Query("action_type")
	collection := c.Query("collection")
	userInfo := c.Query("user_info")

	if c.Query("from_date") != "" {
		formatFromDate, err := time.Parse("2006-01-02", c.Query("from_date"))
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		fromDate = formatFromDate.Format("2006-01-02")
	}

	if c.Query("to_date") != "" {
		formatToDate, err := time.Parse("2006-01-02", c.Query("to_date"))
		if err != nil {
			h.HandleResponse(c, status_http.BadRequest, err.Error())
			return
		}

		toDate = formatToDate.Format("2006-01-02")
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	currEnvironmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(currEnvironmentId.(string)) {
		err = errors.New("error getting environment id | not valid")
		h.HandleResponse(c, status_http.BadRequest, err)
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId.(string),
			EnvironmentId: currEnvironmentId.(string),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	tip := strings.ToUpper(c.Query("type"))
	if tip == "UP" {
		orderby = false
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).VersionHistory().GatAll(c.Request.Context(),
			&obs.GetAllRquest{
				Type:       tip,
				ProjectId:  resource.ResourceEnvironmentId,
				EnvId:      currEnvironmentId.(string),
				ApiKey:     apiKey,
				Offset:     int32(offset),
				Limit:      int32(20000),
				FromDate:   fromDate,
				ToDate:     toDate,
				OrderBy:    orderby,
				ActionType: actionType,
				Collection: collection,
				UserInfo:   userInfo,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

	case pb.ResourceType_POSTGRESQL:
		goResp, err := services.GoObjectBuilderService().VersionHistory().GatAll(
			c.Request.Context(),
			&nb.GetAllRquest{
				Type:       tip,
				ProjectId:  resource.ResourceEnvironmentId,
				EnvId:      currEnvironmentId.(string),
				ApiKey:     apiKey,
				Offset:     int32(offset),
				Limit:      int32(limit),
				FromDate:   fromDate,
				ToDate:     toDate,
				OrderBy:    orderby,
				ActionType: actionType,
				Collection: collection,
				UserInfo:   userInfo,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		//convert goResp to resp
		goRespByte, err := json.Marshal(goResp)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		err = json.Unmarshal(goRespByte, &resp)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
	}

	f := excelize.NewFile()
	sheetName := "Users"
	f.SetSheetName("Sheet1", sheetName)

	headers := []string{"table_slug", "date", "action_type", "user_info", "action_source", "api_key", "type"}

	// this is static solution
	for colIdx, header := range headers {
		cell := fmt.Sprintf("%c1", 'A'+colIdx)
		f.SetCellValue(sheetName, cell, header)
	}

	for rowIdx, hst := range resp.Histories {
		f.SetCellValue(sheetName, fmt.Sprintf("A%d", rowIdx+2), hst.TableSlug)
		f.SetCellValue(sheetName, fmt.Sprintf("B%d", rowIdx+2), hst.Date)
		f.SetCellValue(sheetName, fmt.Sprintf("C%d", rowIdx+2), hst.ActionType)
		f.SetCellValue(sheetName, fmt.Sprintf("D%d", rowIdx+2), hst.UserInfo)
		f.SetCellValue(sheetName, fmt.Sprintf("E%d", rowIdx+2), hst.ActionSource)
		f.SetCellValue(sheetName, fmt.Sprintf("F%d", rowIdx+2), hst.ApiKey)
		f.SetCellValue(sheetName, fmt.Sprintf("G%d", rowIdx+2), hst.Type)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: h.baseConf.MinioProtocol,
	})

	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	_, err = minioClient.PutObject(
		c.Request.Context(),
		resource.ResourceEnvironmentId,
		fileName,
		bytes.NewReader(buf.Bytes()),
		int64(buf.Len()),
		minio.PutObjectOptions{ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"},
	)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	response["link"] = fmt.Sprintf("%s/%s/%s", h.baseConf.MinioEndpoint, resource.ResourceEnvironmentId, fileName)
	h.HandleResponse(c, status_http.OK, response)

}

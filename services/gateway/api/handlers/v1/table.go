package v1

import (
	"encoding/json"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/structpb"
)

type tableAdditionalRequest struct {
	AdditionalField  string   `json:"additional_field"`
	AdditionalValues []string `json:"additional_values"`
}

// CreateTable godoc
// @Security ApiKeyAuth
// @ID create_table
// @Router /v1/table [POST]
// @Summary Create table
// @Description Create table
// @Tags Table
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param table body models.CreateTableRequest true "CreateTableRequestBody"
// @Success 201 {object} status_http.Response{data=obs.Table} "Table data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) CreateTable(c *gin.Context) {
	var (
		table                 obs.CreateTableRequest
		err                   error
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
	)

	if err = c.ShouldBindJSON(&table); err != nil {
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

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	table.ProjectId = resourceEnvironmentId
	table.ViewId = uuid.NewString()
	table.LayoutId = uuid.NewString()

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "TABLE",
			ActionType:   "CREATE TABLE",
			UserInfo:     cast.ToString(userId),
			Request:      &table,
			TableSlug:    table.Slug,
		}
	)

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(nodeType).Table().Create(
			c.Request.Context(),
			&table,
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			table.Id = resp.Id
			logReq.Current = &table
			logReq.Response = &table
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		newReq := nb.CreateTableRequest{}

		if err = helper.MarshalToStruct(&table, &newReq); err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		}

		newReq.UcodeProjectId = resource.ProjectId

		resp, err := services.GoObjectBuilderService().Table().Create(
			c.Request.Context(),
			&newReq,
		)
		if err != nil {
			logReq.Response = err.Error()
			if st, ok := grpcstatus.FromError(err); ok && st.Code() == codes.ResourceExhausted {
				h.HandleResponse(c, status_http.PaymentRequired, models.PaymentTableLimit)
			} else {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
			}
		} else {
			table.Id = resp.Id
			logReq.Current = &table
			logReq.Response = &table
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// GetTableByID godoc
// @Security ApiKeyAuth
// @ID get_table_by_id
// @Router /v1/table/{table_id} [GET]
// @Summary Get table by id
// @Description Get table by id
// @Tags Table
// @Accept json
// @Produce json
// @Param table_id path string true "table_id"
// @Success 200 {object} status_http.Response{data=models.CreateTableResponse} "TableBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetTableByID(c *gin.Context) {
	var (
		tableID               = c.Param("table_id")
		err                   error
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
	)

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

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(nodeType).Table().GetByID(
			c.Request.Context(),
			&obs.TablePrimaryKey{
				Id:        tableID,
				ProjectId: resourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().GetByID(
			c.Request.Context(), &nb.TablePrimaryKey{
				Id:        tableID,
				ProjectId: resourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

// GetAllTables godoc
// @Security ApiKeyAuth
// @ID get_all_tables
// @Router /v1/table [GET]
// @Summary Get all tables
// @Description Get all tables
// @Tags Table
// @Accept json
// @Produce json
// @Param filters query models.GetAllTablesRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetAllTablesResponse} "TableBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllTables(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
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

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	var isLoginTable bool
	var isLoginTableStr = c.Query("is_login_table")
	if isLoginTableStr == "true" {
		isLoginTable = true
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(nodeType).Table().GetAll(
			c.Request.Context(), &obs.GetAllTablesRequest{
				Limit:        int32(limit),
				Offset:       int32(offset),
				Search:       c.DefaultQuery("search", ""),
				ProjectId:    resourceEnvironmentId,
				FolderId:     c.Query("folder_id"),
				IsLoginTable: isLoginTable,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().GetAll(
			c.Request.Context(), &nb.GetAllTablesRequest{
				Limit:        int32(limit),
				Offset:       int32(offset),
				Search:       c.DefaultQuery("search", ""),
				ProjectId:    resourceEnvironmentId,
				IsLoginTable: isLoginTable,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

// GetAllTablesWithAdditional godoc
// @Security ApiKeyAuth
// @ID get_all_tables_with_additional
// @Router /v1/table-list [GET]
// @Summary Get all tables with additional selected tables
// @Description Get all tables with selected tables prepended from additional_request
// @Tags Table
// @Accept json
// @Produce json
// @Param filters query models.GetAllTablesRequest true "filters"
// @Param additional_request query string false "JSON object with additional_field and additional_values"
// @Param additional_field query string false "Selected table field, currently id"
// @Param additional_values query []string false "Selected table ids"
// @Success 200 {object} status_http.Response{data=obs.GetAllTablesResponse} "TableBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetAllTablesWithAdditional(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
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

	additionalRequest, err := parseTableAdditionalRequest(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
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

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	var isLoginTable bool
	var isLoginTableStr = c.Query("is_login_table")
	if isLoginTableStr == "true" {
		isLoginTable = true
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resourceType {
	case pb.ResourceType_MONGODB:
		tableService := services.GetBuilderServiceByType(nodeType).Table()
		resp, err := tableService.GetAll(
			c.Request.Context(), &obs.GetAllTablesRequest{
				Limit:        int32(limit),
				Offset:       int32(offset),
				Search:       c.DefaultQuery("search", ""),
				ProjectId:    resourceEnvironmentId,
				FolderId:     c.Query("folder_id"),
				IsLoginTable: isLoginTable,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		selectedTables := make([]*obs.Table, 0, len(additionalRequest.AdditionalValues))
		for _, id := range additionalRequest.AdditionalValues {
			table, err := tableService.GetByID(
				c.Request.Context(),
				&obs.TablePrimaryKey{
					Id:        id,
					ProjectId: resourceEnvironmentId,
				},
			)
			if err != nil {
				if st, ok := grpcstatus.FromError(err); ok && st.Code() == codes.NotFound {
					continue
				}
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			selectedTables = append(selectedTables, table)
		}
		resp.Tables = mergeObsTables(selectedTables, resp.GetTables())

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		tableService := services.GoObjectBuilderService().Table()
		resp, err := tableService.GetAll(
			c.Request.Context(), &nb.GetAllTablesRequest{
				Limit:        int32(limit),
				Offset:       int32(offset),
				Search:       c.DefaultQuery("search", ""),
				ProjectId:    resourceEnvironmentId,
				IsLoginTable: isLoginTable,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		selectedTables := make([]*nb.Table, 0, len(additionalRequest.AdditionalValues))
		for _, id := range additionalRequest.AdditionalValues {
			table, err := tableService.GetByID(
				c.Request.Context(), &nb.TablePrimaryKey{
					Id:        id,
					ProjectId: resourceEnvironmentId,
				},
			)
			if err != nil {
				if st, ok := grpcstatus.FromError(err); ok && st.Code() == codes.NotFound {
					continue
				}
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			selectedTables = append(selectedTables, table)
		}
		resp.Tables = mergeNbTables(selectedTables, resp.GetTables())

		h.HandleResponse(c, status_http.OK, resp)
	}
}

func parseTableAdditionalRequest(c *gin.Context) (tableAdditionalRequest, error) {
	req := tableAdditionalRequest{}

	if raw := c.Query("additional_request"); raw != "" {
		if err := json.Unmarshal([]byte(raw), &req); err != nil {
			return req, err
		}
	}

	if field := c.Query("additional_field"); field != "" {
		req.AdditionalField = field
	}

	if values := c.QueryArray("additional_values"); len(values) > 0 {
		req.AdditionalValues = splitQueryValues(values)
	}

	if req.AdditionalField == "" && len(req.AdditionalValues) == 0 {
		return req, nil
	}

	if req.AdditionalField != "id" {
		return req, fmt.Errorf("additional_field must be id")
	}

	req.AdditionalValues = uniqueValidUUIDs(req.AdditionalValues)

	return req, nil
}

func splitQueryValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			result = append(result, part)
		}
	}

	return result
}

func uniqueValidUUIDs(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))

	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || !util.IsValidUUID(value) {
			continue
		}

		if _, ok := seen[value]; ok {
			continue
		}

		seen[value] = struct{}{}
		result = append(result, value)
	}

	return result
}

func mergeObsTables(selectedTables, tables []*obs.Table) []*obs.Table {
	seen := make(map[string]struct{}, len(selectedTables)+len(tables))
	result := make([]*obs.Table, 0, len(selectedTables)+len(tables))

	for _, table := range selectedTables {
		if table == nil || table.GetId() == "" {
			continue
		}
		if _, ok := seen[table.GetId()]; ok {
			continue
		}

		seen[table.GetId()] = struct{}{}
		result = append(result, table)
	}

	for _, table := range tables {
		if table == nil || table.GetId() == "" {
			continue
		}
		if _, ok := seen[table.GetId()]; ok {
			continue
		}

		seen[table.GetId()] = struct{}{}
		result = append(result, table)
	}

	return result
}

func mergeNbTables(selectedTables, tables []*nb.Table) []*nb.Table {
	seen := make(map[string]struct{}, len(selectedTables)+len(tables))
	result := make([]*nb.Table, 0, len(selectedTables)+len(tables))

	for _, table := range selectedTables {
		if table == nil || table.GetId() == "" {
			continue
		}
		if _, ok := seen[table.GetId()]; ok {
			continue
		}

		seen[table.GetId()] = struct{}{}
		result = append(result, table)
	}

	for _, table := range tables {
		if table == nil || table.GetId() == "" {
			continue
		}
		if _, ok := seen[table.GetId()]; ok {
			continue
		}

		seen[table.GetId()] = struct{}{}
		result = append(result, table)
	}

	return result
}

// UpdateTable godoc
// @Security ApiKeyAuth
// @ID update_table
// @Router /v1/table [PUT]
// @Summary Update table
// @Description Update table
// @Tags Table
// @Accept json
// @Produce json
// @Param table body models.UpdateTableRequest  true "UpdateTableRequestBody"
// @Success 200 {object} status_http.Response{data=obs.Table} "Table data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateTable(c *gin.Context) {
	var (
		table                 models.UpdateTableRequest
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
	)

	if err := c.ShouldBindJSON(&table); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	authInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("error getting auth info: %w", err).Error())
		return
	}

	table.AuthorId = authInfo.GetUserId()
	table.Name = fmt.Sprintf("Auto Created Commit Update table - %s", time.Now().Format(time.RFC1123))
	table.CommitType = config.COMMIT_TYPE_TABLE

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

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	structData, err := helper.ConvertMapToStruct(table.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err)
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	table.ProjectId = resourceEnvironmentId

	var (
		updateTable = &obs.UpdateTableRequest{
			Id:                table.Id,
			Description:       table.Description,
			Label:             table.Label,
			Slug:              table.Slug,
			ShowInMenu:        table.ShowInMenu,
			Icon:              table.Icon,
			SubtitleFieldSlug: table.SubtitleFieldSlug,
			IsVisible:         table.IsVisible,
			IsOwnTable:        table.IsOwnTable,
			IncrementId: &obs.IncrementID{
				WithIncrementId: table.IncrementId.WithIncrementID,
				DigitNumber:     table.IncrementId.DigitNumber,
				Prefix:          table.IncrementId.Prefix,
			},
			ProjectId:    table.ProjectId,
			FolderId:     table.FolderId,
			AuthorId:     table.AuthorId,
			CommitType:   table.CommitType,
			Name:         table.Name,
			IsCached:     table.IsCached,
			IsLoginTable: table.IsLoginTable,
			Attributes:   structData,
			OrderBy:      table.OrderBy,
			SoftDelete:   table.SoftDelete,
		}

		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "TABLE",
			ActionType:   "UPDATE TABLE",
			UserInfo:     cast.ToString(userId),
			Request:      &updateTable,
			TableSlug:    table.Slug,
		}
	)

	switch resourceType {
	case pb.ResourceType_MONGODB:
		oldTable, err := services.GetBuilderServiceByType(nodeType).Table().GetByID(
			c.Request.Context(), &obs.TablePrimaryKey{
				Id:        table.Id,
				ProjectId: table.ProjectId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err := services.GetBuilderServiceByType(nodeType).Table().Update(
			c.Request.Context(), updateTable,
		)

		logReq.Previous = oldTable
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.OK, resp)
		}

		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		oldTable, err := services.GoObjectBuilderService().Table().GetByID(
			c.Request.Context(), &nb.TablePrimaryKey{
				Id:        table.Id,
				ProjectId: table.ProjectId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		newTable := nb.UpdateTableRequest{}

		err = helper.MarshalToStruct(&updateTable, &newTable)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		}

		resp, err := services.GoObjectBuilderService().Table().Update(
			c.Request.Context(), &newTable,
		)

		logReq.Previous = oldTable
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// DeleteTable godoc
// @Security ApiKeyAuth
// @ID delete_table
// @Router /v1/table/{table_id} [DELETE]
// @Summary Delete Table
// @Description Delete Table
// @Tags Table
// @Accept json
// @Produce json
// @Param table_id path string true "table_id"
// @Param project-id query string true "project-id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteTable(c *gin.Context) {
	var (
		tableID               = c.Param("table_id")
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
	)

	if !util.IsValidUUID(tableID) {
		h.HandleResponse(c, status_http.InvalidArgument, "table id is an invalid uuid")
		return
	}

	authInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("error getting auth info: %w", err).Error())
		return
	}

	resourceId, resourceIdOk := c.Get("resource_id")

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

	if !resourceIdOk {
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

		resourceEnvironmentId = resource.ResourceEnvironmentId
		resourceType = resource.ResourceType
		nodeType = resource.NodeType
	} else {
		resourceEnvironment, err := h.companyServices.Resource().GetResourceEnvironment(
			c.Request.Context(),
			&pb.GetResourceEnvironmentReq{
				EnvironmentId: environmentId.(string),
				ResourceId:    resourceId.(string),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resourceEnvironmentId = resourceEnvironment.GetId()
		resourceType = pb.ResourceType(resourceEnvironment.ResourceType)
		nodeType = resourceEnvironment.GetNodeType()
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		oldTable = &obs.Table{}
		logReq   = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "TABLE",
			ActionType:   "DELETE TABLE",
			UserInfo:     cast.ToString(userId),
			TableSlug:    tableID,
		}
	)

	switch resourceType {
	case pb.ResourceType_MONGODB:
		oldTable, err = services.GetBuilderServiceByType(nodeType).Table().GetByID(
			c.Request.Context(), &obs.TablePrimaryKey{
				Id:        tableID,
				ProjectId: resourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err := services.GetBuilderServiceByType(nodeType).Table().Delete(
			c.Request.Context(), &obs.TablePrimaryKey{
				Id:         tableID,
				ProjectId:  resourceEnvironmentId,
				AuthorId:   authInfo.GetUserId(),
				Name:       fmt.Sprintf("Auto Created Commit Delete table - %s", time.Now().Format(time.RFC1123)),
				CommitType: config.COMMIT_TYPE_TABLE,
				EnvId:      environmentId.(string),
			},
		)

		logReq.Previous = oldTable
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)

	case pb.ResourceType_POSTGRESQL:
		oldTable, err := services.GoObjectBuilderService().Table().GetByID(
			c.Request.Context(), &nb.TablePrimaryKey{
				Id:        tableID,
				ProjectId: resourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleError(c, status_http.InternalServerError, err)
			return
		}

		resp, err := services.GoObjectBuilderService().Table().Delete(
			c.Request.Context(), &nb.TablePrimaryKey{
				Id:        tableID,
				ProjectId: resourceEnvironmentId,
			},
		)

		logReq.Previous = oldTable
		if err != nil {
			logReq.Response = err.Error()
			h.handleError(c, status_http.InternalServerError, err)
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// GetTableDetails godoc
// @Security ApiKeyAuth
// @ID get_table_details
// @Router /v1/table-details/{collection} [POST]
// @Summary Get table details
// @Description Get table details
// @Tags Object
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param projectId query string true "projectId"
// @Param language_setting query string false "language_setting"
// @Param object body models.CommonMessage true "GetListObjectRequestBody"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "ObjectBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetTableDetails(c *gin.Context) {
	var (
		objectRequest models.CommonMessage
		statusHttp    = status_http.GrpcStatusToHTTP["Ok"]
		tableSlug     = c.Param("collection")
	)

	if err := c.ShouldBindJSON(&objectRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}
	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
		return
	}
	if tokenInfo != nil {
		if tokenInfo.Tables != nil {
			objectRequest.Data["tables"] = tokenInfo.GetTables()
		}
		objectRequest.Data["user_id_from_token"] = tokenInfo.GetUserId()
		objectRequest.Data["role_id_from_token"] = tokenInfo.GetRoleId()
		objectRequest.Data["client_type_id_from_token"] = tokenInfo.GetClientTypeId()
	}
	objectRequest.Data["language_setting"] = c.DefaultQuery("language_setting", "")

	structData, err := helper.ConvertMapToStruct(objectRequest.Data)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).ObjectBuilder().GetTableDetails(
			c.Request.Context(), &obs.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().ObjectBuilder().GetTableDetails(
			c.Request.Context(), &nb.CommonMessage{
				TableSlug: tableSlug,
				Data:      structData,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleError(c, status_http.GRPCError, err)
			return
		}

		statusHttp.CustomMessage = resp.GetCustomMessage()
		h.HandleResponse(c, statusHttp, resp)
	}
}

func (h *HandlerV1) GetChart(c *gin.Context) {
	statusHttp := status_http.GrpcStatusToHTTP["Ok"]

	projectId := c.DefaultQuery("project-id", "")
	if !util.IsValidUUID(projectId) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId := c.DefaultQuery("environment-id", "")
	if !util.IsValidUUID(environmentId) {
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     projectId,
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetChart(
			c.Request.Context(), &obs.ChartPrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, statusHttp, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().GetChart(
			c.Request.Context(), &nb.ChartPrimaryKey{
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, statusHttp, resp)
	}
}

func (h *HandlerV1) TrackTables(c *gin.Context) {
	var (
		resourceEnvironmentId string
		resourceType          pb.ResourceType
		nodeType              string
	)

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

	resourceEnvironmentId = resource.ResourceEnvironmentId
	resourceType = resource.ResourceType
	nodeType = resource.NodeType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resEnv, err := h.companyServices.Resource().GetResource(c.Request.Context(), &pb.GetResourceRequest{
		Id: resourceEnvironmentId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp := &nb.GetAllTablesResponse{}

	var tables []models.Tables

	switch resourceType {
	case pb.ResourceType_MONGODB:
	case pb.ResourceType_POSTGRESQL:
		resp, err = services.GoObjectBuilderService().Table().GetAll(
			c.Request.Context(), &nb.GetAllTablesRequest{
				Limit:     1000,
				Offset:    0,
				ProjectId: resourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	for _, val := range resp.GetTables() {
		tables = append(tables, models.Tables{
			Table: models.Table{
				Name:   val.GetSlug(),
				Schema: "public",
			},
			Source: resEnv.GetCredentials().GetDatabase(),
		})
	}

	tableArgs := models.TableArgs{
		Args: models.Args{
			Tables:        tables,
			AllowWarnings: true,
		},
		Type: "postgres_track_tables",
	}

	trackTableRequest := models.TrackRequest{
		Type:   "bulk",
		Source: resEnv.GetCredentials().GetDatabase(),
		Args:   []models.TableArgs{tableArgs},
	}

	headers := map[string]string{
		"Content-Type": "application/json",
	}

	respByte, err := util.DoDynamicRequest("", headers, http.MethodPost, trackTableRequest)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, string(respByte))
}

func (h *HandlerV1) CreateConnectionAndSchema(c *gin.Context) {
	var (
		request nb.CreateConnectionAndSchemaReq
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     cast.ToString(projectId),
			EnvironmentId: cast.ToString(environmentId),
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
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().CreateConnectionAndSchema(
			c.Request.Context(),
			&nb.CreateConnectionAndSchemaReq{
				Name:             request.Name,
				ConnectionString: request.ConnectionString,
				ProjectId:        resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

func (h *HandlerV1) GetTrackedUntrackedTables(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	connectionId := c.Param("connection_id")

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     cast.ToString(projectId),
			EnvironmentId: cast.ToString(environmentId),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().GetTrackedUntrackedTables(
			c.Request.Context(), &nb.GetTrackedUntrackedTablesReq{
				ProjectId:    resource.ResourceEnvironmentId,
				ConnectionId: connectionId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
		return
	}
}

func (h *HandlerV1) GetTrackedConnections(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     cast.ToString(projectId),
			EnvironmentId: cast.ToString(environmentId),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().GetTrackedConnections(
			c.Request.Context(), &nb.GetTrackedConnectionsReq{
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
		return
	}
}

func (h *HandlerV1) TrackTablesByIds(c *gin.Context) {
	var (
		request      nb.TrackedTablesByIdsReq
		connectionId = c.Param("connection_id")
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     cast.ToString(projectId),
			EnvironmentId: cast.ToString(environmentId),
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
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().TrackTables(
			c.Request.Context(),
			&nb.TrackedTablesByIdsReq{
				TableIds:     request.TableIds,
				ConnectionId: connectionId,
				ProjectId:    resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

func (h *HandlerV1) UntrackTableById(c *gin.Context) {
	var (
		connectionId = c.Param("connection_id")
		tableId      = c.Param("table_id")
	)

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "environment id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			ProjectId:     cast.ToString(projectId),
			EnvironmentId: cast.ToString(environmentId),
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
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Table().UntrackTableById(
			c.Request.Context(),
			&nb.UntrackTableByIdReq{
				TableId:      tableId,
				ProjectId:    resource.ResourceEnvironmentId,
				ConnectionId: connectionId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

func (h *HandlerV1) UpdateTableByMCP(c *gin.Context) {
	var (
		tableSlug = c.Param("collection")
		request   models.TableMCP
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_POSTGRESQL:
		for _, field := range request.Fields {
			fieldType := GetFieldType(strings.ToLower(field.Type))
			attributes := &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"label":    structpb.NewStringValue(formatString(field.Slug)),
					"label_en": structpb.NewStringValue(formatString(field.Slug)),
				},
			}

			if fieldType == "MULTISELECT" {
				options := make([]*structpb.Value, len(field.Enum))
				for i, value := range field.Enum {
					option, _ := structpb.NewStruct(map[string]any{
						"value": value,
						"icon":  "",
						"color": "",
						"label": value,
					})
					options[i] = structpb.NewStructValue(option)
				}

				attributes.Fields["options"] = structpb.NewListValue(&structpb.ListValue{
					Values: options,
				})
			}

			switch field.Action {
			case "create":
				_, err = services.GoObjectBuilderService().Field().Create(
					c.Request.Context(),
					&nb.CreateFieldRequest{
						Id:         uuid.NewString(),
						TableId:    tableSlug,
						Type:       fieldType,
						Label:      formatString(field.Slug),
						Slug:       field.Slug,
						Attributes: attributes,
						ProjectId:  resource.ResourceEnvironmentId,
						EnvId:      resource.EnvironmentId,
					},
				)
				if err != nil {
					continue
				}
			case "update":
				_, err = services.GoObjectBuilderService().Field().Update(
					c.Request.Context(),
					&nb.Field{
						Id:         field.Slug,
						TableId:    tableSlug,
						Type:       fieldType,
						Label:      formatString(field.Slug),
						Attributes: attributes,
						ProjectId:  resource.ResourceEnvironmentId,
						EnvId:      resource.EnvironmentId,
					},
				)
			case "delete":
				_, err = services.GoObjectBuilderService().Field().Delete(
					c.Request.Context(),
					&nb.FieldPrimaryKey{
						Id:        field.Slug,
						ProjectId: resource.ResourceEnvironmentId,
						TableSlug: tableSlug,
					},
				)
			}
		}

		for _, relation := range request.Relations {
			switch relation.Action {
			case "create":
				viewField, err := services.GoObjectBuilderService().Field().ObtainRandomOne(
					c.Request.Context(),
					&nb.ObtainRandomRequest{
						TableSlug: tableSlug,
						ProjectId: resource.ResourceEnvironmentId,
						EnvId:     resource.EnvironmentId,
					},
				)
				_, err = services.GoObjectBuilderService().Relation().Create(
					c.Request.Context(),
					&nb.CreateRelationRequest{
						Id:        uuid.NewString(),
						Type:      relation.Type,
						TableFrom: tableSlug,
						TableTo:   relation.TableTo,
						Attributes: &structpb.Struct{
							Fields: map[string]*structpb.Value{
								"label_en":    structpb.NewStringValue(formatString(relation.TableTo)),
								"label_to_en": structpb.NewStringValue(formatString(tableSlug)),
							},
						},
						RelationFieldId:   uuid.NewString(),
						RelationTableSlug: relation.TableTo,
						RelationToFieldId: uuid.NewString(),
						ProjectId:         resource.ResourceEnvironmentId,
						EnvId:             resource.EnvironmentId,
						ViewFields:        []string{viewField.Id},
					},
				)
				if err != nil {
					continue
				}
			case "delete":
				relations, err := services.GoObjectBuilderService().Relation().GetIds(
					c.Request.Context(),
					&nb.GetIdsReq{
						TableFrom: tableSlug,
						TableTo:   relation.TableTo,
						ProjectId: resource.ResourceEnvironmentId,
					},
				)
				if err != nil {
					continue
				}

				for _, relationId := range relations.GetIds() {
					_, err := services.GoObjectBuilderService().Relation().Delete(
						c.Request.Context(),
						&nb.RelationPrimaryKey{
							Id:        relationId,
							ProjectId: resource.ResourceEnvironmentId,
							EnvId:     resource.EnvironmentId,
						},
					)
					if err != nil {
						continue
					}
				}

			}
		}
	}

	h.HandleResponse(c, status_http.OK, "Table updated successfully")
}

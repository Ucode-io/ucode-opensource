package v2

import (
	"errors"
	"fmt"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
)

// CreateCollection godoc
// @Security ApiKeyAuth
// @ID create_collection
// @Router /v2/collections [POST]
// @Summary Create collection
// @Description Create collection
// @Tags Collections
// @Accept json
// @Produce json
// @Param table body models.CreateTableRequest true "CreateCollectionRequestBody"
// @Success 201 {object} status_http.Response{data=obs.Table} "Collection data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateCollection(c *gin.Context) {
	var (
		tableRequest models.CreateTableRequest
		resp         *obs.CreateTableResponse
		err          error
	)

	err = c.ShouldBindJSON(&tableRequest)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	attributes, err := helper.ConvertMapToStruct(tableRequest.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	authInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("error getting auth info: %w", err).Error())
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

	var fields []*obs.CreateFieldsRequest

	var table = obs.CreateTableRequest{
		Label:             tableRequest.Label,
		Description:       tableRequest.Description,
		Slug:              tableRequest.Slug,
		ShowInMenu:        tableRequest.ShowInMeny,
		Icon:              tableRequest.Icon,
		Fields:            fields,
		SubtitleFieldSlug: tableRequest.SubtitleFieldSlug,
		IncrementId: &obs.IncrementID{
			WithIncrementId: tableRequest.IncrementID.WithIncrementID,
			DigitNumber:     tableRequest.IncrementID.DigitNumber,
			Prefix:          tableRequest.IncrementID.Prefix,
		},
		AuthorId:   authInfo.GetUserId(),
		Name:       fmt.Sprintf("Auto Created Commit Create table - %s", time.Now().Format(time.RFC1123)),
		CommitType: config.COMMIT_TYPE_TABLE,
		OrderBy:    tableRequest.OrderBy,
		Attributes: attributes,
	}

	table.ProjectId = resource.ResourceEnvironmentId

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Table().Create(
			c.Request.Context(),
			&table,
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// GetSingleCollection godoc
// @Security ApiKeyAuth
// @ID get_single_collection
// @Router /v2/collections/{collection} [GET]
// @Summary Get single collection
// @Description Get single collection
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Success 200 {object} status_http.Response{data=models.CreateTableResponse} "CollectionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetSingleCollection(c *gin.Context) {
	tableID := c.Param("collection")
	var (
		resp *obs.Table
		err  error
	)

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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Table().GetByID(
			c.Request.Context(),
			&obs.TablePrimaryKey{
				Id:        tableID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}
	h.HandleResponse(c, status_http.OK, resp)
}

// GetAllCollections godoc
// @Security ApiKeyAuth
// @ID get_all_collections
// @Router /v2/collections [GET]
// @Summary Get all collections
// @Description Get all collections
// @Tags Collections
// @Accept json
// @Produce json
// @Param filters query models.GetAllTablesRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetAllTablesResponse} "CollectionBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllCollections(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	limitStr := c.DefaultQuery("limit", "120")
	limit := cast.ToInt32(limitStr)

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var isLoginTable bool
	var isLoginTableStr = c.Query("is_login_table")
	if isLoginTableStr == "true" {
		isLoginTable = true
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Table().GetAll(
			c.Request.Context(),
			&obs.GetAllTablesRequest{
				Limit:        int32(limit),
				Offset:       int32(offset),
				Search:       c.Query("search"),
				ProjectId:    resource.ResourceEnvironmentId,
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
			c.Request.Context(),
			&nb.GetAllTablesRequest{
				Limit:        int32(limit),
				Offset:       int32(offset),
				Search:       c.Query("search"),
				ProjectId:    resource.ResourceEnvironmentId,
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

// UpdateCollection godoc
// @Security ApiKeyAuth
// @ID update_collection
// @Router /v2/collections [PUT]
// @Summary Update collection
// @Description Update collection
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection body models.UpdateTableRequest  true "UpdateCollectionRequestBody"
// @Success 200 {object} status_http.Response{data=obs.Table} "Collection data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateCollection(c *gin.Context) {
	var (
		table models.UpdateTableRequest
		resp  *obs.Table
	)

	err := c.ShouldBindJSON(&table)
	if err != nil {
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

	structData, err := helper.ConvertMapToStruct(table.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err)
		return
	}

	table.ProjectId = resource.ResourceEnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Table().Update(
			c.Request.Context(),
			&obs.UpdateTableRequest{
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
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// DeleteCollection godoc
// @Security ApiKeyAuth
// @ID delete_collection
// @Router /v2/collections/{collection} [DELETE]
// @Summary Delete Collection
// @Description Delete Collection
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteCollection(c *gin.Context) {
	tableID := c.Param("collection")
	var (
		resp *emptypb.Empty
	)

	if !util.IsValidUUID(tableID) {
		h.HandleResponse(c, status_http.InvalidArgument, "collection id is an invalid uuid")
		return
	}

	authInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Errorf("error getting auth info: %w", err).Error())
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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Table().Delete(
			c.Request.Context(),
			&obs.TablePrimaryKey{
				Id:         tableID,
				ProjectId:  resource.ResourceEnvironmentId,
				AuthorId:   authInfo.GetUserId(),
				Name:       fmt.Sprintf("Auto Created Commit Delete table - %s", time.Now().Format(time.RFC1123)),
				CommitType: config.COMMIT_TYPE_TABLE,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}
	h.HandleResponse(c, status_http.NoContent, resp)
}

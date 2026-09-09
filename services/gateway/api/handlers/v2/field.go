package v2

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/structpb"
)

// CreateField godoc
// @Security ApiKeyAuth
// @ID v2_create_field
// @Router /v2/fields/{collection} [POST]
// @Summary Create field
// @Description Create field
// @Tags V2_Field
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param table body models.CreateFieldRequest true "CreateFieldRequestBody"
// @Success 201 {object} status_http.Response{data=obs.Field} "Field data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateField(c *gin.Context) {
	var (
		fieldRequest models.CreateFieldRequest
		resp         *obs.Field
		fields       []*obs.CreateFieldRequest
	)

	if err := c.ShouldBindJSON(&fieldRequest); err != nil {
		h.handleError(c, status_http.BadRequest, err)
		return
	}

	attributes, err := helper.ConvertMapToStruct(fieldRequest.Attributes)
	if err != nil {
		h.handleError(c, status_http.InvalidArgument, err)
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleError(c, status_http.InvalidArgument, errors.New("project id is not valid"))
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleError(c, status_http.BadRequest, errors.New("environment id is not valid"))
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
		h.handleError(c, status_http.GRPCError, err)
		return
	}

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.handleError(c, status_http.InternalServerError, err)
		return
	}

	if fieldRequest.EnableMultilanguage {
		if fieldRequest.Type == "SINGLE_LINE" || fieldRequest.Type == "MULTI_LINE" {
			languages, err := h.companyServices.Project().GetById(
				c.Request.Context(), &pb.GetProjectByIdRequest{
					ProjectId: resource.GetProjectId(),
				},
			)
			if err != nil {
				h.handleError(c, status_http.InternalServerError, err)
				return
			}
			if len(languages.GetLanguage()) > 1 {
				for _, value := range languages.GetLanguage() {
					fieldRequest.ID = uuid.New().String()
					fields = append(fields, SetTitlePrefix(fieldRequest, value.ShortName, resource.ResourceEnvironmentId, attributes, true, false))
				}
			} else {
				fields = append(fields, SetTitlePrefix(fieldRequest, "", resource.ResourceEnvironmentId, attributes, true, false))
			}
		} else {
			fields = append(fields, SetTitlePrefix(fieldRequest, "", resource.ResourceEnvironmentId, attributes, false, false))
		}
	} else {
		fields = append(fields, SetTitlePrefix(fieldRequest, "", resource.ResourceEnvironmentId, attributes, false, false))
	}

	logReq := &models.CreateVersionHistoryRequest{
		Services:     services,
		NodeType:     resource.NodeType,
		ProjectId:    resource.ResourceEnvironmentId,
		ActionSource: "FIELD",
		ActionType:   "CREATE FIELD",
		UserInfo:     cast.ToString(userId),
		TableSlug:    c.Param("collection"),
	}

	if fieldRequest.IsAlt && len(fields) == 1 {
		atr, err := helper.ConvertMapToStruct(map[string]any{
			"label":    fields[0].Label + " Alt",
			"label_en": fields[0].Label + " Alt",
		})
		if err != nil {
			h.handleError(c, status_http.InternalServerError, err)
			return
		}

		fields = append(fields, &obs.CreateFieldRequest{
			Default:             "",
			Type:                "SINGLE_LINE",
			Index:               "string",
			Label:               fields[0].Label + " Alt",
			Slug:                fields[0].Slug + "_alt",
			TableId:             fields[0].TableId,
			Attributes:          atr,
			IsVisible:           true,
			AutofillTable:       "",
			AutofillField:       "",
			Unique:              false,
			Automatic:           false,
			ProjectId:           resource.ResourceEnvironmentId,
			ShowLabel:           true,
			EnableMultilanguage: false,
		})
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		for _, field := range fields {
			resp, err = services.GetBuilderServiceByType(resource.NodeType).Field().Create(
				c.Request.Context(), field,
			)
			if err != nil {
				logReq.Request = field
				logReq.Response = err.Error()
				go h.versionHistory(logReq)
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			logReq.Request = field
			logReq.Response = resp
			logReq.Current = resp
			go h.versionHistory(logReq)
		}

		h.HandleResponse(c, status_http.Created, resp)
	case pb.ResourceType_POSTGRESQL:
		var resp = &nb.Field{}

		for _, field := range fields {
			var newReq = nb.CreateFieldRequest{}

			if err = helper.MarshalToStruct(&field, &newReq); err != nil {
				h.handleError(c, status_http.InternalServerError, err)
				return
			}

			resp, err = services.GoObjectBuilderService().Field().Create(
				c.Request.Context(), &newReq,
			)
			if err != nil {
				logReq.Request = field
				logReq.Response = err.Error()
				go h.versionHistory(logReq)
				h.handleError(c, status_http.InternalServerError, err)
				return
			}
			logReq.Request = field
			logReq.Response = resp
			logReq.Current = resp
			go h.versionHistoryGo(c, logReq)
		}

		h.HandleResponse(c, status_http.Created, resp)
	}
}

// GetAllFields godoc
// @Security ApiKeyAuth
// @ID v2_get_all_fields
// @Router /v2/fields/{collection} [GET]
// @Summary Get all fields
// @Description Get all fields
// @Tags V2_Field
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param filters query obs.GetAllFieldsRequest true "filters"
// @Success 200 {object} status_http.Response{data=models.GetAllFieldsResponse} "FieldBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllFields(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	var withManyRelation, withOneRelation = false, false
	if c.Query("with_many_relation") == "true" {
		withManyRelation = true
	}

	if c.Query("with_one_relation") == "true" {
		withOneRelation = true
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	limit := 100
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Field().GetAll(
			c.Request.Context(), &obs.GetAllFieldsRequest{
				Limit:            int32(limit),
				Offset:           int32(offset),
				Search:           c.Query("search"),
				TableId:          c.Query("table_id"),
				TableSlug:        c.Param("collection"),
				WithManyRelation: withManyRelation,
				WithOneRelation:  withOneRelation,
				ProjectId:        resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Field().GetAll(
			c.Request.Context(), &nb.GetAllFieldsRequest{
				Limit:            int32(limit),
				Offset:           int32(offset),
				Search:           c.Query("search"),
				TableId:          c.Query("table_id"),
				TableSlug:        c.Param("collection"),
				WithManyRelation: withManyRelation,
				WithOneRelation:  withOneRelation,
				ProjectId:        resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

// UpdateField godoc
// @Security ApiKeyAuth
// @ID v2_update_field
// @Router /v2/fields/{collection} [PUT]
// @Summary Update field
// @Description Update field
// @Tags V2_Field
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param relation body models.Field  true "UpdateFieldRequestBody"
// @Success 200 {object} status_http.Response{data=models.Field} "Field data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateField(c *gin.Context) {
	var fieldRequest models.Field

	if err := c.ShouldBindJSON(&fieldRequest); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	attributes, err := helper.ConvertMapToStruct(fieldRequest.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	var field = obs.Field{
		Id:                  fieldRequest.ID,
		Default:             fieldRequest.Default,
		Type:                fieldRequest.Type,
		Index:               fieldRequest.Index,
		Label:               fieldRequest.Label,
		Slug:                fieldRequest.Slug,
		TableId:             fieldRequest.TableID,
		Required:            fieldRequest.Required,
		Attributes:          attributes,
		IsVisible:           fieldRequest.IsVisible,
		AutofillField:       fieldRequest.AutoFillField,
		AutofillTable:       fieldRequest.AutoFillTable,
		RelationId:          fieldRequest.RelationId,
		Automatic:           fieldRequest.Automatic,
		Unique:              fieldRequest.Unique,
		RelationField:       fieldRequest.RelationField,
		ShowLabel:           fieldRequest.ShowLabel,
		EnableMultilanguage: fieldRequest.EnableMultilanguage,
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "FIELD",
			ActionType:   "UPDATE FIELD",
			UserInfo:     cast.ToString(userId),
			TableSlug:    c.Param("collection"),
		}
	)

	field.ProjectId = resource.ResourceEnvironmentId
	field.EnvId = resource.EnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		oldField, deferErr := services.GetBuilderServiceByType(resource.NodeType).Field().GetByID(
			c.Request.Context(), &obs.FieldPrimaryKey{
				Id:        field.Id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if deferErr != nil {
			h.HandleResponse(c, status_http.GRPCError, deferErr.Error())
			return
		}

		resp, deferErr := services.GetBuilderServiceByType(resource.NodeType).Field().Update(
			c.Request.Context(), &field,
		)
		logReq.Request = &field
		logReq.Previous = oldField
		if deferErr != nil {
			logReq.Response = deferErr.Error()
			h.HandleResponse(c, status_http.GRPCError, deferErr.Error())
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)

	case pb.ResourceType_POSTGRESQL:
		newReq := nb.Field{}

		err = helper.MarshalToStruct(&field, &newReq)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		oldField, deferErr := services.GoObjectBuilderService().Field().GetByID(
			c.Request.Context(), &nb.FieldPrimaryKey{
				Id:        field.Id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if deferErr != nil {
			h.HandleResponse(c, status_http.GRPCError, deferErr.Error())
			return
		}

		resp, deferErr := services.GoObjectBuilderService().Field().Update(
			c.Request.Context(), &newReq,
		)
		logReq.Request = &field
		logReq.Previous = oldField
		if deferErr != nil {
			logReq.Response = deferErr.Error()
			h.handleDynamicError(c, status_http.GRPCError, deferErr)
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistoryGo(c, logReq)

	}
}

func (h *HandlerV2) UpdateSearch(c *gin.Context) {
	var searchRequest obs.SearchUpdateRequest

	if err := c.ShouldBindJSON(&searchRequest); err != nil {
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	searchRequest.ProjectId = resource.ResourceEnvironmentId
	searchRequest.TableSlug = c.Param("collection")
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Field().UpdateSearch(
			c.Request.Context(), &searchRequest,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.Created, resp)
	case pb.ResourceType_POSTGRESQL:
		newReq := nb.SearchUpdateRequest{}

		if err = helper.MarshalToStruct(&searchRequest, &newReq); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		resp, err := services.GoObjectBuilderService().Field().UpdateSearch(
			c.Request.Context(), &newReq,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.Created, resp)
	}
}

// DeleteField godoc
// @Security ApiKeyAuth
// @ID v2_delete_field
// @Router /v2/fields/{collection}/{id} [DELETE]
// @Summary Delete Field
// @Description Delete Field
// @Tags V2_Field
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Param project-id query string true "project-id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteField(c *gin.Context) {
	var (
		fieldID   = c.Param("id")
		tableSlug = c.Param("collection")
	)

	if !util.IsValidUUID(fieldID) {
		h.HandleResponse(c, status_http.InvalidArgument, "field id is an invalid uuid")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "FIELD",
			ActionType:   "DELETE FIELD",
			UserInfo:     cast.ToString(userId),
			TableSlug:    tableSlug,
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		oldField, err := services.GetBuilderServiceByType(resource.NodeType).Field().GetByID(
			c.Request.Context(), &obs.FieldPrimaryKey{
				Id:        fieldID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if config.RelationFieldTypes[oldField.GetType()] {
			h.HandleResponse(c, status_http.GRPCError, "relation field cannot be deleted")
			return
		}

		resp, err := services.GetBuilderServiceByType(resource.NodeType).Field().Delete(
			c.Request.Context(), &obs.FieldPrimaryKey{
				Id:        fieldID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			logReq.Previous = oldField
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Previous = oldField
			logReq.Response = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)

		h.HandleResponse(c, status_http.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		oldField, err := services.GoObjectBuilderService().Field().GetByID(
			c.Request.Context(), &nb.FieldPrimaryKey{
				Id:        fieldID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if config.RelationFieldTypes[oldField.GetType()] {
			h.HandleResponse(c, status_http.GRPCError, "relation field cannot be deleted")
			return
		}

		resp, err := services.GoObjectBuilderService().Field().Delete(
			c.Request.Context(), &nb.FieldPrimaryKey{
				Id:        fieldID,
				ProjectId: resource.ResourceEnvironmentId,
				TableSlug: tableSlug,
			},
		)
		if err != nil {
			logReq.Previous = oldField
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Previous = oldField
			logReq.Response = resp
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistoryGo(c, logReq)

		h.HandleResponse(c, status_http.NoContent, resp)
	}
}

// GetAllFieldsWithDetails godoc
// @Security ApiKeyAuth
// @ID v2_get_all_fields_with_details
// @Router /v2/fields/{collection}/details [GET]
// @Summary Get all fields with details
// @Description Get all fields with details
// @Tags V2_Field
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param with_relations query string false "with_relations"
// @Param language_setting query string false "language_setting"
// @Success 200 {object} status_http.Response{data=models.CommonMessage} "FieldBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllFieldsWithDetails(c *gin.Context) {
	var (
		resp          *obs.AllFields
		roleId        string
		withRelations = false
	)

	if c.Query("with_relations") == "true" {
		withRelations = true
	}

	autoInfo, _ := h.GetAuthInfo(c)
	if autoInfo != nil {
		if autoInfo.GetRoleId() != "" {
			roleId = autoInfo.GetRoleId()
		}
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Field().GetAllForItems(
			c.Request.Context(), &obs.GetAllFieldsForItemsRequest{
				Collection:       c.Param("collection"),
				ProjectId:        resource.ResourceEnvironmentId,
				LanguageSettings: c.Query("language_setting"),
				WithRelations:    withRelations,
				RoleId:           roleId,
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

func (h *HandlerV2) FieldsWithPermissions(c *gin.Context) {
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp, err := services.GoObjectBuilderService().Field().FieldsWithRelations(
		c.Request.Context(), &nb.FieldsWithRelationRequest{
			ProjectId: resource.ResourceEnvironmentId,
			TableSlug: c.Param("collection"),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func SetTitlePrefix(fieldRequest models.CreateFieldRequest, prefix, project_id string, attributes *structpb.Struct, enable, hide bool) *obs.CreateFieldRequest {
	if prefix != "" {
		return &obs.CreateFieldRequest{
			Id:                  fieldRequest.ID,
			Default:             fieldRequest.Default,
			Type:                fieldRequest.Type,
			Index:               fieldRequest.Index,
			Label:               fieldRequest.Label,
			Slug:                fieldRequest.Slug + "_" + prefix,
			TableId:             fieldRequest.TableID,
			Attributes:          attributes,
			IsVisible:           fieldRequest.IsVisible,
			AutofillTable:       fieldRequest.AutoFillTable,
			AutofillField:       fieldRequest.AutoFillField,
			RelationField:       fieldRequest.RelationField,
			Automatic:           fieldRequest.Automatic,
			ShowLabel:           fieldRequest.ShowLabel,
			Unique:              fieldRequest.Unique,
			ProjectId:           project_id,
			EnableMultilanguage: enable,
			HideMultilanguage:   hide,
		}
	} else {
		return &obs.CreateFieldRequest{
			Id:                  fieldRequest.ID,
			Default:             fieldRequest.Default,
			Type:                fieldRequest.Type,
			Index:               fieldRequest.Index,
			Label:               fieldRequest.Label,
			Slug:                fieldRequest.Slug,
			TableId:             fieldRequest.TableID,
			Attributes:          attributes,
			IsVisible:           fieldRequest.IsVisible,
			AutofillTable:       fieldRequest.AutoFillTable,
			AutofillField:       fieldRequest.AutoFillField,
			RelationField:       fieldRequest.RelationField,
			Automatic:           fieldRequest.Automatic,
			ShowLabel:           fieldRequest.ShowLabel,
			Unique:              fieldRequest.Unique,
			ProjectId:           project_id,
			EnableMultilanguage: enable,
			HideMultilanguage:   hide,
		}
	}
}

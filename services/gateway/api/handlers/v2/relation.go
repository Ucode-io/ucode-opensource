package v2

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetRelationCascading godoc
// @Security ApiKeyAuth
// @ID get_relation_cascading
// @Router /v2/relations/{collection}/cascading [GET]
// @Security ApiKeyAuth
// @Summary Get relation cascading
// @Description Get relation cascading
// @Tags V2_Relation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Success 200 {object} status_http.Response{data=string} "CascadingBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetRelationCascading(c *gin.Context) {
	var resp *obs.CommonMessage

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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Cascading().GetCascadings(
			c.Request.Context(), &obs.GetCascadingRequest{
				TableSlug: c.Param("collection"),
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

// GetByIdRelation godoc
// @ID v2_get_by_id_relation
// @Router /v2/relations/{collection}/{id} [GET]
// @Security ApiKeyAuth
// @Summary Get relation by id
// @Description Get relation by id
// @Tags V2_Relation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// Success 200 {object} status_http.Response{data=obs.RelationForGetAll} "RelationBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetByIdRelation(c *gin.Context) {
	var (
		relationId string = c.Param("id")
		resp       *obs.RelationForGetAll
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Relation().GetByID(
			c.Request.Context(), &obs.RelationPrimaryKey{
				Id:        relationId,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Relation().GetByID(
			c.Request.Context(), &nb.RelationPrimaryKey{
				Id:        relationId,
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

// CreateRelation godoc
// @ID create_relations_V2
// @Router /v2/relations/{collection} [POST]
// @Security ApiKeyAuth
// @Summary Create relation
// @Description Create relation
// @Tags Relation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param project-id query string true "project-id"
// @Param table body obs.CreateRelationRequest true "CreateRelationRequestBody"
// @Success 201 {object} status_http.Response{data=string} "Relation data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateRelation(c *gin.Context) {
	var (
		relation   obs.CreateRelationRequest
		err        error
		goRelation nb.CreateRelationRequest
	)

	if err = c.ShouldBindJSON(&relation); err != nil {
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	relation.RelationFieldId = uuid.NewString()
	relation.RelationToFieldId = uuid.NewString()
	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "RELATION",
			ActionType:   "CREATE RELATION",
			UserInfo:     cast.ToString(userId),
			Request:      &relation,
			TableSlug:    c.Param("collection"),
		}
	)

	relation.ProjectId = resource.ResourceEnvironmentId
	relation.EnvId = resource.EnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Relation().Create(
			c.Request.Context(), &relation,
		)
		relation.Id = resp.Id
		logReq.Request = &relation
		if err != nil {
			logReq.Response = err.Error()
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		if err = helper.MarshalToStruct(&relation, &goRelation); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		resp, err := services.GoObjectBuilderService().Relation().Create(
			c.Request.Context(), &goRelation,
		)
		relation.Id = resp.GetId()
		logReq.Request = &goRelation
		if err != nil {
			logReq.Response = err.Error()
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// GetAllRelations godoc
// @Security ApiKeyAuth
// @ID get_all_relations
// @Router /v2/relations/{collection} [GET]
// @Security ApiKeyAuth
// @Summary Get all relations
// @Description Get all relations
// @Tags Relation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param project-id query string true "project-id"
// @Param filters query obs.GetAllRelationsRequest true "filters"
// @Success 200 {object} status_http.Response{data=string} "RelationBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllRelations(c *gin.Context) {
	var (
		resp      *obs.GetAllRelationsResponse
		tableSlug = c.Param("collection")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Relation().GetAll(
			c.Request.Context(), &obs.GetAllRelationsRequest{
				Limit:          int32(limit),
				Offset:         int32(offset),
				TableSlug:      tableSlug,
				TableId:        c.DefaultQuery("table_id", ""),
				ProjectId:      resource.ResourceEnvironmentId,
				DisableTableTo: c.DefaultQuery("disable_table_to", "false") == "true",
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Relation().GetAll(
			c.Request.Context(), &nb.GetAllRelationsRequest{
				Limit:          int32(limit),
				Offset:         int32(offset),
				TableSlug:      tableSlug,
				TableId:        c.DefaultQuery("table_id", ""),
				ProjectId:      resource.ResourceEnvironmentId,
				DisableTableTo: c.DefaultQuery("disable_table_to", "false") == "true",
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

// UpdateRelation godoc
// @Security ApiKeyAuth
// @ID update_relations_v2
// @Router /v2/relations/:collection [PUT]
// @Security ApiKeyAuth
// @Summary Update relation
// @Description Update relation
// @Tags Relation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param relation body obs.UpdateRelationRequest  true "UpdateRelationRequestBody"
// @Success 200 {object} status_http.Response{data=string} "Relation data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateRelation(c *gin.Context) {
	var (
		relation   obs.UpdateRelationRequest
		resp       *obs.RelationForGetAll
		goRelation nb.UpdateRelationRequest
	)

	if err := c.ShouldBindJSON(&relation); err != nil {
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		oldRelation = &obs.RelationForGetAll{}
		logReq      = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "RELATION",
			ActionType:   "UPDATE RELATION",
			UserInfo:     cast.ToString(userId),
			Request:      &relation,
			TableSlug:    c.Param("collection"),
		}
	)

	relation.ProjectId = resource.ResourceEnvironmentId
	relation.EnvId = resource.EnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		oldRelation, err = services.GetBuilderServiceByType(resource.NodeType).Relation().GetByID(
			c.Request.Context(), &obs.RelationPrimaryKey{
				Id:        relation.Id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err = services.GetBuilderServiceByType(resource.NodeType).Relation().Update(
			c.Request.Context(),
			&relation,
		)
		logReq.Previous = oldRelation
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
		goOldRelation, err := services.GoObjectBuilderService().Relation().GetByID(
			c.Request.Context(), &nb.RelationPrimaryKey{
				Id:        relation.Id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if err = helper.MarshalToStruct(&relation, &goRelation); err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		goRelation.ProjectId = resource.ResourceEnvironmentId

		goResp, err := services.GoObjectBuilderService().Relation().Update(
			c.Request.Context(),
			&goRelation,
		)
		logReq.Previous = goOldRelation
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = goResp
			logReq.Current = goResp
			h.HandleResponse(c, status_http.OK, goResp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// DeleteRelation godoc
// @Security ApiKeyAuth
// @ID delete_relations_v2
// @Router /v2/relations/{collection}/{relation_id} [DELETE]
// @Security ApiKeyAuth
// @Summary Delete Relation
// @Description Delete Relation
// @Tags Relation
// @Accept json
// @Produce json
// @Param relation_id path string true "relation_id"
// @Param collection path string true "collection"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteRelation(c *gin.Context) {
	var (
		resp       *emptypb.Empty
		relationID = c.Param("id")
	)

	if !util.IsValidUUID(relationID) {
		h.HandleResponse(c, status_http.InvalidArgument, "relation id is an invalid uuid")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "RELATION",
			ActionType:   "DELETE RELATION",
			UserInfo:     cast.ToString(userId),
			TableSlug:    c.Param("collection"),
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Relation().Delete(
			c.Request.Context(), &obs.RelationPrimaryKey{
				Id:        relationID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		resp, err = services.GoObjectBuilderService().Relation().Delete(
			c.Request.Context(), &nb.RelationPrimaryKey{
				Id:        relationID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// GetRelationCascaders godoc
// @Security ApiKeyAuth
// @ID get_relation_cascaders
// @Router /v2/get-relation-cascading/{collection} [GET]
// @Security ApiKeyAuth
// @Summary Get all relations
// @Description Get all relations
// @Tags Relation
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Success 200 {object} status_http.Response{data=string} "CascaderBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetRelationCascaders(c *gin.Context) {
	var resp *obs.CommonMessage

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

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Cascading().GetCascadings(
			c.Request.Context(), &obs.GetCascadingRequest{
				TableSlug: c.Param("collection"),
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

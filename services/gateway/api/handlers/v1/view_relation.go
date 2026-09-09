package v1

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetViewRelation godoc
// @Security ApiKeyAuth
// @ID get_single_view_relation
// @Router /v1/view_relation [GET]
// @Summary Get single view relation
// @Description Get single view relation
// @Description get list view relation switch to get single view relation because for one table be one view relation
// @Tags ViewRelation
// @Accept json
// @Produce json
// @Param filters query obs.GetAllSectionsRequest true "filters"
// @Success 200 {object} status_http.Response{data=string} "ViewRelationBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetViewRelation(c *gin.Context) {

	// get list view relation switch to get single view relation because for one table be one view relation
	tokenInfo, err := h.GetAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Forbidden, err.Error())
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Section().GetViewRelation(
			c.Request.Context(),
			&obs.GetAllSectionsRequest{
				TableId:   c.Query("table_id"),
				TableSlug: c.Query("table_slug"),
				RoleId:    tokenInfo.GetRoleId(),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Section().GetViewRelation(
			c.Request.Context(),
			&nb.GetAllSectionsRequest{
				TableId:   c.Query("table_id"),
				TableSlug: c.Query("table_slug"),
				RoleId:    tokenInfo.GetRoleId(),
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

// UpsertViewRelations godoc
// @Security ApiKeyAuth
// @ID upsert_view_relation
// @Router /v1/view_relation [PUT]
// @Summary Upsert view relation
// @Description Upsert view relation
// @Tags ViewRelation
// @Accept json
// @Produce json
// @Param table body obs.UpsertViewRelationsBody  true "UpsertViewRelationsBody"
// @Success 200 {object} status_http.Response{data=string} "View Relation data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpsertViewRelations(c *gin.Context) {
	var viewRelation obs.UpsertViewRelationsBody

	err := c.ShouldBindJSON(&viewRelation)
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

	viewRelation.ProjectId = resource.ResourceEnvironmentId

	var resp *emptypb.Empty
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		// resp, err = services.GetBuilderServiceByType(resource.NodeType).Section().UpsertViewRelations(
		// 	c.Request.Context(),
		// 	&viewRelation,
		// )

		// if err != nil {
		// 	h.HandleResponse(c, status_http.GRPCError, err.Error())
		// 	return
		// }
	case pb.ResourceType_POSTGRESQL:
		// resp, err = services.PostgresBuilderService().Section().UpsertViewRelations(
		// 	c.Request.Context(),
		// 	&viewRelation,
		// )

		// if err != nil {
		// 	h.HandleResponse(c, status_http.GRPCError, err.Error())
		// 	return
		// }
	}

	h.HandleResponse(c, status_http.OK, resp)
}

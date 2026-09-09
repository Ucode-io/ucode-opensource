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
	"github.com/spf13/cast"
)

// CreateView godoc
// @Security ApiKeyAuth
// @ID v2_create_view
// @Router /v2/views/{collection} [POST]
// @Summary Create view
// @Description Create view
// @Tags V2_View
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param view body obs.CreateViewRequest true "CreateViewRequestBody"
// @Success 201 {object} status_http.Response{data=obs.View} "View data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateView(c *gin.Context) {
	var view obs.CreateViewRequest

	if err := c.ShouldBindJSON(&view); err != nil {
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
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "VIEW",
			ActionType:   "CREATE VIEW",
			UserInfo:     cast.ToString(userId),
			Request:      &view,
			TableSlug:    c.Param("collection"),
		}
	)

	view.ProjectId = resource.ResourceEnvironmentId
	view.EnvId = resource.EnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).View().Create(
			c.Request.Context(), &view,
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Current = resp
			logReq.Response = resp
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistory(logReq)
	case pb.ResourceType_POSTGRESQL:
		newReq := nb.CreateViewRequest{}
		err = helper.MarshalToStruct(&view, &newReq)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err := services.GoObjectBuilderService().View().Create(
			c.Request.Context(), &newReq,
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Current = resp
			logReq.Response = resp
			h.HandleResponse(c, status_http.Created, resp)
		}
		go h.versionHistoryGo(c, logReq)
	}
}

// GetSingleView godoc
// @Security ApiKeyAuth
// @ID v2_get_view_by_id
// @Router /v2/views/{collection}/{id} [GET]
// @Summary Get single view
// @Description Get single view
// @Tags V2_View
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=obs.View} "ViewBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetSingleView(c *gin.Context) {
	viewID := c.Param("id")

	if !util.IsValidUUID(viewID) {
		h.HandleResponse(c, status_http.InvalidArgument, "view id is an invalid uuid")
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
		resp, err := services.GetBuilderServiceByType(resource.NodeType).View().GetSingle(
			c.Request.Context(), &obs.ViewPrimaryKey{
				Id:        viewID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().View().GetSingle(
			c.Request.Context(), &nb.ViewPrimaryKey{
				Id:        viewID,
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

// UpdateView godoc
// @Security ApiKeyAuth
// @ID v2_update_view
// @Router /v2/views/{collection} [PUT]
// @Summary Update view
// @Description Update view
// @Tags V2_View
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param view body obs.View true "UpdateViewRequestBody"
// @Success 200 {object} status_http.Response{data=obs.View} "View data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateView(c *gin.Context) {
	var view obs.View

	if err := c.ShouldBindJSON(&view); err != nil {
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
		oldView *obs.View
		logReq  = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "VIEW",
			ActionType:   "UPDATE VIEW",
			UserInfo:     cast.ToString(userId),
			Request:      &view,
			TableSlug:    c.Param("collection"),
		}
	)

	view.ProjectId = resource.ResourceEnvironmentId
	view.EnvId = resource.EnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		oldView, err = services.GetBuilderServiceByType(resource.NodeType).View().GetSingle(
			c.Request.Context(), &obs.ViewPrimaryKey{
				Id:        view.Id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		logReq.Previous = oldView

		resp, err := services.GetBuilderServiceByType(resource.NodeType).View().Update(
			c.Request.Context(),
			&view,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		oldView, err := services.GoObjectBuilderService().View().GetSingle(
			c.Request.Context(), &nb.ViewPrimaryKey{
				Id:        view.Id,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		logReq.Previous = oldView

		newReq := nb.View{}
		err = helper.MarshalToStruct(&view, &newReq)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err := services.GoObjectBuilderService().View().Update(
			c.Request.Context(), &newReq,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		} else {
			logReq.Response = resp
			logReq.Current = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistoryGo(c, logReq)
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// DeleteView godoc
// @Security ApiKeyAuth
// @ID v2_delete_view
// @Router /v2/views/{collection}/{id} [DELETE]
// @Summary Delete view
// @Description Delete view
// @Tags V2_View
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteView(c *gin.Context) {
	viewID := c.Param("id")

	if !util.IsValidUUID(viewID) {
		h.HandleResponse(c, status_http.InvalidArgument, "view id is an invalid uuid")
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

	var (
		oldView = &obs.View{}
		logReq  = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "VIEW",
			ActionType:   "DELETE VIEW",
			UserInfo:     cast.ToString(userId),
			TableSlug:    c.Param("collection"),
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		oldView, err = services.GetBuilderServiceByType(resource.NodeType).View().GetSingle(
			c.Request.Context(), &obs.ViewPrimaryKey{
				Id:        viewID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		logReq.Previous = oldView
		resp, err := services.GetBuilderServiceByType(resource.NodeType).View().Delete(
			c.Request.Context(), &obs.ViewPrimaryKey{
				Id:        viewID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.NoContent, resp)
			return
		}
		go h.versionHistory(logReq)
		h.HandleResponse(c, status_http.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		oldView, err := services.GoObjectBuilderService().View().GetSingle(
			c.Request.Context(), &nb.ViewPrimaryKey{
				Id:        viewID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		logReq.Previous = oldView

		resp, err := services.GoObjectBuilderService().View().Delete(
			c.Request.Context(), &nb.ViewPrimaryKey{
				Id:        viewID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		go h.versionHistoryGo(c, logReq)
		h.HandleResponse(c, status_http.NoContent, resp)
	}
}

// GetAllViews godoc
// @Security ApiKeyAuth
// @ID v2_get_view_list
// @Router /v2/views/{collection} [GET]
// @Summary Get view list
// @Description Get view list
// @Tags V2_View
// @Accept json
// @Produce json
// @Param filters query obs.GetAllViewsRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetAllViewsResponse} "ViewBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllViews(c *gin.Context) {
	var roleId string

	if c.Param("collection") == "" {
		h.HandleResponse(c, status_http.BadRequest, "collection is required")
		return
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

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).View().GetList(
			c.Request.Context(), &obs.GetAllViewsRequest{
				TableSlug: c.Param("collection"),
				ProjectId: resource.ResourceEnvironmentId,
				RoleId:    roleId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().View().GetList(
			c.Request.Context(), &nb.GetAllViewsRequest{
				TableSlug: c.Param("collection"),
				ProjectId: resource.ResourceEnvironmentId,
				RoleId:    roleId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// UpdateViewOrder godoc
// @Security ApiKeyAuth
// @ID v2_update_view_order
// @Router /v2/views/{collection}/update-order [PUT]
// @Summary Update view order
// @Description Update view order
// @Tags V2_View
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param view body obs.UpdateViewOrderRequest true "UpdateViewOrderRequestBody"
// @Success 200 {object} status_http.Response{data=string} "View data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateViewOrder(c *gin.Context) {
	var view obs.UpdateViewOrderRequest

	if err := c.ShouldBindJSON(&view); err != nil {
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
	view.ProjectId = resource.ResourceEnvironmentId

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	view.ProjectId = resource.ResourceEnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).View().UpdateViewOrder(
			c.Request.Context(), &view,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		newReq := nb.UpdateViewOrderRequest{}
		err = helper.MarshalToStruct(&view, &newReq)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err := services.GoObjectBuilderService().View().UpdateViewOrder(
			c.Request.Context(), &newReq,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

package v3

import (
	"errors"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func (h *HandlerV3) GetSingleLayout(c *gin.Context) {
	var (
		collection = c.Param("collection")
		menuId     = c.Param("menu_id")
	)

	if collection == "" && menuId == "" {
		h.HandleResponse(c, status_http.BadRequest, "table-slug or table-id is required")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
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

	authInfo, _ := h.GetAuthInfo(c)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Layout().GetSingleLayout(
			c.Request.Context(), &obs.GetSingleLayoutRequest{
				TableSlug: collection,
				ProjectId: resource.ResourceEnvironmentId,
				MenuId:    menuId,
				RoleId:    authInfo.GetRoleId(),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Layout().GetSingleLayout(
			c.Request.Context(), &nb.GetSingleLayoutRequest{
				TableSlug: collection,
				ProjectId: resource.ResourceEnvironmentId,
				MenuId:    menuId,
				RoleId:    authInfo.GetRoleId(),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	}
}

// GetListLayouts godoc
// @Security ApiKeyAuth
// @ID get_list_layouts_v3
// @Router /v3/menus/{menu_id}/views/{view_id}/tables/{collection}/layout [GET]
// @Summary Get list layouts
// @Description Get list layouts
// @Tags Layout
// @Accept json
// @Produce json
// @Param table-id query string false "table-id"
// @Param table-slug query string false "table-slug"
// @Param language_setting query string false "language_setting"
// @Success 200 {object} status_http.Response{data=obs.GetListLayoutResponse} "TableBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV3) GetListLayouts(c *gin.Context) {
	var (
		tableSlug = c.Query("collection")
	)

	if tableSlug == "" {
		h.HandleResponse(c, status_http.BadRequest, "table-slug or table-id is required")
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

	var (
		resourceEnvironmentId string
		nodeType              string
	)

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
	nodeType = resource.NodeType

	authInfo, _ := h.GetAuthInfo(c)

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), nodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).Layout().GetAll(
			c.Request.Context(), &obs.GetListLayoutRequest{
				TableSlug:       tableSlug,
				ProjectId:       resourceEnvironmentId,
				IsDefualt:       cast.ToBool(c.Query("is_defualt")),
				RoleId:          authInfo.GetRoleId(),
				LanguageSetting: c.Query("language_setting"),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)

	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Layout().GetAll(
			c.Request.Context(), &nb.GetListLayoutRequest{
				TableSlug:       tableSlug,
				ProjectId:       resourceEnvironmentId,
				IsDefault:       cast.ToBool(c.Query("is_defualt")),
				RoleId:          authInfo.GetRoleId(),
				LanguageSetting: c.Query("language_setting"),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)

	}
}

func (h *HandlerV3) UpdateLayout(c *gin.Context) {
	var (
		input obs.LayoutRequest
		resp  *obs.LayoutResponse
	)

	if err := c.ShouldBindJSON(&input); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if input.TableId == "" {
		h.HandleResponse(c, status_http.BadRequest, errors.New("table id is required"))
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
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

	input.ProjectId = resource.ResourceEnvironmentId

	if input.Id == "" {
		input.Id = uuid.NewString()
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "LAYOUT",
			ActionType:   "UPDATE LAYOUT",
			UserInfo:     cast.ToString(userId),
			Request:      &input,
			TableSlug:    c.Param("collection"),
		}
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Layout().Update(c.Request.Context(), &input)

		logReq.Previous = &input
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)
		h.HandleResponse(c, status_http.OK, resp)
		return
	case pb.ResourceType_POSTGRESQL:
		var newInput = nb.LayoutRequest{}

		if err = helper.MarshalToStruct(&input, &newInput); err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp2, err := services.GoObjectBuilderService().Layout().Update(c.Request.Context(), &newInput)
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, resp2)
			return
		}

		if err = helper.MarshalToStruct(resp2, &resp); err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		logReq.Response = resp
		go h.versionHistoryGo(c, logReq)

		h.HandleResponse(c, status_http.OK, resp)
		return
	}
}

func (h *HandlerV3) DeleteLayout(c *gin.Context) {
	var resp = &empty.Empty{}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "error getting environment id | not valid")
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
		oldLayout = &obs.LayoutResponse{}
		logReq    = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "LAYOUT",
			ActionType:   "DELETE LAYOUT",
			UserInfo:     cast.ToString(userId),
			TableSlug:    c.Param("collection"),
		}
	)

	defer func() {
		logReq.Previous = oldLayout
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			h.HandleResponse(c, status_http.OK, resp)
		}
		switch resource.ResourceType {
		case pb.ResourceType_MONGODB:
			go h.versionHistory(logReq)
		case pb.ResourceType_POSTGRESQL:
			go h.versionHistoryGo(c, logReq)
		}
	}()

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		oldLayout, err = services.GetBuilderServiceByType(resource.NodeType).Layout().GetByID(
			c.Request.Context(), &obs.LayoutPrimaryKey{
				Id:        c.Param("id"),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		resp, err = services.GetBuilderServiceByType(resource.NodeType).Layout().RemoveLayout(
			c.Request.Context(), &obs.LayoutPrimaryKey{
				Id:        c.Param("id"),
				ProjectId: resource.ResourceEnvironmentId,
				EnvId:     environmentId.(string),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		resp, err = services.GoObjectBuilderService().Layout().RemoveLayout(
			c.Request.Context(), &nb.LayoutPrimaryKey{
				Id:        c.Param("id"),
				ProjectId: resource.ResourceEnvironmentId,
				EnvId:     environmentId.(string),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}
}

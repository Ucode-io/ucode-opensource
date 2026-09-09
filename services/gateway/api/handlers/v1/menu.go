package v1

import (
	"context"
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"reflect"

	"github.com/gin-gonic/gin"
	"google.golang.org/protobuf/types/known/emptypb"
)

func (h *HandlerV1) CreateMenu(c *gin.Context) {
	var (
		menu models.CreateMenuRequest
		resp *obs.Menu
	)

	err := c.ShouldBindJSON(&menu)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if menu.Attributes == nil {
		menu.Attributes = make(map[string]any)
	}

	attributes, err := helper.ConvertMapToStruct(menu.Attributes)
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
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().Create(
			context.Background(),
			&obs.CreateMenuRequest{
				Label:           menu.Label,
				Icon:            menu.Icon,
				TableId:         menu.TableId,
				LayoutId:        menu.LayoutId,
				ParentId:        menu.ParentId,
				Type:            menu.Type,
				ProjectId:       resource.ResourceEnvironmentId,
				MicrofrontendId: menu.MicrofrontendId,
				WebpageId:       menu.WebpageId,
				Attributes:      attributes,
			},
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

func (h *HandlerV1) GetMenuByID(c *gin.Context) {
	menuID := c.Param("menu_id")
	var (
		resp *obs.Menu
	)

	if !util.IsValidUUID(menuID) {
		h.HandleResponse(c, status_http.InvalidArgument, "menu id is an invalid uuid")
		return
	}

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().GetByID(
			context.Background(),
			&obs.MenuPrimaryKey{
				Id:        menuID,
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

func (h *HandlerV1) GetAllMenus(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	var (
		resp *obs.GetAllMenusResponse
	)
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
	authInfo, _ := h.GetAuthInfo(c)
	limit := 100

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

		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().GetAll(
			context.Background(),
			&obs.GetAllMenusRequest{
				Limit:     int32(limit),
				Offset:    int32(offset),
				Search:    c.DefaultQuery("search", ""),
				ProjectId: resource.ResourceEnvironmentId,
				ParentId:  c.DefaultQuery("parent_id", ""),
				RoleId:    authInfo.GetRoleId(),
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

func (h *HandlerV1) UpdateMenu(c *gin.Context) {
	var (
		menu models.Menu
		resp *obs.Menu
	)

	err := c.ShouldBindJSON(&menu)
	if err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if menu.Attributes == nil {
		menu.Attributes = make(map[string]any)
	}

	attributes, err := helper.ConvertMapToStruct(menu.Attributes)
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
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().Update(
			context.Background(),
			&obs.Menu{
				Id:              menu.Id,
				Icon:            menu.Icon,
				TableId:         menu.TableId,
				LayoutId:        menu.LayoutId,
				Label:           menu.Label,
				ParentId:        menu.ParentId,
				Type:            menu.Type,
				ProjectId:       resource.ResourceEnvironmentId,
				MicrofrontendId: menu.MicrofrontendId,
				WebpageId:       menu.WebpageId,
				Attributes:      attributes,
				IsVisible:       menu.IsVisible,
				WikiId:          menu.WikiId,
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

func (h *HandlerV1) DeleteMenu(c *gin.Context) {
	menuID := c.Param("menu_id")
	var (
		resp *emptypb.Empty
	)

	if !util.IsValidUUID(menuID) {
		h.HandleResponse(c, status_http.InvalidArgument, "menu id is an invalid uuid")
		return
	}

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().Delete(
			context.Background(),
			&obs.MenuPrimaryKey{
				Id:        menuID,
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

	h.HandleResponse(c, status_http.NoContent, resp)
}

func (h *HandlerV1) UpdateMenuOrder(c *gin.Context) {
	var (
		resp  *emptypb.Empty
		menus obs.UpdateMenuOrderRequest
	)
	err := c.ShouldBindJSON(&menus)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().UpdateMenuOrder(
			context.Background(),
			&obs.UpdateMenuOrderRequest{
				Menus:     menus.GetMenus(),
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

	h.HandleResponse(c, status_http.NoContent, resp)
}

func (h *HandlerV1) CreateMenuSettings(c *gin.Context) {
	var (
		menu obs.CreateMenuSettingsRequest
		resp *obs.MenuSettings
	)

	err := c.ShouldBindJSON(&menu)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	menu.ProjectId = resource.ResourceEnvironmentId

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().CreateMenuSettings(
			context.Background(),
			&menu,
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

func (h *HandlerV1) GetAllMenuSettings(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	var (
		resp *obs.GetAllMenuSettingsResponse
	)
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
		resp, _ = services.GetBuilderServiceByType(resource.NodeType).Menu().GetAllMenuSettings(
			context.Background(),
			&obs.GetAllMenuSettingsRequest{
				Limit:     int32(limit),
				Offset:    int32(offset),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}
	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) GetMenuSettingByID(c *gin.Context) {
	ID := c.Param("id")
	var (
		resp *obs.MenuSettings
	)
	if !util.IsValidUUID(ID) {
		h.HandleResponse(c, status_http.InvalidArgument, "menu id is an invalid uuid")
		return
	}
	template_id := c.Query("template_id")

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().GetByIDMenuSettings(
			context.Background(),
			&obs.MenuSettingPrimaryKey{
				Id:         ID,
				ProjectId:  resource.ResourceEnvironmentId,
				TemplateId: template_id,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}
	if IsEmptyStruct2(resp.MenuTemplate) {
		resp.MenuTemplate, err = h.GetMenuTemplateById(template_id, services)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		resp.MenuTemplateId = template_id
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func IsEmptyStruct2(s any) bool {
	return reflect.DeepEqual(s, reflect.Zero(reflect.TypeOf(s)).Interface())
}

func (h *HandlerV1) UpdateMenuSettings(c *gin.Context) {
	var (
		menu obs.UpdateMenuSettingsRequest
		resp *emptypb.Empty
	)

	err := c.ShouldBindJSON(&menu)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	menu.ProjectId = resource.ResourceEnvironmentId

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().UpdateMenuSettings(
			context.Background(),
			&menu,
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

func (h *HandlerV1) DeleteMenuSettings(c *gin.Context) {
	ID := c.Param("id")
	var (
		resp *emptypb.Empty
	)

	if !util.IsValidUUID(ID) {
		h.HandleResponse(c, status_http.InvalidArgument, "menu id is an invalid uuid")
		return
	}

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().DeleteMenuSettings(
			context.Background(),
			&obs.MenuSettingPrimaryKey{
				Id:        ID,
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

	h.HandleResponse(c, status_http.NoContent, resp)
}

func (h *HandlerV1) CreateMenuTemplate(c *gin.Context) {
	var (
		menu obs.CreateMenuTemplateRequest
		resp *obs.MenuTemplate
	)

	err := c.ShouldBindJSON(&menu)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	menu.ProjectId = resource.ResourceEnvironmentId

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().CreateMenuTemplate(
			context.Background(),
			&menu,
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

func (h *HandlerV1) CreateProjectMenuTemplate(c *gin.Context) {
	var (
		menuTemplate pb.CreateMenuTemplateRequest
	)

	err := c.ShouldBindJSON(&menuTemplate)
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

	menuTemplate.ProjectId = resource.ResourceEnvironmentId
	resp, err := h.companyServices.Company().CreateMenuTemplate(c.Request.Context(), &menuTemplate)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, resp)
}

func (h *HandlerV1) GetProjectMenuTemplates(c *gin.Context) {
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

	resp, err := h.companyServices.Company().GetProjectMenuTemplates(
		c.Request.Context(),
		&pb.GetProjectMenuTemplateRequest{
			ProjectId: projectId.(string),
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) GetAllMenuTemplates(c *gin.Context) {
	offset, err := h.getOffsetParam(c)
	var (
		resp *obs.GatAllMenuTemplateResponse
	)
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
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().GetAllMenuTemplate(
			context.Background(),
			&obs.GetAllMenuSettingsRequest{
				Limit:     int32(limit),
				Offset:    int32(offset),
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

	globalMenus, err := h.companyServices.Company().GetAllMenuTemplate(context.Background(), &emptypb.Empty{})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for _, value := range globalMenus.GetMenuTemplates() {
		resp.MenuTemplates = append(resp.MenuTemplates, &obs.MenuTemplate{
			Id:               value.GetId(),
			Background:       value.GetBackground(),
			ActiveBackground: value.GetActiveBackground(),
			Text:             value.GetText(),
			ActiveText:       value.GetActiveText(),
			Title:            value.GetTitle(),
		})
	}

	resp.Count += int32(len(globalMenus.GetMenuTemplates()))
	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) GetMenuTemplateByID(c *gin.Context) {
	var (
		ID   = c.Param("id")
		resp *obs.MenuTemplate
	)

	if !util.IsValidUUID(ID) {
		h.HandleResponse(c, status_http.InvalidArgument, "menu id is an invalid uuid")
		return
	}

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().GetByIDMenuTemplate(
			context.Background(),
			&obs.MenuSettingPrimaryKey{
				Id:        ID,
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

	if resp == (&obs.MenuTemplate{}) {
		resp, err = h.GetMenuTemplateById(ID, services)
		if err != nil {
			return
		}
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func (h *HandlerV1) GetMenuTemplateById(id string, services services.ServiceManagerI) (*obs.MenuTemplate, error) {
	var resp *obs.MenuTemplate
	global, err := h.companyServices.Company().GetMenuTemplateById(context.Background(), &pb.GetMenuTemplateRequest{
		Id: id,
	})
	if err != nil {
		return nil, err
	}
	resp = &obs.MenuTemplate{
		Id:               global.GetId(),
		Background:       global.GetBackground(),
		ActiveBackground: global.GetActiveBackground(),
		Text:             global.GetText(),
		ActiveText:       global.GetActiveText(),
		Title:            global.GetTitle(),
	}
	return resp, nil
}

func (h *HandlerV1) UpdateMenuTemplate(c *gin.Context) {
	var (
		menu obs.UpdateMenuTemplateRequest
		resp *emptypb.Empty
	)

	err := c.ShouldBindJSON(&menu)
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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	menu.ProjectId = resource.ResourceEnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().UpdateMenuTemplate(
			context.Background(),
			&menu,
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

func (h *HandlerV1) DeleteMenuTemplate(c *gin.Context) {
	var (
		ID   = c.Param("id")
		resp *emptypb.Empty
	)

	if !util.IsValidUUID(ID) {
		h.HandleResponse(c, status_http.InvalidArgument, "menu id is an invalid uuid")
		return
	}

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
		projectId.(string),
		resource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Menu().DeleteMenuTemplate(
			context.Background(),
			&obs.MenuSettingPrimaryKey{
				Id:        ID,
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

	h.HandleResponse(c, status_http.NoContent, resp)
}

func (h *HandlerV1) GetWikiFolder(c *gin.Context) {
	var resp *obs.GetAllMenusResponse

	projectId := c.DefaultQuery("project_id", "")
	if !util.IsValidUUID(projectId) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId := c.DefaultQuery("environment_id", "")
	if !util.IsValidUUID(environmentId) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	services, _ := h.GetProjectSrvc(
		c.Request.Context(),
		projectId,
		resource.NodeType,
	)

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, _ = services.GetBuilderServiceByType(resource.NodeType).Menu().GetWikiFolder(
			context.Background(),
			&obs.GetWikiFolderRequest{
				ProjectId: resource.ResourceEnvironmentId,
				ParentId:  c.DefaultQuery("parent_id", ""),
				IsVisible: true,
			},
		)
	case pb.ResourceType_POSTGRESQL:
		// Does Not Implemented
	}
	h.HandleResponse(c, status_http.OK, resp)
}

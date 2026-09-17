package handlers

import (
	"errors"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbCompany "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/gin-gonic/gin"
	"github.com/saidamir98/udevs_pkg/util"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/emptypb"
)

// GetGlobalPermissionByRoleId godoc
// @ID get_global_permission_by_role_id
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role-golabal-permission/{project-id}/{role-id} [GET]
// @Summary Get Global Role By ID
// @Description Get Global Role By ID
// @Tags V2_Permission
// @Accept json
// @Produce json
// @Param role-id path string true "role-id"
// @Param project-id query string false "project-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetGlobalPermission(c *gin.Context) {
	roleId := c.Param("role-id")
	if !util.IsValidUUID(roleId) {
		h.handleResponse(c, http.InvalidArgument, "role id is an invalid uuid")
		return
	}
	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	h.handleResponse(c, http.NoContent, nil)
}

// @Security ApiKeyAuth
// V2AddRole godoc
// @ID create_role_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role [POST]
// @Summary Create Role
// @Description Create Role
// @Tags V2_Role
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param role body auth_service.V2AddRoleRequest true "AddRoleRequestBody"
// @Success 201 {object} http.Response{data=models.CommonMessage} "Role data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2AddRole(c *gin.Context) {
	var (
		role auth_service.V2AddRoleRequest
		resp *auth_service.CommonMessage
	)

	if err := c.ShouldBindJSON(&role); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	role.ProjectId = resource.ResourceEnvironmentId
	role.ResourceType = int32(resource.ResourceType)
	role.NodeType = resource.NodeType

	userId, _ := c.Get("user_id")
	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "CREATE ROLE",
			UserInfo:     cast.ToString(userId),
			Request:      &role,
			TableSlug:    "ROLE",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.log.Info("!!!V2AddRole -> error")
		} else {
			logReq.Response = resp
			h.log.Info("V2AddRole -> success")
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.services.PermissionService().V2AddRole(
		c.Request.Context(), &role,
	)
	if err != nil {
		var httpErrorStr = ""
		httpErrorStr = strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
		httpErrorStr = strings.ToLower(httpErrorStr)

		switch httpErrorStr {
		case "invalid role":
			h.handleResponse(c, http.InvalidArgument, "Inactive role is already exist for this client_type")
			return
		default:
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	}
	h.handleResponse(c, http.Created, resp)
}

// V2GetRoleByID godoc
// @ID get_role_by_id_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role/{role-id} [GET]
// @Summary Get Role By ID
// @Description Get Role By ID
// @Tags V2_Role
// @Accept json
// @Produce json
// @Param role-id path string true "role-id"
// @Param project-id query string false "project-id"
// @Success 200 {object} http.Response{data=models.CommonMessage} "ClientTypeBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetRoleByID(c *gin.Context) {
	var err error

	roleId := c.Param("role-id")
	if !util.IsValidUUID(roleId) {
		h.handleResponse(c, http.InvalidArgument, "role id is an invalid uuid")
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resp, err := h.services.PermissionService().V2GetRoleById(c.Request.Context(), &auth_service.V2RolePrimaryKey{
		Id:           roleId,
		ProjectId:    resource.ResourceEnvironmentId,
		ResourceType: int32(resource.ResourceType),
		NodeType:     resource.NodeType,
	})

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// V2GetRolesList godoc
// @ID get_roles_list_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role [GET]
// @Summary Get Roles List
// @Description  Get Roles List
// @Tags V2_Role
// @Accept json
// @Produce json
// @Param offset query integer false "offset"
// @Param limit query integer false "limit"
// @Param client-platform-id query string false "client-platform-id"
// @Param client-type-id query string false "client-type-id"
// @Param project-id query string false "project-id"
// @Success 200 {object} http.Response{data=models.CommonMessage} "GetRolesListResponseBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2GetRolesList(c *gin.Context) {
	var err error

	offset, err := h.getOffsetParam(c)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	limit, err := h.getLimitParam(c)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	var clientTypeId string
	if c.Query("clienty_type_id") != "" {
		clientTypeId = c.Query("clienty_type_id")
	} else {
		clientTypeId = c.Query("client-type-id")
	}

	resp, err := h.services.PermissionService().V2GetRolesList(
		c.Request.Context(), &auth_service.V2GetRolesListRequest{
			Offset:                uint32(offset),
			Limit:                 uint32(limit),
			ClientPlatformId:      c.Query("client-platform-id"),
			ClientTypeId:          clientTypeId,
			ProjectId:             resource.ProjectId,
			ResourceEnvironmentId: resource.ResourceEnvironmentId,
			ResourceType:          int32(resource.ResourceType),
			NodeType:              resource.NodeType,
			Status:                cast.ToBool(c.DefaultQuery("status", "true")),
			FromPermission:        cast.ToBool(c.DefaultQuery("from_permission", "true")),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// V2UpdateRole godoc
// @ID update_role_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role [PUT]
// @Summary Update Role
// @Description Update Role
// @Tags V2_Role
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param role body auth_service.V2UpdateRoleRequest true "UpdateRoleRequestBody"
// @Success 200 {object} http.Response{data=models.CommonMessage} "Role data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2UpdateRole(c *gin.Context) {
	var role auth_service.V2UpdateRoleRequest

	err := c.ShouldBindJSON(&role)
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	role.ProjectId = resource.ResourceEnvironmentId
	role.ResourceType = int32(resource.ResourceType)
	role.NodeType = resource.NodeType

	resp, err := h.services.PermissionService().V2UpdateRole(
		c.Request.Context(), &role,
	)

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// @Security ApiKeyAuth
// V2RemoveRole godoc
// @ID delete_role_v2
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role/{role-id} [DELETE]
// @Summary Delete Role
// @Description Get Role
// @Tags V2_Role
// @Accept json
// @Produce json
// @Param role-id path string true "role-id"
// @Param project-id query string false "project-id"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2RemoveRole(c *gin.Context) {
	var (
		err  error
		resp *auth_service.CommonMessage
	)

	roleID := c.Param("role-id")
	if !util.IsValidUUID(roleID) {
		h.handleResponse(c, http.InvalidArgument, "role id is an invalid uuid")
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	userId, _ := c.Get("user_id")
	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE ROLE",
			UserInfo:     cast.ToString(userId),
			Request:      roleID,
			TableSlug:    "ROLE",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.log.Info("!!!V2RemoveRole -> error")
		} else {
			logReq.Response = resp
			h.log.Info("V2RemoveRole -> success")
		}

	}()

	resp, err = h.services.PermissionService().V2RemoveRole(
		c.Request.Context(), &auth_service.V2RolePrimaryKey{
			Id:                    roleID,
			ProjectId:             resource.ResourceEnvironmentId,
			ResourceType:          int32(resource.ResourceType),
			NodeType:              resource.NodeType,
			ResourceEnvironmentId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

// @Security ApiKeyAuth
// GetListWithRoleAppTablePermissions godoc
// @ID get_list_with_role_app_table_permissions
// @Param Resource-Id header string true "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role-permission/detailed/{project-id}/{role-id} [GET]
// @Summary Get Permission List
// @Description  Get Permission List
// @Tags V2_Permission
// @Accept json
// @Produce json
// @Param project-id path string false "project-id"
// @Param role-id path string false "role-id"
// @Success 200 {object} http.Response{data=obs.GetListWithRoleAppTablePermissionsResponse} "GetPermissionListResponseBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetListWithRoleAppTablePermissions(c *gin.Context) {
	var (
		resp *obs.GetListWithRoleAppTablePermissionsResponse
		err  error
	)

	projectId := c.Param("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.BadRequest, "not valid project id")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, resource.ProjectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pbCompany.ResourceType_MONGODB:
		resp, err = services.GetBuilderPermissionServiceByType(resource.NodeType).GetListWithRoleAppTablePermissions(
			c.Request.Context(), &obs.GetListWithRoleAppTablePermissionsRequest{
				RoleId:    c.Param("role-id"),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		resp.ProjectId = projectId

		h.handleResponse(c, http.OK, resp)
	case pbCompany.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderPermissionService().GetListWithRoleAppTablePermissions(c.Request.Context(),
			&nb.GetListWithRoleAppTablePermissionsRequest{
				ProjectId: resource.ResourceEnvironmentId,
				RoleId:    c.Param("role-id"),
			})

		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		resp.ProjectId = projectId

		h.handleResponse(c, http.OK, resp)

	}
}

// @Security ApiKeyAuth
// UpdateRoleAppTablePermissions godoc
// @ID update_role_app_table_permissions
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/role-permission/detailed [PUT]
// @Summary Update Permission
// @Description Update Permission
// @Tags V2_Permission
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param permission body obs.UpdateRoleAppTablePermissionsRequest true "UpdateRoleRequestBody"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UpdateRoleAppTablePermissions(c *gin.Context) {
	var (
		permission obs.UpdateRoleAppTablePermissionsRequest
		resp       *emptypb.Empty
	)

	if err := c.ShouldBindJSON(&permission); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(),
		&pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	userId, _ := c.Get("user_id")

	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE PERMISSION",
			UserInfo:     cast.ToString(userId),
			Request:      &permission,
			TableSlug:    "ROLE_PERMISSION",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.log.Info("!!!UpdateRoleAppTablePermissions -> error")
		} else {
			logReq.Response = resp
			h.log.Info("UpdateRoleAppTablePermissions -> success")
		}
		go func() { _ = h.versionHistory(logReq) }()
	}()

	services, err := h.GetProjectSrvc(c, resource.ProjectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	permission.ProjectId = resource.ResourceEnvironmentId
	switch resource.ResourceType {
	case pbCompany.ResourceType_MONGODB:
		resp, err = services.GetBuilderPermissionServiceByType(resource.NodeType).UpdateRoleAppTablePermissions(
			c.Request.Context(), &permission,
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, http.OK, resp)
	case pbCompany.ResourceType_POSTGRESQL:
		newPermission := &nb.UpdateRoleAppTablePermissionsRequest{}

		if err := helper.MarshalToStruct(&permission, &newPermission); err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		resp, err := services.GoObjectBuilderPermissionService().UpdateRoleAppTablePermissions(
			c.Request.Context(), newPermission,
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		h.handleResponse(c, http.OK, resp)
	}

}

// @Security ApiKeyAuth
// GetListMenuPermissions godoc
// @ID get_list_menu_permissions
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/menu-permission/detailed/{project-id}/{role-id}/{parent-id} [GET]
// @Summary Get Permission List
// @Description  Get Permission List
// @Tags V2_Permission
// @Accept json
// @Produce json
// @Param project-id path string true "project-id"
// @Param role-id path string true "role-id"
// @Param parent-id path string true "parent-id"
// @Success 200 {object} http.Response{data=obs.GetAllMenuPermissionsResponse} "GetMenuPermissionListResponseBody"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetListMenuPermissions(c *gin.Context) {
	var (
		resp *obs.GetAllMenuPermissionsResponse
		err  error
	)

	projectId := c.Param("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.BadRequest, "not valid project id")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}
	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	services, _ := h.GetProjectSrvc(c, resource.ProjectId, resource.NodeType)

	switch resource.ResourceType {
	case pbCompany.ResourceType_MONGODB:
		resp, err = services.GetBuilderPermissionServiceByType(resource.NodeType).GetAllMenuPermissions(
			c.Request.Context(),
			&obs.GetAllMenuPermissionsRequest{
				RoleId:    c.Param("role-id"),
				ProjectId: resource.ResourceEnvironmentId,
				ParentId:  c.Param("parent-id"),
			},
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	case pbCompany.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderPermissionService().GetAllMenuPermissions(
			c.Request.Context(),
			&nb.GetAllMenuPermissionsRequest{
				RoleId:    c.Param("role-id"),
				ProjectId: resource.ResourceEnvironmentId,
				ParentId:  c.Param("parent-id"),
			},
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		h.handleResponse(c, http.OK, resp)
		return
	}
	h.handleResponse(c, http.OK, resp)
}

// @Security ApiKeyAuth
// UpdateMenuPermissions godoc
// @ID update_menu_permissions
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/menu-permission/detailed [PUT]
// @Summary Update Permission
// @Description Update Permission
// @Tags V2_Permission
// @Accept json
// @Produce json
// @Param project-id query string true "project-id"
// @Param permission body obs.UpdateMenuPermissionsRequest true "UpdateMenuPermissionRequestBody"
// @Success 200 {object} http.Response{data=obs.UpdateMenuPermissionsRequest} "Role data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) UpdateMenuPermissions(c *gin.Context) {
	var (
		permission obs.UpdateMenuPermissionsRequest
		resp       *emptypb.Empty
	)

	err := c.ShouldBindJSON(&permission)
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId := c.Query("project-id")
	if !util.IsValidUUID(projectId) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(),
		&pbCompany.GetSingleServiceResourceReq{
			ProjectId:     projectId,
			EnvironmentId: environmentId.(string),
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	userId, _ := c.Get("user_id")
	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE PERMISSION",
			UserInfo:     cast.ToString(userId),
			Request:      &permission,
			TableSlug:    "MENU_PERMISSION",
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
		} else {
			logReq.Response = resp
		}
		go func() { _ = h.versionHistory(logReq) }()
	}()

	services, err := h.GetProjectSrvc(c, resource.ProjectId, resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	permission.ProjectId = resource.ResourceEnvironmentId
	switch resource.ResourceType {
	case pbCompany.ResourceType_MONGODB:
		resp, err = services.GetBuilderPermissionServiceByType(resource.NodeType).UpdateMenuPermissions(
			c.Request.Context(), &permission,
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	case pbCompany.ResourceType_POSTGRESQL:
		newReq := nb.UpdateMenuPermissionsRequest{}

		if err = helper.MarshalToStruct(&permission, &newReq); err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		resp, err = services.GoObjectBuilderPermissionService().UpdateMenuPermissions(
			c.Request.Context(), &newReq,
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, http.OK, resp)
}

package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"log"

	"github.com/gin-gonic/gin"
)

// ==================== Definition ====================

func (h *HandlerV1) CreateCustomPermission(c *gin.Context) {
	var request pbo.CreateCustomPermissionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	request.ProjectId = resourceEnvId

	response, err := service.GoObjectBuilderService().CustomPermission().CreateCustomPermission(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	log.Println("RESPONSE:", response)

	h.HandleResponse(c, status_http.Created, response)
}

func (h *HandlerV1) UpdateCustomPermission(c *gin.Context) {
	var request pbo.UpdateCustomPermissionRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	request.ProjectId = resourceEnvId

	response, err := service.GoObjectBuilderService().CustomPermission().UpdateCustomPermission(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) DeleteCustomPermission(c *gin.Context) {
	var id = c.Param("id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid id")
		return
	}

	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomPermission().DeleteCustomPermission(
		c.Request.Context(), &pbo.DeleteCustomPermissionRequest{
			ProjectId: resourceEnvId,
			Id:        id,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) GetAllCustomPermissions(c *gin.Context) {
	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomPermission().GetAllCustomPermissions(
		c.Request.Context(), &pbo.GetAllCustomPermissionsRequest{
			ProjectId: resourceEnvId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ==================== Access ====================

func (h *HandlerV1) GetCustomPermissionAccesses(c *gin.Context) {
	var (
		roleId       = c.Query("role_id")
		clientTypeId = c.Query("client_type_id")
		parentId     = c.Query("parent_id")
	)

	if roleId == "" || clientTypeId == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "role_id and client_type_id are required")
		return
	}

	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomPermission().GetCustomPermissionAccesses(
		c.Request.Context(), &pbo.GetCustomPermissionAccessesRequest{
			ProjectId:    resourceEnvId,
			RoleId:       roleId,
			ClientTypeId: clientTypeId,
			ParentId:     parentId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) GetAllCustomPermissionAccesses(c *gin.Context) {
	var (
		roleId       = c.Query("role_id")
		clientTypeId = c.Query("client_type_id")
	)

	if roleId == "" || clientTypeId == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "role_id and client_type_id are required")
		return
	}

	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().CustomPermission().GetAllCustomPermissionAccesses(
		c.Request.Context(), &pbo.GetAllCustomPermissionAccessesRequest{
			ProjectId:    resourceEnvId,
			RoleId:       roleId,
			ClientTypeId: clientTypeId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

func (h *HandlerV1) UpdateCustomPermissionAccess(c *gin.Context) {
	var request pbo.UpdateCustomPermissionAccessRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if request.RoleId == "" || request.ClientTypeId == "" {
		h.HandleResponse(c, status_http.InvalidArgument, "role_id and client_type_id are required")
		return
	}

	service, resourceEnvId, err := h.getCustomPermissionServices(c)
	if err != nil {
		return
	}

	request.ProjectId = resourceEnvId

	response, err := service.GoObjectBuilderService().CustomPermission().UpdateCustomPermissionAccess(c.Request.Context(), &request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ==================== Helper ====================

func (h *HandlerV1) getCustomPermissionServices(c *gin.Context) (services.ServiceManagerI, string, error) {

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return nil, "", config.ErrProjectIdValid
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return nil, "", config.ErrEnvironmentIdValid
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
		return nil, "", err
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return nil, "", config.ErrProjectIdValid
	}

	service, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return nil, "", err
	}

	return service, resource.ResourceEnvironmentId, nil
}

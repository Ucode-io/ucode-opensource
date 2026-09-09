package v1

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// ==================== Helper ====================

func (h *HandlerV1) getProjectFolderServices(c *gin.Context) (services.ServiceManagerI, string, error) {
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

// ==================== Create ====================

func (h *HandlerV1) CreateProjectFolder(c *gin.Context) {
	var request pbo.CreateProjectFolderRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getProjectFolderServices(c)
	if err != nil {
		return
	}

	request.ResourceEnvId = resourceEnvId

	response, err := service.GoObjectBuilderService().ProjectFolders().CreateProjectFolder(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.Created, response)
}

// ==================== Get By ID ====================

func (h *HandlerV1) GetProjectFolderById(c *gin.Context) {
	var id = c.Param("folder_id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid folder id")
		return
	}

	service, resourceEnvId, err := h.getProjectFolderServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().ProjectFolders().GetProjectFolderById(
		c.Request.Context(), &pbo.ProjectFolderPrimaryKey{
			ResourceEnvId: resourceEnvId,
			Id:            id,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ==================== Get All ====================

func (h *HandlerV1) GetAllProjectFolders(c *gin.Context) {
	var (
		parentId       = c.Query("parent_id")
		folderType     = c.Query("type")
		label          = c.Query("label")
		orderBy        = c.Query("order_by")
		orderDirection = c.Query("order_direction")
		limit          = cast.ToInt32(c.Query("limit"))
		offset         = cast.ToInt32(c.Query("offset"))
	)

	service, resourceEnvId, err := h.getProjectFolderServices(c)
	if err != nil {
		return
	}

	response, err := service.GoObjectBuilderService().ProjectFolders().GetAllProjectFolders(
		c.Request.Context(), &pbo.GetAllProjectFoldersRequest{
			ResourceEnvId:  resourceEnvId,
			ParentId:       parentId,
			Type:           folderType,
			Label:          label,
			OrderBy:        orderBy,
			OrderDirection: orderDirection,
			Limit:          limit,
			Offset:         offset,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ==================== Update ====================

func (h *HandlerV1) UpdateProjectFolder(c *gin.Context) {
	var request pbo.UpdateProjectFolderRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	var id = c.Param("folder_id")
	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid folder id")
		return
	}

	service, resourceEnvId, err := h.getProjectFolderServices(c)
	if err != nil {
		return
	}

	request.ResourceEnvId = resourceEnvId
	request.Id = id

	response, err := service.GoObjectBuilderService().ProjectFolders().UpdateProjectFolder(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, response)
}

// ==================== Delete ====================

func (h *HandlerV1) DeleteProjectFolder(c *gin.Context) {
	var id = c.Param("folder_id")

	if !util.IsValidUUID(id) {
		h.HandleResponse(c, status_http.InvalidArgument, "invalid folder id")
		return
	}

	service, resourceEnvId, err := h.getProjectFolderServices(c)
	if err != nil {
		return
	}

	_, err = service.GoObjectBuilderService().ProjectFolders().DeleteProjectFolder(
		c.Request.Context(), &pbo.ProjectFolderPrimaryKey{
			ResourceEnvId: resourceEnvId,
			Id:            id,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, "deleted")
}

// ==================== Update Order ====================

func (h *HandlerV1) UpdateProjectFolderOrder(c *gin.Context) {
	var request pbo.UpdateProjectFolderOrderRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	service, resourceEnvId, err := h.getProjectFolderServices(c)
	if err != nil {
		return
	}

	request.ResourceEnvId = resourceEnvId

	_, err = service.GoObjectBuilderService().ProjectFolders().UpdateProjectFolderOrder(
		c.Request.Context(), &request,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, "order updated")
}

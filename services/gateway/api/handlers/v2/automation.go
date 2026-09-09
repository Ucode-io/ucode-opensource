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
)

// CreateAutomation godoc
// @Security ApiKeyAuth
// @ID create_automation
// @Router /v2/collections/{collection}/automation [POST]
// @Summary Create Automation
// @Description Create Automation
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param Automation body obs.CreateCustomEventRequest true "AutomationRequestBody"
// @Success 201 {object} status_http.Response{data=obs.CustomEvent} "Automation data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateAutomation(c *gin.Context) {
	var (
		customevent models.CreateCustomEventRequest
		resp        *obs.CustomEvent
	)

	if err := c.ShouldBindJSON(&customevent); err != nil {
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

	structData, err := helper.ConvertMapToStruct(customevent.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Create(
			c.Request.Context(), &obs.CreateCustomEventRequest{
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.Created, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().CustomEvent().Create(
			c.Request.Context(), &nb.CreateCustomEventRequest{
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
				Path:       customevent.Path,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.Created, resp)
	}

}

// GetByIdAutomation godoc
// @Security ApiKeyAuth
// @ID get_automatio_by_id
// @Router /v2/collections/{collection}/automation/{id} [GET]
// @Summary Get Automation by id
// @Description Get Automation by id
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=obs.CustomEvent} "AutomationBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetByIdAutomation(c *gin.Context) {
	customeventID := c.Param("id")

	if !util.IsValidUUID(customeventID) {
		h.HandleResponse(c, status_http.InvalidArgument, "automation id is an invalid uuid")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).CustomEvent().GetSingle(
			c.Request.Context(), &obs.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.EnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().CustomEvent().GetSingle(
			c.Request.Context(), &nb.CustomEventPrimaryKey{
				Id:        customeventID,
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

// GetAllAutomation godoc
// @Security ApiKeyAuth
// @ID get_all_automation
// @Router /v2/collections/{collection}/automation [GET]
// @Summary Get all automation
// @Description Get all automation
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param filters query obs.GetCustomEventsListRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetCustomEventsListResponse} "AutomationBody"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetAllAutomation(c *gin.Context) {
	var resp *obs.GetCustomEventsListResponse

	authInfo, err := h.GetAuthInfo(c)
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
		resp, err = services.GetBuilderServiceByType(resource.NodeType).CustomEvent().GetList(
			c.Request.Context(), &obs.GetCustomEventsListRequest{
				TableSlug: c.Query("table_slug"),
				RoleId:    authInfo.GetRoleId(),
				ProjectId: resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().CustomEvent().GetList(
			c.Request.Context(), &nb.GetCustomEventsListRequest{
				TableSlug: c.Query("table_slug"),
				RoleId:    authInfo.GetRoleId(),
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

// UpdateAutomation godoc
// @Security ApiKeyAuth
// @ID update_automation
// @Router /v2/collections/{collection}/automation [PUT]
// @Summary Update Automation
// @Description Update automation
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param Customevent body models.CustomEvent true "UpdateAutomationRequestBody"
// @Success 200 {object} status_http.Response{data=string} "Automation data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateAutomation(c *gin.Context) {
	var (
		customevent models.CustomEvent
	)

	if err := c.ShouldBindJSON(&customevent); err != nil {
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	structData, err := helper.ConvertMapToStruct(customevent.Attributes)
	if err != nil {
		h.HandleResponse(c, status_http.InvalidArgument, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Update(
			c.Request.Context(), &obs.CustomEvent{
				Id:         customevent.Id,
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().CustomEvent().Update(
			c.Request.Context(), &nb.CustomEvent{
				Id:         customevent.Id,
				TableSlug:  customevent.TableSlug,
				EventPath:  customevent.EventPath,
				Label:      customevent.Label,
				Icon:       customevent.Icon,
				Url:        customevent.Url,
				Disable:    customevent.Disable,
				ActionType: customevent.ActionType,
				Method:     customevent.Method,
				Attributes: structData,
				ProjectId:  resource.ResourceEnvironmentId,
				Path:       customevent.Path,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}

}

// DeleteAutomation godoc
// @Security ApiKeyAuth
// @ID delete_automation
// @Router /v2/collections/{collection}/automation/{id} [DELETE]
// @Summary Delete Automation
// @Description Delete Automation
// @Tags Collections
// @Accept json
// @Produce json
// @Param collection path string true "collection"
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) DeleteAutomation(c *gin.Context) {
	customeventID := c.Param("id")

	if !util.IsValidUUID(customeventID) {
		h.HandleResponse(c, status_http.InvalidArgument, "Customevent id is an invalid uuid")
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err := services.GetBuilderServiceByType(resource.NodeType).CustomEvent().Delete(
			c.Request.Context(), &obs.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.NoContent, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().CustomEvent().Delete(
			c.Request.Context(), &nb.CustomEventPrimaryKey{
				Id:        customeventID,
				ProjectId: resource.ResourceEnvironmentId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.NoContent, resp)
	}
}

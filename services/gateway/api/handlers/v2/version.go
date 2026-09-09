package v2

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// CreateVersion godoc
// @Security ApiKeyAuth
// @ID create_version
// @Router /v2/version [POST]
// @Summary Create version
// @Description Create version
// @Tags Version
// @Accept json
// @Produce json
// @Param version body obs.CreateVersionRequest true "CreateVersionRequest"
// @Success 201 {object} status_http.Response{data=obs.Version} "Version data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) CreateVersion(c *gin.Context) {
	var version obs.CreateVersionRequest

	if err := c.ShouldBindJSON(&version); err != nil {
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

	info, err := h.authService.User().GetUserByID(
		context.Background(), &auth_service.UserPrimaryKey{
			Id: userId.(string),
		},
	)
	if err == nil {
		if info.Login != "" {
			version.UserInfo = info.Login
		} else {
			version.UserInfo = info.Phone
		}
	}

	version.ProjectId = resource.ResourceEnvironmentId
	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		var resp *obs.Version

		resp, err = services.GetBuilderServiceByType(resource.NodeType).Version().Create(
			c.Request.Context(), &version,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.Created, resp)
	case pb.ResourceType_POSTGRESQL:
		var resp *nb.Version
		resp, err = services.GoObjectBuilderService().Version().Create(
			c.Request.Context(), &nb.CreateVersionRequest{
				Name:          version.Name,
				Description:   version.Description,
				IsCurrent:     version.IsCurrent,
				UserInfo:      version.UserInfo,
				VersionNumber: version.VersionNumber,
				ProjectId:     version.ProjectId,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.Created, resp)
	}
}

// GeVersionList godoc
// @Security ApiKeyAuth
// @ID get_version_list
// @Router /v2/version [GET]
// @Summary Get version list
// @Description Get version list
// @Tags Version
// @Accept json
// @Produce json
// @Param filters query obs.GetVersionListRequest true "filters"
// @Success 200 {object} status_http.Response{data=obs.GetVersionListResponse} "VersionList"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) GetVersionList(c *gin.Context) {
	var (
		resp *obs.GetVersionListResponse
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

	services, err := h.GetProjectSrvc(c.Request.Context(), resource.GetProjectId(), resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		resp, err = services.GetBuilderServiceByType(resource.NodeType).Version().GetList(
			c.Request.Context(), &obs.GetVersionListRequest{
				ProjectId: resource.ResourceEnvironmentId,
				Offset:    int32(offset),
				Limit:     int32(limit),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		h.HandleResponse(c, status_http.OK, resp)
	case pb.ResourceType_POSTGRESQL:
		resp, err := services.GoObjectBuilderService().Version().GetList(
			c.Request.Context(), &nb.GetVersionListRequest{
				ProjectId: resource.ResourceEnvironmentId,
				Offset:    int32(offset),
				Limit:     int32(limit),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		h.HandleResponse(c, status_http.OK, resp)
	}
}

// UpdateVersion godoc
// @Security ApiKeyAuth
// @ID update_version
// @Router /v2/version [PUT]
// @Summary Update version
// @Description Update version
// @Tags Version
// @Accept json
// @Produce json
// @Param version body obs.Version true "Version"
// @Success 200 {object} status_http.Response{data=obs.Version} "UpdateVersionRequest"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) UpdateVersion(c *gin.Context) {
	var (
		version obs.Version
		resp    *obs.Version
	)

	if err := c.ShouldBindJSON(&version); err != nil {
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

	userId, _ := c.Get("user_id")

	var (
		oldView = &obs.View{}
		logReq  = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: "VERSION",
			ActionType:   "UPDATE VERSION",
			UserInfo:     cast.ToString(userId),
			Request:      &version,
			TableSlug:    "version",
		}
	)

	defer func() {
		logReq.Previous = oldView
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			logReq.Response = resp
			logReq.Current = resp
		}
		go h.versionHistory(logReq)
	}()

	version.ProjectId = resource.ResourceEnvironmentId

	switch resource.ResourceType {
	case pb.ResourceType_MONGODB:
		_, err = services.GetBuilderServiceByType(resource.NodeType).Version().Update(
			c.Request.Context(), &version,
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	case pb.ResourceType_POSTGRESQL:
		_, err = services.GoObjectBuilderService().Version().Update(
			c.Request.Context(), &nb.Version{
				Id:            version.Id,
				Name:          version.Name,
				Description:   version.Description,
				IsCurrent:     version.IsCurrent,
				UserInfo:      version.UserInfo,
				VersionNumber: version.VersionNumber,
				ProjectId:     version.ProjectId,
			},
		)

		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

	h.HandleResponse(c, status_http.OK, nil)
}

// PublishVersion godoc
// @Security ApiKeyAuth
// @ID publish_version
// @Router /v2/version/publish [POST]
// @Summary Publish version
// @Description Publish version
// @Tags Version
// @Accept json
// @Produce json
// @Param publish body obs.PublishVersionRequest true "Publish"
// @Success 200 {object} status_http.Response{data=obs.Version} "UpdateVersionRequest"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV2) PublishVersion(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
		return
	}

	currentResource, err := h.companyServices.ServiceResource().GetSingle(
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

	switch currentResource.ResourceType {
	case pb.ResourceType_MONGODB:
		h.PublishVersionMongo(c)
	case pb.ResourceType_POSTGRESQL:
		h.PublishVersionPostgres(c)
	}
}

func (h *HandlerV2) PublishVersionMongo(c *gin.Context) {
	var (
		push       obs.PublishVersionRequest
		fromDate   string
		toDate     string
		upOrDown   bool
		versionIDs []string
	)

	if err := c.ShouldBindJSON(&push); err != nil {
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
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
		return
	}

	userId, _ := c.Get("user_id")

	currentResource, err := h.companyServices.ServiceResource().GetSingle(
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

	currentNodeType := currentResource.NodeType
	currentResourceType := currentResource.ResourceType

	services, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), currentResource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if environmentId.(string) != push.EnvId {
		publishedResource, err := h.companyServices.ServiceResource().GetSingle(
			c.Request.Context(),
			&pb.GetSingleServiceResourceReq{
				ProjectId:     projectId.(string),
				EnvironmentId: push.EnvId,
				ServiceType:   pb.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		publishedNodeType := publishedResource.NodeType
		resourceType := publishedResource.ResourceType

		publishedServices, err := h.GetProjectSrvc(c.Request.Context(), projectId.(string), publishedResource.NodeType)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		publishedEnvLiveVersion, _ := publishedServices.GetBuilderServiceByType(publishedNodeType).Version().GetSingle(
			c.Request.Context(), &obs.VersionPrimaryKey{
				ProjectId: publishedResource.ResourceEnvironmentId,
				Live:      true,
			},
		)

		if publishedEnvLiveVersion.GetCreatedAt() > push.GetVersion().GetCreatedAt() {
			fromDate = push.GetVersion().GetCreatedAt()
			toDate = publishedEnvLiveVersion.GetCreatedAt()
		} else {
			fromDate = publishedEnvLiveVersion.GetCreatedAt()
			toDate = push.GetVersion().GetCreatedAt()
			upOrDown = true
		}

		versions, err := services.GetBuilderServiceByType(currentNodeType).Version().GetList(
			c.Request.Context(), &obs.GetVersionListRequest{
				ProjectId: currentResource.ResourceEnvironmentId,
				FromDate:  fromDate,
				ToDate:    toDate,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		for _, version := range versions.Versions {
			versionIDs = append(versionIDs, version.Id)
		}

		_, _ = publishedServices.GetBuilderServiceByType(publishedNodeType).Version().CreateMany(
			c.Request.Context(), &obs.CreateManyVersionRequest{
				Versions:  versions.Versions,
				ProjectId: publishedResource.ResourceEnvironmentId,
			},
		)

		activityLogs, err := services.GetBuilderServiceByType(currentNodeType).VersionHistory().GatAll(
			c.Request.Context(), &obs.GetAllRquest{
				ProjectId:  currentResource.ResourceEnvironmentId,
				VersionIds: versionIDs,
				OrderBy:    upOrDown,
				Type:       "UP",
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if upOrDown {
			err = h.MigrateUpByVersion(c, publishedServices, activityLogs, publishedResource.ResourceEnvironmentId, publishedNodeType, userId.(string), resourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		} else {
			err = h.MigrateDownByVersion(c, publishedServices, activityLogs, publishedResource.ResourceEnvironmentId, publishedNodeType, userId.(string), resourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		}

		_, err = publishedServices.GetBuilderServiceByType(publishedNodeType).Version().UpdateLive(
			c.Request.Context(),
			&obs.VersionPrimaryKey{
				ProjectId: publishedResource.ResourceEnvironmentId,
				Id:        push.GetVersion().GetId(),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

	} else {
		publishedEnvLiveVersion, err := services.GetBuilderServiceByType(currentNodeType).Version().GetSingle(
			c.Request.Context(),
			&obs.VersionPrimaryKey{
				ProjectId: currentResource.ResourceEnvironmentId,
				Live:      true,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if publishedEnvLiveVersion.CreatedAt > push.Version.CreatedAt {
			fromDate = push.Version.CreatedAt
			toDate = publishedEnvLiveVersion.CreatedAt
		} else {
			fromDate = publishedEnvLiveVersion.CreatedAt
			toDate = push.Version.CreatedAt
			upOrDown = true
		}

		versions, err := services.GetBuilderServiceByType(currentNodeType).Version().GetList(
			c.Request.Context(),
			&obs.GetVersionListRequest{
				ProjectId: currentResource.ResourceEnvironmentId,
				FromDate:  fromDate,
				ToDate:    toDate,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		for _, version := range versions.Versions {
			versionIDs = append(versionIDs, version.Id)
		}

		activityLogs, err := services.GetBuilderServiceByType(currentNodeType).VersionHistory().GatAll(
			c.Request.Context(),
			&obs.GetAllRquest{
				ProjectId:  currentResource.ResourceEnvironmentId,
				VersionIds: versionIDs,
				OrderBy:    upOrDown,
				Type:       "DOWN",
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if upOrDown {
			err = h.MigrateUpByVersion(c, services, activityLogs, currentResource.ResourceEnvironmentId, currentNodeType, userId.(string), currentResourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		} else {
			err = h.MigrateDownByVersion(c, services, activityLogs, currentResource.ResourceEnvironmentId, currentNodeType, userId.(string), currentResourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		}

		_, err = services.GetBuilderServiceByType(currentNodeType).Version().UpdateLive(
			c.Request.Context(),
			&obs.VersionPrimaryKey{
				ProjectId: currentResource.ResourceEnvironmentId,
				Id:        push.GetVersion().GetId(),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

	}
}

func (h *HandlerV2) PublishVersionPostgres(c *gin.Context) {
	var (
		push       nb.PublishVersionRequest
		fromDate   string
		toDate     string
		upOrDown   bool
		versionIDs []string
	)

	if err := c.ShouldBindJSON(&push); err != nil {
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
		h.HandleResponse(c, status_http.InvalidArgument, "environment id is an invalid uuid")
		return
	}

	userId, _ := c.Get("user_id")

	currentResource, err := h.companyServices.ServiceResource().GetSingle(
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

	currentNodeType := currentResource.NodeType
	currentResourceType := currentResource.ResourceType

	services, err := h.GetProjectSrvc(
		c.Request.Context(),
		projectId.(string),
		currentResource.NodeType,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if environmentId.(string) != push.EnvId {
		publishedResource, err := h.companyServices.ServiceResource().GetSingle(
			c.Request.Context(),
			&pb.GetSingleServiceResourceReq{
				ProjectId:     projectId.(string),
				EnvironmentId: push.EnvId,
				ServiceType:   pb.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		publishedNodeType := publishedResource.NodeType
		resourceType := publishedResource.ResourceType

		publishedServices, err := h.GetProjectSrvc(
			c.Request.Context(),
			projectId.(string),
			publishedResource.NodeType,
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		publishedEnvLiveVersion, _ := publishedServices.GoObjectBuilderService().Version().GetSingle(
			c.Request.Context(),
			&nb.VersionPrimaryKey{
				ProjectId: publishedResource.ResourceEnvironmentId,
				Live:      true,
			},
		)

		if publishedEnvLiveVersion.GetCreatedAt() > push.GetVersion().GetCreatedAt() {
			fromDate = push.GetVersion().GetCreatedAt()
			toDate = publishedEnvLiveVersion.GetCreatedAt()
		} else {
			fromDate = publishedEnvLiveVersion.GetCreatedAt()
			toDate = push.GetVersion().GetCreatedAt()
			upOrDown = true
		}

		versions, err := services.GoObjectBuilderService().Version().GetList(
			c.Request.Context(),
			&nb.GetVersionListRequest{
				ProjectId: currentResource.ResourceEnvironmentId,
				FromDate:  fromDate,
				ToDate:    toDate,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		for _, version := range versions.Versions {
			versionIDs = append(versionIDs, version.Id)
		}

		_, _ = publishedServices.GoObjectBuilderService().Version().CreateMany(
			c.Request.Context(),
			&nb.CreateManyVersionRequest{
				Versions:  versions.Versions,
				ProjectId: publishedResource.ResourceEnvironmentId,
			},
		)

		activityLogs, err := services.GoObjectBuilderService().VersionHistory().GatAll(
			c.Request.Context(),
			&nb.GetAllRquest{
				ProjectId:  currentResource.ResourceEnvironmentId,
				VersionIds: versionIDs,
				OrderBy:    upOrDown,
				Type:       "UP",
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		activityLogsM := &obs.ListVersionHistory{}

		err = helper.MarshalToStruct(&activityLogs, &activityLogsM)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if upOrDown {
			err = h.MigrateUpByVersion(c, publishedServices, activityLogsM, publishedResource.ResourceEnvironmentId, publishedNodeType, userId.(string), resourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		} else {
			err = h.MigrateDownByVersion(c, publishedServices, activityLogsM, publishedResource.ResourceEnvironmentId, publishedNodeType, userId.(string), resourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		}

		_, err = publishedServices.GoObjectBuilderService().Version().UpdateLive(
			c.Request.Context(),
			&nb.VersionPrimaryKey{
				ProjectId: publishedResource.ResourceEnvironmentId,
				Id:        push.GetVersion().GetId(),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	} else {
		publishedEnvLiveVersion, err := services.GoObjectBuilderService().Version().GetSingle(
			c.Request.Context(),
			&nb.VersionPrimaryKey{
				ProjectId: currentResource.ResourceEnvironmentId,
				Live:      true,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if publishedEnvLiveVersion.CreatedAt > push.Version.CreatedAt {
			fromDate = push.Version.CreatedAt
			toDate = publishedEnvLiveVersion.CreatedAt
		} else {
			fromDate = publishedEnvLiveVersion.CreatedAt
			toDate = push.Version.CreatedAt
			upOrDown = true
		}

		versions, err := services.GoObjectBuilderService().Version().GetList(
			c.Request.Context(),
			&nb.GetVersionListRequest{
				ProjectId: currentResource.ResourceEnvironmentId,
				FromDate:  fromDate,
				ToDate:    toDate,
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		for _, version := range versions.Versions {
			versionIDs = append(versionIDs, version.Id)
		}

		activityLogs, err := services.GoObjectBuilderService().VersionHistory().GatAll(
			c.Request.Context(),
			&nb.GetAllRquest{
				ProjectId:  currentResource.ResourceEnvironmentId,
				VersionIds: versionIDs,
				OrderBy:    upOrDown,
				Type:       "DOWN",
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		activityLogsM := &obs.ListVersionHistory{}

		err = helper.MarshalToStruct(&activityLogs, &activityLogsM)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if upOrDown {
			err = h.MigrateUpByVersion(c, services, activityLogsM, currentResource.ResourceEnvironmentId, currentNodeType, userId.(string), currentResourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		} else {
			err = h.MigrateDownByVersion(c, services, activityLogsM, currentResource.ResourceEnvironmentId, currentNodeType, userId.(string), currentResourceType)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
		}

		_, err = services.GoObjectBuilderService().Version().UpdateLive(
			c.Request.Context(),
			&nb.VersionPrimaryKey{
				ProjectId: currentResource.ResourceEnvironmentId,
				Id:        push.GetVersion().GetId(),
			},
		)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
	}

}

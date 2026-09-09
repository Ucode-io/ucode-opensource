package v1

import (
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
)

// Defaults previously provided by the Google Calendar integration package.
// The integration itself is not part of the open-source build; these remain so
// stored resource settings keep the same shape.
const (
	defaultGoogleCalendarID            = "primary"
	defaultGoogleCalendarSyncDirection = "ucode_to_google"
)

// AddResourceToProject godoc
// @Security ApiKeyAuth
// @ID add_resource_to_project
// @Router /v2/company/project/resource [POST]
// @Summary Add rosource to project
// @Description Add rosource to project
// @Tags Project resource
// @Accept json
// @Produce json
// @Param data body pb.AddResourceToProjectRequest true "AddResourceToProjectRequest"
// @Success 201 {object} status_http.Response{data=pb.ProjectResource} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) AddResourceToProject(c *gin.Context) {
	var (
		request = &pb.AddResourceToProjectRequest{}
		resp    = &pb.ProjectResource{}
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Secret = nil

	if isTelegramAddResourceRequest(request) {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/telegram endpoints to manage Telegram Support")
		return
	}
	if isInstagramAddResourceRequest(request) {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/instagram endpoints to manage Instagram Support")
		return
	}
	if request.GetType() == pb.ResourceType_GOOGLE_DRIVE {
		if request.GetSettings() == nil || request.GetSettings().GetGoogleDrive() == nil || strings.TrimSpace(request.GetSettings().GetGoogleDrive().GetRefreshToken()) == "" {
			h.HandleResponse(c, status_http.BadRequest, "use /v1/google-drive/connect to connect Google Drive")
			return
		}

		if request.Name == "" {
			request.Name = "Google Drive"
		}
		request.Settings = sanitizeGoogleDriveSettingsForStorage(request.GetSettings(), h.baseConf.GoogleDriveVisibility)
	}
	if request.GetType() == pb.ResourceType_GOOGLE_CALENDAR {
		if request.GetSettings() == nil || request.GetSettings().GetGoogleCalendar() == nil || strings.TrimSpace(request.GetSettings().GetGoogleCalendar().GetRefreshToken()) == "" {
			h.HandleResponse(c, status_http.BadRequest, "use /v1/google-calendar/connect to connect Google Calendar")
			return
		}
		if request.Name == "" {
			request.Name = "Google Calendar"
		}
		request.Settings = sanitizeGoogleCalendarSettingsForStorage(request.GetSettings())
	}

	resp, err := h.companyServices.Resource().AddResourceToProject(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}
	resp.Secret = nil

	if request.GetType() == pb.ResourceType_GOOGLE_DRIVE {
		resp.Settings = request.GetSettings()
		resp.ResourceType = int32(request.GetType())
		sanitizeGoogleDriveResourceForResponse(resp)
	}
	if request.GetType() == pb.ResourceType_GOOGLE_CALENDAR {
		resp.Settings = request.GetSettings()
		resp.ResourceType = int32(request.GetType())
		sanitizeGoogleCalendarResourceForResponse(resp)
	}

	h.HandleResponse(c, status_http.Created, resp)
}

// UpdateProjectResource godoc
// @Security ApiKeyAuth
// @ID update_project_resource
// @Router /v2/company/project/resource [PUT]
// @Summary Update Project resource
// @Description update Project resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param Company body pb.ProjectResource  true "ProjectResource"
// @Success 200 {object} status_http.Response{data=pb.Empty} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Bad Request"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) UpdateProjectResource(c *gin.Context) {
	var (
		request = &pb.ProjectResource{}
		resp    = &pb.Empty{}
	)

	if err := c.ShouldBindJSON(&request); err != nil {
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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Secret = nil

	if isTelegramProjectResource(request) {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/telegram endpoints to manage Telegram Support")
		return
	}
	if isInstagramProjectResource(request) {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/instagram endpoints to manage Instagram Support")
		return
	}
	if util.IsValidUUID(request.GetId()) {
		current, currentErr := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), &pb.PrimaryKeyProjectResource{
			Id:            request.GetId(),
			ProjectId:     request.GetProjectId(),
			EnvironmentId: request.GetEnvironmentId(),
		})
		if currentErr == nil && isTelegramProjectResource(current) {
			h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/telegram endpoints to manage Telegram Support")
			return
		}
		if currentErr == nil && isInstagramProjectResource(current) {
			h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/instagram endpoints to manage Instagram Support")
			return
		}
	}
	if shouldSanitizeGoogleDriveSettings(request) {
		current, err := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), &pb.PrimaryKeyProjectResource{
			Id:            request.GetId(),
			ProjectId:     request.GetProjectId(),
			EnvironmentId: request.GetEnvironmentId(),
		})
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}

		if current.GetType() == pb.ResourceType_GOOGLE_DRIVE.String() || request.GetType() == pb.ResourceType_GOOGLE_DRIVE.String() || request.GetResourceType() == int32(pb.ResourceType_GOOGLE_DRIVE) {
			var currentDrive *pb.GoogleDriveCredentials
			if current.GetSettings() != nil {
				currentDrive = current.GetSettings().GetGoogleDrive()
			}
			var requestDrive *pb.GoogleDriveCredentials
			if request.GetSettings() != nil {
				requestDrive = request.GetSettings().GetGoogleDrive()
			}

			folderID := ""
			if currentDrive != nil {
				folderID = currentDrive.GetFolderId()
			}
			if folderID == "" && requestDrive != nil {
				folderID = requestDrive.GetFolderId()
			}
			if strings.TrimSpace(folderID) == "" {
				h.HandleResponse(c, status_http.BadRequest, "google drive resource folder_id is empty")
				return
			}

			visibility := strings.TrimSpace(h.baseConf.GoogleDriveVisibility)
			if visibility == "" {
				visibility = "private"
			}
			if requestDrive != nil && strings.TrimSpace(requestDrive.GetVisibility()) != "" {
				visibility = requestDrive.GetVisibility()
			}

			refreshToken := ""
			authType := "oauth"
			if currentDrive != nil {
				refreshToken = currentDrive.GetRefreshToken()
				if strings.TrimSpace(currentDrive.GetAuthType()) != "" {
					authType = currentDrive.GetAuthType()
				}
			}
			if requestDrive != nil {
				if strings.TrimSpace(requestDrive.GetRefreshToken()) != "" {
					refreshToken = requestDrive.GetRefreshToken()
				}
				if strings.TrimSpace(requestDrive.GetAuthType()) != "" {
					authType = requestDrive.GetAuthType()
				}
			}

			request.Type = pb.ResourceType_GOOGLE_DRIVE.String()
			request.ResourceType = int32(pb.ResourceType_GOOGLE_DRIVE)
			request.Settings = &pb.Settings{
				GoogleDrive: &pb.GoogleDriveCredentials{
					AuthType:     authType,
					FolderId:     folderID,
					Visibility:   visibility,
					RefreshToken: refreshToken,
				},
			}
		}
	}
	if shouldSanitizeGoogleCalendarSettings(request) {
		if request.GetSettings() == nil || request.GetSettings().GetGoogleCalendar() == nil {
			h.HandleResponse(c, status_http.BadRequest, "google calendar settings are required")
			return
		}
		current, err := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), &pb.PrimaryKeyProjectResource{
			Id:            request.GetId(),
			ProjectId:     request.GetProjectId(),
			EnvironmentId: request.GetEnvironmentId(),
		})
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, err.Error())
			return
		}
		requestCalendar := request.GetSettings().GetGoogleCalendar()
		if current.GetSettings() != nil && current.GetSettings().GetGoogleCalendar() != nil {
			currentCalendar := current.GetSettings().GetGoogleCalendar()
			if strings.TrimSpace(requestCalendar.GetRefreshToken()) == "" {
				requestCalendar.RefreshToken = currentCalendar.GetRefreshToken()
			}
			if strings.TrimSpace(requestCalendar.GetCalendarId()) == "" {
				requestCalendar.CalendarId = currentCalendar.GetCalendarId()
			}
		}
		request.Type = pb.ResourceType_GOOGLE_CALENDAR.String()
		request.ResourceType = int32(pb.ResourceType_GOOGLE_CALENDAR)
		request.Settings = sanitizeGoogleCalendarSettingsForStorage(request.GetSettings())
	}

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "UPDATE",
			UserInfo:     cast.ToString(userId),
			Request:      projectResourceForLog(request),
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.HandleResponse(c, status_http.OK, resp)
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.companyServices.Resource().UpdateProjectResource(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, resp)
}

func shouldSanitizeGoogleDriveSettings(resource *pb.ProjectResource) bool {
	if resource == nil {
		return false
	}
	if resource.GetType() == pb.ResourceType_GOOGLE_DRIVE.String() || resource.GetResourceType() == int32(pb.ResourceType_GOOGLE_DRIVE) {
		return true
	}
	return resource.GetSettings() != nil && resource.GetSettings().GetGoogleDrive() != nil
}

func sanitizeGoogleDriveSettingsForStorage(settings *pb.Settings, defaultVisibility string) *pb.Settings {
	if settings == nil || settings.GetGoogleDrive() == nil {
		return &pb.Settings{}
	}

	visibility := strings.TrimSpace(defaultVisibility)
	if visibility == "" {
		visibility = "private"
	}
	if settings != nil && settings.GetGoogleDrive() != nil && strings.TrimSpace(settings.GetGoogleDrive().GetVisibility()) != "" {
		visibility = settings.GetGoogleDrive().GetVisibility()
	}

	drive := settings.GetGoogleDrive()
	authType := strings.TrimSpace(drive.GetAuthType())
	if authType == "" {
		authType = "oauth"
	}

	return &pb.Settings{
		GoogleDrive: &pb.GoogleDriveCredentials{
			AuthType:     authType,
			FolderId:     strings.TrimSpace(drive.GetFolderId()),
			Visibility:   visibility,
			RefreshToken: strings.TrimSpace(drive.GetRefreshToken()),
		},
	}
}

func sanitizeGoogleDriveResourceForResponse(resource *pb.ProjectResource) {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetGoogleDrive() == nil {
		return
	}
	resource.Secret = nil

	drive := resource.GetSettings().GetGoogleDrive()
	resource.Settings.GoogleDrive = &pb.GoogleDriveCredentials{
		AuthType:   drive.GetAuthType(),
		FolderId:   drive.GetFolderId(),
		Visibility: drive.GetVisibility(),
	}
}

func googleDriveProjectResourceForLog(resource *pb.ProjectResource) *pb.ProjectResource {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetGoogleDrive() == nil {
		return resource
	}

	safe := *resource
	safe.Secret = nil
	drive := resource.GetSettings().GetGoogleDrive()
	safe.Settings = &pb.Settings{
		GoogleDrive: &pb.GoogleDriveCredentials{
			AuthType:   drive.GetAuthType(),
			FolderId:   drive.GetFolderId(),
			Visibility: drive.GetVisibility(),
		},
	}
	return &safe
}

func shouldSanitizeGoogleCalendarSettings(resource *pb.ProjectResource) bool {
	if resource == nil {
		return false
	}
	if resource.GetType() == pb.ResourceType_GOOGLE_CALENDAR.String() || resource.GetResourceType() == int32(pb.ResourceType_GOOGLE_CALENDAR) {
		return true
	}
	return resource.GetSettings() != nil && resource.GetSettings().GetGoogleCalendar() != nil
}

func sanitizeGoogleCalendarSettingsForStorage(settings *pb.Settings) *pb.Settings {
	if settings == nil || settings.GetGoogleCalendar() == nil {
		return &pb.Settings{}
	}
	calendarSettings := settings.GetGoogleCalendar()
	authType := strings.TrimSpace(calendarSettings.GetAuthType())
	if authType == "" {
		authType = "oauth"
	}
	calendarID := strings.TrimSpace(calendarSettings.GetCalendarId())
	if calendarID == "" {
		calendarID = defaultGoogleCalendarID
	}
	syncDirection := strings.TrimSpace(calendarSettings.GetSyncDirection())
	if syncDirection == "" {
		syncDirection = defaultGoogleCalendarSyncDirection
	}
	return &pb.Settings{
		GoogleCalendar: &pb.GoogleCalendarCredentials{
			AuthType:      authType,
			CalendarId:    calendarID,
			RefreshToken:  strings.TrimSpace(calendarSettings.GetRefreshToken()),
			SyncDirection: syncDirection,
			Mapping:       calendarSettings.GetMapping(),
		},
	}
}

func sanitizeGoogleCalendarResourceForResponse(resource *pb.ProjectResource) {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetGoogleCalendar() == nil {
		return
	}
	resource.Secret = nil
	calendarSettings := resource.GetSettings().GetGoogleCalendar()
	resource.Settings.GoogleCalendar = &pb.GoogleCalendarCredentials{
		AuthType:      calendarSettings.GetAuthType(),
		CalendarId:    calendarSettings.GetCalendarId(),
		SyncDirection: calendarSettings.GetSyncDirection(),
		Mapping:       calendarSettings.GetMapping(),
	}
}

func googleCalendarProjectResourceForLog(resource *pb.ProjectResource) *pb.ProjectResource {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetGoogleCalendar() == nil {
		return resource
	}
	safe := *resource
	safe.Secret = nil
	calendarSettings := resource.GetSettings().GetGoogleCalendar()
	safe.Settings = &pb.Settings{
		GoogleCalendar: &pb.GoogleCalendarCredentials{
			AuthType:      calendarSettings.GetAuthType(),
			CalendarId:    calendarSettings.GetCalendarId(),
			SyncDirection: calendarSettings.GetSyncDirection(),
			Mapping:       calendarSettings.GetMapping(),
		},
	}
	return &safe
}

func isTelegramAddResourceRequest(resource *pb.AddResourceToProjectRequest) bool {
	if resource == nil {
		return false
	}
	if resource.GetType() == pb.ResourceType_TELEGRAM {
		return true
	}
	return resource.GetSettings() != nil && resource.GetSettings().GetTelegram() != nil
}

func isTelegramProjectResource(resource *pb.ProjectResource) bool {
	if resource == nil {
		return false
	}
	if resource.GetType() == pb.ResourceType_TELEGRAM.String() || resource.GetResourceType() == int32(pb.ResourceType_TELEGRAM) {
		return true
	}
	return resource.GetSettings() != nil && resource.GetSettings().GetTelegram() != nil
}

func sanitizeTelegramResourceForResponse(resource *pb.ProjectResource) {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetTelegram() == nil {
		return
	}
	resource.Secret = nil
	telegramSettings := resource.GetSettings().GetTelegram()
	resource.Settings.Telegram = &pb.TelegramCredentials{
		BotId:       telegramSettings.GetBotId(),
		BotUsername: telegramSettings.GetBotUsername(),
		Status:      telegramSettings.GetStatus(),
		Mapping:     telegramSettings.GetMapping(),
	}
}

func isInstagramAddResourceRequest(resource *pb.AddResourceToProjectRequest) bool {
	if resource == nil {
		return false
	}
	if resource.GetType() == pb.ResourceType_INSTAGRAM {
		return true
	}
	return resource.GetSettings() != nil && resource.GetSettings().GetInstagram() != nil
}

func isInstagramProjectResource(resource *pb.ProjectResource) bool {
	if resource == nil {
		return false
	}
	if resource.GetType() == pb.ResourceType_INSTAGRAM.String() || resource.GetResourceType() == int32(pb.ResourceType_INSTAGRAM) {
		return true
	}
	return resource.GetSettings() != nil && resource.GetSettings().GetInstagram() != nil
}

func sanitizeInstagramResourceForResponse(resource *pb.ProjectResource) {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetInstagram() == nil {
		return
	}
	resource.Secret = nil
	instagramSettings := resource.GetSettings().GetInstagram()
	resource.Settings.Instagram = &pb.InstagramCredentials{
		IgId:              instagramSettings.GetIgId(),
		Username:          instagramSettings.GetUsername(),
		AccountType:       instagramSettings.GetAccountType(),
		ProfilePictureUrl: instagramSettings.GetProfilePictureUrl(),
		Status:            instagramSettings.GetStatus(),
		ConnectedUserId:   instagramSettings.GetConnectedUserId(),
		ConnectedAt:       instagramSettings.GetConnectedAt(),
		Mapping:           instagramSettings.GetMapping(),
	}
}

func instagramProjectResourceForLog(resource *pb.ProjectResource) *pb.ProjectResource {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetInstagram() == nil {
		return resource
	}
	safe := *resource
	safe.Secret = nil
	instagramSettings := resource.GetSettings().GetInstagram()
	safe.Settings = &pb.Settings{
		Instagram: &pb.InstagramCredentials{
			IgId:              instagramSettings.GetIgId(),
			Username:          instagramSettings.GetUsername(),
			AccountType:       instagramSettings.GetAccountType(),
			ProfilePictureUrl: instagramSettings.GetProfilePictureUrl(),
			Status:            instagramSettings.GetStatus(),
			ConnectedUserId:   instagramSettings.GetConnectedUserId(),
			ConnectedAt:       instagramSettings.GetConnectedAt(),
			Mapping:           instagramSettings.GetMapping(),
		},
	}
	return &safe
}

func telegramProjectResourceForLog(resource *pb.ProjectResource) *pb.ProjectResource {
	if resource == nil || resource.GetSettings() == nil || resource.GetSettings().GetTelegram() == nil {
		return resource
	}
	safe := *resource
	safe.Secret = nil
	telegramSettings := resource.GetSettings().GetTelegram()
	safe.Settings = &pb.Settings{
		Telegram: &pb.TelegramCredentials{
			BotId:       telegramSettings.GetBotId(),
			BotUsername: telegramSettings.GetBotUsername(),
			Status:      telegramSettings.GetStatus(),
			Mapping:     telegramSettings.GetMapping(),
		},
	}
	return &safe
}

func projectResourceForLog(resource *pb.ProjectResource) *pb.ProjectResource {
	if resource == nil {
		return resource
	}
	safe := *resource
	safe.Secret = nil
	resource = &safe
	if resource.GetSettings() != nil && resource.GetSettings().GetTelegram() != nil {
		return telegramProjectResourceForLog(resource)
	}
	if resource.GetSettings() != nil && resource.GetSettings().GetInstagram() != nil {
		return instagramProjectResourceForLog(resource)
	}
	if resource.GetSettings() != nil && resource.GetSettings().GetGoogleCalendar() != nil {
		return googleCalendarProjectResourceForLog(resource)
	}
	return googleDriveProjectResourceForLog(resource)
}

// GetListProjectResource godoc
// @Security ApiKeyAuth
// @ID get_list_project_resource
// @Router /v2/company/project/resource [GET]
// @Summary Get list project resource
// @Description Get list project resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param type query string false "type"
// @Success 200 {object} status_http.Response{data=pb.ListProjectResource} "Company data"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetListProjectResourceList(c *gin.Context) {
	var request = &pb.GetProjectResourceListRequest{}

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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)

	if c.DefaultQuery("type", "") != "" && pb.ResourceType(pb.ResourceType_value[c.DefaultQuery("type", "")]) != 0 {
		request.Type = pb.ResourceType(pb.ResourceType_value[c.DefaultQuery("type", "")])
	}

	resp, err := h.companyServices.Resource().GetProjectResourceList(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for _, resource := range resp.GetResources() {
		resource.Secret = nil
		sanitizeGoogleDriveResourceForResponse(resource)
		sanitizeGoogleCalendarResourceForResponse(resource)
		sanitizeTelegramResourceForResponse(resource)
		sanitizeInstagramResourceForResponse(resource)
	}

	h.HandleResponse(c, status_http.OK, resp)
}

// GetProjectResourceByID godoc
// @Security ApiKeyAuth
// @ID get_single_project_resource
// @Router /v2/company/project/resource/{id} [GET]
// @Summary Get single variable resource
// @Description Get single variable resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 200 {object} status_http.Response{data=pb.ProjectResource} "ProjectResource"
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) GetSingleProjectResource(c *gin.Context) {
	request := &pb.PrimaryKeyProjectResource{}

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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Id = c.Param("id")

	resp, err := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	resp.Secret = nil
	sanitizeGoogleDriveResourceForResponse(resp)
	sanitizeGoogleCalendarResourceForResponse(resp)
	sanitizeTelegramResourceForResponse(resp)
	sanitizeInstagramResourceForResponse(resp)

	h.HandleResponse(c, status_http.OK, resp)
}

// DeletevariableResource godoc
// @Security ApiKeyAuth
// @ID delete_project_resource
// @Router /v2/company/project/resource/{id} [DELETE]
// @Summary Delete variable resource
// @Description Delete variable resource
// @Tags Project resource
// @Accept json
// @Produce json
// @Param id path string true "id"
// @Success 204
// @Response 400 {object} status_http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status_http.Response{data=string} "Server Error"
func (h *HandlerV1) DeleteProjectResource(c *gin.Context) {
	var (
		request = &pb.PrimaryKeyProjectResource{}
		resp    = &pb.Empty{}
	)

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

	request.ProjectId = projectId.(string)
	request.EnvironmentId = environmentId.(string)
	request.Id = c.Param("id")

	current, currentErr := h.companyServices.Resource().GetSingleProjectResouece(c.Request.Context(), request)
	if currentErr == nil && isTelegramProjectResource(current) {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/telegram endpoints to disconnect Telegram Support")
		return
	}
	if currentErr == nil && isInstagramProjectResource(current) {
		h.HandleResponse(c, status_http.BadRequest, "use /v1/mcp_project/:id/instagram endpoints to disconnect Instagram Support")
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

	var (
		logReq = &models.CreateVersionHistoryRequest{
			Services:     services,
			NodeType:     resource.NodeType,
			ProjectId:    resource.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "DELETE",
			UserInfo:     cast.ToString(userId),
		}
	)

	defer func() {
		if err != nil {
			logReq.Response = err.Error()
			h.HandleResponse(c, status_http.GRPCError, err.Error())
		} else {
			h.HandleResponse(c, status_http.NoContent, resp)
		}
		go h.versionHistory(logReq)
	}()

	resp, err = h.companyServices.Resource().DeleteProjectResource(c.Request.Context(), request)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.NoContent, nil)
}

package handlers

import (
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"

	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/gin-gonic/gin"
)

// Login godoc
// @ID login
// @Router /login [POST]
// @Summary Login
// @Description Login
// @Tags Session
// @Accept json
// @Produce json
// @Param login body auth_service.LoginRequest true "LoginRequestBody"
// @Success 201 {object} http.Response{data=models.LoginResponse} "Login data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) Login(c *gin.Context) {
	var login auth_service.LoginRequest

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().Login(
		c.Request.Context(), &login,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// Logout godoc
// @ID logout
// @Router /logout [DELETE]
// @Summary Logout User
// @Description Logout User
// @Tags Session
// @Accept json
// @Produce json
// @Param data body auth_service.LogoutRequest true "LogoutRequest"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) Logout(c *gin.Context) {
	var logout auth_service.LogoutRequest

	err := c.ShouldBindJSON(&logout)
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().Logout(
		c.Request.Context(),
		&logout,
	)

	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

// RefreshToken godoc
// @ID refresh
// @Router /refresh [PUT]
// @Summary Refresh Token
// @Description Refresh Token
// @Tags Session
// @Accept json
// @Produce json
// @Param user body auth_service.RefreshTokenRequest true "RefreshTokenRequestBody"
// @Success 200 {object} http.Response{data=auth_service.RefreshTokenResponse} "Refresh token data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) RefreshToken(c *gin.Context) {
	var user auth_service.RefreshTokenRequest

	if err := c.ShouldBindJSON(&user); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().RefreshToken(
		c.Request.Context(), &user,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// HasAccess godoc
// @ID has_access
// @Router /has-access [POST]
// @Summary Has Access
// @Description Has Access
// @Tags Session
// @Accept json
// @Produce json
// @Param has-access body auth_service.HasAccessRequest true "HasAccessRequestBody"
// @Success 201 {object} http.Response{data=auth_service.HasAccessResponse} "User access data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) HasAccess(c *gin.Context) {
	var login auth_service.HasAccessRequest

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().HasAccess(
		c.Request.Context(), &login,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// HasAccessSuperAdmin godoc
// @ID has_access_superadmin
// @Router /has-access-super-admin [POST]
// @Summary Has Access
// @Description Has Access
// @Tags Session
// @Accept json
// @Produce json
// @Param has-access body auth_service.HasAccessSuperAdminReq true "HasAccessRequestBody"
// @Success 201 {object} http.Response{data=auth_service.HasAccessSuperAdminRes} "Admin access data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) HasAccessSuperAdmin(c *gin.Context) {
	var login auth_service.HasAccessSuperAdminReq

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().HasAccessSuperAdmin(
		c.Request.Context(), &login,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// GetSessionList godoc
// @Security ApiKeyAuth
// @ID v2_get_session_list
// @Router /v2/session [GET]
// @Summary Get session list
// @Description Get paginated sessions for the current project, optionally filtered by user, client type, or search text.
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token or API-KEY"
// @Param X-API-KEY header string false "API key when Authorization is API-KEY"
// @Param project-id query string false "Project id"
// @Param limit query int false "Limit"
// @Param offset query int false "Offset"
// @Param search query string false "Search"
// @Param user_id query string false "User id"
// @Param client_type_id query string false "Client type id"
// @Success 200 {object} http.Response{data=auth_service.GetSessionListResponse} "Session list"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetSessionList(c *gin.Context) {
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

	// Session id of the caller's own token, set by AuthMiddleware on the
	// Bearer path; absent for API-KEY auth, so is_current stays false.
	currentSessionId := c.GetString("session_id")

	resp, err := h.services.SessionService().GetList(
		c.Request.Context(),
		&auth_service.GetSessionListRequest{
			Limit:            int32(limit),
			Offset:           int32(offset),
			Search:           c.Query("search"),
			UserId:           c.Query("user_id"),
			ClientTypeId:     c.Query("client_type_id"),
			CurrentSessionId: currentSessionId,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// DeleteSession godoc
// @Security ApiKeyAuth
// @ID v2_delete_session
// @Router /v2/session/{id} [DELETE]
// @Summary Delete session
// @Description Delete one session by id.
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token or API-KEY"
// @Param X-API-KEY header string false "API key when Authorization is API-KEY"
// @Param id path string true "Session id"
// @Success 200 {object} http.Response{data=auth_service.SessionPrimaryKey} "Deleted session"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteSession(c *gin.Context) {
	resp, err := h.services.SessionService().Delete(
		c.Request.Context(),
		&auth_service.SessionPrimaryKey{
			Id: c.Param("id"),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// GetSessionDevices godoc
// @Security ApiKeyAuth
// @ID v2_get_session_devices
// @Router /v2/session/devices [GET]
// @Summary Get session devices
// @Description Get devices with active sessions for a user in the current project.
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token or API-KEY"
// @Param X-API-KEY header string false "API key when Authorization is API-KEY"
// @Param project-id query string false "Project id"
// @Param user_id query string false "User id. If omitted, token user is used."
// @Success 200 {object} http.Response{data=auth_service.GetSessionDevicesResponse} "Session devices"
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) GetSessionDevices(c *gin.Context) {
	userIdAuth, err := h.getFromHeaderOrQuery(c, "user_id")
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, "project_id is required")
		return
	}

	resp, err := h.services.SessionService().GetSessionDevices(
		c.Request.Context(),
		&auth_service.GetSessionDevicesRequest{
			UserIdAuth: userIdAuth,
			ProjectId:  projectId.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// DeleteSessionsByDevice godoc
// @Security ApiKeyAuth
// @ID v2_delete_sessions_by_device
// @Router /v2/session/by-device [DELETE]
// @Summary Delete sessions by device
// @Description Delete sessions for the current user/project by device information.
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token or API-KEY"
// @Param X-API-KEY header string false "API key when Authorization is API-KEY"
// @Param project-id query string false "Project id"
// @Param user_id query string false "User id. If omitted, token user is used."
// @Param request body auth_service.DeleteSessionsByDeviceRequest true "Delete sessions by device request"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteSessionsByDevice(c *gin.Context) {
	var req auth_service.DeleteSessionsByDeviceRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	userIdAuth, err := h.getFromHeaderOrQuery(c, "user_id")
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, "project_id is required")
		return
	}

	req.ProjectId = projectId.(string)
	req.UserIdAuth = userIdAuth

	resp, err := h.services.SessionService().DeleteSessionsByDevice(
		c.Request.Context(),
		&req,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

// DeleteSessionsExceptCurrent godoc
// @Security ApiKeyAuth
// @ID v2_delete_sessions_except_current
// @Router /v2/session/except-current [DELETE]
// @Summary Delete sessions except current
// @Description Delete all sessions for the current user/project except the current session.
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token or API-KEY"
// @Param X-API-KEY header string false "API key when Authorization is API-KEY"
// @Param project-id query string false "Project id"
// @Param user_id query string false "User id. If omitted, token user is used."
// @Param session_id query string false "Current session id. If omitted, token session is used."
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteSessionsExceptCurrent(c *gin.Context) {
	userIdAuth, err := h.getFromHeaderOrQuery(c, "user_id")
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	sessionId, err := h.getFromHeaderOrQuery(c, "session_id")
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, "project_id is required")
		return
	}

	var req = auth_service.DeleteSessionsExceptCurrentRequest{
		UserIdAuth: userIdAuth,
		ProjectId:  projectId.(string),
		SessionId:  sessionId,
	}

	resp, err := h.services.SessionService().DeleteSessionsExceptCurrent(
		c.Request.Context(),
		&req,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.NoContent, resp)
}

func (h *Handler) getFromHeaderOrQuery(c *gin.Context, key string) (string, error) {
	value := c.Query(key)
	if value == "" {
		id, ok := c.Get(key)
		if !ok {
			return "", errors.New("user_id not found")
		}

		value = id.(string)
	}

	return value, nil
}

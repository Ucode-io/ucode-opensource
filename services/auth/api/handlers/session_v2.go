package handlers

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pba "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	pbCompany "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	obs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/spf13/cast"
)

func (h *Handler) V3MultiCompanyLogin(c *gin.Context) {
	var (
		login  pba.UserDefaultProjectReq
		limit  = 100
		offset = 0

		ctx = c.Request.Context()
	)

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if login.Type == "" {
		login.Type = config.Default
	}

	switch login.Type {
	case config.Default:
		if login.Username == "" || login.Password == "" {
			h.handleResponse(c, http.BadRequest, "username and password are required")
			return
		}
	case config.WithPhone:
		if login.Phone == "" || login.Otp == "" {
			h.handleResponse(c, http.BadRequest, "phone and otp are required")
			return
		}
		if login.ServiceType == "firebase" && login.SessionInfo == "" {
			h.handleResponse(c, http.BadRequest, "session info is required for firebase")
			return
		} else if login.ServiceType != "firebase" && login.SmsId == "" {
			h.handleResponse(c, http.BadRequest, "SmsId is required")
			return
		}
	case config.WithEmail:
		if login.Email == "" || login.Otp == "" || login.SmsId == "" {
			h.handleResponse(c, http.BadRequest, "email, otp, and SmsId are required")
			return
		}
	case config.WithGoogle:
		if login.GoogleToken == "" {
			h.handleResponse(c, http.BadRequest, "google token is required")
			return
		}
	}

	userProject, err := h.services.SessionService().UserDefaultProject(ctx, &login)
	if err != nil {
		h.handleError(c, http.BadRequest, err)
		return
	}

	resource, err := h.services.ServiceResource().GetSingle(ctx, &pbCompany.GetSingleServiceResourceReq{
		ProjectId:     userProject.GetProjectId(),
		EnvironmentId: userProject.GetEnvironmentId(),
		ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, userProject.GetProjectId(), resource.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	var rawResponse []any

	// FIXED TEMPORARY
	reconnectIfPoolMissing := func(err error) bool {
		if err == nil || !strings.Contains(err.Error(), "connection not found") {
			return false
		}
		_, recErr := h.services.ResourceService().ReconnectResource(ctx, &pbCompany.ReconnectResourceRequest{
			Id:        resource.GetResourceId(),
			ProjectId: resource.GetProjectId(),
		})
		if recErr != nil {
			h.log.Error("V3MultiCompanyLogin: ReconnectResource failed: " + recErr.Error())
			return false
		}
		return true
	}

	switch resource.ResourceType {
	case 1:
		structData, _ := helper.ConvertMapToStruct(map[string]any{
			"limit":          limit,
			"offset":         offset,
			"client_type_id": userProject.GetClientTypeId(),
		})

		req := &obs.CommonMessage{
			TableSlug: "connections",
			ProjectId: resource.ResourceEnvironmentId,
			Data:      structData,
		}
		resp, err := services.GetObjectBuilderServiceByType(resource.NodeType).GetList(ctx, req)
		if err != nil && reconnectIfPoolMissing(err) {
			resp, err = services.GetObjectBuilderServiceByType(resource.NodeType).GetList(ctx, req)
		}
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		rawResponse, _ = resp.Data.AsMap()["response"].([]any)

	case 3:
		structData, _ := helper.ConvertMapToStruct(map[string]any{
			"limit":                     limit,
			"offset":                    offset,
			"client_type_id_from_token": userProject.GetClientTypeId(),
		})

		req := &nobs.CommonMessage{
			TableSlug: "connections",
			ProjectId: resource.ResourceEnvironmentId,
			Data:      structData,
		}
		resp, err := services.GoObjectBuilderService().GetList(ctx, req)
		if err != nil && reconnectIfPoolMissing(err) {
			resp, err = services.GoObjectBuilderService().GetList(ctx, req)
		}
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		rawResponse, _ = resp.Data.AsMap()["response"].([]any)
	}

	var (
		finalConnections = make([]map[string]any, len(rawResponse))
		wg               sync.WaitGroup
	)

	for i, v := range rawResponse {
		connMap, ok := v.(map[string]any)
		if !ok {
			continue
		}

		finalConnections[i] = connMap
		guid, ok := connMap["guid"].(string)
		if !ok {
			continue
		}

		wg.Add(1)
		go func(index int, connectionId string, cMap map[string]any) {
			defer wg.Done()
			var optionsData any

			if resource.ResourceType == pbCompany.ResourceType_MONGODB {
				optResp, err := services.GetLoginServiceByType(resource.NodeType).GetConnetionOptions(ctx, &obs.GetConnetionOptionsRequest{
					ConnectionId:          connectionId,
					ResourceEnvironmentId: resource.ResourceEnvironmentId,
					UserId:                userProject.GetUserId(),
				})
				if err == nil && optResp.Data != nil {
					optionsData = optResp.Data.AsMap()["response"]
				}
			} else if resource.ResourceType == pbCompany.ResourceType_POSTGRESQL {
				optResp, err := services.GoLoginService().GetConnetionOptions(ctx, &nobs.GetConnetionOptionsRequest{
					ConnectionId:          connectionId,
					ResourceEnvironmentId: resource.ResourceEnvironmentId,
					UserId:                userProject.GetUserId(),
					ProjectId:             resource.ProjectId,
				})

				if err == nil && optResp.Data != nil {
					optionsData = optResp.Data.AsMap()["response"]
				}
			}

			if optionsData != nil {
				cMap["options"] = optionsData
			} else {
				cMap["options"] = make([]any, 0)
			}
			finalConnections[index] = cMap
		}(i, guid, connMap)
	}
	wg.Wait()

	var (
		shouldAutoLogin = false
		tables          []*pba.Object
	)

	if len(finalConnections) == 0 {
		shouldAutoLogin = true

	}

	if len(finalConnections) == 1 {

		conn := finalConnections[0]
		options, ok := conn["options"].([]any)

		if !ok || len(options) == 0 {
			shouldAutoLogin = true

		} else if len(options) == 1 {

			shouldAutoLogin = true
			opt, okOpt := options[0].(map[string]any)
			tableSlug, okSlug := conn["table_slug"].(string)

			if okOpt && okSlug {
				if objectId, okObj := opt["guid"].(string); okObj {
					tables = []*pba.Object{
						{
							TableSlug: tableSlug,
							ObjectId:  objectId,
						},
					}
				}
			}
		}
	}

	if shouldAutoLogin {
		v2Req := &pba.V2LoginRequest{
			Type:                  login.GetType(),
			Username:              login.GetUsername(),
			Password:              login.GetPassword(),
			Phone:                 login.GetPhone(),
			Email:                 login.GetEmail(),
			Otp:                   login.GetOtp(),
			SmsId:                 login.GetSmsId(),
			SessionInfo:           login.GetSessionInfo(),
			ServiceType:           login.GetServiceType(),
			GoogleToken:           login.GetGoogleToken(),
			ProjectId:             userProject.GetProjectId(),
			EnvironmentId:         userProject.GetEnvironmentId(),
			ClientType:            userProject.GetClientTypeId(),
			ResourceEnvironmentId: resource.GetResourceEnvironmentId(),
			NodeType:              resource.GetNodeType(),
			ResourceType:          int32(resource.GetResourceType()),
			Tables:                tables,
			ClientIp:              c.RemoteIP(),
			UserAgent:             c.Request.UserAgent(),
		}

		v2Resp, err := h.services.SessionService().V2Login(ctx, v2Req)
		if err != nil {
			var httpErrorStr = ""

			httpErrorStr = strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
			httpErrorStr = strings.ToLower(httpErrorStr)

			switch httpErrorStr {
			case "user not found":
				h.handleResponse(c, http.NotFound, "Пользователь не найдено")
				return
			case "session has been expired":
				h.handleResponse(c, http.InvalidArgument, "срок действия пользователя истек")
				return
			case "invalid username":
				h.handleResponse(c, http.InvalidArgument, "неверное имя пользователя")
				return
			case "invalid password":
				h.handleResponse(c, http.InvalidArgument, "неверное пароль")
				return
			case "user blocked":
				h.handleResponse(c, http.Forbidden, "Пользователь заблокирован")
				return
			default:
				h.handleResponse(c, http.InvalidArgument, err.Error())
				return
			}
		}

		v2Resp.EnvironmentId = resource.GetEnvironmentId()
		v2Resp.ResourceId = resource.GetResourceId()

		h.handleResponse(c, http.OK, map[string]any{
			"response":     v2Resp,
			"project_data": userProject.GetProjectData(),
		})
		return
	}

	h.handleResponse(c, http.OK, map[string]any{
		"response":     finalConnections,
		"client_type":  userProject.GetClientTypeId(),
		"environment":  userProject.GetEnvironmentId(),
		"project":      userProject.GetProjectId(),
		"user_id":      userProject.GetUserId(),
		"project_data": userProject.GetProjectData(),
	})
}

// @Security ApiKeyAuth
// V2Login godoc
// @ID V2login
// @Param Resource-Id header string true "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Router /v2/login [POST]
// @Summary V2Login
// @Description V2Login
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body auth_service.V2LoginRequest true "LoginRequestBody"
// @Success 201 {object} http.Response{data=models.V2LoginResponse} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2Login(c *gin.Context) {
	var (
		login pba.V2LoginRequest
		resp  *pba.V2LoginResponse
	)

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if login.ClientType == "" {
		h.handleResponse(c, http.BadRequest, "Необходимо выбрать тип пользователя")
		return
	}

	if login.ProjectId == "" {
		h.handleResponse(c, http.BadRequest, "Необходимо выбрать проекта")
		return
	}

	if login.Type == "" {
		login.Type = config.Default
	}

	switch login.Type {
	case config.Default:
		if login.Username == "" {
			h.handleResponse(c, http.BadRequest, "username is required")
			return
		}

		if login.Password == "" {
			h.handleResponse(c, http.BadRequest, "password is required")
			return
		}
	case config.WithPhone:
		if login.ServiceType != "firebase" {
			if login.SmsId == "" {
				h.handleResponse(c, http.BadRequest, "SmsId is required when type is not default")
				return
			}
		}

		if login.Otp == "" {
			h.handleResponse(c, http.BadRequest, "otp is required when type is not default")
			return
		}

		if login.Phone == "" {
			h.handleResponse(c, http.BadRequest, "phone is required when type is phone")
			return
		}
	case config.WithEmail:
		if login.SmsId == "" {
			h.handleResponse(c, http.BadRequest, "SmsId is required when type is not default")
			return
		}

		if login.Otp == "" {
			h.handleResponse(c, http.BadRequest, "otp is required when type is not default")
			return
		}

		if login.Email == "" {
			h.handleResponse(c, http.BadRequest, "email is required when type is email")
			return
		}
	case config.WithGoogle:
		if login.GoogleToken == "" {
			h.handleResponse(c, http.BadRequest, "google token is required when type is not default")
			return
		}
	}

	environmentId, ok := c.Get("environment_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	resourceEnvironment, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pb.GetSingleServiceResourceReq{
			EnvironmentId: environmentId.(string),
			ProjectId:     login.GetProjectId(),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	login.ResourceEnvironmentId = resourceEnvironment.GetResourceEnvironmentId()
	login.ResourceType = int32(resourceEnvironment.GetResourceType())
	login.EnvironmentId = resourceEnvironment.GetEnvironmentId()
	login.NodeType = resourceEnvironment.GetNodeType()
	login.ClientIp = c.RemoteIP()
	login.UserAgent = c.Request.UserAgent()

	var (
		logReq = &models.CreateVersionHistoryRequest{
			NodeType:     resourceEnvironment.NodeType,
			ProjectId:    resourceEnvironment.ResourceEnvironmentId,
			ActionSource: c.Request.URL.String(),
			ActionType:   "LOGIN",
			UserInfo:     cast.ToString(login.Username),
			Request:      &login,
			TableSlug:    "User",
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

	resp, err = h.services.SessionService().V2Login(
		c.Request.Context(), &login,
	)
	if err != nil {
		var httpErrorStr = ""

		httpErrorStr = strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
		httpErrorStr = strings.ToLower(httpErrorStr)

		switch httpErrorStr {
		case "user not found":
			h.handleResponse(c, http.NotFound, "Пользователь не найдено")
			return
		case "session has been expired":
			h.handleResponse(c, http.InvalidArgument, "срок действия пользователя истек")
			return
		case "invalid username":
			h.handleResponse(c, http.InvalidArgument, "неверное имя пользователя")
			return
		case "invalid password":
			h.handleResponse(c, http.InvalidArgument, "неверное пароль")
			return
		case "user blocked":
			h.handleResponse(c, http.Forbidden, "Пользователь заблокирован")
			return
		default:
			h.handleResponse(c, http.InvalidArgument, err.Error())
			return
		}
	}

	resp.EnvironmentId = resourceEnvironment.GetEnvironmentId()
	resp.ResourceId = resourceEnvironment.GetResourceId()

	h.handleResponse(c, http.Created, resp)
}

// V2RefreshToken godoc
// @ID v2refresh
// @Router /v2/refresh [PUT]
// @Summary V2Refresh Token
// @Description V2Refresh Token
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param for_env query string false "for_env"
// @Param user body auth_service.RefreshTokenRequest true "RefreshTokenRequestBody"
// @Success 200 {object} http.Response{data=models.V2LoginResponse} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2RefreshToken(c *gin.Context) {
	var (
		user    pba.RefreshTokenRequest
		resp    *pba.V2LoginResponse
		for_env = c.DefaultQuery("for_env", "")
		err     error
	)

	if err = c.ShouldBindJSON(&user); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if for_env == "true" {
		resp, err = h.services.SessionService().V2RefreshTokenForEnv(
			c.Request.Context(),
			&user,
		)
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	} else {
		resp, err = h.services.SessionService().V2RefreshToken(
			c.Request.Context(),
			&user,
		)
		if err != nil {
			h.handleError(c, http.GRPCError, err)
			return
		}
	}

	h.handleResponse(c, http.OK, resp)
}

// V2RefreshTokenSuperAdmin godoc
// @ID v2refresh_superadmin
// @Router /v2/refresh-superadmin [PUT]
// @Summary V2Refresh Token
// @Description V2Refresh Token
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param user body auth_service.RefreshTokenRequest true "RefreshTokenRequestBody"
// @Success 200 {object} http.Response{data=auth_service.V2RefreshTokenSuperAdminResponse} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2RefreshTokenSuperAdmin(c *gin.Context) {
	var user pba.RefreshTokenRequest

	if err := c.ShouldBindJSON(&user); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().V2RefreshTokenSuperAdmin(
		c.Request.Context(), &user,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, resp)
}

// V2LoginWithOption godoc
// @ID V2login_withoption
// @Router /v2/login/with-option [POST]
// @Summary V2LoginWithOption
// @Description V2LoginWithOption
// @Description inside the data you must be passed client_type_id field
// @Description you must be give environment_id and project_id in body or
// @Description Environment-Id hearder and project-id in query parameters or
// @Description X-API-KEY in hearder
// @Description login strategy must be one of the following values
// @Description ["EMAIL", "PHONE", "EMAIL_OTP", "PHONE_OTP", "LOGIN", "LOGIN_PWD", "GOOGLE_AUTH", "APPLE_AUTH", "PHONE_PWD", "EMAIL_PWD"]
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Environment-Id header string false "Environment-Id"
// @Param X-API-KEY header string false "X-API-KEY"
// @Param project-id query string false "project-id"
// @Param login body auth_service.V2LoginWithOptionRequest true "V2LoginRequest"
// @Success 201 {object} http.Response{data=models.V2LoginWithOptionsResponse} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2LoginWithOption(c *gin.Context) {
	var login pba.V2LoginWithOptionRequest

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	clientType := login.Data["client_type_id"]
	if clientType == "" {
		h.handleResponse(c, http.InvalidArgument, "inside data client_type_id is required")
		return
	}

	if ok := util.IsValidUUID(clientType); !ok {
		h.handleResponse(c, http.InvalidArgument, "client_type_id is an invalid uuid")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, http.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "error getting environment id | not valid")
		return
	}

	login.Data["environment_id"] = environmentId.(string)
	login.Data["project_id"] = projectId.(string)

	resp, err := h.services.SessionService().V2LoginWithOption(
		c.Request.Context(), &pba.V2LoginWithOptionRequest{
			Data:          login.GetData(),
			LoginStrategy: login.GetLoginStrategy(),
			Tables:        login.GetTables(),
			ClientIp:      c.ClientIP(),
			UserAgent:     c.Request.UserAgent(),
		})

	if err != nil {
		var httpErrorStr = ""

		httpErrorStr = strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
		httpErrorStr = strings.ToLower(httpErrorStr)

		switch httpErrorStr {
		case "user not found":
			h.handleResponse(c, http.NotFound, "Пользователь не найдено")
			return
		case "user verified but not found":
			h.handleResponse(c, http.OK, "Пользователь проверен, но не найден")
			return
		case "session has been expired":
			h.handleResponse(c, http.InvalidArgument, "срок действия пользователя истек")
			return
		case "invalid username":
			h.handleResponse(c, http.InvalidArgument, "срок действия пользователя истек")
			return
		case "invalid password":
			h.handleResponse(c, http.InvalidArgument, "неверное пароль")
			return
		case "user blocked":
			h.handleResponse(c, http.Forbidden, "Пользователь заблокирован")
			return
		default:
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
	}

	h.handleResponse(c, http.Created, &pba.V2LoginSuperAdminRes{
		UserFound: resp.GetUserFound(),
		Token:     resp.GetToken(),
		Companies: resp.GetCompanies(),
		UserId:    resp.GetUserId(),
		Sessions:  resp.GetSessions(),
		UserData:  resp.GetUserData(),
	})
}

// MultiCompanyLogin godoc
// @ID multi_company_login
// @Router /v2/multi-company/login [POST]
// @Summary MultiCompanyLogin
// @Description MultiCompanyLogin
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body auth_service.MultiCompanyLoginRequest true "LoginRequestBody"
// @Success 201 {object} http.Response{data=models.MultiCompanyLoginResponse} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) MultiCompanyLogin(c *gin.Context) {
	var login pba.MultiCompanyLoginRequest

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().MultiCompanyLogin(
		c.Request.Context(), &login,
	)
	if err != nil {
		httpErrorStr := strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
		httpErrorStr = strings.ToLower(httpErrorStr)

		switch httpErrorStr {
		case "user not found":
			h.handleResponse(c, http.NotFound, "Пользователь не найдено")
			return
		case "session has been expired":
			h.handleResponse(c, http.InvalidArgument, "срок действия пользователя истек")
			return
		case "invalid username":
			h.handleResponse(c, http.InvalidArgument, "неверное имя пользователя")
			return
		case "invalid password":
			h.handleResponse(c, http.InvalidArgument, "неверное пароль")
			return
		}
	}

	h.handleResponse(c, http.Created, resp)
}

// V2MultiCompanyLogin godoc
// @ID multi_company_login_v2
// @Router /v2/v2multi-company/login [POST]
// @Summary MultiCompanyLogin
// @Description MultiCompanyLogin
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body auth_service.V2MultiCompanyLoginReq true "LoginRequestBody"
// @Success 201 {object} http.Response{data=models.V2MultiCompanyOneLoginRes} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2MultiCompanyLogin(c *gin.Context) {
	var login pba.V2MultiCompanyLoginReq

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "error getting environment id | not valid")
		return
	}

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if login.Type == "" {
		login.Type = config.Default
	}

	switch login.Type {
	case config.Default:
		if login.Username == "" {
			err := errors.New("username is required")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Password == "" {
			err := errors.New("password is required")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	case config.WithPhone:
		if login.SmsId == "" {
			err := errors.New("SmsId is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Otp == "" {
			err := errors.New("otp is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Phone == "" {
			err := errors.New("phone is required when type is phone")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	case config.WithEmail:
		if login.SmsId == "" {
			err := errors.New("SmsId is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Otp == "" {
			err := errors.New("otp is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Email == "" {
			err := errors.New("email is required when type is email")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	case config.WithGoogle:
		if login.GoogleToken == "" {
			err := errors.New("google token is required when type is google")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	}

	login.EnvId = environmentId.(string)

	resp, err := h.services.SessionService().V2MultiCompanyLogin(
		c.Request.Context(), &login,
	)
	if err != nil {
		h.handleError(c, http.InvalidArgument, err)
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// V2MultiCompanyOneLogin godoc
// @ID multi_company_one_login_v2
// @Router /v2/multi-company/one-login [POST]
// @Summary V2MultiCompanyOneLogin
// @Description V2MultiCompanyOneLogin
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body auth_service.V2MultiCompanyLoginReq true "LoginRequestBody"
// @Success 201 {object} http.Response{data=models.V2MultiCompanyOneLoginRes} "User data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2MultiCompanyOneLogin(c *gin.Context) {
	var login pba.V2MultiCompanyLoginReq

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if login.Type == "" {
		login.Type = config.Default
	}

	switch login.Type {
	case config.Default:
		if login.Username == "" {
			err := errors.New("username is required")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Password == "" {
			err := errors.New("password is required")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	case config.WithPhone:
		if login.Otp == "" {
			err := errors.New("otp is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Phone == "" {
			err := errors.New("phone is required when type is phone")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.ServiceType == "firebase" {
			if login.SessionInfo == "" {
				err := errors.New("session info is required when service type is firebase")
				h.handleResponse(c, http.BadRequest, err.Error())
				return
			}
		} else {
			if login.SmsId == "" {
				err := errors.New("SmsId is required when type is not default")
				h.handleResponse(c, http.BadRequest, err.Error())
				return
			}
		}
	case config.WithEmail:
		if login.SmsId == "" {
			err := errors.New("SmsId is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Otp == "" {
			err := errors.New("otp is required when type is not default")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}

		if login.Email == "" {
			err := errors.New("email is required when type is email")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	case config.WithGoogle:
		if login.GoogleToken == "" {
			err := errors.New("google token is required when type is google")
			h.handleResponse(c, http.BadRequest, err.Error())
			return
		}
	}

	resp, err := h.services.SessionService().V2MultiCompanyOneLogin(
		c.Request.Context(), &login,
	)
	if err != nil {
		h.handleError(c, http.InvalidArgument, err)
		return
	}

	h.handleResponse(c, http.Created, resp)
}

// V2ForgotPassword godoc
// @ID forgot_password
// @Router /v2/forgot-password [POST]
// @Summary ForgotPassword
// @Description Forgot Password
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body auth_service.ForgotPasswordRequest true "ForgotPasswordRequest"
// @Success 201 {object} http.Response{data=models.ForgotPasswordResponse} "Response"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) ForgotPassword(c *gin.Context) {
	var request pba.ForgotPasswordRequest

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*60)
	defer cancel()

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	user, err := h.services.UserService().GetUserByUsername(
		ctx, &pba.GetUserByUsernameRequest{
			Username: request.Login,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	if user.GetEmail() == "" {
		h.handleResponse(c, http.OK, models.ForgotPasswordResponse{
			EmailFound: false,
			UserId:     user.GetId(),
			Email:      user.GetEmail(),
		})
		return
	}

	code, err := util.GenerateCode(6)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	expire := time.Now().Add(time.Hour * 5).Add(time.Minute * 5)

	id, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	resp, err := h.services.EmailService().Create(
		c.Request.Context(), &pba.Email{
			Id:        id.String(),
			Email:     user.GetEmail(),
			Otp:       code,
			ExpiresAt: expire.String()[:19],
		},
	)
	if err != nil {
		h.handleError(c, http.GRPCError, err)
		return
	}

	err = helper.SendCodeToEmail("Код для подтверждения", user.GetEmail(), code, h.cfg.Email, h.cfg.EmailPassword)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	h.handleResponse(c, http.OK, models.ForgotPasswordResponse{
		EmailFound: true,
		SmsId:      resp.GetId(),
		UserId:     user.GetId(),
		Email:      user.GetEmail(),
	})
}

// V2ForgotPassword godoc
// @ID forgot_password_with_environment_email
// @Router /v2/forgot-password-with-environment-email [POST]
// @Summary ForgotPasswordWithEnvironmentEmail
// @Description Forgot Password With Environment Email
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body auth_service.ForgotPasswordRequest true "ForgotPasswordRequest"
// @Success 201 {object} http.Response{data=models.ForgotPasswordResponse} "Response"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) ForgotPasswordWithEnvironmentEmail(c *gin.Context) {
	var request pba.ForgotPasswordRequest

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*60)
	defer cancel()

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, "cant get environment_id")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, "cant get project_id")
		return
	}

	user, err := h.services.UserService().GetUserByUsername(
		ctx, &pba.GetUserByUsernameRequest{
			Username: request.Login,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	if user.GetEmail() == "" {
		h.handleResponse(c, http.OK, models.ForgotPasswordResponse{
			EmailFound: false,
			UserId:     user.GetId(),
			Email:      user.GetEmail(),
		})
		return
	}

	code, err := util.GenerateCode(6)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	expire := time.Now().Add(time.Hour * 5).Add(time.Minute * 5)

	id, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	resp, err := h.services.EmailService().Create(
		c.Request.Context(), &pba.Email{
			Id:        id.String(),
			Email:     user.GetEmail(),
			Otp:       code,
			ExpiresAt: expire.String()[:19],
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	emailSettings, err := h.services.EmailService().GetListEmailSettings(
		c.Request.Context(), &pba.GetListEmailSettingsRequest{
			ProjectId: projectId.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	if len(emailSettings.Items) < 1 {
		h.handleResponse(c, http.InvalidArgument, "email settings not found")
		return
	}

	err = helper.SendCodeToEnvironmentEmail("Your verification code", user.GetEmail(), code, emailSettings.Items[0].Email, emailSettings.Items[0].Password)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	h.handleResponse(c, http.OK, models.ForgotPasswordResponse{
		EmailFound: true,
		SmsId:      resp.GetId(),
		UserId:     user.GetId(),
		Email:      user.GetEmail(),
	})
}

// V2ForgotPassword godoc
// @ID set_email
// @Router /v2/set-email/send-code [PUT]
// @Summary SetEmail
// @Description Set Email
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param login body models.SetEmail true "SetEmailRequest"
// @Success 201 {object} http.Response{data=models.ForgotPasswordResponse} "Response"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) EmailEnter(c *gin.Context) {
	var request models.SetEmail

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*60)
	defer cancel()

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	res, err := h.services.SessionService().V2ResetPassword(ctx, &pba.V2ResetPasswordRequest{
		UserId: request.UserId,
		Email:  request.Email,
	})
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	code, err := util.GenerateCode(6)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}
	expire := time.Now().Add(time.Hour * 5).Add(time.Minute * 5)

	id, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	resp, err := h.services.EmailService().Create(
		c.Request.Context(),
		&pba.Email{
			Id:        id.String(),
			Email:     res.GetEmail(),
			Otp:       code,
			ExpiresAt: expire.String()[:19],
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	err = helper.SendCodeToEmail("Код для подтверждения", res.GetEmail(), code, h.cfg.Email, h.cfg.EmailPassword)
	if err != nil {
		h.handleResponse(c, http.InvalidArgument, err.Error())
		return
	}

	h.handleResponse(c, http.OK, models.ForgotPasswordResponse{
		EmailFound: true,
		SmsId:      resp.GetId(),
		UserId:     res.GetId(),
		Email:      res.GetEmail(),
	})
}

// V2ResetPassword godoc
// @ID v2_reset_password
// @Router /v2/reset-password [PUT]
// @Summary ResetPassword
// @Description Reset Password
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param body body models.ResetPassword true "ResetPasswordRequest"
// @Success 201 {object} http.Response{data=auth_service.User} "Response"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2ResetPassword(c *gin.Context) {
	var request models.ResetPassword

	ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second*60)
	defer cancel()

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	res, err := h.services.SessionService().V2ResetPassword(ctx, &pba.V2ResetPasswordRequest{
		Password: request.Password,
		UserId:   request.UserId,
	})
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}
	h.handleResponse(c, http.OK, res)
}

// ExpireSessions godoc
// @ID expire_sesssions
// @Router /expire-sessions [PUT]
// @Summary Expire Sessions
// @Description Expire Sessions
// @Tags Expire Sessions
// @Accept json
// @Produce json
// @Param sessions body auth_service.ExpireSessionsRequest true "ExpireSessionsRequestBody"
// @Success 200 {object} http.Response{data=string} "Response data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) ExpireSessions(c *gin.Context) {
	var req pba.ExpireSessionsRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	_, err := h.services.SessionService().ExpireSessions(
		c.Request.Context(),
		&req,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.OK, map[string]any{"message": "success"})
}

// DeleteByParams godoc
// @Security ApiKeyAuth
// @ID v2_delete_sessions_by_params
// @Router /v2/session [DELETE]
// @Summary Delete sessions by params
// @Description Delete sessions by user, client type, or session id in the current project.
// @Tags V2_Session
// @Accept json
// @Produce json
// @Param Authorization header string true "Bearer access token or API-KEY"
// @Param X-API-KEY header string false "API key when Authorization is API-KEY"
// @Param project-id query string false "Project id"
// @Param request body auth_service.DeleteByParamsRequest true "Delete sessions by params request"
// @Success 204
// @Response 400 {object} http.Response{data=string} "Invalid Argument"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) DeleteByParams(c *gin.Context) {
	var req pba.DeleteByParamsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok && !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, http.BadRequest, "project_id is required")
		return
	}

	req.ProjectId = cast.ToString(projectId)

	res, err := h.services.SessionService().DeleteByParams(
		c.Request.Context(),
		&req,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}
	h.handleResponse(c, http.NoContent, res)
}

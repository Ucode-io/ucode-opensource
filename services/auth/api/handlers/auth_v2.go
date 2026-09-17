package handlers

import (
	"context"
	"errors"
	"strings"

	status "github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	cfg "github.com/Ucode-io/ucode-opensource/services/auth/config"
	pba "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	obs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	pbSms "github.com/Ucode-io/ucode-opensource/services/auth/genproto/sms_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/firebase"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"

	"github.com/gin-gonic/gin"
	"github.com/saidamir98/udevs_pkg/util"
)

// V2Logout godoc
// @ID v2_logout
// @Router /v2/auth/logout [POST]
// @Summary V2Logout User
// @Description V2Logout User
// @Tags v2_auth
// @Accept json
// @Produce json
// @Param data body pba.LogoutRequest true "LogoutRequest"
// @Success 204
// @Response 400 {object} status.Response{data=string} "Invalid Argument"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) V2Logout(c *gin.Context) {
	var logout pba.LogoutRequest

	if err := c.ShouldBindJSON(&logout); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	resp, err := h.services.SessionService().Logout(
		c.Request.Context(),
		&logout,
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.NoContent, resp)
}

// V2Register godoc
// @ID V2RegisterProvider
// @Router /v2/register/{provider} [POST]
// @Summary V2RegisterProvider
// @Description V2RegisterProvider
// @Description in data must be have type, type must be one of the following values
// @Description ["google", "apple", "email", "phone"]
// @Description client_type_id and role_id must be in body parameters
// @Description you must be give environment_id and project_id in body or
// @Description Environment-Id hearder and project-id in query parameters or
// @Description X-API-KEY in hearder
// @Tags v2_auth
// @Accept json
// @Produce json
// @Param provider path string true "provider"
// @Param X-API-KEY header string false "X-API-KEY"
// @Param Environment-Id header string false "Environment-Id"
// @Param project-id query string false "project-id"
// @Param registerBody body models.RegisterOtp true "register_body"
// @Success 201 {object} status.Response{data=pba.V2LoginResponse} "User data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) V2RegisterProvider(c *gin.Context) {
	var body models.RegisterOtp

	if err := c.ShouldBindJSON(&body); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if c.Param("provider") == "" {
		h.handleResponse(c, status.BadRequest, "register provider is required")
		return
	}

	body.Data["type"] = c.Param("provider")

	if _, ok := body.Data["type"]; !ok {
		h.handleResponse(c, status.BadRequest, "register type is required")
		return
	}

	if !cfg.RegisterTypes[body.Data["type"].(string)] {
		h.handleResponse(c, status.BadRequest, "invalid register type")
		return
	}

	if _, ok := body.Data["client_type_id"].(string); !ok {
		if !util.IsValidUUID(body.Data["client_type_id"].(string)) {
			h.handleResponse(c, status.BadRequest, "client_type_id is an invalid uuid")
			return
		}
		h.handleResponse(c, status.BadRequest, "client_type_id is required")
		return
	}

	if _, ok := body.Data["role_id"].(string); !ok {
		if !util.IsValidUUID(body.Data["role_id"].(string)) {
			h.handleResponse(c, status.BadRequest, "role_id is an invalid uuid")
			return
		}
		h.handleResponse(c, status.BadRequest, "role_id is required")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.BadRequest, "cant get project_id")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, errors.New("cant get environment_id"))
		return
	}

	serviceResource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(),
		&obs.GetSingleServiceResourceReq{
			EnvironmentId: environmentId.(string),
			ProjectId:     projectId.(string),
			ServiceType:   obs.ServiceType_BUILDER_SERVICE,
		},
	)

	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	project, err := h.services.ProjectServiceClient().GetById(context.Background(), &obs.GetProjectByIdRequest{
		ProjectId: serviceResource.GetProjectId(),
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	switch body.Data["type"] {
	case cfg.WithGoogle:
		{
			h.handleResponse(c, status.BadRequest, "register with goole not implemented")
			return

		}
	case cfg.WithApple:
		{
			h.handleResponse(c, status.BadRequest, "registre with apple not implemented")
			return
		}
	case cfg.WithEmail:
		{
			if v, ok := body.Data["email"]; ok {
				if !util.IsValidEmail(v.(string)) {
					h.handleResponse(c, status.BadRequest, "Неверный формат email")
					return
				}
			} else {
				h.handleResponse(c, status.BadRequest, "Поле email не заполнено")
				return
			}

			if _, ok := body.Data["login"]; !ok {
				h.handleResponse(c, status.BadRequest, "Поле login не заполнено")
				return
			}

			if _, ok := body.Data["name"]; !ok {
				h.handleResponse(c, status.BadRequest, "Поле name не заполнено")
				return
			}

			if _, ok := body.Data["phone"]; !ok {
				h.handleResponse(c, status.BadRequest, "Поле phone не заполнено")
				return
			}
		}
	case cfg.WithPhone:
		{
			if _, ok := body.Data["phone"]; !ok {
				h.handleResponse(c, status.BadRequest, "Поле phone не заполнено")
				return

			}
		}
	}

	if body.Data["addational_table"] != nil {
		if body.Data["addational_table"].(map[string]any)["table_slug"] == nil {
			h.log.Error("Addational user create >>>> ")
			h.handleResponse(c, status.BadRequest, "If addional table have, table slug is required")
			return
		}
	}

	body.Data["project_id"] = serviceResource.GetProjectId()
	body.Data["environment_id"] = serviceResource.GetEnvironmentId()
	body.Data["resource_environment_id"] = serviceResource.GetResourceEnvironmentId()
	body.Data["environment_id"] = serviceResource.GetEnvironmentId()
	body.Data["company_id"] = project.GetCompanyId()
	body.Data["resource_type"] = serviceResource.GetResourceType()
	body.Data["node_type"] = serviceResource.GetNodeType()

	structData, err := helper.ConvertMapToStruct(body.Data)
	if err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	response, err := h.services.RegisterService().RegisterUser(c.Request.Context(), &pba.RegisterUserRequest{
		Data:     structData,
		NodeType: serviceResource.NodeType,
	})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.Created, response)
}

// V2VerifyProvider godoc
// @ID V2VerifyProvider
// @Router /v2/auth/verify/{verify_id} [POST]
// @Summary Verify Otp
// @Description V2VerifyProvider
// @Tags v2_auth
// @Accept json
// @Produce json
// @Param verify_id path string true "verify_id"
// @Param X-API-KEY header string false "X-API-KEY"
// @Param Resource-Id header string true "Resource-Id"
// @Param Environment-Id header string true "Environment-Id"
// @Param verifyBody body models.Verify true "verify_body"
// @Success 201 {object} status.Response{data=pba.V2LoginResponse} "User data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) V2VerifyOtp(c *gin.Context) {
	var (
		body                models.Verify
		resourceEnvironment *obs.ResourceEnvironment
	)

	if err := c.ShouldBindJSON(&body); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	if body.Provider == "" {
		h.handleResponse(c, status.BadRequest, "Provider type is required")
		return
	}

	body.RegisterType = body.Provider
	if body.Data == nil {
		body.Data = &pbObject.V2LoginResponse{}
	}

	resourceId, ok := c.Get("resource_id")
	if !ok || !util.IsValidUUID(resourceId.(string)) {
		h.handleResponse(c, status.BadRequest, errors.New("cant get resource_id"))
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, errors.New("cant get environment_id"))
		return
	}

	resourceEnvironment, err := h.services.ResourceService().GetResourceEnvironment(
		c.Request.Context(),
		&obs.GetResourceEnvironmentReq{
			EnvironmentId: environmentId.(string),
			ResourceId:    resourceId.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(
		c,
		resourceEnvironment.ProjectId,
		resourceEnvironment.NodeType,
	)

	switch strings.ToLower(body.Provider) {
	case "email", cfg.Default:
		{
			if !cfg.DEFAULT_OTPS[body.Otp] {
				_, err := h.services.SmsService().ConfirmOtp(
					c.Request.Context(),
					&pbSms.ConfirmOtpRequest{
						SmsId: c.Param("verify_id"),
						Otp:   body.Otp,
					},
				)
				if err != nil {
					h.handleResponse(c, status.GRPCError, err.Error())
					return
				}
			}
		}
	case cfg.WithPhone:
		{
			if !cfg.DEFAULT_OTPS[body.Otp] {
				_, err := services.SmsService().ConfirmOtp(
					c.Request.Context(),
					&pbSms.ConfirmOtpRequest{
						SmsId: c.Param("verify_id"),
						Otp:   body.Otp,
					},
				)
				if err != nil {
					h.handleResponse(c, status.GRPCError, err.Error())
					return
				}
			}
		}
	case cfg.WithFirebase:
		{
			if !cfg.DEFAULT_OTPS[body.Otp] {
				err := firebase.VerifyPhoneCode(h.cfg, body.SessionInfo, body.Otp)
				if err != nil {
					h.handleResponse(c, status.GRPCError, err.Error())
					return
				}
			}
		}
	case cfg.WithGoogle:
		{
			if body.GoogleToken == "" {
				h.handleResponse(c, status.BadRequest, "google token is required when register type is google")
				return
			}

			userInfo, err := helper.GetGoogleUserInfo(body.GoogleToken)
			if err != nil {
				h.handleResponse(c, status.BadRequest, "Invalid arguments google auth")
				return
			}
			if userInfo["error"] != nil || !(userInfo["email_verified"].(bool)) {
				h.handleResponse(c, status.BadRequest, "Invalid google access token")
				return
			}

			respObject, err := services.GetLoginServiceByType(resourceEnvironment.NodeType).LoginWithEmailOtp(
				c.Request.Context(),
				&pbObject.EmailOtpRequest{
					ClientType: "WEB_USER",
					TableSlug:  "user",
					Email:      userInfo["email"].(string),
					ProjectId:  resourceEnvironment.GetId(),
				},
			)
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}

			if respObject == nil || !respObject.UserFound {
				h.handleResponse(c, status.OK, "User verified with google token but not found")
				return
			}

			convertedToAuthPb := helper.ConvertPbToAnotherPb(respObject)
			res, err := h.services.SessionService().SessionAndTokenGenerator(
				c.Request.Context(),
				&pba.SessionAndTokenRequest{
					LoginData: convertedToAuthPb,
					Tables:    body.Tables,
					ProjectId: resourceEnvironment.GetProjectId(),
					ClientIp:  c.ClientIP(),
					UserAgent: c.Request.UserAgent(),
				})
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}

			h.handleResponse(c, status.Created, res)
			return
		}
	case cfg.WithApple:
		{
			if body.AppleCode == "" {
				h.handleResponse(c, status.BadRequest, "apple code is required when register type is apple id")
				return
			}

			appleConfig, err := h.GetAppleConfig(resourceEnvironment.ProjectId, c.Request.Context())
			if err != nil {
				h.handleResponse(c, status.BadRequest, "can't get apple configs to get user info")
				return
			}

			userInfo, err := helper.GetAppleUserInfo(body.AppleCode, appleConfig)
			if err != nil {
				h.handleResponse(c, status.BadRequest, err.Error())
				return
			}

			respObject, err := services.GetLoginServiceByType(resourceEnvironment.NodeType).LoginWithEmailOtp(
				c.Request.Context(),
				&pbObject.EmailOtpRequest{
					ClientType: "WEB_USER",
					TableSlug:  "user",
					Email:      userInfo.Email,
					ProjectId:  resourceEnvironment.GetId(),
				},
			)
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}

			if respObject == nil || !respObject.UserFound {
				h.handleResponse(c, status.OK, "User verified with apple code but not found")
				return
			}

			convertedToAuthPb := helper.ConvertPbToAnotherPb(respObject)
			res, err := h.services.SessionService().SessionAndTokenGenerator(
				c.Request.Context(),
				&pba.SessionAndTokenRequest{
					LoginData: convertedToAuthPb,
					Tables:    body.Tables,
					ProjectId: resourceEnvironment.GetProjectId(),
					ClientIp:  c.ClientIP(),
					UserAgent: c.Request.UserAgent(),
				})
			if err != nil {
				h.handleResponse(c, status.GRPCError, err.Error())
				return
			}

			h.handleResponse(c, status.Created, res)
			return
		}
	default:
		{
			h.handleResponse(c, status.GRPCError, err.Error())
			return
		}
	}
	if !body.Data.UserFound {
		h.handleResponse(c, status.OK, "User verified but not found")
		return
	}

	convertedToAuthPb := helper.ConvertPbToAnotherPb(body.Data)
	res, err := h.services.SessionService().SessionAndTokenGenerator(
		c.Request.Context(),
		&pba.SessionAndTokenRequest{
			LoginData: convertedToAuthPb,
			Tables:    body.Tables,
			ProjectId: resourceEnvironment.GetProjectId(),
			ClientIp:  c.ClientIP(),
			UserAgent: c.Request.UserAgent(),
		})
	if err != nil {
		h.handleResponse(c, status.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, status.Created, res)
}

// V2LoginProvider godoc
// @ID V2LoginProvider
// @Router /v2/auth/login/{provider} [POST]
// @Summary V2LoginProvider
// @Description V2LoginProvider
// @Description inside the data you must be passed client_type_id field
// @Description you must be give environment_id and project_id in body or
// @Description Environment-Id hearder and project-id in query parameters or
// @Description X-API-KEY in hearder
// @Description login strategy must be one of the following values
// @Description ["EMAIL", "PHONE", "EMAIL_OTP", "PHONE_OTP", "LOGIN", "LOGIN_PWD", "GOOGLE_AUTH", "APPLE_AUTH", "PHONE_PWD", "EMAIL_PWD"]
// @Tags v2_auth
// @Accept json
// @Produce json
// @Param Environment-Id header string false "Environment-Id"
// @Param X-API-KEY header string false "X-API-KEY"
// @Param project-id query string false "project-id"
// @Param login body pba.V2LoginWithOptionRequest true "V2LoginRequest"
// @Success 201 {object} status.Response{data=pba.V2LoginSuperAdminRes} "User data"
// @Response 400 {object} status.Response{data=string} "Bad Request"
// @Failure 500 {object} status.Response{data=string} "Server Error"
func (h *Handler) V2LoginProvider(c *gin.Context) {
	var login pba.V2LoginWithOptionRequest

	if err := c.ShouldBindJSON(&login); err != nil {
		h.handleResponse(c, status.BadRequest, err.Error())
		return
	}

	clientType := login.Data["client_type_id"]
	if !util.IsValidUUID(clientType) {
		h.handleResponse(c, status.InvalidArgument, "inside data client_type_id is required")
		return
	}

	projectId, ok := c.Get("project_id")
	if !ok || !util.IsValidUUID(projectId.(string)) {
		h.handleResponse(c, status.InvalidArgument, "project id is an invalid uuid")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, status.BadRequest, "error getting environment id | not valid")
		return
	}

	provider := c.Param("provider")
	if provider == "" {
		h.handleResponse(c, status.InvalidArgument, "provider is required(param)")
		return
	}

	login.LoginStrategy = provider
	login.Data["environment_id"] = environmentId.(string)
	login.Data["project_id"] = projectId.(string)

	resp, err := h.services.SessionService().V2LoginWithOption(
		c.Request.Context(), &pba.V2LoginWithOptionRequest{
			Data:          login.GetData(),
			LoginStrategy: login.GetLoginStrategy(),
			Tables:        login.GetTables(),
		},
	)
	if err != nil {
		var httpErrorStr = ""

		httpErrorStr = strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
		httpErrorStr = strings.ToLower(httpErrorStr)

		switch httpErrorStr {
		case "user not found":
			h.handleResponse(c, status.NotFound, "Пользователь не найдено")
			return
		case "session has been expired":
			h.handleResponse(c, status.InvalidArgument, "срок действия пользователя истек")
			return
		case "invalid username":
			h.handleResponse(c, status.InvalidArgument, "неверное имя пользователя")
			return
		case "invalid password":
			h.handleResponse(c, status.InvalidArgument, "неверное пароль")
			return
		case "user blocked":
			h.handleResponse(c, status.BadRequest, "Пользователь заблокирован")
			return
		default:
			h.handleResponse(c, status.InvalidArgument, err.Error())
			return
		}
	}

	res := &pba.V2LoginSuperAdminRes{
		UserFound: resp.GetUserFound(),
		Token:     resp.GetToken(),
		Companies: resp.GetCompanies(),
		UserId:    resp.GetUserId(),
		Sessions:  resp.GetSessions(),
		UserData:  resp.GetUserData(),
	}

	h.handleResponse(c, status.Created, res)
}

package handlers

import (
	"bytes"
	"errors"
	"fmt"
	"text/template"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	cfg "github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbc "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	os "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	pbSms "github.com/Ucode-io/ucode-opensource/services/auth/genproto/sms_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/cast"
)

// V2SendCodeApp godoc
// @ID V2SendCodeApp
// @Router /v2/send-code-app [POST]
// @Summary SendCodeApp
// @Description SendCode type must be one of the following values ["EMAIL", "PHONE"]
// @Tags v2_register
// @Accept json
// @Produce json
// @Param login body models.V2SendCodeRequest true "SendCode"
// @Success 201 {object} http.Response{data=models.V2SendCodeResponse} "Send Code data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2SendCodeApp(c *gin.Context) {
	var request models.V2SendCodeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	id, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}
	if !util.ValidRecipients[request.Type] {
		h.handleResponse(c, http.BadRequest, "Invalid recipient type")
		return
	}

	expire := time.Now().Add(time.Minute * 5) // todo dont write expire time here

	code, err := util.GenerateCode(4)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	body := &pbSms.Sms{
		Id:        id.String(),
		Text:      cfg.SMS_TEXT,
		Otp:       code,
		Recipient: request.Recipient,
		ExpiresAt: expire.String()[:19],
		Type:      request.Type,
	}

	switch request.Type {
	case "PHONE":
		if !util.IsValidPhone(request.Recipient) {
			h.handleResponse(c, http.BadRequest, "Неверный номер телефона, он должен содержать двенадцать цифр и +")
			return
		}

	case "EMAIL":
		if !util.IsValidEmail(request.Recipient) {
			h.handleResponse(c, http.BadRequest, "Email is not valid")
			return
		}
	}

	_, err = h.services.UserService().V2GetUserByLoginTypes(c.Request.Context(), &pb.GetUserByLoginTypesRequest{
		Email: request.Recipient,
		Phone: request.Recipient,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resp, err := h.services.SmsService().Send(
		c.Request.Context(), body,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, models.V2SendCodeResponse{SmsId: resp.SmsId})
}

// V2SendCode godoc
// @ID V2SendCode
// @Router /v2/send-code [POST]
// @Summary SendCode
// @Description SendCode type must be one of the following values ["EMAIL", "PHONE"]
// @Tags v2_register
// @Accept json
// @Produce json
// @Param X-API-KEY header string false "X-API-KEY"
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string false "Environment-Id"
// @Param login body models.V2SendCodeRequest true "SendCode"
// @Success 201 {object} http.Response{data=models.V2SendCodeResponse} "Send Code data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2SendCode(c *gin.Context) {
	var request models.V2SendCodeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	if request.Variables == nil {
		request.Variables = make(map[string]any)
	}

	id, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	if !util.ValidRecipients[request.Type] {
		h.handleResponse(c, http.BadRequest, cfg.InvalidRecipientError)
		return
	}

	resourceId, ok := c.Get("resource_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, "cant get resource_id")
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, cfg.EnvironmentIdError)
		return
	}

	expire := time.Now().Add(time.Minute * 2) // todo dont write expire time here

	code, err := util.GenerateCode(4)
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}

	resourceEnvironment, err := h.services.ResourceService().GetResourceEnvironment(
		c.Request.Context(), &pbc.GetResourceEnvironmentReq{
			EnvironmentId: environmentId.(string),
			ResourceId:    resourceId.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	services, err := h.GetProjectSrvc(c, resourceEnvironment.ProjectId, resourceEnvironment.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	var text string
	if request.SmsTemplateId != "" {
		structData, err := helper.ConvertMapToStruct(map[string]any{"id": request.SmsTemplateId})
		if err != nil {
			h.handleResponse(c, http.InvalidArgument, err.Error())
			return
		}

		type SmsTemplateResponse struct {
			Response map[string]any `json:"response" mapstructure:"response"`
		}

		switch resourceEnvironment.ResourceType {
		case 1:
			smsTemplateResp, err := services.GetObjectBuilderServiceByType(resourceEnvironment.NodeType).GetSingleSlim(
				c.Request.Context(),
				&os.CommonMessage{
					TableSlug: "sms_template",
					ProjectId: resourceEnvironment.Id,
					Data:      structData,
				},
			)
			if err != nil {
				h.handleResponse(c, http.GRPCError, err.Error())
				return
			}

			var smsTemplateRespData SmsTemplateResponse
			if err := mapstructure.Decode(smsTemplateResp.Data.AsMap(), &smsTemplateRespData); err == nil {
				text = cast.ToString(smsTemplateRespData.Response[request.FieldSlug])
				if text == "" {
					text = cast.ToString(smsTemplateRespData.Response["text"])
				}
			}
		case 3:
			smsTemplateResp, err := services.GoObjectBuilderService().GetSingleSlim(
				c.Request.Context(),
				&nobs.CommonMessage{
					TableSlug: "sms_template",
					ProjectId: resourceEnvironment.Id,
					Data:      structData,
				},
			)
			if err != nil {
				h.handleResponse(c, http.GRPCError, err.Error())
				return
			}

			var smsTemplateRespData SmsTemplateResponse
			if err := mapstructure.Decode(smsTemplateResp.Data.AsMap(), &smsTemplateRespData); err == nil {
				text = cast.ToString(smsTemplateRespData.Response[request.FieldSlug])
				if text == "" {
					text = cast.ToString(smsTemplateRespData.Response["text"])
				}
			}
		}
	}

	body := &pbSms.Sms{
		Id:        id.String(),
		Text:      text,
		Otp:       code,
		Recipient: request.Recipient,
		ExpiresAt: expire.String()[:19],
		Type:      request.Type,
	}

	switch request.Type {
	case "PHONE":
		if !util.IsValidPhone(request.Recipient) {
			h.handleResponse(c, http.BadRequest, cfg.InvalidPhoneError)
			return
		}
		smsOtpSettings, err := h.services.ResourceService().GetProjectResourceList(
			c.Request.Context(), &pbc.GetProjectResourceListRequest{
				ProjectId:     resourceEnvironment.ProjectId,
				EnvironmentId: environmentId.(string),
				Type:          pbc.ResourceType_SMS,
			})
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		if len(smsOtpSettings.GetResources()) > 0 {
			if smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetNumberOfOtp() != 0 {
				code, err := util.GenerateCode(int(smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetNumberOfOtp()))
				if err != nil {
					h.handleResponse(c, http.InvalidArgument, cfg.InvalidOTPError)
					return
				}
				body.Otp = code
			}
			body.DevEmail = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetLogin()
			body.DevEmailPassword = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetPassword()
			body.Originator = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetOriginator()
		}
	case "EMAIL":
		if !util.IsValidEmail(request.Recipient) {
			h.handleResponse(c, http.BadRequest, cfg.InvalidEmailError)
			return
		}

		emailSettings, err := h.services.ResourceService().GetProjectResourceList(
			c.Request.Context(), &pbc.GetProjectResourceListRequest{
				ProjectId:     resourceEnvironment.ProjectId,
				EnvironmentId: environmentId.(string),
				Type:          pbc.ResourceType_SMTP,
			})
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		if len(emailSettings.GetResources()) < 1 {
			h.handleResponse(c, http.InvalidArgument, cfg.EmailSettingsError)
			return
		}

		if len(emailSettings.GetResources()) > 0 {
			code, err := util.GenerateCode(int(emailSettings.GetResources()[0].GetSettings().GetSmtp().GetNumberOfOtp()))
			if err != nil {
				h.handleResponse(c, http.InvalidArgument, cfg.InvalidOTPError)
				return
			}
			body.Otp = code

			body.DevEmail = emailSettings.GetResources()[0].GetSettings().GetSmtp().GetEmail()
			body.DevEmailPassword = emailSettings.GetResources()[0].GetSettings().GetSmtp().GetPassword()
		}
	case "MAILCHIMP":
		if !util.IsValidEmail(request.Recipient) {
			h.handleResponse(c, http.BadRequest, cfg.InvalidEmailError)
			return
		}

		emailSettings, err := h.services.ResourceService().GetProjectResourceList(
			c.Request.Context(), &pbc.GetProjectResourceListRequest{
				ProjectId:     resourceEnvironment.ProjectId,
				EnvironmentId: environmentId.(string),
				Type:          pbc.ResourceType_MAILCHIMP,
			})
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}

		if len(emailSettings.GetResources()) < 1 {
			h.handleResponse(c, http.InvalidArgument, cfg.EmailSettingsError)
			return
		}

		if len(emailSettings.GetResources()) > 0 {
			code, err := util.GenerateCode(int(emailSettings.GetResources()[0].GetSettings().GetMailchimp().GetNumberOfOtp()))
			if err != nil {
				h.handleResponse(c, http.InvalidArgument, cfg.InvalidOTPError)
				return
			}
			body.Otp = code

			body.DevEmail = emailSettings.GetResources()[0].GetSettings().GetMailchimp().GetFromEmail()
			body.DevEmailPassword = emailSettings.GetResources()[0].GetSettings().GetMailchimp().GetApiKey()
		}
	}

	if text == "" {
		text = fmt.Sprintf("%s: %s", cfg.SMS_TEXT, body.Otp)
	} else {
		request.Variables["code"] = body.Otp
		t, err := template.New("sms").Parse(text)
		if err != nil {
			h.handleResponse(c, http.InternalServerError, err.Error())
			return
		}
		var buf bytes.Buffer
		err = t.Execute(&buf, request.Variables)
		if err != nil {
			h.handleResponse(c, http.InternalServerError, err.Error())
			return
		}
		text = buf.String()
	}

	body.Text = text
	resp, err := services.SmsService().Send(
		c.Request.Context(), body,
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, models.V2SendCodeResponse{SmsId: resp.SmsId})
}

// V2Register godoc
// @ID V2register
// @Router /v2/register [POST]
// @Summary V2Register
// @Description V2Register
// @Description in data must be have type, type must be one of the following values
// @Description ["google", "apple", "email", "phone"]
// @Description client_type_id and role_id must be in body parameters
// @Description you must be give environment_id and project_id in body or
// @Description Environment-Id hearder and project-id in query parameters or
// @Description X-API-KEY in hearder
// @Tags v2_register
// @Accept json
// @Produce json
// @Param X-API-KEY header string false "X-API-KEY"
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string false "Environment-Id"
// @Param project-id query string false "project-id"
// @Param registerBody body models.RegisterOtp true "register_body"
// @Success 201 {object} http.Response{data=models.V2LoginResponse} "Register Data data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) V2Register(c *gin.Context) {
	var body models.RegisterOtp

	if err := c.ShouldBindJSON(&body); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	var (
		registerType = body.Data["type"].(string)
		clientTypeId = body.Data["client_type_id"].(string)
		roleId       = body.Data["role_id"].(string)

		projectId     = cast.ToString(body.Data["project_id"])
		environmentId = cast.ToString(body.Data["environment_id"])
	)

	if len(projectId) == 0 {
		projectId = helper.AnyToString(c.Get("project_id"))
	}

	if len(environmentId) == 0 {
		environmentId = helper.AnyToString(c.Get("environment_id"))
	}

	for _, id := range []string{clientTypeId, roleId, projectId, environmentId} {
		if !util.IsValidUUID(id) {
			h.handleResponse(c, http.BadRequest, fmt.Sprintf("%s is an invalid uuid or not exist", id))
			return
		}
	}

	serviceResource, err := h.services.ServiceResource().GetSingle(
		c.Request.Context(), &pbc.GetSingleServiceResourceReq{
			EnvironmentId: environmentId,
			ProjectId:     projectId,
			ServiceType:   pbc.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	project, err := h.services.ProjectServiceClient().GetById(
		c.Request.Context(), &pbc.GetProjectByIdRequest{
			ProjectId: serviceResource.GetProjectId(),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	switch registerType {
	case cfg.WithEmail:
		if value, ok := body.Data[cfg.WithEmail]; ok {
			if !util.IsValidEmail(value.(string)) {
				h.handleResponse(c, http.BadRequest, "Неверный формат email")
				return
			}
		} else {
			h.handleResponse(c, http.BadRequest, "Поле email не заполнено")
			return
		}
	case cfg.WithPhone:
		if _, ok := body.Data[cfg.WithPhone]; !ok {
			h.handleResponse(c, http.BadRequest, "Поле phone не заполнено")
			return
		}
	case cfg.WithLogin:
		if _, ok := body.Data[cfg.WithLogin]; !ok {
			h.handleResponse(c, http.BadRequest, "неверный формат email")
			return
		}
	default:
		h.handleResponse(c, http.BadRequest, "register with google and apple not implemented")
		return
	}

	if value, ok := body.Data["addational_table"]; ok {
		if value.(map[string]any)["table_slug"] == nil {
			h.handleResponse(c, http.BadRequest, "If addional table have, table slug is required")
			return
		}
	}

	body.Data["company_id"] = project.GetCompanyId()
	body.Data["node_type"] = serviceResource.GetNodeType()
	body.Data["project_id"] = serviceResource.GetProjectId()
	body.Data["resource_type"] = serviceResource.GetResourceType()
	body.Data["environment_id"] = serviceResource.GetEnvironmentId()
	body.Data["resource_environment_id"] = serviceResource.GetResourceEnvironmentId()

	structData, err := helper.ConvertMapToStruct(body.Data)
	if err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	response, err := h.services.RegisterService().RegisterUser(c.Request.Context(), &pb.RegisterUserRequest{
		RoleId:                roleId,
		Data:                  structData,
		ClientIp:              c.ClientIP(),
		ClientTypeId:          clientTypeId,
		Type:                  registerType,
		CompanyId:             project.CompanyId,
		UserAgent:             c.Request.UserAgent(),
		NodeType:              serviceResource.NodeType,
		ProjectId:             serviceResource.ProjectId,
		ResourceId:            serviceResource.ResourceId,
		EnvironmentId:         serviceResource.EnvironmentId,
		ResourceEnvironmentId: serviceResource.ResourceEnvironmentId,
	})
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, response)
}

// SendMessage godoc
// @ID SendMessage
// @Router /v2/send-message [POST]
// @Summary SendMessage
// @Description SendMessage type must be one of the following values ["EMAIL", "PHONE"]
// @Tags v2_register
// @Accept json
// @Produce json
// @Param X-API-KEY header string false "X-API-KEY"
// @Param Resource-Id header string false "Resource-Id"
// @Param Environment-Id header string false "Environment-Id"
// @Param login body models.V2SendCodeRequest true "SendCode"
// @Success 201 {object} http.Response{data=models.V2SendCodeResponse} "Send Message data"
// @Response 400 {object} http.Response{data=string} "Bad Request"
// @Failure 500 {object} http.Response{data=string} "Server Error"
func (h *Handler) SendMessage(c *gin.Context) {
	var request models.V2SendCodeRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.handleResponse(c, http.BadRequest, err.Error())
		return
	}

	id, err := uuid.NewRandom()
	if err != nil {
		h.handleResponse(c, http.InternalServerError, err.Error())
		return
	}
	if !util.ValidRecipients[request.Type] {
		h.handleResponse(c, http.BadRequest, "Invalid recipient type")
		return
	}

	resourceId, ok := c.Get("resource_id")
	if !ok {
		h.handleResponse(c, http.BadRequest, errors.New("cant get resource_id").Error())
		return
	}

	environmentId, ok := c.Get("environment_id")
	if !ok || !util.IsValidUUID(environmentId.(string)) {
		h.handleResponse(c, http.BadRequest, errors.New("cant get environment_id").Error())
		return
	}

	expire := time.Now().Add(time.Minute * 5) // todo dont write expire time here

	resourceEnvironment, err := h.services.ResourceService().GetResourceEnvironment(
		c.Request.Context(), &pbc.GetResourceEnvironmentReq{
			EnvironmentId: environmentId.(string),
			ResourceId:    resourceId.(string),
		},
	)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	body := &pbSms.Sms{
		Id:        id.String(),
		Text:      cfg.SMS_TEXT,
		Otp:       "",
		Recipient: request.Recipient,
		ExpiresAt: expire.String()[:19],
		Type:      request.Type,
	}

	switch request.Type {
	case "PHONE":
		if !util.IsValidPhone(request.Recipient) {
			h.handleResponse(c, http.BadRequest, "Неверный номер телефона, он должен содержать двенадцать цифр и +")
			return
		}
		smsOtpSettings, err := h.services.ResourceService().GetProjectResourceList(
			c.Request.Context(), &pbc.GetProjectResourceListRequest{
				ProjectId:     resourceEnvironment.ProjectId,
				EnvironmentId: environmentId.(string),
				Type:          pbc.ResourceType_SMS,
			})
		if err != nil {
			h.handleResponse(c, http.GRPCError, err.Error())
			return
		}
		if len(smsOtpSettings.GetResources()) > 0 {
			body.DevEmail = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetLogin()
			body.DevEmailPassword = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetPassword()
			body.Originator = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetOriginator()
		}
	}

	services, err := h.GetProjectSrvc(c, resourceEnvironment.ProjectId, resourceEnvironment.NodeType)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	resp, err := services.SmsService().Send(c.Request.Context(), body)
	if err != nil {
		h.handleResponse(c, http.GRPCError, err.Error())
		return
	}

	h.handleResponse(c, http.Created, models.V2SendCodeResponse{
		SmsId: resp.SmsId,
	})
}

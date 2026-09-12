package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	status_http "github.com/Ucode-io/ucode-opensource/services/auth/api/http"
	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbCompany "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	pbObject "github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/sms_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/eimzo"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/firebase"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"

	"github.com/google/uuid"
	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

var (
	errUserNotFound   = status.Error(codes.NotFound, "user not found")
	errNoValidProject = status.Error(codes.NotFound, "user project not found")
	errUnableGenToken = status.Error(codes.Internal, "unable to generate token")
)

func (s *sessionService) verifyAndMigratePassword(user *pb.User, password string) error {
	switch config.HashTypes[user.GetHashType()] {
	case 1:
		match, err := security.ComparePassword(user.GetPassword(), password)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if !match {
			return status.Error(codes.Unauthenticated, "username or password is wrong")
		}
		go func() {
			migrateCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			hashed, err := security.HashPasswordBcrypt(password)
			if err != nil {
				s.log.Error("!!!migratePassword--->HashBcrypt", logger.Error(err))
				return
			}
			if err = s.strg.User().UpdatePassword(migrateCtx, user.GetId(), hashed); err != nil {
				s.log.Error("!!!migratePassword--->UpdatePassword", logger.Error(err))
			}
		}()

	case 2:
		match, err := security.ComparePasswordBcrypt(user.GetPassword(), password)
		if err != nil {
			return status.Error(codes.Internal, err.Error())
		}
		if !match {
			return status.Error(codes.Unauthenticated, "username or password is wrong")
		}

	default:
		return status.Error(codes.Internal, "invalid hash type")
	}
	return nil
}

func (s *sessionService) UserDefaultProject(ctx context.Context, req *pb.UserDefaultProjectReq) (*pb.UserDefaultProjectResp, error) {
	s.log.Info("UserDefaultProject --> ", logger.Any("request: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.UserDefaultProject", req)
	defer dbSpan.Finish()

	user, err := s.lookupUser(ctx, req)
	if err != nil {
		return nil, err
	}

	if user == nil || user.Id == "" {
		return nil, status.Error(codes.NotFound, "cannot get user")
	}

	var (
		userProjects   *pb.GetUserProjectsRes
		userEnvProject *models.GetUserEnvProjectRes
		projectsErr    error
		envProjectsErr error
		initWg         sync.WaitGroup
	)

	initWg.Add(2)
	go func() {
		defer initWg.Done()
		userProjects, projectsErr = s.strg.User().GetUserProjects(ctx, user.GetId())
	}()
	go func() {
		defer initWg.Done()
		userEnvProject, envProjectsErr = s.strg.User().GetUserEnvProjects(ctx, user.GetId())
	}()
	initWg.Wait()

	if projectsErr != nil {
		s.log.Error("!!!UserDefaultProject--->GetUserProjects", logger.Error(projectsErr))
		return nil, status.Error(codes.NotFound, "cant get user projects")
	}
	if envProjectsErr != nil {
		s.log.Error("!!!UserDefaultProject--->GetUserEnvProjects", logger.Error(envProjectsErr))
		return nil, status.Error(codes.NotFound, "cant get user env projects")
	}

	for _, item := range userProjects.Companies {
		companyId := item.Id

		for _, projId := range item.ProjectIds {
			var (
				company      *pbCompany.GetCompanyByIdResponse
				clientType   *pb.GetUserProjectClientTypesResponse
				projectInfo  *pbCompany.Project
				environments *pbCompany.GetEnvironmentListResponse
				userSt       string

				companyErr error
				clientErr  error
				projectErr error
				envErr     error
				statusErr  error

				innerWg sync.WaitGroup
			)

			innerWg.Add(5)

			go func() {
				defer innerWg.Done()
				company, companyErr = s.services.CompanyServiceClient().GetById(ctx, &pbCompany.GetCompanyByIdRequest{
					Id: companyId,
				})
			}()

			go func() {
				defer innerWg.Done()
				clientType, clientErr = s.strg.User().GetUserProjectClientTypes(ctx, &pb.UserInfoPrimaryKey{
					UserId:    user.GetId(),
					ProjectId: projId,
				})
			}()

			go func() {
				defer innerWg.Done()
				projectInfo, projectErr = s.services.ProjectServiceClient().GetById(ctx, &pbCompany.GetProjectByIdRequest{
					ProjectId: projId,
					CompanyId: companyId,
				})
			}()

			go func() {
				defer innerWg.Done()
				environments, envErr = s.services.EnvironmentService().GetList(ctx, &pbCompany.GetEnvironmentListRequest{
					Ids:       userEnvProject.EnvProjects[projId],
					Limit:     100,
					ProjectId: projId,
				})
			}()

			go func() {
				defer innerWg.Done()
				userSt, statusErr = s.strg.User().GetUserStatus(ctx, user.Id, projId)
			}()

			innerWg.Wait()

			if companyErr != nil || company.GetCompany().GetId() == "" {
				continue
			}
			if clientErr != nil || len(clientType.GetClientTypeIds()) == 0 {
				continue
			}
			if projectErr != nil || projectInfo.GetProjectId() == "" {
				continue
			}
			if envErr != nil || len(environments.GetEnvironments()) == 0 {
				continue
			}
			if statusErr != nil || userSt == config.UserStatusBlocked {
				continue
			}

			var (
				prodEnvId      string
				projectInfoMap map[string]any
			)

			for _, env := range environments.Environments {
				if env.Name == "Production" {
					prodEnvId = env.Id
					break
				}
			}
			if prodEnvId == "" {
				prodEnvId = environments.Environments[0].Id
			}

			// FIXED TEMPORARY
			if _, srErr := s.services.ServiceResource().GetSingle(ctx, &pbCompany.GetSingleServiceResourceReq{
				ProjectId:     projectInfo.ProjectId,
				EnvironmentId: prodEnvId,
				ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
			}); srErr != nil {
				s.log.Warn("!!!UserDefaultProject--->skip project: no BUILDER_SERVICE",
					logger.String("project_id", projectInfo.ProjectId),
					logger.String("environment_id", prodEnvId),
					logger.Error(srErr),
				)
				continue
			}

			projectInfoByte, err := json.Marshal(projectInfo)
			if err != nil {
				s.log.Error("!!!UserDefaultProject--->marshal projectInfo", logger.Error(err))
				continue
			}

			if err := json.Unmarshal(projectInfoByte, &projectInfoMap); err != nil {
				s.log.Error("!!!UserDefaultProject--->unmarshal projectInfo", logger.Error(err))
				continue
			}

			resourceEnvStruct, err := helper.ConvertMapToStruct(projectInfoMap)
			if err != nil {
				s.log.Error("!!!UserDefaultProject--->ConvertMapToStruct", logger.Error(err))
				continue
			}

			return &pb.UserDefaultProjectResp{
				ProjectId:     projectInfo.ProjectId,
				ClientTypeId:  clientType.ClientTypeIds[0],
				EnvironmentId: prodEnvId,
				ProjectData:   resourceEnvStruct,
				UserId:        user.GetId(),
			}, nil
		}
	}

	s.log.Error("!!!UserDefaultProject--->NoValidProjectFound", logger.Any("userId", user.GetId()))
	return nil, status.Error(codes.NotFound, "user project not found")
}

func (s *sessionService) V2Login(ctx context.Context, req *pb.V2LoginRequest) (*pb.V2LoginResponse, error) {
	s.log.Info("V2Login --> ", logger.Any("request: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2Login", req)
	defer dbSpan.Finish()

	var (
		user = &pb.User{}
		err  error
		data *pbObject.LoginDataRes
	)

	user, err = s.authenticateUser(ctx, req)
	if err != nil {
		return nil, err
	}

	if user == nil || len(user.GetId()) == 0 {
		return nil, errUserNotFound
	}

	reqLoginData := &pbObject.LoginDataReq{
		UserId:                user.GetId(),
		ClientType:            req.GetClientType(),
		ProjectId:             req.GetProjectId(),
		ResourceEnvironmentId: req.GetResourceEnvironmentId(),
	}

	var (
		userStatus string
		statusErr  error
		loginWg    sync.WaitGroup
	)

	loginWg.Add(1)
	go func() {
		defer loginWg.Done()
		userStatus, statusErr = s.strg.User().GetUserStatus(ctx, user.Id, req.GetProjectId())
	}()

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, req.NodeType)
	if err != nil {
		loginWg.Wait()
		return nil, err
	}

	switch req.ResourceType {
	case 1:
		data, err = services.GetLoginServiceByType(req.NodeType).LoginData(ctx, reqLoginData)
		if err != nil {
			errGetUserProjectData := errors.New("invalid user project data")
			s.log.Error("!!!Login--->", logger.Error(err))
			loginWg.Wait()
			return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
		}
	case 3:
		newReq := nb.LoginDataReq{}

		err = helper.MarshalToStruct(&reqLoginData, &newReq)
		if err != nil {
			s.log.Error("!!!Login--->", logger.Error(err))
			loginWg.Wait()
			return nil, status.Error(400, err.Error())
		}

		newReq.ProjectId = newReq.ResourceEnvironmentId

		newData, err := services.GoLoginService().LoginData(ctx, &newReq)
		if err != nil {
			errGetUserProjectData := errors.New("invalid user project data")
			s.log.Error("!!!PostgresBuilder.Login--->", logger.Error(err))
			loginWg.Wait()
			return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
		}

		err = helper.MarshalToStruct(&newData, &data)
		if err != nil {
			s.log.Error("!!!Login--->", logger.Error(err))
			loginWg.Wait()
			return nil, status.Error(400, err.Error())
		}
	}

	loginWg.Wait()

	if statusErr != nil {
		s.log.Error("!!!V2Login--->GetUserStatus", logger.Error(statusErr))
		return nil, status.Error(codes.Internal, statusErr.Error())
	}

	if userStatus == config.UserStatusBlocked {
		err := errors.New("user blocked")
		s.log.Error("!!!V2Login--->UserBlocked", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if !data.UserFound {
		s.log.Error("!!!Login--->", logger.Error(errUserNotFound))
		return nil, status.Error(codes.NotFound, errUserNotFound.Error())
	}

	userData, err := helper.ConvertStructToResponse(data.UserData)
	if err != nil {
		return nil, status.Error(400, err.Error())
	}

	delete(userData, "password")

	data.UserData, err = helper.ConvertMapToStruct(userData)
	if err != nil {
		return nil, status.Error(400, err.Error())
	}

	res := helper.ConvertPbToAnotherPb(&pbObject.V2LoginResponse{
		Role:             data.GetRole(),
		UserId:           data.GetUserId(),
		UserData:         data.GetUserData(),
		UserFound:        data.GetUserFound(),
		ClientType:       data.GetClientType(),
		UserIdAuth:       data.GetUserIdAuth(),
		Permissions:      data.GetPermissions(),
		ClientPlatform:   data.GetClientPlatform(),
		LoginTableSlug:   data.GetLoginTableSlug(),
		AppPermissions:   data.GetAppPermissions(),
		GlobalPermission: data.GetGlobalPermission(),
	})

	resp, err := s.SessionAndTokenGenerator(ctx, &pb.SessionAndTokenRequest{
		LoginData:     res,
		Tables:        req.Tables,
		ProjectId:     req.GetProjectId(),
		EnvironmentId: req.GetEnvironmentId(),
		ClientId:      req.GetClientId(),
		ClientIp:      req.GetClientIp(),
		UserAgent:     req.GetUserAgent(),
	})
	if resp == nil {
		errGenerateToken := errors.New("unable to generate token")
		s.log.Error("!!!Login--->", logger.Error(errGenerateToken))
		return nil, status.Error(codes.NotFound, errGenerateToken.Error())
	}
	if err != nil {
		errGenerateToken := errors.New("unable to generate token")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.Internal, errGenerateToken.Error())
	}

	if req.Tables != nil {
		res.Tables = req.Tables
	}

	return res, nil
}

func (s *sessionService) V2LoginWithOption(ctx context.Context, req *pb.V2LoginWithOptionRequest) (*pb.V2LoginWithOptionsResponse, error) {
	s.log.Info("V2LoginWithOption-->", logger.Any("request: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2LoginWithOption", req)
	defer dbSpan.Finish()

	var (
		before   runtime.MemStats
		userId   string
		verified bool
		user     *pb.User
		err      error
	)
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2LoginWithOption", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2LoginWithOption", memoryUsed))
		}
	}()

pwd:
	switch strings.ToUpper(req.GetLoginStrategy()) {
	case "LOGIN_PWD":
		username, ok := req.GetData()["username"]
		if ok {
			if len(username) < 6 {
				err := errors.New("invalid username")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		} else {
			err := errors.New("username is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		password, ok := req.GetData()["password"]
		if ok {
			if len(password) < 6 {
				err := errors.New("invalid password")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		} else {
			err := errors.New("password is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByUsername(ctx, username)
		if err != nil {
			s.log.Error("!!!V2V2LoginWithOption--->", logger.Error(err))
			if err == sql.ErrNoRows {
				errNoRows := errors.New("no user found")
				return nil, status.Error(codes.Internal, errNoRows.Error())
			}
			return nil, status.Error(codes.Internal, err.Error())
		}

		userId = user.Id
	case "PHONE":
		phone, ok := req.GetData()["phone"]
		if !ok {
			err := errors.New("phone is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByUsername(ctx, phone)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = user.GetId()
	case "EMAIL":
		email, ok := req.GetData()["email"]
		if !ok {
			err := errors.New("email is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByUsername(ctx, email)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = user.GetId()
	case "LOGIN":
		username, ok := req.GetData()["username"]
		if !ok {
			err := errors.New("username is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByUsername(ctx, username)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = user.GetId()
	case "PHONE_OTP":
		sms_id, ok := req.GetData()["sms_id"]
		if !ok {
			err := errors.New("sms_id is empty")
			s.log.Error("!!!V2LoginWithOption--->NoSMSId", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		otp, ok := req.GetData()["otp"]
		if !ok {
			err := errors.New("otp is empty")
			s.log.Error("!!!V2LoginWithOption--->NoOTP", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		phone, ok := req.GetData()["phone"]
		if !ok {
			err := errors.New("phone is empty")
			s.log.Error("!!!V2LoginWithOption--->NoPhone", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		smsOtpSettings, err := s.services.ResourceService().GetProjectResourceList(
			ctx, &pbCompany.GetProjectResourceListRequest{
				EnvironmentId: req.Data["environment_id"],
				ProjectId:     req.Data["project_id"],
				Type:          pbCompany.ResourceType_SMS,
			})
		if err != nil {
			s.log.Error("!!!V2LoginWithOption.SmsOtpSettingsService().GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		var defaultOtp string
		if len(smsOtpSettings.GetResources()) > 0 {
			if smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetDefaultOtp() != "" {
				defaultOtp = smsOtpSettings.GetResources()[0].GetSettings().GetSms().GetDefaultOtp()
			}
		}

		if defaultOtp != otp {
			_, err = s.services.SmsService().ConfirmOtp(
				ctx, &sms_service.ConfirmOtpRequest{
					SmsId: sms_id, Otp: otp,
				},
			)
			if err != nil {
				s.log.Error("!!!V2LoginWithOption--->ConfirmOTP", logger.Error(err))
				return nil, err
			}
		}
		verified = true

		user, err = s.strg.User().GetByUsername(ctx, phone)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->GetUserByUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = user.GetId()
	case "EMAIL_OTP":
		sms_id, ok := req.GetData()["sms_id"]
		if !ok {
			err := errors.New("sms_id is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		otp, ok := req.GetData()["otp"]
		if !ok {
			err := errors.New("otp is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		email, ok := req.GetData()["email"]
		if !ok {
			err := errors.New("email is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		emailOtpSettings, err := s.services.ResourceService().GetProjectResourceList(
			ctx, &pbCompany.GetProjectResourceListRequest{
				EnvironmentId: req.Data["environment_id"],
				ProjectId:     req.Data["project_id"],
				Type:          pbCompany.ResourceType_SMTP,
			},
		)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption.EmailtpSettingsService().GetList--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		var defaultOtp string
		if len(emailOtpSettings.GetResources()) > 0 {
			if emailOtpSettings.GetResources()[0].GetSettings().GetSmtp().GetDefaultOtp() != "" {
				defaultOtp = emailOtpSettings.GetResources()[0].GetSettings().GetSmtp().GetDefaultOtp()
			}
		}

		if otp != defaultOtp {
			_, err := s.services.SmsService().ConfirmOtp(
				ctx, &sms_service.ConfirmOtpRequest{
					SmsId: sms_id,
					Otp:   otp,
				},
			)
			if err != nil {
				return nil, err
			}
		}
		verified = true

		user, err = s.strg.User().GetByUsername(ctx, email)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		userId = user.GetId()
	case "PHONE_PWD":
		phone, ok := req.GetData()["phone"]
		if !ok {
			err := errors.New("phone is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		password, ok := req.GetData()["password"]
		if ok {
			if len(password) < 6 {
				err := errors.New("invalid password")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		} else {
			err := errors.New("password is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userIdRes, err := s.strg.User().GetByUsername(ctx, phone)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
			Id: userIdRes.GetId(),
		})
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = user.GetId()
	case "EMAIL_PWD":
		email, ok := req.GetData()["email"]
		if !ok {
			err := errors.New("email is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		password, ok := req.GetData()["password"]
		if ok {
			if len(password) < 6 {
				err := errors.New("invalid password")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		} else {
			err := errors.New("password is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userIdRes, err := s.strg.User().GetByUsername(ctx, email)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err := s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
			Id: userIdRes.GetId(),
		})
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = user.GetId()
	case "GOOGLE_AUTH":
		email, ok := req.GetData()["email"]
		if !ok {
			err := errors.New("email is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		if gooleToken, ok := req.GetData()["google_token"]; ok {
			userInfo, err := helper.GetGoogleUserInfo(gooleToken)
			if err != nil {
				s.log.Error("!!!V2LoginWithOption--->failed to get google user info---->", logger.Error(err))
				err = errors.New("invalid arguments google auth")
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			if userInfo["error"] != nil || !(userInfo["email_verified"].(bool)) {
				err = errors.New("invalid arguments google auth")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
		} else {
			err := errors.New("google token is required when login type is google auth")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userIdRes, err := s.strg.User().GetUserByLoginType(ctx, &pb.GetUserByLoginTypesRequest{
			Email: email,
		})
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = userIdRes.GetUserId()
	case "APPLE_AUTH":
		err := errors.New("not implemented")
		return nil, status.Error(codes.InvalidArgument, err.Error())
	case "E-IMZO":
		pkcs7, ok := req.GetData()["pkcs7"]
		if !ok {
			err := errors.New("pkcs7 is empty")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		extractResp, err := eimzo.ExtractUserFromPKCS7(s.cfg, pkcs7, req.ClientIp)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		if extractResp.Status != 1 {
			break
		}

		tin := extractResp.SubjectCertificateInfo.SubjectName.TIN
		tinValue := eimzo.ExtractFromX500(extractResp.SubjectCertificateInfo.X500Name, "1.2.860.3.16.1.1")
		if tinValue != "" {
			tin = tinValue
		}

		userIdRes, err := s.strg.User().GetByUsername(ctx, tin)
		if err != nil {
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		userId = userIdRes.GetId()
	default:
		req.LoginStrategy = "LOGIN_PWD"
		goto pwd
	}

	if user.GetId() != "" {
		userStatus, err := s.strg.User().GetUserStatus(ctx, user.Id, req.Data["project_id"])
		if err != nil {
			s.log.Error("!!!V2Login--->GetUserStatus", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		if userStatus == config.UserStatusBlocked {
			err := errors.New("user blocked")
			s.log.Error("!!!V2Login--->UserBlocked", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	req.Data["user_id"] = userId
	data, err := s.LoginMiddleware(ctx, models.LoginMiddlewareReq{
		Data:      req.Data,
		Tables:    req.Tables,
		ClientId:  req.ClientId,
		ClientIp:  req.GetClientIp(),
		UserAgent: req.GetUserAgent(),
	})
	if err != nil {
		var httpErrorStr = ""

		httpErrorStr = strings.Split(err.Error(), "=")[len(strings.Split(err.Error(), "="))-1][1:]
		httpErrorStr = strings.ToLower(httpErrorStr)

		if httpErrorStr == "user not found" && verified {
			err := errors.New("user verified but not found")
			s.log.Error("!!!V2LoginWithOption--->UserNotFound", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		s.log.Error("!!!V2LoginWithOption--->LoginMiddleware", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return data, nil
}

func (s *sessionService) LoginMiddleware(ctx context.Context, req models.LoginMiddlewareReq) (*pb.V2LoginWithOptionsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.LoginMiddleware", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the LoginMiddleware", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("LoginMiddleware", memoryUsed))
		}
	}()

	var res *pb.V2LoginResponse

	if req.Data["project_id"] != "" && req.Data["environment_id"] != "" {
		var data *pbObject.LoginDataRes

		serviceResource, err := s.services.ServiceResource().GetSingle(ctx, &pbCompany.GetSingleServiceResourceReq{
			EnvironmentId: req.Data["environment_id"],
			ProjectId:     req.Data["project_id"],
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		})
		if err != nil {
			errGetUserProjectData := errors.New("unable to get resource")
			s.log.Error("!!!LoginMiddleware--->LoginService()", logger.Error(err))
			return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
		}

		reqLoginData := &pbObject.LoginDataReq{
			UserId:                req.Data["user_id"],
			NodeType:              serviceResource.GetNodeType(),
			ProjectId:             req.Data["project_id"],
			ClientType:            req.Data["client_type_id"],
			ResourceEnvironmentId: serviceResource.GetResourceEnvironmentId(),
			Password:              req.Data["password"],
		}

		services, err := s.serviceNode.GetByNodeType(req.Data["project_id"], req.NodeType)
		if err != nil {
			s.log.Error("!!!LoginMiddleware--->GetByNodeType", logger.Error(err))
			return nil, err
		}

		switch serviceResource.ResourceType {
		case 1:
			data, err = services.GetLoginServiceByType(req.NodeType).LoginData(ctx, reqLoginData)
			if err != nil {
				errGetUserProjectData := errors.New("invalid user project data")
				s.log.Error("!!!LoginMiddleware--->LoginService()", logger.Error(err))
				return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
			}
		case 3:
			goReq := &nb.LoginDataReq{}

			err = helper.MarshalToStruct(reqLoginData, &goReq)
			if err != nil {
				s.log.Error("!!!LoginMiddleware--->PostgresMarshal2Struct", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}

			goData, err := services.GoLoginService().LoginData(ctx, goReq)
			if err != nil {
				errGetUserProjectData := errors.New("invalid user project data")
				s.log.Error("!!!LoginMiddleware--->PostgresLoginService", logger.Error(err))
				return nil, status.Error(codes.Internal, errGetUserProjectData.Error())
			}

			if err = helper.MarshalToStruct(goData, &data); err != nil {
				s.log.Error("!!!LoginMiddleware--->LoginDataMarshal", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}
		}

		if !data.UserFound {
			s.log.Error("!!!LoginMiddleware--->", logger.Error(errUserNotFound))
			return nil, status.Error(codes.NotFound, errUserNotFound.Error())
		}

		if !data.ComparePassword {
			err := errors.New("invalid password")
			s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
			return nil, err
		}

		res = helper.ConvertPbToAnotherPb(&pbObject.V2LoginResponse{
			Role:           data.GetRole(),
			UserId:         data.GetUserId(),
			UserData:       data.GetUserData(),
			UserFound:      data.GetUserFound(),
			UserIdAuth:     data.GetUserIdAuth(),
			ClientType:     data.GetClientType(),
			Permissions:    data.GetPermissions(),
			ClientPlatform: data.GetClientPlatform(),
			LoginTableSlug: data.GetLoginTableSlug(),
		})

		// Opt-in: when the caller sets `is_connections=true`, resolve the user's
		// connections for this client_type and seed `Tables` so the JWT carries
		// {table_slug, object_id} pairs used by the connection-based row filter
		// (see object_builder_service/pkg/helper/automaticFilter.go).
		if req.Data["is_connections"] == "true" {
			connTables, connErr := s.resolveConnectionTables(
				ctx, services, serviceResource,
				req.Data["client_type_id"],
				req.Data["user_id"],
				req.Data["connection_options"],
			)
			if connErr != nil {
				s.log.Error("!!!LoginMiddleware--->resolveConnectionTables", logger.Error(connErr))
				return nil, connErr
			}
			req.Tables = append(req.Tables, connTables...)
		}
	}
	if req.Tables == nil {
		req.Tables = []*pb.Object{}
	}

	resp, err := s.SessionAndTokenGenerator(ctx, &pb.SessionAndTokenRequest{
		Tables:        req.Tables,
		LoginData:     res,
		ProjectId:     req.Data["project_id"],
		EnvironmentId: req.Data["environment_id"],
		ClientId:      req.ClientId,
		ClientIp:      req.ClientIp,
		UserAgent:     req.UserAgent,
	})

	if resp == nil {
		err := errors.New("error while generating token")
		s.log.Error("!!!LoginMiddleware--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}
	if err != nil {
		s.log.Error("!!!LoginMiddleware--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.V2LoginWithOptionsResponse{
		User:            resp.GetUser(),
		Role:            resp.GetRole(),
		Token:           resp.GetToken(),
		Tables:          resp.GetTables(),
		UserId:          resp.GetUserId(),
		Sessions:        resp.GetSessions(),
		UserData:        res.GetUserData(),
		UserFound:       true,
		ResourceId:      resp.GetResourceId(),
		ClientType:      resp.GetClientType(),
		Permissions:     resp.GetPermissions(),
		EnvironmentId:   resp.GetEnvironmentId(),
		ClientPlatform:  resp.GetClientPlatform(),
		AppPermissions:  resp.GetAppPermissions(),
		LoginTableSlug:  resp.GetLoginTableSlug(),
		AddationalTable: resp.GetAddationalTable(),
	}, nil
}

// resolveConnectionTables loads the user's connections for the given client_type,
// fetches the matching options for each, and returns the {table_slug, object_id}
// pairs to embed in the JWT's `tables` claim.
//
//   - 0 options for a connection → skipped (nothing to filter on).
//   - 1 option → auto-picked.
//   - >1 options → caller must supply `connection_options` (JSON object
//     {table_slug: object_id}) to pick; otherwise FailedPrecondition with the
//     list of unresolved connections + their option ids.
func (s *sessionService) resolveConnectionTables(
	ctx context.Context,
	services client.SharedServiceManagerI,
	serviceResource *pbCompany.ServiceResourceModel,
	clientTypeId string,
	userId string,
	connectionOptionsRaw string,
) ([]*pb.Object, error) {
	choice := map[string]string{}
	if connectionOptionsRaw != "" {
		if err := json.Unmarshal([]byte(connectionOptionsRaw), &choice); err != nil {
			return nil, status.Error(codes.InvalidArgument,
				"connection_options must be a JSON object of {table_slug: object_id}")
		}
	}

	var rawConnections []any
	switch serviceResource.ResourceType {
	case pbCompany.ResourceType_MONGODB:
		structData, err := helper.ConvertMapToStruct(map[string]any{
			"limit":          100,
			"offset":         0,
			"client_type_id": clientTypeId,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		listResp, err := services.GetObjectBuilderServiceByType(serviceResource.GetNodeType()).GetList(ctx, &pbObject.CommonMessage{
			TableSlug: "connections",
			ProjectId: serviceResource.GetResourceEnvironmentId(),
			Data:      structData,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		rawConnections, _ = listResp.GetData().AsMap()["response"].([]any)
	case pbCompany.ResourceType_POSTGRESQL:
		structData, err := helper.ConvertMapToStruct(map[string]any{
			"limit":                     100,
			"offset":                    0,
			"client_type_id_from_token": clientTypeId,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		listResp, err := services.GoObjectBuilderService().GetList(ctx, &nb.CommonMessage{
			TableSlug: "connections",
			ProjectId: serviceResource.GetResourceEnvironmentId(),
			Data:      structData,
		})
		if err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
		rawConnections, _ = listResp.GetData().AsMap()["response"].([]any)
	}

	type ambig struct {
		TableSlug string   `json:"table_slug"`
		Options   []string `json:"options"`
	}

	var (
		tables    []*pb.Object
		ambiguous []ambig
		fetchErr  error
		mu        sync.Mutex
		wg        sync.WaitGroup
	)

	for _, raw := range rawConnections {
		connMap, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		guid, _ := connMap["guid"].(string)
		tableSlug, _ := connMap["table_slug"].(string)
		if guid == "" || tableSlug == "" {
			continue
		}

		wg.Add(1)
		go func(connectionId, slug string) {
			defer wg.Done()

			var optionsData any
			switch serviceResource.ResourceType {
			case pbCompany.ResourceType_MONGODB:
				optResp, err := services.GetLoginServiceByType(serviceResource.GetNodeType()).GetConnetionOptions(ctx, &pbObject.GetConnetionOptionsRequest{
					ConnectionId:          connectionId,
					ResourceEnvironmentId: serviceResource.GetResourceEnvironmentId(),
					UserId:                userId,
				})
				if err != nil {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					mu.Unlock()
					return
				}
				if optResp.GetData() != nil {
					optionsData = optResp.GetData().AsMap()["response"]
				}
			case pbCompany.ResourceType_POSTGRESQL:
				optResp, err := services.GoLoginService().GetConnetionOptions(ctx, &nb.GetConnetionOptionsRequest{
					ConnectionId:          connectionId,
					ResourceEnvironmentId: serviceResource.GetResourceEnvironmentId(),
					UserId:                userId,
					ProjectId:             serviceResource.GetProjectId(),
				})
				if err != nil {
					mu.Lock()
					if fetchErr == nil {
						fetchErr = err
					}
					mu.Unlock()
					return
				}
				if optResp.GetData() != nil {
					optionsData = optResp.GetData().AsMap()["response"]
				}
			}

			options, _ := optionsData.([]any)
			optIds := make([]string, 0, len(options))
			for _, o := range options {
				if om, _ := o.(map[string]any); om != nil {
					if id, _ := om["guid"].(string); id != "" {
						optIds = append(optIds, id)
					}
				}
			}

			mu.Lock()
			defer mu.Unlock()

			switch len(optIds) {
			case 0:
			case 1:
				tables = append(tables, &pb.Object{
					TableSlug: slug,
					ObjectId:  optIds[0],
				})
			default:
				if chosen := choice[slug]; chosen != "" {
					for _, id := range optIds {
						if id == chosen {
							tables = append(tables, &pb.Object{
								TableSlug: slug,
								ObjectId:  chosen,
							})
							return
						}
					}
				}
				ambiguous = append(ambiguous, ambig{TableSlug: slug, Options: optIds})
			}
		}(guid, tableSlug)
	}
	wg.Wait()

	if fetchErr != nil {
		return nil, status.Error(codes.Internal, fetchErr.Error())
	}
	if len(ambiguous) > 0 {
		payload, _ := json.Marshal(ambiguous)
		return nil, status.Errorf(codes.FailedPrecondition,
			"connection_options required for ambiguous connections: %s", string(payload))
	}

	return tables, nil
}

func (s *sessionService) V2RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.V2LoginResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2RefreshToken", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2RefreshToken", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2RefreshToken", memoryUsed))
		}
	}()

	tokenInfo, err := security.ParseClaims(req.RefreshToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->ParseClaims", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	session, err := s.strg.Session().GetByPK(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!RefreshToken--->SessionGetByPK", logger.Error(err))
		return nil, status.Error(codes.Code(status_http.Unauthorized.Code), err.Error())
	}
	if req.ClientTypeId != "" {
		session.ClientTypeId = req.ClientTypeId
	}
	if req.ProjectId != "" {
		session.ProjectId = req.ProjectId
	}
	if req.RoleId != "" {
		session.RoleId = req.RoleId
	}
	if req.EnvId != "" {
		session.EnvId = req.EnvId
	}

	expiresAt, err := time.Parse(config.DatabaseTimeLayout, session.ExpiresAt)
	if err != nil {
		s.log.Error("!!!RefreshToken--->ParseExpiresAt", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if expiresAt.Unix() < time.Now().Unix() {
		err := errors.New("session has been expired")
		s.log.Error("!!!V2HasAccessUser->CheckExpiredToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_, err = s.strg.User().CHeckUserProject(ctx, session.GetUserIdAuth(), session.GetProjectId())
	if err != nil {
		s.log.Error("!!!V2Login--->CHeckUserProject", logger.Error(err))
		if err == sql.ErrNoRows {
			errNoRows := errors.New("no user found")
			return nil, status.Error(codes.Internal, errNoRows.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	_, err = s.strg.Session().Update(ctx, &pb.UpdateSessionRequest{
		Id:               session.Id,
		Ip:               session.Ip,
		Data:             session.Data,
		EnvId:            session.EnvId,
		UserId:           session.UserId,
		RoleId:           session.RoleId,
		ProjectId:        session.ProjectId,
		IsChanged:        session.IsChanged,
		ExpiresAt:        time.Now().Add(config.RefreshTokenExpiresInTime).Format(config.DatabaseTimeLayout),
		ClientTypeId:     session.ClientTypeId,
		ClientPlatformId: session.ClientPlatformId,
	})
	if err != nil {
		s.log.Error("!!!V2RefreshToken.SessionUpdate--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var authTables []*pb.TableBody

	if tokenInfo.Tables != nil {
		for _, table := range tokenInfo.Tables {
			authTables = append(authTables, &pb.TableBody{
				TableSlug: table.TableSlug,
				ObjectId:  table.ObjectID,
			})
		}
	}

	// TODO - wrap in a function
	m := map[string]any{
		"id":                 session.Id,
		"ip":                 session.Ip,
		"data":               session.Data,
		"tables":             authTables,
		"user_id":            session.UserId,
		"role_id":            session.RoleId,
		"project_id":         session.ProjectId,
		"user_id_auth":       session.UserIdAuth,
		"client_type_id":     session.ClientTypeId,
		"login_table_slug":   tokenInfo.LoginTableSlug,
		"client_platform_id": session.ClientPlatformId,
	}

	accessToken, err := security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->GenerateAccessJWT", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	refreshToken, err := security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->GenerateRefreshJWT", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	res := &pb.V2LoginResponse{
		Token: &pb.Token{
			AccessToken:      accessToken,
			RefreshToken:     refreshToken,
			CreatedAt:        session.CreatedAt,
			UpdatedAt:        session.UpdatedAt,
			ExpiresAt:        time.Now().Add(config.AccessTokenExpiresInTime).Format(config.DatabaseTimeLayout),
			RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
		},
	}

	return res, nil
}

func (s *sessionService) SessionAndTokenGenerator(ctx context.Context, input *pb.SessionAndTokenRequest) (*pb.V2LoginResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.SessionAndTokenGenerator", input)
	defer dbSpan.Finish()

	if _, err := uuid.Parse(input.GetLoginData().GetUserIdAuth()); err != nil {
		err := errors.New("INVALID USER_ID(UUID)" + err.Error())
		s.log.Error("!!!TokenGenerator->UserIdAuthExist-->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var (
		userSessionList *pb.GetSessionListResponse
		deleteErr       error
		listErr         error
		sessionWg       sync.WaitGroup
	)

	sessionWg.Add(2)
	go func() {
		defer sessionWg.Done()
		_, deleteErr = s.strg.Session().DeleteExpiredUserSessions(ctx, input.GetLoginData().GetUserIdAuth())
	}()
	go func() {
		defer sessionWg.Done()
		userSessionList, listErr = s.strg.Session().GetSessionListByUserID(ctx, input.GetLoginData().GetUserIdAuth())
	}()
	sessionWg.Wait()

	if deleteErr != nil {
		s.log.Error("!!!SessionAndTokenGenerator--->DeleteExpiredUserSessions", logger.Error(deleteErr))
		return nil, status.Error(codes.InvalidArgument, deleteErr.Error())
	}
	if listErr != nil {
		s.log.Error("!!!SessionAndTokenGenerator--->GetSessionListByUserID", logger.Error(listErr))
		return nil, status.Error(codes.InvalidArgument, listErr.Error())
	}

	input.LoginData.Sessions = userSessionList.GetSessions()

	_, err := uuid.Parse(input.GetProjectId())
	if err != nil {
		err = errors.New("project id is invalid")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	sessionPKey, err := s.strg.Session().Create(ctx, &pb.CreateSessionRequest{
		Ip:               input.GetClientIp(),
		Data:             input.GetUserAgent(),
		EnvId:            input.GetEnvironmentId(),
		UserId:           input.GetLoginData().GetUserId(),
		RoleId:           input.GetLoginData().GetRole().GetId(),
		ClientId:         input.GetClientId(),
		ProjectId:        input.GetProjectId(),
		ExpiresAt:        time.Now().Add(config.RefreshTokenExpiresInTime).Format(config.DatabaseTimeLayout),
		UserIdAuth:       input.GetLoginData().GetUserIdAuth(),
		ClientTypeId:     input.GetLoginData().GetClientType().GetId(),
		ClientPlatformId: input.GetLoginData().GetClientPlatform().GetId(),
		SessionLimit:     input.GetLoginData().GetClientType().GetSessionLimit(),
	})
	if err != nil {
		s.log.Error("!!!Create--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	// Session.GetByPK and User.GetByPK are independent — run in parallel
	var (
		session    *pb.Session
		userData   *pb.User
		sessionErr error
		userErr    error
		pWg        sync.WaitGroup
	)

	pWg.Add(2)
	go func() {
		defer pWg.Done()
		session, sessionErr = s.strg.Session().GetByPK(ctx, sessionPKey)
	}()
	go func() {
		defer pWg.Done()
		userData, userErr = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{
			ProjectId: input.GetProjectId(),
			Id:        input.GetLoginData().GetUserIdAuth(),
		})
	}()
	pWg.Wait()

	if sessionErr != nil {
		s.log.Error("!!!GetByPK--->", logger.Error(sessionErr))
		return nil, status.Error(codes.Internal, sessionErr.Error())
	}
	if userErr != nil {
		s.log.Error("!!!Login->GetByPK--->", logger.Error(userErr))
		return nil, status.Error(codes.Internal, userErr.Error())
	}

	if input.Tables == nil {
		input.Tables = []*pb.Object{}
	}

	m := map[string]any{
		"id":                 session.GetId(),
		"ip":                 session.GetIp(),
		"data":               session.GetData(),
		"tables":             input.GetTables(),
		"user_id":            session.GetUserId(),
		"role_id":            session.GetRoleId(),
		"client_id":          session.GetClientId(),
		"project_id":         session.GetProjectId(),
		"user_id_auth":       session.GetUserIdAuth(),
		"client_type_id":     session.GetClientTypeId(),
		"login_table_slug":   input.GetLoginData().GetLoginTableSlug(),
		"client_platform_id": session.GetClientPlatformId(),
	}

	// Generate access and refresh tokens in parallel
	var (
		accessToken  string
		refreshToken string
		accessErr    error
		refreshErr   error
		jwtWg        sync.WaitGroup
	)

	jwtWg.Add(2)
	go func() {
		defer jwtWg.Done()
		accessToken, accessErr = security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	}()
	go func() {
		defer jwtWg.Done()
		refreshToken, refreshErr = security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	}()
	jwtWg.Wait()

	if accessErr != nil {
		s.log.Error("!!!Login--->", logger.Error(accessErr))
		return nil, status.Error(codes.Internal, accessErr.Error())
	}
	if refreshErr != nil {
		s.log.Error("!!!Login--->", logger.Error(refreshErr))
		return nil, status.Error(codes.Internal, refreshErr.Error())
	}

	go func() {
		err = s.strg.ApiKeys().CreateClientToken(context.Background(), input.ClientId, m)
	}()

	input.LoginData.Token = &pb.Token{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		CreatedAt:        session.GetCreatedAt(),
		UpdatedAt:        session.GetUpdatedAt(),
		ExpiresAt:        session.GetExpiresAt(),
		RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
	}
	input.LoginData.User = userData

	return input.LoginData, nil
}

func (s *sessionService) V2HasAccessUser(ctx context.Context, req *pb.V2HasAccessUserReq) (*pb.V2HasAccessUserRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2HasAccessUser", req)
	defer dbSpan.Finish()

	var (
		before                 runtime.MemStats
		arr_path               = strings.Split(req.Path, "/")
		methodField            string
		exist, checkPermission bool
		authTables             []*pb.TableBody
		tableSlug              string = req.GetTableSlug()
	)
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2HasAccessUser", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2HasAccessUser", memoryUsed))
		}
	}()

	tokenInfo, err := security.ParseClaims(req.AccessToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!V2HasAccessUser->ParseClaims--->", logger.Error(err))
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	if tokenInfo.ClientID != "" {
		stats, err := s.strg.ApiKeys().CheckClientIdStatus(ctx, tokenInfo.ClientID)
		if err != nil {
			s.log.Error("!!!V2HasAccessUser->CheckClientIdStatus--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if !stats {
			err = config.ErrInactiveClientId
			s.log.Error("!!!V2HasAccessUser->InactiveClientId--->", logger.Error(err))
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}
	}

	session, err := s.strg.Session().GetByPK(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!V2HasAccessUser->GetByPK--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	expiresAt, err := time.Parse(config.DatabaseTimeLayout, session.ExpiresAt)
	if err != nil {
		s.log.Error("!!!V2HasAccessUser->TimeParse--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if expiresAt.Unix() < time.Now().Unix() {
		err := errors.New("session has been expired")
		s.log.Error("!!!V2HasAccessUser->CheckExpiredToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	s.touchSessionActivity(session)

	// User activity
	go func() {
		var userActivityReq = &nb.UserActivityReqeust{
			LoginTable: tokenInfo.LoginTableSlug,
			UserId:     tokenInfo.UserId,
		}

		s.UserActivity(session, userActivityReq)
	}()

	switch req.Method {
	case http.MethodGet:
		methodField = config.READ
	case http.MethodPost:
		methodField = config.WRITE
	case http.MethodPut:
		methodField = config.UPDATE
	case http.MethodDelete:
		methodField = config.DELETE
	}
	// this condition need our object/get-list api because this api's method is post we change it to get
	// this condition need our object/get-list-group-by and object/get-group-by-field api because this api's method is post we change it to get
	if ((strings.Contains(req.GetPath(), "object/get-list")) ||
		(strings.Contains(req.GetPath(), "object/get-list-group-by")) ||
		(strings.Contains(req.GetPath(), "object/get-group-by-field"))) ||
		(strings.Contains(req.GetPath(), "items/list")) && req.GetMethod() != http.MethodGet {
		methodField = config.READ
	}

	projects, err := s.services.UserService().GetProjectsByUserId(ctx, &pb.GetProjectsByUserIdReq{
		UserId: session.GetUserIdAuth(),
	})
	if err != nil {
		s.log.Error("---V2HasAccessUser->GetProjectsByUserId--->", logger.Error(err))
		return nil, err
	}
	for _, item := range projects.GetUserProjects() {
		if item.ProjectId == session.GetProjectId() {
			exist = true
			break
		}
	}
	if !exist {
		err = errors.New("---V2HasAccessUser->Access denied")
		s.log.Error("---V2HasAccessUser--->AccessDenied--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if req.EnvironmentId != "" {
		exist = false
		for _, item := range projects.GetUserProjects() {
			if item.EnvId == req.EnvironmentId {
				exist = true
				break
			}
		}

		if !exist {
			err = errors.New("user not access environment")
			s.log.Error("---V2HasAccessUser--->AccessNotEnvironment--->", logger.Error(err))
			return nil, status.Error(codes.Unavailable, err.Error())
		}
	}

	for _, path := range arr_path {
		if val, exist := config.Path[path]; exist {
			checkPermission = val
		}
	}

	if config.SystemTableSlugs[tableSlug] {
		checkPermission = false
	}

	if checkPermission {
		resource, err := s.services.ServiceResource().GetSingle(ctx,
			&pbCompany.GetSingleServiceResourceReq{
				ProjectId:     session.ProjectId,
				EnvironmentId: session.EnvId,
				ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
			},
		)
		if err != nil {
			s.log.Error("!!!V2HasAccessUser->GetSingleServiceResource--->", logger.Error(err))
			return nil, err
		}

		if config.IsProjectStatusBlocking(resource.GetProjectStatus()) && methodField != config.READ {
			return nil, status.Error(codes.PermissionDenied, resource.GetProjectStatus())
		}

		services, err := s.serviceNode.GetByNodeType(resource.ProjectId, resource.NodeType)
		if err != nil {
			return nil, err
		}

		switch resource.ResourceType {
		case pbCompany.ResourceType_MONGODB:
			resp, err := services.GetBuilderPermissionServiceByType(resource.NodeType).GetTablePermission(ctx,
				&pbObject.GetTablePermissionRequest{
					TableSlug:             tableSlug,
					RoleId:                session.RoleId,
					ResourceEnvironmentId: resource.ResourceEnvironmentId,
					Method:                methodField,
				},
			)
			if err != nil {
				s.log.Error("!!!V2HasAccessUser->GetTablePermission--->", logger.Error(err))
				return nil, err
			}

			if !resp.IsHavePermission {
				err := status.Error(codes.PermissionDenied, config.PermissionDenied)
				return nil, err
			}
		case pbCompany.ResourceType_POSTGRESQL:
			resp, err := services.GoObjectBuilderPermissionService().GetTablePermission(ctx,
				&nb.GetTablePermissionRequest{
					TableSlug:             tableSlug,
					RoleId:                session.RoleId,
					ResourceEnvironmentId: resource.ResourceEnvironmentId,
					Method:                methodField,
				},
			)
			if err != nil {
				s.log.Error("!!!V2HasAccessUser->GetTablePermission--->", logger.Error(err))
				return nil, err
			}

			if !resp.IsHavePermission {
				err := status.Error(codes.PermissionDenied, config.PermissionDenied)
				return nil, err
			}
		}
	}

	for _, table := range tokenInfo.Tables {
		authTables = append(authTables, &pb.TableBody{
			TableSlug: table.TableSlug,
			ObjectId:  table.ObjectID,
		})
	}

	return &pb.V2HasAccessUserRes{
		Id:               session.Id,
		Ip:               session.Ip,
		Data:             session.Data,
		EnvId:            session.EnvId,
		UserId:           session.UserId,
		RoleId:           session.RoleId,
		Tables:           authTables,
		ProjectId:        session.ProjectId,
		ExpiresAt:        session.ExpiresAt,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
		UserIdAuth:       session.UserIdAuth,
		ClientTypeId:     session.ClientTypeId,
		ClientPlatformId: session.ClientPlatformId,
	}, nil
}

func (s *sessionService) V2MultiCompanyOneLogin(ctx context.Context, req *pb.V2MultiCompanyLoginReq) (*pb.V2MultiCompanyOneLoginRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2MultiCompanyOneLogin", req)
	defer dbSpan.Finish()

	var (
		before runtime.MemStats
		user   = &pb.User{}
		err    error
		resp   = pb.V2MultiCompanyOneLoginRes{Companies: []*pb.Company2{}}
	)

	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2MultiCompanyOneLogin", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2MultiCompanyOneLogin", memoryUsed))
		}
	}()

	switch req.Type {
	case config.Default:
		if len(req.Username) < 6 {
			err := errors.New("invalid username")
			s.log.Error("!!!MultiCompanyLogin--->InvalidUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, config.ErrIncorrectLoginOrPassword)
		}

		if len(req.Password) < 6 {
			err := errors.New("invalid password")
			s.log.Error("!!!MultiCompanyLogin--->InvalidPassword", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, config.ErrIncorrectLoginOrPassword)
		}

		user, err = s.strg.User().GetByUsername(ctx, req.GetUsername())
		if err != nil {
			s.log.Error("!!!MultiCompanyLogin--->UserGetByUsername", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		hashType := user.GetHashType()
		switch config.HashTypes[hashType] {
		case 1:
			match, err := security.ComparePassword(user.GetPassword(), req.Password)
			if err != nil {
				s.log.Error("!!!MultiCompanyLogin-->ComparePasswordArgon", logger.Error(err))
				return nil, err
			}
			if !match {
				err := errors.New("username or password is wrong")
				s.log.Error("!!!MultiCompanyOneLogin-->Wrong", logger.Error(err))
				return nil, err
			}

			go func() {
				hashedPassword, err := security.HashPasswordBcrypt(req.Password)
				if err != nil {
					s.log.Error("!!!MultiCompanyOneLogin--->HashPasswordBcryptGo", logger.Error(err))
					return
				}
				err = s.strg.User().UpdatePassword(context.Background(), user.Id, hashedPassword)
				if err != nil {
					s.log.Error("!!!MultiCompanyOneLogin--->HashPasswordBcryptGo", logger.Error(err))
					return
				}
			}()
		case 2:
			match, err := security.ComparePasswordBcrypt(user.GetPassword(), req.Password)
			if err != nil {
				s.log.Error("!!!MultiCompanyOneLogin-->ComparePasswordBcrypt", logger.Error(err))
				return nil, status.Error(codes.Internal, config.ErrIncorrectLoginOrPassword)
			}
			if !match {
				err := errors.New("username or password is wrong")
				s.log.Error("!!!MultiCompanyOneLogin--->", logger.Error(err))
				return nil, status.Error(codes.Internal, config.ErrIncorrectLoginOrPassword)
			}
		default:
			err := config.ErrUserNotFound
			s.log.Error("!!!MultiCompanyOneLogin--->", logger.Error(err))
			return nil, status.Error(codes.Internal, config.ErrIncorrectLoginOrPassword)
		}
	case config.WithPhone:
		if req.ServiceType == "firebase" {
			err = firebase.VerifyPhoneCode(s.cfg, req.GetSessionInfo(), req.GetOtp())
			if err != nil {
				return nil, err
			}
		} else if config.DefaultOtp != req.Otp {
			_, err := s.services.SmsService().ConfirmOtp(ctx, &sms_service.ConfirmOtpRequest{
				SmsId: req.GetSmsId(),
				Otp:   req.GetOtp(),
			})
			if err != nil {
				return nil, err
			}
		}

		user, err = s.strg.User().GetByUsername(ctx, req.GetPhone())
		if err != nil {
			s.log.Error("!!!MultiCompanyLogin Phone--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case config.WithEmail:
		if config.DefaultOtp != req.Otp {
			_, err := s.services.SmsService().ConfirmOtp(ctx, &sms_service.ConfirmOtpRequest{
				SmsId: req.GetSmsId(),
				Otp:   req.GetOtp(),
			})
			if err != nil {
				return nil, err
			}
		}

		user, err = s.strg.User().GetByUsername(ctx, req.GetEmail())
		if err != nil {
			s.log.Error("!!!MultiCompanyLogin Email--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case config.WithGoogle:
		var email string
		if req.GetGoogleToken() != "" {
			userInfo, err := helper.GetGoogleUserInfo(req.GoogleToken)
			if err != nil {
				err = errors.New("invalid arguments google auth")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			if userInfo["error"] != nil || !(userInfo["email_verified"].(bool)) {
				err = errors.New("invalid arguments google auth")
				s.log.Error("!!!V2LoginWithOption--->", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			email = cast.ToString(userInfo["email"])
		} else {
			err := errors.New("google token is required when login type is google auth")
			s.log.Error("!!!MultiCompanyOneLogin--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		user, err = s.strg.User().GetByUsername(ctx, email)
		if err != nil {
			s.log.Error("!!!MultiCompanyOneLogin Email--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		if user.Id == "" {
			err = errors.New(config.ErrGoogle)
			s.log.Error("!!!MultiCompanyOneLogin--->", logger.Error(err))
			return nil, err
		}
	}
	userProjects, err := s.strg.User().GetUserProjects(ctx, user.GetId())
	if err != nil {
		errGetProjects := errors.New("cant get user projects")
		s.log.Error("!!!MultiCompanyLogin--->GetUserProjects", logger.Error(err))
		return nil, status.Error(codes.NotFound, errGetProjects.Error())
	}

	userEnvProject, err := s.strg.User().GetUserEnvProjects(ctx, user.GetId())
	if err != nil {
		errGetEnvProjects := errors.New("cant get user env projects")
		s.log.Error("!!!MultiCompanyLogin--->GetUserEnvProjects", logger.Error(err))
		return nil, status.Error(codes.NotFound, errGetEnvProjects.Error())
	}

	for _, item := range userProjects.Companies {
		projects := make([]*pb.Project2, 0, 20)
		company, err := s.services.CompanyServiceClient().GetById(ctx, &pbCompany.GetCompanyByIdRequest{
			Id: item.Id,
		})
		if err != nil {
			errGetProjects := errors.New("cant get user projects")
			s.log.Error("!!!MultiCompanyLogin--->CompanyGetById", logger.Error(err))
			return nil, status.Error(codes.NotFound, errGetProjects.Error())
		}

		for _, projectId := range item.ProjectIds {
			clientType, _ := s.strg.User().GetUserProjectClientTypes(ctx,
				&pb.UserInfoPrimaryKey{UserId: user.GetId(), ProjectId: projectId})

			projectInfo, err := s.services.ProjectServiceClient().GetById(ctx, &pbCompany.GetProjectByIdRequest{
				ProjectId: projectId, CompanyId: item.Id,
			})
			if err != nil {
				errGetProjects := errors.New("cant get user projects")
				s.log.Error("!!!MultiCompanyLogin---->ProjectInfo", logger.Error(err))
				return nil, status.Error(codes.NotFound, errGetProjects.Error())
			}

			resProject := &pb.Project2{
				Id:         projectInfo.GetProjectId(),
				CompanyId:  projectInfo.GetCompanyId(),
				Name:       projectInfo.GetTitle(),
				Domain:     projectInfo.GetK8SNamespace(),
				NewDesign:  projectInfo.GetNewDesign(),
				Status:     projectInfo.GetStatus(),
				ExpireDate: projectInfo.GetExpireDate(),
				NewLayout:  projectInfo.GetNewLayout(),
				NewRouter:  projectInfo.GetNewRouter(),
			}

			currencienJson, err := json.Marshal(projectInfo.GetCurrencies())
			if err != nil {
				errGetProjects := errors.New("cant get currencies")
				s.log.Error("!!!MultiCompanyLogin--->Currencies", logger.Error(err))
				return nil, status.Error(codes.NotFound, errGetProjects.Error())
			}

			err = json.Unmarshal(currencienJson, &resProject.Currencies)
			if err != nil {
				errGetProjects := errors.New("cant get currencies")
				s.log.Error("!!!MultiCompanyLogin--->Currencies", logger.Error(err))
				return nil, status.Error(codes.NotFound, errGetProjects.Error())
			}

			environments, err := s.services.EnvironmentService().GetList(ctx,
				&pbCompany.GetEnvironmentListRequest{
					Ids:       userEnvProject.EnvProjects[projectId],
					Limit:     1000,
					ProjectId: projectId,
				},
			)
			if err != nil {
				errGetProjects := errors.New("cant get environments")
				s.log.Error("!!!MultiCompanyLogin--->EnvironmentsList", logger.Error(err))
				return nil, status.Error(codes.NotFound, errGetProjects.Error())
			}

			for _, en := range environments.Environments {
				resourceEnv, err := s.services.ServiceResource().GetList(ctx,
					&pbCompany.GetListServiceResourceReq{
						ProjectId:     projectId,
						EnvironmentId: en.Id,
					},
				)
				if err != nil {
					errGetProjects := errors.New("cant get resourse environments")
					s.log.Error("!!!MultiCompanyLogin--->ServiceResourceList", logger.Error(err))
					return nil, status.Error(codes.NotFound, errGetProjects.Error())
				}

				respResourceEnvironment := &pb.ResourceEnvironmentV2MultiCompany{
					Id:            resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceEnvironmentId,
					Name:          en.Name,
					ProjectId:     en.ProjectId,
					AccessType:    en.AccessType,
					ResourceId:    resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceId,
					ServiceType:   int32(resourceEnv.ServiceResources[config.ObjectBuilderService].ServiceType.Number()),
					Description:   en.Description,
					IsConfigured:  true,
					ResourceType:  int32(resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceType.Number()),
					DisplayColor:  en.DisplayColor,
					EnvironmentId: en.Id,
				}

				if resourceEnv.ServiceResources[config.ObjectBuilderService] == nil || resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceEnvironmentId == "" {
					continue
				}

				if clientType == nil || len(clientType.ClientTypeIds) == 0 {
					clientTypes, err := s.services.ClientService().V2GetClientTypeList(ctx,
						&pb.V2GetClientTypeListRequest{
							ProjectId:              resourceEnv.ServiceResources[config.ObjectBuilderService].ProjectId,
							ResourceType:           int32(resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceType.Number()),
							NodeType:               resourceEnv.ServiceResources[config.ObjectBuilderService].NodeType,
							ResourceEnvrironmentId: resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceEnvironmentId,
						},
					)
					if err != nil {
						errGetProjects := errors.New("cant get client types")
						s.log.Error("!!!MultiCompanyLogin--->ClientTypes", logger.Error(err))
						return nil, status.Error(codes.NotFound, errGetProjects.Error())
					}

					respResourceEnvironment.ClientTypes = clientTypes.Data
				} else if len(clientType.ClientTypeIds) > 0 {
					clientTypes, err := s.services.ClientService().V2GetClientTypeList(ctx,
						&pb.V2GetClientTypeListRequest{
							ProjectId:              resourceEnv.ServiceResources[config.ObjectBuilderService].ProjectId,
							ResourceType:           int32(resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceType.Number()),
							Guids:                  clientType.ClientTypeIds,
							NodeType:               resourceEnv.ServiceResources[config.ObjectBuilderService].NodeType,
							ResourceEnvrironmentId: resourceEnv.ServiceResources[config.ObjectBuilderService].ResourceEnvironmentId,
						},
					)
					if err != nil {
						errGetProjects := errors.New("cant get client types")
						s.log.Error("!!!MultiCompanyLogin--->ClientTypes2", logger.Error(err))
						return nil, status.Error(codes.NotFound, errGetProjects.Error())
					}

					respResourceEnvironment.ClientTypes = clientTypes.Data
				}

				resProject.ResourceEnvironments = append(resProject.ResourceEnvironments, respResourceEnvironment)
			}

			projects = append(projects, resProject)
		}

		resp.Companies = append(resp.Companies, &pb.Company2{
			Id:          company.GetCompany().GetId(),
			Name:        company.GetCompany().GetName(),
			Logo:        company.GetCompany().GetLogo(),
			Description: company.GetCompany().GetLogo(),
			OwnerId:     company.GetCompany().GetOwnerId(),
			Projects:    projects,
		})
	}
	resp.UserId = user.Id

	return &resp, nil
}

func (s *sessionService) V2ResetPassword(ctx context.Context, req *pb.V2ResetPasswordRequest) (*pb.User, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2ResetPassword", req)
	defer dbSpan.Finish()

	s.log.Info("V2ResetPassword -> ", logger.Any("req: ", req))

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2ResetPassword", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2ResetPassword", memoryUsed))
		}
	}()

	if req.GetPassword() != "" {
		if len(req.GetPassword()) < 6 {
			err := fmt.Errorf("password must not be less than 6 characters")
			s.log.Error("!!!ResetPassword-->PasswordCheck", logger.Error(err))
			return nil, err
		}

		hashedPassword, err := security.HashPasswordBcrypt(req.GetPassword())
		if err != nil {
			s.log.Error("!!!ResetPassword-->HashPassword", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		req.Password = hashedPassword
	}
	rowsAffected, err := s.strg.User().V2ResetPassword(ctx, req)
	if err != nil {
		s.log.Error("!!!ResetPassword-->V2ResetPassword", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if rowsAffected <= 0 {
		return nil, status.Error(codes.InvalidArgument, "no rows were affected")
	}

	return s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{Id: req.GetUserId()})
}

func (s *sessionService) V2RefreshTokenForEnv(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.V2LoginResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2RefreshTokenForEnv", req)
	defer dbSpan.Finish()

	var (
		before runtime.MemStats
		data   = &pbObject.LoginDataRes{}
		roleId string
	)
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2RefreshTokenForEnv", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2RefreshTokenForEnv", memoryUsed))
		}
	}()

	tokenInfo, err := security.ParseClaims(req.RefreshToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	session, err := s.strg.Session().GetByPK(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}
	if req.ClientTypeId != "" {
		session.ClientTypeId = req.ClientTypeId
	}
	if req.ProjectId != "" {
		session.ProjectId = req.ProjectId
	}
	if req.RoleId != "" {
		session.RoleId = req.RoleId
	}
	if req.EnvId != "" {
		session.EnvId = req.EnvId
	}

	resource, err := s.services.ServiceResource().GetSingle(ctx,
		&pbCompany.GetSingleServiceResourceReq{
			ProjectId:     session.ProjectId,
			EnvironmentId: req.EnvId,
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		s.log.Error("!!!V2Refresh.SessionService().GetServiceResource--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(resource.ProjectId, resource.NodeType)
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv.ServiceNode", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	clientTypeId, err := s.strg.User().GetUserProjectByUserIdProjectIdEnvId(ctx, session.GetUserIdAuth(), req.GetProjectId(), req.GetEnvId())
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv.ClientType", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	reqLoginData := &pbObject.LoginDataReq{
		UserId:                session.GetUserIdAuth(),
		ClientType:            clientTypeId,
		ProjectId:             req.GetProjectId(),
		ResourceEnvironmentId: resource.GetResourceEnvironmentId(),
	}

	switch resource.ResourceType {
	case 1:
		data, err = services.GetLoginServiceByType(resource.NodeType).LoginData(ctx, reqLoginData)
		if err != nil {
			s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case 3:
		loginData, err := services.GoObjectBuilderLoginService().LoginData(ctx, &nb.LoginDataReq{
			UserId:                session.GetUserIdAuth(),
			ClientType:            clientTypeId,
			ProjectId:             req.GetProjectId(),
			ResourceEnvironmentId: resource.GetResourceEnvironmentId(),
		})
		if err != nil {
			s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		err = helper.MarshalToStruct(&loginData, &data)
		if err != nil {
			s.log.Error("!!!V2RefreshTokenForEnv-->", logger.Error(err))
			return nil, status.Error(400, err.Error())
		}
	}

	if !data.UserFound {
		customError := config.ErrUserNotFound
		s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(customError))
		return nil, status.Error(codes.NotFound, customError.Error())
	}

	resp := helper.ConvertPbToAnotherPb(&pbObject.V2LoginResponse{
		ClientPlatform:   data.GetClientPlatform(),
		ClientType:       data.GetClientType(),
		UserFound:        data.GetUserFound(),
		UserId:           data.GetUserId(),
		Role:             data.GetRole(),
		Permissions:      data.GetPermissions(),
		LoginTableSlug:   data.GetLoginTableSlug(),
		AppPermissions:   data.GetAppPermissions(),
		GlobalPermission: data.GetGlobalPermission(),
		UserData:         data.GetUserData(),
	})

	if userRole, ok := resp.UserData.Fields["role_id"].GetKind().(*structpb.Value_StringValue); ok {
		roleId = userRole.StringValue
	}

	_, err = s.strg.Session().Update(ctx, &pb.UpdateSessionRequest{
		Id:           session.Id,
		ProjectId:    req.ProjectId,
		ClientTypeId: data.GetClientType().GetGuid(),
		UserId:       data.UserId,
		RoleId:       roleId,
		Ip:           session.Ip,
		Data:         session.Data,
		ExpiresAt:    session.ExpiresAt,
		IsChanged:    session.IsChanged,
		EnvId:        req.EnvId,
	})
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// TODO - wrap in a function
	m := map[string]any{
		"id":               session.Id,
		"ip":               session.Ip,
		"data":             session.Data,
		"tables":           req.GetTables(),
		"user_id":          data.UserId,
		"role_id":          roleId,
		"project_id":       req.ProjectId,
		"user_id_auth":     session.GetUserIdAuth(),
		"client_type_id":   clientTypeId,
		"login_table_slug": tokenInfo.LoginTableSlug,
	}

	accessToken, err := security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	refreshToken, err := security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!V2RefreshTokenForEnv--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	token := &pb.Token{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
		ExpiresAt:        session.ExpiresAt,
		RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
	}

	resp.Token = token

	return resp, nil
}

func (s *sessionService) ExpireSessions(ctx context.Context, req *pb.ExpireSessionsRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.ExpireSessions", req)
	defer dbSpan.Finish()

	s.log.Info("---ExpireSessions--->>>", logger.Any("req", req.SessionIds))

	err := s.strg.Session().ExpireSessions(ctx, req)
	if err != nil {
		s.log.Error("!!!ExpireSessiona--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *sessionService) DeleteByParams(ctx context.Context, req *pb.DeleteByParamsRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.ExpireSessions", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteByParams--->>>", logger.Any("req", req))

	deletedIDs, err := s.strg.Session().DeleteByParams(ctx, req)
	if err != nil {
		s.log.Error("!!!DeleteByParams--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	for _, id := range deletedIDs {
		_ = s.redisClient.Del(ctx, fmt.Sprintf("has_access_user:%s", id)).Err()
	}

	return &emptypb.Empty{}, nil
}

type authParams interface {
	GetUsername() string
	GetPassword() string
	GetType() string
	GetSmsId() string
	GetOtp() string
	GetPhone() string
	GetEmail() string
	GetGoogleToken() string
	GetSessionInfo() string
	GetServiceType() string
}

func (s *sessionService) authenticateUser(ctx context.Context, req authParams) (*pb.User, error) {
	var (
		user *pb.User
		err  error
	)

	switch req.GetType() {
	// empty type means username/password login: older generated admin panels
	// omit the field, and without this case they always got "user not found"
	case config.Default, "":
		if len(req.GetUsername()) < 6 {
			err := errors.New("invalid username")
			s.log.Error("!!!V2Login--->InvalidUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		if len(req.GetUsername()) < 6 {
			err := errors.New("invalid password")
			s.log.Error("!!!V2Login--->InvalidPassword", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByUsername(ctx, req.GetUsername())
		if err != nil {
			s.log.Error("!!!V2Login--->GetByUsername", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}

		hashType := user.GetHashType()
		switch config.HashTypes[hashType] {
		case 1:
			match, err := security.ComparePassword(user.GetPassword(), req.GetPassword())
			if err != nil {
				s.log.Error("!!!MultiCompanyLogin-->ComparePasswordArgon", logger.Error(err))
				return nil, err
			}
			if !match {
				err := errors.New("username or password is wrong")
				s.log.Error("!!!MultiCompanyOneLogin-->Wrong", logger.Error(err))
				return nil, err
			}

			go func() {
				hashedPassword, err := security.HashPasswordBcrypt(req.GetPassword())
				if err != nil {
					s.log.Error("!!!MultiCompanyOneLogin--->HashPasswordBcryptGo", logger.Error(err))
					return
				}

				err = s.strg.User().UpdatePassword(context.Background(), user.Id, hashedPassword)
				if err != nil {
					s.log.Error("!!!MultiCompanyOneLogin--->HashPasswordBcryptGo", logger.Error(err))
					return
				}
			}()
		case 2:
			match, err := security.ComparePasswordBcrypt(user.GetPassword(), req.GetPassword())
			if err != nil {
				s.log.Error("!!!MultiCompanyOneLogin-->ComparePasswordBcrypt", logger.Error(err))
				return nil, err
			}
			if !match {
				err := errors.New("username or password is wrong")
				s.log.Error("!!!MultiCompanyOneLogin--->", logger.Error(err))
				return nil, err
			}
		default:
			err := errors.New("invalid hash type")
			s.log.Error("!!!MultiCompanyOneLogin--->", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case config.WithPhone:
		if req.GetServiceType() == "firebase" {
			err = firebase.VerifyPhoneCode(s.cfg, req.GetSessionInfo(), req.GetOtp())
			if err != nil {
				return nil, err
			}
		} else if config.DefaultOtp != req.GetOtp() {
			_, err := s.services.SmsService().ConfirmOtp(
				ctx,
				&sms_service.ConfirmOtpRequest{
					SmsId: req.GetSmsId(),
					Otp:   req.GetOtp(),
				},
			)
			if err != nil {
				return nil, err
			}
		}

		user, err = s.strg.User().GetByUsername(ctx, req.GetPhone())
		if err != nil {
			s.log.Error("!!!MultiCompanyLogin Phone--->GetByUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case config.WithEmail:
		if config.DefaultOtp != req.GetOtp() {
			_, err := s.services.SmsService().ConfirmOtp(
				ctx,
				&sms_service.ConfirmOtpRequest{
					SmsId: req.GetSmsId(),
					Otp:   req.GetOtp(),
				},
			)
			if err != nil {
				return nil, err
			}
		}

		user, err = s.strg.User().GetByUsername(ctx, req.GetEmail())
		if err != nil {
			s.log.Error("!!!V2Login Email--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case config.WithGoogle:
		var email string
		if req.GetGoogleToken() != "" {
			userInfo, err := helper.GetGoogleUserInfo(req.GetGoogleToken())
			if err != nil {
				s.log.Error("!!!V2LoginWithOption--->failed to decode google id token", logger.Error(err))
				err = errors.New("invalid arguments google auth")
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			if userInfo["error"] != nil || !(userInfo["email_verified"].(bool)) {
				err = errors.New("invalid arguments google auth")
				s.log.Error("!!!V2LoginWithOption--->failed to verify google user info", logger.Error(err))
				return nil, status.Error(codes.InvalidArgument, err.Error())
			}
			email = cast.ToString(userInfo["email"])
		} else {
			err := errors.New("google token is required when login type is google auth")
			s.log.Error("!!!V2LoginWithOption--->no google token--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}

		user, err = s.strg.User().GetByUsername(ctx, email)
		if err != nil {
			s.log.Error("!!!V2Login Email--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	return user, nil
}

func (s *sessionService) lookupUser(ctx context.Context, req authParams) (*pb.User, error) {
	var (
		user *pb.User
		err  error
	)

	switch req.GetType() {
	case config.Default:
		user, err = s.strg.User().GetByUsername(ctx, req.GetUsername())
		if err != nil {
			s.log.Error("!!!lookupUser--->GetByUsername", logger.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	case config.WithPhone:
		user, err = s.strg.User().GetByUsername(ctx, req.GetPhone())
		if err != nil {
			s.log.Error("!!!lookupUser Phone--->GetByUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case config.WithEmail:
		user, err = s.strg.User().GetByUsername(ctx, req.GetEmail())
		if err != nil {
			s.log.Error("!!!lookupUser Email--->GetByUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	case config.WithGoogle:
		if req.GetGoogleToken() == "" {
			return nil, status.Error(codes.InvalidArgument, "google token is required")
		}
		userInfo, err := helper.GetGoogleUserInfo(req.GetGoogleToken())
		if err != nil {
			s.log.Error("!!!lookupUser--->GetGoogleUserInfo", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, "invalid google auth")
		}
		email := cast.ToString(userInfo["email"])
		user, err = s.strg.User().GetByUsername(ctx, email)
		if err != nil {
			s.log.Error("!!!lookupUser Google--->GetByUsername", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
	}

	return user, nil
}

func (s *sessionService) GetSessionDevices(ctx context.Context, req *pb.GetSessionDevicesRequest) (*pb.GetSessionDevicesResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.GetSessionDevices", req)
	defer dbSpan.Finish()

	s.log.Info("---GetSessionDevices--->>>", logger.Any("req", req))

	resp, err := s.strg.Session().GetSessionDevices(ctx, req)
	if err != nil {
		s.log.Error("!!!GetSessionDevices--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return resp, nil
}

func (s *sessionService) DeleteSessionsByDevice(ctx context.Context, req *pb.DeleteSessionsByDeviceRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.DeleteSessionsByDevice", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteSessionsByDevice--->>>", logger.Any("req", req))

	err := s.strg.Session().DeleteSessionsByDevice(ctx, req)
	if err != nil {
		s.log.Error("!!!DeleteSessionsByDevice--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *sessionService) DeleteSessionsExceptCurrent(ctx context.Context, req *pb.DeleteSessionsExceptCurrentRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.DeleteSessionsExceptCurrent", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteSessionsExceptCurrent--->>>", logger.Any("req", req))

	err := s.strg.Session().DeleteSessionsExceptCurrent(ctx, req)
	if err != nil {
		s.log.Error("!!!DeleteSessionsExceptCurrent--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return &emptypb.Empty{}, nil
}

func (s *sessionService) V2RefreshTokenSuperAdmin(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.V2RefreshTokenSuperAdminResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2RefreshToken", req)
	defer dbSpan.Finish()

	var before runtime.MemStats
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the V2RefreshToken", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("V2RefreshToken", memoryUsed))
		}
	}()

	tokenInfo, err := security.ParseClaims(req.RefreshToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->ParseClaims", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	session, err := s.strg.Session().GetByPK(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!RefreshToken--->SessionGetByPK", logger.Error(err))
		return nil, status.Error(codes.Code(status_http.Unauthorized.Code), err.Error())
	}
	if req.ClientTypeId != "" {
		session.ClientTypeId = req.ClientTypeId
	}
	if req.ProjectId != "" {
		session.ProjectId = req.ProjectId
	}
	if req.RoleId != "" {
		session.RoleId = req.RoleId
	}
	if req.EnvId != "" {
		session.EnvId = req.EnvId
	}

	expiresAt, err := time.Parse(config.DatabaseTimeLayout, session.ExpiresAt)
	if err != nil {
		s.log.Error("!!!RefreshToken--->ParseExpiresAt", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if expiresAt.Unix() < time.Now().Unix() {
		err := errors.New("session has been expired")
		s.log.Error("!!!V2HasAccessUser->CheckExpiredToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	//_, err = s.strg.User().CHeckUserProject(ctx, session.GetUserIdAuth(), session.GetProjectId())
	//if err != nil {
	//	s.log.Error("!!!V2Login--->CHeckUserProject", logger.Error(err))
	//	if err == sql.ErrNoRows {
	//		errNoRows := errors.New("no user found")
	//		return nil, status.Error(codes.Internal, errNoRows.Error())
	//	}
	//	return nil, status.Error(codes.Internal, err.Error())
	//}

	_, err = s.strg.Session().Update(ctx, &pb.UpdateSessionRequest{
		Id:               session.Id,
		Ip:               session.Ip,
		Data:             session.Data,
		EnvId:            session.EnvId,
		UserId:           session.UserId,
		RoleId:           session.RoleId,
		ProjectId:        session.ProjectId,
		IsChanged:        session.IsChanged,
		ExpiresAt:        time.Now().Add(config.RefreshTokenExpiresInTime).Format(config.DatabaseTimeLayout),
		ClientTypeId:     session.ClientTypeId,
		ClientPlatformId: session.ClientPlatformId,
	})
	if err != nil {
		s.log.Error("!!!V2RefreshToken.SessionUpdate--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	var authTables []*pb.TableBody

	if tokenInfo.Tables != nil {
		for _, table := range tokenInfo.Tables {
			authTables = append(authTables, &pb.TableBody{
				TableSlug: table.TableSlug,
				ObjectId:  table.ObjectID,
			})
		}
	}

	// TODO - wrap in a function
	m := map[string]any{
		"id":                 session.Id,
		"ip":                 session.Ip,
		"data":               session.Data,
		"tables":             authTables,
		"user_id":            session.UserId,
		"role_id":            session.RoleId,
		"project_id":         session.ProjectId,
		"user_id_auth":       session.UserIdAuth,
		"client_type_id":     session.ClientTypeId,
		"login_table_slug":   tokenInfo.LoginTableSlug,
		"client_platform_id": session.ClientPlatformId,
	}

	accessToken, err := security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->GenerateAccessJWT", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	refreshToken, err := security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->GenerateRefreshJWT", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	res := &pb.V2RefreshTokenSuperAdminResponse{
		Token: &pb.Token{
			AccessToken:      accessToken,
			RefreshToken:     refreshToken,
			CreatedAt:        session.CreatedAt,
			UpdatedAt:        session.UpdatedAt,
			ExpiresAt:        time.Now().Add(config.AccessTokenExpiresInTime).Format(config.DatabaseTimeLayout),
			RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
		},
	}

	return res, nil
}

func (s *sessionService) GetUserInfoByToken(ctx context.Context, req *pb.GetUserInfoByTokenReq) (*pb.GetUserInfoByTokenResp, error) {
	s.log.Info("GetUserInfoByToken", logger.Any("req", req))

	tokenInfo, err := security.ParseClaims(req.Token, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!GetUserInfoByToken->ParseClaims--->", logger.Error(err))
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	return &pb.GetUserInfoByTokenResp{
		SessionId:      tokenInfo.ID,
		UserId:         tokenInfo.UserId,
		RoleId:         tokenInfo.RoleID,
		ProjectId:      tokenInfo.ProjectID,
		ClientTypeId:   tokenInfo.ClientTypeId,
		UserIdAuth:     tokenInfo.UserIdAuth,
		LoginTableSlug: tokenInfo.LoginTableSlug,
	}, nil
}

package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"runtime"
	nb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"

	"time"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	pbCompany "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/sms_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/redis/go-redis/v9"
	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type sessionService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	redisClient *redis.Client
	pb.UnimplementedSessionServiceServer
}

func NewSessionService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI, redisClient *redis.Client) *sessionService {
	return &sessionService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		redisClient: redisClient,
		serviceNode: projectServiceNodes,
	}
}

func (s *sessionService) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	res := &pb.LoginResponse{}

	if len(req.Username) < 6 {
		err := errors.New("invalid username")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if len(req.Password) < 6 {
		err := errors.New("invalid password")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := s.strg.User().GetByUsername(ctx, req.Username)
	if err != nil {
		err := errors.New("invalid username or password")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	hashType := user.GetHashType()
	switch config.HashTypes[hashType] {
	case 1:
		match, err := security.ComparePassword(user.GetPassword(), req.Password)
		if err != nil {
			s.log.Error("!!!Login-->ComparePasswordArgon", logger.Error(err))
			return nil, err
		}
		if !match {
			err := errors.New("username or password is wrong")
			s.log.Error("!!!Login-->Wrong", logger.Error(err))
			return nil, err
		}

		go func() {
			hashedPassword, err := security.HashPasswordBcrypt(req.Password)
			if err != nil {
				s.log.Error("!!!V2UserResetPassword--->HashPasswordBcryptGo", logger.Error(err))
				return
			}
			err = s.strg.User().UpdatePassword(context.Background(), user.Id, hashedPassword)
			if err != nil {
				s.log.Error("!!!V2UserResetPassword--->UpdatePassword", logger.Error(err))
				return
			}
		}()
	case 2:
		match, err := security.ComparePasswordBcrypt(user.GetPassword(), req.Password)
		if err != nil {
			s.log.Error("!!!Login-->ComparePasswordBcrypt", logger.Error(err))
			return nil, err
		}
		if !match {
			err := errors.New("username or password is wrong")
			s.log.Error("!!!Login--->", logger.Error(err))
			return nil, err
		}
	default:
		err := errors.New("hash type is not supported")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if user.Active < 0 {
		err := errors.New("user is not active")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if user.Active == 0 {
		err := errors.New("user hasn't been activated yet")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	expiresAt, err := time.Parse(config.DatabaseTimeLayout, user.ExpiresAt)
	if err != nil {
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if expiresAt.Unix() < time.Now().Unix() {
		err := errors.New("session has been expired")
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res.UserFound = true
	res.User = user

	companies, err := s.services.CompanyService().GetList(ctx, &pb.GetComapnyListRequest{UserId: user.Id})
	if err != nil {
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res.Companies = companies.Companies

	// TODO - Delete all old sessions & refresh token has this function too
	_, err = s.strg.Session().DeleteExpiredUserSessions(ctx, user.Id)
	if err != nil {
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	userSessionList, err := s.strg.Session().GetSessionListByUserID(ctx, user.Id)
	if err != nil {
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res.Sessions = userSessionList.Sessions

	return res, nil
}

func (s *sessionService) GetList(ctx context.Context, req *pb.GetSessionListRequest) (*pb.GetSessionListResponse, error) {
	s.log.Info("!!!SessionGetList--->", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session.SessionGetList", req)
	defer dbSpan.Finish()

	resp, err := s.strg.Session().GetList(ctx, req)
	if err != nil {
		s.log.Error("!!!GetList--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if req.GetCurrentSessionId() != "" {
		for _, session := range resp.GetSessions() {
			session.IsCurrent = session.GetId() == req.GetCurrentSessionId()
		}
	}

	return resp, nil
}

func (s *sessionService) Delete(ctx context.Context, req *pb.SessionPrimaryKey) (*emptypb.Empty, error) {
	s.log.Info("!!!SessionDelete--->", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session.Delete", req)
	defer dbSpan.Finish()

	_, err := s.strg.Session().Delete(ctx, req)
	if err != nil {
		s.log.Error("!!!Delete--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_ = s.redisClient.Del(ctx, fmt.Sprintf("has_access_user:%s", req.Id)).Err()

	return &emptypb.Empty{}, nil
}

func (s *sessionService) RefreshToken(ctx context.Context, req *pb.RefreshTokenRequest) (*pb.RefreshTokenResponse, error) {
	var res = &pb.RefreshTokenResponse{}
	tokenInfo, err := security.ParseClaims(req.RefreshToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!RefreshToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	session, err := s.strg.Session().GetByPK(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!RefreshToken session getbypk--->", logger.Error(err))
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

	_, err = s.strg.Session().Update(ctx, &pb.UpdateSessionRequest{
		Id:               session.Id,
		ProjectId:        session.ProjectId,
		ClientPlatformId: session.ClientPlatformId,
		ClientTypeId:     session.ClientTypeId,
		UserId:           session.UserId,
		RoleId:           session.RoleId,
		Ip:               session.Ip,
		Data:             session.Data,
		ExpiresAt:        session.ExpiresAt,
		IsChanged:        session.IsChanged,
		EnvId:            session.EnvId,
	})
	if err != nil {
		s.log.Error("!!!RefreshToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	user, err := s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{Id: session.UserId})
	if err != nil {
		s.log.Error("!!!RefreshToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if user.Active < 0 {
		err := errors.New("user is not active")
		s.log.Error("!!!RefreshToken--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// TODO - wrap in a function
	m := map[string]any{
		"id":                 session.Id,
		"project_id":         session.ProjectId,
		"client_platform_id": session.ClientPlatformId,
		"client_type_id":     session.ClientTypeId,
		"user_id":            session.UserId,
		"role_id":            session.RoleId,
		"ip":                 session.Data,
		"data":               session.Data,
		"env_id":             session.EnvId,
	}

	accessToken, err := security.GenerateJWT(m, config.AccessTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	refreshToken, err := security.GenerateJWT(m, config.RefreshTokenExpiresInTime, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!Login--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	res.Token = &pb.Token{
		AccessToken:      accessToken,
		RefreshToken:     refreshToken,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
		ExpiresAt:        session.ExpiresAt,
		RefreshInSeconds: int32(config.AccessTokenExpiresInTime.Seconds()),
	}

	return res, nil
}

func (s *sessionService) HasAccessSuperAdmin(ctx context.Context, req *pb.HasAccessSuperAdminReq) (*pb.HasAccessSuperAdminRes, error) {
	s.log.Info("---HasAccessSuperAdmin--->", logger.Any("req", req))
	tokenInfo, err := security.ParseClaims(req.AccessToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!HasAccess token parse--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	session, err := s.strg.Session().GetByPK(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!HasAccess session--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_, err = s.strg.User().GetByPK(ctx, &pb.UserPrimaryKey{Id: session.UserIdAuth})
	if err != nil {
		s.log.Error("!!!HasAccess user--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	// User activity
	go func() {
		var userActivityReq = &nb.UserActivityReqeust{
			LoginTable: tokenInfo.LoginTableSlug,
			UserId:     tokenInfo.UserId,
		}

		s.UserActivity(session, userActivityReq)
	}()

	var authTables []*pb.TableBody
	for _, table := range tokenInfo.Tables {
		authTable := &pb.TableBody{
			TableSlug: table.TableSlug,
			ObjectId:  table.ObjectID,
		}
		authTables = append(authTables, authTable)
	}
	return &pb.HasAccessSuperAdminRes{
		Id:           session.Id,
		ProjectId:    session.ProjectId,
		UserId:       session.UserId,
		Ip:           session.Ip,
		Data:         session.Data,
		ExpiresAt:    session.ExpiresAt,
		CreatedAt:    session.CreatedAt,
		UpdatedAt:    session.UpdatedAt,
		Tables:       authTables,
		EnvId:        session.EnvId,
		ClientTypeId: session.ClientTypeId,
		RoleId:       session.RoleId,
		UserIdAuth:   session.UserIdAuth,
	}, nil
}

func (s *sessionService) Logout(ctx context.Context, req *pb.LogoutRequest) (*emptypb.Empty, error) {
	tokenInfo, err := security.ParseClaims(req.AccessToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!Logout--->", logger.Error(err))
		return &emptypb.Empty{}, nil
	}

	_, err = s.strg.Session().Delete(ctx, &pb.SessionPrimaryKey{Id: tokenInfo.ID})
	if err != nil {
		s.log.Error("!!!Logout--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	_ = s.redisClient.Del(ctx, fmt.Sprintf("has_access_user:%s", tokenInfo.ID)).Err()

	return &emptypb.Empty{}, nil
}

func (s *sessionService) V2MultiCompanyLogin(ctx context.Context, req *pb.V2MultiCompanyLoginReq) (*pb.V2MultiCompanyLoginRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_session_v2.V2MultiCompanyOneLogin", req)
	defer dbSpan.Finish()

	var (
		before runtime.MemStats
		user   = &pb.User{}
		err    error
		resp   = pb.V2MultiCompanyLoginRes{Companies: []*pb.Company2{}}
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
		if config.DefaultOtp != req.Otp {
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
	userProjects, err := s.strg.User().GetUserProjectsEnv(ctx, user.GetId(), req.GetEnvId())
	if err != nil {
		errGetProjects := errors.New("cant get user projects")
		s.log.Error("!!!MultiCompanyLogin--->GetUserProjects", logger.Error(err))
		return nil, status.Error(codes.NotFound, errGetProjects.Error())
	}

	userEnvProject, err := s.strg.User().GetUserEnvProjectsV2(ctx, user.GetId(), req.GetEnvId())
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

func (s *sessionService) HasAccessUser(ctx context.Context, req *pb.V2HasAccessUserReq) (*pb.V2HasAccessUserRes, error) {
	var (
		exist bool
	)

	tokenInfo, err := security.ParseClaims(req.AccessToken, s.cfg.SecretKey)
	if err != nil {
		s.log.Error("!!!V2HasAccessUser->ParseClaims--->", logger.Error(err))
		return nil, status.Error(codes.Unauthenticated, err.Error())
	}

	// Build cache key based on session ID and optional environment context
	cacheKey := fmt.Sprintf("has_access_user:%s", tokenInfo.ID)

	// Try to fetch from cache first
	if cachedStr, cacheErr := s.redisClient.Get(ctx, cacheKey).Result(); cacheErr == nil && cachedStr != "" {
		var cachedRes pb.V2HasAccessUserRes
		if unmarshalErr := json.Unmarshal([]byte(cachedStr), &cachedRes); unmarshalErr == nil {
			return &cachedRes, nil
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
		err = errors.New("user not access project")
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

	// Build response
	res := &pb.V2HasAccessUserRes{
		Id:               session.Id,
		EnvId:            session.EnvId,
		UserId:           session.UserId,
		RoleId:           session.RoleId,
		ProjectId:        session.ProjectId,
		ExpiresAt:        session.ExpiresAt,
		CreatedAt:        session.CreatedAt,
		UpdatedAt:        session.UpdatedAt,
		UserIdAuth:       session.UserIdAuth,
		ClientTypeId:     session.ClientTypeId,
		ClientPlatformId: session.ClientPlatformId,
	}

	// Store in cache
	if bytes, marshalErr := json.Marshal(res); marshalErr == nil {
		_ = s.redisClient.Set(ctx, cacheKey, bytes, config.HAS_ACCESS_USER_CACHE_TTL).Err()
	}

	return res, nil
}

// touchSessionActivity bumps session.last_activity_at asynchronously, at most
// once per SESSION_LAST_ACTIVITY_THRESHOLD, using the last_activity value
// already loaded on the session to avoid extra reads.
func (s *sessionService) touchSessionActivity(session *pb.Session) {
	lastActivity, err := time.Parse(config.DatabaseTimeLayout, session.GetLastActivity())
	if err == nil && time.Since(lastActivity) < config.SESSION_LAST_ACTIVITY_THRESHOLD {
		return
	}

	go func() {
		if err := s.strg.Session().UpdateLastActivity(context.Background(), session.GetId()); err != nil {
			s.log.Error("!!!touchSessionActivity->UpdateLastActivity--->", logger.Error(err))
		}
	}()
}

// User activity
func (s *sessionService) UserActivity(session *pb.Session, req *nb.UserActivityReqeust) {
	var ctx = context.Background()

	resource, err := s.services.ServiceResource().GetSingle(ctx,
		&pbCompany.GetSingleServiceResourceReq{
			ProjectId:     session.ProjectId,
			EnvironmentId: session.EnvId,
			ServiceType:   pbCompany.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		s.log.Error("!!!UserActivity->GetSingleServiceResource--->", logger.Error(err))
		return
	}

	if resource.ResourceType != pbCompany.ResourceType_POSTGRESQL {
		s.log.Info("!!!UserActivity->UserActivity not implemented for this resource type --->", logger.Error(err))
		return
	}

	services, err := s.serviceNode.GetByNodeType(resource.ProjectId, resource.NodeType)
	if err != nil {
		return
	}

	req.ResourceEnvId = resource.ResourceEnvironmentId
	_, err = services.GoObjectBuilderService().UserActivity(ctx, req)
	if err != nil {
		s.log.Error("!!!UserActivity->UserActivity--->", logger.Error(err))
		return
	}
}

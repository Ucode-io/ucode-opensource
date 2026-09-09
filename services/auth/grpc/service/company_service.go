package service

import (
	"context"
	"errors"
	"runtime"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/auth/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
	"github.com/saidamir98/udevs_pkg/logger"
	"github.com/spf13/cast"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type companyService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedCompanyServiceServer
}

func NewCompanyService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *companyService {
	return &companyService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (s *companyService) Register(ctx context.Context, req *pb.RegisterCompanyRequest) (*pb.CompanyPrimaryKey, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.Register", req)
	defer dbSpan.Finish()

	var (
		before      runtime.MemStats
		email       string = req.GetUserInfo().GetEmail()
		googleToken string = req.GetGoogleToken().GetAccessToken()
		password    string = req.GetUserInfo().GetPassword()
		login              = req.GetUserInfo().GetLogin()
	)
	runtime.ReadMemStats(&before)

	defer func() {
		var after runtime.MemStats
		runtime.ReadMemStats(&after)
		memoryUsed := (after.TotalAlloc - before.TotalAlloc) / (1024 * 1024)
		s.log.Info("Memory used by the CompanyRegister", logger.Any("memoryUsed", memoryUsed))
		if memoryUsed > 300 {
			s.log.Info("Memory used over 300 mb", logger.Any("CompanyRegister", memoryUsed))
		}
	}()

	tempOwnerId, err := uuid.NewRandom()
	if err != nil {
		s.log.Error("---RegisterCompany-->NewUUID", logger.Error(err))
		return nil, err
	}

	clientTypeId := uuid.NewString()
	roleId := uuid.NewString()

	if googleToken != "" {
		userInfo, err := helper.DecodeGoogleIDToken(googleToken)
		if err != nil {
			err = errors.New("invalid arguments google auth")
			s.log.Error("!!!RegisterCompany--->", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		if userInfo["error"] != nil || !(userInfo["email_verified"].(bool)) {
			err = errors.New("invalid arguments google auth")
			s.log.Error("!!!RegisterCompany-->EmailVerified", logger.Error(err))
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		email = cast.ToString(userInfo["email"])
	}

	if len(login) < 6 {
		err = errors.New("invalid username")
		s.log.Error("!!!RegisterCompany-->InvalidUsername", logger.Error(err))
		return nil, err
	}

	err = util.ValidStrongPassword(password)
	if err != nil {
		s.log.Error("!!!RegisterCompany-->ValidStrong password", logger.Error(err))
		return nil, err
	}

	companyPKey, err := s.services.CompanyServiceClient().Create(ctx, &company_service.CreateCompanyRequest{
		Title:   req.Name,
		OwnerId: tempOwnerId.String(),
	})
	if err != nil {
		s.log.Error("---RegisterCompany--->CreateCompanyServiceClient", logger.Error(err))
		return nil, err
	}

	project, err := s.services.ProjectServiceClient().Create(ctx, &company_service.CreateProjectRequest{
		CompanyId:    companyPKey.GetId(),
		K8SNamespace: config.K8SNamespace,
		Title:        req.GetName(),
		FareId:       req.GetFareId(),
		IsUgen:       req.IsUgen,
	})
	if err != nil {
		s.log.Error("---RegisterCompany--->CreateProject", logger.Error(err))
		return nil, err
	}

	_, _ = s.services.ProjectServiceClient().Update(ctx, &company_service.Project{
		CompanyId:    companyPKey.GetId(),
		K8SNamespace: config.K8SNamespace,
		ProjectId:    project.GetProjectId(),
		Title:        req.Name,
		Language: []*company_service.Language{{
			Id:         config.LanguageId,
			Name:       config.NativeName,
			NativeName: config.NativeName,
			ShortName:  config.ShortName,
		}},
		NewDesign: true,
		NewLayout: true,
		NewRouter: true,
	})

	environment, err := s.services.EnvironmentService().Create(ctx,
		&company_service.CreateEnvironmentRequest{
			ProjectId:    project.ProjectId,
			Name:         "Production",
			DisplayColor: "#00FF00",
			Description:  "Production Environment",
		},
	)
	if err != nil {
		s.log.Error("---RegisterCompany-->CreateEnvironment", logger.Error(err))
		return nil, err
	}

	_, err = s.services.ApiKeysService().Create(ctx, &pb.CreateReq{
		Name:             "Function",
		ProjectId:        project.ProjectId,
		EnvironmentId:    environment.GetId(),
		ClientPlatformId: config.OpenFaaSPlatformID,
		Disable:          true,
		ClientTypeId:     clientTypeId,
		RoleId:           roleId,
	})
	if err != nil {
		s.log.Error("!!!RegisterCompany-->CreateApiKey", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	hashedPassword, err := security.HashPasswordBcrypt(password)
	if err != nil {
		s.log.Error("!!!RegisterCompany-->HashPassword", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	createUserRes, err := s.strg.User().Create(ctx, &pb.CreateUserRequest{
		Phone:     req.GetUserInfo().GetPhone(),
		Email:     email,
		Login:     login,
		Password:  hashedPassword,
		Active:    1,
		CompanyId: companyPKey.GetId(),
	})
	if err != nil {
		s.log.Error("---RegisterCompany--->CreateUser", logger.Error(err))
		return nil, err
	}

	_, err = s.services.CompanyServiceClient().Update(ctx, &company_service.Company{
		Id:      companyPKey.Id,
		Name:    req.Name,
		OwnerId: createUserRes.GetId(),
	})
	if err != nil {
		s.log.Error("---RegisterCompany-->UpdateCompany", logger.Error(err))
		return nil, err
	}

	_, err = s.services.UserService().AddUserToProject(ctx, &pb.AddUserToProjectReq{
		CompanyId:    companyPKey.GetId(),
		ProjectId:    project.GetProjectId(),
		UserId:       createUserRes.GetId(),
		EnvId:        environment.GetId(),
		ClientTypeId: clientTypeId,
		RoleId:       roleId,
	})
	if err != nil {
		s.log.Error("---RegisterCompany-->AddUser2Project", logger.Error(err))
		return nil, err
	}

	resource, err := s.services.ResourceService().CreateResource(ctx, &company_service.CreateResourceReq{
		CompanyId:     companyPKey.GetId(),
		EnvironmentId: environment.GetId(),
		ProjectId:     project.GetProjectId(),
		Resource: &company_service.Resource{
			ResourceType: 3,
			NodeType:     config.LOW_NODE_TYPE,
		},
		UserId:       createUserRes.GetId(),
		ClientTypeId: clientTypeId,
		RoleId:       roleId,
	})
	if err != nil {
		s.log.Error("---RegisterCompany-AutoCreateResource--->", logger.Error(err))
		return nil, err
	}

	_, err = s.services.ServiceResource().Update(ctx, &company_service.UpdateServiceResourceReq{
		EnvironmentId:    environment.GetId(),
		ProjectId:        project.GetProjectId(),
		ServiceResources: helper.MakeBodyServiceResource(resource.GetId()),
	})
	if err != nil {
		s.log.Error("---RegisterCompany-AutoSettingMicroServices--->", logger.Error(err))
		return nil, err
	}

	return &pb.CompanyPrimaryKey{Id: companyPKey.GetId()}, nil
}

func (s *companyService) Update(ctx context.Context, req *pb.UpdateCompanyRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.Update", req)
	defer dbSpan.Finish()

	_, err := s.strg.Company().Update(ctx, req)
	if err != nil {
		s.log.Error("---UpdateCompany--->", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *companyService) Remove(ctx context.Context, req *pb.CompanyPrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.Remove", req)
	defer dbSpan.Finish()

	_, err := s.strg.Company().Remove(ctx, req)
	if err != nil {
		s.log.Error("---RemoveCompany--->", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *companyService) GetList(ctx context.Context, req *pb.GetComapnyListRequest) (*pb.GetListCompanyResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetList", req)
	defer dbSpan.Finish()

	resp, err := s.strg.Company().GetList(ctx, req)
	if err != nil {
		s.log.Error("---RemoveCompany--->", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *companyService) GetByID(ctx context.Context, pKey *pb.CompanyPrimaryKey) (*pb.Company, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetByID", pKey)
	defer dbSpan.Finish()

	resp, err := s.strg.Company().GetByID(ctx, pKey)
	if err != nil {
		s.log.Error("---RemoveCompany--->", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

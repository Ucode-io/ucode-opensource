package service

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	minioclient "github.com/Ucode-io/ucode-opensource/services/company/pkg/minio"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

// AdminService ...
type ProjectService struct {
	storage  storage.StorageI
	services ServiceNodesI
	logger   l.Logger
	cfg      config.BaseConfig
	minio    *minioclient.Client
	pb.UnimplementedProjectServiceServer
}

// NewAdminService ...
func NewProjectService(strg storage.StorageI, log l.Logger, services ServiceNodesI, cfg config.BaseConfig, minio *minioclient.Client) *ProjectService {
	return &ProjectService{
		storage:  strg,
		services: services,
		logger:   log,
		cfg:      cfg,
		minio:    minio,
	}
}

func (s *ProjectService) Create(ctx context.Context, req *pb.CreateProjectRequest) (*pb.CreateProjectResponse, error) {
	s.logger.Info("--CreateProject-- requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.Create", req)
	defer dbSpan.Finish()

	_, err := s.storage.Company().GetById(ctx, req.CompanyId)
	if err != nil {
		s.logger.Error("--CreateProject--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if req.K8SNamespace == "" {
		req.K8SNamespace = "u-code"
	}

	projectId := uuid.New()

	company, err := s.storage.Project().GetProjectsByCompanyId(ctx, &pb.GetProjectsByCompanyIdReq{
		CompanyId: req.CompanyId,
	})
	if err != nil {
		s.logger.Error("--CreateProject--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	projects := len(company.GetCompany().GetProjects())
	limit := selfHostLimit{noun: "project", setting: "MAX_PROJECTS", max: s.cfg.MaxProjects}
	if err := limit.reached(projects); err != nil {
		s.logger.Info("--CreateProject-- refused by the installation limit",
			l.Int("projects", projects), l.Int("max", s.cfg.MaxProjects))
		return nil, err
	}

	resp, err := s.storage.Project().Create(ctx, projectId.String(), req)
	if err != nil {
		s.logger.Error("!!!CreateProject", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) GetById(ctx context.Context, req *pb.GetProjectByIdRequest) (*pb.Project, error) {
	s.logger.Info("--GetProjectById-- requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetById", req)
	defer dbSpan.Finish()

	project, err := s.storage.Project().GetById(ctx, req.ProjectId)
	if err != nil {
		s.logger.Error("--GetProjectById--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return project, nil
}

func (s *ProjectService) GetList(ctx context.Context, req *pb.GetProjectListRequest) (*pb.GetProjectListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetList", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetProjectList-- requested", l.Any("req", req))

	companies, err := s.storage.Project().GetList(ctx, req)
	if err != nil {
		s.logger.Error("--GetProjectList--", l.Error(err))
	}

	return companies, nil
}

func (s *ProjectService) Update(ctx context.Context, req *pb.Project) (*pb.Project, error) {
	s.logger.Info("--UpdateProject-- requested", l.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.Update", req)
	defer dbSpan.Finish()

	company, err := s.storage.Project().Update(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateProject--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return company, nil
}

func (s *ProjectService) Delete(ctx context.Context, req *pb.DeleteProjectRequest) (*pb.EmptyProto, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.Delete", req)
	defer dbSpan.Finish()

	s.logger.Info("--DeleteProject-- requested", l.Any("req", req))
	// @TODO delete all environments and resources
	err := s.storage.Project().Delete(ctx, req.ProjectId)
	if err != nil {
		s.logger.Error("--DeleteProject--", l.Error(err))
		return nil, err
	}

	return &pb.EmptyProto{}, nil
}

func (s *ProjectService) GetProjectsByCompanyId(ctx context.Context, req *pb.GetProjectsByCompanyIdReq) (*pb.GetProjectsByCompanyIdRes, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetProjectsByCompanyId", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetProjectsByCompanyId--", l.Any("req", req))

	projects, err := s.storage.Project().GetProjectsByCompanyId(ctx, req)
	if err != nil {
		errGetProject := errors.New("cant get projects info")
		s.logger.Error("--GetProjectsByCompanyId--", l.Error(err))
		return nil, errGetProject
	}

	return projects, nil
}

func (s *ProjectService) GetListSetting(ctx context.Context, req *pb.GetListSettingReq) (*pb.Setting, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetListSetting", req)
	defer dbSpan.Finish()

	var (
		res      pb.Setting
		structPb structpb.Struct
	)
	s.logger.Info("--GetListSetting--", l.Any("req", req))

	switch pb.SettingType(req.GetType()) {
	case pb.SettingType_LANGUAGE:
		lan, err := s.storage.Project().GetListLanguage(ctx, req)
		if err != nil {
			errGetSetting := errors.New("cant get language list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}
		lanJson, err := json.Marshal(lan)
		if err != nil {
			errGetSetting := errors.New("cant get language list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		err = structPb.UnmarshalJSON(lanJson)
		if err != nil {
			errGetSetting := errors.New("cant get language list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		res.Data = &structPb
	case pb.SettingType_CURRENCY:
		cur, err := s.storage.Project().GetListCurrency(ctx, req)
		if err != nil {
			errGetSetting := errors.New("cant get currency list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		curJson, err := json.Marshal(cur)
		if err != nil {
			errGetSetting := errors.New("cant get currency list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		err = structPb.UnmarshalJSON(curJson)
		if err != nil {
			errGetSetting := errors.New("cant get currency list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		res.Data = &structPb
	case pb.SettingType_TIMEZONE:
		tzones, err := s.storage.Project().GetListTimezone(ctx, req)
		if err != nil {
			errGetSetting := errors.New("cant get timezone list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		tzonesJson, err := json.Marshal(tzones)
		if err != nil {
			errGetSetting := errors.New("cant get timezone list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		err = structPb.UnmarshalJSON(tzonesJson)
		if err != nil {
			errGetSetting := errors.New("cant get timezone list")
			s.logger.Error("--GetListSetting--", l.Error(err))
			return nil, errGetSetting
		}

		res.Data = &structPb
	default:
		err := errors.New("invalid setting type")
		s.logger.Error("--GetListSetting--", l.Error(err))
		return nil, err
	}

	return &res, nil
}

func (s *ProjectService) CreateProjectLoginMicroFront(ctx context.Context, req *pb.ProjectLoginMicroFrontend) (*pb.ProjectLoginMicroFrontend, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.CreateProjectLoginMicroFront", req)
	defer dbSpan.Finish()

	s.logger.Info("--CreateProjectLoginMicroFront-- requested", l.Any("req: ", req))

	resp, err := s.storage.Project().BindMicroFrontToProject(ctx, req)
	if err != nil {
		s.logger.Error("--CreateProjectLoginMicroFront--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) GetProjectLoginMicroFront(ctx context.Context, req *pb.GetProjectLoginMicroFrontRequest) (*pb.ProjectLoginMicroFrontend, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetProjectLoginMicroFront", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetProjectLoginMicroFront-- requested", l.Any("req: ", req))

	resp, err := s.storage.Project().GetProjectLoginMicroFront(ctx, req)
	if err != nil {
		s.logger.Error("--GetProjectLoginMicroFront--", l.Error(err))
	}

	return resp, nil
}

func (s *ProjectService) UpdateProjectLoginMicroFront(ctx context.Context, req *pb.ProjectLoginMicroFrontend) (*pb.ProjectLoginMicroFrontend, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.UpdateProjectLoginMicroFront", req)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateProjectLoginMicroFront-- requested", l.Any("req", req))

	company, err := s.storage.Project().UpdateProjectLoginMicroFront(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateProjectLoginMicroFront--", l.Error(err))
	}

	return company, nil
}

func (s *ProjectService) UpdateProjectConfig(ctx context.Context, in *pb.ProjectConfig) (*pb.ProjectConfig, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.UpdateProjectConfig", in)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateProjectConfig-- requested", l.Any("req: ", in))

	var resp = &pb.ProjectConfig{}

	config, err := s.storage.Project().GetProjectConfigById(
		ctx,
		&pb.GetPorjectConfigByProjectIdRequest{
			ProjectId: in.ProjectId,
		},
	)
	if err != nil {
		s.logger.Error("--GetPorjectConfigByProjectId--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if config == nil {
		resp, err = s.storage.Project().CreateProjectConfig(
			ctx,
			in,
		)
		if err != nil {
			s.logger.Error("--CreateProjectConfig--", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		resp, err = s.storage.Project().UpdateProjectConfig(
			ctx,
			in,
		)
		if err != nil {
			s.logger.Error("--UpdateProjectConfig--", l.Error(err))
			return nil, status.Error(codes.Internal, err.Error())
		}
	}

	return resp, nil
}

func (s *ProjectService) GetProjectConfigList(ctx context.Context, in *emptypb.Empty) (*pb.ListPorjectConfig, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetProjectConfigList", in)
	defer dbSpan.Finish()

	s.logger.Info("--GetProjectConfigList-- requested", l.Any("req: ", in))

	resp, err := s.storage.Project().GetListProjectConfig(ctx, in)
	if err != nil {
		s.logger.Error("--GetProjectConfigList--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) GetPorjectConfigByProjectId(ctx context.Context, in *pb.GetPorjectConfigByProjectIdRequest) (*pb.ProjectConfig, error) {
	s.logger.Info("--GetPorjectConfigByProjectId-- requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetPorjectConfigByProjectId", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Project().GetProjectConfigById(ctx, in)
	if err != nil {
		s.logger.Error("--GetPorjectConfigByProjectId--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) ListProjectsRPS(ctx context.Context, in *pb.GetProjectListRequest) (*pb.ListProjectsRPSResponse, error) {
	s.logger.Info("--ListProjectsRPS--requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetPorjectConfigByProjectId", in)
	defer dbSpan.Finish()

	resp, err := s.storage.Project().ListProjectRPS(ctx, in)
	if err != nil {
		s.logger.Error("--ListProjectsRPS--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) AttachCustomer(ctx context.Context, in *pb.AttachCustomerRequest) (*pb.EmptyProto, error) {
	s.logger.Info("--AttachCustomer--requested", l.Any("req: ", in))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.AttachCustomer", in)
	defer dbSpan.Finish()

	_, err := s.storage.Project().AttachCustomer(ctx, in)
	if err != nil {
		s.logger.Error("--AttachCustomer--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.EmptyProto{}, nil
}

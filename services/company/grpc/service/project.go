package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	minioclient "github.com/Ucode-io/ucode-opensource/services/company/pkg/minio"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/structpb"
)

const ugenProjectsExcelContentType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"

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

	if len(company.GetCompany().GetProjects()) >= 1 && s.cfg.Environment != config.PRODUCTION {
		s.logger.Error("!!!CreateProject", l.String("error", "only one project allowed"))
		return nil, status.Error(codes.Internal, "only one project allowed")
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

func (s *ProjectService) GetUgenProjectByCompanyId(ctx context.Context, req *pb.GetUgenProjectByCompanyIdReq) (*pb.Project, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetUgenProjectByCompanyId", req)
	defer dbSpan.Finish()

	if req.GetCompanyId() == "" {
		return nil, status.Error(codes.InvalidArgument, "company_id is required")
	}

	project, err := s.storage.Project().GetUgenProjectByCompanyId(ctx, req.GetCompanyId())
	if err != nil {
		s.logger.Error("--GetUgenProjectByCompanyId--", l.Error(err))
		return nil, status.Error(codes.NotFound, "head ugen project not found for company")
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

func (s *ProjectService) GetProjectUgenStatus(ctx context.Context, req *pb.GetProjectUgenStatusRequest) (*pb.GetProjectUgenStatusResponse, error) {
	s.logger.Info("--GetProjectUgenStatus--requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.GetProjectUgenStatus", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Project().GetProjectUgenStatus(ctx, req)
	if err != nil {
		s.logger.Error("--GetProjectUgenStatus--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) UpdateProjectUgenAccess(ctx context.Context, req *pb.UpdateProjectUgenAccessRequest) (*pb.UpdateProjectUgenAccessResponse, error) {
	s.logger.Info("--UpdateProjectUgenAccess--requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.UpdateProjectUgenAccess", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Project().UpdateProjectUgenAccess(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateProjectUgenAccess--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) AutoAssignUgenIfSingle(ctx context.Context, req *pb.AutoAssignUgenIfSingleRequest) (*pb.AutoAssignUgenIfSingleResponse, error) {
	s.logger.Info("--AutoAssignUgenIfSingle--requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.AutoAssignUgenIfSingle", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Project().AutoAssignUgenIfSingle(ctx, req)
	if err != nil {
		s.logger.Error("--AutoAssignUgenIfSingle--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) ListUgenProjects(ctx context.Context, req *pb.ListUgenProjectsRequest) (*pb.ListUgenProjectsResponse, error) {
	s.logger.Info("--ListUgenProjects--requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.ListUgenProjects", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Project().ListUgenProjects(ctx, req)
	if err != nil {
		s.logger.Error("--ListUgenProjects--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *ProjectService) ExportUgenProjects(ctx context.Context, req *pb.ExportUgenProjectsRequest) (*pb.ExportUgenProjectsResponse, error) {
	s.logger.Info("--ExportUgenProjects--requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project.ExportUgenProjects", req)
	defer dbSpan.Finish()

	if s.minio == nil {
		return nil, status.Error(codes.FailedPrecondition, config.ErrMinioNotConfigured.Error())
	}

	projects, err := s.storage.Project().ListAllUgenProjects(ctx, req.GetSearch())
	if err != nil {
		s.logger.Error("--ExportUgenProjects--ListAllUgenProjects", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	file, err := buildUgenProjectsExcel(projects)
	if err != nil {
		s.logger.Error("--ExportUgenProjects--buildExcel", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	fileName := fmt.Sprintf("ugen_projects_%s.xlsx", time.Now().Format("20060102_150405"))
	objectName := fmt.Sprintf("ugen_projects/%s", fileName)

	fileURL, err := s.minio.Upload(ctx, objectName, ugenProjectsExcelContentType, file)
	if err != nil {
		s.logger.Error("--ExportUgenProjects--upload", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.ExportUgenProjectsResponse{
		FileUrl:  fileURL,
		FileName: fileName,
		Count:    int32(len(projects)),
		Projects: projects,
	}, nil
}

func buildUgenProjectsExcel(projects []*pb.UgenProjectItem) ([]byte, error) {
	const sheet = "Ugen Projects"

	f := excelize.NewFile()
	defer f.Close()

	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return nil, err
	}

	headers := []string{
		"Project ID", "Title", "Company ID", "K8s Namespace", "Logo", "Fare ID",
		"Balance", "Credit Limit", "Status", "Created At", "Updated At",
		"Environment ID", "Company Projects Count", "Last Activity Date",
	}
	for col, h := range headers {
		cell, err := excelize.CoordinatesToCellName(col+1, 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}

	for i, p := range projects {
		values := []any{
			p.GetProjectId(),
			p.GetTitle(),
			p.GetCompanyId(),
			p.GetK8SNamespace(),
			p.GetLogo(),
			p.GetFareId(),
			p.GetBalance(),
			p.GetCreditLimit(),
			p.GetStatus(),
			p.GetCreatedAt(),
			p.GetUpdatedAt(),
			p.GetEnvironmentId(),
			p.GetCompanyProjectsCount(),
			p.GetLastActivityDate(),
		}
		for col, v := range values {
			cell, err := excelize.CoordinatesToCellName(col+1, i+2)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

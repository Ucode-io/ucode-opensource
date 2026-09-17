package service

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/company/genproto/new_object_builder_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
)

// AdminService ...
type CompanyService struct {
	storage  storage.StorageI
	services ServiceNodesI
	logger   l.Logger
	cfg      config.BaseConfig
	pb.UnimplementedCompanyServiceServer
}

// NewAdminService ...
func NewCompanyService(strg storage.StorageI, log l.Logger, services ServiceNodesI, cfg config.BaseConfig) *CompanyService {
	return &CompanyService{
		storage:  strg,
		logger:   log,
		services: services,
		cfg:      cfg,
	}
}

func (s *CompanyService) Create(ctx context.Context, req *pb.CreateCompanyRequest) (*pb.CreateCompanyResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.Create", req)
	defer dbSpan.Finish()

	companies, err := s.storage.Company().GetList(ctx, &pb.GetCompanyListRequest{})
	if companies.Count >= 1 && s.cfg.Environment != config.PRODUCTION {
		s.logger.Error("--CreateCompany--", l.String("error", "only one company allowed"))
		return nil, status.Error(codes.Internal, "only one company allowed")
	}

	resp, err := s.storage.Company().Create(ctx, req)
	if err != nil {
		s.logger.Error("--CreateCompany--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *CompanyService) GetById(ctx context.Context, req *pb.GetCompanyByIdRequest) (*pb.GetCompanyByIdResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetById", req)
	defer dbSpan.Finish()

	company, err := s.storage.Company().GetById(ctx, req.GetId())
	if err != nil {
		s.logger.Error("--GetCompanyById--", l.Error(err))
	}
	return company, nil
}

func (s *CompanyService) GetList(ctx context.Context, req *pb.GetCompanyListRequest) (*pb.GetComanyListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetList", req)
	defer dbSpan.Finish()

	companies, err := s.storage.Company().GetList(ctx, req)
	if err != nil {
		s.logger.Error("--GetCompanyList--", l.Error(err))
	}

	s.logger.Info("--GetCompanyList-- requested", l.Any("response", companies))

	return companies, nil
}

func (s *CompanyService) Update(ctx context.Context, req *pb.Company) (*pb.Company, error) {
	s.logger.Info("--UpdateCompany-- requested", l.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.Update", req)
	defer dbSpan.Finish()

	company, err := s.storage.Company().Update(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateCompany--", l.Error(err))
	}

	return company, nil
}

func (s *CompanyService) Delete(ctx context.Context, req *pb.DeleteCompanyRequest) (*pb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.Delete", req)
	defer dbSpan.Finish()

	err := s.storage.Company().Delete(ctx, req.Id)
	if err != nil {
		s.logger.Error("--DeleteCompany--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.Empty{}, nil
}

func (s *CompanyService) GetListWithProjects(ctx context.Context, req *pb.GetListWithProjectsRequest) (*pb.GetListWithProjectsResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetListWithProjects", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Company().GetListWithProjects(ctx, req)
	if err != nil {
		s.logger.Error("--GetListWithProjects--", l.Error(err))
	}

	return resp, nil
}

func (s *CompanyService) GetAllMenuTemplate(ctx context.Context, req *emptypb.Empty) (*pb.GatAllMenuTemplateResponse, error) {
	resp, err := s.storage.Company().GetAllMenuTemplate(ctx, &emptypb.Empty{})
	if err != nil {
		s.logger.Error("--GetAllMenuTemplate--", l.Error(err))
		return nil, err
	}
	return resp, nil
}

func (s *CompanyService) GetMenuTemplateById(ctx context.Context, req *pb.GetMenuTemplateRequest) (*pb.MenuTemplate, error) {
	s.logger.Info("--GetMenuTemplateById-- requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetMenuTemplateById", req)
	defer dbSpan.Finish()

	res, err := s.storage.Company().GetMenuTemplateById(ctx, req)
	if err != nil {
		s.logger.Error("--GetMenuTemplateById--", l.Error(err))
		return nil, err
	}
	return res, nil
}

func (s *CompanyService) CreateMenuTemplate(ctx context.Context, req *pb.CreateMenuTemplateRequest) (*pb.Empty, error) {
	s.logger.Info("--CreateMenuTemplate-- requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.CreateMenuTemplate", req)
	defer dbSpan.Finish()

	service, err := s.services.GetByNodeType(req.GetProjectId(), "LOW")
	if err != nil {
		s.logger.Error("--CreateMenuTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	menuTemplate, err := service.GoMenuService().GetMenuTemplateWithEntities(ctx, &nobs.GetMenuTemplateRequest{
		ProjectId: req.GetProjectId(),
		Name:      req.GetName(),
		MenuId:    req.GetMenuId(),
	})
	if err != nil {
		s.logger.Error("--CreateMenuTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	jsonData, err := json.Marshal(menuTemplate)
	if err != nil {
		s.logger.Error("--CreateMenuTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	err = s.storage.Company().CreateMenuTemplate(ctx, &pb.CreateMenuTemplateRequest{
		Name: req.GetName(),
		Data: string(jsonData),
	})
	if err != nil {
		s.logger.Error("--CreateMenuTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &pb.Empty{}, nil
}

func (s *CompanyService) GetProjectMenuTemplates(ctx context.Context, req *pb.GetProjectMenuTemplateRequest) (*pb.GetProjectMenuTemplateResponse, error) {
	s.logger.Info("--GetProjectMenuTemplates-- requested", l.Any("req: ", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_company.GetProjectMenuTemplates", req)
	defer dbSpan.Finish()

	resp, err := s.storage.Company().GetProjectMenuTemplates(ctx, req)
	if err != nil {
		s.logger.Error("--GetProjectMenuTemplates--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

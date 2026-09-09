package service

import (
	"context"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
)

// AdminService ...
type TemplateService struct {
	storage  storage.StorageI
	services ServiceNodesI
	logger   l.Logger
	pb.UnimplementedTemplateMetadataServiceServer
}

// NewAdminService ...
func NewTemplateService(strg storage.StorageI, log l.Logger, services ServiceNodesI) *TemplateService {
	return &TemplateService{
		storage:  strg,
		logger:   log,
		services: services,
	}
}

func (s *TemplateService) Create(ctx context.Context, req *pb.CreateTemplateMetadataReq) (*pb.TemplateMetadata, error) {
	s.logger.Info("--CreateTemplate-- requested", logger.Any("req", req))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_template.Create", req)
	defer dbSpan.Finish()

	resp, err := s.storage.TemplateMetadata().Create(ctx, req)
	if err != nil {
		s.logger.Error("--CreateTemplate--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

func (s *TemplateService) GetById(ctx context.Context, req *pb.GetTemplateMetadataByIdReq) (*pb.TemplateMetadata, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_template.GetById", req)
	defer dbSpan.Finish()

	template, err := s.storage.TemplateMetadata().GetById(ctx, req.GetId())
	if err != nil {
		s.logger.Error("--GetTemplateById--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return template, nil
}

func (s *TemplateService) List(ctx context.Context, req *pb.GetTemplateMetadataListReq) (*pb.GetTemplateMetadataListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_template.List", req)
	defer dbSpan.Finish()

	templates, err := s.storage.TemplateMetadata().GetList(ctx, req)
	if err != nil {
		s.logger.Error("--GetTemplateList--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return templates, nil
}

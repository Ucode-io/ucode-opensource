package service

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	span "github.com/Ucode-io/ucode-opensource/services/company/pkg/jaeger"
	l "github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// IntegrationResourceService handles gRPC calls for integration resources (GitHub, GitLab, etc.)
type IntegrationResourceService struct {
	storage storage.StorageI
	logger  l.Logger
	pb.UnimplementedIntegrationResourceServiceServer
}

// NewIntegrationResourceService creates a new IntegrationResourceService.
func NewIntegrationResourceService(strg storage.StorageI, log l.Logger) *IntegrationResourceService {
	return &IntegrationResourceService{
		storage: strg,
		logger:  log,
	}
}

func (s *IntegrationResourceService) CreateIntegrationResource(ctx context.Context, req *pb.CreateIntegrationResourceRequest) (*pb.IntegrationResource, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.Create", req)
	defer dbSpan.Finish()

	s.logger.Info("--CreateIntegrationResource-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().Create(ctx, req)
	if err != nil {
		s.logger.Error("--CreateIntegrationResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *IntegrationResourceService) UpsertIntegrationResource(ctx context.Context, req *pb.CreateIntegrationResourceRequest) (*pb.IntegrationResource, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.Upsert", req)
	defer dbSpan.Finish()

	s.logger.Info("--UpsertIntegrationResource-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().Upsert(ctx, req)
	if err != nil {
		s.logger.Error("--UpsertIntegrationResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *IntegrationResourceService) GetById(ctx context.Context, req *pb.IntegrationResourcePrimaryKey) (*pb.IntegrationResource, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.GetById", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetIntegrationResourceById-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().GetById(ctx, req)
	if err != nil {
		s.logger.Error("--GetIntegrationResourceById--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *IntegrationResourceService) GetByUsername(ctx context.Context, req *pb.GetByUsernameRequest) (*pb.GetByUsernameResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.GetByUsername", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetIntegrationResourceByUsername-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().GetByUsername(ctx, req)
	if err != nil {
		s.logger.Error("--GetIntegrationResourceByUsername--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *IntegrationResourceService) GetIntegrationResourceList(ctx context.Context, req *pb.GetListIntegrationResourceRequest) (*pb.GetListIntegrationResourceResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.GetList", req)
	defer dbSpan.Finish()

	s.logger.Info("--GetIntegrationResourceList-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().GetList(ctx, req)
	if err != nil {
		s.logger.Error("--GetIntegrationResourceList--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *IntegrationResourceService) UpdateIntegrationResource(ctx context.Context, req *pb.UpdateIntegrationResourceRequest) (*empty.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.Update", req)
	defer dbSpan.Finish()

	s.logger.Info("--UpdateIntegrationResource-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().Update(ctx, req)
	if err != nil {
		s.logger.Error("--UpdateIntegrationResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}

func (s *IntegrationResourceService) DeleteIntegrationResource(ctx context.Context, req *pb.IntegrationResourcePrimaryKey) (*empty.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_integration_resource.Delete", req)
	defer dbSpan.Finish()

	s.logger.Info("--DeleteIntegrationResource-- requested", l.Any("req", req))

	res, err := s.storage.IntegrationResource().Delete(ctx, req)
	if err != nil {
		s.logger.Error("--DeleteIntegrationResource--", l.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}
	return res, nil
}
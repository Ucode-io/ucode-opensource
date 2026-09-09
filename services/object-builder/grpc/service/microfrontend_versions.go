package service

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type MicrofrontendVersionsService struct {
	nb.UnimplementedMicrofrontendVersionsServiceServer
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
}

func NewMicrofrontendVersionsService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *MicrofrontendVersionsService {
	return &MicrofrontendVersionsService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (s *MicrofrontendVersionsService) CreateVersion(ctx context.Context, req *nb.CreateMicrofrontendVersionRequest) (*nb.MicrofrontendVersion, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_microfrontend_versions.CreateVersion", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateMicrofrontendVersion--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.MicrofrontendVersions().Create(ctx, req)
	if err != nil {
		s.log.Error("---CreateMicrofrontendVersion--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *MicrofrontendVersionsService) GetVersionList(ctx context.Context, req *nb.GetMicrofrontendVersionListRequest) (*nb.GetMicrofrontendVersionListResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_microfrontend_versions.GetVersionList", req)
	defer dbSpan.Finish()

	s.log.Info("---GetMicrofrontendVersionList--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.MicrofrontendVersions().GetList(ctx, req)
	if err != nil {
		s.log.Error("---GetMicrofrontendVersionList--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *MicrofrontendVersionsService) GetVersion(ctx context.Context, req *nb.GetMicrofrontendVersionRequest) (*nb.MicrofrontendVersion, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_microfrontend_versions.GetVersion", req)
	defer dbSpan.Finish()

	s.log.Info("---GetMicrofrontendVersion--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.MicrofrontendVersions().GetVersion(ctx, req)
	if err != nil {
		s.log.Error("---GetMicrofrontendVersion--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *MicrofrontendVersionsService) SetCurrentVersion(ctx context.Context, req *nb.SetCurrentMicrofrontendVersionRequest) (*nb.MicrofrontendVersion, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_microfrontend_versions.SetCurrentVersion", req)
	defer dbSpan.Finish()

	s.log.Info("---SetCurrentMicrofrontendVersion--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.MicrofrontendVersions().SetCurrent(ctx, req)
	if err != nil {
		s.log.Error("---SetCurrentMicrofrontendVersion--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

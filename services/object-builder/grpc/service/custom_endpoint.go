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

type customEndpointService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedCustomEndpointServiceServer
}

func NewCustomEndpointService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *customEndpointService {
	return &customEndpointService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (s *customEndpointService) Create(ctx context.Context, req *nb.CreateCustomEndpointRequest) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_endpoint.Create", req)
	defer dbSpan.Finish()

	s.log.Info("--- CustomEndpoint.Create --->", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.CustomEndpoint().Create(ctx, req)
	if err != nil {
		s.log.Error("--- CustomEndpoint.Create --->", logger.Error(err))
		return nil, err
	}
	return resp, nil
}

func (s *customEndpointService) Update(ctx context.Context, req *nb.CustomEndpoint) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_endpoint.Update", req)
	defer dbSpan.Finish()

	s.log.Info("--- CustomEndpoint.Update --->", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.CustomEndpoint().Update(ctx, req)
	if err != nil {
		s.log.Error("--- CustomEndpoint.Update --->", logger.Error(err))
		return nil, err
	}
	return resp, nil
}

func (s *customEndpointService) GetAll(ctx context.Context, req *nb.GetCustomEndpointListRequest) (*nb.CustomEndpointList, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_endpoint.GetAll", req)
	defer dbSpan.Finish()

	s.log.Info("--- CustomEndpoint.GetAll --->", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.CustomEndpoint().GetAll(ctx, req)
	if err != nil {
		s.log.Error("--- CustomEndpoint.GetAll --->", logger.Error(err))
		return nil, err
	}
	return resp, nil
}

func (s *customEndpointService) GetById(ctx context.Context, req *nb.CustomEndpointId) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_endpoint.GetById", req)
	defer dbSpan.Finish()

	s.log.Info("--- CustomEndpoint.GetById --->", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.CustomEndpoint().GetById(ctx, req)
	if err != nil {
		s.log.Error("--- CustomEndpoint.GetById --->", logger.Error(err))
		return nil, err
	}
	return resp, nil
}

func (s *customEndpointService) Delete(ctx context.Context, req *nb.CustomEndpointId) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_endpoint.Delete", req)
	defer dbSpan.Finish()

	s.log.Info("--- CustomEndpoint.Delete --->", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.CustomEndpoint().Delete(ctx, req)
	if err != nil {
		s.log.Error("--- CustomEndpoint.Delete --->", logger.Error(err))
		return nil, err
	}
	return resp, nil
}

func (s *customEndpointService) Run(ctx context.Context, req *nb.RunCustomEndpointRequest) (*nb.RunCustomEndpointResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_endpoint.Run", req)
	defer dbSpan.Finish()

	s.log.Info("--- CustomEndpoint.Run --->", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.CustomEndpoint().Run(ctx, req)
	if err != nil {
		s.log.Error("--- CustomEndpoint.Run --->", logger.Error(err))
		return nil, err
	}
	return resp, nil
}

package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/jackc/pgx/v5"
	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type apiKeyUsageService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedApiKeyUsageServiceServer
}

func NewApiKeyUsageService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *apiKeyUsageService {
	return &apiKeyUsageService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (s *apiKeyUsageService) CheckLimit(ctx context.Context, req *pb.CheckLimitRequest) (*pb.CheckLimitResponse, error) {
	s.log.Info("---CheckLimitApiKeyUsage--->", logger.Any("req", req))

	res, err := s.strg.ApiKeyUsage().CheckLimit(ctx, req)
	if err != nil {
		s.log.Error("!!!CheckLimitApiKeyUsage--->", logger.Error(err))
		return nil, status.Error(codes.Internal, "error on creating new api key")
	}

	if res.IsLimitReached {
		return res, status.Error(codes.ResourceExhausted, "monthly limit exceeded")
	}

	return res, nil
}

func (s *apiKeyUsageService) Create(ctx context.Context, req *pb.ApiKeyUsage) (*emptypb.Empty, error) {
	s.log.Info("---CreateApiKeyUsage--->", logger.Any("req", req))

	res := &emptypb.Empty{}

	err := s.strg.ApiKeyUsage().Create(ctx, req)
	if err == pgx.ErrNoRows {
		s.log.Error("!!!CreateApiKeyUsage--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, "api-key not found")
	}
	if err != nil {
		s.log.Error("!!!CreateApiKeyUsage--->", logger.Error(err))
		return nil, status.Error(codes.Internal, "error on creating api key usage")
	}

	return res, nil
}

func (s *apiKeyUsageService) Upsert(ctx context.Context, req *pb.ApiKeyUsage) (*emptypb.Empty, error) {
	s.log.Info("---UpsertApiKeyUsage--->", logger.Any("req", req))

	res := &emptypb.Empty{}

	err := s.strg.ApiKeyUsage().Upsert(ctx, req)
	if err == pgx.ErrNoRows {
		s.log.Error("!!!UpsertApiKeyUsage--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, "api-key not found")
	}
	if err != nil {
		s.log.Error("!!!UpsertApiKeyUsage--->", logger.Error(err))
		return nil, status.Error(codes.Internal, "error on upserting api key usage")
	}

	return res, nil
}

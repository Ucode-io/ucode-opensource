package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/saidamir98/udevs_pkg/logger"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type appleIdService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedAppleIdLoginServiceServer
}

func NewAppleSettingsService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *appleIdService {
	return &appleIdService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (e *appleIdService) CreateAppleIdSettings(ctx context.Context, req *pb.AppleIdSettings) (*pb.AppleIdSettings, error) {
	e.log.Info("---AppleIdSettings.CreateAppleIdSettings--->", logger.Any("req", req))

	res, err := e.strg.AppleSettings().Create(ctx, req)
	if err != nil {
		e.log.Error("!!!---AppleIdSettings.CreateAppleIdSettings--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return res, nil
}

func (e *appleIdService) UpdateAppleIdSettings(ctx context.Context, req *pb.AppleIdSettings) (*pb.AppleIdSettings, error) {
	e.log.Info("---AppleIdSettings.UpdateAppleIdSettings--->", logger.Any("req", req))

	id, err := e.strg.AppleSettings().UpdateAppleSettings(ctx, req)
	if err != nil {
		e.log.Error("!!!---AppleIdSettings.UpdateAppleIdSettings--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res, err := e.strg.AppleSettings().GetByPK(ctx, &pb.AppleIdSettingsPrimaryKey{Id: id})
	if err != nil {
		e.log.Error("!!!---AppleIdSettings.UpdateAppleIdSettings--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return res, nil
}

func (e *appleIdService) GetListAppleIdSettings(ctx context.Context, req *pb.GetListAppleIdSettingsRequest) (*pb.GetListAppleIdSettingsResponse, error) {
	e.log.Info("---AppleIdSettings.GetListAppleIdSettings--->", logger.Any("req", req))

	res, err := e.strg.AppleSettings().GetListAppleSettings(ctx, req)
	if err != nil {
		e.log.Error("!!!---AppleIdSettings.GetListAppleIdSettings--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return res, nil
}

func (e *appleIdService) DeleteAppleIdSettings(ctx context.Context, req *pb.AppleIdSettingsPrimaryKey) (*emptypb.Empty, error) {
	e.log.Info("---AppleIdSettingsService.DeleteAppleIdSettings--->", logger.Any("req", req))

	res, err := e.strg.AppleSettings().DeleteAppleSettings(ctx, req)

	if err != nil {
		e.log.Error("!!!AppleIdSettingsService.DeleteAppleId--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return res, nil
}

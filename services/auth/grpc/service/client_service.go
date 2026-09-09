package service

import (
	"context"
	"errors"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/saidamir98/udevs_pkg/logger"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type clientService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedClientServiceServer
}

func NewClientService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *clientService {
	return &clientService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (s *clientService) CreateClientPlatform(ctx context.Context, req *pb.CreateClientPlatformRequest) (*pb.ClientPlatform, error) {
	s.log.Info("---CreateClientPlatform--->", logger.Any("req", req))

	pKey, err := s.strg.ClientPlatform().Create(ctx, req)
	if err != nil {
		s.log.Error("!!!CreateClientPlatform--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return s.strg.ClientPlatform().GetByPK(ctx, pKey)
}

func (s *clientService) GetClientPlatformByID(ctx context.Context, req *pb.ClientPlatformPrimaryKey) (*pb.ClientPlatform, error) {
	s.log.Info("---GetClientPlatformByID--->", logger.Any("req", req))

	res, err := s.strg.ClientPlatform().GetByPK(ctx, req)
	if err != nil {
		s.log.Error("!!!GetClientPlatformByID--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return res, nil
}

func (s *clientService) GetClientPlatformList(ctx context.Context, req *pb.GetClientPlatformListRequest) (*pb.GetClientPlatformListResponse, error) {
	s.log.Info("---GetClientPlatformList--->", logger.Any("req", req))

	res, err := s.strg.ClientPlatform().GetList(ctx, req)
	if err != nil {
		s.log.Error("!!!GetClientPlatformList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, err
}

func (s *clientService) CreateClientType(ctx context.Context, req *pb.CreateClientTypeRequest) (*pb.ClientType, error) {
	s.log.Info("---CreateClientType--->", logger.Any("req", req))

	pKey, err := s.strg.ClientType().Create(ctx, req)
	if err != nil {
		s.log.Error("!!!CreateClientType--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return s.strg.ClientType().GetByPK(ctx, pKey)
}

func (s *clientService) GetClientTypeByID(ctx context.Context, req *pb.ClientTypePrimaryKey) (*pb.CompleteClientType, error) {
	s.log.Info("---GetClientTypeByID--->", logger.Any("req", req))

	res, err := s.strg.ClientType().GetCompleteByPK(ctx, req)
	if err != nil {
		s.log.Error("!!!GetClientTypeByID--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return res, nil
}

func (s *clientService) GetClientTypeList(ctx context.Context, req *pb.GetClientTypeListRequest) (*pb.GetClientTypeListResponse, error) {
	s.log.Info("---GetClientTypeList--->", logger.Any("req", req))

	res, err := s.strg.ClientType().GetList(ctx, req)
	if err != nil {
		s.log.Error("!!!GetClientTypeList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	services, err := s.serviceNode.GetByNodeType(req.ProjectId, "LOW")
	if err != nil {
		return nil, err
	}
	
	for i, el := range res.ClientTypes {

		for e, table := range el.Tables {
			var message = make(map[string]any)
			message["table_slug"] = table.Slug

			structData, err := helper.ConvertMapToStruct(message)
			if err != nil {
				s.log.Error("!!!GetClientTypeList.ConvertMapToStruct--->", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}
			resp, err := services.ObjectBuilderService().GetList(ctx, &object_builder_service.CommonMessage{
				TableSlug: table.Slug,
				Data:      structData,
				ProjectId: config.ProjectID,
			})

			if err != nil {
				s.log.Error("!!!GetClientTypeList.ObjectBuilderService.GetList--->", logger.Error(err))
				return nil, status.Error(codes.Internal, err.Error())
			}

			res.ClientTypes[i].Tables[e].Data = resp.Data
		}

	}

	return res, err
}

func (s *clientService) UpdateClientType(ctx context.Context, req *pb.UpdateClientTypeRequest) (*pb.ClientType, error) {
	s.log.Info("---UpdateClientType--->", logger.Any("req", req))

	rowsAffected, err := s.strg.ClientType().Update(ctx, req)
	if err != nil {
		s.log.Error("!!!UpdateClientType--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if rowsAffected <= 0 {
		return nil, status.Error(codes.InvalidArgument, "no rows were affected")
	}

	res, err := s.strg.ClientType().GetByPK(ctx, &pb.ClientTypePrimaryKey{Id: req.Id})
	if err != nil {
		s.log.Error("!!!UpdateClientType--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return res, err
}

func (s *clientService) DeleteClientType(ctx context.Context, req *pb.ClientTypePrimaryKey) (*emptypb.Empty, error) {
	s.log.Info("---DeleteClientType--->", logger.Any("req", req))

	rowsAffected, err := s.strg.ClientType().Delete(ctx, req)
	if err != nil {
		s.log.Error("!!!DeleteClientType--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if rowsAffected <= 0 {
		return nil, status.Error(codes.InvalidArgument, "no rows were affected")
	}

	return &emptypb.Empty{}, nil
}

func (s *clientService) AddClient(ctx context.Context, req *pb.AddClientRequest) (*pb.Client, error) {
	s.log.Info("---AddClient--->", logger.Any("req", req))

	clientPlatform, err := s.strg.ClientPlatform().GetByPK(ctx, &pb.ClientPlatformPrimaryKey{Id: req.ClientPlatformId})
	if err != nil {
		s.log.Error("!!!AddClient--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	clientType, err := s.strg.ClientType().GetByPK(ctx, &pb.ClientTypePrimaryKey{Id: req.ClientTypeId})
	if err != nil {
		s.log.Error("!!!AddClient--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if clientPlatform.ProjectId != clientType.ProjectId {
		err = errors.New("mismatch between client platform&client project_id")
		s.log.Error("!!!AddClient--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	err = s.strg.Client().Add(ctx, clientPlatform.ProjectId, req)
	if err != nil {
		s.log.Error("!!!AddClient--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	return s.strg.Client().GetByPK(ctx, &pb.ClientPrimaryKey{
		ClientPlatformId: req.ClientPlatformId,
		ClientTypeId:     req.ClientTypeId,
	})
}

func (s *clientService) GetClientList(ctx context.Context, req *pb.GetClientListRequest) (*pb.GetClientListResponse, error) {
	s.log.Info("---GetClientList--->", logger.Any("req", req))

	res, err := s.strg.Client().GetList(ctx, req)

	if err != nil {
		s.log.Error("!!!GetClientList--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, err
}

func (s *clientService) GetClientMatrix(ctx context.Context, req *pb.GetClientMatrixRequest) (*pb.GetClientMatrixResponse, error) {
	s.log.Info("---GetClientMatrix--->", logger.Any("req", req))

	res, err := s.strg.Client().GetMatrix(ctx, req)

	if err != nil {
		s.log.Error("!!!GetClientMatrix--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	return res, err
}

func (s *clientService) UpdateClient(ctx context.Context, req *pb.UpdateClientRequest) (*pb.Client, error) {
	s.log.Info("---UpdateClient--->", logger.Any("req", req))

	rowsAffected, err := s.strg.Client().Update(ctx, req)

	if err != nil {
		s.log.Error("!!!UpdateClient--->", logger.Error(err))
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	if rowsAffected <= 0 {
		return nil, status.Error(codes.InvalidArgument, "no rows were affected")
	}

	res, err := s.strg.Client().GetByPK(ctx, &pb.ClientPrimaryKey{
		ClientPlatformId: req.ClientPlatformId,
		ClientTypeId:     req.ClientTypeId,
	})

	if err != nil {
		s.log.Error("!!!UpdateClient--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	return res, err
}

func (s *clientService) RemoveClient(ctx context.Context, req *pb.ClientPrimaryKey) (*pb.Client, error) {
	s.log.Info("---RemoveClient--->", logger.Any("req", req))

	res, err := s.strg.Client().GetByPK(ctx, req)

	if err != nil {
		s.log.Error("!!!GetClientPlatformByID--->", logger.Error(err))
		return nil, status.Error(codes.NotFound, err.Error())
	}

	rowsAffected, err := s.strg.Client().Remove(ctx, req)

	if err != nil {
		s.log.Error("!!!RemoveClient--->", logger.Error(err))
		return nil, status.Error(codes.Internal, err.Error())
	}

	if rowsAffected <= 0 {
		return nil, status.Error(codes.InvalidArgument, "no rows were affected")
	}

	return res, nil
}

package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/saidamir98/udevs_pkg/logger"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"google.golang.org/protobuf/types/known/emptypb"
)

type projectService struct {
	cfg         config.BaseConfig
	log         logger.LoggerI
	strg        storage.StorageI
	services    client.ServiceManagerI
	serviceNode ServiceNodesI
	pb.UnimplementedProjectServiceServer
}

func NewProjectService(cfg config.BaseConfig, log logger.LoggerI, strg storage.StorageI, svcs client.ServiceManagerI, projectServiceNodes ServiceNodesI) *projectService {
	return &projectService{
		cfg:         cfg,
		log:         log,
		strg:        strg,
		services:    svcs,
		serviceNode: projectServiceNodes,
	}
}

func (s *projectService) Create(ctx context.Context, req *pb.CreateProjectRequest) (*pb.ProjectPrimaryKey, error) {
	res, err := s.strg.Project().Create(ctx, req)
	if err != nil {
		s.log.Error("---CreateProject--->", logger.Error(err))
		return nil, err
	}

	return res, nil
}

func (s *projectService) GetByPK(ctx context.Context, req *pb.ProjectPrimaryKey) (*pb.Project, error) {
	res, err := s.strg.Project().GetByPK(ctx, req)
	if err != nil {
		s.log.Error("---GetByPKProject--->", logger.Error(err))
		return nil, err
	}

	return res, nil
}

func (s *projectService) GetList(ctx context.Context, req *pb.GetProjectListRequest) (*pb.GetProjectListResponse, error) {
	res, err := s.strg.Project().GetList(ctx, req)
	if err != nil {
		s.log.Error("---GetListProject--->", logger.Error(err))
		return nil, err
	}

	return res, nil
}

func (s *projectService) Update(ctx context.Context, req *pb.UpdateProjectRequest) (*emptypb.Empty, error) {
	_, err := s.strg.Project().Update(ctx, req)
	if err != nil {
		s.log.Error("---UpdateProject--->", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *projectService) Delete(ctx context.Context, req *pb.ProjectPrimaryKey) (*emptypb.Empty, error) {
	_, err := s.strg.Project().Delete(ctx, req)
	if err != nil {
		s.log.Error("---DeleteProject--->", logger.Error(err))
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

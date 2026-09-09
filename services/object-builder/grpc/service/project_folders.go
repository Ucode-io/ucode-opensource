package service

import (
	"context"

	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type ProjectFoldersService struct {
	nb.UnimplementedProjectFoldersServiceServer
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
}

func NewProjectFoldersService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *ProjectFoldersService {
	return &ProjectFoldersService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (s *ProjectFoldersService) CreateProjectFolder(ctx context.Context, req *nb.CreateProjectFolderRequest) (*nb.ProjectFolder, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project_folders.CreateProjectFolder", req)
	defer dbSpan.Finish()

	s.log.Info("---CreateProjectFolder--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.ProjectFolders().CreateProjectFolder(ctx, req)
	if err != nil {
		s.log.Error("---CreateProjectFolder--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *ProjectFoldersService) GetProjectFolderById(ctx context.Context, req *nb.ProjectFolderPrimaryKey) (*nb.ProjectFolder, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project_folders.GetProjectFolderById", req)
	defer dbSpan.Finish()

	s.log.Info("---GetProjectFolderById--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.ProjectFolders().GetProjectFolderById(ctx, req)
	if err != nil {
		s.log.Error("---GetProjectFolderById--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *ProjectFoldersService) GetAllProjectFolders(ctx context.Context, req *nb.GetAllProjectFoldersRequest) (*nb.GetAllProjectFoldersResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project_folders.GetAllProjectFolders", req)
	defer dbSpan.Finish()

	s.log.Info("---GetAllProjectFolders--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.ProjectFolders().GetAllProjectFolders(ctx, req)
	if err != nil {
		s.log.Error("---GetAllProjectFolders--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *ProjectFoldersService) UpdateProjectFolder(ctx context.Context, req *nb.UpdateProjectFolderRequest) (*nb.ProjectFolder, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project_folders.UpdateProjectFolder", req)
	defer dbSpan.Finish()

	s.log.Info("---UpdateProjectFolder--->>>", logger.Any("request", compactRequest(req)))

	resp, err := s.strg.ProjectFolders().UpdateProjectFolder(ctx, req)
	if err != nil {
		s.log.Error("---UpdateProjectFolder--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (s *ProjectFoldersService) DeleteProjectFolder(ctx context.Context, req *nb.ProjectFolderPrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project_folders.DeleteProjectFolder", req)
	defer dbSpan.Finish()

	s.log.Info("---DeleteProjectFolder--->>>", logger.Any("request", compactRequest(req)))

	err := s.strg.ProjectFolders().DeleteProjectFolder(ctx, req)
	if err != nil {
		s.log.Error("---DeleteProjectFolder--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (s *ProjectFoldersService) UpdateProjectFolderOrder(ctx context.Context, req *nb.UpdateProjectFolderOrderRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_project_folders.UpdateProjectFolderOrder", req)
	defer dbSpan.Finish()

	s.log.Info("---UpdateProjectFolderOrder--->>>", logger.Any("request", compactRequest(req)))

	err := s.strg.ProjectFolders().UpdateProjectFolderOrder(ctx, req)
	if err != nil {
		s.log.Error("---UpdateProjectFolderOrder--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

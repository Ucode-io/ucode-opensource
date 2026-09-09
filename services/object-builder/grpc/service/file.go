package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"google.golang.org/protobuf/types/known/emptypb"
)

type fileService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedFileServiceServer
}

func NewFileService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *fileService { // ,
	return &fileService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *fileService) Create(ctx context.Context, req *nb.CreateFileRequest) (resp *nb.File, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_file.Create", req)
	defer dbSpan.Finish()

	f.log.Info("---CreateFile--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.File().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateFile--->>>", logger.Error(err))
		return &nb.File{}, err
	}

	return resp, nil
}

func (f *fileService) GetSingle(ctx context.Context, req *nb.FilePrimaryKey) (resp *nb.File, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_file.GetSingle", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByIDFile--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.File().GetSingle(ctx, req)
	if err != nil {
		f.log.Error("---GetByIDFile--->>>", logger.Error(err))
		return &nb.File{}, err
	}

	return resp, nil
}

func (f *fileService) GetList(ctx context.Context, req *nb.GetAllFilesRequest) (resp *nb.GetAllFilesResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_file.GetList", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllFile--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.File().GetList(ctx, req)
	if err != nil {
		f.log.Error("---GetAllFile--->>>", logger.Error(err))
		return &nb.GetAllFilesResponse{}, err
	}

	return resp, nil
}

func (f *fileService) Update(ctx context.Context, req *nb.File) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_file.Update", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateFile--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.File().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateFile--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return resp, nil
}

func (f *fileService) Delete(ctx context.Context, req *nb.FileDeleteRequest) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_file.Delete", req)
	defer dbSpan.Finish()

	f.log.Info("---DeleteFile--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.File().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteFile--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

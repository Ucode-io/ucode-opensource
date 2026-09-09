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

type versionService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedVersionServiceServer
}

func NewVersionService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *versionService { // ,
	return &versionService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *versionService) Create(ctx context.Context, req *nb.CreateVersionRequest) (resp *nb.Version, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_version.Create", req)
	defer dbSpan.Finish()

	f.log.Info("---CreateFunction--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Version().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateFunction--->>>", logger.Error(err))
		return &nb.Version{}, err
	}

	return resp, nil
}

func (f *versionService) GetList(ctx context.Context, req *nb.GetVersionListRequest) (resp *nb.GetVersionListResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_version.GetList", req)
	defer dbSpan.Finish()

	f.log.Info("---GetListFunction--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Version().GetList(ctx, req)
	if err != nil {
		f.log.Error("---GetListFunction--->>>", logger.Error(err))
		return &nb.GetVersionListResponse{}, err
	}

	return resp, nil
}

func (f *versionService) GetSingle(ctx context.Context, req *nb.VersionPrimaryKey) (resp *nb.Version, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_version.GetSingle", req)
	defer dbSpan.Finish()

	f.log.Info("---GetSingleFunction--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Version().GetSingle(ctx, req)
	if err != nil {
		f.log.Error("---GetSingleFunction--->>>", logger.Error(err))
		return &nb.Version{}, err
	}

	return resp, nil
}

func (f *versionService) Update(ctx context.Context, req *nb.Version) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_version.Update", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateFunction--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Version().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateFunction--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *versionService) Delete(ctx context.Context, req *nb.VersionPrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_version.Delete", req)
	defer dbSpan.Finish()

	f.log.Info("---DeleteFunction--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Version().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteFunction--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

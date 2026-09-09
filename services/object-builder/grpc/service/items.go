package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type itemsService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedItemsServiceServer
}

func NewItemsService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *itemsService {
	return &itemsService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (i *itemsService) Create(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.Create", req)
	defer dbSpan.Finish()

	i.log.Info("---CreateItems--->>>", logger.Any("request", compactRequest(req)))

	resp, err = i.strg.Items().Create(ctx, req)
	if err != nil {
		i.log.Error("---CreateItems--->>> !!!", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return resp, nil
}

func (i *itemsService) GetSingle(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.GetSingle", req)
	defer dbSpan.Finish()

	i.log.Info("---GetSingleItems--->>>", logger.Any("request", compactRequest(req)))

	resp, err = i.strg.Items().GetSingle(ctx, req)
	if err != nil {
		i.log.Error("---GetSingleItems--->>>", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return resp, nil
}

func (i *itemsService) Update(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.Update", req)
	defer dbSpan.Finish()

	i.log.Info("---UpdateItems--->>>", logger.Any("request", compactRequest(req)))

	resp, err = i.strg.Items().Update(ctx, req)
	if err != nil {
		i.log.Error("---UpdateItems--->>>", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return resp, nil
}

func (i *itemsService) Delete(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.Delete", req)
	defer dbSpan.Finish()

	i.log.Info("---DeleteItems--->", logger.Any("request", compactRequest(req)))

	resp, err = i.strg.Items().Delete(ctx, req)
	if err != nil {
		i.log.Error("---DeleteItems--->", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return resp, nil
}

func (i *itemsService) DeleteMany(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.DeleteMany", req)
	defer dbSpan.Finish()

	i.log.Info("---DeleteItems--->>>", logger.Any("request", compactRequest(req)))

	_, err = i.strg.Items().DeleteMany(ctx, req)
	if err != nil {
		i.log.Error("---DeleteItems--->>>", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return &nb.CommonMessage{}, err
}

func (i *itemsService) MultipleUpdate(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	i.log.Info("!!!MultipleUpdate--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.MultipleUpdate", req)
	defer dbSpan.Finish()

	resp, err = i.strg.Items().MultipleUpdate(ctx, req)
	if err != nil {
		i.log.Error("---MultipleUpdateItems--->>>", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return resp, nil
}

func (i *itemsService) UpsertMany(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	i.log.Info("---UpdateItems--->>>", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.UpsertMany", req)
	defer dbSpan.Finish()

	if err = i.strg.Items().UpsertMany(ctx, req); err != nil {
		i.log.Error("---UpsertMany--->>>", logger.Error(err))
		return &nb.CommonMessage{}, err
	}

	return &nb.CommonMessage{}, nil
}

func (i *itemsService) UpdateByUserIdAuth(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_items.Update", req)
	defer dbSpan.Finish()

	i.log.Info("---UpdateItems--->>>", logger.Any("request", compactRequest(req)))

	resp, err = i.strg.Items().UpdateByUserIdAuth(ctx, req)
	if err != nil {
		i.log.Error("---UpdateItems--->>>", logger.Error(err))
		return &nb.CommonMessage{}, status.Error(codes.Internal, err.Error())
	}

	return resp, nil
}

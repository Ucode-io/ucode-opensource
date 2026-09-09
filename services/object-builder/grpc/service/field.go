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

type fieldService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedFieldServiceServer
}

func NewFieldService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *fieldService { // ,
	return &fieldService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *fieldService) Create(ctx context.Context, req *nb.CreateFieldRequest) (resp *nb.Field, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.Create", req)
	defer dbSpan.Finish()

	f.log.Info("---CreateField--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Field().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateField--->>>", logger.Error(err))
		return &nb.Field{}, err
	}

	return resp, nil
}

func (f *fieldService) GetByID(ctx context.Context, req *nb.FieldPrimaryKey) (resp *nb.Field, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.GetByID", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByIDField--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Field().GetByID(ctx, req)
	if err != nil {
		f.log.Error("---GetByIDField--->>>", logger.Error(err))
		return &nb.Field{}, err
	}

	return resp, nil
}

func (f *fieldService) GetAll(ctx context.Context, req *nb.GetAllFieldsRequest) (resp *nb.GetAllFieldsResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.GetAll", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllField--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Field().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetAllField--->>>", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (f *fieldService) GetAllForItems(ctx context.Context, req *nb.GetAllFieldsForItemsRequest) (resp *nb.AllFields, err error) {
	return &nb.AllFields{}, nil
}

func (f *fieldService) Update(ctx context.Context, req *nb.Field) (resp *nb.Field, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.Update", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateField--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Field().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateField--->>>", logger.Error(err))
		return &nb.Field{}, err
	}

	return resp, nil
}

func (f *fieldService) UpdateSearch(ctx context.Context, req *nb.SearchUpdateRequest) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.UpdateSearch", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateSearchField--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Field().UpdateSearch(ctx, req)
	if err != nil {
		f.log.Error("---UpdateSearchField--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *fieldService) Delete(ctx context.Context, req *nb.FieldPrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.Delete", req)
	defer dbSpan.Finish()

	f.log.Info("---DeleteField--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Field().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteField--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *fieldService) FieldsWithRelations(ctx context.Context, req *nb.FieldsWithRelationRequest) (resp *nb.FieldsWithRelationsResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.FieldsWithRelations", req)
	defer dbSpan.Finish()

	f.log.Info("---FieldsWithRelations--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Field().FieldsWithPermissions(ctx, req)
	if err != nil {
		f.log.Error("---FieldsWithRelations--->>>", logger.Error(err))
		return &nb.FieldsWithRelationsResponse{}, err
	}

	return resp, nil
}

func (f *fieldService) ObtainRandomOne(ctx context.Context, req *nb.ObtainRandomRequest) (resp *nb.ObtainRandomResponse, err error) {
	f.log.Info("!!!ObtainRandomOne-->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_field.ObtainRandomOne", req)
	defer dbSpan.Finish()

	resp, err = f.strg.Field().ObtainRandomOne(ctx, req)
	if err != nil {
		f.log.Error("!!!ObtainRandomOne-->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

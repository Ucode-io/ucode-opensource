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

type functionService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedFunctionServiceV2Server
}

func NewFunctionService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *functionService { // ,
	return &functionService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *functionService) Create(ctx context.Context, req *nb.CreateFunctionRequest) (resp *nb.Function, err error) {
	f.log.Info("---CreateFunction--->>>", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_function.Create", req)
	defer dbSpan.Finish()

	resp, err = f.strg.Function().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateFunction--->>>", logger.Error(err))
		return &nb.Function{}, err
	}

	return resp, nil
}

func (f *functionService) GetList(ctx context.Context, req *nb.GetAllFunctionsRequest) (resp *nb.GetAllFunctionsResponse, err error) {
	f.log.Info("---GetListFunction--->>>", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_function.GetList", req)
	defer dbSpan.Finish()

	resp, err = f.strg.Function().GetList(ctx, req)
	if err != nil {
		f.log.Error("---GetListFunction--->>>", logger.Error(err))
		return &nb.GetAllFunctionsResponse{}, err
	}

	return resp, nil
}

func (f *functionService) GetSingle(ctx context.Context, req *nb.FunctionPrimaryKey) (resp *nb.Function, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_function.GetSingle", req)
	defer dbSpan.Finish()

	f.log.Info("---GetSingleFunction--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Function().GetSingle(ctx, req)
	if err != nil {
		f.log.Error("---GetSingleFunction--->>>", logger.Error(err))
		return &nb.Function{}, err
	}

	return resp, nil
}

func (f *functionService) Update(ctx context.Context, req *nb.Function) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_function.Update", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateFunction--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Function().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateFunction--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *functionService) Delete(ctx context.Context, req *nb.FunctionPrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_function.Delete", req)
	defer dbSpan.Finish()

	f.log.Info("---DeleteFunction--->>>", logger.Any("request", compactRequest(req)))

	err = f.strg.Function().Delete(ctx, req)
	if err != nil {
		f.log.Error("---DeleteFunction--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (f *functionService) GetCountByType(ctx context.Context, req *nb.GetCountByTypeRequest) (resp *nb.GetCountByTypeResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_function.GetCountByType", req)
	defer dbSpan.Finish()

	f.log.Info("---GetCountByTypeFunction--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Function().GetCountByType(ctx, req)
	if err != nil {
		f.log.Error("---GetCountByTypeFunction--->>>", logger.Error(err))
		return &nb.GetCountByTypeResponse{}, err
	}

	return resp, nil
}

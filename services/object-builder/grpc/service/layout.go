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

type layoutService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedLayoutServiceServer
}

func NewLayoutService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *layoutService { // ,
	return &layoutService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *layoutService) Update(ctx context.Context, req *nb.LayoutRequest) (resp *nb.LayoutResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_layout.Update", req)
	defer dbSpan.Finish()

	resp, err = f.strg.Layout().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateLayout--->>>", logger.Error(err))
		return &nb.LayoutResponse{}, err
	}

	return resp, nil
}

func (f *layoutService) GetAll(ctx context.Context, req *nb.GetListLayoutRequest) (resp *nb.GetListLayoutResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_layout.GetAll", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllLayouts--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Layout().GetAllV2(ctx, req)
	if err != nil {
		f.log.Error("---GetAllLayouts--->>>", logger.Error(err))
		return &nb.GetListLayoutResponse{}, err
	}

	return resp, nil

}

func (f *layoutService) GetByID(ctx context.Context, req *nb.LayoutPrimaryKey) (resp *nb.LayoutResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_layout.GetByID", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByIDLayout--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Layout().GetByID(ctx, req)
	if err != nil {
		f.log.Error("---GetByIDLayout--->>>", logger.Error(err))
		return &nb.LayoutResponse{}, err
	}

	return resp, nil

}

func (f *layoutService) GetSingleLayout(ctx context.Context, req *nb.GetSingleLayoutRequest) (resp *nb.LayoutResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_layout.GetSingleLayout", req)
	defer dbSpan.Finish()

	f.log.Info("---GetSingleLayout--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.Layout().GetSingleLayout(ctx, req)
	if err != nil {
		f.log.Error("---GetSingleLayout--->>>", logger.Error(err))
		return &nb.LayoutResponse{}, err
	}

	return resp, nil

}

func (f *layoutService) RemoveLayout(ctx context.Context, req *nb.LayoutPrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_layout.RemoveLayout", req)
	defer dbSpan.Finish()

	f.log.Info("---RemvoeLayout--->>>", logger.Any("request", compactRequest(req)))

	err := f.strg.Layout().RemoveLayout(ctx, req)
	if err != nil {
		f.log.Error("---RemoveLayout--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

// GetLayoutByTableID retrieves all layouts with tabs and sections by table ID
func (f *layoutService) GetLayoutByTableID(ctx context.Context, req *nb.GetLayoutByTableIDRequest) (resp *nb.GetListLayoutResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_layout.GetLayoutByTableID", req)
	defer dbSpan.Finish()

	f.log.Info("---GetLayoutByTableID--->>>", logger.Any("request", compactRequest(req)))

	layouts, err := f.strg.Layout().GetLayoutByTableID(ctx, req.GetTableId(), req.GetProjectId())
	if err != nil {
		f.log.Error("---GetLayoutByTableID--->>>", logger.Error(err))
		return &nb.GetListLayoutResponse{}, err
	}

	return &nb.GetListLayoutResponse{
		Layouts: layouts,
	}, nil
}

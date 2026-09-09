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

type customEventService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedCustomEventServiceServer
}

func NewCustomEventService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *customEventService {
	return &customEventService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (c *customEventService) Create(ctx context.Context, req *nb.CreateCustomEventRequest) (resp *nb.CustomEvent, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_event.Create", req)
	defer dbSpan.Finish()
	c.log.Info("---CreateCustomEvent--->>>", logger.Any("request", compactRequest(req)))

	resp, err = c.strg.CustomEvent().Create(ctx, req)
	if err != nil {
		c.log.Error("---CreateCustomEvent--->>>", logger.Any("error", err))
		return &nb.CustomEvent{}, err
	}

	return resp, nil
}

func (c *customEventService) Update(ctx context.Context, req *nb.CustomEvent) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_event.Update", req)
	defer dbSpan.Finish()

	c.log.Info("---UpdateCustomEvent--->>>", logger.Any("request", compactRequest(req)))

	err = c.strg.CustomEvent().Update(ctx, req)
	if err != nil {
		c.log.Error("---UpdateCustomEvent--->>>", logger.Any("error", err))
		return resp, err
	}

	return resp, nil
}

func (c *customEventService) GetList(ctx context.Context, req *nb.GetCustomEventsListRequest) (resp *nb.GetCustomEventsListResponse, err error) {
	c.log.Info("!!!GetListCustomEvent--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_event.GetList", req)
	defer dbSpan.Finish()

	resp, err = c.strg.CustomEvent().GetList(ctx, req)
	if err != nil {
		c.log.Error("!!!GetListCustomEvent--->", logger.Any("error", err))
		return &nb.GetCustomEventsListResponse{}, err
	}

	return resp, nil
}

func (c *customEventService) GetSingle(ctx context.Context, req *nb.CustomEventPrimaryKey) (resp *nb.CustomEvent, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_event.GetSingle", req)
	defer dbSpan.Finish()

	c.log.Info("---GetSingleCustomEvent--->>>", logger.Any("request", compactRequest(req)))

	resp, err = c.strg.CustomEvent().GetSingle(ctx, req)
	if err != nil {
		c.log.Error("---GetSingleCustomEvent--->>>", logger.Any("error", err))
		return &nb.CustomEvent{}, err
	}

	return resp, nil
}

func (c *customEventService) Delete(ctx context.Context, req *nb.CustomEventPrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_event.Delete", req)
	defer dbSpan.Finish()

	c.log.Info("---DeleteCustomEvent--->>>", logger.Any("request", compactRequest(req)))

	err = c.strg.CustomEvent().Delete(ctx, req)
	if err != nil {
		c.log.Error("---DeleteCustomEvent--->>>", logger.Any("error", err))
		return resp, err
	}

	return resp, nil
}

func (c *customEventService) UpdateByFunctionId(ctx context.Context, req *nb.UpdateByFunctionIdRequest) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_custom_event.UpdateByFunctionId", req)
	defer dbSpan.Finish()

	c.log.Info("---UpdateByFunctionIdCustomEvent--->>>", logger.Any("request", compactRequest(req)))

	err = c.strg.CustomEvent().UpdateByFunctionId(ctx, req)
	if err != nil {
		c.log.Error("---UpdateByFunctionIdCustomEvent--->>>", logger.Any("error", err))
		return resp, err
	}

	return resp, nil
}

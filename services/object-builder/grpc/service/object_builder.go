package service

import (
	"context"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/golang/protobuf/ptypes/empty"
)

type objectBuilderService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedObjectBuilderServiceServer
}

func NewObjectBuilderService(strg storage.StorageI, cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI) *objectBuilderService {
	return &objectBuilderService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (b *objectBuilderService) GetList(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetList", req)
	defer dbSpan.Finish()

	b.log.Info("!!!ObjectBuilderGetList--->", logger.Any("request", compactRequest(req)))

	switch req.TableSlug {
	case "client_type":
		resp, err = b.strg.ObjectBuilder().GetList(ctx, req)
		if err != nil {
			b.log.Error("!!!ObjectBuilderGetList--->", logger.Error(err))
			return resp, err
		}
	case "connections":
		resp, err = b.strg.ObjectBuilder().GetListConnection(ctx, req)
		if err != nil {
			b.log.Error("!!!ObjectBuilderGetList--->", logger.Error(err))
			return resp, err
		}
	}

	return resp, nil
}

func (b *objectBuilderService) GetTableDetails(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	b.log.Info("!!!GetTableDetails--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetTableDetails", req)
	defer dbSpan.Finish()

	resp, err = b.strg.ObjectBuilder().GetTableDetails(ctx, req)
	if err != nil {
		b.log.Error("!!!GetTableDetails--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (b *objectBuilderService) GetAll(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	b.log.Info("!!!GetAll--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetTableDetails", req)
	defer dbSpan.Finish()

	resp, err = b.strg.ObjectBuilder().GetAll(ctx, req)
	if err != nil {
		b.log.Error("!!!GetAll--->", logger.Error(err))
		return resp, err
	}
	return resp, nil
}

func (b *objectBuilderService) GetList2(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetList2", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetList2--->", logger.Any("request", compactRequest(req)))

	if config.GetList2TableSlug[req.TableSlug] {
		resp, err = b.strg.ObjectBuilder().GetList2(ctx, req)
		if err != nil {
			b.log.Error("!!!GetList2--->", logger.Error(err))
			return resp, err
		}
	} else {
		resp, err = b.strg.ObjectBuilder().GetListV2(ctx, req)
		if err != nil {
			b.log.Error("!!!GetList2--->", logger.Error(err))
			return resp, err
		}
	}

	return resp, nil
}

func (b *objectBuilderService) GetListInExcel(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetListInExcel", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetListInExcel--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.ObjectBuilder().GetListInExcel(ctx, req)
	if err != nil {
		b.log.Error("!!!GetListInExcel--->GetList", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (b *objectBuilderService) GetListSlim(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetListSlim", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetListSlim--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.ObjectBuilder().GetListSlim(ctx, req)
	if err != nil {
		b.log.Error("!!!GetListSlim--->", logger.Error(err))
		return resp, err
	}
	return resp, nil
}

func (b *objectBuilderService) GetGroupByField(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetGroupByField", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GroupByColumns--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.ObjectBuilder().GroupByColumns(ctx, req)
	if err != nil {
		b.log.Error("!!!GroupByColumns--->", logger.Error(err))
		return resp, err
	}
	return resp, nil
}

func (b *objectBuilderService) UpdateWithParams(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.UpdateWithParams", req)
	defer dbSpan.Finish()

	b.log.Info("!!!UpdateWithParams--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.ObjectBuilder().UpdateWithParams(ctx, req)
	if err != nil {
		b.log.Error("!!!UpdateWithParams--->", logger.Error(err))
		return resp, err
	}
	return resp, nil
}

func (b *objectBuilderService) GetListForDocx(ctx context.Context, req *nb.CommonForDocxMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetListForDocx", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetListForDocx--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.ObjectBuilder().GetListForDocxMultiTables(ctx, req)
	if err != nil {
		b.log.Error("!!!GetListForDocx--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (b *objectBuilderService) GetSingleSlim(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetSingleSlim", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetSingleSlim--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.ObjectBuilder().GetSingleSlim(ctx, req)
	if err != nil {
		b.log.Error("!!!GetSingleSlim--->", logger.Error(err))
		return resp, err
	}
	return resp, nil
}

func (b *objectBuilderService) GetAllForDocx(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetAllForDocx", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetAllForDocx--->", logger.Any("request", compactRequest(req)))

	response, err := b.strg.ObjectBuilder().GetAllForDocx(ctx, req)
	if err != nil {
		b.log.Error("!!!GetAllForDocx--->", logger.Error(err))
		return resp, err
	}

	respStruct, err := helper.ConvertMapToStruct(response)
	if err != nil {
		b.log.Error("!!!GetAllForDocx--->", logger.Error(err))
		return resp, err
	}

	return &nb.CommonMessage{
		Data: respStruct,
	}, nil
}

func (b *objectBuilderService) GetAllFieldsForDocx(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetAllFieldsForDocx", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetAllFieldsForDocx--->", logger.Any("request", compactRequest(req)))

	response, err := b.strg.ObjectBuilder().GetAllFieldsForDocx(ctx, req)
	if err != nil {
		b.log.Error(fmt.Sprintf("!!!GetAllForDocx---> %d", 1), logger.Error(err))
		return resp, err
	}

	return response, nil
}

func (b *objectBuilderService) GetListAggregation(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetListAggregation", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetListAggregation--->", logger.Any("request", compactRequest(req)))

	response, err := b.strg.ObjectBuilder().GetListAggregation(ctx, req)
	if err != nil {
		b.log.Error("!!!GetListAggregation--->", logger.Error(err))
		return resp, err
	}

	return response, nil
}

func (b *objectBuilderService) AgGridTree(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	b.log.Info("!!!AgGridTree--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.AgGridTree", req)
	defer dbSpan.Finish()

	response, err := b.strg.ObjectBuilder().AgGridTree(ctx, req)
	if err != nil {
		b.log.Error("!!!AgGridTree--->", logger.Error(err))
		return resp, err
	}

	return response, nil
}

func (b *objectBuilderService) GetBoardStructure(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	b.log.Info("!!!GetBoardStructure--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetBoardStructure", req)
	defer dbSpan.Finish()

	response, err := b.strg.ObjectBuilder().GetBoardStructure(ctx, req)
	if err != nil {
		b.log.Error("!!!GetBoardStructure--->", logger.Error(err))
		return resp, err
	}

	return response, nil
}

func (b *objectBuilderService) GetBoardData(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	b.log.Info("!!!GetBoardData--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetBoardData", req)
	defer dbSpan.Finish()

	response, err := b.strg.ObjectBuilder().GetBoardData(ctx, req)
	if err != nil {
		b.log.Error("!!!GetBoardData--->", logger.Error(err))
		return resp, err
	}

	return response, nil
}

func (b *objectBuilderService) UserActivity(ctx context.Context, req *nb.UserActivityReqeust) (*empty.Empty, error) {
	b.log.Info("!!!UserActivity--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.UserActivity", req)
	defer dbSpan.Finish()

	err := b.strg.ObjectBuilder().UserActivity(ctx, req)
	if err != nil {
		b.log.Error("!!!UserActivity--->", logger.Error(err))
		return nil, err
	}

	return nil, nil
}

func (b *objectBuilderService) ExecuteSQL(ctx context.Context, req *nb.ExecuteSQLRequest) (*nb.ExecuteSQLResponse, error) {
	b.log.Info("!!!ExecuteSQL--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.ExecuteSQL", req)
	defer dbSpan.Finish()

	response, err := b.strg.ObjectBuilder().ExecuteSQL(ctx, req)
	if err != nil {
		b.log.Error("!!!ExecuteSQL--->", logger.Error(err))
		return nil, err
	}

	return response, nil
}

func (b *objectBuilderService) GetResourceUsage(ctx context.Context, req *nb.GetResourceUsageRequest) (*nb.GetResourceUsageResponse, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetResourceUsage", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetResourceUsage--->", logger.Any("request", compactRequest(req)))

	response, err := b.strg.ObjectBuilder().GetResourceUsage(ctx, req)
	if err != nil {
		b.log.Error("!!!GetResourceUsage--->", logger.Error(err))
		return nil, err
	}

	return response, nil
}

func (b *objectBuilderService) GetTableSchema(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_object_builder.GetTableSchema", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetTableSchema--->", logger.Any("request", compactRequest(req)))

	response, err := b.strg.ObjectBuilder().GetTableSchema(ctx, req)
	if err != nil {
		b.log.Error("!!!GetTableSchema--->", logger.Error(err))
		return nil, err
	}

	return response, nil
}

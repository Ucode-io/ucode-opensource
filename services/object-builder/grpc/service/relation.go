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

type relationService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedRelationServiceServer
}

func NewRelationService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *relationService {
	return &relationService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (r *relationService) Create(ctx context.Context, req *nb.CreateRelationRequest) (resp *nb.CreateRelationRequest, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.Create", req)
	defer dbSpan.Finish()

	r.log.Info("---CreateRelation--->", logger.Any("request", compactRequest(req)))

	resp, err = r.strg.Relation().Create(ctx, req)
	if err != nil {
		r.log.Error("---CreateRelation--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (r *relationService) GetByID(ctx context.Context, req *nb.RelationPrimaryKey) (resp *nb.RelationForGetAll, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.GetByID", req)
	defer dbSpan.Finish()

	r.log.Info("---GetSingleRelation--->", logger.Any("request", compactRequest(req)))

	resp, err = r.strg.Relation().GetByID(ctx, req)
	if err != nil {
		r.log.Error("---GetSingleRelation--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (r *relationService) GetAll(ctx context.Context, req *nb.GetAllRelationsRequest) (resp *nb.GetAllRelationsResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.GetAll", req)
	defer dbSpan.Finish()

	r.log.Info("---ListRelations--->", logger.Any("request", compactRequest(req)))

	resp, err = r.strg.Relation().GetList(ctx, req)
	if err != nil {
		r.log.Error("---ListRelations--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (r *relationService) Update(ctx context.Context, req *nb.UpdateRelationRequest) (resp *nb.RelationForGetAll, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.Update", req)
	defer dbSpan.Finish()

	r.log.Info("---UpdateRelation--->", logger.Any("request", compactRequest(req)))

	resp, err = r.strg.Relation().Update(ctx, req)
	if err != nil {
		r.log.Error("---UpdateRelation--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (r *relationService) Delete(ctx context.Context, req *nb.RelationPrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.Delete", req)
	defer dbSpan.Finish()

	r.log.Info("---DeleteRelation--->", logger.Any("request", compactRequest(req)))

	err = r.strg.Relation().Delete(ctx, req)
	if err != nil {
		r.log.Error("---DeleteRelation--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (r *relationService) GetIds(ctx context.Context, req *nb.GetIdsReq) (resp *nb.GetIdsResp, err error) {
	r.log.Info("---GetIds--->", logger.Any("request", compactRequest(req)))

	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.GetIds", req)
	defer dbSpan.Finish()

	resp, err = r.strg.Relation().GetIds(ctx, req)
	if err != nil {
		r.log.Error("---GetIds--->", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

// GetRelationsByTableFrom retrieves relations by table_from using pure SQL
func (r *relationService) GetRelationsByTableFrom(ctx context.Context, req *nb.GetRelationsByTableFromRequest) (resp *nb.GetRelationsByTableFromResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_relation.GetRelationsByTableFrom", req)
	defer dbSpan.Finish()

	r.log.Info("---GetRelationsByTableFrom--->", logger.Any("request", compactRequest(req)))

	relations, err := r.strg.Relation().GetRelationsByTableFrom(ctx, req.GetProjectId(), req.GetTableFrom())
	if err != nil {
		r.log.Error("---GetRelationsByTableFrom--->", logger.Error(err))
		return &nb.GetRelationsByTableFromResponse{}, err
	}

	return &nb.GetRelationsByTableFromResponse{
		Relations: relations,
	}, nil
}

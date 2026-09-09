package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	pbc "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

type tableService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedTableServiceServer
}

func NewTableService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *tableService { // ,
	return &tableService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (t *tableService) Create(ctx context.Context, req *nb.CreateTableRequest) (resp *nb.CreateTableResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.Create", req)
	defer dbSpan.Finish()

	t.log.Info("---CreateTable--->>>", logger.Any("request", compactRequest(req)))

	project, err := t.services.ProjectServiceClient().GetById(ctx, &pbc.GetProjectByIdRequest{ProjectId: req.GetUcodeProjectId()})
	if err != nil {
		t.log.Error("---CreateTable--->GetProjectById", logger.Error(err))
		return nil, status.Error(codes.Internal, "error getting project info")
	}

	if len(project.GetFareId()) != 0 {
		count, err := t.strg.Table().GetProjectTablesCount(ctx, req.GetProjectId())
		if err != nil {
			t.log.Error("---CreateTable--->GetProjectTablesCount", logger.Error(err))
			return nil, status.Error(codes.Internal, "error getting tables count")
		}

		limitResp, err := t.services.BillingServiceClient().CompareFunction(ctx, &pbc.CompareFunctionRequest{
			Type:   config.FARE_TABLES,
			FareId: project.GetFareId(),
			Count:  count + 1,
		})
		if err != nil {
			t.log.Error("---CreateTable--->CompareFunction", logger.Error(err))
			return nil, status.Error(codes.Internal, "error checking table limit")
		}

		if !limitResp.HasAccess {
			return nil, status.Error(codes.ResourceExhausted, "you have reached the table limit on your current plan. Please upgrade to create more tables.")
		}
	}

	resp, err = t.strg.Table().Create(ctx, req)
	if err != nil {
		t.log.Error("---CreateTable--->>>", logger.Error(err))
		return &nb.CreateTableResponse{}, err
	}

	return resp, nil
}

func (t *tableService) GetByID(ctx context.Context, req *nb.TablePrimaryKey) (resp *nb.Table, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.GetByID", req)
	defer dbSpan.Finish()

	t.log.Info("---GetByIDTable--->>>", logger.Any("request", compactRequest(req)))

	resp, err = t.strg.Table().GetByID(ctx, req)
	if err != nil {
		t.log.Error("---GetByIDTable--->>>", logger.Error(err))
		return &nb.Table{}, err
	}

	return resp, nil
}

func (t *tableService) GetAll(ctx context.Context, req *nb.GetAllTablesRequest) (resp *nb.GetAllTablesResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.GetAll", req)
	defer dbSpan.Finish()

	t.log.Info("---GetAllTable--->>>", logger.Any("request", compactRequest(req)))

	resp, err = t.strg.Table().GetAll(ctx, req)
	if err != nil {
		t.log.Error("---GetAllTable--->>>", logger.Error(err))
		return &nb.GetAllTablesResponse{}, err
	}

	return resp, nil
}

func (t *tableService) Update(ctx context.Context, req *nb.UpdateTableRequest) (resp *nb.Table, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.Update", req)
	defer dbSpan.Finish()

	t.log.Info("---UpdateTable--->>>", logger.Any("request", compactRequest(req)))

	resp, err = t.strg.Table().Update(ctx, req)
	if err != nil {
		t.log.Error("---UpdateTable--->>>", logger.Error(err))
		return &nb.Table{}, err
	}

	return resp, nil
}

func (t *tableService) Delete(ctx context.Context, req *nb.TablePrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.Delete", req)
	defer dbSpan.Finish()

	t.log.Info("---DeleteTable--->>>", logger.Any("request", compactRequest(req)))

	err = t.strg.Table().Delete(ctx, req)
	if err != nil {
		t.log.Error("---DeleteTable--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

func (t *tableService) GetTablesByLabel(ctx context.Context, req *nb.GetTablesByLabelReq) (resp *nb.GetAllTablesResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.GetTablesByLabel", req)
	defer dbSpan.Finish()

	t.log.Info("---UpdateLabel--->>>", logger.Any("request", compactRequest(req)))

	resp, err = t.strg.Table().GetTablesByLabel(ctx, req)
	if err != nil {
		t.log.Error("---UpdateLabel--->>>", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (t *tableService) GetChart(ctx context.Context, req *nb.ChartPrimaryKey) (resp *nb.GetChartResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.GetChart", req)
	defer dbSpan.Finish()

	t.log.Info("---GetChart--->>>", logger.Any("request", compactRequest(req)))

	resp, err = t.strg.Table().GetChart(ctx, req)
	if err != nil {
		t.log.Error("---GetChart--->>>", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (t *tableService) CreateConnectionAndSchema(ctx context.Context, req *nb.CreateConnectionAndSchemaReq) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.CreateConnectionAndSchema", req)
	defer dbSpan.Finish()

	t.log.Info("---CreateConnectionAndSchema--->>>", logger.Any("request", compactRequest(req)))

	err = t.strg.Table().CreateConnectionAndSchema(ctx, req)
	if err != nil {
		t.log.Error("---CreateConnectionAndSchema--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (t *tableService) GetTrackedUntrackedTables(ctx context.Context, req *nb.GetTrackedUntrackedTablesReq) (*nb.GetTrackedUntrackedTableResp, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.GetTrackedUntrackedTables", req)
	defer dbSpan.Finish()

	t.log.Info("---GetTrackedUntrackedTables--->>>", logger.Any("request", compactRequest(req)))

	resp, err := t.strg.Table().GetTrackedUntrackedTables(ctx, req)
	if err != nil {
		t.log.Error("---GetTrackedUntrackedTables--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (t *tableService) GetTrackedConnections(ctx context.Context, req *nb.GetTrackedConnectionsReq) (*nb.GetTrackedConnectionsResp, error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.GetTrackedConnections", req)
	defer dbSpan.Finish()

	t.log.Info("---GetTrackedConnections--->>>", logger.Any("request", compactRequest(req)))

	resp, err := t.strg.Table().GetTrackedConnections(ctx, req)
	if err != nil {
		t.log.Error("---GetTrackedConnections--->>>", logger.Error(err))
		return nil, err
	}

	return resp, nil
}

func (t *tableService) TrackTables(ctx context.Context, req *nb.TrackedTablesByIdsReq) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.TrackTables", req)
	defer dbSpan.Finish()

	t.log.Info("---TrackTables--->>>", logger.Any("request", compactRequest(req)))

	err = t.strg.Table().TrackTables(ctx, req)
	if err != nil {
		t.log.Error("---TrackTables--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return resp, nil
}

func (t *tableService) UntrackTableById(ctx context.Context, req *nb.UntrackTableByIdReq) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_table.UntrackTableById", req)
	defer dbSpan.Finish()

	t.log.Info("---UntrackTableById--->>>", logger.Any("request", compactRequest(req)))

	err = t.strg.Table().UntrackTableById(ctx, req)
	if err != nil {
		t.log.Error("---UntrackTableById--->>>", logger.Error(err))
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

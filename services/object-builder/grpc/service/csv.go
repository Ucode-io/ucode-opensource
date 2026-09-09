package service

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	span "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/jaeger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type csvService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedCSVServiceServer
}

func NewCSVService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *csvService {
	return &csvService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (b *csvService) GetListInCSV(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_csv.GetListInCSV", req)
	defer dbSpan.Finish()

	b.log.Info("!!!GetListInCSV--->", logger.Any("request", compactRequest(req)))

	resp, err = b.strg.CSV().GetListInCSV(ctx, req)
	if err != nil {
		b.log.Error("!!!GetListInCSV--->GetList", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

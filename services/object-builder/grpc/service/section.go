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

type sectionService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedSectionServiceServer
}

func NewSectionService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *sectionService { // ,
	return &sectionService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (s *sectionService) GetViewRelation(ctx context.Context, req *nb.GetAllSectionsRequest) (resp *nb.GetViewRelationResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_section.GetViewRelation", req)
	defer dbSpan.Finish()

	s.log.Info("---GetViewRelation--->>>", logger.Any("request", compactRequest(req)))

	resp, err = s.strg.Section().GetViewRelation(ctx, req)
	if err != nil {
		s.log.Error("---GetViewRelation--->>>", logger.Error(err))
		return &nb.GetViewRelationResponse{}, err
	}

	return resp, nil
}

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

type docxTemplateService struct {
	cfg      config.Config
	log      logger.LoggerI
	strg     storage.StorageI
	services client.ServiceManagerI
	nb.UnimplementedDocxTemplateServiceServer
}

func NewDocxTemplateService(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) *docxTemplateService {
	return &docxTemplateService{
		cfg:      cfg,
		log:      log,
		strg:     strg,
		services: svcs,
	}
}

func (f *docxTemplateService) Create(ctx context.Context, req *nb.CreateDocxTemplateRequest) (resp *nb.DocxTemplate, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_docx_template.Create", req)
	defer dbSpan.Finish()

	f.log.Info("---CreateDocxTemplate--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.DocxTemplate().Create(ctx, req)
	if err != nil {
		f.log.Error("---CreateDocxTemplate--->>>", logger.Error(err))
		return &nb.DocxTemplate{}, err
	}

	return resp, nil
}

func (f *docxTemplateService) GetByID(ctx context.Context, req *nb.DocxTemplatePrimaryKey) (resp *nb.DocxTemplate, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_docx_template.GetByID", req)
	defer dbSpan.Finish()

	f.log.Info("---GetByIDDocxTemplate--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.DocxTemplate().GetById(ctx, req)
	if err != nil {
		f.log.Error("---GetByIDDocxTemplate--->>>", logger.Error(err))
		return &nb.DocxTemplate{}, err
	}

	return resp, nil
}

func (f *docxTemplateService) GetAll(ctx context.Context, req *nb.GetAllDocxTemplateRequest) (resp *nb.GetAllDocxTemplateResponse, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_docx_template.GetAll", req)
	defer dbSpan.Finish()

	f.log.Info("---GetAllDocxTemplate--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.DocxTemplate().GetAll(ctx, req)
	if err != nil {
		f.log.Error("---GetAllDocxTemplatesResponse--->>>", logger.Error(err))
		return &nb.GetAllDocxTemplateResponse{}, err
	}

	return resp, nil
}

func (f *docxTemplateService) Update(ctx context.Context, req *nb.DocxTemplate) (resp *nb.DocxTemplate, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_docx_template.Update", req)
	defer dbSpan.Finish()

	f.log.Info("---UpdateDocxTemplate--->>>", logger.Any("request", compactRequest(req)))

	resp, err = f.strg.DocxTemplate().Update(ctx, req)
	if err != nil {
		f.log.Error("---UpdateDocxTemplate--->>>", logger.Error(err))
		return resp, err
	}

	return resp, nil
}

func (f *docxTemplateService) Delete(ctx context.Context, req *nb.DocxTemplatePrimaryKey) (resp *emptypb.Empty, err error) {
	dbSpan, ctx := span.StartSpanFromContext(ctx, "grpc_docx_template.Delete", req)
	defer dbSpan.Finish()

	f.log.Info("---DeleteDocxTemplate--->>>", logger.Any("request", compactRequest(req)))

	if err = f.strg.DocxTemplate().Delete(ctx, req); err != nil {
		f.log.Error("---DeleteDocxTemplate--->>>", logger.Error(err))
		return &emptypb.Empty{}, err
	}

	return &emptypb.Empty{}, nil
}

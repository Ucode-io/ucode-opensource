package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/doc_generator_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type DocGeneratorServiceI interface {
	DocumentGenerator() doc_generator_service.DocumentGeneratorServiceClient
}

type docGeneratorServiceClient struct {
	documentGeneratorService doc_generator_service.DocumentGeneratorServiceClient
}

func NewDocGeneratorServiceClient(ctx context.Context, cfg config.Config) (DocGeneratorServiceI, error) {

	connDocGeneratorService, err := grpc.DialContext(
		ctx,
		cfg.DocGeneratorGrpcHost+cfg.DocGeneratorGrpcPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &docGeneratorServiceClient{
		documentGeneratorService: doc_generator_service.NewDocumentGeneratorServiceClient(connDocGeneratorService),
	}, nil
}

func (d *docGeneratorServiceClient) DocumentGenerator() doc_generator_service.DocumentGeneratorServiceClient {
	return d.documentGeneratorService
}

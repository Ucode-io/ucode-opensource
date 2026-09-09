package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/transcoder_service"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type TranscoderServiceI interface {
	Pipeline() transcoder_service.PipelineServiceClient
}

type transcoderServiceClient struct {
	pipelineService transcoder_service.PipelineServiceClient
}

func NewTranscoderServiceClient(ctx context.Context, cfg config.Config) (TranscoderServiceI, error) {
	connTranscoderService, err := grpc.DialContext(
		ctx,
		cfg.TranscoderServiceHost+cfg.TranscoderServicePort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, err
	}

	return &transcoderServiceClient{
		pipelineService: transcoder_service.NewPipelineServiceClient(connTranscoderService),
	}, nil
}

func (g *transcoderServiceClient) Pipeline() transcoder_service.PipelineServiceClient {
	return g.pipelineService
}

package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/sms_service"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SmsServiceI interface {
	SmsService() sms_service.SmsServiceClient
}

type smsServiceClient struct {
	smsService sms_service.SmsServiceClient
}

func NewSmsServiceClient(ctx context.Context, cfg config.Config) (SmsServiceI, error) {

	connSmsService, err := grpc.DialContext(
		ctx,
		cfg.SmsServiceHost+cfg.SmsGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &smsServiceClient{
		smsService: sms_service.NewSmsServiceClient(connSmsService),
	}, nil
}

func (g *smsServiceClient) SmsService() sms_service.SmsServiceClient {
	return g.smsService
}

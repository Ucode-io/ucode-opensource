package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

type ServiceManagerI interface {
	BuilderService() BuilderServiceI
	HighBuilderService() BuilderServiceI
	AuthService() AuthServiceI
	CompanyService() CompanyServiceI
	SmsService() SmsServiceI
	DocGeneratorService() DocGeneratorServiceI
	GetBuilderServiceByType(nodeType string) BuilderServiceI
	GoObjectBuilderService() GoBuilderServiceI
	TranscoderService() TranscoderServiceI
}

type grpcClients struct {
	builderService         BuilderServiceI
	highBuilderService     BuilderServiceI
	authService            AuthServiceI
	companyService         CompanyServiceI
	smsService             SmsServiceI
	docGeneratorService    DocGeneratorServiceI
	goObjectBuilderService GoBuilderServiceI
	transcoderService      TranscoderServiceI
}

func NewGrpcClients(ctx context.Context, cfg config.Config) (ServiceManagerI, error) {
	builderServiceClient, err := NewBuilderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	highBuilderServiceClient, err := NewHighBuilderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	authServiceClient, err := NewAuthServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	companyServiceClient, err := NewCompanyServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	smsServiceClient, err := NewSmsServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	docGeneratorServiceClient, err := NewDocGeneratorServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	goObjectBuilderServiceClient, err := NewGoBuilderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	transcoderServiceClient, err := NewTranscoderServiceClient(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return grpcClients{
		authService:            authServiceClient,
		builderService:         builderServiceClient,
		highBuilderService:     highBuilderServiceClient,
		smsService:             smsServiceClient,
		companyService:         companyServiceClient,
		docGeneratorService:    docGeneratorServiceClient,
		goObjectBuilderService: goObjectBuilderServiceClient,
		transcoderService:      transcoderServiceClient,
	}, nil
}

func (g grpcClients) GetBuilderServiceByType(nodeType string) BuilderServiceI {
	switch nodeType {
	case config.LOW_NODE_TYPE:
		return g.builderService
	case config.HIGH_NODE_TYPE:
		return g.highBuilderService
	}

	return g.builderService
}

func (g grpcClients) BuilderService() BuilderServiceI {
	return g.builderService
}

func (g grpcClients) GoObjectBuilderService() GoBuilderServiceI {
	return g.goObjectBuilderService
}

func (g grpcClients) HighBuilderService() BuilderServiceI {
	return g.highBuilderService
}

func (g grpcClients) AuthService() AuthServiceI {
	return g.authService
}

func (g grpcClients) CompanyService() CompanyServiceI {
	return g.companyService
}

func (g grpcClients) SmsService() SmsServiceI {
	return g.smsService
}

func (g grpcClients) DocGeneratorService() DocGeneratorServiceI {
	return g.docGeneratorService
}

func (g grpcClients) TranscoderService() TranscoderServiceI {
	return g.transcoderService
}

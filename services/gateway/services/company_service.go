package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type CompanyServiceI interface {
	Company() company_service.CompanyServiceClient
	Project() company_service.ProjectServiceClient
	Environment() company_service.EnvironmentServiceClient
	Resource() company_service.ResourceServiceClient
	ServiceResource() company_service.MicroserviceResourceClient
	Redirect() company_service.RedirectUrlServiceClient
	CompanyPing() company_service.CompanyPingServiceClient
	IntegrationResource() company_service.IntegrationResourceServiceClient
	AirByte() company_service.AirbyteServiceClient
	Billing() company_service.BillingServiceClient
	Visualization() company_service.VisualizationServiceClient
	Template() company_service.TemplateMetadataServiceClient
	UgenTemplate() company_service.UgenTemplateServiceClient
	MfeShortLink() company_service.MfeShortLinkServiceClient
}

type companyServiceClient struct {
	companyService             company_service.CompanyServiceClient
	projectService             company_service.ProjectServiceClient
	environmentService         company_service.EnvironmentServiceClient
	resourceService            company_service.ResourceServiceClient
	serviceResource            company_service.MicroserviceResourceClient
	redirectService            company_service.RedirectUrlServiceClient
	companyPingService         company_service.CompanyPingServiceClient
	integrationResourceService company_service.IntegrationResourceServiceClient
	airbyteService             company_service.AirbyteServiceClient
	billingService             company_service.BillingServiceClient
	visualizationService       company_service.VisualizationServiceClient
	templateService            company_service.TemplateMetadataServiceClient
	ugenTemplateService        company_service.UgenTemplateServiceClient
	mfeShortLinkService        company_service.MfeShortLinkServiceClient
}

func NewCompanyServiceClient(ctx context.Context, cfg config.Config) (CompanyServiceI, error) {

	connCompanyService, err := grpc.DialContext(
		ctx,
		cfg.CompanyServiceHost+cfg.CompanyServicePort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, err
	}

	return &companyServiceClient{
		companyService:             company_service.NewCompanyServiceClient(connCompanyService),
		projectService:             company_service.NewProjectServiceClient(connCompanyService),
		environmentService:         company_service.NewEnvironmentServiceClient(connCompanyService),
		resourceService:            company_service.NewResourceServiceClient(connCompanyService),
		serviceResource:            company_service.NewMicroserviceResourceClient(connCompanyService),
		redirectService:            company_service.NewRedirectUrlServiceClient(connCompanyService),
		companyPingService:         company_service.NewCompanyPingServiceClient(connCompanyService),
		integrationResourceService: company_service.NewIntegrationResourceServiceClient(connCompanyService),
		airbyteService:             company_service.NewAirbyteServiceClient(connCompanyService),
		billingService:             company_service.NewBillingServiceClient(connCompanyService),
		visualizationService:       company_service.NewVisualizationServiceClient(connCompanyService),
		templateService:            company_service.NewTemplateMetadataServiceClient(connCompanyService),
		ugenTemplateService:        company_service.NewUgenTemplateServiceClient(connCompanyService),
		mfeShortLinkService:        company_service.NewMfeShortLinkServiceClient(connCompanyService),
	}, nil
}

func (g *companyServiceClient) Company() company_service.CompanyServiceClient {
	return g.companyService
}

func (g *companyServiceClient) Project() company_service.ProjectServiceClient {
	return g.projectService
}

func (g *companyServiceClient) Environment() company_service.EnvironmentServiceClient {
	return g.environmentService
}

func (g *companyServiceClient) Resource() company_service.ResourceServiceClient {
	return g.resourceService
}

func (g *companyServiceClient) ServiceResource() company_service.MicroserviceResourceClient {
	return g.serviceResource
}

func (g *companyServiceClient) Redirect() company_service.RedirectUrlServiceClient {
	return g.redirectService
}

func (g *companyServiceClient) CompanyPing() company_service.CompanyPingServiceClient {
	return g.companyPingService
}

func (g *companyServiceClient) IntegrationResource() company_service.IntegrationResourceServiceClient {
	return g.integrationResourceService
}

func (g *companyServiceClient) AirByte() company_service.AirbyteServiceClient {
	return g.airbyteService
}

func (g *companyServiceClient) Billing() company_service.BillingServiceClient {
	return g.billingService
}

func (g *companyServiceClient) Visualization() company_service.VisualizationServiceClient {
	return g.visualizationService
}

func (g *companyServiceClient) Template() company_service.TemplateMetadataServiceClient {
	return g.templateService
}

func (g *companyServiceClient) UgenTemplate() company_service.UgenTemplateServiceClient {
	return g.ugenTemplateService
}

func (g *companyServiceClient) MfeShortLink() company_service.MfeShortLinkServiceClient {
	return g.mfeShortLinkService
}

package client

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	company_service "github.com/Ucode-io/ucode-opensource/services/auth/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/sms_service"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ServiceManagerI interface {
	CompanyServiceClient() company_service.CompanyServiceClient
	ResourceService() company_service.ResourceServiceClient
	EnvironmentService() company_service.EnvironmentServiceClient
	ServiceResource() company_service.MicroserviceResourceClient
	ProjectServiceClient() company_service.ProjectServiceClient
	MicroServiceResourceService() company_service.MicroserviceResourceClient
	BillingServiceClient() company_service.BillingServiceClient

	IntegrationService() auth_service.IntegrationServiceClient
	ClientService() auth_service.ClientServiceClient
	PermissionService() auth_service.PermissionServiceClient
	UserService() auth_service.UserServiceClient
	SessionService() auth_service.SessionServiceClient
	CompanyService() auth_service.CompanyServiceClient
	ProjectService() auth_service.ProjectServiceClient
	ApiKeysService() auth_service.ApiKeysClient
	EmailService() auth_service.EmailOtpServiceClient
	AppleIdService() auth_service.AppleIdLoginServiceClient
	LoginStrategyService() auth_service.LoginStrategyServiceClient
	RegisterService() auth_service.RegisterServiceClient
	LoginPlatformType() auth_service.LoginPlatformTypeLoginServiceClient
	SmsOtpSettingsService() auth_service.SmsOtpSettingsServiceClient
	SyncUserService() auth_service.SyncUserServiceClient

	SmsService() sms_service.SmsServiceClient
}

type grpcClients struct {
	companyServiceClient        company_service.CompanyServiceClient
	projectServiceClient        company_service.ProjectServiceClient
	resourceService             company_service.ResourceServiceClient
	environmentService          company_service.EnvironmentServiceClient
	microServiceResourceService company_service.MicroserviceResourceClient
	serviceResource             company_service.MicroserviceResourceClient
	billingServiceClient        company_service.BillingServiceClient

	integrationService    auth_service.IntegrationServiceClient
	clientService         auth_service.ClientServiceClient
	permissionService     auth_service.PermissionServiceClient
	userService           auth_service.UserServiceClient
	sessionService        auth_service.SessionServiceClient
	emailService          auth_service.EmailOtpServiceClient
	companyService        auth_service.CompanyServiceClient
	projectService        auth_service.ProjectServiceClient
	apiKeysClients        auth_service.ApiKeysClient
	appleIdService        auth_service.AppleIdLoginServiceClient
	loginStrategyService  auth_service.LoginStrategyServiceClient
	registerService       auth_service.RegisterServiceClient
	loginPlatformType     auth_service.LoginPlatformTypeLoginServiceClient
	smsOtpSettingsService auth_service.SmsOtpSettingsServiceClient
	syncUserService       auth_service.SyncUserServiceClient

	smsService sms_service.SmsServiceClient
}

func NewGrpcClients(ctx context.Context, cfg config.BaseConfig) (ServiceManagerI, error) {
	connAuthService, err := grpc.DialContext(
		ctx,
		cfg.AuthServiceHost+cfg.AuthGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, err
	}

	connCompanyService, err := grpc.DialContext(
		ctx,
		cfg.CompanyServiceHost+cfg.CompanyGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, err
	}

	connSmsService, err := grpc.Dial(
		cfg.SmsServiceHost+cfg.SmsGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	return &grpcClients{
		companyServiceClient:        company_service.NewCompanyServiceClient(connCompanyService),
		projectServiceClient:        company_service.NewProjectServiceClient(connCompanyService),
		resourceService:             company_service.NewResourceServiceClient(connCompanyService),
		environmentService:          company_service.NewEnvironmentServiceClient(connCompanyService),
		microServiceResourceService: company_service.NewMicroserviceResourceClient(connCompanyService),
		serviceResource:             company_service.NewMicroserviceResourceClient(connCompanyService),
		billingServiceClient:        company_service.NewBillingServiceClient(connCompanyService),

		clientService:         auth_service.NewClientServiceClient(connAuthService),
		permissionService:     auth_service.NewPermissionServiceClient(connAuthService),
		userService:           auth_service.NewUserServiceClient(connAuthService),
		sessionService:        auth_service.NewSessionServiceClient(connAuthService),
		integrationService:    auth_service.NewIntegrationServiceClient(connAuthService),
		emailService:          auth_service.NewEmailOtpServiceClient(connAuthService),
		companyService:        auth_service.NewCompanyServiceClient(connAuthService),
		projectService:        auth_service.NewProjectServiceClient(connAuthService),
		apiKeysClients:        auth_service.NewApiKeysClient(connAuthService),
		appleIdService:        auth_service.NewAppleIdLoginServiceClient(connAuthService),
		loginStrategyService:  auth_service.NewLoginStrategyServiceClient(connAuthService),
		registerService:       auth_service.NewRegisterServiceClient(connAuthService),
		loginPlatformType:     auth_service.NewLoginPlatformTypeLoginServiceClient(connAuthService),
		smsOtpSettingsService: auth_service.NewSmsOtpSettingsServiceClient(connAuthService),
		syncUserService:       auth_service.NewSyncUserServiceClient(connAuthService),

		smsService: sms_service.NewSmsServiceClient(connSmsService),
	}, nil
}

func (g *grpcClients) MicroServiceResourceService() company_service.MicroserviceResourceClient {
	return g.microServiceResourceService
}

func (g *grpcClients) BillingServiceClient() company_service.BillingServiceClient {
	return g.billingServiceClient
}

func (g *grpcClients) ServiceResource() company_service.MicroserviceResourceClient {
	return g.serviceResource
}

func (g *grpcClients) CompanyServiceClient() company_service.CompanyServiceClient {
	return g.companyServiceClient
}

func (g *grpcClients) ProjectServiceClient() company_service.ProjectServiceClient {
	return g.projectServiceClient
}

func (g *grpcClients) ResourceService() company_service.ResourceServiceClient {
	return g.resourceService
}

func (g *grpcClients) EnvironmentService() company_service.EnvironmentServiceClient {
	return g.environmentService
}

func (g *grpcClients) ClientService() auth_service.ClientServiceClient {
	return g.clientService
}

func (g *grpcClients) PermissionService() auth_service.PermissionServiceClient {
	return g.permissionService
}

func (g *grpcClients) UserService() auth_service.UserServiceClient {
	return g.userService
}

func (g *grpcClients) SessionService() auth_service.SessionServiceClient {
	return g.sessionService
}

func (g *grpcClients) IntegrationService() auth_service.IntegrationServiceClient {
	return g.integrationService
}

func (g *grpcClients) CompanyService() auth_service.CompanyServiceClient {
	return g.companyService
}

func (g *grpcClients) ProjectService() auth_service.ProjectServiceClient {
	return g.projectService
}

func (g *grpcClients) ApiKeysService() auth_service.ApiKeysClient {
	return g.apiKeysClients
}

func (g *grpcClients) AppleIdService() auth_service.AppleIdLoginServiceClient {
	return g.appleIdService
}

func (g *grpcClients) LoginStrategyService() auth_service.LoginStrategyServiceClient {
	return g.loginStrategyService
}

func (g *grpcClients) RegisterService() auth_service.RegisterServiceClient {
	return g.registerService
}
func (g *grpcClients) LoginPlatformType() auth_service.LoginPlatformTypeLoginServiceClient {
	return g.loginPlatformType
}

func (g *grpcClients) SmsOtpSettingsService() auth_service.SmsOtpSettingsServiceClient {
	return g.smsOtpSettingsService
}

func (g *grpcClients) EmailService() auth_service.EmailOtpServiceClient {
	return g.emailService
}

func (g *grpcClients) SmsService() sms_service.SmsServiceClient {
	return g.smsService
}

func (g *grpcClients) SyncUserService() auth_service.SyncUserServiceClient {
	return g.syncUserService
}

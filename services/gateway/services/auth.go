package services

import (
	"context"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	grpcpool "github.com/processout/grpc-go-pool"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type AuthServiceManagerI interface {
	Client() auth_service.ClientServiceClient
	Session(ctx context.Context) (auth_service.SessionServiceClient, *grpcpool.ClientConn, error)
	Integration() auth_service.IntegrationServiceClient
	Permission() auth_service.PermissionServiceClient
	User() auth_service.UserServiceClient
	Email() auth_service.EmailOtpServiceClient
	Company() auth_service.CompanyServiceClient
	ApiKey() auth_service.ApiKeysClient
	AuthPing() auth_service.AuthPingServiceClient
	ApiKeyUsage() auth_service.ApiKeyUsageServiceClient
}

type authGrpcClients struct {
	clientService         auth_service.ClientServiceClient
	sessionService        *grpcpool.Pool
	integrationService    auth_service.IntegrationServiceClient
	clientServiceAuth     auth_service.ClientServiceClient
	permissionServiceAuth auth_service.PermissionServiceClient
	userService           auth_service.UserServiceClient
	sessionServiceAuth    auth_service.SessionServiceClient
	emailServie           auth_service.EmailOtpServiceClient
	authCompanyService    auth_service.CompanyServiceClient
	apikeyService         auth_service.ApiKeysClient
	authPingService       auth_service.AuthPingServiceClient
	apiKeyUsageService    auth_service.ApiKeyUsageServiceClient
}

func NewAuthGrpcClient(ctx context.Context, cfg config.BaseConfig) (AuthServiceManagerI, error) {

	connAuthService, err := grpc.DialContext(
		ctx,
		cfg.AuthServiceHost+cfg.AuthGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, err
	}

	factory := func() (*grpc.ClientConn, error) {
		conn, err := grpc.DialContext(
			ctx,
			cfg.AuthServiceHost+cfg.AuthGRPCPort,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
			grpc.WithUnaryInterceptor(
				otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
			grpc.WithStreamInterceptor(
				otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())))
		if err != nil {
			return nil, err
		}
		return conn, err
	}

	sessionServicePool, err := grpcpool.New(factory, 12, 18, time.Second*3)
	if err != nil {
		return nil, err
	}

	return &authGrpcClients{
		clientService:         auth_service.NewClientServiceClient(connAuthService),
		sessionService:        sessionServicePool,
		clientServiceAuth:     auth_service.NewClientServiceClient(connAuthService),
		permissionServiceAuth: auth_service.NewPermissionServiceClient(connAuthService),
		userService:           auth_service.NewUserServiceClient(connAuthService),
		sessionServiceAuth:    auth_service.NewSessionServiceClient(connAuthService),
		integrationService:    auth_service.NewIntegrationServiceClient(connAuthService),
		emailServie:           auth_service.NewEmailOtpServiceClient(connAuthService),
		authCompanyService:    auth_service.NewCompanyServiceClient(connAuthService),
		apikeyService:         auth_service.NewApiKeysClient(connAuthService),
		authPingService:       auth_service.NewAuthPingServiceClient(connAuthService),
		apiKeyUsageService:    auth_service.NewApiKeyUsageServiceClient(connAuthService),
	}, nil
}

func (g *authGrpcClients) Client() auth_service.ClientServiceClient {
	return g.clientService
}

func (g *authGrpcClients) Session(ctx context.Context) (auth_service.SessionServiceClient, *grpcpool.ClientConn, error) {
	conn, err := g.sessionService.Get(ctx)
	if err != nil {
		return nil, nil, err
	}
	service := auth_service.NewSessionServiceClient(conn)

	return service, conn, nil
}

func (g *authGrpcClients) Permission() auth_service.PermissionServiceClient {
	return g.permissionServiceAuth
}

func (g *authGrpcClients) User() auth_service.UserServiceClient {
	return g.userService
}

func (g *authGrpcClients) Integration() auth_service.IntegrationServiceClient {
	return g.integrationService
}

func (g *authGrpcClients) Email() auth_service.EmailOtpServiceClient {
	return g.emailServie
}

func (g *authGrpcClients) Company() auth_service.CompanyServiceClient {
	return g.authCompanyService
}

func (g *authGrpcClients) ApiKey() auth_service.ApiKeysClient {
	return g.apikeyService
}

func (g *authGrpcClients) AuthPing() auth_service.AuthPingServiceClient {
	return g.authPingService
}

func (g *authGrpcClients) ApiKeyUsage() auth_service.ApiKeyUsageServiceClient {
	return g.apiKeyUsageService
}

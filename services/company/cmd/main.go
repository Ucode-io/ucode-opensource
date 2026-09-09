package main

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/company/grpc/service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	minioclient "github.com/Ucode-io/ucode-opensource/services/company/pkg/minio"
	vaultclient "github.com/Ucode-io/ucode-opensource/services/company/pkg/vault"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/postgres"

	_ "github.com/lib/pq"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaeger_config "github.com/uber/jaeger-client-go/config"
	"google.golang.org/grpc"
)

func main() {
	baseLoad := config.BaseLoad()

	log := logger.New(baseLoad.LogLevel, baseLoad.ServiceName)
	defer func() {
		_ = logger.Cleanup(log)
	}()

	log.Info("main: pgxConfig",
		logger.String("host", baseLoad.PostgresHost),
		logger.Int("port", baseLoad.PostgresPort),
		logger.String("database", baseLoad.PostgresDatabase),
	)

	jaegerCfg := &jaeger_config.Configuration{
		ServiceName: baseLoad.ServiceName,
		Sampler: &jaeger_config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaeger_config.ReporterConfig{
			LogSpans:           false,
			LocalAgentHostPort: baseLoad.JaegerHostPort,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	tracer, closer, err := jaegerCfg.NewTracer(jaeger_config.Logger(jaeger.StdLogger))
	if err != nil {
		log.Error("ERROR: cannot init Jaeger", logger.Error(err))
	}
	defer closer.Close()
	opentracing.SetGlobalTracer(tracer)

	pgStore, err := postgres.NewPostgres(context.Background(), baseLoad, log)
	if err != nil {
		log.Panic("postgres.NewPostgres", logger.Error(err))
	}
	defer pgStore.CloseDB()

	requestedProvider := strings.ToLower(baseLoad.SecretsProvider)

	vaultClient, err := vaultclient.NewClient(ctx, vaultclient.NewClientArgs{
		Provider:      requestedProvider,
		Address:       baseLoad.Vault.Address,
		RoleID:        baseLoad.Vault.RoleID,
		SecretID:      baseLoad.Vault.SecretID,
		MountPath:     baseLoad.Vault.MountPath,
		SecretPath:    baseLoad.Vault.SecretPath,
		RedisAddress:  fmt.Sprintf("%s:%s", baseLoad.Redis.Host, baseLoad.Redis.Port),
		RedisPassword: baseLoad.Redis.Password,
		RedisDB:       baseLoad.Redis.DB,
	})
	if err != nil {
		log.Error("Error while connecting to secrets", logger.Error(err))
		return
	}

	if vaultClient.Provider() == vaultclient.ProviderVault {
		go func(ctx context.Context) {
			for {
				err := vaultClient.RenewToken(ctx)
				if err != nil {
					log.Error("Error while renewing vault token. Re-attempting", logger.Error(err))
				}
			}

		}(ctx)
	}

	serviceNodes := service.NewServiceNodes()
	s := grpc.NewServer(
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
		grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer())),
	)

	companyService := service.NewCompanyService(pgStore, log, serviceNodes, baseLoad)
	pb.RegisterCompanyServiceServer(s, companyService)

	uConf := config.Load()

	grpcClients, err := client.NewGrpcClients(ctx, uConf)
	if err != nil {
		log.Error("Error adding grpc client with base config. NewGrpcClients", logger.Error(err))
		return
	}

	err = serviceNodes.Add(grpcClients, baseLoad.UcodeNamespace)
	if err != nil {
		log.Error("Error adding company grpc client to serviceNode. ServiceNode", logger.Error(err))
		return
	}
	log.Info(" --- U-code company services --- added to serviceNodes")

	/// connecting enter prise projects
	projectServiceNodes, mapProjectConfs := service.EnterPriceProjectsGrpcSvcs(ctx, pgStore, serviceNodes, log)

	if projectServiceNodes == nil {
		projectServiceNodes = serviceNodes
	}

	if mapProjectConfs == nil {
		mapProjectConfs = make(map[string]config.Config)
	}

	mapProjectConfs[baseLoad.UcodeNamespace] = uConf
	projectServiceNodes.SetConfigs(mapProjectConfs)

	minioClient, err := minioclient.NewClient(baseLoad)
	if err != nil {
		log.Error("Error while connecting to minio. NewClient", logger.Error(err))
		return
	}
	if minioClient == nil {
		log.Warn("minio is not configured, ugen projects export will be unavailable")
	}

	projectService := service.NewProjectService(pgStore, log, projectServiceNodes, baseLoad, minioClient)
	resourceService := service.NewResourceService(pgStore, log, vaultClient, projectServiceNodes, uConf, grpcClients)
	environmentService := service.NewEnvironmentService(pgStore, log, projectServiceNodes, resourceService, grpcClients, baseLoad)
	microserviceResourceService := service.NewMicroserviceResourceService(pgStore, log, projectServiceNodes)
	redirectService := service.NewRedirectService(pgStore, log)
	companyPingService := service.NewCompanyPingService(log, serviceNodes)
	airbyteService := service.NewAirbyteService(pgStore, log, vaultClient)
	visualizationService := service.NewVisualizationService(log, uConf)
	templateService := service.NewTemplateService(pgStore, log, serviceNodes)
	integrationResourceService := service.NewIntegrationResourceService(pgStore, log)
	ugenTemplateService := service.NewUgenTemplateService(pgStore, log, serviceNodes)
	mfeShortLinkService := service.NewMfeShortLinkService(pgStore, log)

	pb.RegisterProjectServiceServer(s, projectService)
	pb.RegisterEnvironmentServiceServer(s, environmentService)
	pb.RegisterResourceServiceServer(s, resourceService)
	pb.RegisterMicroserviceResourceServer(s, microserviceResourceService)
	pb.RegisterRedirectUrlServiceServer(s, redirectService)
	pb.RegisterCompanyPingServiceServer(s, companyPingService)
	pb.RegisterAirbyteServiceServer(s, airbyteService)
	pb.RegisterVisualizationServiceServer(s, visualizationService)
	pb.RegisterTemplateMetadataServiceServer(s, templateService)
	pb.RegisterIntegrationResourceServiceServer(s, integrationResourceService)
	pb.RegisterUgenTemplateServiceServer(s, ugenTemplateService)
	pb.RegisterMfeShortLinkServiceServer(s, mfeShortLinkService)

	lis, err := net.Listen("tcp", baseLoad.RPCPort)
	if err != nil {
		log.Fatal("Error while listening: %v", logger.Error(err))
	}

	log.Info("main: server running!!",
		logger.String("port", baseLoad.RPCPort))

	if err := s.Serve(lis); err != nil {
		log.Fatal("Error while listening: %v", logger.Error(err))
	}

}

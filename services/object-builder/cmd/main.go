package main

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage/postgres"
	"net"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/cron"

	"github.com/gin-gonic/gin"
	_ "github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaeger_config "github.com/uber/jaeger-client-go/config"
)

func main() {
	var (
		loggerLevel string
		cfg         = config.Load()
	)

	switch cfg.Environment {
	case config.DebugMode:
		loggerLevel = logger.LevelDebug
		gin.SetMode(gin.DebugMode)
	case config.TestMode:
		loggerLevel = logger.LevelDebug
		gin.SetMode(gin.TestMode)
	default:
		loggerLevel = logger.LevelInfo
		gin.SetMode(gin.ReleaseMode)
	}

	jaegerCfg := &jaeger_config.Configuration{
		ServiceName: cfg.ServiceName,
		Sampler: &jaeger_config.SamplerConfig{
			Type:  "const",
			Param: 1,
		},
		Reporter: &jaeger_config.ReporterConfig{
			LogSpans:           false,
			LocalAgentHostPort: cfg.JaegerHostPort,
		},
	}

	log := logger.NewLogger(cfg.ServiceName, loggerLevel)
	defer func() {
		_ = logger.Cleanup(log)
	}()

	log.Info("Service env", logger.Any("cfg", cfg.SafeLogFields()))

	tracer, closer, err := jaegerCfg.NewTracer(jaeger_config.Logger(jaeger.StdLogger))
	if err != nil {
		log.Error("ERROR: cannot init Jaeger", logger.Error(err))
	}
	// closer is nil when the tracer failed to initialise; tracing is optional,
	// so carry on without it rather than panicking on the deferred Close.
	if closer != nil {
		defer closer.Close()
	}
	if tracer != nil {
		opentracing.SetGlobalTracer(tracer)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	svcs, err := client.NewGrpcClients(cfg)
	if err != nil {
		log.Panic("client.NewGrpcClients", logger.Error(err))
	}

	pgStore, err := postgres.NewPostgres(ctx, cfg, svcs, log)
	if err != nil {
		log.Panic("postgres.NewPostgres", logger.Error(err))
	}

	// ------------ cron -------------
	{
		cronJ := cron.New(log, pgStore, svcs)
		err = cronJ.RunJobs(ctx)
		if err != nil {
			log.Panic("cronJ.RunJobs", logger.Error(err))
		}
	}

	grpcServer := grpc.SetUpServer(cfg, log, svcs, pgStore)

	lis, err := net.Listen("tcp", cfg.ServicePort)
	if err != nil {
		log.Panic("net.Listen", logger.Error(err))
	}

	log.Info("GRPC: Server being started...", logger.String("port", cfg.ServicePort))

	if err := grpcServer.Serve(lis); err != nil {
		log.Panic("grpcServer.Serve", logger.Error(err))
	}
}

package grpc

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func SetUpServer(cfg config.Config, log logger.LoggerI, svcs client.ServiceManagerI, strg storage.StorageI) (grpcServer *grpc.Server) { // ,
	grpcServer = grpc.NewServer(
		grpc.UnaryInterceptor(otgrpc.OpenTracingServerInterceptor(opentracing.GlobalTracer())),
		grpc.StreamInterceptor(otgrpc.OpenTracingStreamServerInterceptor(opentracing.GlobalTracer())),
		grpc.MaxRecvMsgSize(config.GRPC_MAX_CALL_RECV_MSG_SIZE),
		grpc.MaxSendMsgSize(config.GRPC_MAX_CALL_SEND_MSG_SIZE),
	)

	project := service.NewBuilderProjectService(strg, cfg, log, svcs)
	err := project.AutoConnect(context.Background())
	if err != nil {
		logger.Any("project.AutoConnect", logger.Error(err))
	}

	nb.RegisterBuilderProjectServiceServer(grpcServer, project)
	nb.RegisterFieldServiceServer(grpcServer, service.NewFieldService(cfg, log, svcs, strg))
	nb.RegisterFunctionServiceV2Server(grpcServer, service.NewFunctionService(cfg, log, svcs, strg))
	nb.RegisterTableServiceServer(grpcServer, service.NewTableService(cfg, log, svcs, strg))
	nb.RegisterFileServiceServer(grpcServer, service.NewFileService(cfg, log, svcs, strg))
	nb.RegisterViewServiceServer(grpcServer, service.NewViewService(cfg, log, svcs, strg))
	nb.RegisterCustomErrorMessageServiceServer(grpcServer, service.NewCustomErrorMessageService(cfg, log, svcs, strg))
	nb.RegisterObjectBuilderServiceServer(grpcServer, service.NewObjectBuilderService(strg, cfg, log, svcs))
	nb.RegisterLoginServiceServer(grpcServer, service.NewLoginService(cfg, log, svcs, strg))
	nb.RegisterMenuServiceServer(grpcServer, service.NewMenuService(cfg, log, svcs, strg))
	nb.RegisterLayoutServiceServer(grpcServer, service.NewLayoutService(cfg, log, svcs, strg))
	nb.RegisterSectionServiceServer(grpcServer, service.NewSectionService(cfg, log, svcs, strg))
	nb.RegisterItemsServiceServer(grpcServer, service.NewItemsService(cfg, log, svcs, strg))
	nb.RegisterRelationServiceServer(grpcServer, service.NewRelationService(cfg, log, svcs, strg))
	nb.RegisterPermissionServiceServer(grpcServer, service.NewPermissionService(cfg, log, svcs, strg))
	nb.RegisterExcelServiceServer(grpcServer, service.NewExcelService(cfg, log, svcs, strg))
	nb.RegisterVersionServiceServer(grpcServer, service.NewVersionService(cfg, log, svcs, strg))
	nb.RegisterCustomEventServiceServer(grpcServer, service.NewCustomEventService(cfg, log, svcs, strg))
	nb.RegisterVersionHistoryServiceServer(grpcServer, service.NewVersionHistoryService(cfg, log, svcs, strg))
	nb.RegisterCSVServiceServer(grpcServer, service.NewCSVService(cfg, log, svcs, strg))
	nb.RegisterDocxTemplateServiceServer(grpcServer, service.NewDocxTemplateService(cfg, log, svcs, strg))
	nb.RegisterLanguageServiceServer(grpcServer, service.NewLanguageService(cfg, log, svcs, strg))
	nb.RegisterMcpProjectServiceServer(grpcServer, service.NewMcpProjectService(cfg, log, svcs, strg))
	nb.RegisterCustomPermissionsServiceServer(grpcServer, service.NewCustomPermissionsService(strg))
	nb.RegisterAiChatServiceServer(grpcServer, service.NewAiChatService(cfg, log, svcs, strg))
	nb.RegisterAgentServiceServer(grpcServer, service.NewAgentService(cfg, log, svcs, strg))
	nb.RegisterProjectFoldersServiceServer(grpcServer, service.NewProjectFoldersService(cfg, log, svcs, strg))
	nb.RegisterCustomEndpointServiceServer(grpcServer, service.NewCustomEndpointService(cfg, log, svcs, strg))
	nb.RegisterMicrofrontendVersionsServiceServer(grpcServer, service.NewMicrofrontendVersionsService(cfg, log, svcs, strg))
	nb.RegisterAiEditPromptServiceServer(grpcServer, service.NewAiEditPromptService(cfg, log, svcs, strg))

	reflection.Register(grpcServer)
	return
}

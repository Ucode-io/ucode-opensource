package client

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"

	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/sms_service"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type SharedServiceManagerI interface {
	ObjectBuilderService() object_builder_service.ObjectBuilderServiceClient
	LoginService() object_builder_service.LoginServiceClient
	GoLoginService() new_object_builder_service.LoginServiceClient
	BuilderPermissionService() object_builder_service.PermissionServiceClient
	VersionHistoryService() object_builder_service.VersionHistoryServiceClient
	TableService() object_builder_service.TableServiceClient

	GoObjectBuilderService() new_object_builder_service.ObjectBuilderServiceClient
	GoItemService() new_object_builder_service.ItemsServiceClient
	GoObjectBuilderPermissionService() new_object_builder_service.PermissionServiceClient
	GoObjectBuilderLoginService() new_object_builder_service.LoginServiceClient
	GoTableService() new_object_builder_service.TableServiceClient

	HighObjectBuilderService() object_builder_service.ObjectBuilderServiceClient
	HighLoginService() object_builder_service.LoginServiceClient
	HighBuilderPermissionService() object_builder_service.PermissionServiceClient
	HighTableService() object_builder_service.TableServiceClient

	GetObjectBuilderServiceByType(nodeType string) object_builder_service.ObjectBuilderServiceClient
	GetLoginServiceByType(nodeType string) object_builder_service.LoginServiceClient
	GetBuilderPermissionServiceByType(nodeType string) object_builder_service.PermissionServiceClient
	GetTableServiceByType(nodeType string) object_builder_service.TableServiceClient

	SmsService() sms_service.SmsServiceClient
}

type sharedGrpcClients struct {
	objectBuilderService     object_builder_service.ObjectBuilderServiceClient
	loginService             object_builder_service.LoginServiceClient
	goLoginService           new_object_builder_service.LoginServiceClient
	builderPermissionService object_builder_service.PermissionServiceClient
	versionHisotryService    object_builder_service.VersionHistoryServiceClient
	tableService             object_builder_service.TableServiceClient

	goObjectBuilderService           new_object_builder_service.ObjectBuilderServiceClient
	goItemsService                   new_object_builder_service.ItemsServiceClient
	goObjectBuilderPermissionService new_object_builder_service.PermissionServiceClient
	goItemService                    new_object_builder_service.ItemsServiceClient
	goObjectBuilderLoginService      new_object_builder_service.LoginServiceClient
	goTableService                   new_object_builder_service.TableServiceClient

	highObjectBuilderService     object_builder_service.ObjectBuilderServiceClient
	highLoginService             object_builder_service.LoginServiceClient
	highBuilderPermissionService object_builder_service.PermissionServiceClient
	highTableService             object_builder_service.TableServiceClient

	smsService sms_service.SmsServiceClient
}

func NewSharedGrpcClients(ctx context.Context, cfg config.Config) (SharedServiceManagerI, error) {
	connObjectBuilderService, _ := grpc.Dial(
		cfg.ObjectBuilderServiceHost+cfg.ObjectBuilderGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
	)

	connHighObjectBuilderService, _ := grpc.Dial(
		cfg.HighObjectBuilderServiceHost+cfg.HighObjectBuilderGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
	)

	connSmsService, _ := grpc.Dial(
		cfg.SmsServiceHost+cfg.SmsGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)

	connGoObjectBuilderService, _ := grpc.DialContext(
		ctx,
		cfg.GoObjectBuilderServiceHost+cfg.GoObjectBuilderServicePort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(52428800), grpc.MaxCallSendMsgSize(52428800)),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)

	return &sharedGrpcClients{
		objectBuilderService:     object_builder_service.NewObjectBuilderServiceClient(connObjectBuilderService),
		loginService:             object_builder_service.NewLoginServiceClient(connObjectBuilderService),
		builderPermissionService: object_builder_service.NewPermissionServiceClient(connObjectBuilderService),
		versionHisotryService:    object_builder_service.NewVersionHistoryServiceClient(connObjectBuilderService),
		tableService:             object_builder_service.NewTableServiceClient(connObjectBuilderService),

		highObjectBuilderService:     object_builder_service.NewObjectBuilderServiceClient(connHighObjectBuilderService),
		highLoginService:             object_builder_service.NewLoginServiceClient(connHighObjectBuilderService),
		highBuilderPermissionService: object_builder_service.NewPermissionServiceClient(connHighObjectBuilderService),
		highTableService:             object_builder_service.NewTableServiceClient(connHighObjectBuilderService),

		smsService: sms_service.NewSmsServiceClient(connSmsService),

		goLoginService:                   new_object_builder_service.NewLoginServiceClient(connGoObjectBuilderService),
		goObjectBuilderService:           new_object_builder_service.NewObjectBuilderServiceClient(connGoObjectBuilderService),
		goItemsService:                   new_object_builder_service.NewItemsServiceClient(connGoObjectBuilderService),
		goObjectBuilderPermissionService: new_object_builder_service.NewPermissionServiceClient(connGoObjectBuilderService),
		goItemService:                    new_object_builder_service.NewItemsServiceClient(connGoObjectBuilderService),
		goObjectBuilderLoginService:      new_object_builder_service.NewLoginServiceClient(connGoObjectBuilderService),
		goTableService:                   new_object_builder_service.NewTableServiceClient(connGoObjectBuilderService),
	}, nil
}

func (g *sharedGrpcClients) GetObjectBuilderServiceByType(nodeType string) object_builder_service.ObjectBuilderServiceClient {
	switch nodeType {
	case config.LOW_NODE_TYPE:
		return g.objectBuilderService
	case config.HIGH_NODE_TYPE:
		return g.highObjectBuilderService
	}

	return g.objectBuilderService
}

func (g *sharedGrpcClients) GetLoginServiceByType(nodeType string) object_builder_service.LoginServiceClient {
	switch nodeType {
	case config.LOW_NODE_TYPE:
		return g.loginService
	case config.HIGH_NODE_TYPE:
		return g.highLoginService
	}

	return g.loginService
}

func (g *sharedGrpcClients) GetBuilderPermissionServiceByType(nodeType string) object_builder_service.PermissionServiceClient {
	switch nodeType {
	case config.LOW_NODE_TYPE:
		return g.builderPermissionService
	case config.HIGH_NODE_TYPE:
		return g.highBuilderPermissionService
	}

	return g.builderPermissionService
}

func (g *sharedGrpcClients) GetTableServiceByType(nodeType string) object_builder_service.TableServiceClient {
	switch nodeType {
	case config.LOW_NODE_TYPE:
		return g.tableService
	case config.HIGH_NODE_TYPE:
		return g.highTableService
	}

	return g.tableService
}

func (g *sharedGrpcClients) BuilderPermissionService() object_builder_service.PermissionServiceClient {
	return g.builderPermissionService
}

func (g *sharedGrpcClients) ObjectBuilderService() object_builder_service.ObjectBuilderServiceClient {
	return g.objectBuilderService
}

func (g *sharedGrpcClients) SmsService() sms_service.SmsServiceClient {
	return g.smsService
}

func (g *sharedGrpcClients) HighObjectBuilderService() object_builder_service.ObjectBuilderServiceClient {
	return g.highObjectBuilderService
}

func (g *sharedGrpcClients) HighLoginService() object_builder_service.LoginServiceClient {
	return g.highLoginService
}

func (g *sharedGrpcClients) HighBuilderPermissionService() object_builder_service.PermissionServiceClient {
	return g.highBuilderPermissionService
}

func (g *sharedGrpcClients) LoginService() object_builder_service.LoginServiceClient {
	return g.loginService
}

func (g *sharedGrpcClients) VersionHistoryService() object_builder_service.VersionHistoryServiceClient {
	return g.versionHisotryService
}

func (g *sharedGrpcClients) GoObjectBuilderService() new_object_builder_service.ObjectBuilderServiceClient {
	return g.goObjectBuilderService
}

func (g *sharedGrpcClients) GoLoginService() new_object_builder_service.LoginServiceClient {
	return g.goLoginService
}

func (g *sharedGrpcClients) GoObjectBuilderPermissionService() new_object_builder_service.PermissionServiceClient {
	return g.goObjectBuilderPermissionService
}

func (g *sharedGrpcClients) GoItemService() new_object_builder_service.ItemsServiceClient {
	return g.goItemService
}

func (g *sharedGrpcClients) GoObjectBuilderLoginService() new_object_builder_service.LoginServiceClient {
	return g.goLoginService
}

func (g *sharedGrpcClients) GoTableService() new_object_builder_service.TableServiceClient {
	return g.goTableService
}

func (g *sharedGrpcClients) TableService() object_builder_service.TableServiceClient {
	return g.tableService
}

func (g *sharedGrpcClients) HighTableService() object_builder_service.TableServiceClient {
	return g.tableService
}

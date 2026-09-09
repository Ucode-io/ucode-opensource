package client

import (
	"context"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/new_function_service"
	nobs "github.com/Ucode-io/ucode-opensource/services/company/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/company/genproto/transcoder_service"

	"github.com/go-faster/errors"
	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type ServiceManagerI interface {
	BuilderService() object_builder_service.ObjectBuilderServiceClient
	BuilderProjectService() object_builder_service.BuilderProjectServiceClient

	HighBuilderService() object_builder_service.ObjectBuilderServiceClient
	HighBuilderProjectService() object_builder_service.BuilderProjectServiceClient

	GetBuilderProjectServiceByType(node_type string) object_builder_service.BuilderProjectServiceClient
	GetObjectBuilderServiceByType(nodeType string) object_builder_service.ObjectBuilderServiceClient

	FunctionProjectService() new_function_service.FunctionProjectServiceClient

	GoBuilderProjectService() nobs.BuilderProjectServiceClient
	GoObjectBuilderService() nobs.ObjectBuilderServiceClient
	GoTableService() nobs.TableServiceClient
	GoMenuService() nobs.MenuServiceClient

	UserService() auth_service.UserServiceClient
	ApiKeysService() auth_service.ApiKeysClient

	TranscoderService() transcoder_service.PipelineServiceClient
}

type grpcClients struct {
	builderService            object_builder_service.ObjectBuilderServiceClient
	builderProjectService     object_builder_service.BuilderProjectServiceClient
	functionProjectService    new_function_service.FunctionProjectServiceClient
	highBuilderService        object_builder_service.ObjectBuilderServiceClient
	highBuilderProjectService object_builder_service.BuilderProjectServiceClient
	goBuilderProjectService   nobs.BuilderProjectServiceClient
	goObjectBuilderService    nobs.ObjectBuilderServiceClient
	goTableService            nobs.TableServiceClient
	goMenuService             nobs.MenuServiceClient
	userService               auth_service.UserServiceClient
	apiKeysService            auth_service.ApiKeysClient
	transcoderService         transcoder_service.PipelineServiceClient
}

func NewGrpcClients(ctx context.Context, cfg config.Config) (ServiceManagerI, error) {
	connBuilderService, err := grpc.Dial(
		fmt.Sprintf("%s%s", cfg.BuilderServiceHost, cfg.BuilderServicePort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	connHighBuilderService, err := grpc.Dial(
		fmt.Sprintf("%s%s", cfg.HighBuilderServiceHost, cfg.HighBuilderServicePort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	connFunctionProjectService, err := grpc.Dial(
		fmt.Sprintf("%s%s", cfg.FunctionServiceHost, cfg.FunctionServicePort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		// return nil, err
	}

	connTranscoderService, err := grpc.Dial(
		fmt.Sprintf("%s%s", cfg.TranscoderServiceHost, cfg.TranscoderServicePort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	connAuthService, err := grpc.DialContext(ctx,
		fmt.Sprintf("%s%s", cfg.AuthServiceHost, cfg.AuthGRPCPort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, errors.Wrap(err, "connAuthService")
	}

	connGoObjectBuilderService, err := grpc.DialContext(ctx,
		fmt.Sprintf("%s%s", cfg.GoObjectBuilderServiceHost, cfg.GoObjectBuilderServicePort),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
	)
	if err != nil {
		return nil, errors.Wrap(err, "connGoObjectBuilderService")
	}

	return &grpcClients{
		builderService:            object_builder_service.NewObjectBuilderServiceClient(connBuilderService),
		builderProjectService:     object_builder_service.NewBuilderProjectServiceClient(connBuilderService),
		functionProjectService:    new_function_service.NewFunctionProjectServiceClient(connFunctionProjectService),
		highBuilderService:        object_builder_service.NewObjectBuilderServiceClient(connHighBuilderService),
		highBuilderProjectService: object_builder_service.NewBuilderProjectServiceClient(connHighBuilderService),
		goObjectBuilderService:    nobs.NewObjectBuilderServiceClient(connGoObjectBuilderService),
		goBuilderProjectService:   nobs.NewBuilderProjectServiceClient(connGoObjectBuilderService),
		goTableService:            nobs.NewTableServiceClient(connGoObjectBuilderService),
		goMenuService:             nobs.NewMenuServiceClient(connGoObjectBuilderService),
		userService:               auth_service.NewUserServiceClient(connAuthService),
		apiKeysService:            auth_service.NewApiKeysClient(connAuthService),
		transcoderService:         transcoder_service.NewPipelineServiceClient(connTranscoderService),
	}, nil
}

func (g *grpcClients) GetObjectBuilderServiceByType(nodeType string) object_builder_service.ObjectBuilderServiceClient {
	switch nodeType {
	case config.LowNodeType:
		return g.builderService
	case config.HighNodeType:
		return g.highBuilderService
	}

	return g.builderService
}

func (g *grpcClients) GetBuilderProjectServiceByType(node_type string) object_builder_service.BuilderProjectServiceClient {
	switch node_type {
	case config.LowNodeType:
		return g.builderProjectService
	case config.HighNodeType:
		return g.highBuilderProjectService
	}

	return g.builderProjectService
}

func (g *grpcClients) HighBuilderService() object_builder_service.ObjectBuilderServiceClient {
	return g.builderService
}

func (g *grpcClients) BuilderService() object_builder_service.ObjectBuilderServiceClient {
	return g.builderService
}

func (g *grpcClients) BuilderProjectService() object_builder_service.BuilderProjectServiceClient {
	return g.builderProjectService
}

func (g *grpcClients) HighBuilderProjectService() object_builder_service.BuilderProjectServiceClient {
	return g.builderProjectService
}

func (g *grpcClients) FunctionProjectService() new_function_service.FunctionProjectServiceClient {
	return g.functionProjectService
}

func (g *grpcClients) UserService() auth_service.UserServiceClient {
	return g.userService
}

func (g *grpcClients) GoObjectBuilderService() nobs.ObjectBuilderServiceClient {
	return g.goObjectBuilderService
}

func (g *grpcClients) GoBuilderProjectService() nobs.BuilderProjectServiceClient {
	return g.goBuilderProjectService
}

func (g *grpcClients) GoTableService() nobs.TableServiceClient {
	return g.goTableService
}

func (g *grpcClients) GoMenuService() nobs.MenuServiceClient {
	return g.goMenuService
}

func (g *grpcClients) ApiKeysService() auth_service.ApiKeysClient {
	return g.apiKeysService
}

func (g *grpcClients) TranscoderService() transcoder_service.PipelineServiceClient {
	return g.transcoderService
}

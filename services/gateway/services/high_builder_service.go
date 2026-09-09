package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"

	gRPCClientLb "github.com/golanguzb70/grpc-client-lb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func NewHighBuilderServiceClient(ctx context.Context, cfg config.Config) (BuilderServiceI, error) {

	connObjectBuilderService, err := grpc.DialContext(
		ctx,
		cfg.HighObjectBuilderServiceHost+cfg.HighObjectBuilderGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024), grpc.MaxCallSendMsgSize(100*1024*1024)),
	)
	if err != nil {
		return nil, err
	}

	factory := func() (*grpc.ClientConn, error) {
		conn, err := grpc.Dial(
			cfg.HighObjectBuilderServiceHost+cfg.HighObjectBuilderGRPCPort,
			grpc.WithTransportCredentials(insecure.NewCredentials()),
			grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(100*1024*1024), grpc.MaxCallSendMsgSize(100*1024*1024)))
		if err != nil {
			return nil, err
		}
		return conn, err
	}

	grpcClientLB, err := gRPCClientLb.NewGrpcClientLB(factory, 12)
	if err != nil {
		return nil, err
	}

	return &builderServiceClient{
		tableService:              object_builder_service.NewTableServiceClient(connObjectBuilderService),
		tableFolderService:        object_builder_service.NewTableFolderServiceClient(connObjectBuilderService),
		fieldService:              object_builder_service.NewFieldServiceClient(connObjectBuilderService),
		objectBuilderService:      object_builder_service.NewObjectBuilderServiceClient(connObjectBuilderService),
		sectionService:            object_builder_service.NewSectionServiceClient(connObjectBuilderService),
		relationService:           object_builder_service.NewRelationServiceClient(connObjectBuilderService),
		viewService:               object_builder_service.NewViewServiceClient(connObjectBuilderService),
		dashboardService:          object_builder_service.NewDashboardServiceClient(connObjectBuilderService),
		variableService:           object_builder_service.NewVariableServiceClient(connObjectBuilderService),
		panelService:              object_builder_service.NewPanelServiceClient(connObjectBuilderService),
		appService:                object_builder_service.NewAppServiceClient(connObjectBuilderService),
		htmlTemplateService:       object_builder_service.NewHtmlTemplateServiceClient(connObjectBuilderService),
		documentService:           object_builder_service.NewDocumentServiceClient(connObjectBuilderService),
		eventService:              object_builder_service.NewEventServiceClient(connObjectBuilderService),
		eventLogsService:          object_builder_service.NewEventLogsServiceClient(connObjectBuilderService),
		excelService:              object_builder_service.NewExcelServiceClient(connObjectBuilderService),
		permissionService:         object_builder_service.NewPermissionServiceClient(connObjectBuilderService),
		customEventService:        object_builder_service.NewCustomEventServiceClient(connObjectBuilderService),
		functionService:           object_builder_service.NewFunctionServiceClient(connObjectBuilderService),
		barcodeService:            object_builder_service.NewBarcodeServiceClient(connObjectBuilderService),
		objectBuilderServiceAuth:  object_builder_service.NewObjectBuilderServiceClient(connObjectBuilderService),
		loginService:              object_builder_service.NewLoginServiceClient(connObjectBuilderService),
		queryFolderService:        object_builder_service.NewQueryFolderServiceClient(connObjectBuilderService),
		queriesService:            object_builder_service.NewQueryServiceClient(connObjectBuilderService),
		webPageService:            object_builder_service.NewWebPageServiceClient(connObjectBuilderService),
		cascadingService:          object_builder_service.NewCascadingServiceClient(connObjectBuilderService),
		tableHelpersService:       object_builder_service.NewTableHelpersServiceClient(connObjectBuilderService),
		fieldsAndRelations:        object_builder_service.NewFieldAndRelationServiceClient(connObjectBuilderService),
		settingService:            object_builder_service.NewSettingServiceClient(connObjectBuilderService),
		layoutService:             object_builder_service.NewLayoutServiceClient(connObjectBuilderService),
		menuService:               object_builder_service.NewMenuServiceClient(connObjectBuilderService),
		customErrorMessageService: object_builder_service.NewCustomErrorMessageServiceClient(connObjectBuilderService),
		reportSettingService:      object_builder_service.NewReportSettingServiceClient(connObjectBuilderService),
		fileService:               object_builder_service.NewFileServiceClient(connObjectBuilderService),
		itemsService:              object_builder_service.NewItemsServiceClient(connObjectBuilderService),
		versionHistoryService:     object_builder_service.NewVersionHistoryServiceClient(connObjectBuilderService),
		languageService:           object_builder_service.NewLanguageServiceClient(connObjectBuilderService),
		clientLb:                  grpcClientLB,
	}, nil
}

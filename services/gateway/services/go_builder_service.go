package services

import (
	"context"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"

	otgrpc "github.com/opentracing-contrib/go-grpc"
	"github.com/opentracing/opentracing-go"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GoBuilderServiceI interface {
	Table() nb.TableServiceClient
	Field() nb.FieldServiceClient
	ObjectBuilder() nb.ObjectBuilderServiceClient
	Menu() nb.MenuServiceClient
	View() nb.ViewServiceClient
	Section() nb.SectionServiceClient
	Layout() nb.LayoutServiceClient
	Items() nb.ItemsServiceClient
	Relation() nb.RelationServiceClient
	File() nb.FileServiceClient
	Excel() nb.ExcelServiceClient
	Function() nb.FunctionServiceV2Client
	CustomEvent() nb.CustomEventServiceClient
	Permission() nb.PermissionServiceClient
	Version() nb.VersionServiceClient
	VersionHistory() nb.VersionHistoryServiceClient
	FolderGroup() nb.FolderGroupServiceClient
	CSV() nb.CSVServiceClient
	DocxTemplate() nb.DocxTemplateServiceClient
	Language() nb.LanguageServiceClient
	McpProject() nb.McpProjectServiceClient
	CustomPermission() nb.CustomPermissionsServiceClient
	AiChat() nb.AiChatServiceClient
	Agent() nb.AgentServiceClient
	ProjectFolders() nb.ProjectFoldersServiceClient
	CustomEndpoint() nb.CustomEndpointServiceClient
	MicrofrontendVersions() nb.MicrofrontendVersionsServiceClient
	AiEditPrompt() nb.AiEditPromptServiceClient
}

type goBuilderServiceClient struct {
	tableService            nb.TableServiceClient
	menuService             nb.MenuServiceClient
	viewService             nb.ViewServiceClient
	objectBuilderService    nb.ObjectBuilderServiceClient
	fieldService            nb.FieldServiceClient
	sectionService          nb.SectionServiceClient
	layoutService           nb.LayoutServiceClient
	itemsService            nb.ItemsServiceClient
	relationService         nb.RelationServiceClient
	fileService             nb.FileServiceClient
	excelService            nb.ExcelServiceClient
	functionService         nb.FunctionServiceV2Client
	customEventService      nb.CustomEventServiceClient
	permissionService       nb.PermissionServiceClient
	versionService          nb.VersionServiceClient
	versionHistoryService   nb.VersionHistoryServiceClient
	folderGroupService      nb.FolderGroupServiceClient
	csvService              nb.CSVServiceClient
	docxTemplateService     nb.DocxTemplateServiceClient
	languageService         nb.LanguageServiceClient
	mcpProjectService       nb.McpProjectServiceClient
	customPermissionService nb.CustomPermissionsServiceClient
	aiChatService                  nb.AiChatServiceClient
	agentService                   nb.AgentServiceClient
	projectFoldersService          nb.ProjectFoldersServiceClient
	customEndpointService          nb.CustomEndpointServiceClient
	microfrontendVersionsService   nb.MicrofrontendVersionsServiceClient
	aiEditPromptService            nb.AiEditPromptServiceClient
}

func NewGoBuilderServiceClient(ctx context.Context, cfg config.Config) (GoBuilderServiceI, error) {

	connGoBuilderService, err := grpc.DialContext(
		ctx,
		cfg.GoObjectBuilderServiceHost+cfg.GoObjectBuilderGRPCPort,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithUnaryInterceptor(
			otgrpc.OpenTracingClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithStreamInterceptor(
			otgrpc.OpenTracingStreamClientInterceptor(opentracing.GlobalTracer())),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(config.GRPC_MAX_CALL_RECV_MSG_SIZE), grpc.MaxCallSendMsgSize(config.GRPC_MAX_CALL_SEND_MSG_SIZE)),
	)

	if err != nil {
		return nil, err
	}

	return &goBuilderServiceClient{
		tableService:            nb.NewTableServiceClient(connGoBuilderService),
		menuService:             nb.NewMenuServiceClient(connGoBuilderService),
		viewService:             nb.NewViewServiceClient(connGoBuilderService),
		objectBuilderService:    nb.NewObjectBuilderServiceClient(connGoBuilderService),
		fieldService:            nb.NewFieldServiceClient(connGoBuilderService),
		sectionService:          nb.NewSectionServiceClient(connGoBuilderService),
		layoutService:           nb.NewLayoutServiceClient(connGoBuilderService),
		itemsService:            nb.NewItemsServiceClient(connGoBuilderService),
		relationService:         nb.NewRelationServiceClient(connGoBuilderService),
		fileService:             nb.NewFileServiceClient(connGoBuilderService),
		excelService:            nb.NewExcelServiceClient(connGoBuilderService),
		functionService:         nb.NewFunctionServiceV2Client(connGoBuilderService),
		customEventService:      nb.NewCustomEventServiceClient(connGoBuilderService),
		permissionService:       nb.NewPermissionServiceClient(connGoBuilderService),
		versionService:          nb.NewVersionServiceClient(connGoBuilderService),
		versionHistoryService:   nb.NewVersionHistoryServiceClient(connGoBuilderService),
		folderGroupService:      nb.NewFolderGroupServiceClient(connGoBuilderService),
		csvService:              nb.NewCSVServiceClient(connGoBuilderService),
		docxTemplateService:     nb.NewDocxTemplateServiceClient(connGoBuilderService),
		languageService:         nb.NewLanguageServiceClient(connGoBuilderService),
		mcpProjectService:       nb.NewMcpProjectServiceClient(connGoBuilderService),
		customPermissionService: nb.NewCustomPermissionsServiceClient(connGoBuilderService),
		aiChatService:                nb.NewAiChatServiceClient(connGoBuilderService),
		agentService:                 nb.NewAgentServiceClient(connGoBuilderService),
		projectFoldersService:        nb.NewProjectFoldersServiceClient(connGoBuilderService),
		customEndpointService:        nb.NewCustomEndpointServiceClient(connGoBuilderService),
		microfrontendVersionsService: nb.NewMicrofrontendVersionsServiceClient(connGoBuilderService),
		aiEditPromptService:          nb.NewAiEditPromptServiceClient(connGoBuilderService),
	}, nil
}

func (g *goBuilderServiceClient) Table() nb.TableServiceClient {
	return g.tableService
}

func (g *goBuilderServiceClient) Field() nb.FieldServiceClient {
	return g.fieldService
}

func (g *goBuilderServiceClient) ObjectBuilder() nb.ObjectBuilderServiceClient {
	return g.objectBuilderService
}

func (g *goBuilderServiceClient) Menu() nb.MenuServiceClient {
	return g.menuService
}

func (g *goBuilderServiceClient) View() nb.ViewServiceClient {
	return g.viewService
}

func (g *goBuilderServiceClient) Section() nb.SectionServiceClient {
	return g.sectionService
}

func (g *goBuilderServiceClient) Layout() nb.LayoutServiceClient {
	return g.layoutService
}

func (g *goBuilderServiceClient) Items() nb.ItemsServiceClient {
	return g.itemsService
}

func (g *goBuilderServiceClient) Relation() nb.RelationServiceClient {
	return g.relationService
}

func (g *goBuilderServiceClient) File() nb.FileServiceClient {
	return g.fileService
}

func (g *goBuilderServiceClient) Excel() nb.ExcelServiceClient {
	return g.excelService
}

func (g *goBuilderServiceClient) Function() nb.FunctionServiceV2Client {
	return g.functionService
}

func (g *goBuilderServiceClient) CustomEvent() nb.CustomEventServiceClient {
	return g.customEventService
}

func (g *goBuilderServiceClient) Permission() nb.PermissionServiceClient {
	return g.permissionService
}

func (g *goBuilderServiceClient) Version() nb.VersionServiceClient {
	return g.versionService
}

func (g *goBuilderServiceClient) VersionHistory() nb.VersionHistoryServiceClient {
	return g.versionHistoryService
}

func (g *goBuilderServiceClient) FolderGroup() nb.FolderGroupServiceClient {
	return g.folderGroupService
}

func (g *goBuilderServiceClient) CSV() nb.CSVServiceClient {
	return g.csvService
}

func (g *goBuilderServiceClient) DocxTemplate() nb.DocxTemplateServiceClient {
	return g.docxTemplateService
}

func (g *goBuilderServiceClient) Language() nb.LanguageServiceClient {
	return g.languageService
}

func (g *goBuilderServiceClient) McpProject() nb.McpProjectServiceClient {
	return g.mcpProjectService
}

func (g *goBuilderServiceClient) CustomPermission() nb.CustomPermissionsServiceClient {
	return g.customPermissionService
}

func (g *goBuilderServiceClient) AiChat() nb.AiChatServiceClient {
	return g.aiChatService
}

func (g *goBuilderServiceClient) Agent() nb.AgentServiceClient {
	return g.agentService
}

func (g *goBuilderServiceClient) ProjectFolders() nb.ProjectFoldersServiceClient {
	return g.projectFoldersService
}

func (g *goBuilderServiceClient) CustomEndpoint() nb.CustomEndpointServiceClient {
	return g.customEndpointService
}

func (g *goBuilderServiceClient) MicrofrontendVersions() nb.MicrofrontendVersionsServiceClient {
	return g.microfrontendVersionsService
}

func (g *goBuilderServiceClient) AiEditPrompt() nb.AiEditPromptServiceClient {
	return g.aiEditPromptService
}

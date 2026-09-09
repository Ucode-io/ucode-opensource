package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	db                    *psqlpool.Pool
	logger                logger.LoggerI
	grpcClient            client.ServiceManagerI
	builderProject        storage.BuilderProjectRepoI
	field                 storage.FieldRepoI
	function              storage.FunctionRepoI
	file                  storage.FileRepoI
	table                 storage.TableRepoI
	object_builder        storage.ObjectBuilderRepoI
	view                  storage.ViewRepoI
	menu                  storage.MenuRepoI
	login                 storage.LoginRepoI
	layout                storage.LayoutRepoI
	relation              storage.RelationRepoI
	section               storage.SectionRepoI
	permission            storage.PermissionRepoI
	items                 storage.ItemsRepoI
	excel                 storage.ExcelRepoI
	version               storage.VersionRepoI
	customEvent           storage.CustomEventRepoI
	versionHistory        storage.VersionHistoryRepoI
	csv                   storage.CSVRepoI
	docxTemplate          storage.DocxTemplateRepoI
	language              storage.LanguageRepoI
	mcpProject            storage.McpProjectRepoI
	customPermissions     storage.CustomPermissionsRepoI
	aiChat                storage.AiChatRepoI
	agent                 storage.AgentRepoI
	projectFolders        storage.ProjectFoldersRepoI
	customEndpoint        storage.CustomEndpointRepoI
	microfrontendVersions storage.MicrofrontendVersionsRepoI
	aiEditPrompt          storage.AiEditPromptRepoI
}

func NewPostgres(ctx context.Context, cfg config.Config, grpcClient client.ServiceManagerI, logger logger.LoggerI) (storage.StorageI, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDatabase,
	))
	if err != nil {
		return nil, err
	}

	config.MaxConns = cfg.PostgresMaxConnections

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	dbPool := &psqlpool.Pool{
		Db:     pool,
		Logger: logger,
	}

	return &Store{
		db:         dbPool,
		grpcClient: grpcClient,
		logger:     logger,
	}, err
}

func (s *Store) CloseDB() {
	s.db.Db.Close()
}

func (s *Store) BuilderProject() storage.BuilderProjectRepoI {
	if s.builderProject == nil {
		s.builderProject = NewBuilderProjectRepo(s.db)
	}

	return s.builderProject
}

func (s *Store) Field() storage.FieldRepoI {
	if s.field == nil {
		s.field = NewFieldRepo(s.db, NewRelationRepo(s.db), s.logger)
	}

	return s.field
}

func (s *Store) Function() storage.FunctionRepoI {
	if s.function == nil {
		s.function = NewFunctionRepo(s.db)
	}

	return s.function
}

func (s *Store) File() storage.FileRepoI {
	if s.file == nil {
		s.file = NewFileRepo(s.db)
	}

	return s.file
}

func (s *Store) Table() storage.TableRepoI {
	if s.table == nil {
		s.table = NewTableRepo(s.db, NewMenuRepo(s.db), NewFieldRepo(s.db, NewRelationRepo(s.db), s.logger), NewRelationRepo(s.db), s.logger)
	}

	return s.table
}

func (s *Store) ObjectBuilder() storage.ObjectBuilderRepoI {
	if s.object_builder == nil {
		s.object_builder = NewObjectBuilder(s.logger, s.grpcClient)
	}
	return s.object_builder
}

func (s *Store) View() storage.ViewRepoI {
	if s.view == nil {
		s.view = NewViewRepo(s.db)
	}

	return s.view
}

func (s *Store) Menu() storage.MenuRepoI {
	if s.menu == nil {
		s.menu = NewMenuRepo(s.db)
	}
	return s.menu
}

func (s *Store) Login() storage.LoginRepoI {
	if s.login == nil {
		s.login = NewLoginRepo(s.db)
	}

	return s.login
}

func (s *Store) Layout() storage.LayoutRepoI {
	if s.layout == nil {
		s.layout = NewLayoutRepo()
	}

	return s.layout
}

func (s *Store) Relation() storage.RelationRepoI {
	if s.relation == nil {
		s.relation = NewRelationRepo(s.db)
	}

	return s.relation
}

func (s *Store) Section() storage.SectionRepoI {
	if s.section == nil {
		s.section = NewSectionRepo(s.db)
	}
	return s.section
}

func (s *Store) Permission() storage.PermissionRepoI {
	if s.permission == nil {
		s.permission = NewPermissionRepo(s.db)
	}
	return s.permission
}

func (s *Store) Items() storage.ItemsRepoI {
	if s.items == nil {
		s.items = NewItemsRepo(s.db, s.grpcClient, s.logger)
	}
	return s.items
}

func (s *Store) Excel() storage.ExcelRepoI {
	if s.excel == nil {
		s.excel = NewExcelRepo(s.db)
	}

	return s.excel
}

func (s *Store) Version() storage.VersionRepoI {
	if s.version == nil {
		s.version = NewVersionRepo(s.db)
	}

	return s.version
}

func (s *Store) CustomEvent() storage.CustomEventRepoI {
	if s.customEvent == nil {
		s.customEvent = NewCustomEventRepo(s.db)
	}

	return s.customEvent
}

func (s *Store) VersionHistory() storage.VersionHistoryRepoI {
	if s.versionHistory == nil {
		s.versionHistory = NewVersionHistoryRepo(s.db)
	}

	return s.versionHistory
}

func (s *Store) CSV() storage.CSVRepoI {
	if s.csv == nil {
		s.csv = NewCSVRepo(s.db)
	}
	return s.csv
}

func (s *Store) DocxTemplate() storage.DocxTemplateRepoI {
	if s.docxTemplate == nil {
		s.docxTemplate = NewDocxTemplateRepo(s.db)
	}

	return s.docxTemplate
}

func (s *Store) Language() storage.LanguageRepoI {
	if s.language == nil {
		s.language = NewLanguageRepo(s.db)
	}

	return s.language
}

func (s *Store) McpProject() storage.McpProjectRepoI {
	if s.mcpProject == nil {
		s.mcpProject = NewMcpProjectRepo(s.db)
	}

	return s.mcpProject
}

func (s *Store) CustomPermissions() storage.CustomPermissionsRepoI {
	if s.customPermissions == nil {
		s.customPermissions = NewCustomPermissionsRepo(s.db)
	}

	return s.customPermissions
}

func (s *Store) AiChat() storage.AiChatRepoI {
	if s.aiChat == nil {
		s.aiChat = NewAiChatRepo(s.db)
	}

	return s.aiChat
}

func (s *Store) Agent() storage.AgentRepoI {
	if s.agent == nil {
		s.agent = NewAgentRepo(s.db)
	}

	return s.agent
}

func (s *Store) ProjectFolders() storage.ProjectFoldersRepoI {
	if s.projectFolders == nil {
		s.projectFolders = NewProjectFoldersRepo(s.db)
	}
	return s.projectFolders
}

func (s *Store) CustomEndpoint() storage.CustomEndpointRepoI {
	if s.customEndpoint == nil {
		s.customEndpoint = NewCustomEndpointRepo(s.db)
	}
	return s.customEndpoint
}

func (s *Store) MicrofrontendVersions() storage.MicrofrontendVersionsRepoI {
	if s.microfrontendVersions == nil {
		s.microfrontendVersions = NewMicrofrontendVersionsRepo(s.db)
	}
	return s.microfrontendVersions
}

func (s *Store) AiEditPrompt() storage.AiEditPromptRepoI {
	if s.aiEditPrompt == nil {
		s.aiEditPrompt = NewAiEditPromptRepo(s.db)
	}
	return s.aiEditPrompt
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{Valid: false}
	}
	return sql.NullString{String: s, Valid: true}
}

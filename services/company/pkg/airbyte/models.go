package airbyte

import "github.com/Ucode-io/ucode-opensource/services/company/config"

type Workspace struct {
	AnonymousDataCollection bool     `json:"anonymousDataCollection"`
	CustomerID              string   `json:"customerId"`
	DefaultGeography        string   `json:"defaultGeography"`
	DisplaySetupWizard      bool     `json:"displaySetupWizard"`
	Email                   string   `json:"email"`
	InitialSetupComplete    bool     `json:"initialSetupComplete"`
	Name                    string   `json:"name"`
	News                    bool     `json:"news"`
	Notifications           []string `json:"notifications"`
	SecurityUpdates         bool     `json:"securityUpdates"`
	Slug                    string   `json:"slug"`
	WorkspaceID             string   `json:"workspaceId"`
}

type WorkspacesResponse struct {
	Workspaces []Workspace `json:"workspaces"`
}

type SourceDefinition struct {
	Name               string `json:"name"`
	SourceDefinitionId string `json:"sourceDefinitionId"`
}

type SourceDefinitionsResponse struct {
	SourceDefinitions []SourceDefinition `json:"sourceDefinitions"`
}

type CreatePostgresSource struct {
	Configuration       PostgresConfiguration `json:"connectionConfiguration"`
	Name                string                `json:"name"`
	SourceDefinitionId  string                `json:"sourceDefinitionId"`
	WorkSpaceId         string                `json:"workspaceId"`
	SyncMode            string                `json:"syncMode"`
	DestinationSyncMode string                `json:"destinationSyncMode"`
}

type PostgresConfiguration struct {
	Database          string         `json:"database"`
	Host              string         `json:"host"`
	Password          string         `json:"password"`
	Username          string         `json:"username"`
	Port              int64          `json:"port"`
	Schemas           []string       `json:"schemas"`
	ReplicationMethod map[string]any `json:"replication_method"`
	SslMode           map[string]any `json:"ssl_mode"`
	TunnelMethod      map[string]any `json:"tunnel_method"`
}

type CreatePostgresSourceResponse struct {
	Name     string `json:"name"`
	SourceId string `json:"sourceId"`
}

type CreateMongoSource struct {
	Configuration      MongoConfiguration `json:"connectionConfiguration"`
	Name               string             `json:"name"`
	WorkspaceId        string             `json:"workspaceId"`
	SourceDefinitionId string             `json:"sourceDefinitionId"`
}

type MongoConfiguration struct {
	AuthSource   string            `json:"auth_source"`
	InstanceType MongoInstanceType `json:"instance_type"`
	Database     string            `json:"database"`
	User         string            `json:"user"`
	Password     string            `json:"password"`
}

type MongoInstanceType struct {
	Host     string `json:"host"`
	Instance string `json:"instance"`
	Port     int64  `json:"port"`
	TLS      bool   `json:"tls"`
}

type CreateMongoSourceResponse struct {
	SourceId string `json:"sourceId"`
}

type Stream struct {
	Stream StreamDetails `json:"stream"`
	Config ConfigDetails `json:"config"`
}

type StreamDetails struct {
	Name                    string   `json:"name"`
	JSONSchema              any      `json:"jsonSchema"`
	SupportedSyncModes      []string `json:"supportedSyncModes"`
	DefaultCursorField      []any    `json:"defaultCursorField"`
	SourceDefinedPrimaryKey []any    `json:"sourceDefinedPrimaryKey"`
	Namespace               string   `json:"namespace"`
}

type ConfigDetails struct {
	SyncMode            string `json:"syncMode"`
	CursorField         []any  `json:"cursorField"`
	DestinationSyncMode string `json:"destinationSyncMode"`
	PrimaryKey          []any  `json:"primaryKey"`
	AliasName           string `json:"aliasName"`
	Selected            bool   `json:"selected"`
	Suggested           bool   `json:"suggested"`
}

type SyncCatalog struct {
	Streams []Stream `json:"streams"`
}

type SyncCatalogResponse struct {
	Catalog SyncCatalog `json:"catalog"`
}

type CreateClickhouseDestination struct {
	Configuration           ClickHouseConfiguration `json:"connectionConfiguration"`
	Name                    string                  `json:"name"`
	WorkspaceId             string                  `json:"workspaceId"`
	DestinationDefinitionId string                  `json:"destinationDefinitionId"`
}

type ClickHouseConfiguration struct {
	Database     string            `json:"database"`
	Host         string            `json:"host"`
	Password     string            `json:"password"`
	Port         int64             `json:"port"`
	SSL          bool              `json:"ssl"`
	UserName     string            `json:"username"`
	TunnelMethod map[string]string `json:"tunnel_method"`
}

type DestinationDefinition struct {
	Name                    string `json:"name"`
	DestinationDefinitionId string `json:"destinationDefinitionId"`
}

type DestinationDefinitionResponse struct {
	DestinationDefinitions []DestinationDefinition `json:"destinationDefinitions"`
}

type CreateConnection struct {
	SourceId                     string                `json:"sourceId"`
	DestinitionId                string                `json:"destinationId"`
	SyncCatalog                  SyncCatalog           `json:"syncCatalog"`
	Schedule                     map[string]any        `json:"schedule"`
	Name                         string                `json:"name"`
	Status                       string                `json:"status"`
	NamespaceDefinition          string                `json:"namespaceDefinition"`
	NamespaceFormat              string                `json:"namespaceFormat"`
	NonBreakingChangesPreference string                `json:"nonBreakingChangesPreference"`
	Operations                   []ConnectionOperation `json:"operations"`
	Geography                    string                `json:"geography"`
	SyncMode                     string                `json:"syncMode"`
	DestinationSyncMode          string                `json:"destinationSyncMode"`
}

type ConnectionOperation struct {
	Name                  string `json:"name"`
	WorkspaceId           string `json:"workspaceId"`
	OperatorConfiguration struct {
		OperatorType  string `json:"operatorType"`
		Normalization struct {
			Option string `json:"option"`
		} `json:"normalization"`
	} `json:"operatorConfiguration"`
}

type CreateWorkbookAndConnectionRequest struct {
	Title               string        `json:"title"`
	Description         string        `json:"description"`
	Host                string        `json:"host"`
	Port                string        `json:"port"`
	Username            string        `json:"username"`
	Password            string        `json:"password"`
	Secure              string        `json:"secure"`
	DataExportForbidden string        `json:"data_export_forbidden"`
	Readonly            string        `json:"readonly"`
	RawSQLLevel         string        `json:"raw_sql_level"`
	Type                string        `json:"type"`
	Cfg                 config.Config `json:"cfg"`
}

type WorkbookResponse struct {
	WorkbookID   string `json:"workbookId"`
	CollectionID any    `json:"collectionId"`
	Title        string `json:"title"`
	Description  string `json:"description"`
}

// SUPERSET
type CreateSupersetConnectionRequest struct {
	DatabaseName     string        `json:"database_name"`
	Host             string        `json:"host"`
	Port             string        `json:"port"`
	Database         string        `json:"database"`
	Username         string        `json:"username"`
	Password         string        `json:"password"`
	SupersetName     string        `json:"superset_name"`
	SupersetPassword string        `json:"superset_password"`
	Cfg              config.Config `json:"cfg"`
}

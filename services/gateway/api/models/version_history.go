package models

import "github.com/Ucode-io/ucode-opensource/services/gateway/services"

type CreateVersionHistoryRequest struct {
	Services  services.ServiceManagerI
	NodeType  string
	ProjectId string

	Id               string
	ActionSource     string          `json:"action_source"`
	ActionType       string          `json:"action_type"`
	Previous         any             `json:"previous"`
	Current          any             `json:"current"`
	UsedEnvironments map[string]bool `json:"used_environments"`
	Date             string          `json:"date"`
	UserInfo         string          `json:"user_info"`
	Request          any             `json:"request"`
	Response         any             `json:"response"`
	ApiKey           string          `json:"api_key"`
	Type             string          `json:"type"`
	TableSlug        string          `json:"table_slug"`
	VersionId        string          `json:"version_id"`
	MethodApi        string          `json:"method_api"`
	TimeStarted      string          `json:"time_started"`
	TimeCompleted    string          `json:"time_completed"`
	Duration         int64           `json:"duration"`
	StatusCode       int             `json:"status_code"`
	ResourceType     int
}

type MigrateUp struct {
	Id               string          `json:"id"`
	ActionSource     string          `json:"action_source"`
	ActionType       string          `json:"action_type"`
	Previous         any             `json:"previus"`
	Current          any             `json:"current"`
	UsedEnvironments map[string]bool `json:"used_envrironments"`
	Date             string          `json:"date"`
	UserInfo         string          `json:"user_info"`
	Request          any             `json:"request"`
	Response         any             `json:"response"`
	ApiKey           string          `json:"api_key"`
	Type             string          `json:"type"`
	TableSlug        string          `json:"table_slug"`
	VersionId        string          `json:"version_id"`
}

type MigrateUpRequest struct {
	Data []*MigrateUp `json:"data"`
}

type MigrateUpResponse struct {
	Ids []string `json:"ids"`
}

type PublishVersionRequest struct {
	PublishedEnvironmentID string `json:"to_environment_id"`
	PublishedVersionID     string `json:"published_version_id"`
}

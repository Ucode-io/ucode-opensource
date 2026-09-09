package models

import "github.com/Ucode-io/ucode-opensource/services/auth/grpc/client"

type CreateVersionHistoryRequest struct {
	Services  client.SharedServiceManagerI
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
}

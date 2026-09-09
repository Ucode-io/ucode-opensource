package metabase

import "github.com/Ucode-io/ucode-opensource/services/company/config"

type CreateConnectionRequest struct {
	DatabaseName     string        `json:"database_name"`
	Host             string        `json:"host"`
	Port             string        `json:"port"`
	Database         string        `json:"database"`
	Username         string        `json:"username"`
	Password         string        `json:"password"`
	MetabaseName     string        `json:"metabase_name"`
	MetabasePassword string        `json:"metabase_password"`
	Cfg              config.Config `json:"cfg"`
}

type GetSessionRequest struct {
	Username string        `json:"username"`
	Password string        `json:"password"`
	Cfg      config.Config `json:"cfg"`
}

type GetDashboardsRequest struct {
	Session string        `json:"session"`
	Cfg     config.Config `json:"cfg"`
}

type GetPublicUrlRequest struct {
	DashboardId int           `json:"id"`
	Cfg         config.Config `json:"cfg"`
}

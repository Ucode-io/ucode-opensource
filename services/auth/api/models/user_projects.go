package models

type GetUserProjects struct {
	Companies []Companie `json:"companies"`
}

type Companie struct {
	Id       string   `json:"id"`
	Projects []string `json:"projects"`
}

type GetUserProjectByAllFieldsReq struct {
	ClientTypeId string `json:"client_type_id"`
	RoleId       string `json:"role_id"`
	UserId       string `json:"user_id"`
	CompanyId    string `json:"company_id"`
	ProjectId    string `json:"project_id"`
	EnvId        string `json:"env_id"`
}

// key of map will be project_id and value of map will be envIds
type GetUserEnvProjectRes struct {
	EnvProjects map[string][]string `json:"env_projects"`
}

type AddUserToProjectReq struct {
	UserId         string         `json:"user_id"`
	ProjectId      string         `json:"project_id"`
	RoleId         string         `json:"role_id"`
	Active         int32          `json:"active"`
	CompanyId      string         `json:"company_id"`
	ResourceType   int32          `json:"resource_type"`
	ClientTypeId   string         `json:"client_type_id"`
	TableSlug      string         `json:"table_slug"`
	EnvId          string         `json:"env_id"`
	NodeType       string         `json:"node_type"`
	Status         string         `json:"status"`
	AdditionalData map[string]any `json:"additional_data"`
}

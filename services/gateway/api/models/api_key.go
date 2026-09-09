package models

// ValidateApiKeyResponse is the result of checking an API key against a
// project and environment.
type ValidateApiKeyResponse struct {
	Valid         bool   `json:"valid"`
	AppId         string `json:"app_id"`
	ProjectId     string `json:"project_id"`
	EnvironmentId string `json:"environment_id"`
}

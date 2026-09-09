package models

type TranscoderWebhook struct {
	OutputKey  string `json:"output_key"`
	OutputPath string `json:"output_path"`
	ProjectId  string `json:"project_id"`
	KeyId      string `json:"key_id"`
	FieldSlug  string `json:"field_slug"`
	TableSlug  string `json:"table_slug"`
}

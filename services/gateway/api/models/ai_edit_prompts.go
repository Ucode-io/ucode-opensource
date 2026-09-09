package models

const (
	AIEditPromptKindCodeEditor   = "code_editor"
	AIEditPromptKindVisualEditor = "visual_editor"

	AIEditPromptSourceDefault = "default"
	AIEditPromptSourceCustom  = "custom"
)

type AIEditPrompt struct {
	PromptKind      string `json:"prompt_kind"`
	Content         string `json:"content"`
	Revision        int64  `json:"revision"`
	UpdatedByUserID string `json:"updated_by_user_id,omitempty"`
	CreatedAt       string `json:"created_at,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
}

type UpsertAIEditPromptRequest struct {
	Content          string `json:"content" binding:"required"`
	ExpectedRevision int64  `json:"expected_revision"`
}

type AIEditPromptSetting struct {
	PromptKind      string  `json:"prompt_kind"`
	Content         string  `json:"content"`
	DefaultContent  string  `json:"default_content"`
	CustomContent   *string `json:"custom_content"`
	Source          string  `json:"source"`
	Revision        int64   `json:"revision"`
	UpdatedByUserID string  `json:"updated_by_user_id,omitempty"`
	CreatedAt       string  `json:"created_at,omitempty"`
	UpdatedAt       string  `json:"updated_at,omitempty"`
}

type AIEditPromptSettingsResponse struct {
	Prompts          []AIEditPromptSetting `json:"prompts"`
	StorageAvailable bool                  `json:"storage_available"`
}

package models

import "time"

const (
	AiEditPromptKindCodeEditor   = "code_editor"
	AiEditPromptKindVisualEditor = "visual_editor"
	MaxAiEditPromptBytes         = 128 * 1024
)

// IsAiEditPromptKind reports whether kind is one of the two prompt bodies that
// generated-project users are allowed to override.
func IsAiEditPromptKind(kind string) bool {
	return kind == AiEditPromptKindCodeEditor || kind == AiEditPromptKindVisualEditor
}

// AiEditPrompt is the persistence-layer representation. Keeping it independent
// of generated transport types lets the repository remain stable when the
// shared protobuf contract is regenerated.
type AiEditPrompt struct {
	PromptKind      string
	Content         string
	Revision        int64
	UpdatedByUserID string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type UpsertAiEditPromptRequest struct {
	ResourceEnvID    string
	PromptKind       string
	Content          string
	ExpectedRevision int64
	UpdatedByUserID  string
}

type DeleteAiEditPromptRequest struct {
	ResourceEnvID    string
	PromptKind       string
	ExpectedRevision int64
}

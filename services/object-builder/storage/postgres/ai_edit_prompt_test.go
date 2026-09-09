package postgres

import (
	"errors"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestValidateAiEditPromptUpsert(t *testing.T) {
	tests := []struct {
		name    string
		req     *models.UpsertAiEditPromptRequest
		wantErr error
	}{
		{
			name: "code editor",
			req: &models.UpsertAiEditPromptRequest{
				PromptKind: models.AiEditPromptKindCodeEditor,
				Content:    "Keep components small.",
			},
		},
		{
			name: "visual editor",
			req: &models.UpsertAiEditPromptRequest{
				PromptKind:       models.AiEditPromptKindVisualEditor,
				Content:          "Preserve spacing tokens.",
				ExpectedRevision: 2,
			},
		},
		{
			name: "unknown kind",
			req: &models.UpsertAiEditPromptRequest{
				PromptKind: "planner",
				Content:    "content",
			},
			wantErr: storage.ErrInvalidAiEditPromptKind,
		},
		{
			name: "blank content",
			req: &models.UpsertAiEditPromptRequest{
				PromptKind: models.AiEditPromptKindCodeEditor,
				Content:    " \n\t ",
			},
			wantErr: storage.ErrInvalidAiEditPromptContent,
		},
		{
			name: "negative revision",
			req: &models.UpsertAiEditPromptRequest{
				PromptKind:       models.AiEditPromptKindCodeEditor,
				Content:          "content",
				ExpectedRevision: -1,
			},
			wantErr: storage.ErrInvalidAiEditPromptRevision,
		},
		{
			name: "content too large",
			req: &models.UpsertAiEditPromptRequest{
				PromptKind: models.AiEditPromptKindCodeEditor,
				Content:    string(make([]byte, models.MaxAiEditPromptBytes+1)),
			},
			wantErr: storage.ErrAiEditPromptContentTooLarge,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAiEditPromptUpsert(tt.req)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("validateAiEditPromptUpsert() error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

func TestClassifyAiEditPromptDBError(t *testing.T) {
	missingTable := &pgconn.PgError{Code: "42P01", Message: `relation "ai_edit_prompts" does not exist`}
	if err := classifyAiEditPromptDBError(missingTable); !errors.Is(err, storage.ErrAiEditPromptTableMissing) {
		t.Fatalf("missing table error = %v, want ErrAiEditPromptTableMissing", err)
	}

	other := errors.New("connection closed")
	if err := classifyAiEditPromptDBError(other); !errors.Is(err, other) {
		t.Fatalf("other error = %v, want original error", err)
	}
}

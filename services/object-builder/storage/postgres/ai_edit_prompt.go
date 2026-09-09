package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/opentracing/opentracing-go"
)

type aiEditPromptRepo struct {
	db *psqlpool.Pool
}

func NewAiEditPromptRepo(db *psqlpool.Pool) storage.AiEditPromptRepoI {
	return &aiEditPromptRepo{db: db}
}

func (r *aiEditPromptRepo) Get(ctx context.Context, resourceEnvID, promptKind string) (*models.AiEditPrompt, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "aiEditPromptRepo.Get")
	defer span.Finish()

	if !models.IsAiEditPromptKind(promptKind) {
		return nil, fmt.Errorf("%w: %q", storage.ErrInvalidAiEditPromptKind, promptKind)
	}

	conn, err := psqlpool.Get(resourceEnvID)
	if err != nil {
		return nil, err
	}

	prompt := new(models.AiEditPrompt)
	err = conn.QueryRow(ctx, `
		SELECT prompt_kind, content, revision, updated_by_user_id, created_at, updated_at
		FROM ai_edit_prompts
		WHERE prompt_kind = $1
	`, promptKind).Scan(
		&prompt.PromptKind,
		&prompt.Content,
		&prompt.Revision,
		&prompt.UpdatedByUserID,
		&prompt.CreatedAt,
		&prompt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrAiEditPromptNotFound
		}
		return nil, classifyAiEditPromptDBError(err)
	}

	return prompt, nil
}

func (r *aiEditPromptRepo) GetAll(ctx context.Context, resourceEnvID string) ([]*models.AiEditPrompt, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "aiEditPromptRepo.GetAll")
	defer span.Finish()

	conn, err := psqlpool.Get(resourceEnvID)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(ctx, `
		SELECT prompt_kind, content, revision, updated_by_user_id, created_at, updated_at
		FROM ai_edit_prompts
		ORDER BY prompt_kind
	`)
	if err != nil {
		return nil, classifyAiEditPromptDBError(err)
	}
	defer rows.Close()

	prompts := make([]*models.AiEditPrompt, 0, 2)
	for rows.Next() {
		prompt := new(models.AiEditPrompt)
		if err = rows.Scan(
			&prompt.PromptKind,
			&prompt.Content,
			&prompt.Revision,
			&prompt.UpdatedByUserID,
			&prompt.CreatedAt,
			&prompt.UpdatedAt,
		); err != nil {
			return nil, classifyAiEditPromptDBError(err)
		}
		prompts = append(prompts, prompt)
	}
	if err = rows.Err(); err != nil {
		return nil, classifyAiEditPromptDBError(err)
	}

	return prompts, nil
}

func (r *aiEditPromptRepo) Upsert(ctx context.Context, req *models.UpsertAiEditPromptRequest) (*models.AiEditPrompt, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "aiEditPromptRepo.Upsert")
	defer span.Finish()

	if err := validateAiEditPromptUpsert(req); err != nil {
		return nil, err
	}

	conn, err := psqlpool.Get(req.ResourceEnvID)
	if err != nil {
		return nil, err
	}

	prompt := new(models.AiEditPrompt)
	var row pgx.Row

	if req.ExpectedRevision == 0 {
		// Use the underlying pool intentionally: the shared wrapper records query
		// arguments in tracing tags, and prompt content must not be logged.
		row = conn.Db.QueryRow(ctx, `
			INSERT INTO ai_edit_prompts (prompt_kind, content, updated_by_user_id)
			VALUES ($1, $2, $3)
			ON CONFLICT (prompt_kind) DO NOTHING
			RETURNING prompt_kind, content, revision, updated_by_user_id, created_at, updated_at
		`, req.PromptKind, req.Content, req.UpdatedByUserID)
	} else {
		// See the logging note above. A zero-row update is a stale revision,
		// including the case where another request reset the override first.
		row = conn.Db.QueryRow(ctx, `
			UPDATE ai_edit_prompts
			SET content = $2,
			    revision = nextval('ai_edit_prompt_revision_seq'),
			    updated_by_user_id = $3,
			    updated_at = NOW()
			WHERE prompt_kind = $1 AND revision = $4
			RETURNING prompt_kind, content, revision, updated_by_user_id, created_at, updated_at
		`, req.PromptKind, req.Content, req.UpdatedByUserID, req.ExpectedRevision)
	}

	err = row.Scan(
		&prompt.PromptKind,
		&prompt.Content,
		&prompt.Revision,
		&prompt.UpdatedByUserID,
		&prompt.CreatedAt,
		&prompt.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, storage.ErrAiEditPromptRevisionConflict
		}
		return nil, classifyAiEditPromptDBError(err)
	}

	return prompt, nil
}

func (r *aiEditPromptRepo) Delete(ctx context.Context, req *models.DeleteAiEditPromptRequest) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "aiEditPromptRepo.Delete")
	defer span.Finish()

	if req == nil {
		return storage.ErrInvalidAiEditPromptKind
	}
	if !models.IsAiEditPromptKind(req.PromptKind) {
		return fmt.Errorf("%w: %q", storage.ErrInvalidAiEditPromptKind, req.PromptKind)
	}
	if req.ExpectedRevision < 0 {
		return storage.ErrInvalidAiEditPromptRevision
	}

	conn, err := psqlpool.Get(req.ResourceEnvID)
	if err != nil {
		return err
	}

	if req.ExpectedRevision == 0 {
		var exists bool
		err = conn.QueryRow(ctx, `
			SELECT EXISTS (
				SELECT 1 FROM ai_edit_prompts WHERE prompt_kind = $1
			)
		`, req.PromptKind).Scan(&exists)
		if err != nil {
			return classifyAiEditPromptDBError(err)
		}
		if exists {
			return storage.ErrAiEditPromptRevisionConflict
		}
		return nil
	}

	tag, err := conn.Exec(ctx, `
		DELETE FROM ai_edit_prompts
		WHERE prompt_kind = $1 AND revision = $2
	`, req.PromptKind, req.ExpectedRevision)
	if err != nil {
		return classifyAiEditPromptDBError(err)
	}
	if tag.RowsAffected() == 0 {
		return storage.ErrAiEditPromptRevisionConflict
	}

	return nil
}

func validateAiEditPromptUpsert(req *models.UpsertAiEditPromptRequest) error {
	if req == nil {
		return storage.ErrInvalidAiEditPromptKind
	}
	if !models.IsAiEditPromptKind(req.PromptKind) {
		return fmt.Errorf("%w: %q", storage.ErrInvalidAiEditPromptKind, req.PromptKind)
	}
	if strings.TrimSpace(req.Content) == "" {
		return storage.ErrInvalidAiEditPromptContent
	}
	if len(req.Content) > models.MaxAiEditPromptBytes {
		return storage.ErrAiEditPromptContentTooLarge
	}
	if req.ExpectedRevision < 0 {
		return storage.ErrInvalidAiEditPromptRevision
	}
	return nil
}

// classifyAiEditPromptDBError preserves a distinct missing-migration signal.
// Read callers may safely fall back to compiled defaults for this error; write
// callers must surface it because an override was not persisted.
func classifyAiEditPromptDBError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "42P01" {
		return fmt.Errorf("%w: %v", storage.ErrAiEditPromptTableMissing, err)
	}
	return err
}

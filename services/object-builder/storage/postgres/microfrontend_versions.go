package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type microfrontendVersionsRepo struct {
	db *psqlpool.Pool
}

func NewMicrofrontendVersionsRepo(db *psqlpool.Pool) storage.MicrofrontendVersionsRepoI {
	return &microfrontendVersionsRepo{db: db}
}

func (r *microfrontendVersionsRepo) Create(ctx context.Context, req *nb.CreateMicrofrontendVersionRequest) (*nb.MicrofrontendVersion, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "microfrontendVersionsRepo.Create")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		id  = uuid.NewString()
		now = time.Now()
	)

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE microfrontend_versions SET is_current = false WHERE microfrontend_id = $1 AND deleted_at IS NULL`,
		req.GetMicrofrontendId(),
	)
	if err != nil {
		return nil, err
	}

	var (
		v         nb.MicrofrontendVersion
		createdAt time.Time
		updatedAt time.Time
	)

	err = tx.QueryRow(ctx,
		`INSERT INTO microfrontend_versions (guid, microfrontend_id, commit_message, files, is_current, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, true, $5, $5)
		 RETURNING guid, microfrontend_id, commit_message, files, is_current, created_at, updated_at`,
		id, req.GetMicrofrontendId(), req.GetCommitMessage(), req.GetFiles(), now,
	).Scan(&v.Guid, &v.MicrofrontendId, &v.CommitMessage, &v.Files, &v.IsCurrent, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	v.CreatedAt = createdAt.Format(time.RFC3339)
	v.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &v, nil
}

func (r *microfrontendVersionsRepo) GetList(ctx context.Context, req *nb.GetMicrofrontendVersionListRequest) (*nb.GetMicrofrontendVersionListResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "microfrontendVersionsRepo.GetList")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	limit := int(req.GetLimit())
	if limit <= 0 {
		limit = 20
	}
	offset := int(req.GetOffset())

	var count int32
	err = conn.QueryRow(ctx,
		`SELECT COUNT(*) FROM microfrontend_versions WHERE microfrontend_id = $1 AND deleted_at IS NULL`,
		req.GetMicrofrontendId(),
	).Scan(&count)
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(ctx,
		`SELECT guid, microfrontend_id, commit_message, is_current, created_at, updated_at
		 FROM microfrontend_versions
		 WHERE microfrontend_id = $1 AND deleted_at IS NULL
		 ORDER BY created_at DESC
		 LIMIT $2 OFFSET $3`,
		req.GetMicrofrontendId(), limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var versions []*nb.MicrofrontendVersion
	for rows.Next() {
		var (
			v         nb.MicrofrontendVersion
			createdAt time.Time
			updatedAt time.Time
		)
		if err = rows.Scan(&v.Guid, &v.MicrofrontendId, &v.CommitMessage, &v.IsCurrent, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		v.CreatedAt = createdAt.Format(time.RFC3339)
		v.UpdatedAt = updatedAt.Format(time.RFC3339)
		versions = append(versions, &v)
	}

	return &nb.GetMicrofrontendVersionListResponse{
		Versions: versions,
		Count:    count,
	}, nil
}

func (r *microfrontendVersionsRepo) GetVersion(ctx context.Context, req *nb.GetMicrofrontendVersionRequest) (*nb.MicrofrontendVersion, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "microfrontendVersionsRepo.GetVersion")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		v         nb.MicrofrontendVersion
		createdAt time.Time
		updatedAt time.Time
		files     sql.NullString
	)

	err = conn.QueryRow(ctx,
		`SELECT guid, microfrontend_id, commit_message, files, is_current, created_at, updated_at
		 FROM microfrontend_versions
		 WHERE guid = $1 AND deleted_at IS NULL`,
		req.GetGuid(),
	).Scan(&v.Guid, &v.MicrofrontendId, &v.CommitMessage, &files, &v.IsCurrent, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	v.Files = files.String
	v.CreatedAt = createdAt.Format(time.RFC3339)
	v.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &v, nil
}

func (r *microfrontendVersionsRepo) SetCurrent(ctx context.Context, req *nb.SetCurrentMicrofrontendVersionRequest) (*nb.MicrofrontendVersion, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "microfrontendVersionsRepo.SetCurrent")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	_, err = tx.Exec(ctx,
		`UPDATE microfrontend_versions SET is_current = false WHERE microfrontend_id = $1 AND deleted_at IS NULL`,
		req.GetMicrofrontendId(),
	)
	if err != nil {
		return nil, err
	}

	var (
		v         nb.MicrofrontendVersion
		createdAt time.Time
		updatedAt time.Time
		files     sql.NullString
	)

	err = tx.QueryRow(ctx,
		`UPDATE microfrontend_versions SET is_current = true, updated_at = now()
		 WHERE guid = $1 AND deleted_at IS NULL
		 RETURNING guid, microfrontend_id, commit_message, files, is_current, created_at, updated_at`,
		req.GetGuid(),
	).Scan(&v.Guid, &v.MicrofrontendId, &v.CommitMessage, &files, &v.IsCurrent, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	v.Files = files.String
	v.CreatedAt = createdAt.Format(time.RFC3339)
	v.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &v, nil
}

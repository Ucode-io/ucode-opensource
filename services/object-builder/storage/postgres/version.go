package postgres

import (
	"context"
	"database/sql"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type versionRepo struct {
	db *psqlpool.Pool
}

func NewVersionRepo(db *psqlpool.Pool) storage.VersionRepoI {
	return &versionRepo{
		db: db,
	}
}

func (v *versionRepo) Create(ctx context.Context, req *nb.CreateVersionRequest) (resp *nb.Version, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	versionId := uuid.NewString()

	query := `INSERT INTO "version" (
					id,
					name,
					is_current,
					description,
					version_number,
					user_info
	) VALUES ($1, $2, $3, $4, $5, $6)`

	_, err = conn.Exec(ctx, query,
		versionId,
		req.Name,
		req.IsCurrent,
		req.Description,
		req.VersionNumber,
		req.UserInfo,
	)
	if err != nil {
		return &nb.Version{}, err
	}

	return v.GetSingle(ctx, &nb.VersionPrimaryKey{Id: versionId, ProjectId: req.ProjectId})
}

func (v *versionRepo) GetList(ctx context.Context, req *nb.GetVersionListRequest) (resp *nb.GetVersionListResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version.GetList")
	defer dbSpan.Finish()
	resp = &nb.GetVersionListResponse{}

	var (
		offset int = 0
		limit  int = 20
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT 
			id,
			name,
			is_current,
			description,
			version_number,
			user_info,
			created_at
	FROM "version" 
	ORDER BY created_at DESC
	LIMIT $1 OFFSET $2
	`

	if req.Limit > 0 {
		limit = int(req.Limit)
	}
	if req.Offset > 0 {
		limit = int(req.Offset)
	}

	rows, err := conn.Query(ctx, query, limit, offset)
	if err != nil {
		return &nb.GetVersionListResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		row := &nb.Version{}

		var (
			name           sql.NullString
			is_current     sql.NullBool
			description    sql.NullString
			version_number sql.NullInt16
			user_info      sql.NullString
			created_at     sql.NullString
		)

		err = rows.Scan(
			&row.Id,
			&name,
			&is_current,
			&description,
			&version_number,
			&user_info,
			&created_at,
		)
		if err != nil {
			return &nb.GetVersionListResponse{}, err
		}

		row.Name = name.String
		row.IsCurrent = is_current.Bool
		row.Description = description.String
		row.VersionNumber = int32(version_number.Int16)
		row.UserInfo = user_info.String
		row.CreatedAt = created_at.String
		resp.Versions = append(resp.Versions, row)
	}

	return resp, nil
}

func (v *versionRepo) GetSingle(ctx context.Context, req *nb.VersionPrimaryKey) (resp *nb.Version, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version.GetSingle")
	defer dbSpan.Finish()
	resp = &nb.Version{}

	var (
		name           sql.NullString
		is_current     sql.NullBool
		description    sql.NullString
		version_number sql.NullInt16
		user_info      sql.NullString
		created_at     sql.NullString
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT 
			id,
			name,
			is_current,
			description,
			version_number,
			user_info,
			created_at
	FROM "version" WHERE id = $1`

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&name,
		&is_current,
		&description,
		&version_number,
		&user_info,
		&created_at,
	)
	if err != nil {
		return resp, err
	}

	resp.Name = name.String
	resp.IsCurrent = is_current.Bool
	resp.Description = description.String
	resp.VersionNumber = int32(version_number.Int16)
	resp.UserInfo = user_info.String
	resp.CreatedAt = created_at.String

	return resp, nil
}

func (v *versionRepo) Update(ctx context.Context, req *nb.Version) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version.Update")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `UPDATE "version" SET
			name = $2,
			is_current = $3,
			description = $4,
			version_number = $5,
			user_info = $6
	WHERE id = $1
	`

	_, err = conn.Exec(ctx, query,
		req.Id,
		req.Name,
		req.IsCurrent,
		req.Description,
		req.VersionNumber,
		req.UserInfo,
	)
	if err != nil {
		return err
	}

	return nil
}

func (v *versionRepo) Delete(ctx context.Context, req *nb.VersionPrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "version.Delete")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `DELETE FROM "version" WHERE id = $1`

	_, err = conn.Exec(ctx, query, req.Id)
	if err != nil {
		return err
	}

	return nil
}

func (v *versionRepo) CreateMany(ctx context.Context, req *nb.CreateManyVersionRequest) error {
	return nil
}

func (v *versionRepo) UpdateLive(ctx context.Context, req *nb.VersionPrimaryKey) error {
	return nil
}

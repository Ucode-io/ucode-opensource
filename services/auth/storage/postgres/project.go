package postgres

import (
	"context"
	"database/sql"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type projectRepo struct {
	db *Pool
}

func NewProjectRepo(db *Pool) storage.ProjectRepoI {
	return &projectRepo{
		db: db,
	}
}

func (r *projectRepo) Create(ctx context.Context, entity *pb.CreateProjectRequest) (pKey *pb.ProjectPrimaryKey, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Create")
	defer dbSpan.Finish()

	query := `INSERT INTO "project" (
		id,
		company_id,
		name,
		domain
	) VALUES (
		$1,
		$2,
		$3,
		$4
	)`

	uuid, err := uuid.NewRandom()
	if err != nil {
		return pKey, err
	}

	_, err = r.db.Exec(ctx, query,
		uuid.String(),
		entity.CompanyId,
		entity.Name,
		entity.Domain,
	)

	pKey = &pb.ProjectPrimaryKey{
		Id: uuid.String(),
	}

	return pKey, err
}

func (r *projectRepo) GetByPK(ctx context.Context, pKey *pb.ProjectPrimaryKey) (res *pb.Project, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Create")
	defer dbSpan.Finish()

	res = &pb.Project{}
	query := `SELECT
		id,
		company_id,
		name,
		domain,
		TO_CHAR(created_at, ` + config.DatabaseQueryTimeLayout + `) AS created_at,
		TO_CHAR(updated_at, ` + config.DatabaseQueryTimeLayout + `) AS updated_at
	FROM
		"project"
	WHERE
		id = $1`

	var (
		createdAt sql.NullString
		updatedAt sql.NullString
		domain    sql.NullString
	)

	err = r.db.QueryRow(ctx, query, pKey.Id).Scan(
		&res.Id,
		&res.CompanyId,
		&res.Name,
		&domain,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return res, err
	}

	if domain.Valid {
		res.Domain = domain.String
	}

	if createdAt.Valid {
		res.CreatedAt = createdAt.String
	}

	if updatedAt.Valid {
		res.UpdatedAt = updatedAt.String
	}

	return res, nil
}

func (r *projectRepo) GetList(ctx context.Context, queryParam *pb.GetProjectListRequest) (res *pb.GetProjectListResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.GetList")
	defer dbSpan.Finish()

	res = &pb.GetProjectListResponse{}
	params := make(map[string]any)
	var arr []any
	query := `SELECT
		id,
		company_id,
		name,
		domain,
		TO_CHAR(created_at, ` + config.DatabaseQueryTimeLayout + `) AS created_at,
		TO_CHAR(updated_at, ` + config.DatabaseQueryTimeLayout + `) AS updated_at
	FROM
		"project"`
	filter := " WHERE 1=1"
	order := " ORDER BY created_at"
	arrangement := " DESC"
	offset := " OFFSET 0"
	limit := " LIMIT 10"

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND ((name) ILIKE ('%' || :search || '%'))"
	}

	if len(queryParam.CompanyId) > 0 {
		params["company_id"] = queryParam.CompanyId
		filter += " AND company_id = :company_id"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "project"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err = r.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return res, err
	}

	q := query + filter + order + arrangement + offset + limit
	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := r.db.Query(ctx, q, arr...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		obj := &pb.Project{}
		err = rows.Scan(
			&obj.Id,
			&obj.CompanyId,
			&obj.Name,
			&obj.Domain,
			&obj.CreatedAt,
			&obj.UpdatedAt,
		)

		if err != nil {
			return res, err
		}

		res.Projects = append(res.Projects, obj)
	}

	return res, nil
}

func (r *projectRepo) Update(ctx context.Context, entity *pb.UpdateProjectRequest) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Update")
	defer dbSpan.Finish()

	query := `UPDATE "project" SET
		name = :name,
		domain = :domain,
		updated_at = now()
	WHERE
		id = :id`

	params := map[string]any{
		"id":     entity.Id,
		"name":   entity.Name,
		"domain": entity.Domain,
	}

	q, arr := helper.ReplaceQueryParams(query, params)
	result, err := r.db.Exec(ctx, q, arr...)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, err
}

func (r *projectRepo) Delete(ctx context.Context, pKey *pb.ProjectPrimaryKey) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "project.Delete")
	defer dbSpan.Finish()

	query := `DELETE FROM "project" WHERE id = $1`

	result, err := r.db.Exec(ctx, query, pKey.Id)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, err
}

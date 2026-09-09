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

type companyRepo struct {
	db *Pool
}

func NewCompanyRepo(db *Pool) storage.CompanyRepoI {
	return &companyRepo{
		db: db,
	}
}

func (r *companyRepo) Register(ctx context.Context, entity *pb.RegisterCompanyRequest) (pKey *pb.CompanyPrimaryKey, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.Register")
	defer dbSpan.Finish()

	query := `INSERT INTO "company" (
		id,
		name
	) VALUES (
		$1,
		$2
	)`

	uuid, err := uuid.NewRandom()
	if err != nil {
		return pKey, err
	}

	_, err = r.db.Exec(ctx, query,
		uuid.String(),
		entity.Name,
	)

	pKey = &pb.CompanyPrimaryKey{
		Id: uuid.String(),
	}

	return pKey, err
}

func (r *companyRepo) Update(ctx context.Context, entity *pb.UpdateCompanyRequest) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.Update")
	defer dbSpan.Finish()

	query := `UPDATE "company" SET
		name = :name,
		updated_at = now()
	WHERE
		id = :id`

	params := map[string]any{
		"id":   entity.Id,
		"name": entity.Name,
	}

	q, arr := helper.ReplaceQueryParams(query, params)
	result, err := r.db.Exec(ctx, q, arr...)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, err
}

func (r *companyRepo) Remove(ctx context.Context, pKey *pb.CompanyPrimaryKey) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.Remove")
	defer dbSpan.Finish()

	query := `DELETE FROM "company" WHERE id = $1`

	result, err := r.db.Exec(ctx, query, pKey.Id)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, err
}

func (r *companyRepo) GetList(ctx context.Context, queryParam *pb.GetComapnyListRequest) (*pb.GetListCompanyResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetList")
	defer dbSpan.Finish()

	res := &pb.GetListCompanyResponse{}
	params := make(map[string]any)
	var arr []any
	query := `SELECT
		id, 
		name,
		TO_CHAR(created_at, ` + config.DatabaseQueryTimeLayout + `) AS created_at,
		TO_CHAR(updated_at, ` + config.DatabaseQueryTimeLayout + `) AS updated_at
	FROM
		"company"`
	filter := " WHERE 1=1"
	order := " ORDER BY created_at"
	arrangement := " DESC"
	offset := " OFFSET 0"
	limit := ""

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND (name ILIKE ('%' || :search || '%'))"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "company"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := r.db.QueryRow(ctx, cQ, arr...).Scan(
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
		obj := &pb.Company{}
		var (
			createdAt sql.NullString
			updatedAt sql.NullString
		)

		err = rows.Scan(
			&obj.Id,
			&obj.Name,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return res, err
		}

		if createdAt.Valid {
			obj.CreatedAt = createdAt.String
		}

		if updatedAt.Valid {
			obj.UpdatedAt = updatedAt.String
		}

		res.Companies = append(res.Companies, obj)
	}

	return res, nil
}

func (r *companyRepo) GetByID(ctx context.Context, pKey *pb.CompanyPrimaryKey) (*pb.Company, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.GetByID")
	defer dbSpan.Finish()

	res := &pb.Company{}
	query := `SELECT
		id,
		name,
		TO_CHAR(created_at, ` + config.DatabaseQueryTimeLayout + `) AS created_at,
		TO_CHAR(updated_at, ` + config.DatabaseQueryTimeLayout + `) AS updated_at
	FROM
		"company"
	WHERE
		id = $1`

	err := r.db.QueryRow(ctx, query, pKey.Id).Scan(
		&res.Id,
		&res.Name,
		&res.CreatedAt,
		&res.UpdatedAt,
	)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (r *companyRepo) TransferOwnership(ctx context.Context, companyID, ownerID string) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "company.TransferOwnership")
	defer dbSpan.Finish()

	query := `UPDATE "company" SET
		owner_id = :owner_id,
		updated_at = now()
	WHERE
		id = :id`

	params := map[string]any{
		"id":       companyID,
		"owner_id": ownerID,
	}

	q, arr := helper.ReplaceQueryParams(query, params)
	result, err := r.db.Exec(ctx, q, arr...)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, err
}

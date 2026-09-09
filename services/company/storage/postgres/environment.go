package postgres

import (
	"context"
	"database/sql"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/models"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
)

type environmentRepo struct {
	db *Pool
}

func NewEnvironmentRepo(db *Pool) repo.EnvironmentStorageI {
	return &environmentRepo{
		db: db,
	}
}

func (c *environmentRepo) Create(ctx context.Context, in *pb.CreateEnvironmentRequest) (*pb.Environment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "envoirment.Create")
	defer dbSpan.Finish()

	var (
		response pb.Environment = pb.Environment{}
		err      error
	)

	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	insertEnvironment :=
		`INSERT INTO environment(
			id,
			project_id,
			name,
			display_color,
			description
		) VALUES ($1, $2, $3, $4, $5) 
		RETURNING 
			id,
			project_id,
			name,
			display_color,
			description`

	err = c.db.QueryRow(
		ctx,
		insertEnvironment,
		uuid,
		in.ProjectId,
		in.Name,
		in.DisplayColor,
		in.Description,
	).Scan(
		&response.Id,
		&response.ProjectId,
		&response.Name,
		&response.DisplayColor,
		&response.Description,
	)

	if err != nil {
		return nil, err
	}

	return &response, nil
}

func (c *environmentRepo) GetById(ctx context.Context, in *pb.EnvironmentPrimaryKey) (*pb.Environment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "envoirment.GetById")
	defer dbSpan.Finish()

	var (
		err  error
		resp pb.Environment = pb.Environment{}
	)

	stmt :=
		`SELECT
			id,
			project_id,
			name,
			display_color,
			description,
			data,
			access_type
		FROM environment
		WHERE id = $1 `

	err = c.db.QueryRow(ctx, stmt, in.Id).Scan(
		&resp.Id,
		&resp.ProjectId,
		&resp.Name,
		&resp.DisplayColor,
		&resp.Description,
		&resp.Data,
		&resp.AccessType,
	)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *environmentRepo) GetList(ctx context.Context, in *pb.GetEnvironmentListRequest) (*pb.GetEnvironmentListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "envoirment.GetList")
	defer dbSpan.Finish()

	var (
		res    = &pb.GetEnvironmentListResponse{}
		params = make(map[string]any)

		arr   []any
		query = `
			SELECT
				e.id,
				e.project_id,
				e.name,
				e.display_color,
				e.description,
				e.created_at,
				e.updated_at,
				e.data,
				e.access_type,
				re.id as resource_environment_id,
				re.node_type,
				re.resource_type
			FROM
				"environment" e
			INNER JOIN
				resource_environment re ON e.id = re.environment_id
			WHERE (re.resource_type = 1 OR re.resource_type = 3)
				`
		order       = " ORDER BY e.created_at"
		arrangement = " DESC"
		offset      = " OFFSET 0"
		limit       = " LIMIT 10"
		filter      = " "
	)

	if len(in.Search) > 0 {
		params["search"] = in.Search
		filter += " AND ((e.name) ILIKE ('%' || :search || '%'))"
	}

	if len(in.ProjectId) > 0 {
		params["project_id"] = in.ProjectId
		filter += " AND e.project_id = :project_id"
	}
	if in.Ids != nil {
		params["id"] = in.Ids
		filter += " AND e.id=ANY(:id)"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "environment" e WHERE true=true` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)

	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return res, err
	}

	q := query + filter + order + arrangement + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)

	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		obj := &pb.EnvironmentWithResources{}
		var (
			createdAt sql.NullString
			updatedAt sql.NullString
		)

		err = rows.Scan(
			&obj.Id,
			&obj.ProjectId,
			&obj.Name,
			&obj.DisplayColor,
			&obj.Description,
			&createdAt,
			&updatedAt,
			&obj.Data,
			&obj.AccessType,
			&obj.ResourceEnvironmentId,
			&obj.NodeType,
			&obj.ResourceType,
		)
		if err != nil {
			return res, err
		}

		res.Environments = append(res.Environments, obj)
	}

	return res, nil
}

func (c *environmentRepo) Update(ctx context.Context, in models.Environment) (*pb.Environment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "envoirment.Update")
	defer dbSpan.Finish()

	var (
		err error

		updateQuery = `
			UPDATE environment
			SET 
				name = $1,
				display_color = $2,
				description = $3,
				updated_at = current_timestamp,
				data = $5
			WHERE id = $4`
	)
	_, err = c.db.Exec(
		ctx,
		updateQuery,
		in.Name,
		in.DisplayColor,
		in.Description,
		in.Id,
		in.Data,
	)

	if err != nil {
		return nil, err
	}

	resp, err := c.GetById(ctx, &pb.EnvironmentPrimaryKey{Id: in.Id})
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *environmentRepo) Delete(ctx context.Context, in *pb.EnvironmentPrimaryKey) (*pb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "envoirment.Delete")
	defer dbSpan.Finish()

	query := `
		DELETE FROM environment
		WHERE id = $1
	`
	sqlResult, err := c.db.Exec(
		ctx,
		query,
		in.Id,
	)
	if err != nil {
		return nil, err
	}

	rowsAffected := sqlResult.RowsAffected()

	if rowsAffected < 1 {
		return nil, pgx.ErrNoRows
	}

	return &pb.Empty{}, nil
}

func (c *environmentRepo) GetListWithPKs(ctx context.Context, in []string) (*pb.GetEnvironmentListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "envoirment.GetListWithPKs")
	defer dbSpan.Finish()

	res := &pb.GetEnvironmentListResponse{}

	query := `SELECT
			id,
			project_id,
			name,
			display_color,
			description,
			created_at,
			updated_at,
			data
		FROM
			"environment"`
	filter := " WHERE true=true"
	order := " ORDER BY created_at"
	arrangement := " DESC"

	filter += ` AND id = ANY($1::uuid[])`

	queryCount := `SELECT count(1) FROM "environment"` + filter

	err := c.db.QueryRow(ctx, queryCount, pq.Array(in)).Scan(&res.Count)
	if err != nil {
		return res, err
	}

	q := query + filter + order + arrangement

	rows, err := c.db.Query(ctx, q, pq.Array(in))
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		obj := &pb.EnvironmentWithResources{}
		var (
			createdAt sql.NullString
			updatedAt sql.NullString
		)

		err = rows.Scan(
			&obj.Id,
			&obj.ProjectId,
			&obj.Name,
			&obj.DisplayColor,
			&obj.Description,
			&createdAt,
			&updatedAt,
			&obj.Data,
		)

		if err != nil {
			return res, err
		}

		res.Environments = append(res.Environments, obj)
	}

	return res, nil
}

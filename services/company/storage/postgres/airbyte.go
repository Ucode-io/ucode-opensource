package postgres

import (
	"context"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type airbyteRepo struct {
	db *Pool
}

// NewAdminRepo ...
func NewAirbyteRepo(db *Pool) repo.AirbyteStorageI {
	return &airbyteRepo{
		db: db,
	}
}

func (a *airbyteRepo) Create(ctx context.Context, airbyte *pb.CreateAirbyteRequest) (*pb.CreateAirbyteResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "airbyte.Create")
	defer dbSpan.Finish()

	var (
		err       error
		result    string
		airbyteId = uuid.New().String()
	)

	insertAirbyte :=
		`INSERT INTO airbyte(
			id,
			project_id,
			environment_id,
			resource_id,
			schedule_time,
			workspace_id,
			source_id,
			destination_id,
			connection_id,
			connection_name
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`

	err = a.db.QueryRow(
		ctx,
		insertAirbyte,
		airbyteId,
		airbyte.ProjectId,
		airbyte.EnvironmentId,
		airbyte.ResourceId,
		airbyte.ScheduleTime,
		airbyte.WorkspaceId,
		airbyte.SourceId,
		airbyte.DestinationId,
		airbyte.ConnectionId,
		airbyte.ConnectionName,
	).Scan(&result)

	if err != nil {
		return nil, err
	}

	return &pb.CreateAirbyteResponse{
		Id: result,
	}, nil
}

func (a *airbyteRepo) GetById(ctx context.Context, req *pb.GetAirbyteByIdRequest) (*pb.Airbyte, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "airbyte.GetById")
	defer dbSpan.Finish()

	var (
		err  error
		resp = pb.Airbyte{}
	)

	query := `SELECT 
    	a.id,
    	a.project_id,
    	a.environment_id,
    	a.resource_id,
    	a.schedule_time,
    	a.workspace_id,
    	a.source_id,
    	a.destination_id,
    	a.connection_id,
    	a.connection_name,
    	r.vault_path,
		r.host,
		r.port,
		r.username,
		r.database
	FROM airbyte a 
	JOIN resource r ON a.resource_id = r.id
	WHERE a.id = $1;`

	err = a.db.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&resp.ProjectId,
		&resp.EnvironmentId,
		&resp.ResourceId,
		&resp.ScheduleTime,
		&resp.WorkspaceId,
		&resp.SourceId,
		&resp.DestinationId,
		&resp.ConnectionId,
		&resp.ConnectionName,
		&resp.Password,
		&resp.Host,
		&resp.Port,
		&resp.Username,
		&resp.Database,
	)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

func (c *airbyteRepo) GetList(ctx context.Context, queryParam *pb.GetListAirbyteRequest) (*pb.GetListAirbyteResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "airbyte.GetList")
	defer dbSpan.Finish()

	var (
		res    = &pb.GetListAirbyteResponse{}
		params = make(map[string]any)
	)

	var (
		arr   []any
		query = `SELECT
			id,
			project_id,
			environment_id,
			resource_id,
			schedule_time,
			workspace_id,
			source_id,
			destination_id,
			connection_id,
			connection_name
		FROM "airbyte" `
		filter = " WHERE 1=1"
		offset = " OFFSET 0"
		limit  = " LIMIT 10"
	)

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	if len(queryParam.GetProjectId()) > 0 {
		params["project_id"] = queryParam.GetProjectId()
		filter += " AND project_id = :project_id"
	}

	if len(queryParam.GetEnvironmentId()) > 0 {
		params["environment_id"] = queryParam.GetEnvironmentId()
		filter += " AND environment_id = :environment_id"
	}

	var cQ = `SELECT count(1) FROM "airbyte"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)

	ctxQuery, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := c.db.QueryRow(ctxQuery, cQ, arr...).Scan(&res.Count); err != nil {
		return res, err
	}

	var q = query + filter + offset + limit
	q, arr = helper.ReplaceQueryParams(q, params)

	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return res, err
	}
	defer rows.Close()
	for rows.Next() {
		var obj = &pb.Airbyte{}

		err = rows.Scan(
			&obj.Id,
			&obj.ProjectId,
			&obj.EnvironmentId,
			&obj.ResourceId,
			&obj.ScheduleTime,
			&obj.WorkspaceId,
			&obj.SourceId,
			&obj.DestinationId,
			&obj.ConnectionId,
			&obj.ConnectionName,
		)
		if err != nil {
			return res, err
		}

		res.Airbytes = append(res.Airbytes, obj)
	}

	return res, nil
}

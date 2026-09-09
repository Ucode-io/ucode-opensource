package postgres

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"
)

type integrationResourceRepo struct {
	db *Pool
}

func NewIntegrationResourceRepo(db *Pool) repo.IntegrationResourceStorageI {
	return &integrationResourceRepo{db: db}
}

func (r *integrationResourceRepo) Create(ctx context.Context, req *pb.CreateIntegrationResourceRequest) (*pb.IntegrationResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.Create")
	defer dbSpan.Finish()

	var res pb.IntegrationResource
	err := r.db.QueryRow(ctx, `
		INSERT INTO integration_resource (token, project_id, environment_id, username, name, type)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, token, project_id, environment_id, username, name, type`,
		req.Token,
		req.ProjectId,
		req.EnvironmentId,
		req.Username,
		req.Name,
		req.Type.String(),
	).Scan(
		&res.Id,
		&res.Token,
		&res.ProjectId,
		&res.EnvironmentId,
		&res.Username,
		&res.Name,
		&res.Type,
	)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.Create")
	}
	return &res, nil
}

// Upsert stores the integration token for a (project, environment, type) triple,
// replacing the previous token/identity on re-connect. Atomic via the unique
// index uq_integration_resource_project_env_type.
func (r *integrationResourceRepo) Upsert(ctx context.Context, req *pb.CreateIntegrationResourceRequest) (*pb.IntegrationResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.Upsert")
	defer dbSpan.Finish()

	var res pb.IntegrationResource
	err := r.db.QueryRow(ctx, `
		INSERT INTO integration_resource (token, project_id, environment_id, username, name, type)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (project_id, environment_id, type) DO UPDATE
		SET token = EXCLUDED.token,
		    username = EXCLUDED.username,
		    name = EXCLUDED.name,
		    updated_at = now()
		RETURNING id, token, project_id, environment_id, username, name, type`,
		req.Token,
		req.ProjectId,
		req.EnvironmentId,
		req.Username,
		req.Name,
		req.Type.String(),
	).Scan(
		&res.Id,
		&res.Token,
		&res.ProjectId,
		&res.EnvironmentId,
		&res.Username,
		&res.Name,
		&res.Type,
	)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.Upsert")
	}
	return &res, nil
}

func (r *integrationResourceRepo) GetById(ctx context.Context, req *pb.IntegrationResourcePrimaryKey) (*pb.IntegrationResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.GetById")
	defer dbSpan.Finish()

	var res pb.IntegrationResource
	err := r.db.QueryRow(ctx, `
		SELECT id, token, project_id, environment_id, username, name, type
		FROM integration_resource
		WHERE id = $1`,
		req.Id,
	).Scan(
		&res.Id,
		&res.Token,
		&res.ProjectId,
		&res.EnvironmentId,
		&res.Username,
		&res.Name,
		&res.Type,
	)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.GetById")
	}
	return &res, nil
}

func (r *integrationResourceRepo) GetByUsername(ctx context.Context, req *pb.GetByUsernameRequest) (*pb.GetByUsernameResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.GetByUsername")
	defer dbSpan.Finish()

	rows, err := r.db.Query(ctx, `
		SELECT id, token, project_id, environment_id, username, name, type
		FROM integration_resource
		WHERE username = $1`,
		req.Username,
	)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.GetByUsername")
	}
	defer rows.Close()

	var resp pb.GetByUsernameResponse
	for rows.Next() {
		var ir pb.IntegrationResource
		if err := rows.Scan(&ir.Id, &ir.Token, &ir.ProjectId, &ir.EnvironmentId, &ir.Username, &ir.Name, &ir.Type); err != nil {
			return nil, r.db.HandleDatabaseError(err, "integrationResource.GetByUsername scan")
		}
		resp.IntegrationResources = append(resp.IntegrationResources, &ir)
	}
	return &resp, nil
}

func (r *integrationResourceRepo) GetList(ctx context.Context, req *pb.GetListIntegrationResourceRequest) (*pb.GetListIntegrationResourceResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.GetList")
	defer dbSpan.Finish()

	query := `
		SELECT id, token, project_id, environment_id, username, name, type
		FROM integration_resource
		WHERE project_id = $1 AND environment_id = $2`

	args := []any{req.ProjectId, req.EnvironmentId}

	if req.Type != pb.ResourceType_NOT_DECIDED {
		query += ` AND type = $3`
		args = append(args, req.Type.String())
	}

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.GetList")
	}
	defer rows.Close()

	var resp pb.GetListIntegrationResourceResponse
	for rows.Next() {
		var ir pb.IntegrationResource
		if err := rows.Scan(&ir.Id, &ir.Token, &ir.ProjectId, &ir.EnvironmentId, &ir.Username, &ir.Name, &ir.Type); err != nil {
			return nil, r.db.HandleDatabaseError(err, "integrationResource.GetList scan")
		}
		resp.IntegrationResources = append(resp.IntegrationResources, &ir)
	}
	return &resp, nil
}

func (r *integrationResourceRepo) Update(ctx context.Context, req *pb.UpdateIntegrationResourceRequest) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.Update")
	defer dbSpan.Finish()

	_, err := r.db.Exec(ctx, `
		UPDATE integration_resource
		SET name = $1, updated_at = now()
		WHERE id = $2`,
		req.Name,
		req.Id,
	)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.Update")
	}
	return &emptypb.Empty{}, nil
}

func (r *integrationResourceRepo) Delete(ctx context.Context, req *pb.IntegrationResourcePrimaryKey) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "integrationResource.Delete")
	defer dbSpan.Finish()

	_, err := r.db.Exec(ctx, `
		DELETE FROM integration_resource
		WHERE id = $1`,
		req.Id,
	)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "integrationResource.Delete")
	}
	return &emptypb.Empty{}, nil
}
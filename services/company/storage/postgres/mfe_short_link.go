package postgres

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"
)

type mfeShortLinkRepo struct {
	db *Pool
}

func NewMfeShortLinkRepo(db *Pool) repo.MfeShortLinkStorageI {
	return &mfeShortLinkRepo{db: db}
}

func (r *mfeShortLinkRepo) Create(ctx context.Context, req *pb.MfeShortLink) (*pb.MfeShortLink, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mfe_short_link.Create")
	defer dbSpan.Finish()

	query := `
		INSERT INTO mfe_short_links (slug, url, project_id, mcp_project_id, function_id)
		VALUES ($1, $2, NULLIF($3, '')::UUID, $4::UUID, $5::UUID)
		RETURNING id, slug, url,
		          COALESCE(project_id::TEXT, ''),
		          mcp_project_id::TEXT,
		          function_id::TEXT,
		          created_at::TEXT`

	row := r.db.QueryRow(ctx, query, req.Slug, req.Url, req.ProjectId, req.McpProjectId, req.FunctionId)
	if err := row.Scan(
		&req.Id, &req.Slug, &req.Url,
		&req.ProjectId, &req.McpProjectId, &req.FunctionId,
		&req.CreatedAt,
	); err != nil {
		return nil, r.db.HandleDatabaseError(err, "mfe_short_link.Create")
	}
	return req, nil
}

func (r *mfeShortLinkRepo) GetBySlug(ctx context.Context, req *pb.MfeShortLinkSlugReq) (*pb.MfeShortLink, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mfe_short_link.GetBySlug")
	defer dbSpan.Finish()

	res := &pb.MfeShortLink{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, url,
		       COALESCE(project_id::TEXT, ''),
		       COALESCE(mcp_project_id::TEXT, ''),
		       COALESCE(function_id::TEXT, ''),
		       created_at::TEXT
		FROM mfe_short_links WHERE slug = $1`, req.Slug,
	).Scan(&res.Id, &res.Slug, &res.Url, &res.ProjectId, &res.McpProjectId, &res.FunctionId, &res.CreatedAt)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "mfe_short_link.GetBySlug")
	}
	return res, nil
}

func (r *mfeShortLinkRepo) GetByFunctionId(ctx context.Context, req *pb.MfeShortLinkFunctionReq) (*pb.MfeShortLink, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mfe_short_link.GetByFunctionId")
	defer dbSpan.Finish()

	res := &pb.MfeShortLink{}
	err := r.db.QueryRow(ctx, `
		SELECT id, slug, url,
		       COALESCE(project_id::TEXT, ''),
		       COALESCE(mcp_project_id::TEXT, ''),
		       COALESCE(function_id::TEXT, ''),
		       created_at::TEXT
		FROM mfe_short_links WHERE function_id = $1::UUID`, req.FunctionId,
	).Scan(&res.Id, &res.Slug, &res.Url, &res.ProjectId, &res.McpProjectId, &res.FunctionId, &res.CreatedAt)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "mfe_short_link.GetByFunctionId")
	}
	return res, nil
}

func (r *mfeShortLinkRepo) DeleteByProjectId(ctx context.Context, req *pb.MfeShortLinkProjectReq) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "mfe_short_link.DeleteByProjectId")
	defer dbSpan.Finish()

	_, err := r.db.Exec(ctx, `DELETE FROM mfe_short_links WHERE project_id = $1::UUID`, req.ProjectId)
	if err != nil {
		return nil, r.db.HandleDatabaseError(err, "mfe_short_link.DeleteByProjectId")
	}
	return &emptypb.Empty{}, nil
}

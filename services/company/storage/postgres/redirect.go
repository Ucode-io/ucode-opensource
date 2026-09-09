package postgres

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/emptypb"
)

type redirectRepo struct {
	db *Pool
}

func NewRedirectRepo(db *Pool) repo.RedirectStorageI {
	return &redirectRepo{
		db: db,
	}
}

func (s *redirectRepo) Create(ctx context.Context, req *pb.RedirectUrl) (*pb.RedirectUrl, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "redirect.Create")
	defer dbSpan.Finish()

	query := `INSERT INTO "redirect_url" (
                            id,
                            project_id,
                            env_id,
                            "from",
                            "to",
							"order"
			) VALUES (
			          $1,
			          $2,
			          $3,
			          $4,
			          $5,
					  $6
			) RETURNING id`

	id, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}

	err = s.db.QueryRow(ctx, query,
		id,
		req.ProjectId,
		req.EnvId,
		req.From,
		req.To,
		req.Order,
	).Scan(&req.Id)

	if err != nil {
		return nil, err
	}
	return req, nil
}

func (s *redirectRepo) Update(ctx context.Context, req *pb.RedirectUrl) (*pb.RedirectUrl, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "redirect.Update")
	defer dbSpan.Finish()

	var res pb.RedirectUrl

	query := `UPDATE 
					"redirect_url"
				SET
                    "from" = $1,
                    "to" = $2,
                    updated_at = now()
			    WHERE
			        id = $3
			    RETURNING id, project_id, env_id, "from", "to", created_at::text, updated_at::text`

	err := s.db.QueryRow(ctx, query,
		req.From,
		req.To,
		req.Id,
	).Scan(
		&res.Id,
		&res.ProjectId,
		&res.EnvId,
		&res.From,
		&res.To,
		&res.CreatedAt,
		&res.UpdatedAt,
	)

	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (s *redirectRepo) GetSingle(ctx context.Context, req *pb.GetSingleRedirectUrlReq) (*pb.RedirectUrl, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "redirect.GetSingle")
	defer dbSpan.Finish()

	var res pb.RedirectUrl

	query := `
			SELECT
				id,
				project_id,
				env_id,
				"from",
				"to",
				coalesce(created_at::text, ''),
				coalesce(updated_at::text, '')
			FROM "redirect_url" r
			WHERE id = $1`

	err := s.db.QueryRow(ctx, query, req.Id).Scan(
		&res.Id,
		&res.ProjectId,
		&res.EnvId,
		&res.From,
		&res.To,
		&res.CreatedAt,
		&res.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (s *redirectRepo) GetList(ctx context.Context, req *pb.GetListRedirectUrlReq) (*pb.GetListRedirectUrlRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "redirect.GetList")
	defer dbSpan.Finish()

	var res pb.GetListRedirectUrlRes

	query := `
			SELECT
				count(*) OVER(),
				id,
				project_id,
				env_id,
				"from",
				"to",
				coalesce(created_at::text, ''),
				coalesce(updated_at::text, '')
			FROM "redirect_url" r
			WHERE project_id = $1 AND env_id = $2
			ORDER BY "order"
			OFFSET $3 LIMIT $4`

	rows, err := s.db.Query(ctx, query, req.ProjectId, req.EnvId, req.Offset, req.Limit)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var (
			row pb.RedirectUrl
		)
		err = rows.Scan(
			&res.Count,
			&row.Id,
			&row.ProjectId,
			&row.EnvId,
			&row.From,
			&row.To,
			&row.CreatedAt,
			&row.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}

		res.RedirectUrls = append(res.RedirectUrls, &row)
	}

	return &res, nil
}

func (s *redirectRepo) Delete(ctx context.Context, req *pb.DeleteRedirectUrlReq) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "redirect.Delete")
	defer dbSpan.Finish()

	query := `
		DELETE FROM "redirect_url"
		WHERE id = $1
	`
	sqlResult, err := s.db.Exec(
		ctx,
		query,
		req.Id,
	)
	if err != nil {
		return nil, err
	}

	rowsAffected := sqlResult.RowsAffected()

	if rowsAffected < 1 {
		return nil, pgx.ErrNoRows
	}

	return &emptypb.Empty{}, nil
}

func (s *redirectRepo) UpdateOrder(ctx context.Context, req *pb.UpdateOrderRedirectUrlReq) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "redirect.UpdateOrder")
	defer dbSpan.Finish()

	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			err = tx.Rollback(ctx)
		} else {
			err = tx.Commit(ctx)
		}
	}()
	for i, id := range req.GetIds() {
		query := `UPDATE 
					"redirect_url"
					SET
					"order" = $1,
					updated_at = now()
				WHERE
					id = $2`

		_, err = tx.Exec(ctx, query,
			i+1, id,
		)
		if err != nil {
			return nil, err
		}
	}

	return &emptypb.Empty{}, nil
}

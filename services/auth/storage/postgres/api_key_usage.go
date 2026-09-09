package postgres

import (
	"context"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/opentracing/opentracing-go"
)

type apiKeyUsageRepo struct {
	db *Pool
}

func NewApiKeyUsageRepo(db *Pool) storage.ApiKeyUsageRepoI {
	return &apiKeyUsageRepo{
		db: db,
	}
}

func (r *apiKeyUsageRepo) CheckLimit(ctx context.Context, req *pb.CheckLimitRequest) (*pb.CheckLimitResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_key_usage.CheckLimit")
	defer dbSpan.Finish()
	var res pb.CheckLimitResponse

	query := `
		SELECT
			is_monthly_request_limit_reached
		FROM api_keys 
		WHERE app_id = $1;
	`

	err := r.db.QueryRow(
		ctx,
		query,
		req.GetApiKey(),
	).Scan(&res.IsLimitReached)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (r *apiKeyUsageRepo) Create(ctx context.Context, req *pb.ApiKeyUsage) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_key_usage.Create")
	defer dbSpan.Finish()
	query := `
		INSERT INTO api_key_usage (
			api_key
		) VALUES(
			$1
		)
	`

	_, err := r.db.Exec(ctx, query,
		req.GetApiKey(),
	)

	return err
}

func (r *apiKeyUsageRepo) Upsert(ctx context.Context, req *pb.ApiKeyUsage) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_key_usage.Upsert")
	defer dbSpan.Finish()

	query := `
		INSERT INTO api_key_usage (
			api_key,
			request_count)
		VALUES (
			$1,
			$2
		) ON CONFLICT (api_key, creation_month) DO 
		UPDATE SET request_count = api_key_usage.request_count + $2
	`

	_, err := r.db.Exec(ctx, query,
		req.GetApiKey(),
		req.GetRequestCount(),
	)
	if err != nil {
		return err
	}

	return nil
}

func (r *apiKeyUsageRepo) UpdateMonthlyLimit(ctx context.Context) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_key_usage.UpdateMonthlyLimit")
	defer dbSpan.Finish()

	query := `	
		UPDATE api_keys SET
			is_monthly_request_limit_reached = true
  		WHERE app_id IN(
			SELECT
	  			aku.api_key
			FROM api_key_usage aku
			INNER JOIN api_keys ak ON aku.api_key = ak.app_id
			WHERE aku.creation_month = TO_CHAR(DATE_TRUNC('month', CURRENT_TIMESTAMP), 'YYYY-MM-DD')::DATE
			AND aku.request_count >= ak.monthly_request_limit
  			);
	`

	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return err
	}

	return nil
}

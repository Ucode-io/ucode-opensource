package postgres

import (
	"context"
	"database/sql"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type apiKeysRepo struct {
	db *Pool
}

func NewApiKeysRepo(db *Pool) storage.ApiKeysRepoI {
	return &apiKeysRepo{
		db: db,
	}
}

func (r *apiKeysRepo) Create(ctx context.Context, req *pb.CreateReq, appSecret, appId, id string) (*pb.CreateRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.Create")
	defer dbSpan.Finish()

	var (
		res                  pb.CreateRes
		createdAt, updatedAt sql.NullString
	)

	query := `
		INSERT INTO api_keys(
			id, 
			name, 
			app_id, 
			app_secret, 
			role_id, 
			environment_id, 
			project_id, 
			client_type_id,
			client_platform_id,
			"disable",
			client_id,
			created_at, 
			updated_at
		)
		VALUES (
			$1, 
			$2, 
			$3, 
			$4, 
			$5, 
			$6, 
			$7, 
			$8,
			$9,
			$10,
			$11,
			now(), 
			now()) 
		RETURNING id, status, name, app_id, app_secret, role_id, created_at, updated_at, environment_id, project_id, client_type_id, rps_limit, monthly_request_limit`

	err := r.db.QueryRow(ctx, query,
		id,
		req.GetName(),
		appId,
		appSecret,
		req.GetRoleId(),
		req.GetEnvironmentId(),
		req.GetProjectId(),
		req.GetClientTypeId(),
		req.GetClientPlatformId(),
		req.GetDisable(),
		req.GetClientId(),
	).Scan(&res.Id, &res.Status, &res.Name, &res.AppId, &res.AppSecret, &res.RoleId, &createdAt, &updatedAt, &res.EnvironmentId, &res.ProjectId, &res.ClientTypeId, &res.RpsLimit, &res.MonthlyRequestLimit)

	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		res.CreatedAt = createdAt.String
	}

	if updatedAt.Valid {
		res.UpdatedAt = updatedAt.String
	}

	return &res, nil
}
func (r *apiKeysRepo) GetList(ctx context.Context, req *pb.GetListReq) (*pb.GetListRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.GetList")
	defer dbSpan.Finish()

	var (
		res = pb.GetListRes{Count: 0}
	)

	query := `SELECT
				ak.id,
  				ak.status,
  				ak.name,
  				ak.app_id,
				ak.app_secret,
  				ak.role_id,
  				ak.created_at,
  				ak.updated_at,
  				ak.environment_id,
				ak.project_id,
				ak.client_type_id,
				ak.rps_limit,
				ak.monthly_request_limit,
				cp.id AS client_platform_id,
				cp.name AS client_platform_name,
				cp.subdomain AS client_platform_subdomain,
				ak.disable,
				COALESCE(ak.client_id::text, '')
			FROM
			    api_keys ak
			LEFT JOIN
			    client_platform cp
			ON 
			    ak.client_platform_id = cp.id`

	filter := ` WHERE ak.project_id = :project_id`
	params := make(map[string]any)
	offset := " OFFSET 0"
	limit := " LIMIT 10"
	order := " ORDER BY ak.created_at"
	arrangement := " DESC"
	params["project_id"] = req.GetProjectId()

	if req.Offset > 0 {
		params["offset"] = req.Offset
		offset = " OFFSET :offset"
	}

	if req.Limit > 0 {
		params["limit"] = req.Limit
		limit = " LIMIT :limit"
	}

	if len(req.Search) > 0 {
		params["search"] = req.Search
		filter += " AND (ak.name ILIKE '%' || :search || '%')"
	}

	if util.IsValidUUID(req.EnvironmentId) {
		filter += ` AND ak.environment_id = :environment_id`
		params["environment_id"] = req.GetEnvironmentId()
	}

	if util.IsValidUUID(req.ClientTypeId) {
		filter += ` AND ak.client_type_id = :client_type_id`
		params["client_type_id"] = req.ClientTypeId
	}

	if util.IsValidUUID(req.RoleId) {
		filter += ` AND ak.role_id = :role_id`
		params["role_id"] = req.RoleId
	}

	countQuery := `SELECT count(*) from api_keys ak` + filter
	countStmt, countArgs := helper.ReplaceQueryParams(countQuery, params)

	err := r.db.QueryRow(ctx, countStmt, countArgs...).Scan(
		&res.Count,
	)
	if err != nil {
		return &res, err
	}

	q := query + filter + order + arrangement + offset + limit
	stmt, args := helper.ReplaceQueryParams(q, params)

	rows, err := r.db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		row := pb.GetRes{}
		var (
			createdAt               sql.NullString
			updatedAt               sql.NullString
			clientTypeId            sql.NullString
			clientPlatformId        sql.NullString
			clientPlatformName      sql.NullString
			clientPlatformSubdomain sql.NullString
			usedCount               sql.NullInt32
		)

		err = rows.Scan(
			&row.Id,
			&row.Status,
			&row.Name,
			&row.AppId,
			&row.AppSecret,
			&row.RoleId,
			&createdAt,
			&updatedAt,
			&row.EnvironmentId,
			&row.ProjectId,
			&clientTypeId,
			&row.RpsLimit,
			&row.MonthlyRequestLimit,
			&clientPlatformId,
			&clientPlatformName,
			&clientPlatformSubdomain,
			&row.Disable,
			&row.ClientId,
		)

		if err != nil {
			return nil, err
		}

		if createdAt.Valid {
			row.CreatedAt = createdAt.String
		}

		if clientTypeId.Valid {
			row.ClientTypeId = clientTypeId.String
		}

		if updatedAt.Valid {
			row.UpdatedAt = updatedAt.String
		}

		if usedCount.Valid {
			row.UsedCount = usedCount.Int32
		}

		if clientPlatformId.Valid {
			row.ClientPlatform = &pb.ClientPlatform{
				Id:        clientPlatformId.String,
				Name:      clientPlatformName.String,
				Subdomain: clientPlatformSubdomain.String,
			}
		}

		res.Data = append(res.Data, &row)
	}

	return &res, nil
}

func (r *apiKeysRepo) Get(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.Get")
	defer dbSpan.Finish()

	var (
		res                pb.GetRes
		createdAt          sql.NullString
		updatedAt          sql.NullString
		usedCount          sql.NullInt32
		clientPlatformId   sql.NullString
		clientPlatformName sql.NullString
	)

	query := `SELECT
				ak.id,
  				ak.status,
  				ak.name,
  				ak.app_id,
  				ak.app_secret,
  				ak.role_id,
  				ak.environment_id,
				ak.project_id,
				ak.client_type_id,
				ak.rps_limit,
				ak.monthly_request_limit,
  				ak.created_at,
  				ak.updated_at,
				COALESCE(ak.client_id::text, ''),
				cp.id AS client_platform_id,
				cp.name AS client_platform_name
			FROM
			    api_keys ak
			LEFT JOIN
			    client_platform cp
			ON 
			    ak.client_platform_id = cp.id
			WHERE
			    ak.id = $1`

	err := r.db.QueryRow(ctx, query, req.GetId()).Scan(
		&res.Id,
		&res.Status,
		&res.Name,
		&res.AppId,
		&res.AppSecret,
		&res.RoleId,
		&res.EnvironmentId,
		&res.ProjectId,
		&res.ClientTypeId,
		&res.RpsLimit,
		&res.MonthlyRequestLimit,
		&createdAt,
		&updatedAt,
		&res.ClientId,
		&clientPlatformId,
		&clientPlatformName,
	)
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		res.CreatedAt = createdAt.String
	}

	if updatedAt.Valid {
		res.UpdatedAt = updatedAt.String
	}

	if usedCount.Valid {
		res.UsedCount = usedCount.Int32
	}

	if clientPlatformId.Valid {
		res.ClientPlatform = &pb.ClientPlatform{
			Id:   clientPlatformId.String,
			Name: clientPlatformName.String,
		}
	}

	return &res, nil
}

func (r *apiKeysRepo) Update(ctx context.Context, req *pb.UpdateReq) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.Update")
	defer dbSpan.Finish()

	query := `UPDATE "api_keys" SET
				status = $1,
				name = $2,
				role_id = $3,
				client_type_id = $4,
				updated_at = now()
			WHERE
			    id = $5`

	res, err := r.db.Exec(
		ctx,
		query,
		req.GetStatus(),
		req.GetName(),
		req.GetRoleId(),
		req.GetClientTypeId(),
		req.GetId(),
	)

	if err != nil {
		return 0, err
	}

	rowsAffected = res.RowsAffected()
	return
}
func (r *apiKeysRepo) Delete(ctx context.Context, req *pb.DeleteReq) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.Delete")
	defer dbSpan.Finish()

	query := `DELETE FROM "api_keys"
				WHERE id = $1`

	result, err := r.db.Exec(ctx, query, req.Id)
	if err != nil {
		return rowsAffected, err
	}

	return result.RowsAffected(), nil
}

func (r *apiKeysRepo) GetByAppId(ctx context.Context, appId string) (*pb.GetRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.GetByAppId")
	defer dbSpan.Finish()

	var (
		res       pb.GetRes
		createdAt sql.NullString
		updatedAt sql.NullString
	)

	query := `SELECT
				id,
  				status,
  				name,
  				app_id,
  				app_secret,
  				role_id,
  				environment_id,
				project_id,
				client_type_id,
				rps_limit,
				monthly_request_limit,
  				created_at,
  				updated_at
			FROM
			    api_keys
			WHERE
			    app_id = $1`

	err := r.db.QueryRow(ctx, query, appId).Scan(
		&res.Id,
		&res.Status,
		&res.Name,
		&res.AppId,
		&res.AppSecret,
		&res.RoleId,
		&res.EnvironmentId,
		&res.ProjectId,
		&res.ClientTypeId,
		&res.RpsLimit,
		&res.MonthlyRequestLimit,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}

	if createdAt.Valid {
		res.CreatedAt = createdAt.String
	}

	if updatedAt.Valid {
		res.UpdatedAt = updatedAt.String
	}
	return &res, nil
}

func (r *apiKeysRepo) GetEnvID(ctx context.Context, req *pb.GetReq) (*pb.GetRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.GetEnvID")
	defer dbSpan.Finish()

	res := &pb.GetRes{}

	query := `
		SELECT
  			environment_id,
			project_id,
			id,
			client_type_id,
			role_id,
			status,
			name,
			app_id,
			app_secret,
			rps_limit,
			monthly_request_limit
		FROM
			api_keys
		WHERE
			app_id = $1`

	err := r.db.QueryRow(ctx, query, req.GetId()).Scan(
		&res.EnvironmentId,
		&res.ProjectId,
		&res.Id,
		&res.ClientTypeId,
		&res.RoleId,
		&res.Status,
		&res.Name,
		&res.AppId,
		&res.AppSecret,
		&res.RpsLimit,
		&res.MonthlyRequestLimit,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error while scanning")
	}

	return res, nil
}

func (r *apiKeysRepo) UpdateIsMonthlyLimitReached(ctx context.Context) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.UpdateIsMonthlyLimitReached")
	defer dbSpan.Finish()

	query := `
		UPDATE api_keys SET 
			is_monthly_request_limit_reached = false
	`

	_, err := r.db.Exec(ctx, query)
	if err != nil {
		return errors.Wrap(err, "error while executing UpdateIsMonthlyLimitReached query")
	}

	return nil
}

func (a *apiKeysRepo) ListClientToken(ctx context.Context, req *pb.ListClientTokenRequest) (*pb.ListClientTokenResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.ListClientToken")
	defer dbSpan.Finish()

	var (
		res = &pb.ListClientTokenResponse{Count: 0}
	)

	query := `SELECT
				id,
  				client_id,
  				info,
  				given_time
			FROM
			    client_tokens`

	filter := ` WHERE client_id = :client_id`
	params := make(map[string]any)
	offset := " OFFSET 0"
	limit := " LIMIT 10"
	order := " ORDER BY given_time"
	arrangement := " DESC"
	params["client_id"] = req.GetClientId()

	if req.Offset > 0 {
		params["offset"] = req.Offset
		offset = " OFFSET :offset"
	}

	if req.Limit > 0 {
		params["limit"] = req.Limit
		limit = " LIMIT :limit"
	}

	countQuery := `SELECT COUNT(*) FROM client_tokens` + filter
	countStmt, countArgs := helper.ReplaceQueryParams(countQuery, params)

	err := a.db.QueryRow(ctx, countStmt, countArgs...).Scan(
		&res.Count,
	)
	if err != nil {
		return res, err
	}

	q := query + filter + order + arrangement + offset + limit
	stmt, args := helper.ReplaceQueryParams(q, params)

	rows, err := a.db.Query(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var (
			row       pb.ClientIdToken
			givenTime sql.NullString
		)

		err = rows.Scan(
			&row.Id,
			&row.ClientId,
			&row.Info,
			&givenTime,
		)

		if err != nil {
			return nil, err
		}

		row.GivenTime = givenTime.String

		res.ClientTokens = append(res.ClientTokens, &row)
	}

	return res, nil
}

func (r *apiKeysRepo) CreateClientToken(ctx context.Context, clientId string, info map[string]any) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.CreateClientToken")
	defer dbSpan.Finish()

	id := uuid.NewString()

	query := `
		INSERT INTO client_tokens(
			id, 
			client_id,
			info
		)
		VALUES (
			$1, 
			$2, 
			$3)`

	_, err := r.db.Exec(ctx, query,
		id,
		clientId,
		info,
	)

	if err != nil {
		return err
	}

	return nil
}

func (r *apiKeysRepo) CheckClientIdStatus(ctx context.Context, clientId string) (bool, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.CheckClientIdStatus")
	defer dbSpan.Finish()

	var status bool

	query := `SELECT
  				status
			FROM
			    api_keys
			WHERE
			    client_id = $1`

	err := r.db.QueryRow(ctx, query, clientId).Scan(&status)
	if err != nil {
		return false, err
	}

	return status, nil
}
func (r *apiKeysRepo) GetProjectApiKeysCount(ctx context.Context, projectId string) (int32, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "api_keys.GetProjectApiKeysCount")
	defer dbSpan.Finish()

	var count int32
	query := `SELECT count(*) FROM api_keys WHERE project_id = $1`

	err := r.db.QueryRow(ctx, query, projectId).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

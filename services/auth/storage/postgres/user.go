package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/auth/api/models"
	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/emptypb"
)

type userRepo struct {
	logger logger.LoggerI
	db     *Pool
}

func NewUserRepo(db *Pool, logger logger.LoggerI) storage.UserRepoI {
	return &userRepo{
		db:     db,
		logger: logger,
	}
}

func (r *userRepo) Create(ctx context.Context, entity *pb.CreateUserRequest) (pKey *pb.UserPrimaryKey, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.create")
	defer dbSpan.Finish()

	query := `INSERT INTO "user" (id, phone, email, login, password, company_id, hash_type, tin, google_id)
			  VALUES ($1, $2, $3, $4, $5, $6, 'bcrypt', $7, $8)`

	id := uuid.New().String()

	_, err = r.db.Exec(ctx, query,
		id,
		entity.GetPhone(),
		entity.GetEmail(),
		entity.GetLogin(),
		entity.GetPassword(),
		entity.GetCompanyId(),
		entity.GetTin(),
		entity.GetGoogleId(),
	)
	if err != nil {
		return pKey, helper.HandleDatabaseError(err, r.logger, "Create user: failed to create")
	}

	pKey = &pb.UserPrimaryKey{Id: id}

	return pKey, nil
}

func (r *userRepo) GetByPK(ctx context.Context, pKey *pb.UserPrimaryKey) (res *pb.User, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.getbypk")
	defer dbSpan.Finish()

	res = &pb.User{}
	var (
		lan  pb.Language
		time pb.Timezone
	)
	query := `SELECT
		u.id,
		u.phone,
		u.email,
		u.login,
		u.company_id,
		u.password,
		u.hash_type,
		COALESCE(u.google_id, '') AS google_id
	FROM
		"user" u
	WHERE
		u.id = $1`
	err = r.db.QueryRow(ctx, query, pKey.Id).Scan(
		&res.Id,
		&res.Phone,
		&res.Email,
		&res.Login,
		&res.CompanyId,
		&res.Password,
		&res.HashType,
		&res.GoogleId,
	)
	if err != nil {
		return res, err
	}
	res.Language = &lan
	res.Timezone = &time

	return res, nil
}

func (r *userRepo) GetListByPKs(ctx context.Context, pKeys *pb.UserPrimaryKeyList) (res *pb.GetUserListResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetListByPKs")
	defer dbSpan.Finish()

	res = &pb.GetUserListResponse{}
	query := `SELECT
			u.id,
			u.phone,
			u.email,
			u.login,
			u.password,
			u.created_at,
			u.updated_at,
			u.company_id,
			up.status
		FROM
			"user" u
		JOIN
			user_project up ON u.id = up.user_id
		WHERE
			u.id = ANY($1)
			AND up.project_id = $2
	`

	rows, err := r.db.Query(ctx, query, pKeys.Ids, pKeys.ProjectId)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			createdAt, updatedAt sql.NullString
		)

		user := &pb.User{}
		err = rows.Scan(
			&user.Id,
			&user.Phone,
			&user.Email,
			&user.Login,
			&user.Password,
			&createdAt,
			&updatedAt,
			&user.CompanyId,
			&user.Status,
		)

		if err != nil {
			return res, err
		}

		res.Users = append(res.Users, user)
	}

	return res, nil
}

func (r *userRepo) GetList(ctx context.Context, queryParam *pb.GetUserListRequest) (res *pb.GetUserListResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetList")
	defer dbSpan.Finish()

	res = &pb.GetUserListResponse{}
	params := make(map[string]any)
	var arr []any
	query := `SELECT
		id,
		company_id,
		phone,
		email,
		login,
		created_at,
		updated_at
	FROM
		"user"`
	filter := " WHERE 1=1"
	order := " ORDER BY created_at"
	arrangement := " DESC"
	offset := " OFFSET 0"
	limit := " LIMIT 10"

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND ((phone || email || login) ILIKE ('%' || :search || '%'))"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "user"` + filter
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
		obj := &pb.User{}
		var (
			active                                     sql.NullInt32
			expiresAt, createdAt, updatedAt, companyID sql.NullString
		)

		err = rows.Scan(
			&obj.Id,
			&companyID,
			&obj.Phone,
			&obj.Email,
			&obj.Login,
			&active,
			&expiresAt,
			&createdAt,
			&updatedAt,
		)

		if err != nil {
			return res, err
		}

		res.Users = append(res.Users, obj)
	}

	return res, nil
}

func (r *userRepo) Update(ctx context.Context, entity *pb.UpdateUserRequest) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.update")
	defer dbSpan.Finish()

	query := `UPDATE "user" SET
		company_id = $1,
		phone = $2,
		email = $3,
		login = $4,
		updated_at = now()
	WHERE
		id = $5`

	params := []any{
		entity.GetCompanyId(),
		entity.GetPhone(),
		entity.GetEmail(),
		entity.GetLogin(),
		entity.GetId(),
	}

	result, err := r.db.Exec(ctx, query, params...)
	if err != nil {
		return 0, err
	}

	rowsAffected = result.RowsAffected()

	return rowsAffected, nil
}

func (r *userRepo) Delete(ctx context.Context, pKey *pb.UserPrimaryKey) (int64, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.delete")
	defer dbSpan.Finish()

	if pKey.GetIsTest() {
		queryDeleteFromUserProject := `DELETE FROM user_project WHERE user_id = $1`

		result, err := r.db.Exec(ctx, queryDeleteFromUserProject, pKey.Id)
		if err != nil {
			return 0, err
		}
		rowsAffected := result.RowsAffected()
		if rowsAffected == 0 {
			return 0, errors.New("user not found")
		}

	}

	result, err := r.db.Exec(ctx, `DELETE FROM "user" WHERE id = $1`, pKey.GetId())
	if err != nil {
		return 0, errors.Wrap(err, "delete user error")
	}
	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return 0, errors.New("user not found")
	}

	return 0, nil
}

func (r *userRepo) GetByUsername(ctx context.Context, username string) (res *pb.User, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetByUsername")
	defer dbSpan.Finish()

	res = &pb.User{}

	query := `SELECT
		id,
		phone,
		email,
		login,
		password,
		hash_type,
		COALESCE(tin, '') as tin,
		COALESCE(google_id, '') as google_id
	FROM
		"user"
	WHERE`

	lowercasedUsername := ""

	if util.IsValidEmailNew(username) {
		query = query + ` LOWER(email) = $1`
		lowercasedUsername = strings.ToLower(username)
	} else if util.IsValidPhone(username) {
		query = query + ` phone = $1`
		lowercasedUsername = username
	} else if util.IsValidTin(username) {
		query = query + ` tin = $1`
		lowercasedUsername = username
	} else {
		query = query + ` login = $1`
		lowercasedUsername = username
	}

	err = r.db.QueryRow(ctx, query, lowercasedUsername).Scan(
		&res.Id,
		&res.Phone,
		&res.Email,
		&res.Login,
		&res.Password,
		&res.HashType,
		&res.Tin,
		&res.GoogleId,
	)
	if err == pgx.ErrNoRows && util.IsValidEmailNew(username) {
		queryIf := `
					SELECT
						id,
						phone,
						email,
						login,
						password,
						hash_type,
						COALESCE(tin, '') as tin,
						COALESCE(google_id, '') as google_id
					FROM
						"user"
					WHERE
				`

		queryIf = queryIf + ` LOWER(login) = $1`

		err = r.db.QueryRow(ctx, queryIf, lowercasedUsername).Scan(
			&res.Id,
			&res.Phone,
			&res.Email,
			&res.Login,
			&res.Password,
			&res.HashType,
			&res.Tin,
			&res.GoogleId,
		)
		if err == pgx.ErrNoRows {
			return res, nil
		}
		return res, helper.HandleDatabaseError(err, r.logger, "GetByUsername: failed to scan")
	}

	if err == pgx.ErrNoRows {
		return res, nil
	}

	if err != nil {
		return res, helper.HandleDatabaseError(err, r.logger, "GetByUsername: failed to get user")
	}

	return res, nil
}

func (r *userRepo) GetByGoogleID(ctx context.Context, googleID string) (res *pb.User, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetByGoogleID")
	defer dbSpan.Finish()

	res = &pb.User{}
	if strings.TrimSpace(googleID) == "" {
		return res, nil
	}

	query := `SELECT
		id,
		phone,
		email,
		login,
		password,
		hash_type,
		COALESCE(tin, '') as tin,
		COALESCE(company_id::text, '') as company_id,
		COALESCE(google_id, '') as google_id
	FROM
		"user"
	WHERE
		google_id = $1`

	err = r.db.QueryRow(ctx, query, googleID).Scan(
		&res.Id,
		&res.Phone,
		&res.Email,
		&res.Login,
		&res.Password,
		&res.HashType,
		&res.Tin,
		&res.CompanyId,
		&res.GoogleId,
	)
	if err == pgx.ErrNoRows {
		return res, nil
	}
	if err != nil {
		return res, helper.HandleDatabaseError(err, r.logger, "GetByGoogleID: failed to get user")
	}

	return res, nil
}

func (r *userRepo) UpdateGoogleID(ctx context.Context, userID, googleID string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.UpdateGoogleID")
	defer dbSpan.Finish()

	if strings.TrimSpace(userID) == "" || strings.TrimSpace(googleID) == "" {
		return errors.New("user id and google id are required")
	}

	_, err := r.db.Exec(ctx, `UPDATE "user" SET google_id = $1, updated_at = now() WHERE id = $2`, googleID, userID)
	if err != nil {
		return helper.HandleDatabaseError(err, r.logger, "UpdateGoogleID: failed to update user")
	}

	return nil
}

func (r *userRepo) ResetPassword(ctx context.Context, user *pb.ResetPasswordRequest, tx pgx.Tx) (rowsAffected int64, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.resetpassword")
	defer dbSpan.Finish()

	params := map[string]any{
		"id": user.UserId,
	}

	query := `UPDATE "user" SET
		updated_at = now(),
		hash_type = 'bcrypt'`

	if len(user.GetLogin()) > 0 {
		query += `, login = :login`
		params["login"] = user.Login
	}

	if len(user.GetEmail()) > 0 {
		query += `, email = :email`
		params["email"] = user.Email
	}

	if len(user.GetPhone()) > 0 {
		query += `, phone = :phone`
		params["phone"] = user.Phone
	}

	if len(user.GetPassword()) > 0 {
		query += `, password = :password`
		params["password"] = user.Password
	}

	if len(user.GetTin()) > 0 {
		query += `, tin = :tin`
		params["tin"] = user.Tin
	}

	query += ` WHERE id = :id`

	q, arr := helper.ReplaceQueryParams(query, params)
	if tx == nil {
		result, err := r.db.Exec(ctx, q, arr...)
		if err != nil {
			return 0, errors.Wrap(err, "failed to update user")
		}

		rowsAffected = result.RowsAffected()
	} else {
		result, err := tx.Exec(ctx, q, arr...)
		if err != nil {
			return 0, errors.Wrap(err, "failed to update user with tx")
		}
		rowsAffected = result.RowsAffected()
	}

	return rowsAffected, nil
}

func (r *userRepo) GetUserProjectClientTypes(ctx context.Context, req *pb.UserInfoPrimaryKey) (res *pb.GetUserProjectClientTypesResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserProjectClientTypes")
	defer dbSpan.Finish()

	res = &pb.GetUserProjectClientTypesResponse{}

	query := `SELECT 
				array_agg(client_type_id) as client_type_ids
			FROM user_project 
			WHERE user_id = $1
			AND project_id = $2
			AND client_type_id IS NOT NULL
			GROUP BY  user_id`

	err = r.db.QueryRow(ctx, query, req.UserId, req.ProjectId).Scan(
		&res.ClientTypeIds,
	)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (r *userRepo) GetUserProjects(ctx context.Context, userId string) (*pb.GetUserProjectsRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserProjects")
	defer dbSpan.Finish()

	res := pb.GetUserProjectsRes{}

	query := `SELECT company_id,
       			array_agg(DISTINCT project_id) AS project_ids
				FROM user_project
				WHERE user_id = $1
				GROUP BY company_id`

	rows, err := r.db.Query(ctx, query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			projectIDs = make([]string, 0)
			company    string
		)

		err = rows.Scan(&company, &projectIDs)
		if err != nil {
			return nil, err
		}

		res.Companies = append(res.Companies, &pb.UserCompany{
			Id:         company,
			ProjectIds: projectIDs,
		})
	}

	return &res, nil
}

func (r *userRepo) GetUserProjectsEnv(ctx context.Context, userId, envId string) (*pb.GetUserProjectsRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserProjects")
	defer dbSpan.Finish()

	res := pb.GetUserProjectsRes{}

	query := `SELECT company_id,
       			array_agg(DISTINCT project_id) AS project_ids
				FROM user_project
				WHERE user_id = $1
				AND env_id = $2
				GROUP BY company_id`

	rows, err := r.db.Query(ctx, query, userId, envId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			projectIDs = make([]string, 0)
			company    string
		)

		err = rows.Scan(&company, &projectIDs)
		if err != nil {
			return nil, err
		}

		res.Companies = append(res.Companies, &pb.UserCompany{
			Id:         company,
			ProjectIds: projectIDs,
		})
	}

	return &res, nil
}

func (r *userRepo) AddUserToProject(ctx context.Context, req *pb.AddUserToProjectReq) (*pb.AddUserToProjectRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.AddUserToProject")
	defer dbSpan.Finish()

	var (
		res                         = pb.AddUserToProjectRes{}
		clientTypeId, roleId, envId pgtype.UUID
	)
	if req.GetClientTypeId() != "" {
		err := clientTypeId.Set(req.GetClientTypeId())
		if err != nil {
			return nil, errors.Wrap(err, "failed to set client type id")
		}
	} else {
		clientTypeId.Status = pgtype.Null
	}
	if req.GetRoleId() != "" {
		err := roleId.Set(req.GetRoleId())
		if err != nil {
			return nil, errors.Wrap(err, "failed to set role id")
		}
	} else {
		roleId.Status = pgtype.Null
	}
	if req.GetEnvId() != "" {
		err := envId.Set(req.GetEnvId())
		if err != nil {
			return nil, errors.Wrap(err, "failed to set env id")
		}
	} else {
		envId.Status = pgtype.Null
	}

	query := `INSERT INTO
			user_project(user_id, company_id, project_id, client_type_id, role_id, env_id)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING user_id, company_id, project_id, client_type_id, role_id, env_id`

	err := r.db.QueryRow(ctx,
		query,
		req.GetUserId(),
		req.GetCompanyId(),
		req.GetProjectId(),
		clientTypeId,
		roleId,
		envId,
	).Scan(
		&res.UserId,
		&res.CompanyId,
		&res.ProjectId,
		&clientTypeId,
		&roleId,
		&envId,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert user to project")
	}
	if roleId.Status != pgtype.Null {
		req.RoleId = fmt.Sprintf("%v", roleId.Status)
	}
	if clientTypeId.Status != pgtype.Null {
		req.ClientTypeId = fmt.Sprintf("%v", clientTypeId.Status)
	}
	if envId.Status != pgtype.Null {
		req.EnvId = fmt.Sprintf("%v", envId.Status)
	}

	return &res, nil
}

func (r *userRepo) UpdateUserToProject(ctx context.Context, req *pb.AddUserToProjectReq) (*pb.AddUserToProjectRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.UpdateUserToProject")
	defer dbSpan.Finish()

	var (
		res                         = pb.AddUserToProjectRes{}
		status, query               string
		clientTypeId, roleId, envId pgtype.UUID
		err                         error
		tx                          pgx.Tx
	)

	tx, err = r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if req.GetClientTypeId() != "" {
		err = clientTypeId.Set(req.GetClientTypeId())
		if err != nil {
			return nil, err
		}
	} else {
		clientTypeId.Status = pgtype.Null
	}

	if req.GetRoleId() != "" {
		err = roleId.Set(req.GetRoleId())
		if err != nil {
			return nil, err
		}
	} else {
		roleId.Status = pgtype.Null
	}

	if req.GetEnvId() != "" {
		err = envId.Set(req.GetEnvId())
		if err != nil {
			return nil, err
		}
	} else {
		envId.Status = pgtype.Null
	}

	if req.Status == config.UserStatusBlocked {
		query = `DELETE FROM session WHERE 
			user_id = $1 AND
			project_id = $2 AND 
			env_id = $3 AND
			role_id = $4 AND 
			client_type_id = $5`

		_, err = tx.Exec(ctx, query, req.UserId, req.ProjectId, envId, roleId, clientTypeId)
		if err != nil {
			return nil, err
		}
	}

	query = `UPDATE "user_project" 
			  SET client_type_id = $4, role_id = $5, status = $6
			  WHERE user_id = $1 AND project_id = $2 AND env_id = $3
			  RETURNING user_id, company_id, project_id, client_type_id, role_id, env_id, status`

	err = tx.QueryRow(ctx, query,
		req.UserId, req.ProjectId, envId, clientTypeId, roleId, req.Status,
	).Scan(
		&res.UserId, &res.CompanyId, &res.ProjectId,
		&clientTypeId, &roleId, &envId, &status,
	)
	if err != nil {
		if err.Error() != "no rows in result set" {
			return nil, err
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, err
	}

	if roleId.Status != pgtype.Null {
		req.RoleId = fmt.Sprintf("%v", roleId.Status)
	}
	if clientTypeId.Status != pgtype.Null {
		req.ClientTypeId = fmt.Sprintf("%v", clientTypeId.Status)
	}

	return &res, nil
}

func (r *userRepo) GetProjectsByUserId(ctx context.Context, req *pb.GetProjectsByUserIdReq) (*pb.GetProjectsByUserIdRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetProjectsByUserId")
	defer dbSpan.Finish()

	res := pb.GetProjectsByUserIdRes{}

	query := `SELECT
				project_id,
				env_id
			from user_project
			where user_id = $1`

	rows, err := r.db.Query(ctx, query, req.GetUserId())
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		var (
			projectID sql.NullString
			envID     sql.NullString
		)

		err = rows.Scan(&projectID, &envID)
		if err != nil {
			return nil, err
		}

		res.UserProjects = append(res.UserProjects, &pb.UserProject{
			ProjectId: projectID.String,
			EnvId:     envID.String,
		})
	}

	return &res, nil
}

func (r *userRepo) GetUserIds(ctx context.Context, req *pb.GetUserListRequest) (*[]string, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserIds")
	defer dbSpan.Finish()

	var (
		query  = `SELECT array_agg(user_id) FROM "user_project" WHERE 1=1`
		tmp    = make([]string, 0, 20)
		filter = ` AND project_id = :project_id`
		params = map[string]any{"project_id": req.ProjectId}
	)

	if len(req.Search) > 0 {
		params["search"] = req.Search
		filter += " AND ((name || phone || email || login) ILIKE ('%' || :search || '%'))"
	}

	query, args := helper.ReplaceQueryParams(query+filter, params)

	if err := r.db.QueryRow(ctx, query, args...).Scan(&tmp); err != nil {
		return nil, err
	}

	return &tmp, nil
}

func (r *userRepo) GetUserByLoginType(ctx context.Context, req *pb.GetUserByLoginTypesRequest) (*pb.GetUserByLoginTypesResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserByLoginType")
	defer dbSpan.Finish()
	var (
		userId, filter string
		params         = map[string]any{}
	)

	query := `SELECT
				id
			from "user" WHERE `
	if req.Email != "" {
		filter = "email = :email"
		params["email"] = strings.ToLower(req.Email)
	}
	if req.Login != "" {
		if filter != "" {
			filter += " OR login = :login"
		} else {
			filter = "login = :login"
		}
		params["login"] = req.Login
	}
	if req.Phone != "" {
		if filter != "" {
			filter += " OR phone = :phone"
		} else {
			filter = "phone = :phone"
		}
		params["phone"] = req.Phone
	}

	lastQuery, args := helper.ReplaceQueryParams(query+filter, params)
	err := r.db.QueryRow(ctx, lastQuery, args...).Scan(&userId)
	if err == pgx.ErrNoRows {
		return nil, errors.New("not found")
	} else if err != nil {
		return nil, err
	}
	return &pb.GetUserByLoginTypesResponse{
		UserId: userId,
	}, nil
}

func (c *userRepo) GetListLanguage(cntx context.Context, in *pb.GetListSettingReq) (*models.ListLanguage, error) {
	dbSpan, _ := opentracing.StartSpanFromContext(cntx, "user.GetListLanguage")
	defer dbSpan.Finish()

	var (
		res    models.ListLanguage
		arr    []any
		params = make(map[string]any)
	)
	ctx, cancel := context.WithCancel(cntx)
	defer cancel()

	query := `SELECT
			id,
			name,
			short_name,
			native_name
		FROM
			"language"`
	filter := " "
	offset := " OFFSET 0"
	limit := " LIMIT 10"

	if len(in.GetSearch()) > 0 {
		params["search"] = in.GetSearch()
		filter = " WHERE (((name) ILIKE ('%' || :search || '%'))" +
			" OR ((short_name) ILIKE ('%' || :search || '%'))" +
			" OR ((native_name) ILIKE ('%' || :search || '%')))"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "language"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return &res, err
	}

	q := query + filter + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return &res, err
	}
	defer rows.Close()

	for rows.Next() {
		var obj models.Language

		err = rows.Scan(
			&obj.Id,
			&obj.Name,
			&obj.ShortName,
			&obj.NativeName,
		)

		if err != nil {
			return &res, err
		}

		res.Language = append(res.Language, &obj)
	}

	return &res, nil
}

func (c *userRepo) GetListTimezone(cntx context.Context, in *pb.GetListSettingReq) (*models.ListTimezone, error) {
	dbSpan, _ := opentracing.StartSpanFromContext(cntx, "user.GetListTimezone")
	defer dbSpan.Finish()

	var (
		res    models.ListTimezone
		arr    []any
		params = make(map[string]any)
	)

	ctx, cancel := context.WithCancel(cntx)
	defer cancel()

	query := `SELECT
			id,
			"name",
			"text"
		FROM
			"timezone"`
	filter := " "
	offset := " OFFSET 0"
	limit := " LIMIT 10"

	if len(in.GetSearch()) > 0 {
		params["search"] = in.GetSearch()
		filter = " WHERE (((name) ILIKE ('%' || :search || '%'))" +
			"OR ((text) ILIKE ('%' || :search || '%')))"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "timezone"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(&res.Count)
	if err != nil {
		return &res, err
	}

	q := query + filter + offset + limit

	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return &res, err
	}
	defer rows.Close()

	for rows.Next() {
		var obj models.Timezone

		err = rows.Scan(
			&obj.Id,
			&obj.Name,
			&obj.Text,
		)
		if err != nil {
			return &res, err
		}

		res.Timezone = append(res.Timezone, &obj)
	}

	return &res, nil
}

func (r *userRepo) V2ResetPassword(ctx context.Context, req *pb.V2ResetPasswordRequest) (int64, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.V2ResetPassword")
	defer dbSpan.Finish()

	var (
		params                      = make(map[string]any)
		subQueryEmail, subQueryPass string
	)
	if req.GetPassword() != "" {
		subQueryPass = "password = :password, "
		params["password"] = req.GetPassword()
	}
	if req.GetEmail() != "" {
		subQueryEmail = "email = :email, "
		params["email"] = req.GetEmail()
	}

	query := `UPDATE "user" SET ` + subQueryPass + subQueryEmail + `
		updated_at = now(),
		hash_type = 'bcrypt'
	WHERE
		id = :id`
	params["id"] = req.GetUserId()

	q, arr := helper.ReplaceQueryParams(query, params)

	result, err := r.db.Exec(ctx, q, arr...)
	if err != nil {
		return 0, err
	}

	rowsAffected := result.RowsAffected()

	return rowsAffected, err
}

func (c *userRepo) GetUserProjectByAllFields(ctx context.Context, req models.GetUserProjectByAllFieldsReq) (bool, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserProjectByAllFields")
	defer dbSpan.Finish()

	var (
		isExists bool
		count    int
	)
	query := `
		SELECT
			COUNT(1)
		FROM
			"user_project"
		WHERE user_id = $1 AND project_id = $2 AND company_id = $3
		AND client_type_id = $4 AND role_id = $5`
	err := c.db.QueryRow(
		ctx,
		query,
		req.UserId,
		req.ProjectId,
		req.CompanyId,
		req.ClientTypeId,
		req.RoleId,
	).Scan(&count)
	if err != nil {
		return isExists, nil
	}
	if count > 0 {
		isExists = true
	}

	return isExists, nil
}

func (r *userRepo) DeleteUserFromProject(ctx context.Context, req *pb.DeleteSyncUserRequest) (*empty.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.DeleteUserFromProject")
	defer dbSpan.Finish()

	query, params := deleteUserFromProjectQuery(req)

	q, args := helper.ReplaceQueryParams(query, params)
	_, err := r.db.Exec(ctx, q, args...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to delete user from project")
	}

	return &empty.Empty{}, nil
}

func deleteUserFromProjectQuery(req *pb.DeleteSyncUserRequest) (string, map[string]any) {
	query := `DELETE FROM "user_project"
	WHERE user_id = :user_id AND project_id = :project_id`
	params := map[string]any{
		"user_id":    req.GetUserId(),
		"project_id": req.GetProjectId(),
	}

	optionalFilters := []struct {
		column string
		key    string
		value  string
	}{
		{column: "company_id", key: "company_id", value: req.GetCompanyId()},
		{column: "client_type_id", key: "client_type_id", value: req.GetClientTypeId()},
		{column: "role_id", key: "role_id", value: req.GetRoleId()},
		{column: "env_id", key: "env_id", value: req.GetEnvironmentId()},
	}

	for _, filter := range optionalFilters {
		if filter.value == "" {
			continue
		}
		query += " AND " + filter.column + " = :" + filter.key
		params[filter.key] = filter.value
	}

	return query, params
}

func (r *userRepo) DeleteUsersFromProject(ctx context.Context, req *pb.DeleteManyUserRequest) (*empty.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.DeleteUsersFromProject")
	defer dbSpan.Finish()

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `DELETE FROM "user_project" 
				WHERE 
					project_id = :project_id AND  
					company_id = :company_id AND
					env_id = :env_id`
	for _, user := range req.GetUsers() {
		params := map[string]any{}
		params["project_id"] = req.GetProjectId()
		params["company_id"] = req.GetCompanyId()
		params["env_id"] = req.GetEnvironmentId()

		if user.UserId != "" {
			query = query + " AND user_id = :user_id"
			params["user_id"] = user.GetUserId()
		} else {
			return nil, errors.New("user id is required")
		}

		if user.GetClientTypeId() != "" {
			query = query + " AND client_type_id = :client_type_id"
			params["client_type_id"] = user.GetClientTypeId()
		}

		if user.GetRoleId() != "" {
			query = query + " AND role_id = :role_id"
			params["role_id"] = user.GetRoleId()
		}

		q, args := helper.ReplaceQueryParams(query, params)
		_, err = tx.Exec(ctx, q, args...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to delete user from project")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return &empty.Empty{}, nil
}

func (r *userRepo) GetAllUserProjects(ctx context.Context) ([]string, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetAllUserProjects")
	defer dbSpan.Finish()

	count := 0
	query := `SELECT count(distinct project_id) FROM user_project`

	err := r.db.QueryRow(ctx, query).Scan(&count)
	if err != nil {
		return nil, err
	}

	res := make([]string, 0, count)

	query = `SELECT distinct project_id
				FROM user_project`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var project string

		err = rows.Scan(&project)
		if err != nil {
			return nil, err
		}

		res = append(res, project)
	}

	return res, nil
}

func (r *userRepo) UpdateUserProjects(ctx context.Context, envId, projectId string) (*emptypb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.UpdateUserProjects")
	defer dbSpan.Finish()

	query := `UPDATE user_project SET env_id = $1
	  WHERE project_id = $2`

	_, err := r.db.Exec(ctx, query, envId, projectId)
	if err != nil {
		return nil, err
	}

	return &emptypb.Empty{}, nil
}

func (r *userRepo) GetUserEnvProjects(ctx context.Context, userId string) (*models.GetUserEnvProjectRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserEnvProjects")
	defer dbSpan.Finish()

	res := models.GetUserEnvProjectRes{
		EnvProjects: map[string][]string{},
	}

	query := `SELECT project_id,
       			array_agg(DISTINCT env_id)
				FROM user_project
				WHERE user_id = $1
				GROUP BY project_id				`

	rows, err := r.db.Query(ctx, query, userId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			envIds    = make([]string, 0)
			projectId string
		)

		err = rows.Scan(&projectId, &envIds)
		if err != nil {
			return nil, err
		}

		res.EnvProjects[projectId] = envIds
	}

	return &res, nil
}

func (r *userRepo) GetUserEnvProjectsV2(ctx context.Context, userId, envId string) (*models.GetUserEnvProjectRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserEnvProjects")
	defer dbSpan.Finish()

	res := models.GetUserEnvProjectRes{
		EnvProjects: map[string][]string{},
	}

	query := `SELECT project_id,
       			array_agg(DISTINCT env_id)
				FROM user_project
				WHERE user_id = $1
				AND env_id = $2
				GROUP BY project_id				`

	rows, err := r.db.Query(ctx, query, userId, envId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			envIds    = make([]string, 0)
			projectId string
		)

		err = rows.Scan(&projectId, &envIds)
		if err != nil {
			return nil, err
		}

		res.EnvProjects[projectId] = envIds
	}

	return &res, nil
}

func (r *userRepo) CHeckUserProject(ctx context.Context, id, projectId string) (res *pb.User, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.CHeckUserProject")
	defer dbSpan.Finish()

	res = &pb.User{}

	query := `SELECT
		user_id
	FROM
		"user_project"
	WHERE user_id = $1 AND project_id = $2`
	err = r.db.QueryRow(ctx, query, id, projectId).Scan(&res.ProjectId)
	if err != nil {
		return res, err
	}

	return res, nil
}

func (r *userRepo) UpdatePassword(ctx context.Context, userId, password string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.update_password")
	defer dbSpan.Finish()

	query := `UPDATE "user" SET
		hash_type = :hash_type,
		password = :password
	WHERE
		id = :id`

	params := map[string]any{
		"id":        userId,
		"hash_type": "bcrypt",
		"password":  password,
	}
	q, arr := helper.ReplaceQueryParams(query, params)
	_, err := r.db.Exec(ctx, q, arr...)
	if err != nil {
		return errors.Wrap(err, "failed to update user password")
	}

	return nil
}

func (r *userRepo) V2GetByUsername(ctx context.Context, username, strategy string) (res *pb.User, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.V2GetByUsername")
	defer dbSpan.Finish()

	res = &pb.User{}

	query := fmt.Sprintf(`SELECT
		id,
		phone,
		email,
		login,
		password,
		hash_type
	FROM
		"user"
	WHERE %s = $1`, strategy)

	err = r.db.QueryRow(ctx, query, username).Scan(
		&res.Id,
		&res.Phone,
		&res.Email,
		&res.Login,
		&res.Password,
		&res.HashType,
	)

	if err == pgx.ErrNoRows {
		return res, nil
	}

	if err != nil {
		return res, errors.Wrap(err, "failed to get user by username")
	}

	return res, nil
}

func (r *userRepo) GetUserStatus(ctx context.Context, userId, projectId string) (status string, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserStatus")
	defer dbSpan.Finish()

	query := `SELECT status FROM user_project WHERE user_id = $1 AND project_id = $2`

	err = r.db.QueryRow(ctx, query, userId, projectId).Scan(&status)
	if err != nil {
		if err == pgx.ErrNoRows {
			return config.UserStatusActive, nil
		}
		return "", err
	}

	return
}

func (r *userRepo) GetUserProjectByUserIdProjectIdEnvId(ctx context.Context, userId, projectId, envId string) (string, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetUserProjectByUserIdProjectIdEnvId")
	defer dbSpan.Finish()

	query := `SELECT
		client_type_id
	FROM
		user_project
	WHERE user_id = $1 AND project_id = $2 AND env_id = $3`

	var (
		clientType sql.NullString
	)

	err := r.db.QueryRow(ctx, query, userId, projectId, envId).Scan(
		&clientType,
	)
	if err != nil {
		return "", err
	}

	return clientType.String, nil
}

func (r *userRepo) GetProjectUsersCount(ctx context.Context, projectId string) (int32, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetProjectUsersCount")
	defer dbSpan.Finish()

	var count int32

	err := r.db.QueryRow(ctx, `
		SELECT COUNT(DISTINCT user_id)
		FROM user_project
		WHERE project_id = $1
		  AND status = $2`,
		projectId,
		config.UserStatusActive,
	).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func (r *userRepo) GetCompanyUsersCount(ctx context.Context, companyId string) (int32, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "user.GetCompanyUsersCount")
	defer dbSpan.Finish()

	var count int32
	err := r.db.QueryRow(ctx, `SELECT COUNT(DISTINCT user_id) FROM user_project WHERE company_id = $1`, companyId).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

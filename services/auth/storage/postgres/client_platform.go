package postgres

import (
	"context"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/google/uuid"
)

type clientPlatformRepo struct {
	db *Pool
}

func NewClientPlatformRepo(db *Pool) storage.ClientPlatformRepoI {
	return &clientPlatformRepo{
		db: db,
	}
}

func (r *clientPlatformRepo) Create(ctx context.Context, entity *pb.CreateClientPlatformRequest) (pKey *pb.ClientPlatformPrimaryKey, err error) {
	query := `INSERT INTO "client_platform" (
		id,
		name,
		subdomain
	) VALUES (
		$1,
		$2,
		$3,
	)`

	uuid, err := uuid.NewRandom()
	if err != nil {
		return pKey, err
	}

	_, err = r.db.Exec(ctx, query,
		uuid.String(),
		entity.Name,
		entity.Subdomain,
	)

	pKey = &pb.ClientPlatformPrimaryKey{
		Id: uuid.String(),
	}

	return pKey, err
}

func (r *clientPlatformRepo) GetByPK(ctx context.Context, pKey *pb.ClientPlatformPrimaryKey) (res *pb.ClientPlatform, err error) {

	res = &pb.ClientPlatform{}
	query := `SELECT
		id,
		project_id,
		name,
		subdomain
	FROM
		"client_platform"
	WHERE
		id = $1`

	err = r.db.QueryRow(ctx, query, pKey.Id).Scan(
		&res.Id,
		&res.ProjectId,
		&res.Name,
		&res.Subdomain,
	)

	if err != nil {
		return res, err
	}

	return res, nil
}

func (r *clientPlatformRepo) GetList(ctx context.Context, queryParam *pb.GetClientPlatformListRequest) (res *pb.GetClientPlatformListResponse, err error) {
	res = &pb.GetClientPlatformListResponse{}
	var arr []any
	params := make(map[string]any)
	query := `SELECT
		id,
		name,
		subdomain
	FROM
		"client_platform"`
	filter := " WHERE true"
	order := " ORDER BY created_at"
	arrangement := " DESC"
	offset := " OFFSET 0"
	limit := " LIMIT 10"

	if len(queryParam.Search) > 0 {
		params["search"] = queryParam.Search
		filter += " AND ((name || subdomain) ILIKE ('%' || :search || '%'))"
	}

	if queryParam.Offset > 0 {
		params["offset"] = queryParam.Offset
		offset = " OFFSET :offset"
	}

	if queryParam.Limit > 0 {
		params["limit"] = queryParam.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1) FROM "client_platform"` + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err = r.db.QueryRow(ctx, cQ, arr...).Scan(&res.Count)

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
		obj := &pb.ClientPlatform{}
		err = rows.Scan(
			&obj.Id,
			&obj.Name,
			&obj.Subdomain,
		)
		if err != nil {
			return res, err
		}
		res.ClientPlatforms = append(res.ClientPlatforms, obj)
	}

	return res, nil
}

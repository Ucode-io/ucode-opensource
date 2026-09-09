package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

type resourceRepo struct {
	db *Pool
}

func NewResourceRepo(db *Pool) repo.ResourceStorageI {
	return &resourceRepo{
		db: db,
	}
}

func (c *resourceRepo) GetResourceList(ctx context.Context, in *pb.GetResourceListRequest) (*pb.GetResourceListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetResourceList")
	defer dbSpan.Finish()

	res := &pb.GetResourceListResponse{}
	params := make(map[string]any)

	var arr []any
	querySelect := `SELECT
			res.id,
			res.project_id,
			res.title,
			res.resource_type,
			res.is_configured,
			res.vault_path,
			res.created_at,
			res.updated_at,
			res_env.node_type`

	qFrom := `
		FROM resource res 
		LEFT JOIN resource_environment res_env ON res.id = res_env.resource_id`

	filter := " WHERE 1=1"
	order := " ORDER BY res.created_at"
	arrangement := " DESC"
	offset := " OFFSET 0"
	limit := " LIMIT 10"

	if len(in.Search) > 0 {
		params["search"] = in.Search
		filter += " AND ((res.title) ILIKE ('%' || :search || '%'))"
	}

	if util.IsValidUUID(in.GetProjectId()) {
		params["project_id"] = in.ProjectId
		filter += " AND res.project_id = :project_id"
	}

	includeEnv := false
	if util.IsValidUUID(in.GetEnvironmentId()) {
		includeEnv = true
		params["environment_id"] = in.GetEnvironmentId()
		querySelect += `, res_env`
		filter += " AND res_env.environment_id = :environment_id"
	}

	if in.Offset > 0 {
		params["offset"] = in.Offset
		offset = " OFFSET :offset"
	}

	if in.Limit > 0 {
		params["limit"] = in.Limit
		limit = " LIMIT :limit"
	}

	cQ := `SELECT count(1)` + qFrom + filter
	cQ, arr = helper.ReplaceQueryParams(cQ, params)
	err := c.db.QueryRow(ctx, cQ, arr...).Scan(
		&res.Count,
	)
	if err != nil {
		return res, err
	}

	q := querySelect + qFrom + filter + order + arrangement + offset + limit
	q, arr = helper.ReplaceQueryParams(q, params)
	rows, err := c.db.Query(ctx, q, arr...)
	if err != nil {
		return res, err
	}
	defer rows.Close()

	for rows.Next() {
		obj := &pb.Resource{}

		var (
			createdAt    sql.NullString
			updatedAt    sql.NullString
			node_type    sql.NullString
			env          string
			resourceType string
		)

		if includeEnv {
			err = rows.Scan(
				&obj.Id,
				&obj.ProjectId,
				&obj.Title,
				&resourceType,
				&obj.IsConfigured,
				&obj.VaultPath,
				&createdAt,
				&updatedAt,
				&node_type,
				&env,
			)
		} else {
			err = rows.Scan(
				&obj.Id,
				&obj.ProjectId,
				&obj.Title,
				&resourceType,
				&obj.IsConfigured,
				&obj.VaultPath,
				&createdAt,
				&updatedAt,
				&node_type,
			)
		}

		if err != nil {
			return res, err
		}

		obj.NodeType = node_type.String
		obj.ResourceType = pb.ResourceType(pb.ResourceType_value[resourceType])

		res.Resources = append(res.Resources, obj)
	}

	return res, nil
}

func (c *resourceRepo) GetResource(ctx context.Context, in *pb.GetResourceRequest) (*pb.Resource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetResource")
	defer dbSpan.Finish()

	var (
		resp         *pb.Resource = &pb.Resource{Credentials: &pb.Resource_Credentials{}}
		createdAt    sql.NullString
		updatedAt    sql.NullString
		resourceType string
	)

	stmt :=
		`SELECT
			r.id,
			r.project_id,
			r.title,
			r.resource_type,
			r.is_configured,
			r.vault_path,
			r.host,
			r.port::varchar(255),
			r.username,
			r."database",
			r.created_at,
			r.updated_at,
			coalesce(re.node_type::VARCHAR, '')
		FROM resource as r
		LEFT JOIN resource_environment as re on r.id = re.resource_id
		WHERE r.id = $1 `

	err := c.db.QueryRow(ctx, stmt, in.Id).Scan(
		&resp.Id,
		&resp.ProjectId,
		&resp.Title,
		&resourceType,
		&resp.IsConfigured,
		&resp.VaultPath,
		&resp.Credentials.Host,
		&resp.Credentials.Port,
		&resp.Credentials.Username,
		&resp.Credentials.Database,
		&createdAt,
		&updatedAt,
		&resp.NodeType,
	)
	if err != nil {
		return nil, err
	}

	resp.ResourceType = pb.ResourceType(pb.ResourceType_value[resourceType])

	return resp, nil
}

func (c *resourceRepo) AddResource(ctx context.Context, in *pb.Resource) (*pb.Resource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.AddResource")
	defer dbSpan.Finish()

	resp := &pb.Resource{}

	if !util.IsValidUUID(in.GetId()) {
		uuid, err := uuid.NewRandom()
		if err != nil {
			return nil, err
		}

		in.Id = uuid.String()
	}

	query :=
		`INSERT INTO "resource"(
				"id",
				"project_id",
                "title",
                "resource_type",
                "is_configured",
                "vault_path",
                "host",
                "port",
                "database",
                "username"
			) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`

	_, err := c.db.Exec(ctx, query,
		in.GetId(),
		in.ProjectId,
		in.GetTitle(),
		pb.ResourceType_name[int32(in.GetResourceType())],
		in.GetIsConfigured(),
		in.GetVaultPath(),
		in.GetCredentials().GetHost(),
		in.GetCredentials().GetPort(),
		in.GetCredentials().GetDatabase(),
		in.GetCredentials().GetUsername(),
	)
	if err != nil {
		return nil, err
	}
	resp.Id = in.GetId()

	return resp, nil
}

func (c *resourceRepo) UpdateResource(ctx context.Context, in *pb.UpdateResourceRequest) (*pb.UpdateResourceResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.UpdateResource")
	defer dbSpan.Finish()

	resp := &pb.UpdateResourceResponse{}
	query :=
		`UPDATE "resource" SET
				"title" = $2
			WHERE id = $1
			RETURNING id`

	_, err := c.db.Exec(ctx, query,
		in.Id,
		in.Title,
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *resourceRepo) RemoveResource(ctx context.Context, in *pb.RemoveResourceRequest) (*pb.EmptyProto, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.RemoveResource")
	defer dbSpan.Finish()

	query := `
	DELETE FROM "resource"
	WHERE "id" = $1
	`
	sqlResult, err := c.db.Exec(ctx, query,
		in.Id,
	)
	if err != nil {
		return nil, err
	}

	res := sqlResult.RowsAffected()

	if res == 0 {
		return nil, err
	}

	return &pb.EmptyProto{}, nil
}

func (c *resourceRepo) GetResourceWithPath(ctx context.Context, in *pb.GetResourceRequest) (*pb.GetResourceWithPathResponse, error) {
	// does not implemented

	return nil, errors.New("err not implemented")
}

func (c *resourceRepo) GetDefaultResource(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetDefaultResource")
	defer dbSpan.Finish()

	var (
		res       pb.ResourceEnvironment
		createdAt sql.NullString
		updatedAt sql.NullString
	)

	stmt :=
		`SELECT
			id,
			project_id,
			resource_id,
			environment_id,
			is_configured,
			vault_path,
			resource_type,
			service_type,
			host,
			port::varchar(255),
			username,
			database,
			created_at,
			updated_at,
			"default"
		FROM resource_environment
		 `

	params := map[string]any{}
	filter := `WHERE "default"=true`
	order := ` ORDER BY "created_at"`
	arrangement := ` DESC`

	if util.IsValidUUID(in.ProjectId) {
		filter += ` AND project_id = :project_id`
		params["project_id"] = in.ProjectId
	}

	if util.IsValidUUID(in.EnvironmentId) {
		filter += ` AND environment_id = :environment_id`
		params["environment_id"] = in.EnvironmentId
	}

	if util.IsValidUUID(in.ResourceId) {
		filter += ` AND resource_id = :resource_id`
		params["resource_id"] = in.ResourceId
	}

	query, args := helper.ReplaceQueryParams(stmt+filter, params)

	err := c.db.QueryRow(ctx, query+order+arrangement, args...).Scan(
		&res.Id,
		&res.ProjectId,
		&res.ResourceId,
		&res.EnvironmentId,
		&res.IsConfigured,
		&res.VaultPath,
		&res.ResourceType,
		&res.ServiceType,
		&res.Host,
		&res.Port,
		&res.Username,
		&res.Database,
		&createdAt,
		&updatedAt,
		&res.Default,
	)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *resourceRepo) GetResourceManyWithPath(ctx context.Context, in *pb.GetResourceManyRequest) (*pb.GetResourceManyWithPathResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetResourceManyWithPath")
	defer dbSpan.Finish()

	var (
		res = pb.GetResourceManyWithPathResponse{
			Res: []*pb.GetResourceManyWithPathResponse_ResourceEnvironment{},
		}
		err       error
		createdAt sql.NullString
		updatedAt sql.NullString
		ids       []string
	)
	params := map[string]any{}
	for _, v := range in.Id {
		ids = append(ids, v.Id)
	}

	stmt :=
		`SELECT
			re.id,
			re.project_id,
			re.vault_path,
			service_type,
			re.resource_type,
			re.host,
			re.port::varchar(255),
			re.username,
			re."database",
			re.created_at,
			coalesce(re.updated_at, now()),
			r.title,
			resource_id,
			environment_id,
			re.node_type
		FROM resource_environment re
		INNER JOIN resource r on re.resource_id = r.id
		WHERE re.project_id = any(:ids::uuid[]) AND re.is_configured = true`

	params["ids"] = pq.Array(ids)

	if in.NodeType != "" {
		stmt += ` AND re.node_type = :node_type`
		params["node_type"] = in.NodeType
	}

	query, args := helper.ReplaceQueryParams(stmt, params)

	rows, err := c.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var (
			resp = pb.GetResourceManyWithPathResponse_ResourceEnvironment{
				Credentials: &pb.GetResourceManyWithPathResponse_ResourceEnvironment_Credentials{},
			}
			credentials = pb.GetResourceManyWithPathResponse_ResourceEnvironment_Credentials{}
		)
		err = rows.Scan(
			&resp.Id,
			&resp.ProjectId,
			&resp.Path,
			&resp.ServiceType,
			&resp.ResourceType,
			&credentials.Host,
			&credentials.Port,
			&credentials.Username,
			&credentials.Database,
			&createdAt,
			&updatedAt,
			&resp.Title,
			&resp.ResourceId,
			&resp.EnvironmentId,
			&resp.NodeType,
		)
		if err != nil {
			return nil, err
		}
		resp.Credentials = &credentials
		res.Res = append(res.Res, &resp)
	}

	return &res, nil
}

func (c *resourceRepo) GetListResourceEnvironment(ctx context.Context, in *pb.GetListResourceEnvironmentReq) ([]*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetListResourceEnvironment")
	defer dbSpan.Finish()

	var (
		resp      []*pb.ResourceEnvironment
		createdAt sql.NullString
		updatedAt sql.NullString

		stmt = `SELECT
			id,
			project_id,
			resource_id,
			environment_id,
			is_configured,
			vault_path,
			resource_type,
			service_type,
			host,
			port::varchar(255),
			username,
			database,
			created_at,
			updated_at,
			"default",
			node_type
		FROM resource_environment
		 `

		params      = map[string]any{}
		filter      = `WHERE true=true`
		order       = ` ORDER BY "created_at"`
		arrangement = ` DESC`
	)

	if util.IsValidUUID(in.ProjectId) {
		filter += ` AND project_id = :project_id`
		params["project_id"] = in.ProjectId
	}

	if util.IsValidUUID(in.EnvironmentId) {
		filter += ` AND environment_id = :environment_id`
		params["environment_id"] = in.EnvironmentId
	}

	if util.IsValidUUID(in.ResourceId) {
		filter += ` AND resource_id = :resource_id`
		params["resource_id"] = in.ResourceId
	}

	if in.ResourceType > 0 {
		filter += ` AND resource_type = :resource_type`
		params["resource_type"] = in.ResourceType
	}

	query, args := helper.ReplaceQueryParams(stmt+filter, params)
	query = query + order + arrangement

	if in.Limit > 0 {
		query += fmt.Sprintf(` LIMIT %d OFFSET %d`, in.Limit, in.Offset)
	}

	rows, err := c.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		row := &pb.ResourceEnvironment{}
		err = rows.Scan(
			&row.Id,
			&row.ProjectId,
			&row.ResourceId,
			&row.EnvironmentId,
			&row.IsConfigured,
			&row.VaultPath,
			&row.ResourceType,
			&row.ServiceType,
			&row.Host,
			&row.Port,
			&row.Username,
			&row.Database,
			&createdAt,
			&updatedAt,
			&row.Default,
			&row.NodeType,
		)
		if err != nil {
			return nil, err
		}

		resp = append(resp, row)
	}

	return resp, nil
}

func (c *resourceRepo) GetSingleResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetSingleResourceEnvironment")
	defer dbSpan.Finish()

	var (
		row       *pb.ResourceEnvironment = &pb.ResourceEnvironment{}
		createdAt sql.NullString
		updatedAt sql.NullString
	)

	stmt :=
		`SELECT
			id,
			project_id,
			resource_id,
			environment_id,
			is_configured,
			vault_path,
			resource_type,
			service_type,
			host,
			port::varchar(255),
			username,
			database,
			created_at,
			updated_at,
			"default",
			node_type
		FROM resource_environment
		 `

	params := map[string]any{}
	filter := `WHERE `
	order := ` ORDER BY "created_at"`
	arrangement := ` DESC`

	if in.IsConfigured {
		filter += ` "default" = false`
	} else {
		filter += ` "default" = true`
	}

	if util.IsValidUUID(in.ProjectId) {
		filter += ` AND project_id = :project_id`
		params["project_id"] = in.ProjectId
	}

	if util.IsValidUUID(in.EnvironmentId) {
		filter += ` AND environment_id = :environment_id`
		params["environment_id"] = in.EnvironmentId
	}

	if util.IsValidUUID(in.ResourceId) {
		filter += ` AND resource_id = :resource_id`
		params["resource_id"] = in.ResourceId
	}

	if util.IsValidUUID(in.Id) {
		filter += ` AND id = :id`
		params["id"] = in.Id
	}

	if in.NodeType != "" {
		filter += ` AND node_type = :node_type`
		params["node_type"] = in.NodeType
	}

	if in.Username != "" {
		filter += ` AND username = :username`
		params["username"] = in.Username
	}

	if in.ResourceType != 0 {
		filter += ` AND resource_type = :resource_type`
		params["resource_type"] = in.ResourceType
	}

	query, args := helper.ReplaceQueryParams(stmt+filter, params)

	err := c.db.QueryRow(ctx, query+order+arrangement, args...).Scan(
		&row.Id,
		&row.ProjectId,
		&row.ResourceId,
		&row.EnvironmentId,
		&row.IsConfigured,
		&row.VaultPath,
		&row.ResourceType,
		&row.ServiceType,
		&row.Host,
		&row.Port,
		&row.Username,
		&row.Database,
		&createdAt,
		&updatedAt,
		&row.Default,
		&row.NodeType,
	)

	if err != nil {
		return nil, err
	}

	return row, nil
}

func (c *resourceRepo) InsertResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.InsertResourceEnvironment")
	defer dbSpan.Finish()

	resp := &pb.ResourceEnvironment{}
	params := make(map[string]any)
	respId := ""

	uuid, err := uuid.NewRandom()
	if err != nil {
		return nil, err
	}
	stmt :=
		`INSERT INTO "resource_environment" (
			id,
			project_id,
			resource_id,
			environment_id,
			is_configured,
			vault_path,
			resource_type,
			service_type,
			host,
			port,
			username,
			database,
			created_at,
			updated_at,
            "default",
			node_type
		) VALUES (
			:id,
			:project_id,
			:resource_id,
			:environment_id,
			:is_configured,
			:vault_path,
			:resource_type,
			:service_type,
			:host,
			:port,
			:username,
			:database,
			:created_at,
			:updated_at,
		    :default,
			:node_type
		) RETURNING 
			id
		`
	port, err := strconv.Atoi(in.Port)
	if err != nil {
		return nil, err
	}
	params["id"] = uuid.String()
	params["project_id"] = in.ProjectId
	params["resource_id"] = in.ResourceId
	params["service_type"] = in.ServiceType
	params["environment_id"] = in.EnvironmentId
	params["is_configured"] = in.IsConfigured
	params["vault_path"] = in.VaultPath
	params["resource_type"] = in.ResourceType
	params["host"] = in.Host
	params["port"] = port
	params["username"] = in.Username
	params["database"] = in.Database
	params["created_at"] = time.Now().Format(config.DatabaseTimeLayout)
	params["updated_at"] = time.Now().Format(config.DatabaseTimeLayout)
	params["default"] = in.GetDefault()
	params["node_type"] = in.GetNodeType()
	query, args := helper.ReplaceQueryParams(stmt, params)
	err = c.db.QueryRow(
		ctx,
		query,
		args...,
	).Scan(
		&respId,
	)

	if err != nil {
		return nil, err
	}
	resp.Id = respId
	return resp, nil
}

func (c *resourceRepo) UpdateResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.UpdateResourceEnvironment")
	defer dbSpan.Finish()

	resp := &pb.ResourceEnvironment{}
	params := make(map[string]any)
	stmt :=
		`UPDATE "resource_environment" SET
			project_id = :project_id,
			resource_id = :resource_id,
			environment_id = :environment_id,
			is_configured = :is_configured,
			vault_path = :vault_path,
			resource_type = :resource_type,
			host = :host,
			port = :port,
			username = :username,
			database = :database,
			updated_at = :updated_at,
			"default" = :default,
			node_type = :node_type
		WHERE id = :id
		 RETURNING 
			id
		`

	params["id"] = in.Id
	params["project_id"] = in.ProjectId
	params["resource_id"] = in.ResourceId
	params["environment_id"] = in.EnvironmentId
	params["is_configured"] = in.IsConfigured
	params["vault_path"] = in.VaultPath
	params["resource_type"] = in.ResourceType
	params["host"] = in.Host
	params["port"] = in.Port
	params["username"] = in.Username
	params["database"] = in.Database
	params["updated_at"] = time.Now().Format(config.DatabaseTimeLayout)
	params["default"] = in.GetDefault()
	params["node_type"] = in.GetNodeType()

	query, args := helper.ReplaceQueryParams(stmt, params)

	err := c.db.QueryRow(
		ctx,
		query,
		args...,
	).Scan(
		&resp.Id,
	)

	if err != nil {
		return nil, err
	}

	return resp, nil
}

func (c *resourceRepo) RemoveResourceEnvironmentByID(ctx context.Context, in *pb.ResourceEnvironment) (*pb.EmptyProto, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.RemoveResourceEnvironmentByID")
	defer dbSpan.Finish()

	query := `
		DELETE FROM resource_environment WHERE id=$1;`

	sqlResult, err := c.db.Exec(ctx, query, in.GetId())
	if err != nil {
		return nil, err
	}

	effectedRowsCount := sqlResult.RowsAffected()
	if effectedRowsCount == 0 {
		return nil, fmt.Errorf("0 rows effected")
	}

	return &pb.EmptyProto{}, nil
}

func (c *resourceRepo) RemoveResourceEnvironmentByResourceID(ctx context.Context, in *pb.ResourceEnvironment) (*pb.EmptyProto, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.RemoveResourceEnvironmentByResourceID")
	defer dbSpan.Finish()

	query := `
		DELETE FROM 
			resource_environment 
		WHERE resource_id=$1;`

	sqlResult, err := c.db.Exec(ctx, query, in.GetResourceId())
	if err != nil {
		return nil, err
	}

	res := sqlResult.RowsAffected()
	if res == 0 {
		return nil, err
	}

	return &pb.EmptyProto{}, nil
}

func (c *resourceRepo) GetListConfiguredResourceEnvironment(ctx context.Context, in *pb.GetListConfiguredResourceEnvironmentReq) (*pb.GetListConfiguredResourceEnvironmentRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetListConfiguredResourceEnvironment")
	defer dbSpan.Finish()

	var (
		resp *pb.GetListConfiguredResourceEnvironmentRes = &pb.GetListConfiguredResourceEnvironmentRes{
			Data: []*pb.GetListConfiguredResourceEnvironmentResResourceEnvironment{},
		}
	)

	stmt :=
		`SELECT
			r.id,
			r.project_id,
			r.resource_id,
			r.environment_id,
			r.is_configured,
			r.resource_type,
			r.service_type,
			e.name,
			e.display_color,
			e.description
		FROM resource_environment as r
		INNER JOIN environment as e on r.environment_id = e.id
		 `

	params := map[string]any{}
	filter := `WHERE is_configured = true`
	order := ` ORDER BY r.created_at`
	arrangement := ` DESC`

	if util.IsValidUUID(in.ProjectId) {
		filter += ` AND r.project_id = :project_id`
		params["project_id"] = in.ProjectId
	}

	if util.IsValidUUID(in.EnvironmentId) {
		filter += ` AND r.environment_id = :environment_id`
		params["environment_id"] = in.EnvironmentId
	}

	if util.IsValidUUID(in.ResourceId) {
		filter += ` AND r.resource_id = :resource_id`
		params["resource_id"] = in.ResourceId
	}

	query, args := helper.ReplaceQueryParams(stmt+filter, params)

	rows, err := c.db.Query(ctx, query+order+arrangement, args...)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		row := &pb.GetListConfiguredResourceEnvironmentResResourceEnvironment{}
		err = rows.Scan(
			&row.Id,
			&row.ProjectId,
			&row.ResourceId,
			&row.EnvironmentId,
			&row.IsConfigured,
			&row.ResourceType,
			&row.ServiceType,
			&row.Name,
			&row.DisplayColor,
			&row.Description,
		)
		if err != nil {
			return nil, err
		}

		resp.Data = append(resp.Data, row)
	}

	return resp, nil
}

func (c *resourceRepo) UpdateDefaultResourceEnvironment(ctx context.Context, in *pb.ResourceEnvironment) ([]*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.UpdateDefaultResourceEnvironment")
	defer dbSpan.Finish()

	var resp []*pb.ResourceEnvironment
	params := make(map[string]any)
	stmt :=
		`UPDATE "resource_environment" SET
			"default" = false
		WHERE environment_id = :environment_id AND project_id = :project_id AND resource_id != :resource_id
		 RETURNING 
			id
		`

	params["project_id"] = in.ProjectId
	params["resource_id"] = in.ResourceId
	params["environment_id"] = in.EnvironmentId

	query, args := helper.ReplaceQueryParams(stmt, params)

	rows, err := c.db.Query(
		ctx,
		query,
		args...,
	)

	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var id string
		err = rows.Scan(
			&id,
		)
		if err != nil {
			return nil, err
		}
		resp = append(resp, &pb.ResourceEnvironment{
			Id: id,
		})
	}

	return resp, nil
}

func (c *resourceRepo) GetResourceByEnvID(ctx context.Context, req *pb.GetResourceByEnvIDRequest) (resp *pb.GetResourceByEnvIDResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetResourceByEnvID")
	defer dbSpan.Finish()

	var (
		count     int64
		projectID sql.NullString
		resEnv    pb.ResourceEnvironmentWithPassword
	)

	resp = &pb.GetResourceByEnvIDResponse{
		Resource: &pb.GetResourceByEnvIDResponse_Resourse{},
	}
	query := `
        SELECT 
            count(1) over(),
            resource_id,
            project_id
        FROM 
            resource_environment
        WHERE environment_id = $1 AND "default"=true
        GROUP BY resource_id, project_id`

	rows, err := c.db.Query(ctx, query, req.GetEnvId())
	if err != nil {
		return nil, errors.Wrap(err, "error while scanning")
	}

	resource := &pb.GetResourceByEnvIDResponse_Resourse{}
	for rows.Next() {
		err := rows.Scan(
			&count,
			&resource.Id,
			&projectID,
		)
		if err != nil {
			return nil, errors.Wrap(err, "error while scanning rows")
		} else if count > 1 {
			return nil, fmt.Errorf("too many resources for %s this environment", req.GetEnvId())
		}
	}
	resp.Resource = resource

	if projectID.String != "" {
		status := ""
		stmt :=
			`SELECT
                id,
                project_id,
                resource_id,
                environment_id,
                resource_type,
                service_type,
                node_type,
				(SELECT status FROM project WHERE id = $1) as status
            FROM resource_environment
            WHERE "default" = true AND project_id = $1
            AND environment_id = $2 
            AND resource_id = $3
            ORDER BY created_at DESC
         `
		err = c.db.QueryRow(ctx, stmt, projectID.String, req.GetEnvId(), resource.Id).Scan(
			&resEnv.Id,
			&resEnv.ProjectId,
			&resEnv.ResourceId,
			&resEnv.EnvironmentId,
			&resEnv.ResourceType,
			&resEnv.ServiceType,
			&resEnv.NodeType,
			&status,
		)
		if err != nil {
			return nil, errors.Wrap(err, "error while scanning rows")
		}

		resEnv.ResourceEnvironmentId = resEnv.Id
		resp.ResourceEnvironment = &resEnv
		resp.ProjectStatus = status
	}

	return resp, nil
}

func (c *resourceRepo) GetResEnvByResIdEnvId(ctx context.Context, in *pb.GetResEnvByResIdEnvIdRequest) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetResEnvByResIdEnvId")
	defer dbSpan.Finish()

	var (
		row       *pb.ResourceEnvironment = &pb.ResourceEnvironment{}
		createdAt sql.NullString
		updatedAt sql.NullString
	)

	stmt :=
		`SELECT
			id,
			project_id,
			resource_id,
			environment_id,
			is_configured,
			vault_path,
			resource_type,
			service_type,
			host,
			port::varchar(255),
			username,
			database,
			created_at,
			updated_at,
			"default"
		FROM resource_environment
		 `

	params := map[string]any{}
	filter := `WHERE true=true AND "default"=true`

	filter += ` AND environment_id = :environment_id`
	params["environment_id"] = in.EnvironmentId

	filter += ` AND resource_id = :resource_id`
	params["resource_id"] = in.ResourceId

	query, args := helper.ReplaceQueryParams(stmt+filter, params)

	err := c.db.QueryRow(ctx, query, args...).Scan(
		&row.Id,
		&row.ProjectId,
		&row.ResourceId,
		&row.EnvironmentId,
		&row.IsConfigured,
		&row.VaultPath,
		&row.ResourceType,
		&row.ServiceType,
		&row.Host,
		&row.Port,
		&row.Username,
		&row.Database,
		&createdAt,
		&updatedAt,
		&row.Default,
	)

	if err != nil {
		return nil, err
	}

	return row, nil
}

func (c *resourceRepo) GetServiceResources(ctx context.Context, in *pb.GetServiceResourcesReq) (map[pb.ServiceType][]*pb.GetServiceResourcesRes_ServiceTypeResources_ServiceResources, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetServiceResources")
	defer dbSpan.Finish()

	var res = make(map[pb.ServiceType][]*pb.GetServiceResourcesRes_ServiceTypeResources_ServiceResources)

	query := `
		SELECT
		    service_type,
		    resource_type,
		    is_configured,
		    "default",
		    resource_id,
		    title
		FROM resource_environment re
		LEFT JOIN resource r on re.resource_id = r.id
		WHERE re.project_id = $1 AND re.environment_id = $2`

	rows, err := c.db.Query(
		ctx,
		query,
		in.GetProjectId(),
		in.GetEnvironmentId(),
	)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		var (
			row     pb.GetServiceResourcesRes_ServiceTypeResources_ServiceResources
			service pb.ServiceType
		)

		err = rows.Scan(
			&service,
			&row.ResourceType,
			&row.IsConfigured,
			&row.Default,
			&row.ResourceId,
			&row.Title,
		)
		if err != nil {
			return nil, err
		}

		res[service] = append(res[service], &row)
	}

	return res, nil
}

func (c *resourceRepo) SetDefaultResource(ctx context.Context, in *pb.SetDefaultResourceReq) (*pb.SetDefaultResourceRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.SetDefaultResource")
	defer dbSpan.Finish()

	var res pb.SetDefaultResourceRes

	query := `
		UPDATE resource_environment
		SET "default" = true
		WHERE project_id = $1 AND environment_id = $2 AND resource_id = $3 AND service_type = $4
		RETURNING project_id, environment_id, resource_id, service_type`

	err := c.db.QueryRow(
		ctx,
		query,
		in.GetProjectId(),
		in.GetEnvironmentId(),
		in.GetResourceId(),
		in.GetServiceType(),
	).Scan(&res.ProjectId, &res.EnvironmentId, &res.ResourceId, &res.ServiceType)
	if err != nil {
		return nil, err
	}

	return &res, nil
}

func (c *resourceRepo) RemoveDefaultResource(ctx context.Context, in *pb.SetDefaultResourceReq) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.RemoveDefaultResource")
	defer dbSpan.Finish()

	query := `
		UPDATE resource_environment
		SET "default" = false
		WHERE project_id = $1 AND environment_id = $2 AND resource_id != $3 AND service_type = $4`

	_, err := c.db.Exec(
		ctx,
		query,
		in.GetProjectId(),
		in.GetEnvironmentId(),
		in.GetResourceId(),
		in.GetServiceType(),
	)
	if err != nil {
		return err
	}
	return nil
}

func (c *resourceRepo) GetResEnvByResIdEnvIdNotDefault(ctx context.Context, in *pb.GetResEnvByResIdEnvIdRequest) (*pb.ResourceEnvironment, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetResEnvByResIdEnvIdNotDefault")
	defer dbSpan.Finish()

	var (
		row       *pb.ResourceEnvironment = &pb.ResourceEnvironment{}
		createdAt sql.NullString
		updatedAt sql.NullString
	)

	stmt :=
		`SELECT
			id,
			project_id,
			resource_id,
			environment_id,
			is_configured,
			vault_path,
			resource_type,
			service_type,
			host,
			port::varchar(255),
			username,
			database,
			created_at,
			updated_at,
			"default"
		FROM resource_environment
		 `

	params := map[string]any{}
	filter := `WHERE true=true`

	filter += ` AND environment_id = :environment_id`
	params["environment_id"] = in.EnvironmentId

	filter += ` AND resource_id = :resource_id`
	params["resource_id"] = in.ResourceId

	query, args := helper.ReplaceQueryParams(stmt+filter, params)

	err := c.db.QueryRow(ctx, query, args...).Scan(
		&row.Id,
		&row.ProjectId,
		&row.ResourceId,
		&row.EnvironmentId,
		&row.IsConfigured,
		&row.VaultPath,
		&row.ResourceType,
		&row.ServiceType,
		&row.Host,
		&row.Port,
		&row.Username,
		&row.Database,
		&createdAt,
		&updatedAt,
		&row.Default,
	)

	if err != nil {
		return nil, err
	}

	return row, nil
}

// variable resource
func (c *resourceRepo) CreateVariableResource(ctx context.Context, in *pb.CreateVariableResourceRequest) (*pb.VariableResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.CreateVariableResource")
	defer dbSpan.Finish()

	var (
		result = &pb.VariableResource{}
		err    error
		id     = uuid.New()
	)

	query := `
	INSERT INTO variable_resource
	(
		id,
		project_id,
		environment_id,
		key,
		value,
		project_resource_id
	) VALUES ($1, $2, $3, $4, $5, $6)
	RETURNING id, project_id, environment_id, key, value, project_resource_id`

	err = c.db.QueryRow(
		ctx,
		query,
		id,
		in.ProjectId,
		in.EnvironmentId,
		in.Key,
		in.Value,
		in.ProjectResourceId,
	).Scan(&result.Id, &result.ProjectId, &result.EnvironmentId, &result.Key, &result.Value, &result.ProjectResourceId)

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *resourceRepo) GetVariableResourceList(ctx context.Context, in *pb.GetVariableResourceListRequest) (*pb.GetVariableResourceListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetVariableResourceList")
	defer dbSpan.Finish()

	var result = &pb.GetVariableResourceListResponse{}

	query :=
		`SELECT 
		id,
		project_id,
		environment_id,
		key,
		project_resource_id
	FROM variable_resource
	WHERE project_id = $1 AND environment_id = $2 AND project_resource_id = $3`

	row, err := c.db.Query(ctx, query, in.ProjectId, in.EnvironmentId, in.ProjectResourceId)

	for row.Next() {
		var variableResponse = &pb.VariableResource{}
		err := row.Scan(
			&variableResponse.Id,
			&variableResponse.ProjectId,
			&variableResponse.EnvironmentId,
			&variableResponse.Key,
			&variableResponse.ProjectResourceId,
		)
		if err != nil {
			return nil, err
		}
		result.Variables = append(result.Variables, variableResponse)
	}

	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *resourceRepo) GetSingleVariableResource(ctx context.Context, in *pb.PrimaryKeyVariableResource) (*pb.VariableResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetSingleVariableResource")
	defer dbSpan.Finish()

	var (
		result = &pb.VariableResource{}
		err    error
	)

	query :=
		`SELECT
		id,
		project_id,
		environment_id,
		key,
		value,
		project_resource_id
	FROM variable_resource
	WHERE project_id = $1 AND environment_id = $2
	AND 
	`
	filter := ""
	if in.Id != "" {
		query += `id = $3`
		filter = in.Id
	} else if in.Key != "" {
		query += `key = $3`
		filter = in.Key
	}

	err = c.db.QueryRow(ctx, query, in.ProjectId, in.EnvironmentId, filter).Scan(
		&result.Id,
		&result.ProjectId,
		&result.EnvironmentId,
		&result.Key,
		&result.Value,
		&result.ProjectResourceId,
	)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (c *resourceRepo) UpdateVariableResource(ctx context.Context, in *pb.VariableResource) (*pb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.UpdateVariableResource")
	defer dbSpan.Finish()

	var (
		err   error
		empty = &pb.Empty{}
	)
	params := make(map[string]any)
	query := `
		UPDATE variable_resource
		SET 
		  key = :key,
		  value = :value
		WHERE id = :id and project_id = :project_id and environment_id = :environment_id and project_resource_id = :project_resource_id
	`

	params["key"] = in.GetKey()
	params["value"] = in.GetValue()
	params["project_id"] = in.GetProjectId()
	params["environment_id"] = in.GetEnvironmentId()
	params["project_resource_id"] = in.GetProjectResourceId()
	params["id"] = in.GetId()

	cQuery, arr := helper.ReplaceQueryParams(query, params)

	_, err = c.db.Exec(ctx, cQuery, arr...)

	if err != nil {
		return nil, err
	}

	return empty, nil
}

func (c *resourceRepo) DeleteVariableResource(ctx context.Context, in *pb.PrimaryKeyVariableResource) (*pb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.DeleteVariableResource")
	defer dbSpan.Finish()

	var (
		empty = &pb.Empty{}
		err   error
		Id    string
	)
	query := `
		DELETE FROM variable_resource
		WHERE id = $1
		AND project_id = $2
		AND environment_id = $3
		RETURNING id
	`
	err = c.db.QueryRow(
		ctx,
		query,
		in.Id,
		in.ProjectId,
		in.EnvironmentId,
	).Scan(&Id)

	if err != nil {
		return nil, err
	}

	return empty, nil
}

func (c *resourceRepo) AddResourceToProject(ctx context.Context, in *pb.AddResourceToProjectRequest) (*pb.ProjectResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.AddResourceToProject")
	defer dbSpan.Finish()

	var (
		result = &pb.ProjectResource{}
		err    error
		id     = uuid.New()
	)

	query := `
	INSERT INTO project_resource
	(
		id,
		project_id,
		environment_id,
		name,
		type,
		settings,
		secret,
		external_id
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	RETURNING id, project_id, environment_id, name, type, settings::text, secret::text, external_id`

	var settings, secret sql.NullString

	err = c.db.QueryRow(
		ctx,
		query,
		id,
		in.ProjectId,
		in.EnvironmentId,
		in.Name,
		pb.ResourceType_name[int32(in.GetType())],
		in.Settings,
		in.Secret,
		in.GetExternalId(),
	).Scan(&result.Id, &result.ProjectId, &result.EnvironmentId, &result.Name, &result.Type, &settings, &secret, &result.ExternalId)

	if err != nil {
		return nil, err
	}
	if err = scanProjectResourceSettings(settings, result); err != nil {
		return nil, err
	}
	if err = scanProjectResourceSecret(secret, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *resourceRepo) UpsertProjectResource(ctx context.Context, in *pb.AddResourceToProjectRequest) (*pb.ProjectResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.UpsertProjectResource")
	defer dbSpan.Finish()

	var (
		result           = &pb.ProjectResource{}
		settings, secret sql.NullString
	)

	query := `
	INSERT INTO project_resource
	(
		id,
		project_id,
		environment_id,
		name,
		type,
		settings,
		secret,
		external_id
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	ON CONFLICT (external_id, project_id, environment_id) WHERE external_id <> ''
	DO UPDATE SET
		name = EXCLUDED.name,
		settings = EXCLUDED.settings,
		secret = COALESCE(EXCLUDED.secret, project_resource.secret)
	RETURNING id, project_id, environment_id, name, type, settings::text, secret::text, external_id`

	err := c.db.QueryRow(
		ctx,
		query,
		uuid.New(),
		in.ProjectId,
		in.EnvironmentId,
		in.Name,
		pb.ResourceType_name[int32(in.GetType())],
		in.Settings,
		in.Secret,
		in.GetExternalId(),
	).Scan(&result.Id, &result.ProjectId, &result.EnvironmentId, &result.Name, &result.Type, &settings, &secret, &result.ExternalId)

	if err != nil {
		return nil, err
	}
	if err = scanProjectResourceSettings(settings, result); err != nil {
		return nil, err
	}
	if err = scanProjectResourceSecret(secret, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *resourceRepo) GetProjectResourceList(ctx context.Context, in *pb.GetProjectResourceListRequest) (*pb.ListProjectResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetProjectResourceList")
	defer dbSpan.Finish()

	var (
		result    = &pb.ListProjectResource{}
		queryArgs []any
	)

	query := `WITH all_resources AS (
		SELECT
			id,
			project_id,
			environment_id,
			name,
			type,
			settings::text AS settings,
			secret::text AS secret,
			external_id
		FROM project_resource
		WHERE project_id = $1 AND environment_id = $2

		UNION ALL

		SELECT
			r.id,
			r.project_id,
			re.environment_id,
			r.title AS name,
			r.resource_type AS type,
			NULL::text AS settings,
			NULL::text AS secret,
			'' AS external_id
		FROM resource r
		JOIN resource_environment re ON re.resource_id = r.id
		WHERE r.project_id = $1 AND re.environment_id = $2
	)
	SELECT id, project_id, environment_id, name, type::text, settings, secret, external_id
	FROM all_resources`

	queryArgs = append(queryArgs, in.ProjectId, in.EnvironmentId)

	argIndex := 3 // Start for additional parameters

	if in.GetType() == pb.ResourceType_GIT {
		query += fmt.Sprintf(" WHERE type = ANY($%d)", argIndex)
		queryArgs = append(queryArgs, []string{"GITLAB", "GITHUB", "BITBUCKET"})
		argIndex++
	} else if in.GetType() != 0 {
		query += fmt.Sprintf(" WHERE type = $%d", argIndex)
		queryArgs = append(queryArgs, in.GetType().String())
		argIndex++
	}

	rows, err := c.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			resource = &pb.ProjectResource{}
			settings sql.NullString
			secret   sql.NullString
		)
		err := rows.Scan(
			&resource.Id,
			&resource.ProjectId,
			&resource.EnvironmentId,
			&resource.Name,
			&resource.Type,
			&settings,
			&secret,
			&resource.ExternalId,
		)
		if err != nil {
			return nil, err
		}

		if err = scanProjectResourceSettings(settings, resource); err != nil {
			return nil, err
		}
		if err = scanProjectResourceSecret(secret, resource); err != nil {
			return nil, err
		}
		result.Resources = append(result.Resources, resource)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *resourceRepo) GetSingleProjectResouece(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.ProjectResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetSingleProjectResouece")
	defer dbSpan.Finish()

	var (
		result    = &pb.ProjectResource{}
		err       error
		variables []byte
		settings  sql.NullString
		secret    sql.NullString
	)

	var variables1 []*pb.VariableResource

	query :=
		`SELECT
		pr.id AS pr_id,
		pr.project_id,
		pr.environment_id,
		pr.name,
		pr.type,
		settings,
		secret,
		pr.external_id,
		coalesce(
			JSON_AGG(
				JSON_BUILD_OBJECT(
					'id', vr.id,
					'project_id', vr.project_id,
					'environment_id', vr.environment_id,
					'key', vr.key,
					'value', vr.value)), '[]'::json) AS variable_data
	FROM project_resource AS pr
	LEFT JOIN variable_resource AS vr ON pr.id = vr.project_resource_id
	WHERE pr.project_id = $1 AND pr.environment_id = $2
	AND pr.id = $3
	GROUP BY pr.id, pr.project_id, pr.environment_id, pr.name, pr.type, pr.external_id
	`

	err = c.db.QueryRow(ctx, query, in.ProjectId, in.EnvironmentId, in.Id).Scan(
		&result.Id,
		&result.ProjectId,
		&result.EnvironmentId,
		&result.Name,
		&result.Type,
		&settings,
		&secret,
		&result.ExternalId,
		&variables,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return c.getSystemProjectResource(ctx, in)
		}
		return nil, err
	}

	result.ResourceType = pb.ResourceType_value[result.Type]

	if err = scanProjectResourceSettings(settings, result); err != nil {
		return nil, err
	}
	if err = scanProjectResourceSecret(secret, result); err != nil {
		return nil, err
	}

	err = json.Unmarshal(variables, &variables1)
	if err != nil {
		return nil, err
	}

	result.Variables = variables1

	return result, nil
}

func scanProjectResourceSettings(raw sql.NullString, resource *pb.ProjectResource) error {
	if resource == nil || !raw.Valid || raw.String == "" || raw.String == "null" {
		return nil
	}
	return json.Unmarshal([]byte(raw.String), &resource.Settings)
}

func scanProjectResourceSecret(raw sql.NullString, resource *pb.ProjectResource) error {
	if resource == nil || !raw.Valid || raw.String == "" || raw.String == "null" {
		return nil
	}
	var secret structpb.Struct
	if err := json.Unmarshal([]byte(raw.String), &secret); err != nil {
		return err
	}
	resource.Secret = &secret
	return nil
}

func (c *resourceRepo) getSystemProjectResource(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.ProjectResource, error) {
	query := `
		SELECT
			r.id,
			r.project_id,
			re.environment_id,
			r.title,
			r.resource_type::text,
			r.host,
			r.port::text,
			r.username,
			r.database
		FROM resource r
		JOIN resource_environment re ON re.resource_id = r.id
		WHERE r.id = $1 AND r.project_id = $2 AND re.environment_id = $3
	`

	result := &pb.ProjectResource{
		Settings: &pb.Settings{
			Postgres: &pb.PostgresCredentials{},
		},
	}

	err := c.db.QueryRow(ctx, query, in.Id, in.ProjectId, in.EnvironmentId).Scan(
		&result.Id,
		&result.ProjectId,
		&result.EnvironmentId,
		&result.Name,
		&result.Type,
		&result.Settings.Postgres.Host,
		&result.Settings.Postgres.Port,
		&result.Settings.Postgres.Username,
		&result.Settings.Postgres.Database,
	)
	if err != nil {
		return nil, err
	}

	result.ResourceType = pb.ResourceType_value[result.Type]
	return result, nil
}

func (c *resourceRepo) UpdateProjectResource(ctx context.Context, in *pb.ProjectResource) (*pb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.UpdateProjectResource")
	defer dbSpan.Finish()

	var (
		err   error
		empty = &pb.Empty{}
	)
	params := make(map[string]any)
	query := `
		UPDATE project_resource
		SET
		  name = :name,
		  settings = :settings,
		  external_id = COALESCE(NULLIF(:external_id, ''), external_id)
		WHERE id = :id and project_id = :project_id and environment_id = :environment_id
	`

	params["name"] = in.GetName()
	params["settings"] = in.GetSettings()
	params["project_id"] = in.GetProjectId()
	params["environment_id"] = in.GetEnvironmentId()
	params["id"] = in.GetId()
	params["external_id"] = in.GetExternalId()

	cQuery, arr := helper.ReplaceQueryParams(query, params)

	_, err = c.db.Exec(ctx, cQuery, arr...)

	if err != nil {
		return nil, err
	}

	return empty, nil
}

func (c *resourceRepo) DeleteProjectResource(ctx context.Context, in *pb.PrimaryKeyProjectResource) (*pb.Empty, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.DeleteProjectResource")
	defer dbSpan.Finish()

	var (
		empty = &pb.Empty{}
		err   error
		Id    string
	)

	query := `
		DELETE FROM project_resource
		WHERE id = $1
		AND project_id = $2
		AND environment_id = $3
		RETURNING id
	`

	err = c.db.QueryRow(
		ctx,
		query,
		in.Id,
		in.ProjectId,
		in.EnvironmentId,
	).Scan(&Id)

	if err != nil {
		return nil, err
	}

	return empty, nil
}

func (c *resourceRepo) GetProjectResourcesByExternalId(ctx context.Context, in *pb.GetByExternalIdRequest) (*pb.ListProjectResource, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "resource.GetProjectResourcesByExternalId")
	defer dbSpan.Finish()

	var (
		result    = &pb.ListProjectResource{}
		queryArgs = []any{in.GetExternalId()}
	)

	query := `
		SELECT id, project_id, environment_id, name, type::text, settings::text, secret::text, external_id
		FROM project_resource
		WHERE external_id = $1`

	if in.GetType() != pb.ResourceType_NOT_DECIDED {
		query += ` AND type = $2`
		queryArgs = append(queryArgs, in.GetType().String())
	}

	rows, err := c.db.Query(ctx, query, queryArgs...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			resource = &pb.ProjectResource{}
			settings sql.NullString
			secret   sql.NullString
		)
		err := rows.Scan(
			&resource.Id,
			&resource.ProjectId,
			&resource.EnvironmentId,
			&resource.Name,
			&resource.Type,
			&settings,
			&secret,
			&resource.ExternalId,
		)
		if err != nil {
			return nil, err
		}

		if err = scanProjectResourceSettings(settings, resource); err != nil {
			return nil, err
		}
		if err = scanProjectResourceSecret(secret, resource); err != nil {
			return nil, err
		}

		resource.ResourceType = pb.ResourceType_value[resource.Type]
		result.Resources = append(result.Resources, resource)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return result, nil
}

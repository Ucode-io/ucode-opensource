package postgres

import (
	"context"
	"database/sql"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
)

type serviceResource struct {
	db *Pool
}

// NewServiceResource ...
func NewServiceResource(db *Pool) repo.ServiceResourceStorageI {
	return &serviceResource{
		db: db,
	}
}

func (s *serviceResource) GetSingle(ctx context.Context, req *pb.GetSingleServiceResourceReq) (*pb.ServiceResourceModel, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "service_envoirment.GetSingle")
	defer dbSpan.Finish()

	var (
		filter       string
		res          pb.ServiceResourceModel
		serviceType  string
		resourceType int32
	)
	params := make(map[string]any)
	queryInitial := `
			SELECT
				s.id,
				s.service_type,
				s.project_id,
				s.title,
				description,
				s.resource_id,
				s.environment_id,
				coalesce(r.id::VARCHAR, ''),
				resource_type,
				coalesce(r.node_type::VARCHAR, ''),
				p.status
			FROM "service_resource" s
			INNER JOIN resource_environment r ON s.resource_id = r.resource_id AND s.environment_id = r.environment_id
			INNER JOIN project p ON r.project_id = p.id
			WHERE s.project_id = :project_id`

	params["project_id"] = req.GetProjectId()

	if _, ok := pb.ServiceType_name[int32(req.GetServiceType())]; ok {
		params["service_type"] = req.GetServiceType().String()
		filter += ` AND s.service_type = :service_type`
	}

	if util.IsValidUUID(req.GetId()) {
		params["id"] = req.GetId()
		filter += ` AND s.id = :id`
	}

	if util.IsValidUUID(req.GetEnvironmentId()) {
		params["environment_id"] = req.GetEnvironmentId()
		filter += ` AND s.environment_id = :environment_id`
	}

	query := queryInitial + filter

	rQuery, arr := helper.ReplaceQueryParams(query, params)

	err := s.db.QueryRow(ctx, rQuery, arr...).Scan(
		&res.Id,
		&serviceType,
		&res.ProjectId,
		&res.Title,
		&res.Description,
		&res.ResourceId,
		&res.EnvironmentId,
		&res.ResourceEnvironmentId,
		&resourceType,
		&res.NodeType,
		&res.ProjectStatus,
	)

	if err != nil {
		return &res, err
	}
	res.ServiceType = pb.ServiceType(pb.ServiceType_value[serviceType])
	res.ResourceType = pb.ResourceType(resourceType)

	return &res, nil
}

func (s *serviceResource) Update(ctx context.Context, req *pb.UpdateServiceResourceReq) (*pb.UpdateServiceResourceRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "service_envoirment.Update")
	defer dbSpan.Finish()

	var rowsAffected int32

	qFind := `
			SELECT
				count(id)
			FROM "service_resource"
			WHERE id = $1`

	qUpdate := `
			UPDATE "service_resource"
			SET
			    title = $1,
			    description = $2,
			    resource_id = $3,
			    environment_id = $4
			WHERE id = $5`

	qInsert := `
			INSERT INTO 
			    "service_resource"(id, service_type, project_id, title, description, resource_id, environment_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7)`

	for _, v := range req.GetServiceResources() {
		var (
			count int
		)

		if int32(v.ServiceType) == int32(pb.ServiceType_NOT_SPECIFIED) {
			continue
		}

		_ = s.db.QueryRow(ctx, qFind,
			v.GetId(),
		).Scan(
			&count,
		)

		if count == 0 {
			id, err := uuid.NewRandom()
			if err != nil {
				continue
			}
			_, err = s.db.Exec(ctx, qInsert,
				id.String(),
				v.GetServiceType().String(),
				req.GetProjectId(),
				v.GetTitle(),
				v.GetDescription(),
				v.GetResourceId(),
				req.GetEnvironmentId(),
			)
			if err != nil {
				continue
			}
			rowsAffected++
		} else {
			_, err := s.db.Exec(ctx, qUpdate,
				v.GetTitle(),
				v.GetDescription(),
				v.GetResourceId(),
				req.GetEnvironmentId(),
				v.GetId(),
			)
			if err != nil {
				continue
			}
			rowsAffected++
		}
	}
	return &pb.UpdateServiceResourceRes{
		RowsAffected: rowsAffected,
	}, nil
}

func (s *serviceResource) GetList(ctx context.Context, req *pb.GetListServiceResourceReq) (*pb.GetListServiceResourceRes, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "service_envoirment.GetList")
	defer dbSpan.Finish()

	var res pb.GetListServiceResourceRes

	mServiceResource := make(map[string]*pb.ServiceResourceModel)

	for _, v := range pb.ServiceType_name {
		if v == pb.ServiceType_NOT_SPECIFIED.String() {
			continue
		}
		mServiceResource[v] = &pb.ServiceResourceModel{
			ServiceType:  pb.ServiceType(pb.ServiceType_value[v]),
			ResourceType: pb.ResourceType_NOT_DECIDED,
		}
	}

	params := make(map[string]any)

	queryInit := `
			SELECT
				s.id,
				s.service_type,
				s.project_id,
				s.title,
				s.description,
				s.resource_id,
				s.environment_id,
				coalesce(r.id::VARCHAR, ''),
				resource.resource_type,
				r.node_type
			FROM "service_resource" s
			LEFT JOIN resource_environment r on s.resource_id = r.resource_id AND s.environment_id = r.environment_id
			LEFT JOIN resource ON s.resource_id = resource.id`

	filter := ` WHERE s.project_id = :project_id`
	params["project_id"] = req.ProjectId

	if util.IsValidUUID(req.EnvironmentId) {
		filter += ` AND s.environment_id = :environment_id`
		params["environment_id"] = req.EnvironmentId
	}

	if util.IsValidUUID(req.ResourceId) {
		filter += ` AND s.resource_id = :resource_id`
		params["resource_id"] = req.ResourceId
	}

	query, arr := helper.ReplaceQueryParams(queryInit+filter, params)

	rows, err := s.db.Query(ctx, query, arr...)
	if err != nil {
		return &res, err
	}

	for rows.Next() {
		var (
			row          pb.ServiceResourceModel
			serviceType  string
			resourceType string
			nodeType     sql.NullString
		)

		err = rows.Scan(
			&row.Id,
			&serviceType,
			&row.ProjectId,
			&row.Title,
			&row.Description,
			&row.ResourceId,
			&row.EnvironmentId,
			&row.ResourceEnvironmentId,
			&resourceType,
			&nodeType,
		)
		if err != nil {
			return &res, err
		}

		row.ServiceType = pb.ServiceType(pb.ServiceType_value[serviceType])
		row.ResourceType = pb.ResourceType(pb.ResourceType_value[resourceType])
		row.NodeType = nodeType.String

		mServiceResource[row.ServiceType.String()] = &row
	}
	res.ServiceResources = mServiceResource

	return &res, nil
}

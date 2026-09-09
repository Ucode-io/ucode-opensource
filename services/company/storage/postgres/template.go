package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/structpb"
)

type templateMetadataRepo struct {
	db *Pool
}

func NewTemplateMetadataRepo(db *Pool) repo.TemplateStorageI {
	return &templateMetadataRepo{
		db: db,
	}
}

func (t *templateMetadataRepo) Create(ctx context.Context, template *pb.CreateTemplateMetadataReq) (*pb.TemplateMetadata, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "template_metadata.Create")
	defer dbSpan.Finish()

	var (
		err         error
		result      string
		templateId  = uuid.New()
		insertQuery = `INSERT INTO template(
			id,
			name,
			description,
			photo,
			tables,
			functions,
			microfronts
		) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	)

	// Convert protobuf Struct to JSONB
	tablesJSON, err := json.Marshal(template.Tables)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tables to JSON: %w", err)
	}

	functionsJSON, err := json.Marshal(template.Functions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert functions to JSON: %w", err)
	}

	microfrontsJSON, err := json.Marshal(template.Microfronts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert microfronts to JSON: %w", err)
	}

	err = t.db.QueryRow(
		ctx,
		insertQuery,
		templateId.String(),
		template.Name,
		template.Description,
		template.Photo,
		tablesJSON,
		functionsJSON,
		microfrontsJSON,
	).Scan(&result)

	if err != nil {
		return nil, fmt.Errorf("failed to create template: %w", err)
	}

	// Return the created template
	return &pb.TemplateMetadata{
		Id:          result,
		Name:        template.Name,
		Description: template.Description,
		Photo:       template.Photo,
		Tables:      template.Tables,
		Functions:   template.Functions,
		Microfronts: template.Microfronts,
		CreatedAt:   time.Now().Format(time.RFC3339),
		UpdatedAt:   time.Now().Format(time.RFC3339),
		DeletedAt:   0,
	}, nil
}

func (t *templateMetadataRepo) GetById(ctx context.Context, id string) (*pb.TemplateMetadata, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "template_metadata.GetById")
	defer dbSpan.Finish()

	var (
		err  error
		resp pb.TemplateMetadata
		stmt = `SELECT
					id,
					name,
					description,
					photo,
					tables,
					functions,
					microfronts,
					created_at,
					updated_at,
					deleted_at
				FROM template
				WHERE id = $1 AND deleted_at = 0`
	)

	var tablesJSON, functionsJSON, microfrontsJSON []byte
	var createdAt, updatedAt time.Time
	var deletedAt int64

	err = t.db.QueryRow(ctx, stmt, id).Scan(
		&resp.Id,
		&resp.Name,
		&resp.Description,
		&resp.Photo,
		&tablesJSON,
		&functionsJSON,
		&microfrontsJSON,
		&createdAt,
		&updatedAt,
		&deletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template not found: %s", id)
		}
		return nil, fmt.Errorf("failed to get template: %w", err)
	}

	// Convert JSONB to protobuf Struct
	resp.Tables, err = jsonToStruct(tablesJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tables JSON: %w", err)
	}

	resp.Functions, err = jsonToStruct(functionsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to convert functions JSON: %w", err)
	}

	resp.Microfronts, err = jsonToStruct(microfrontsJSON)
	if err != nil {
		return nil, fmt.Errorf("failed to convert microfronts JSON: %w", err)
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	resp.DeletedAt = deletedAt

	return &resp, nil
}

func (t *templateMetadataRepo) GetList(ctx context.Context, req *pb.GetTemplateMetadataListReq) (*pb.GetTemplateMetadataListResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "template_metadata.GetList")
	defer dbSpan.Finish()

	var (
		resp = &pb.GetTemplateMetadataListResponse{}
		stmt = `SELECT
					id,
					name,
					description,
					photo,
					tables,
					functions,
					microfronts,
					created_at,
					updated_at,
					deleted_at
				FROM template
				WHERE deleted_at = 0
				ORDER BY created_at DESC
				LIMIT $1 OFFSET $2`
		countStmt = `SELECT COUNT(*) FROM template WHERE deleted_at = 0`
	)

	// Get total count of active templates
	var count int32
	err := t.db.QueryRow(ctx, countStmt).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to get template count: %w", err)
	}
	resp.Count = count

	// Get active templates only
	rows, err := t.db.Query(ctx, stmt, req.Limit, req.Offset)
	if err != nil {
		return nil, fmt.Errorf("failed to get template list: %w", err)
	}
	defer rows.Close()

	var templates []*pb.TemplateMetadata
	for rows.Next() {
		var template pb.TemplateMetadata
		var tablesJSON, functionsJSON, microfrontsJSON []byte
		var createdAt, updatedAt time.Time
		var deletedAt int64

		err := rows.Scan(
			&template.Id,
			&template.Name,
			&template.Description,
			&template.Photo,
			&tablesJSON,
			&functionsJSON,
			&microfrontsJSON,
			&createdAt,
			&updatedAt,
			&deletedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan template: %w", err)
		}

		// Convert JSONB to protobuf Struct
		template.Tables, err = jsonToStruct(tablesJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to convert tables JSON: %w", err)
		}

		template.Functions, err = jsonToStruct(functionsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to convert functions JSON: %w", err)
		}

		template.Microfronts, err = jsonToStruct(microfrontsJSON)
		if err != nil {
			return nil, fmt.Errorf("failed to convert microfronts JSON: %w", err)
		}

		template.CreatedAt = createdAt.Format(time.RFC3339)
		template.UpdatedAt = updatedAt.Format(time.RFC3339)
		template.DeletedAt = deletedAt

		templates = append(templates, &template)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over template rows: %w", err)
	}

	resp.Templates = templates
	return resp, nil
}

func (t *templateMetadataRepo) Update(ctx context.Context, template *pb.UpdateTemplateMetadataReq) (*pb.TemplateMetadata, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "template_metadata.Update")
	defer dbSpan.Finish()

	var (
		err        error
		updateStmt = `UPDATE template SET
						name = $2,
						description = $3,
						photo = $4,
						tables = $5,
						functions = $6,
						microfronts = $7,
						updated_at = CURRENT_TIMESTAMP
					WHERE id = $1 AND deleted_at = 0
					RETURNING id, name, description, photo, tables, functions, microfronts, created_at, updated_at, deleted_at`
	)

	// Convert protobuf Struct to JSONB
	tablesJSON, err := structToJSON(template.Tables)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tables to JSON: %w", err)
	}

	functionsJSON, err := structToJSON(template.Functions)
	if err != nil {
		return nil, fmt.Errorf("failed to convert functions to JSON: %w", err)
	}

	microfrontsJSON, err := structToJSON(template.Microfronts)
	if err != nil {
		return nil, fmt.Errorf("failed to convert microfronts to JSON: %w", err)
	}

	var resp pb.TemplateMetadata
	var tablesJSONResult, functionsJSONResult, microfrontsJSONResult []byte
	var createdAt, updatedAt time.Time
	var deletedAt int64

	err = t.db.QueryRow(
		ctx,
		updateStmt,
		template.Id,
		template.Name,
		template.Description,
		template.Photo,
		tablesJSON,
		functionsJSON,
		microfrontsJSON,
	).Scan(
		&resp.Id,
		&resp.Name,
		&resp.Description,
		&resp.Photo,
		&tablesJSONResult,
		&functionsJSONResult,
		&microfrontsJSONResult,
		&createdAt,
		&updatedAt,
		&deletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("template not found or deleted: %s", template.Id)
		}
		return nil, fmt.Errorf("failed to update template: %w", err)
	}

	// Convert JSONB to protobuf Struct
	resp.Tables, err = jsonToStruct(tablesJSONResult)
	if err != nil {
		return nil, fmt.Errorf("failed to convert tables JSON: %w", err)
	}

	resp.Functions, err = jsonToStruct(functionsJSONResult)
	if err != nil {
		return nil, fmt.Errorf("failed to convert functions JSON: %w", err)
	}

	resp.Microfronts, err = jsonToStruct(microfrontsJSONResult)
	if err != nil {
		return nil, fmt.Errorf("failed to convert microfronts JSON: %w", err)
	}

	resp.CreatedAt = createdAt.Format(time.RFC3339)
	resp.UpdatedAt = updatedAt.Format(time.RFC3339)
	resp.DeletedAt = deletedAt

	return &resp, nil
}

func (t *templateMetadataRepo) Delete(ctx context.Context, id string) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "template_metadata.Delete")
	defer dbSpan.Finish()

	var (
		err        error
		deleteStmt = `UPDATE template SET 
						deleted_at = date_part('epoch', CURRENT_TIMESTAMP)::int, 
						updated_at = CURRENT_TIMESTAMP 
					WHERE id = $1 AND deleted_at = 0`
	)

	result, err := t.db.Exec(ctx, deleteStmt, id)
	if err != nil {
		return fmt.Errorf("failed to delete template: %w", err)
	}

	rowsAffected := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("template not found or already deleted: %s", id)
	}

	return nil
}

// Helper functions for JSON conversion
func structToJSON(s *structpb.Struct) ([]byte, error) {
	if s == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(s.AsMap())
}

func jsonToStruct(data []byte) (*structpb.Struct, error) {
	if len(data) == 0 {
		return &structpb.Struct{}, nil
	}

	var mapData map[string]any
	err := json.Unmarshal(data, &mapData)
	if err != nil {
		return nil, err
	}

	return structpb.NewStruct(mapData)
}

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/structpb"
)

type projectFoldersRepo struct {
	db *psqlpool.Pool
}

func NewProjectFoldersRepo(db *psqlpool.Pool) storage.ProjectFoldersRepoI {
	return &projectFoldersRepo{db: db}
}

// ==================== Create ====================

func (r *projectFoldersRepo) CreateProjectFolder(ctx context.Context, req *nb.CreateProjectFolderRequest) (*nb.ProjectFolder, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "projectFoldersRepo.CreateProjectFolder")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		id = uuid.NewString()

		folder                 nb.ProjectFolder
		parentId, mcpProjectId sql.NullString
		chatId                 sql.NullString
		createdAt, updatedAt   time.Time

		attrBytes = []byte("{}")
	)

	if req.GetAttributes() != nil {
		attrBytes, err = json.Marshal(req.GetAttributes().AsMap())
		if err != nil {
			attrBytes = []byte("{}")
		}
	}

	var (
		query = `
			INSERT INTO project_folders (id, label, parent_id, type, icon, order_number, mcp_project_id, chat_id, attributes)
			VALUES ($1, $2, $3, $4::project_folders_type, $5, $6, $7, $8, $9)
			RETURNING id, label, parent_id, type, icon, order_number, mcp_project_id, chat_id, attributes, created_at, updated_at
		`

		retAttr []byte
	)

	err = conn.QueryRow(ctx, query,
		id, req.GetLabel(), nullString(req.GetParentId()),
		req.GetType(), req.GetIcon(), req.GetOrderNumber(),
		nullString(req.GetMcpProjectId()), nullString(req.GetChatId()), attrBytes,
	).Scan(
		&folder.Id, &folder.Label, &parentId, &folder.Type,
		&folder.Icon, &folder.OrderNumber, &mcpProjectId, &chatId,
		&retAttr, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create project_folder: %w", err)
	}

	if parentId.Valid {
		folder.ParentId = parentId.String
	}
	if mcpProjectId.Valid {
		folder.McpProjectId = mcpProjectId.String
	}
	if chatId.Valid {
		folder.ChatId = chatId.String
	}
	if len(retAttr) > 0 {
		var raw map[string]any
		if err = json.Unmarshal(retAttr, &raw); err == nil {
			folder.Attributes, _ = structpb.NewStruct(raw)
		}
	}

	folder.CreatedAt = createdAt.Format(time.RFC3339)
	folder.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &folder, nil
}

// ==================== Get By ID ====================

func (r *projectFoldersRepo) GetProjectFolderById(ctx context.Context, req *nb.ProjectFolderPrimaryKey) (*nb.ProjectFolder, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "projectFoldersRepo.GetProjectFolderById")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		folder                 nb.ProjectFolder
		parentId, mcpProjectId sql.NullString
		chatId                 sql.NullString
		attrBytes              []byte
		createdAt, updatedAt   time.Time

		query = `
			SELECT id, label, parent_id, type, icon, order_number, mcp_project_id, chat_id, attributes, created_at, updated_at
			FROM project_folders
			WHERE id = $1
		`
	)

	err = conn.QueryRow(ctx, query, req.GetId()).Scan(
		&folder.Id, &folder.Label, &parentId, &folder.Type,
		&folder.Icon, &folder.OrderNumber, &mcpProjectId, &chatId,
		&attrBytes, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get project_folder: %w", err)
	}

	if parentId.Valid {
		folder.ParentId = parentId.String
	}
	if mcpProjectId.Valid {
		folder.McpProjectId = mcpProjectId.String
	}
	if chatId.Valid {
		folder.ChatId = chatId.String
	}
	if len(attrBytes) > 0 {
		var raw map[string]any
		if err = json.Unmarshal(attrBytes, &raw); err == nil {
			folder.Attributes, _ = structpb.NewStruct(raw)
		}
	}

	folder.CreatedAt = createdAt.Format(time.RFC3339)
	folder.UpdatedAt = updatedAt.Format(time.RFC3339)

	// Load children
	children, err := r.GetAllProjectFolders(ctx, &nb.GetAllProjectFoldersRequest{
		ResourceEnvId: req.GetResourceEnvId(),
		ParentId:      folder.Id,
	})
	if err != nil {
		return nil, err
	}
	folder.Children = children.GetProjectFolders()

	return &folder, nil
}

// ==================== Get All ====================

func (r *projectFoldersRepo) GetAllProjectFolders(ctx context.Context, req *nb.GetAllProjectFoldersRequest) (*nb.GetAllProjectFoldersResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "projectFoldersRepo.GetAllProjectFolders")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		queryBuilder strings.Builder
		countBuilder strings.Builder
		args         = make([]any, 0)
		folders      = make([]*nb.ProjectFolder, 0)

		count       int32
		orderDir    = "ASC"
		orderColumn = "pf.order_number"
	)

	queryBuilder.WriteString(`
		SELECT pf.id, pf.label, pf.parent_id, pf.type, pf.icon, pf.order_number,
		       pf.mcp_project_id, pf.chat_id, pf.attributes, pf.created_at, pf.updated_at,
		       mp.id, mp.title, mp.description, mp.created_at, mp.updated_at
		FROM project_folders pf
		LEFT JOIN mcp_project mp ON pf.mcp_project_id = mp.id
		WHERE 1=1
	`)
	countBuilder.WriteString(`SELECT COUNT(*) FROM project_folders pf WHERE 1=1`)

	if req.GetParentId() != "" {
		args = append(args, req.GetParentId())
		queryBuilder.WriteString(fmt.Sprintf(" AND pf.parent_id = $%d", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND pf.parent_id = $%d", len(args)))
	} else {
		queryBuilder.WriteString(" AND pf.parent_id IS NULL")
		countBuilder.WriteString(" AND pf.parent_id IS NULL")
	}

	if req.GetType() != "" {
		args = append(args, req.GetType())
		queryBuilder.WriteString(fmt.Sprintf(" AND pf.type = $%d::project_folders_type", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND pf.type = $%d::project_folders_type", len(args)))
	}

	if req.GetLabel() != "" {
		args = append(args, "%"+req.GetLabel()+"%")
		queryBuilder.WriteString(fmt.Sprintf(" AND pf.label ILIKE $%d", len(args)))
		countBuilder.WriteString(fmt.Sprintf(" AND pf.label ILIKE $%d", len(args)))
	}

	err = conn.QueryRow(ctx, countBuilder.String(), args...).Scan(&count)
	if err != nil {
		return nil, fmt.Errorf("failed to count project_folders: %w", err)
	}

	if col, ok := config.ProjectFolderAllowedOrder[req.GetOrderBy()]; ok {
		orderColumn = col
	}

	if req.GetOrderDirection() == "desc" {
		orderDir = "DESC"
	}

	queryBuilder.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderColumn, orderDir))

	if req.GetLimit() > 0 {
		args = append(args, req.GetLimit())
		queryBuilder.WriteString(fmt.Sprintf(" LIMIT $%d", len(args)))
	}

	if req.GetOffset() > 0 {
		args = append(args, req.GetOffset())
		queryBuilder.WriteString(fmt.Sprintf(" OFFSET $%d", len(args)))
	}

	rows, err := conn.Query(ctx, queryBuilder.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query project_folders: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			folder                 nb.ProjectFolder
			parentId, mcpProjectId sql.NullString
			chatId                 sql.NullString
			attrBytes              []byte
			createdAt, updatedAt   time.Time

			mpId, mpTitle, mpDesc    sql.NullString
			mpCreatedAt, mpUpdatedAt sql.NullTime
		)

		err = rows.Scan(
			&folder.Id, &folder.Label, &parentId, &folder.Type,
			&folder.Icon, &folder.OrderNumber, &mcpProjectId, &chatId,
			&attrBytes, &createdAt, &updatedAt,
			&mpId, &mpTitle, &mpDesc, &mpCreatedAt, &mpUpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project_folder: %w", err)
		}

		if parentId.Valid {
			folder.ParentId = parentId.String
		}
		if mcpProjectId.Valid {
			folder.McpProjectId = mcpProjectId.String
		}
		if chatId.Valid {
			folder.ChatId = chatId.String
		}
		if len(attrBytes) > 0 {
			var raw map[string]any
			if err = json.Unmarshal(attrBytes, &raw); err == nil {
				folder.Attributes, _ = structpb.NewStruct(raw)
			}
		}

		folder.CreatedAt = createdAt.Format(time.RFC3339)
		folder.UpdatedAt = updatedAt.Format(time.RFC3339)

		if mpId.Valid {
			folder.ProjectData = &nb.ProjectFolderProjectData{
				Id:          mpId.String,
				Title:       mpTitle.String,
				Description: mpDesc.String,
			}
			if mpCreatedAt.Valid {
				folder.ProjectData.CreatedAt = mpCreatedAt.Time.Format(time.RFC3339)
			}
			if mpUpdatedAt.Valid {
				folder.ProjectData.UpdatedAt = mpUpdatedAt.Time.Format(time.RFC3339)
			}
		}

		folders = append(folders, &folder)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	return &nb.GetAllProjectFoldersResponse{
		ProjectFolders: folders,
		Count:          count,
	}, nil
}

// ==================== Update ====================

func (r *projectFoldersRepo) UpdateProjectFolder(ctx context.Context, req *nb.UpdateProjectFolderRequest) (*nb.ProjectFolder, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "projectFoldersRepo.UpdateProjectFolder")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		setClauses []string
		args       []any
		argIndex   = 1
	)

	setClauses = append(setClauses, "updated_at = NOW()")

	if req.GetLabel() != "" {
		setClauses = append(setClauses, fmt.Sprintf("label = $%d", argIndex))
		args = append(args, req.GetLabel())
		argIndex++
	}

	if req.GetParentId() != "" {
		setClauses = append(setClauses, fmt.Sprintf("parent_id = $%d", argIndex))
		args = append(args, req.GetParentId())
		argIndex++
	}

	if req.GetIcon() != "" {
		setClauses = append(setClauses, fmt.Sprintf("icon = $%d", argIndex))
		args = append(args, req.GetIcon())
		argIndex++
	}

	if req.GetOrderNumber() > 0 {
		setClauses = append(setClauses, fmt.Sprintf("order_number = $%d", argIndex))
		args = append(args, req.GetOrderNumber())
		argIndex++
	}

	if req.GetAttributes() != nil {
		attrBytes, marshalErr := json.Marshal(req.GetAttributes().AsMap())
		if marshalErr == nil {
			setClauses = append(setClauses, fmt.Sprintf("attributes = $%d", argIndex))
			args = append(args, attrBytes)
			argIndex++
		}
	}

	args = append(args, req.GetId())

	var (
		folder                 nb.ProjectFolder
		parentId, mcpProjectId sql.NullString
		chatId                 sql.NullString
		attrBytes              []byte
		createdAt, updatedAt   time.Time

		query = fmt.Sprintf(`
			UPDATE project_folders SET %s
			WHERE id = $%d
			RETURNING id, label, parent_id, type, icon, order_number,
			          mcp_project_id, chat_id, attributes, created_at, updated_at
		`, strings.Join(setClauses, ", "), argIndex)
	)

	err = conn.QueryRow(ctx, query, args...).Scan(
		&folder.Id, &folder.Label, &parentId, &folder.Type,
		&folder.Icon, &folder.OrderNumber, &mcpProjectId, &chatId,
		&attrBytes, &createdAt, &updatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to update project_folder: %w", err)
	}

	if parentId.Valid {
		folder.ParentId = parentId.String
	}
	if mcpProjectId.Valid {
		folder.McpProjectId = mcpProjectId.String
	}
	if chatId.Valid {
		folder.ChatId = chatId.String
	}
	if len(attrBytes) > 0 {
		var raw map[string]any
		if err = json.Unmarshal(attrBytes, &raw); err == nil {
			folder.Attributes, _ = structpb.NewStruct(raw)
		}
	}

	folder.CreatedAt = createdAt.Format(time.RFC3339)
	folder.UpdatedAt = updatedAt.Format(time.RFC3339)

	return &folder, nil
}

// ==================== Delete ====================

func (r *projectFoldersRepo) DeleteProjectFolder(ctx context.Context, req *nb.ProjectFolderPrimaryKey) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "projectFoldersRepo.DeleteProjectFolder")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return err
	}

	var query = `DELETE FROM project_folders WHERE id = $1`
	res, err := conn.Exec(ctx, query, req.GetId())
	if err != nil {
		return fmt.Errorf("failed to delete project_folder: %w", err)
	}

	if res.RowsAffected() == 0 {
		return fmt.Errorf("project_folder not found: %s", req.GetId())
	}

	return nil
}

// ==================== Update Order ====================

func (r *projectFoldersRepo) UpdateProjectFolderOrder(ctx context.Context, req *nb.UpdateProjectFolderOrderRequest) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "projectFoldersRepo.UpdateProjectFolderOrder")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return err
	}

	for _, item := range req.GetItems() {
		var query = `UPDATE project_folders SET order_number = $1, updated_at = NOW() WHERE id = $2`
		_, err = conn.Exec(ctx, query, item.GetOrderNumber(), item.GetId())
		if err != nil {
			return fmt.Errorf("failed to update order for folder %s: %w", item.GetId(), err)
		}
	}

	return nil
}

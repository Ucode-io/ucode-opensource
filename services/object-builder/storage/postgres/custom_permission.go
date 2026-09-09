package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/types/known/structpb"
)

type customPermissionsRepo struct {
	db *psqlpool.Pool
}

func NewCustomPermissionsRepo(db *psqlpool.Pool) storage.CustomPermissionsRepoI {
	return &customPermissionsRepo{
		db: db,
	}
}

// ==================== Definition CRUD ====================

func (r *customPermissionsRepo) Create(ctx context.Context, req *nb.CreateCustomPermissionRequest) (*nb.CustomPermission, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.Create")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if _, err := uuid.Parse(req.GetClientTypeId()); err != nil {
		return nil, fmt.Errorf("client_type_id is required")
	}

	id := uuid.NewString()

	attributesBytes, err := json.Marshal(req.Attributes)
	if err != nil {
		attributesBytes = []byte("{}")
	}

	// Insert and return the full row
	var (
		perm     models.CustomPermDef
		parentId sql.NullString
		retAttrs []byte
	)

	query := `
       INSERT INTO custom_permission (id, parent_id, title, attributes, client_type_id)
       VALUES ($1, $2, $3, $4, $5)
       RETURNING id, parent_id, title, attributes, created_at, updated_at, client_type_id
    `
	err = conn.QueryRow(ctx, query,
		id,
		nullString(req.ParentId),
		req.Title,
		attributesBytes,
		req.ClientTypeId,
	).Scan(&perm.Id, &parentId, &perm.Title, &retAttrs, &perm.CreatedAt, &perm.UpdatedAt, &perm.ClientTypeId)
	if err != nil {
		return nil, err
	}

	perm.ParentId = parentId
	if err := json.Unmarshal(retAttrs, &perm.Attributes); err != nil {
		return nil, err
	}

	// Auto-create access rows only for roles that belong to this custom permission's client type.
	// Defaults in DB are 'No' (deny). Exception: a nav-gating permission with
	// attributes.nav_path seeds read='Yes' so a new sidebar item stays visible
	// until an admin explicitly hides it.
	readDefault := "No"
	if attrs := req.GetAttributes(); attrs != nil {
		if v, ok := attrs.GetFields()["nav_path"]; ok && strings.TrimSpace(v.GetStringValue()) != "" {
			readDefault = "Yes"
		}
	}
	accessQuery := `
       INSERT INTO custom_permission_access (id, custom_permission_id, role_id, client_type_id, "read")
       SELECT uuid_generate_v4(), $1, r.guid, r.client_type_id, $2
       FROM role r
       WHERE r.client_type_id = $3
       ON CONFLICT (custom_permission_id, role_id, client_type_id) DO NOTHING
    `
	// Используем Exec, так как нам не нужны данные обратно
	_, err = conn.Exec(ctx, accessQuery, id, readDefault, req.ClientTypeId)
	if err != nil {
		return nil, err
	}

	log.Println("Created Custom Permission:", perm)

	return perm.ToProto(), nil
}

func (r *customPermissionsRepo) Update(ctx context.Context, req *nb.UpdateCustomPermissionRequest) (*nb.CustomPermission, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.Update")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	attributesBytes, err := json.Marshal(req.Attributes)
	if err != nil {
		attributesBytes = []byte("{}")
	}

	var (
		perm     models.CustomPermDef
		parentId sql.NullString
		retAttrs []byte
	)

	query := `
       UPDATE custom_permission SET
          parent_id = $1,
          title = $2,
          attributes = $3,
          updated_at = NOW()
       WHERE id = $4
       RETURNING id, parent_id, title, attributes, created_at, updated_at, client_type_id
    `
	err = conn.QueryRow(ctx, query,
		nullString(req.ParentId),
		req.Title,
		attributesBytes,
		req.Id,
	).Scan(&perm.Id, &parentId, &perm.Title, &retAttrs, &perm.CreatedAt, &perm.UpdatedAt, &perm.ClientTypeId)
	if err != nil {
		return nil, err
	}

	perm.ParentId = parentId
	if err := json.Unmarshal(retAttrs, &perm.Attributes); err != nil {
		return nil, err
	}

	return perm.ToProto(), nil
}

func (r *customPermissionsRepo) Delete(ctx context.Context, req *nb.DeleteCustomPermissionRequest) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.Delete")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `DELETE FROM custom_permission WHERE id = $1`
	_, err = conn.Exec(ctx, query, req.Id)
	return err
}

func (r *customPermissionsRepo) GetAll(ctx context.Context, req *nb.GetAllCustomPermissionsRequest) (*nb.GetAllCustomPermissionsResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.GetAll")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var args []any

	query := `
       SELECT id, parent_id, title, attributes, created_at, updated_at, client_type_id
       FROM custom_permission
       ORDER BY created_at
    `

	if req.Limit > 0 {
		args = append(args, req.Limit)
		query += fmt.Sprintf(" LIMIT $%d", len(args))
	}

	if req.Page > 0 && req.Limit > 0 {
		offset := (req.Page - 1) * req.Limit
		args = append(args, offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := &nb.GetAllCustomPermissionsResponse{
		CustomPermissions: []*nb.CustomPermission{},
	}

	for rows.Next() {
		var (
			perm            = &models.CustomPermDef{}
			parentId        sql.NullString
			attributesBytes []byte
		)

		err = rows.Scan(
			&perm.Id,
			&parentId,
			&perm.Title,
			&attributesBytes,
			&perm.CreatedAt,
			&perm.UpdatedAt,
			&perm.ClientTypeId,
		)
		if err != nil {
			return nil, err
		}

		perm.ParentId = parentId
		if err := json.Unmarshal(attributesBytes, &perm.Attributes); err != nil {
			return nil, err
		}

		res.CustomPermissions = append(res.CustomPermissions, perm.ToProto())
	}

	return res, nil
}

// ==================== Access ====================

func (r *customPermissionsRepo) GetAccesses(ctx context.Context, req *nb.GetCustomPermissionAccessesRequest) (*nb.GetCustomPermissionAccessesResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.GetAccesses")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var (
		parentFilter string
		args         = []any{req.RoleId, req.ClientTypeId}
	)

	if req.ParentId == "" {
		parentFilter = "cp.parent_id IS NULL"
	} else {
		args = append(args, req.ParentId)
		parentFilter = fmt.Sprintf("cp.parent_id = $%d", len(args))
	}

	query := fmt.Sprintf(`
       SELECT cp.id, cp.title, cp.parent_id, cp.attributes,
              cpa."read", cpa."write", cpa."update", cpa."delete"
       FROM custom_permission cp
       JOIN custom_permission_access cpa ON cpa.custom_permission_id = cp.id
       WHERE cpa.role_id = $1 AND cpa.client_type_id = $2
         AND %s
       ORDER BY cp.created_at
    `, parentFilter)

	return r.scanPermissionWithAccessRows(ctx, conn, query, args...)
}

func (r *customPermissionsRepo) GetAllAccesses(ctx context.Context, req *nb.GetAllCustomPermissionAccessesRequest) (*nb.GetCustomPermissionAccessesResponse, error) {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.GetAllAccesses")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
       SELECT cp.id, cp.title, cp.parent_id, cp.attributes,
              cpa."read", cpa."write", cpa."update", cpa."delete"
       FROM custom_permission cp
       JOIN custom_permission_access cpa ON cpa.custom_permission_id = cp.id
       WHERE cpa.role_id = $1 AND cpa.client_type_id = $2
       ORDER BY cp.created_at
    `

	return r.scanPermissionWithAccessRows(ctx, conn, query, req.RoleId, req.ClientTypeId)
}

// UpdateAccess — unified update handler.
// Теперь работает со строками "Yes"/"No"
func (r *customPermissionsRepo) UpdateAccess(ctx context.Context, req *nb.UpdateCustomPermissionAccessRequest) error {
	span, ctx := opentracing.StartSpanFromContext(ctx, "customPermissionsRepo.UpdateAccess")
	defer span.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	var (
		setClauses []string
		args       []interface{}
		argIdx     = 1
	)

	// Проверяем не на nil, а на пустую строку
	if req.Read != "" {
		setClauses = append(setClauses, fmt.Sprintf("\"read\" = $%d", argIdx))
		args = append(args, req.Read)
		argIdx++
	}

	if req.Write != "" {
		setClauses = append(setClauses, fmt.Sprintf("\"write\" = $%d", argIdx))
		args = append(args, req.Write)
		argIdx++
	}

	if req.Update != "" {
		setClauses = append(setClauses, fmt.Sprintf("\"update\" = $%d", argIdx))
		args = append(args, req.Update)
		argIdx++
	}

	if req.Delete != "" {
		setClauses = append(setClauses, fmt.Sprintf("\"delete\" = $%d", argIdx))
		args = append(args, req.Delete)
		argIdx++
	}

	if len(setClauses) == 0 {
		return nil
	}

	setClauses = append(setClauses, "updated_at = NOW()")

	var whereParts []string

	whereParts = append(whereParts, fmt.Sprintf("role_id = $%d", argIdx))
	args = append(args, req.RoleId)
	argIdx++

	whereParts = append(whereParts, fmt.Sprintf("client_type_id = $%d", argIdx))
	args = append(args, req.ClientTypeId)
	argIdx++

	if req.CustomPermissionId != "" {
		whereParts = append(whereParts, fmt.Sprintf("custom_permission_id = $%d", argIdx))
		args = append(args, req.CustomPermissionId)
	}

	query := fmt.Sprintf(
		"UPDATE custom_permission_access SET %s WHERE %s",
		strings.Join(setClauses, ", "),
		strings.Join(whereParts, " AND "),
	)

	_, err = conn.Exec(ctx, query, args...)
	return err
}

// ==================== Helpers ====================

func (r *customPermissionsRepo) scanPermissionWithAccessRows(ctx context.Context, conn *psqlpool.Pool, query string, args ...interface{}) (*nb.GetCustomPermissionAccessesResponse, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	res := &nb.GetCustomPermissionAccessesResponse{
		Permissions: []*nb.CustomPermissionWithAccess{},
	}

	for rows.Next() {
		var (
			permId          string
			title           string
			parentId        sql.NullString
			attributesBytes []byte
			read            string
			write           string
			update          string
			del             string
		)

		err = rows.Scan(&permId, &title, &parentId, &attributesBytes, &read, &write, &update, &del)
		if err != nil {
			return nil, err
		}

		var attrs *structpb.Struct

		if len(attributesBytes) > 0 {
			attrs = &structpb.Struct{}
			var raw map[string]interface{}
			if err = json.Unmarshal(attributesBytes, &raw); err == nil {
				attrs, _ = structpb.NewStruct(raw)
			}
		}

		item := &nb.CustomPermissionWithAccess{
			CustomPermissionId: permId,
			Title:              title,
			Attributes:         attrs,
			Read:               read,   // теперь это string ("Yes"/"No")
			Write:              write,  // string
			Update:             update, // string
			Delete:             del,    // string
		}

		if parentId.Valid {
			item.ParentId = parentId.String
		}

		res.Permissions = append(res.Permissions, item)
	}

	return res, nil
}

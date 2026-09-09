package helper

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/gogo/protobuf/jsonpb"
	"github.com/jackc/pgx/v5/pgxpool"
	"google.golang.org/protobuf/types/known/structpb"
)

func AddPermissionToField(ctx context.Context, conn *psqlpool.Pool, fields []*new_object_builder_service.FieldResponse, roleId string, tableSlug string, projectID string) ([]*new_object_builder_service.FieldResponse, error) {
	var (
		query                      string
		fieldIds                   []string
		fieldsWithPermissions      []*new_object_builder_service.FieldResponse
		unusedFieldsSlugs          = make(map[string]int)
		fieldPermissionMap         = make(map[string]any)
		relationFieldPermissionMap = make(map[string]string)
	)

	for _, field := range fields {
		if strings.Contains(field.Id, "#") {
			parts := strings.Split(field.Id, "#")
			relationID := parts[1]

			query := `
				SELECT id FROM field WHERE relation_id = $1 AND table_id = (SELECT id FROM table WHERE slug = $2)
			`
			var fieldID string
			err := conn.QueryRow(ctx, query, relationID, tableSlug).Scan(&fieldID)
			if err != nil {
				continue
			}

			relationFieldPermissionMap[relationID] = fieldID
			fieldIds = append(fieldIds, fieldID)
			continue
		}

		fieldIds = append(fieldIds, field.Id)
	}

	query = `
		SELECT
			field_id,
			view_permission,
			field_permission
		FROM field_permission
		WHERE field_id = ANY($1)
			AND role_id = $2
			AND table_slug = $3
	`
	rows, err := conn.Query(ctx, query, fieldIds, roleId, tableSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var fieldID string
		var viewPermission bool
		var fieldPermission json.RawMessage

		if err := rows.Scan(&fieldID, &viewPermission, &fieldPermission); err != nil {
			return nil, err
		}

		fieldPermissionMap[fieldID] = map[string]any{
			"view_permission":  viewPermission,
			"field_permission": fieldPermission,
		}
	}

	for _, field := range fields {
		var id string

		if strings.Contains(field.Id, "#") {
			parts := strings.Split(field.Id, "#")
			id = relationFieldPermissionMap[parts[1]]
		} else {
			id = field.Id
		}

		fieldPer, ok := fieldPermissionMap[id]

		if ok && roleId != "" {
			if field.Attributes != nil {
				decodedAttributes := make(map[string]any)
				attributesBytes, err := field.Attributes.MarshalJSON()
				if err != nil {
					return nil, err
				}
				err = json.Unmarshal(attributesBytes, &decodedAttributes)
				if err != nil {
					return nil, err
				}
				decodedAttributes["field_permission"] = fieldPer.(map[string]any)["field_permission"]
				encodedAttributes, err := json.Marshal(decodedAttributes)
				if err != nil {
					return nil, err
				}
				var structAttributes *structpb.Struct
				err = jsonpb.UnmarshalString(string(encodedAttributes), structAttributes)
				if err != nil {
					return nil, err
				}
				field.Attributes = structAttributes
			} else {
				attributes := map[string]any{
					"field_permission": fieldPer.(map[string]any)["field_permission"],
				}
				encodedAttributes, err := json.Marshal(attributes)
				if err != nil {
					return nil, err
				}
				var structAttributes *structpb.Struct
				err = jsonpb.UnmarshalString(string(encodedAttributes), structAttributes)
				if err != nil {
					return nil, err
				}
				field.Attributes = structAttributes
			}
			if fieldPerMap, ok := fieldPer.(map[string]any); ok {
				if !fieldPerMap["view_permission"].(bool) {
					unusedFieldsSlugs[field.Slug] = 0
					continue
				}
			}
			fieldsWithPermissions = append(fieldsWithPermissions, field)
			fieldsWithPermissions = append(fieldsWithPermissions, field)
		} else if roleId == "" || !ok {
			fieldsWithPermissions = append(fieldsWithPermissions, field)
		} else {
			unusedFieldsSlugs[field.Slug] = 0
		}
	}

	return fieldsWithPermissions, nil
}

func AddPermissionToTab(ctx context.Context, relation map[string]any, conn *pgxpool.Pool, roleId string, tableSlug string, projectID string) (map[string]any, error) {
	var (
		guid              string
		role_id           string
		query             string
		relation_id       sql.NullString
		view_permission   bool
		create_permission bool
		edit_permission   bool
		delete_permission bool
	)

	query = `
        SELECT 
		"guid",
		"role_id",
		"table_slug",
		"relation_id",
		"view_permission",
		"create_permission",
		"edit_permission",
		"delete_permission"
		FROM view_relation_permission
        WHERE role_id = $1 AND table_slug = $2 AND relation_id = $3
    `

	err := conn.QueryRow(ctx, query, roleId, tableSlug, relation["id"]).Scan(&guid, &role_id, &tableSlug, &relation_id, &view_permission, &create_permission, &edit_permission, &delete_permission)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}

	encodedPermission := make(map[string]any)
	encodedPermission["guid"] = guid
	encodedPermission["role_id"] = roleId
	encodedPermission["table_slug"] = tableSlug
	encodedPermission["relation_id"] = relation["id"]
	encodedPermission["view_permission"] = view_permission
	encodedPermission["create_permission"] = create_permission
	encodedPermission["edit_permission"] = edit_permission
	encodedPermission["delete_permission"] = delete_permission

	relation["permission"] = encodedPermission

	return relation, nil
}

package helper

import (
	"context"
	"encoding/json"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
)

func GetFieldBySlug(ctx context.Context, req models.GetFieldBySlugReq) (map[string]any, error) {
	var (
		id, ftype, query string
		attributes       []byte
	)

	query = `SELECT id, type, attributes FROM "field" WHERE slug = $1 AND table_id = $2`

	err := req.Conn.QueryRow(ctx, query, req.Slug, req.TableId).Scan(&id, &req.Slug)
	if err != nil {
		return map[string]any{}, err
	}

	return map[string]any{
		"id":         id,
		"type":       ftype,
		"attributes": attributes,
	}, nil
}

func AddPermissionToField1(ctx context.Context, req models.AddPermissionToFieldRequest) ([]models.Field, map[string]int, error) {
	var (
		fieldPermissionMap         = make(map[string]models.FieldPermission)
		relationFieldPermissionMap = make(map[string]string)
		unusedFieldsSlugs          = make(map[string]int)
		fieldsWithPermissions      = []models.Field{}
		fieldIds                   = []string{}

		query   string
		tableId string
	)

	query = `SELECT "id" FROM "table" WHERE "slug" = $1`

	err := req.Conn.QueryRow(ctx, query, req.TableSlug).Scan(&tableId)
	if err != nil {
		return []models.Field{}, map[string]int{}, err
	}

	for _, field := range req.Fields {
		var (
			fieldId    string
			relationId string
		)

		if strings.Contains(field.Id, "#") {
			relationId = strings.Split(field.Id, "#")[1]

			query = `SELECT "id" FROM "field" WHERE relation_id = $1 AND table_id = $2`

			err = req.Conn.QueryRow(ctx, query, relationId, tableId).Scan(&fieldId)
			if err != nil {
				return []models.Field{}, map[string]int{}, err
			}

			if fieldId != "" {
				relationFieldPermissionMap[relationId] = fieldId
				fieldIds = append(fieldIds, fieldId)
				continue
			}
		} else {
			fieldIds = append(fieldIds, field.Id)
		}
	}

	if len(fieldIds) > 0 {
		query := `
			SELECT
				"guid",
				"role_id",
				"label",
				"table_slug",
				"field_id",
				"edit_permission",
				"view_permission"
			FROM "field_permission" 
			WHERE field_id = ANY($1) AND role_id = $2 AND table_slug = $3
		`

		rows, err := req.Conn.Query(ctx, query, pq.Array(fieldIds), req.RoleId, req.TableSlug)
		if err != nil {
			return []models.Field{}, map[string]int{}, err
		}
		defer rows.Close()

		for rows.Next() {
			fp := models.FieldPermission{}

			err := rows.Scan(
				&fp.Guid,
				&fp.RoleId,
				&fp.Label,
				&fp.TableSlug,
				&fp.FieldId,
				&fp.EditPermission,
				&fp.ViewPermission,
			)
			if err != nil {
				return []models.Field{}, map[string]int{}, err
			}

			fieldPermissionMap[fp.FieldId] = fp
		}
	}

	for _, field := range req.Fields {
		id := field.Id
		if strings.Contains(id, "#") {
			id = relationFieldPermissionMap[strings.Split(id, "#")[1]]
		}
		fieldPer, ok := fieldPermissionMap[id]

		if v, ok := field.Attributes.Fields["enable_multi_language"]; ok {
			field.EnableMultilanguage = v.GetBoolValue()
		}

		if ok && req.RoleId != "" {
			if field.Attributes != nil {
				decoded := make(map[string]any)
				body, err := json.Marshal(field.Attributes)
				if err != nil {
					return []models.Field{}, map[string]int{}, err
				}
				if err := json.Unmarshal(body, &decoded); err != nil {
					return []models.Field{}, map[string]int{}, err
				}
				decoded["field_permission"] = fieldPer
				newAtb, err := ConvertMapToStruct(decoded)
				if err != nil {
					return []models.Field{}, map[string]int{}, err
				}
				field.Attributes = newAtb
			} else {
				atributes := map[string]any{
					"field_permission": fieldPer,
				}

				newAtb, err := ConvertMapToStruct(atributes)
				if err != nil {
					return []models.Field{}, map[string]int{}, err
				}

				field.Attributes = newAtb
			}

			if !fieldPer.ViewPermission {
				unusedFieldsSlugs[field.Slug] = 0
				continue
			}
			fieldsWithPermissions = append(fieldsWithPermissions, field)
		} else if req.RoleId == "" {
			fieldsWithPermissions = append(fieldsWithPermissions, field)
		} else {
			unusedFieldsSlugs[field.Slug] = 0
		}
	}
	return fieldsWithPermissions, unusedFieldsSlugs, nil
}

func GetFieldByType(ctx context.Context, conn *pgxpool.Pool, tableId, fieldType string) (models.FieldBody, error) {

	var (
		slug       string
		body       []byte
		attributes = make(map[string]any)
	)

	query := `SELECT 
		"slug",
		"attributes"
	FROM "field" WHERE table_id = $1 AND "type" = $2`

	err := conn.QueryRow(ctx, query, tableId, fieldType).Scan(&slug, &body)
	if err != nil {
		return models.FieldBody{}, err
	}
	if err := json.Unmarshal(body, &attributes); err != nil {
		return models.FieldBody{}, err
	}

	return models.FieldBody{Slug: slug, Attributes: attributes}, nil
}

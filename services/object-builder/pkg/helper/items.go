package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

func IsExists(ctx context.Context, conn *pgxpool.Pool, req models.IsExistsBody) (bool, error) {

	count := 0

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE %s=$1`, req.TableSlug, req.FieldSlug)

	err := conn.QueryRow(ctx, query, req.FieldValue).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

func IsExistsWithTx(ctx context.Context, conn pgx.Tx, req models.IsExistsBody) (bool, error) {

	count := 0

	query := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" WHERE %s=$1`, req.TableSlug, req.FieldSlug)

	err := conn.QueryRow(ctx, query, req.FieldValue).Scan(&count)
	if err != nil {
		return false, err
	}

	if count > 0 {
		return true, nil
	} else {
		return false, nil
	}
}

func GetItem(ctx context.Context, conn *psqlpool.Pool, tableSlug, guid string, fromAuth bool) (map[string]any, error) {
	var query string

	if !fromAuth {
		query = fmt.Sprintf(`SELECT * FROM "%s" WHERE guid = $1`, tableSlug)
	} else {
		query = fmt.Sprintf(`SELECT * FROM "%s" WHERE user_id_auth = $1`, tableSlug)
	}

	rows, err := conn.Query(ctx, query, guid)
	if err != nil {
		return map[string]any{}, errors.Wrap(err, "query rows")
	}
	defer rows.Close()

	data := make(map[string]any)

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return map[string]any{}, errors.Wrap(err, "values")
		}

		for i, value := range values {
			if strings.Contains(string(rows.FieldDescriptions()[i].Name), "_id") || string(rows.FieldDescriptions()[i].Name) == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = ConvertGuid(arr)
				}
			}

			data[string(rows.FieldDescriptions()[i].Name)] = value
		}
	}

	return data, nil
}

func GetItemLogin(ctx context.Context, conn *psqlpool.Pool, tableSlug, guid, clientType string) (map[string]any, error) {
	query := fmt.Sprintf(`SELECT * FROM "%s" WHERE user_id_auth = $1 AND client_type_id = $2`, tableSlug)

	rows, err := conn.Query(ctx, query, guid, clientType)
	if err != nil {
		return map[string]any{}, errors.Wrap(err, "query rows")
	}
	defer rows.Close()

	data := make(map[string]any)

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return map[string]any{}, errors.Wrap(err, "values")
		}

		for i, value := range values {
			if strings.Contains(string(rows.FieldDescriptions()[i].Name), "_id") || string(rows.FieldDescriptions()[i].Name) == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = ConvertGuid(arr)
				}
			}

			data[string(rows.FieldDescriptions()[i].Name)] = value
		}
	}

	return data, nil
}

func GetItemWithTx(ctx context.Context, conn pgx.Tx, tableSlug, guid string, fromAuth bool) (map[string]any, error) {
	var query string

	if !fromAuth {
		query = fmt.Sprintf(`SELECT * FROM "%s" WHERE guid = $1`, tableSlug)
	} else {
		query = fmt.Sprintf(`SELECT * FROM "%s" WHERE user_id_auth = $1`, tableSlug)
	}

	rows, err := conn.Query(ctx, query, guid)
	if err != nil {
		return map[string]any{}, errors.Wrap(err, "")
	}
	defer rows.Close()

	data := make(map[string]any)

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return map[string]any{}, errors.Wrap(err, "")
		}

		for i, value := range values {
			if strings.Contains(string(rows.FieldDescriptions()[i].Name), "_id") || string(rows.FieldDescriptions()[i].Name) == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = ConvertGuid(arr)
				}
			}

			data[string(rows.FieldDescriptions()[i].Name)] = value
		}
	}

	return data, nil
}

func GetItems(ctx context.Context, conn *psqlpool.Pool, req models.GetItemsBody) ([]map[string]any, int, error) {
	var (
		relations       []models.Relation
		relationMap     = make(map[string]map[string]any)
		tableSlug       = req.TableSlug
		params          = req.Params
		fields          = req.FieldsMap
		order           = " ORDER BY created_at DESC "
		filter          = " WHERE deleted_at IS NULL "
		limit, offset   = " LIMIT 20 ", " OFFSET 0"
		args, argCount  = []any{}, 1
		query           = fmt.Sprintf(`SELECT * FROM "%s" `, tableSlug)
		countQuery      = fmt.Sprintf(`SELECT COUNT(*) FROM "%s" `, tableSlug)
		searchValue     = cast.ToString(params["search"])
		searchCondition string
	)

	table, err := TableFindOne(ctx, conn, tableSlug)
	if err != nil {
		return nil, 0, err
	}

	if !table.OrderBy {
		order = " ORDER BY created_at ASC "
	}

	if tableSlug == "user" {
		query = `SELECT * FROM "user" `
		countQuery = `SELECT COUNT(*) FROM "user" `
	}

	for key, val := range params {
		switch key {
		case "limit":
			limit = fmt.Sprintf(" LIMIT %d ", cast.ToInt(val))
		case "offset":
			offset = fmt.Sprintf(" OFFSET %d ", cast.ToInt(val))
		case "order":
			orders := cast.ToStringMap(val)
			counter := 0

			if len(orders) > 0 {
				order = " ORDER BY "
			}

			for k, v := range orders {
				oType := " ASC"
				if cast.ToInt(v) == -1 {
					oType = " DESC"
				}

				if counter == 0 {
					order += fmt.Sprintf(" %s"+oType, k)
				} else {
					order += fmt.Sprintf(", %s"+oType, k)
				}
				counter++
			}
		default:
			if _, ok := fields[key]; ok {
				switch val.(type) {
				case []string:
					filter += fmt.Sprintf(" AND %s IN($%d) ", key, argCount)
					args = append(args, pq.Array(val))
				case int, float32, float64, int32:
					filter += fmt.Sprintf(" AND %s = $%d ", key, argCount)
					args = append(args, val)
				case []any:
					if fields[key].Type == "MULTISELECT" || strings.Contains(fields[key].Slug, "_ids") {
						filter += fmt.Sprintf(" AND %s && $%d ", key, argCount)
						args = append(args, pq.Array(val))
					} else {
						filter += fmt.Sprintf(" AND %s = ANY($%d) ", key, argCount)
						args = append(args, pq.Array(val))
					}
				case bool:
					filter += fmt.Sprintf(" AND %s = $%v ", key, argCount)
					args = append(args, val)
				case map[string]any:
					newOrder := cast.ToStringMap(val)

					for k, val := range newOrder {
						switch val.(type) {
						case string:
							if cast.ToString(val) == "" {
								continue
							}
						case int, float32, float64, int32:
							if cast.ToFloat32(val) == 0 {
								continue
							}
						}

						switch k {
						case "$gt":
							filter += fmt.Sprintf(" AND %s > $%d ", key, argCount)
						case "$gte":
							filter += fmt.Sprintf(" AND %s >= $%d ", key, argCount)
						case "$lt":
							filter += fmt.Sprintf(" AND %s < $%d ", key, argCount)
						case "$lte":
							filter += fmt.Sprintf(" AND %s <= $%d ", key, argCount)
						case "$in":
							filter += fmt.Sprintf(" AND %s::VARCHAR = ANY($%d)", key, argCount)
						}

						args = append(args, val)
						argCount++
					}
					argCount--
				default:
					if strings.Contains(key, "_id") || key == "guid" {
						if tableSlug == "client_type" {
							filter += " AND guid = ANY($1::UUID[]) "

							args = append(args, pq.Array(cast.ToStringSlice(val)))
						} else {
							if val == nil {
								filter += fmt.Sprintf(" AND %s IS NULL ", key)
								argCount -= 1
							} else {
								filter += fmt.Sprintf(" AND %s = $%d ", key, argCount)
								args = append(args, val)
							}
						}
					} else {
						typeOfVal := reflect.TypeOf(val)
						if typeOfVal.Kind() == reflect.String {
							valString := val.(string)
							if after, ok0 := strings.CutPrefix(valString, "+"); ok0 {
								valString = after
								val = valString
							}
						}
						filter += fmt.Sprintf(" AND %s ~* $%d ", key, argCount)
						args = append(args, val)

					}
				}

				argCount++
			}
		}
	}

	if len(searchValue) > 0 {
		for idx, val := range req.SearchFields {
			if idx == 0 {
				filter += " AND ("
				searchCondition = ""
			} else {
				searchCondition = " OR "
			}
			filter += fmt.Sprintf(" %s %s ~* $%d ", searchCondition, val, argCount)
			args = append(args, searchValue)
			argCount++

			if idx == len(req.SearchFields)-1 {
				filter += " ) "
			}
		}
	}

	countQuery += filter
	query += filter + order + limit + offset

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var result []map[string]any

	skipFields := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
	}

	var (
		withRelations  = cast.ToBool(params["with_relations"])
		selectedFields = cast.ToSlice(params["selected_relations"])
	)

	if withRelations {
		var relRows pgx.Rows
		var relationQuery = `SELECT
						id,
						table_from,
						table_to,
						field_from,
						type
					FROM
						relation`

		if len(selectedFields) > 0 {
			relationQuery += " WHERE  table_from = $1 AND table_to = ANY($2)"
			relRows, err = conn.Query(ctx, relationQuery, tableSlug, pq.Array(selectedFields))
		} else {
			relationQuery += " WHERE  table_from = $1 OR table_to = $1"
			relRows, err = conn.Query(ctx, relationQuery, tableSlug)
		}

		if err != nil {
			return nil, 0, err
		}
		defer relRows.Close()

		for relRows.Next() {
			relation := models.Relation{}

			err := relRows.Scan(
				&relation.Id,
				&relation.TableFrom,
				&relation.TableTo,
				&relation.FieldFrom,
				&relation.Type,
			)
			if err != nil {
				return nil, 0, err
			}

			if config.SKIPPED_RELATION_TYPES[relation.Type] {
				continue
			}

			relations = append(relations, relation)
		}
	}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, 0, err
		}

		data := make(map[string]any, len(values))

		for i, value := range values {
			fieldName := string(rows.FieldDescriptions()[i].Name)

			if skipFields[fieldName] {
				continue
			}
			if strings.Contains(fieldName, "_id") || fieldName == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = ConvertGuid(arr)
				}

				if tableSlug == "client_type" {
					if arr, ok := value.([]any); ok {
						ids := []any{}
						for _, a := range arr {
							ids = append(ids, ConvertGuid(a.([16]uint8)))
						}

						value = ids
					}
				}
			}
			data[fieldName] = value
		}

		if len(relations) > 0 {
			for _, relation := range relations {
				joinId := cast.ToString(data[relation.TableTo+"_id"])
				if _, ok := relationMap[joinId]; ok {
					data[relation.TableTo+"_id_data"] = relationMap[joinId]
					continue
				}
				relationData, err := GetItem(ctx, conn, relation.TableTo, joinId, false)
				if err != nil {
					return nil, 0, err
				}

				data[relation.TableTo+"_id_data"] = relationData
				relationMap[joinId] = relationData
			}
		}

		result = append(result, data)
	}

	count := 0
	err = conn.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return result, count, nil
}

func GetItemsGetList(ctx context.Context, conn *psqlpool.Pool, req models.GetItemsBody) ([]map[string]any, int, error) {
	var (
		relations       []models.Relation
		relationMap     = make(map[string]map[string]any)
		tableSlug       = req.TableSlug
		params          = req.Params
		fields          = req.FieldsMap
		order           = " ORDER BY created_at DESC "
		filter          = " WHERE  1=1 "
		limit, offset   = " LIMIT 20 ", " OFFSET 0"
		args, argCount  = []any{}, 1
		query           = fmt.Sprintf(`SELECT * FROM "%s" `, tableSlug)
		countQuery      = fmt.Sprintf(`SELECT COUNT(*) FROM "%s" `, tableSlug)
		searchValue     = cast.ToString(params["search"])
		searchCondition string
	)

	if tableSlug == "user" {
		query = `SELECT * FROM "user" `
		countQuery = `SELECT COUNT(*) FROM "user" `
	}

	for key, val := range params {
		switch key {
		case "limit":
			limit = fmt.Sprintf(" LIMIT %d ", cast.ToInt(val))
		case "offset":
			offset = fmt.Sprintf(" OFFSET %d ", cast.ToInt(val))
		case "order":
			orders := cast.ToStringMap(val)
			counter := 0

			if len(orders) > 0 {
				order = " ORDER BY "
			}

			for k, v := range orders {
				oType := " ASC"
				if cast.ToInt(v) == -1 {
					oType = " DESC"
				}

				if counter == 0 {
					order += fmt.Sprintf(" %s"+oType, k)
				} else {
					order += fmt.Sprintf(", %s"+oType, k)
				}
				counter++
			}
		default:
			if _, ok := fields[key]; ok {
				switch val.(type) {
				case []string:
					filter += fmt.Sprintf(" AND %s IN($%d) ", key, argCount)
					args = append(args, pq.Array(val))
				case int, float32, float64, int32, bool:
					filter += fmt.Sprintf(" AND %s = $%d ", key, argCount)
					args = append(args, val)
				case []any:
					if fields[key].Type == "MULTISELECT" || strings.Contains(fields[key].Slug, "_ids") {
						filter += fmt.Sprintf(" AND %s && $%d ", key, argCount)
						args = append(args, pq.Array(val))
					} else {
						filter += fmt.Sprintf(" AND %s = ANY($%d) ", key, argCount)
						args = append(args, pq.Array(val))
					}

				case map[string]any:
					newOrder := cast.ToStringMap(val)

					for k, val := range newOrder {
						switch val.(type) {
						case string:
							if cast.ToString(val) == "" {
								continue
							}
						case int, float32, float64, int32:
							if cast.ToFloat32(val) == 0 {
								continue
							}
						}

						switch k {
						case "$gt":
							filter += fmt.Sprintf(" AND %s > $%d ", key, argCount)
						case "$gte":
							filter += fmt.Sprintf(" AND %s >= $%d ", key, argCount)
						case "$lt":
							filter += fmt.Sprintf(" AND %s < $%d ", key, argCount)
						case "$lte":
							filter += fmt.Sprintf(" AND %s <= $%d ", key, argCount)
						case "$in":
							filter += fmt.Sprintf(" AND %s::varchar = ANY($%d)", key, argCount)
						}

						args = append(args, val)
						argCount++
					}
					argCount--
				default:
					if strings.Contains(key, "_id") || key == "guid" {
						if tableSlug == "client_type" {
							filter += " AND guid = ANY($1::uuid[]) "

							args = append(args, pq.Array(cast.ToStringSlice(val)))
						} else {
							if val == nil {
								filter += fmt.Sprintf(" AND %s is null ", key)
								argCount -= 1
							} else {
								filter += fmt.Sprintf(" AND %s = $%d ", key, argCount)
								args = append(args, val)
							}
						}
					} else {
						typeOfVal := reflect.TypeOf(val)
						if typeOfVal.Kind() == reflect.String {
							valString := val.(string)
							if strings.HasPrefix(valString, "+") {
								valString = strings.TrimPrefix(valString, "+")
								val = valString
							}
						}
						filter += fmt.Sprintf(" AND %s ~* $%d ", key, argCount)
						args = append(args, val)

					}
				}

				argCount++
			}
		}
	}

	if len(searchValue) > 0 {
		for idx, val := range req.SearchFields {
			if idx == 0 {
				filter += " AND ("
				searchCondition = ""
			} else {
				searchCondition = " OR "
			}
			filter += fmt.Sprintf(" %s %s ~* $%d ", searchCondition, val, argCount)
			args = append(args, searchValue)
			argCount++

			if idx == len(req.SearchFields)-1 {
				filter += " ) "
			}
		}
	}

	countQuery += filter
	query += filter + order + limit + offset

	var result []map[string]any

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	skipFields := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
	}

	withRelations := cast.ToBool(params["with_relations"])
	if withRelations {
		relationQuery := `
			SELECT
				id,
				table_from,
				table_to,
				field_from,
				type
			FROM
				relation
			WHERE  table_from = $1 OR table_to = $1 `

		relRows, err := conn.Query(ctx, relationQuery, tableSlug)
		if err != nil {
			return nil, 0, err
		}
		defer relRows.Close()

		for relRows.Next() {
			var relation models.Relation

			err := relRows.Scan(
				&relation.Id,
				&relation.TableFrom,
				&relation.TableTo,
				&relation.FieldFrom,
				&relation.Type,
			)
			if err != nil {
				return nil, 0, err
			}

			if relation.Type == config.MANY2MANY || relation.Type == config.MANY2DYNAMIC || relation.Type == config.RECURSIVE {
				continue
			}

			relations = append(relations, relation)
		}
	}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, 0, err
		}

		data := make(map[string]any, len(values))

		for i, value := range values {
			fieldName := string(rows.FieldDescriptions()[i].Name)

			if skipFields[fieldName] {
				continue
			}
			if strings.Contains(fieldName, "_id") || fieldName == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = ConvertGuid(arr)
				}

				if tableSlug == "client_type" {
					if arr, ok := value.([]any); ok {
						ids := []any{}
						for _, a := range arr {
							ids = append(ids, ConvertGuid(a.([16]uint8)))
						}

						value = ids
					}
				}
			}
			data[fieldName] = value
		}

		if len(relations) > 0 {
			for _, relation := range relations {
				joinId := cast.ToString(data[relation.FieldFrom])
				if _, ok := relationMap[joinId]; ok {
					data[relation.FieldFrom+"_data"] = relationMap[joinId]
					continue
				}
				relationData, err := GetItem(ctx, conn, relation.TableTo, joinId, false)
				if err != nil {
					return nil, 0, err
				}

				data[relation.FieldFrom+"_data"] = relationData
				relationMap[joinId] = relationData
			}
		}

		result = append(result, data)
	}

	if err = rows.Err(); err != nil {
		return nil, 0, err
	}

	count := 0
	err = conn.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return nil, 0, err
	}

	return result, count, nil
}

func ConvertGuid(arr [16]uint8) string {
	guidString := fmt.Sprintf("%02x%02x%02x%02x-%02x%02x-%02x%02x-%02x%02x-%02x%02x%02x%02x%02x%02x",
		arr[0], arr[1], arr[2], arr[3],
		arr[4], arr[5],
		arr[6], arr[7],
		arr[8], arr[9],
		arr[10], arr[11], arr[12], arr[13], arr[14], arr[15])

	return guidString
}

func Contains(slice []string, val string) bool {
	return slices.Contains(slice, val)
}

func AppendMany2Many(ctx context.Context, conn pgx.Tx, req []map[string]any) error {
	for _, data := range req {
		idTo := cast.ToStringSlice(data["id_to"])
		idTos := []string{}
		idFrom := cast.ToString(data["id_from"])

		query := fmt.Sprintf(`SELECT %s_ids FROM "%s" WHERE guid = $1`, data["table_to"], data["table_from"])

		err := conn.QueryRow(ctx, query, idFrom).Scan(&idTos)
		if err != nil {
			return errors.Wrap(err, "AppendMany2Many")
		}

		for _, id := range idTo {
			if len(idTos) > 0 {
				if !Contains(idTos, id) {
					idTos = append(idTos, id)
				}
			} else {
				idTos = []string{id}
			}
		}

		query = fmt.Sprintf(`UPDATE "%s" SET %s_ids=$1 WHERE guid = $2`, data["table_from"], data["table_to"])
		_, err = conn.Exec(ctx, query, pq.Array(idTos), idFrom)
		if err != nil {
			return errors.Wrap(err, "AppendMany2Many")
		}

		for _, id := range idTo {
			ids := []string{}
			query := fmt.Sprintf(`SELECT %s_ids FROM "%s" WHERE guid = $1`, data["table_from"], data["table_to"])
			err = conn.QueryRow(ctx, query, id).Scan(&ids)
			if err != nil {
				return errors.Wrap(err, "AppendMany2Many")
			}

			if len(ids) > 0 {
				if !Contains(ids, idFrom) {
					ids = append(ids, idFrom)
				}
			} else {
				ids = []string{idFrom}
			}

			query = fmt.Sprintf(`UPDATE "%s" SET %s_ids=$1 WHERE guid = $2`, data["table_to"], data["table_from"])
			_, err = conn.Exec(ctx, query, pq.Array(ids), id)
			if err != nil {
				return errors.Wrap(err, "AppendMany2Many")
			}
		}
	}

	return nil
}

func AddPermissionToFieldv2(ctx context.Context, conn *psqlpool.Pool, fields []models.Field, roleId string, tableSlug string) ([]models.Field, error) {
	var (
		fieldPermissionMap         = make(map[string]models.FieldPermission)
		relationFieldPermissionMap = make(map[string]string)
		fieldIds                   = []string{}
		tableId                    string
		fieldsWithPermissions      = []models.Field{}
	)

	for _, field := range fields {
		var fieldId string
		if strings.Contains(field.Id, "#") {
			query := `SELECT "id" FROM "table" WHERE "slug" = $1`

			err := conn.QueryRow(ctx, query, tableSlug).Scan(&tableId)
			if err != nil {
				return []models.Field{}, err
			}
			relationID := strings.Split(field.Id, "#")[1]

			query = `SELECT "id" FROM "field" WHERE relation_id = $1 AND table_id = $2`

			err = conn.QueryRow(ctx, query, relationID, tableId).Scan(&fieldId)
			if err != nil {
				return []models.Field{}, err
			}

			if fieldId != "" {
				relationFieldPermissionMap[relationID] = fieldId
				fieldIds = append(fieldIds, fieldId)
				continue
			}
		} else {
			fieldIds = append(fieldIds, field.Id)
		}
	}

	if len(fieldIds) > 0 {
		query := `SELECT
			"guid",
			"role_id",
			"label",
			"table_slug",
			"field_id",
			"edit_permission",
			"view_permission"
		FROM "field_permission" WHERE field_id IN ($1) AND role_id = $2 AND table_slug = $3`

		rows, err := conn.Query(ctx, query, pq.Array(fieldIds), roleId, tableSlug)
		if err != nil {
			return []models.Field{}, err
		}
		defer rows.Close()

		for rows.Next() {
			var fp models.FieldPermission

			err = rows.Scan(
				&fp.Guid,
				&fp.RoleId,
				&fp.Label,
				&fp.TableSlug,
				&fp.FieldId,
				&fp.EditPermission,
				&fp.ViewPermission,
			)
			if err != nil {
				return []models.Field{}, err
			}

			fieldPermissionMap[fp.FieldId] = fp
		}
	}

	for _, field := range fields {
		id := field.Id
		if strings.Contains(id, "#") {
			id = relationFieldPermissionMap[strings.Split(id, "#")[1]]
		}
		fieldPer, ok := fieldPermissionMap[id]

		if ok && roleId != "" {
			if field.Attributes != nil {
				decoded := make(map[string]any)
				body, err := json.Marshal(field.Attributes)
				if err != nil {
					return []models.Field{}, err
				}
				if err := json.Unmarshal(body, &decoded); err != nil {
					return []models.Field{}, err
				}
				decoded["field_permission"] = fieldPer
				newAtb, err := ConvertMapToStruct(decoded)
				if err != nil {
					return []models.Field{}, err
				}
				field.Attributes = newAtb
			} else {
				atributes := map[string]any{
					"field_permission": fieldPer,
				}

				newAtb, err := ConvertMapToStruct(atributes)
				if err != nil {
					return []models.Field{}, err
				}

				field.Attributes = newAtb
			}
			if !fieldPer.ViewPermission {
				continue
			}
			fieldsWithPermissions = append(fieldsWithPermissions, field)
		} else if roleId == "" {
			fieldsWithPermissions = append(fieldsWithPermissions, field)
		}
	}

	return fieldsWithPermissions, nil
}

func CallJS(value string) (string, error) {
	/*
		local
		 cmd := exec.Command("node", "./pkg/js_parser/frontend_formula.js", value)
	*/
	cmd := exec.Command("node", "/js/pkg/js_parser/frontend_formula.js", value)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(output))

	return result, nil
}

func IsEmpty(value any) bool {
	if value == nil {
		return true
	}

	v := reflect.ValueOf(value)
	switch v.Kind() {
	case reflect.String, reflect.Array:
		return v.Len() == 0
	case reflect.Map, reflect.Slice, reflect.Chan:
		return v.Len() == 0 || v.IsNil()
	case reflect.Ptr, reflect.Interface:
		return v.IsNil() || IsEmpty(v.Elem().Interface())
	case reflect.Struct:
		return reflect.DeepEqual(value, reflect.Zero(v.Type()).Interface())
	case reflect.Bool:
		return false
	case reflect.Int, reflect.Float32, reflect.Float64:
		return false
	default:
		return reflect.DeepEqual(value, reflect.Zero(v.Type()).Interface())
	}
}

func ConvertTimestamp2DB(timestamp string) string {
	layouts := []string{
		"2006-01-02T15:04:05Z",
		"02.01.2006 15:04",
	}

	var parsedTime time.Time
	var err error

	for _, layout := range layouts {
		parsedTime, err = time.Parse(layout, timestamp)
		if err == nil {
			break
		}
	}

	if err != nil {
		return ""
	}

	return parsedTime.Format("2006-01-02 15:04:05.000000")
}

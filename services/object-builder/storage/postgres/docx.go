package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

func (o *objectBuilderRepo) GetListForDocxMultiTables(ctx context.Context, req *nb.CommonForDocxMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx.GetListForDocxMultiTables")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	params, _ := helper.ConvertStructToMap(req.Data)

	query := "WITH combined_data AS ("
	tableOrderBy := false
	fields := make(map[string]map[string]any)
	searchFields := make(map[string][]string)
	tableSubqueries := make([]string, len(req.GetTableSlugs()))

	for i, tableSlug := range req.GetTableSlugs() {
		fquery := `SELECT f.slug, f.type, t.order_by, f.is_search 
                   FROM field f 
                   JOIN "table" t ON t.id = f.table_id 
                   WHERE t.slug = $1`
		fieldRows, err := conn.Query(ctx, fquery, tableSlug)
		if err != nil {
			return &nb.CommonMessage{}, err
		}
		defer fieldRows.Close()

		fields[tableSlug] = make(map[string]any)
		searchFields[tableSlug] = []string{}

		tableSubqueries[i] = "SELECT jsonb_build_object("
		for fieldRows.Next() {
			var (
				slug, ftype string
				isSearch    bool
			)

			err := fieldRows.Scan(&slug, &ftype, &tableOrderBy, &isSearch)
			if err != nil {
				return &nb.CommonMessage{}, err
			}

			tableSubqueries[i] += fmt.Sprintf(`'%s', %s.%s,`, slug, tableSlug, slug)
			fields[tableSlug][slug] = ftype

			if helper.FIELD_TYPES[ftype] == "VARCHAR" && isSearch {
				searchFields[tableSlug] = append(searchFields[tableSlug], slug)
			}
		}

		if cast.ToBool(params["with_relations"]) {
			for j, slug := range req.GetTableSlugs() {
				as := fmt.Sprintf("r%d", j+1)
				tableSubqueries[i] += fmt.Sprintf(`'%s_id_data', (
                    SELECT row_to_json(%s)
                    FROM %s %s WHERE %s.guid = %s.%s_id
                ),`, slug, as, slug, as, as, tableSlug, slug)
			}
		}

		tableSubqueries[i] += fmt.Sprintf(`'table_slug', '%s'`, tableSlug)
		tableSubqueries[i] += fmt.Sprintf(`) AS data from %s`, tableSlug)
	}

	query += strings.Join(tableSubqueries, " UNION ALL ") + ")"

	query += " SELECT DISTINCT data FROM combined_data WHERE 1=1"

	filter := ""
	limit := " LIMIT 200"
	offset := " OFFSET 0"
	args := []any{}
	argCount := 1

	for key, val := range params {
		for _, tableSlug := range req.GetTableSlugs() {
			if _, ok := fields[tableSlug][key]; ok {
				switch val.(type) {
				case []string:
					filter += fmt.Sprintf(" AND %s.%s IN($%d) ", tableSlug, key, argCount)
					args = append(args, pq.Array(val))
				case int, float32, float64, int32:
					filter += fmt.Sprintf(" AND %s.%s = $%d ", tableSlug, key, argCount)
					args = append(args, val)
				case []any:
					if fields[tableSlug][key] == "MULTISELECT" {
						filter += fmt.Sprintf(" AND %s.%s && $%d", tableSlug, key, argCount)
						args = append(args, pq.Array(val))
					} else {
						filter += fmt.Sprintf(" AND %s.%s = ANY($%d) ", tableSlug, key, argCount)
						args = append(args, pq.Array(val))
					}
				case map[string]any:
					newOrder := cast.ToStringMap(val)
					for k, v := range newOrder {
						switch v.(type) {
						case string:
							if cast.ToString(v) == "" {
								continue
							}
						}
						switch k {
						case "$gt":
							filter += fmt.Sprintf(" AND %s.%s > $%d ", tableSlug, key, argCount)
						case "$gte":
							filter += fmt.Sprintf(" AND %s.%s >= $%d ", tableSlug, key, argCount)
						case "$lt":
							filter += fmt.Sprintf(" AND %s.%s < $%d ", tableSlug, key, argCount)
						case "$lte":
							filter += fmt.Sprintf(" AND %s.%s <= $%d ", tableSlug, key, argCount)
						case "$in":
							filter += fmt.Sprintf(" AND %s.%s::varchar = ANY($%d)", tableSlug, key, argCount)
						}
						args = append(args, val)
						argCount++
					}
				default:
					if strings.Contains(key, "_id") || key == "guid" {
						//filter += fmt.Sprintf(" AND %s.%s = $%d ", tableSlug, key, argCount)
						filter += fmt.Sprintf(" AND data->>'%s' = $%d ", key, argCount)
						args = append(args, val)
					} else {
						filter += fmt.Sprintf(" AND %s.%s ~* $%d ", tableSlug, key, argCount)
						args = append(args, val)
					}
				}
				argCount++
			}
		}
	}

	searchValue := cast.ToString(params["search"])
	if len(searchValue) > 0 {
		for _, tableSlug := range req.GetTableSlugs() {
			for idx, val := range searchFields[tableSlug] {
				if idx == 0 {
					filter += " AND ("
				} else {
					filter += " OR "
				}
				filter += fmt.Sprintf(" %s.%s ~* $%d ", tableSlug, val, argCount)
				args = append(args, searchValue)
				argCount++
			}
		}
		filter += " ) "
	}

	query += filter + limit + offset

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer rows.Close()

	result := make(map[string]any)
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return &nb.CommonMessage{}, err
		}

		for _, value := range values {
			res, _ := helper.ConvertMapToStruct(value.(map[string]any))
			for j, val := range value.(map[string]any) {
				if j == "table_slug" {
					if arr, ok := result[val.(string)]; ok {
						arr = append(arr.([]any), res)
						result[val.(string)] = arr
					} else {
						result[val.(string)] = []any{res}
					}
					break
				}
			}
		}
	}

	response, _ := helper.ConvertMapToStruct(result)

	return &nb.CommonMessage{
		Data: response,
	}, nil
}

func (o *objectBuilderRepo) GetListForDocx(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx.GetListForDocx")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	params, _ := helper.ConvertStructToMap(req.Data)

	fquery := `SELECT f.slug, f.type, t.order_by, f.is_search FROM field f JOIN "table" t ON t.id = f.table_id WHERE t.slug = $1`
	query := `SELECT jsonb_build_object( `

	tableSlugs := []string{}
	tableOrderBy := false
	fields := make(map[string]any)
	searchFields := []string{}

	fieldRows, err := conn.Query(ctx, fquery, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			slug, ftype string
			isSearch    bool
		)

		err := fieldRows.Scan(&slug, &ftype, &tableOrderBy, &isSearch)
		if err != nil {
			return &nb.CommonMessage{}, err
		}

		query += fmt.Sprintf(`'%s', a.%s,`, slug, slug)
		fields[slug] = ftype

		if strings.Contains(slug, "_id") && !strings.Contains(slug, req.TableSlug) && ftype == "LOOKUP" {
			tableSlugs = append(tableSlugs, strings.ReplaceAll(slug, "_id", ""))
		}

		if helper.FIELD_TYPES[ftype] == "VARCHAR" && isSearch {
			searchFields = append(searchFields, slug)
		}
	}

	_, ok := params["with_relations"]

	if cast.ToBool(params["with_relations"]) || !ok {

		for i, slug := range tableSlugs {

			as := fmt.Sprintf("r%d", i+1)

			query += fmt.Sprintf(`'%s_id_data', (
				SELECT row_to_json(%s)
				FROM %s %s WHERE %s.guid = a.%s_id
			),`, slug, as, slug, as, as, slug)

		}
	}

	query = strings.TrimRight(query, ",")

	query += fmt.Sprintf(`) AS DATA FROM %s a`, req.TableSlug)

	var (
		filter          = " WHERE 1=1 "
		limit           = " LIMIT 20 "
		offset          = " OFFSET 0"
		order           = " ORDER BY a.created_at DESC "
		args            = []any{}
		argCount        = 1
		searchCondition string
	)

	if !tableOrderBy {
		order = " ORDER BY a.created_at ASC "
	}

	for key, val := range params {
		if key == "limit" {
			limit = fmt.Sprintf(" LIMIT %d ", cast.ToInt(val))
		} else if key == "offset" {
			offset = fmt.Sprintf(" OFFSET %d ", cast.ToInt(val))
		} else if key == "order" {
			orders := cast.ToStringMap(val)
			counter := 0

			if len(orders) > 0 {
				order = " ORDER BY "
			}

			for k, v := range orders {
				if k == "created_at" {
					continue
				}
				oType := " ASC"
				if cast.ToInt(v) == -1 {
					oType = " DESC"
				}

				if counter == 0 {
					order += fmt.Sprintf(" a.%s"+oType, k)
				} else {
					order += fmt.Sprintf(", a.%s"+oType, k)
				}
				counter++
			}
		} else {
			_, ok := fields[key]

			if ok {
				switch val.(type) {
				case []string:
					filter += fmt.Sprintf(" AND a.%s IN($%d) ", key, argCount)
					args = append(args, pq.Array(val))
				case int, float32, float64, int32:
					filter += fmt.Sprintf(" AND a.%s = $%d ", key, argCount)
					args = append(args, val)
				case []any:
					if fields[key] == "MULTISELECT" {
						filter += fmt.Sprintf(" AND a.%s && $%d", key, argCount)
						args = append(args, pq.Array(val))
					} else {
						filter += fmt.Sprintf(" AND a.%s = ANY($%d) ", key, argCount)
						args = append(args, pq.Array(val))
					}
				case map[string]any:
					newOrder := cast.ToStringMap(val)

					for k, v := range newOrder {
						switch v.(type) {
						case string:
							if cast.ToString(v) == "" {
								continue
							}
						}

						switch k {
						case "$gt":
							filter += fmt.Sprintf(" AND a.%s > $%d ", key, argCount)
						case "$gte":
							filter += fmt.Sprintf(" AND a.%s >= $%d ", key, argCount)
						case "$lt":
							filter += fmt.Sprintf(" AND a.%s < $%d ", key, argCount)
						case "$lte":
							filter += fmt.Sprintf(" AND a.%s <= $%d ", key, argCount)
						case "$in":
							filter += fmt.Sprintf(" AND a.%s::varchar = ANY($%d)", key, argCount)
						}

						args = append(args, val)

						argCount++
					}
				default:
					if strings.Contains(key, "_id") || key == "guid" {
						if req.TableSlug == "client_type" {
							filter += " AND a.guid = ANY($1::uuid[]) "

							args = append(args, pq.Array(cast.ToStringSlice(val)))
						} else {
							filter += fmt.Sprintf(" AND a.%s = $%d ", key, argCount)
							args = append(args, val)
						}
					} else {
						filter += fmt.Sprintf(" AND a.%s ~* $%d ", key, argCount)
						args = append(args, val)
					}
				}

				argCount++
			}
		}
	}

	searchValue := cast.ToString(params["search"])
	if len(searchValue) > 0 {
		for idx, val := range searchFields {
			if idx == 0 {
				filter += " AND ("
				searchCondition = ""
			} else {
				searchCondition = " OR "
			}
			filter += fmt.Sprintf(" %s a.%s ~* $%d ", searchCondition, val, argCount)
			args = append(args, searchValue)
			argCount++

			if idx == len(searchFields)-1 {
				filter += " ) "
			}
		}
	}

	// countQuery += filter
	query += filter + order + limit + offset

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer rows.Close()

	result := []any{}

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return &nb.CommonMessage{}, err
		}

		var (
			data any
			temp = make(map[string]any)
		)

		for i, value := range values {
			temp[rows.FieldDescriptions()[i].Name] = value
			data = temp["data"]
		}

		result = append(result, data)
	}

	rr := map[string]any{
		"response": result,
	}

	response, _ := helper.ConvertMapToStruct(rr)

	return &nb.CommonMessage{
		Data: response,
	}, nil
}

func (o *objectBuilderRepo) GetAllForDocx(ctx context.Context, req *nb.CommonMessage) (resp map[string]any, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx.GetAllForDocx")
	defer dbSpan.Finish()

	var (
		params = make(map[string]any)
	)

	conn, err := psqlpool.Get(req.ProjectId)
	if err != nil {
		return nil, err
	}

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(paramBody, &params); err != nil {
		return nil, err
	}

	additionalFields := cast.ToStringMap(params["additional_fields"])

	delete(params, "additional_fields")

	response := map[string]any{}

	for key, value := range additionalFields {
		additionalItem, err := helper.GetItem(ctx, conn, strings.TrimSuffix(key, "_id"), cast.ToString(value), false)
		if err != nil {
			return nil, errors.Wrap(err, "error while getting additional item")
		}
		response[key+"_data"] = additionalItem
	}

	return response, nil
}

func (o *objectBuilderRepo) GetAllFieldsForDocx(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "docx.GetAllFieldsForDocx")
	defer dbSpan.Finish()
	var (
		fields = []models.Field{}
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `select f.table_id, f.label, f.slug from field f join "table" t on t.id = f.table_id where t.slug = $1`

	rows, err := conn.Query(ctx, query, req.GetTableSlug())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var field = models.Field{}

		if err = rows.Scan(&field.TableId, &field.Label, &field.Slug); err != nil {
			return nil, err
		}

		fields = append(fields, field)
	}

	item := map[string]any{
		"fields":     fields,
		"relations:": []any{},
	}

	res, err := helper.ConvertMapToStruct(item)
	if err != nil {
		return nil, err
	}

	return &nb.CommonMessage{
		TableSlug: req.GetTableSlug(),
		Data:      res,
	}, nil
}

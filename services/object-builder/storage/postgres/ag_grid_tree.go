package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/spf13/cast"
)

type QueryContext struct {
	TableSlug     string
	Fields        []string
	LookupFields  []string
	RelatedTables []string
	FilterValue   string
	Limit         int
	Offset        int
	Autofilter    map[string]any
}

func (o *objectBuilderRepo) AgGridTree(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "AgGridTree: Failed to get database connection")
	}

	fields, filterValue, autoFilter, limit, offset, err := parseAndValidateRequest(ctx, conn, req)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "AgGridTree: Failed to parse request")
	}

	lookupFields, relatedTables, err := getLookupFields(ctx, conn, fields, req.TableSlug)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "AgGridTree: Failed to get lookup fields")
	}

	qc := QueryContext{
		TableSlug:     req.TableSlug,
		Fields:        fields,
		LookupFields:  lookupFields,
		RelatedTables: relatedTables,
		FilterValue:   filterValue,
		Limit:         limit,
		Offset:        offset,
		Autofilter:    autoFilter,
	}

	results, err := buildAndExecuteQuery(ctx, conn, qc)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "AgGridTree: Failed to build and execute query")
	}

	response := map[string]any{"response": results}
	respData, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "AgGridTree: Failed to convert response")
	}

	return &nb.CommonMessage{
		TableSlug: req.TableSlug,
		ProjectId: req.ProjectId,
		Data:      respData,
	}, nil
}

func parseAndValidateRequest(ctx context.Context, conn *psqlpool.Pool, req *nb.CommonMessage) ([]string, string, map[string]any, int, int, error) {
	var params map[string]any

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return nil, "", nil, 0, 0, err
	}
	if err := json.Unmarshal(paramBody, &params); err != nil {
		return nil, "", nil, 0, 0, err
	}

	fields, ok := params["fields"].([]any)
	if !ok {
		return nil, "", nil, 0, 0, fmt.Errorf("fields not found or invalid")
	}

	userIdFromToken := cast.ToString(params["user_id_from_token"])
	roleIdFromToken := cast.ToString(params["role_id_from_token"])
	params, err = helper.GetAutomaticFilter(ctx, models.GetAutomaticFilterRequest{
		Conn:            conn,
		Params:          params,
		RoleIdFromToken: roleIdFromToken,
		UserIdFromToken: userIdFromToken,
		TableSlug:       req.TableSlug,
	})
	if err != nil {
		return nil, "", nil, 0, 0, fmt.Errorf("failed to get automatic filter: %w", err)
	}

	childField := req.TableSlug + "_id"
	var filterValue string
	if filterValues, ok := params[childField].([]any); ok && len(filterValues) > 0 {
		if filterValues[0] == nil {
			filterValue = ""
		} else {
			val := cast.ToString(filterValues[0])
			filterValue = val
		}
	}

	limit := 100
	if l, ok := params["limit"]; ok {
		limit = cast.ToInt(l)
	}

	offset := 0
	if o, ok := params["offset"]; ok {
		offset = cast.ToInt(o)
	}

	return cast.ToStringSlice(fields), filterValue, params["auto_filter"].(map[string]any), limit, offset, nil
}

func getLookupFields(ctx context.Context, conn *psqlpool.Pool, fields []string, tableSlug string) ([]string, []string, error) {
	const fieldQuery = `
		SELECT
			f.slug,
			COALESCE(r.table_to::varchar, ''),
			COALESCE(f.autofill_table, '')
		FROM field f 
		JOIN "table" t ON t.id = f.table_id 
		LEFT JOIN "relation" r ON r.id = f.relation_id
		WHERE t.slug = $1 AND f.type IN ('LOOKUP', 'LOOKUPS') AND f.slug = ANY($2)`

	rows, err := conn.Query(ctx, fieldQuery, tableSlug, pq.Array(fields))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var lookupFields, relatedTables []string

	for rows.Next() {
		var slug, relatedTable, autofillTable string
		if err := rows.Scan(&slug, &relatedTable, &autofillTable); err != nil {
			return nil, nil, err
		}

		if relatedTable == "" && autofillTable != "" {
			relatedTable = strings.Split(autofillTable, "#")[0]
		}
		if relatedTable == "" && strings.HasSuffix(slug, "_id") {
			relatedTable = strings.TrimSuffix(slug, "_id")
		}
		if relatedTable == "" {
			continue
		}

		lookupFields = append(lookupFields, slug)
		relatedTables = append(relatedTables, relatedTable)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	return lookupFields, relatedTables, nil
}

func buildAndExecuteQuery(ctx context.Context, conn *psqlpool.Pool, qc QueryContext) ([]map[string]any, error) {
	query := buildRecursiveQuery(qc)
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	var results []map[string]any
	for rows.Next() {
		row, err := processRow(rows, columns)
		if err != nil {
			return nil, err
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func buildRecursiveQuery(qc QueryContext) string {
	childField := qc.TableSlug + "_id"
	baseSelect := buildSelectClause("parent_node", qc)
	recursiveSelect := buildSelectClause("child_node", qc)

	var lookupDataSelects strings.Builder
	for _, field := range qc.LookupFields {
		lookupDataSelects.WriteString(fmt.Sprintf(", h.%s_data", field))
	}

	var baseFilterCondition string
	if qc.FilterValue == "" {
		baseFilterCondition = fmt.Sprintf("parent_node.%s IS NULL", childField)
	} else {
		baseFilterCondition = fmt.Sprintf("parent_node.%s = '%s'", childField, qc.FilterValue)
	}

	query := fmt.Sprintf(`
        WITH RECURSIVE hierarchy AS (
            SELECT 
                %s,
                ARRAY[parent_node.guid] AS path,
                EXISTS (
                    SELECT 1 FROM %s child 
                    WHERE child.%s = parent_node.guid
                ) AS has_child,
                parent_node.%s AS original_parent_id,
				parent_node.created_at
            FROM %s parent_node
            WHERE %s

            UNION ALL

            SELECT 
                %s,
                parent.path || child_node.guid AS path,
                EXISTS (
                    SELECT 1 FROM %s grandchild 
                    WHERE grandchild.%s = child_node.guid
                ) AS has_child,
                parent.original_parent_id,
				child_node.created_at
            FROM %s child_node
            INNER JOIN hierarchy parent ON child_node.%s = parent.guid
        )
        SELECT %s, h.path, h.has_child%s FROM hierarchy h
        WHERE h.%s %s
		ORDER BY h.created_at ASC
		LIMIT %d OFFSET %d`,
		baseSelect,
		qc.TableSlug,
		childField,
		childField,
		qc.TableSlug,
		baseFilterCondition,
		recursiveSelect,
		qc.TableSlug,
		childField,
		qc.TableSlug,
		childField,
		joinColumnsWithPrefix(qc.Fields, "h."),
		lookupDataSelects.String(),
		childField,
		ifElse(qc.FilterValue == "", "IS NULL", fmt.Sprintf("= '%s'", qc.FilterValue)),
		qc.Limit,
		qc.Offset,
	)

	return query
}

func ifElse(condition bool, trueVal, falseVal string) string {
	if condition {
		return trueVal
	}
	return falseVal
}

func buildSelectClause(prefix string, qc QueryContext) string {
	var sb strings.Builder
	sb.WriteString(joinColumnsWithPrefix(qc.Fields, prefix+"."))

	for i, field := range qc.LookupFields {
		relationAlias := fmt.Sprintf("related_table%d", i+1)
		sb.WriteString(fmt.Sprintf(`, 
            (SELECT row_to_json(%s) 
             FROM "%s" %s 
             WHERE %s.guid = %s.%s
            ) AS %s_data`,
			relationAlias,
			qc.RelatedTables[i],
			relationAlias,
			relationAlias,
			prefix,
			field,
			field))
	}

	return sb.String()
}

func processRow(rows pgx.Rows, columns []string) (map[string]any, error) {
	values, err := rows.Values()
	if err != nil {
		return nil, err
	}

	rowMap := make(map[string]any)
	for i, colName := range columns {
		val := values[i]
		switch {
		case val == nil:
			rowMap[colName] = nil
		case (colName == config.GUID || strings.Contains(colName, config.ID)) && !strings.Contains(colName, "_id_data"):
			if arr, ok := val.([16]byte); ok {
				rowMap[colName] = helper.ConvertGuid(arr)
			}
		case colName == config.PATH:
			if arr, ok := val.([]any); ok {
				rowMap[colName] = convertGuidPath(arr)
			}
		case colName == "has_child":
			if b, ok := val.(bool); ok {
				rowMap[colName] = b
			}
		default:
			rowMap[colName] = val
		}
	}
	return rowMap, nil
}

func convertGuidPath(path []any) []string {
	var guidList []string
	for _, guid := range path {
		if guidArr, ok := guid.([16]byte); ok {
			guidList = append(guidList, helper.ConvertGuid(guidArr))
		}
	}
	return guidList
}

func joinColumnsWithPrefix(columns []string, prefix string) string {
	prefixed := make([]string, len(columns))
	for i, col := range columns {
		prefixed[i] = prefix + col
	}
	return strings.Join(prefixed, ", ")
}

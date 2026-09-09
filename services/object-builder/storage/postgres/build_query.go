package postgres

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"

	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type QueryBuilder struct {
	query            string
	filter           string
	order            string
	limit            string
	offset           string
	autoFilters      string
	args             []any
	argCount         int
	fields           map[string]string
	tableSlugs       []string
	tableSlugsTable  []string
	searchFields     []string
	isCached         bool
	additionalField  string
	additionalValues []any
}

func (qb *QueryBuilder) finalizeQuery(tableSlug string) string {
	qb.query = strings.TrimRight(qb.query, ",")

	if len(qb.additionalField) == 0 || len(qb.additionalValues) == 0 {
		qb.query += fmt.Sprintf(`) AS DATA FROM "%s" a`, tableSlug)
		return qb.query + qb.filter + qb.autoFilters + qb.order + qb.limit + qb.offset
	}

	return qb.buildAdditionalValues(tableSlug)
}

func (qb *QueryBuilder) buildAdditionalValues(tableSlug string) string {
	baseSelect := qb.query + `) AS DATA FROM "` + tableSlug + `" a`

	requiredPart := fmt.Sprintf(`
        %s
        WHERE a.deleted_at IS NULL
        AND a.%s = ANY($%d)`,
		baseSelect,
		qb.additionalField,
		qb.argCount,
	)

	otherPart := fmt.Sprintf(`
        %s
        %s
        AND a.%s != ANY($%d)
        %s%s%s%s`,
		baseSelect,
		qb.filter,
		qb.additionalField,
		qb.argCount,
		qb.autoFilters,
		qb.order,
		qb.limit,
		qb.offset,
	)

	finalQuery := fmt.Sprintf("(%s) UNION ALL (%s)", requiredPart, otherPart)

	qb.args = append(qb.args, qb.additionalValues)
	qb.argCount++

	return finalQuery
}

// buildDefaultFilter handles default filter cases
func (qb *QueryBuilder) buildDefaultFilter(key string, val any) {
	if strings.Contains(key, "_id") || key == "guid" {
		if key == "client_type" {
			qb.filter += " AND a.guid = ANY($1::uuid[]) "
			qb.args = append(qb.args, pq.Array(cast.ToStringSlice(val)))
		} else {
			qb.filter += fmt.Sprintf(" AND a.%s = $%d ", key, qb.argCount)
			qb.args = append(qb.args, val)
			qb.argCount++
		}
	} else {
		val = escapeSpecialCharacters(cast.ToString(val))
		qb.filter += fmt.Sprintf(" AND a.%s ~* $%d ", key, qb.argCount)
		qb.args = append(qb.args, val)
		qb.argCount++
	}
}

// buildSearchFilter adds search functionality to the  query
func (qb *QueryBuilder) buildSearchFilter(searchValue string) {
	if len(searchValue) == 0 {
		return
	}

	searchValue = escapeSpecialCharacters(searchValue)
	for idx, val := range qb.searchFields {
		if idx == 0 {
			qb.filter += " AND ("
		}
		if idx > 0 {
			qb.filter += " OR "
		}
		qb.filter += fmt.Sprintf(" a.%s ~* $%d ", val, qb.argCount)
		qb.args = append(qb.args, searchValue)
		qb.argCount++

		if idx == len(qb.searchFields)-1 {
			qb.filter += " ) "
		}
	}
}

// NewQueryBuilder initializes a new QueryBuilder with default values
func NewQueryBuilder() *QueryBuilder {
	return &QueryBuilder{
		query:    `SELECT jsonb_build_object( `,
		filter:   " WHERE deleted_at IS NULL ",
		limit:    " LIMIT 20 ",
		offset:   " OFFSET 0",
		order:    " ORDER BY a.created_at DESC ",
		argCount: 1,
		fields:   make(map[string]string),
	}
}

// buildFieldQuery processes field rows and builds the field part of the query
func (qb *QueryBuilder) buildFieldQuery(fieldRows pgx.Rows) error {
	var (
		counter = 0

		hasCreatedAt = false
		hasUpdatedAt = false
	)

	for fieldRows.Next() {
		var (
			slug, ftype                      string
			tableOrderBy, isSearch, isCached bool
		)

		if err := fieldRows.Scan(&slug, &ftype, &tableOrderBy, &isSearch, &isCached); err != nil {
			return errors.Wrap(err, "error while scanning fields")
		}

		if slug == "created_at" {
			hasCreatedAt = true
		}
		if slug == "updated_at" {
			hasUpdatedAt = true
		}

		qb.isCached = isCached

		// Handle special datetime fields
		if ftype == "DATE_TIME_WITHOUT_TIME_ZONE" {
			qb.query += fmt.Sprintf(`'%s', TO_CHAR(a.%s, 'DD.MM.YYYY HH24:MI'),`, slug, slug)
			qb.fields[slug] = ftype
			continue
		}
		if ftype == "DATE_TIME" {
			qb.query += fmt.Sprintf(`'%s', TO_CHAR(a.%s AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS"Z"'),`, slug, slug)
			qb.fields[slug] = ftype
			continue
		}

		// Reset counter and start new object if needed
		if counter >= 30 {
			qb.query = strings.TrimRight(qb.query, ",")
			qb.query += `) || jsonb_build_object( `
			counter = 0
		}

		qb.query += fmt.Sprintf(`'%s', a.%s,`, slug, slug)
		qb.fields[slug] = ftype

		// Handle lookup fields
		if strings.Contains(slug, "_id") && !strings.Contains(slug, "table_slug") && ftype == "LOOKUP" {
			qb.handleLookupField(slug)
		}

		// Add search fields
		if helper.FIELD_TYPES[ftype] == "VARCHAR" && isSearch {
			qb.searchFields = append(qb.searchFields, slug)
		}

		counter++

		if tableOrderBy {
			qb.order = " ORDER BY a.created_at ASC "
		}
	}

	extraFields := []string{"created_at", "updated_at"}

	for _, slug := range extraFields {
		if slug == "created_at" && hasCreatedAt {
			continue
		}
		if slug == "updated_at" && hasUpdatedAt {
			continue
		}

		if counter >= 30 {
			qb.query = strings.TrimRight(qb.query, ",")
			qb.query += `) || jsonb_build_object( `
			counter = 0
		}

		qb.query += fmt.Sprintf(`'%s', a.%s,`, slug, slug)
		qb.fields[slug] = "timestamp"

		counter++
	}

	return nil
}

// handleLookupField processes lookup fields and updates table slugs
func (qb *QueryBuilder) handleLookupField(slug string) {
	qb.tableSlugs = append(qb.tableSlugs, slug)
	parts := strings.Split(slug, "_")
	if len(parts) > 2 {
		lastPart := parts[len(parts)-1]
		if _, err := strconv.Atoi(lastPart); err == nil {
			slug = strings.ReplaceAll(slug, fmt.Sprintf("_%v", lastPart), "")
		}
	}
	qb.tableSlugsTable = append(qb.tableSlugsTable, strings.ReplaceAll(slug, "_id", ""))
}

// buildRelationsQuery adds relation data to the query
func (qb *QueryBuilder) buildRelationsQuery() {
	qb.query = strings.TrimRight(qb.query, ",")
	qb.query += `) || jsonb_build_object( `

	for i, slug := range qb.tableSlugs {
		as := fmt.Sprintf("r%d", i+1)
		qb.query += fmt.Sprintf(`'%s_data', (
			SELECT row_to_json(%s)
			FROM "%s" %s WHERE %s.guid = a.%s
		),`, slug, as, qb.tableSlugsTable[i], as, as, slug)
	}
}

// applyFilters processes and applies filters from parameters
func (qb *QueryBuilder) applyFilters(params map[string]any) {
	for key, val := range params {
		switch key {
		case "limit":
			qb.limit = fmt.Sprintf(" LIMIT %d ", cast.ToInt(val))
		case "offset":
			qb.offset = fmt.Sprintf(" OFFSET %d ", cast.ToInt(val))
		case "order":
			qb.buildOrderClause(cast.ToStringMap(val))
		case "auto_filter":
			qb.buildAutoFilters(cast.ToStringMap(val))
		default:
			qb.buildFieldFilter(key, val)
		}
	}
}

// buildOrderClause constructs the ORDER BY clause
func (qb *QueryBuilder) buildOrderClause(orders map[string]any) {
	if len(orders) == 0 {
		return
	}

	qb.order = " ORDER BY "
	counter := 0
	for k, v := range orders {
		if k == "created_at" {
			continue
		}
		oType := " DESC"
		if cast.ToInt(v) == 1 {
			oType = " ASC"
		}

		if counter == 0 {
			qb.order += fmt.Sprintf(" a.%s"+oType, k)
		} else {
			qb.order += fmt.Sprintf(", a.%s"+oType, k)
		}
		counter++
	}
}

// buildAutoFilters constructs automatic filters
func (qb *QueryBuilder) buildAutoFilters(filters map[string]any) {
	var counter int
	for k, v := range filters {
		if counter == 0 {
			qb.autoFilters += fmt.Sprintf(" AND (a.%s = $%d", k, qb.argCount)
		} else {
			qb.autoFilters += fmt.Sprintf(" OR a.%s = $%d", k, qb.argCount)
		}

		if counter == len(filters)-1 {
			qb.autoFilters += " )"
		}

		qb.args = append(qb.args, v)
		qb.argCount++
		counter++
	}
}

// buildFieldFilter constructs field-specific filters
func (qb *QueryBuilder) buildFieldFilter(key string, val any) {
	var (
		fieldType string
		ok        bool
	)
	if fieldType, ok = qb.fields[key]; !ok {
		return
	}

	switch valTyped := val.(type) {
	case []string:
		qb.filter += fmt.Sprintf(" AND a.%s IN($%d) ", key, qb.argCount)
		qb.args = append(qb.args, pq.Array(valTyped))
		qb.argCount++
	case int, float32, float64, int32, bool:
		qb.filter += fmt.Sprintf(" AND a.%s = $%d ", key, qb.argCount)
		qb.args = append(qb.args, valTyped)
		qb.argCount++
	case []any:
		if qb.fields[key] == "MULTISELECT" {
			qb.filter += fmt.Sprintf(" AND a.%s && $%d", key, qb.argCount)
		} else {
			qb.filter += fmt.Sprintf(" AND a.%s = ANY($%d) ", key, qb.argCount)
		}
		qb.args = append(qb.args, pq.Array(valTyped))
		qb.argCount++
	case map[string]any:
		qb.buildComparisonFilters(key, valTyped)
	default:
		if helper.NUMERIC_TYPES[fieldType] {
			qb.filter += fmt.Sprintf(" AND a.%s = $%d ", key, qb.argCount)
			qb.args = append(qb.args, val)
			qb.argCount++
		} else {
			qb.buildDefaultFilter(key, val)
		}
	}
}

// buildComparisonFilters handles comparison operators in filters
func (qb *QueryBuilder) buildComparisonFilters(key string, comparisons map[string]any) {
	for op, v := range comparisons {
		switch op {
		case "$gt":
			qb.filter += fmt.Sprintf(" AND a.%s > $%d ", key, qb.argCount)
		case "$gte":
			qb.filter += fmt.Sprintf(" AND a.%s >= $%d ", key, qb.argCount)
		case "$lt":
			qb.filter += fmt.Sprintf(" AND a.%s < $%d ", key, qb.argCount)
		case "$lte":
			qb.filter += fmt.Sprintf(" AND a.%s <= $%d ", key, qb.argCount)
		case "$in":
			qb.filter += fmt.Sprintf(" AND a.%s::VARCHAR = ANY($%d)", key, qb.argCount)
		}
		qb.args = append(qb.args, v)
		qb.argCount++
	}
}

func escapeSpecialCharacters(input string) string {
	return regexp.QuoteMeta(input)
} 

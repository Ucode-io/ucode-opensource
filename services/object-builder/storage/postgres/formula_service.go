package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type FormulaCalculationService struct {
	conn      *psqlpool.Pool
	tableSlug string
	body      map[string]any
	oldData   map[string]any

	fields                []models.Field
	formulaFields         []models.Field
	formulaFrontendFields []models.Field
}

type FormulaFilter struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

// NewFormulaCalculationService creates a new formula calculation service
func NewFormulaCalculationService(conn *psqlpool.Pool, tableSlug string, body, oldData map[string]any, fields, formulaFronts []models.Field) *FormulaCalculationService {
	service := &FormulaCalculationService{
		conn:                  conn,
		body:                  body,
		oldData:               oldData,
		tableSlug:             tableSlug,
		fields:                fields,
		formulaFields:         make([]models.Field, 0),
		formulaFrontendFields: formulaFronts,
	}

	// Initialize all fields from the database
	service.initializeFields()

	return service
}

// initializeFields loads all fields from the database and caches them
func (f *FormulaCalculationService) initializeFields() {
	ctx := context.Background()

	query := `
		SELECT 
    		f.id,
    		f.attributes,
			f.slug,
			(SELECT slug FROM "table" WHERE id = f.table_id) AS table_slug,
    		f2.slug AS relation_field_slug
		FROM field AS f
		LEFT JOIN field AS f2 
    	ON f2.relation_id = split_part(f.attributes->>'table_from', '#', 2)::UUID
		WHERE split_part(f.attributes->>'table_from', '#', 1) = $1 AND f.type = 'FORMULA';

	`

	rows, err := f.conn.Query(ctx, query, f.tableSlug)
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var (
			field      models.Field
			attributes []byte
			f2Slug     sql.NullString
		)

		err := rows.Scan(
			&field.Id,
			&attributes,
			&field.Slug,
			&field.TableSlug,
			&f2Slug,
		)
		if err != nil {
			continue
		}

		field.RelationFieldSlug = f2Slug.String

		if err := json.Unmarshal(attributes, &field.Attributes); err != nil {
			continue
		}

		f.formulaFields = append(f.formulaFields, field)

	}
}

func (f *FormulaCalculationService) CalculateFormulaFields(ctx context.Context, recordId string) error {
	calculatedValues := make(map[string]any)

	for _, field := range f.formulaFrontendFields {
		attributes, err := helper.ConvertStructToMap(field.Attributes)
		if err != nil {
			return errors.Wrap(err, "failed to convert field attributes")
		}

		value, err := f.calculateFrontendFormula(attributes)
		if err != nil {
			return errors.Wrap(err, "failed to calculate frontend formula")
		}
		calculatedValues[field.Slug] = value
	}

	if len(calculatedValues) > 0 {
		err := f.UpdateRecordWithFormulas(ctx, f.tableSlug, recordId, calculatedValues)
		if err != nil {
			log.Println("UpdateRecordWithFormulas", err.Error())
		}
	}

	return nil
}

func (f *FormulaCalculationService) UpdateRecordWithFormulas(ctx context.Context, tableSlug, recordId string, calculatedValues map[string]any) error {
	if len(calculatedValues) == 0 {
		return nil
	}

	var (
		setClauses []string
		args       []any
		argCount   = 1
	)

	for fieldSlug, value := range calculatedValues {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", fieldSlug, argCount))
		args = append(args, value)
		argCount++
	}

	args = append(args, recordId)
	query := fmt.Sprintf(`
		UPDATE "%s" 
		SET %s 
		WHERE guid = $%d`,
		tableSlug,
		strings.Join(setClauses, ", "),
		argCount,
	)

	_, err := f.conn.Exec(ctx, query, args...)
	if err != nil {
		return errors.Wrap(err, "failed to update record with formula values")
	}

	return nil
}

func (f *FormulaCalculationService) RecalculateAffectedFormulas(ctx context.Context, changedRecordId string) error {
	var (
		relationRowId string
	)

	for _, formulaField := range f.formulaFields {
		var (
			args []any
		)

		attributes, err := helper.ConvertStructToMap(formulaField.Attributes)
		if err != nil {
			return errors.Wrap(err, "error while converting struct to map")
		}

		tableFrom := cast.ToString(attributes["table_from"])
		sumField := cast.ToString(attributes["sum_field"])

		if tableFrom == "" || sumField == "" {
			return nil
		}

		if f.body[sumField] == f.oldData[sumField] {
			continue
		}

		if _, exist := f.body[sumField]; !exist {
			continue
		}

		if rowId, exist := f.body[formulaField.RelationFieldSlug]; exist {
			relationRowId = cast.ToString(rowId)
			num, err := f.calculateBackendFormula(attributes, sumField, relationRowId, formulaField)
			if err != nil {
				continue
			}

			args = append(args, relationRowId, num)

			query := fmt.Sprintf(`
				UPDATE "%s" 
					SET "%s" = $2
				WHERE guid = $1`,
				formulaField.TableSlug,
				formulaField.Slug,
			)

			_, err = f.conn.Exec(ctx, query, args...)
			if err != nil {
				return errors.Wrap(err, "failed to update record with formula values")
			}
		}
	}

	return nil
}

func (f *FormulaCalculationService) RecalculateAffectedFormulasDelete(ctx context.Context, tableSlug, changedRecordId string, changedField string) error {
	var (
		args          []any
		relationRowId string
	)

	for _, formulaField := range f.formulaFields {
		attributes, err := helper.ConvertStructToMap(formulaField.Attributes)
		if err != nil {
			return errors.Wrap(err, "error while converting struct to map")
		}

		tableFrom := cast.ToString(attributes["table_from"])
		sumField := cast.ToString(attributes["sum_field"])

		if tableFrom == "" || sumField == "" {
			return nil
		}

		if rowId, exist := f.oldData[formulaField.RelationFieldSlug]; exist {
			relationRowId = cast.ToString(rowId)
			num, err := f.calculateBackendFormula(attributes, sumField, relationRowId, formulaField)
			if err != nil {
				continue
			}

			args = append(args, relationRowId, num)

			query := fmt.Sprintf(`
				UPDATE "%s" 
					SET "%s" = $2
				WHERE guid = $1`,
				formulaField.TableSlug,
				formulaField.Slug,
			)

			_, err = f.conn.Exec(ctx, query, args...)
			if err != nil {
				return errors.Wrap(err, "failed to update record with formula values")
			}
		}
	}

	return nil
}

// calculateBackendFormula calculates a backend formula value
func (f *FormulaCalculationService) calculateBackendFormula(attributes map[string]any, sumField, recordId string, field models.Field) (float32, error) {

	resp, err := f.CalculateFormulaBackend(attributes, sumField, recordId, field)
	if err != nil {
		return 0, errors.Wrap(err, "failed to calculate formula backend")
	}

	return resp, nil
}

// calculateFrontendFormula calculates a frontend formula value
func (f *FormulaCalculationService) calculateFrontendFormula(attributes map[string]any) (any, error) {
	_, ok := attributes["formula"]
	if !ok {
		return nil, nil
	}

	return f.CalculateFormulaFrontend(attributes)
}

func (f *FormulaCalculationService) CalculateFormulaBackend(attributes map[string]any, sumField, rowId string, field models.Field) (float32, error) {
	var (
		query string

		formulaFilter []FormulaFilter
		num           float32
	)

	round := cast.ToInt(attributes["number_of_rounds"])

	if attributes["formula_filters"] != nil {
		formulaFilterByte, err := json.Marshal(attributes["formula_filters"])
		if err != nil {
			return 0, errors.Wrap(err, "CalculateFormulaBackend - json.Marshal")
		}

		err = json.Unmarshal(formulaFilterByte, &formulaFilter)
		if err != nil {
			return 0, errors.Wrap(err, "CalculateFormulaBackend - json.Unmarshal")
		}
	}

	if sumField == "" {
		sumField = "1"
	}

	formulaFilter = append(formulaFilter, FormulaFilter{Key: field.RelationFieldSlug, Value: rowId})

	whereClause, params, _ := buildWhereClause(formulaFilter)

	switch cast.ToString(attributes["type"]) {
	case "SUMM":
		query = fmt.Sprintf(`SELECT SUM(%s) FROM "%s" WHERE deleted_at IS NULL %s GROUP BY %s`, sumField, f.tableSlug, whereClause, field.RelationFieldSlug)
	case "MAX":
		query = fmt.Sprintf(`SELECT MAX(%s) FROM "%s" WHERE deleted_at IS NULL %s GROUP BY %s`, sumField, f.tableSlug, whereClause, field.RelationFieldSlug)
	case "AVG":
		query = fmt.Sprintf(`SELECT AVG(%s) FROM "%s" WHERE deleted_at IS NULL %s GROUP BY %s`, sumField, f.tableSlug, whereClause, field.RelationFieldSlug)
	}

	err := f.conn.QueryRow(context.Background(), query, params...).Scan(&num)
	if err != nil {
		return 0, errors.Wrap(err, "CalculateFormulaBackend - conn.Query")
	}

	if round > 0 {
		format := "%." + fmt.Sprint(round) + "f"
		num = cast.ToFloat32(fmt.Sprintf(format, num))
	}

	return num, nil
}

func (f *FormulaCalculationService) CalculateFormulaFrontend(attributes map[string]any) (any, error) {
	computedFormula := attributes["formula"].(string)

	sort.Slice(f.fields, func(i, j int) bool {
		return len(f.fields[i].Slug) > len(f.fields[j].Slug)
	})

	for _, el := range f.fields {
		value, ok := f.body[el.Slug]
		if !ok {
			value = 0
		}

		valBytes, err := json.Marshal(value)
		if err != nil {
			valBytes = []byte(fmt.Sprintf(`"%v"`, value))
		}

		computedFormula = strings.ReplaceAll(computedFormula, el.Slug, string(valBytes))
	}

	result, err := helper.CallJS(computedFormula)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func buildWhereClause(filters []FormulaFilter) (string, []any, error) {
	var (
		whereClauses []string
		params       []any
	)

	if len(filters) == 0 {
		return "", params, nil
	}

	for i, filter := range filters {
		keyParts := strings.Split(filter.Key, "#")
		if len(keyParts) == 0 {
			return "", nil, fmt.Errorf("invalid filter key at index %d", i)
		}
		field := keyParts[0]

		switch v := filter.Value.(type) {
		case []any:
			if len(v) == 0 {
				continue
			}
			placeholders := make([]string, len(v))
			for i := range v {
				placeholders[i] = fmt.Sprintf("$%d", len(params)+1+i)
			}
			params = append(params, v...)
			whereClauses = append(whereClauses, fmt.Sprintf("%s IN (%s)", field, strings.Join(placeholders, ",")))
		default:
			// Handle single value
			params = append(params, v)
			whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", field, len(params)))
		}
	}

	return fmt.Sprintf(" AND %s", strings.Join(whereClauses, " AND ")), params, nil
}

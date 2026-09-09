package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
)

var paramRegexp = regexp.MustCompile(`:(\w+)`)

type customEndpointRepo struct {
	db *psqlpool.Pool
}

func NewCustomEndpointRepo(db *psqlpool.Pool) storage.CustomEndpointRepoI {
	return &customEndpointRepo{db: db}
}

func (r *customEndpointRepo) Create(ctx context.Context, req *nb.CreateCustomEndpointRequest) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_endpoint.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	id := uuid.NewString()
	now := time.Now()

	paramsJSON, err := json.Marshal(req.GetParameters())
	if err != nil {
		return nil, fmt.Errorf("marshal parameters: %w", err)
	}

	query := `
		INSERT INTO custom_endpoint (id, name, description, sql_query, method, in_transaction, parameters, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $8)
	`

	_, err = conn.Exec(ctx, query,
		id,
		req.GetName(),
		req.GetDescription(),
		req.GetSql(),
		req.GetMethod(),
		req.GetInTransaction(),
		paramsJSON,
		now,
	)
	if err != nil {
		return nil, fmt.Errorf("custom_endpoint.Create: %w", err)
	}

	return r.GetById(ctx, &nb.CustomEndpointId{
		ResourceEnvId: req.GetResourceEnvId(),
		Id:            id,
	})
}

func (r *customEndpointRepo) Update(ctx context.Context, req *nb.CustomEndpoint) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_endpoint.Update")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	setClauses := []string{"updated_at = $1"}
	args := []any{time.Now()}
	idx := 2

	if req.GetName() != "" {
		setClauses = append(setClauses, fmt.Sprintf("name = $%d", idx))
		args = append(args, req.GetName())
		idx++
	}
	if req.GetDescription() != "" {
		setClauses = append(setClauses, fmt.Sprintf("description = $%d", idx))
		args = append(args, req.GetDescription())
		idx++
	}
	if req.GetSql() != "" {
		setClauses = append(setClauses, fmt.Sprintf("sql_query = $%d", idx))
		args = append(args, req.GetSql())
		idx++
	}
	if req.GetMethod() != "" {
		setClauses = append(setClauses, fmt.Sprintf("method = $%d", idx))
		args = append(args, req.GetMethod())
		idx++
	}

	setClauses = append(setClauses, fmt.Sprintf("in_transaction = $%d", idx))
	args = append(args, req.GetInTransaction())
	idx++

	if len(req.GetParameters()) > 0 {
		paramsJSON, _ := json.Marshal(req.GetParameters())
		setClauses = append(setClauses, fmt.Sprintf("parameters = $%d", idx))
		args = append(args, paramsJSON)
		idx++
	}

	args = append(args, req.GetId())
	query := fmt.Sprintf(
		"UPDATE custom_endpoint SET %s WHERE id = $%d",
		strings.Join(setClauses, ", "), idx,
	)

	_, err = conn.Exec(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("custom_endpoint.Update: %w", err)
	}

	return r.GetById(ctx, &nb.CustomEndpointId{
		ResourceEnvId: req.GetResourceEnvId(),
		Id:            req.GetId(),
	})
}

func (r *customEndpointRepo) GetAll(ctx context.Context, req *nb.GetCustomEndpointListRequest) (*nb.CustomEndpointList, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_endpoint.GetAll")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	var (
		qb strings.Builder

		args = make([]any, 0)

		orderColumn = "created_at"
		orderDir    = "DESC"

		limit = req.GetLimit()

		endpoints []*nb.CustomEndpoint
	)

	qb.WriteString(`SELECT id, name, description, sql_query, method, in_transaction, parameters, created_at, updated_at
		FROM custom_endpoint WHERE deleted_at IS NULL`)

	if req.GetSearch() != "" {
		args = append(args, "%"+req.GetSearch()+"%")
		qb.WriteString(fmt.Sprintf(" AND (name ILIKE $%d OR description ILIKE $%d)", len(args), len(args)))
	}

	if col, ok := config.CustomEndpointAllowedOrder[req.GetOrderBy()]; ok {
		orderColumn = col
	}
	if req.GetOrderDirection() == "asc" {
		orderDir = "ASC"
	}

	qb.WriteString(fmt.Sprintf(" ORDER BY %s %s", orderColumn, orderDir))

	if limit == 0 {
		limit = 20
	}

	args = append(args, limit, req.GetOffset())
	qb.WriteString(fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)))

	rows, err := conn.Query(ctx, qb.String(), args...)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return &nb.CustomEndpointList{}, nil
		}
		return nil, fmt.Errorf("custom_endpoint.GetAll: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		e, scanErr := scanEndpoint(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		e.ResourceEnvId = req.GetResourceEnvId()
		endpoints = append(endpoints, e)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("custom_endpoint.GetAll rows: %w", err)
	}

	return &nb.CustomEndpointList{
		Endpoints: endpoints,
		Count:     uint32(len(endpoints)),
	}, nil
}

func (r *customEndpointRepo) GetById(ctx context.Context, req *nb.CustomEndpointId) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_endpoint.GetById")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	query := `SELECT id, name, description, sql_query, method, in_transaction, parameters, created_at, updated_at
		FROM custom_endpoint WHERE id = $1 AND deleted_at IS NULL`

	row := conn.QueryRow(ctx, query, req.GetId())
	e, err := scanEndpointRow(row)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("custom endpoint not found: %s", req.GetId())
		}
		return nil, fmt.Errorf("custom_endpoint.GetById: %w", err)
	}
	e.ResourceEnvId = req.GetResourceEnvId()
	return e, nil
}

func (r *customEndpointRepo) Delete(ctx context.Context, req *nb.CustomEndpointId) (*nb.CustomEndpoint, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_endpoint.Delete")
	defer dbSpan.Finish()

	// Fetch before delete so we can return the deleted record
	e, err := r.GetById(ctx, req)
	if err != nil {
		return nil, err
	}

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(ctx,
		"UPDATE custom_endpoint SET deleted_at = $1 WHERE id = $2",
		time.Now(), req.GetId(),
	)
	if err != nil {
		return nil, fmt.Errorf("custom_endpoint.Delete: %w", err)
	}

	return e, nil
}

func (r *customEndpointRepo) Run(ctx context.Context, req *nb.RunCustomEndpointRequest) (*nb.RunCustomEndpointResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_endpoint.Run")
	defer dbSpan.Finish()

	e, err := r.GetById(ctx, &nb.CustomEndpointId{
		ResourceEnvId: req.GetResourceEnvId(),
		Id:            req.GetId(),
	})
	if err != nil {
		return nil, err
	}

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return nil, err
	}

	sqlQuery := e.GetSql()
	inputParams := req.GetParams()

	var args []any
	argNameMap := make(map[string]int)

	matches := paramRegexp.FindAllStringSubmatch(sqlQuery, -1)
	finalSQL := sqlQuery

	if len(matches) > 0 {
		// ─── Формат :paramName → заменяем на $N ───────────────────────
		for _, match := range matches {
			paramName := match[1]
			if _, ok := argNameMap[paramName]; !ok {
				val, exists := inputParams[paramName]
				if !exists {
					args = append(args, nil)
				} else {
					args = append(args, val)
				}
				argNameMap[paramName] = len(args)
			}
			placeholder := fmt.Sprintf("$%d", argNameMap[paramName])
			finalSQL = strings.ReplaceAll(finalSQL, ":"+paramName, placeholder)
		}
	} else {
		// ─── Формат $1, $2... → строим args по порядку из e.Parameters ─
		for _, param := range e.GetParameters() {
			val, exists := inputParams[param.GetName()]
			if !exists {
				args = append(args, nil)
			} else {
				args = append(args, val)
			}
		}
	}

	rows, err := conn.Query(ctx, finalSQL, args...)
	if err != nil {
		return &nb.RunCustomEndpointResponse{Error: err.Error()}, nil
	}
	defer rows.Close()

	fields := rows.FieldDescriptions()
	var result []map[string]any

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, err
		}
		rowMap := make(map[string]any)
		for i, field := range fields {
			val := values[i]
			// pgx возвращает UUID как [16]byte — конвертируем в строку
			if b, ok := val.([16]byte); ok {
				val = fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
					b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
			}
			rowMap[field.Name] = val
		}
		result = append(result, rowMap)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("custom_endpoint.Run rows: %w", err)
	}

	data, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	return &nb.RunCustomEndpointResponse{Data: data}, nil
}

// ─── helpers ──────────────────────────────────────────────────────────────────

type rowScanner interface {
	Scan(dest ...any) error
}

func scanEndpoint(row rowScanner) (*nb.CustomEndpoint, error) {
	var (
		e                    nb.CustomEndpoint
		createdAt, updatedAt time.Time
		desc                 sql.NullString
		paramsJSON           []byte
	)
	if err := row.Scan(&e.Id, &e.Name, &desc, &e.Sql, &e.Method, &e.InTransaction, &paramsJSON, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("scanEndpoint: %w", err)
	}
	if desc.Valid {
		e.Description = desc.String
	}
	if len(paramsJSON) > 0 {
		var params []*nb.Parameter
		if err := json.Unmarshal(paramsJSON, &params); err == nil {
			e.Parameters = params
		}
	}
	e.CreatedAt = createdAt.Format(time.RFC3339)
	e.UpdatedAt = updatedAt.Format(time.RFC3339)
	return &e, nil
}

func scanEndpointRow(row interface{ Scan(...any) error }) (*nb.CustomEndpoint, error) {
	return scanEndpoint(row)
}

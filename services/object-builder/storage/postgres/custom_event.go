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
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
)

type customeEventRepo struct {
	db *psqlpool.Pool
}

func NewCustomEventRepo(db *psqlpool.Pool) storage.CustomEventRepoI {
	return &customeEventRepo{
		db: db,
	}
}

func (c *customeEventRepo) Create(ctx context.Context, req *nb.CreateCustomEventRequest) (resp *nb.CustomEvent, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_event.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	atrBody := []byte(`{}`)

	if req.Attributes != nil {
		atrBody, err = json.Marshal(req.Attributes)
		if err != nil {
			return nil, errors.Wrap(err, "marshal")
		}
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	query := `INSERT INTO "custom_event" (
		id,
		table_slug,
		icon,
		label,
		event_path,
		url,
		disable,
		method,
		action_type,
		attributes,
		path
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11) `

	customEventId := uuid.NewString()

	_, err = tx.Exec(ctx, query,
		customEventId,
		req.TableSlug,
		req.Icon,
		req.Label,
		req.EventPath,
		req.Url,
		req.Disable,
		req.Method,
		req.ActionType,
		atrBody,
		req.Path,
	)
	if err != nil {
		return nil, errors.Wrap(err, "create custom event")
	}

	var (
		tableId  string
		argCount int
		args     []any
	)

	query = `SELECT id FROM "table" WHERE slug = $1`

	err = tx.QueryRow(ctx, query, req.TableSlug).Scan(&tableId)
	if err != nil {
		return nil, errors.Wrap(err, "get function")
	}

	roles, err := helper.RolesFind(ctx, models.RelationHelper{Tx: tx})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find roles")
	}

	query = `INSERT INTO "action_permission" (
		table_slug,
		permission,
		role_id,
		custom_event_id
	) VALUES `

	for _, roleId := range roles {
		query += fmt.Sprintf(` ($%d,$%d,$%d,$%d),`, argCount+1, argCount+2, argCount+3, argCount+4)
		argCount += 4
		args = append(args, req.TableSlug, true, roleId, customEventId)
	}

	query = strings.TrimRight(query, ",")
	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, "insert to action_permission")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "commit transaction")
	}

	return &nb.CustomEvent{Label: req.Label}, nil
}

func (c *customeEventRepo) Update(ctx context.Context, req *nb.CustomEvent) (err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_event.Update")
	defer dbSpan.Finish()
	var (
		atrBody = []byte(`{}`)
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	if req.Attributes != nil {
		atrBody, err = json.Marshal(req.Attributes)
		if err != nil {
			return errors.Wrap(err, "marshal")
		}
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	query := `UPDATE custom_event SET
		icon = $2,
		label = $3,
		event_path = $4,
		url = $5,
		disable = $6,
		method = $7,
		action_type = $8,
		attributes = $9,
		path = $10
	WHERE id = $1`

	res, err := tx.Exec(ctx, query,
		req.Id,
		req.Icon,
		req.Label,
		req.EventPath,
		req.Url,
		req.Disable,
		req.Method,
		req.ActionType,
		atrBody,
		req.Path,
	)
	if err != nil {
		return errors.Wrap(err, "update custom event")
	}
	if res.RowsAffected() == 0 {
		return errors.Wrap(fmt.Errorf("action not found with given id"), "action not found with given id")
	}

	query = `UPDATE action_permission SET label = $2 WHERE custom_event_id = $1`

	_, err = tx.Exec(ctx, query, req.Id, req.Label)
	if err != nil {
		return errors.Wrap(err, "update action permission")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return errors.Wrap(err, "commit transaction")
	}

	return nil
}

func (c *customeEventRepo) GetList(ctx context.Context, req *nb.GetCustomEventsListRequest) (resp *nb.GetCustomEventsListResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_event.GetList")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.GetCustomEventsListResponse{}
	query := fmt.Sprintf(`
	SELECT 
		c.id,
		c.table_slug,
		c.icon,
		c.label,
		c.event_path,
		c.url,
		c.disable,
		c.method,
		c.action_type,
		c.attributes,
		c.path,

		COALESCE(ac.guid::varchar, ''),
		COALESCE(ac.label, ''),
		COALESCE(ac.table_slug, ''),
		COALESCE(ac.permission, false),

		jsonb_agg(jsonb_build_object(
			'id', f.id,
			'path', f.path,
			'name', f.name,
			'description', f.description,
			'project_id', f.project_id,
			'request_type', f.request_type,
			'type', f.type
		)) as functions

	FROM custom_event c
	JOIN function f ON f.id = c.event_path
	LEFT JOIN action_permission ac ON ac.custom_event_id = c.id AND ac.role_id::varchar = '%s'
	WHERE c.table_slug = '%s' 
	`, req.RoleId, req.TableSlug)

	if req.Method != "" {
		query += fmt.Sprintf(` AND c.method = '%s' `, req.Method)
	}

	query += "GROUP BY c.id, ac.guid"

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, "get rows")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			cs                         = nb.CustomEvent{}
			acId, acLabel, acTableSlug string
			atr, function              []byte
			acPermission               bool
		)

		err = rows.Scan(
			&cs.Id,
			&cs.TableSlug,
			&cs.Icon,
			&cs.Label,
			&cs.EventPath,
			&cs.Url,
			&cs.Disable,
			&cs.Method,
			&cs.ActionType,
			&atr,
			&cs.Path,
			&acId,
			&acLabel,
			&acTableSlug,
			&acPermission,

			&function,
		)
		if err != nil {
			return nil, errors.Wrap(err, "scan row")
		}

		if err := json.Unmarshal(atr, &cs.Attributes); err != nil {
			return nil, errors.Wrap(err, "unmarshal atributes")
		}
		ac := map[string]any{
			"id":              acId,
			"label":           acLabel,
			"table_slug":      acTableSlug,
			"permission":      acPermission,
			"custom_event_id": cs.Id,
			"role_id":         req.RoleId,
		}

		acBody, err := helper.ConvertMapToStruct(ac)
		if err != nil {
			return nil, errors.Wrap(err, "convert map to struct")
		}

		cs.ActionPermission = acBody

		if err := json.Unmarshal(function, &cs.Functions); err != nil {
			return nil, errors.Wrap(err, "unmarshal functions")
		}

		resp.CustomEvents = append(resp.CustomEvents, &cs)
	}

	query = `SELECT COUNT(*) FROM custom_event WHERE table_slug = $1`

	if req.Method != "" {
		query += fmt.Sprintf(` AND method = '%s' `, req.Method)
	}

	err = conn.QueryRow(ctx, query, req.TableSlug).Scan(&resp.Count)
	if err != nil {
		return nil, errors.Wrap(err, "get count custom event")
	}

	return resp, nil
}

func (c *customeEventRepo) GetSingle(ctx context.Context, req *nb.CustomEventPrimaryKey) (resp *nb.CustomEvent, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_event.GetSingle")
	defer dbSpan.Finish()
	var (
		atr = []byte{}
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.CustomEvent{}

	query := `SELECT
		id,
		table_slug,
		icon,
		label,
		event_path,
		url,
		disable,
		method,
		action_type,
		attributes,
		path
	FROM custom_event WHERE id = $1`

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&resp.TableSlug,
		&resp.Icon,
		&resp.Label,
		&resp.EventPath,
		&resp.Url,
		&resp.Disable,
		&resp.Method,
		&resp.ActionType,
		&atr,
		&resp.Path,
	)
	if err != nil {
		return nil, errors.Wrap(err, "get custom event")
	}

	if err := json.Unmarshal(atr, &resp.Attributes); err != nil {
		return nil, errors.Wrap(err, "unmarshal atributes")
	}

	return resp, nil
}

func (c *customeEventRepo) Delete(ctx context.Context, req *nb.CustomEventPrimaryKey) (err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_event.Delete")
	defer dbSpan.Finish()
	var (
		funcPath, tableId, tableSlug string
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	query := `SELECT f.path, t.id, t.slug FROM custom_event c 
	JOIN function f ON f.id = c.event_path
	JOIN "table" t ON t.slug = c.table_slug
	WHERE c.id = $1`

	err = tx.QueryRow(ctx, query, req.Id).Scan(
		&funcPath,
		&tableId,
		&tableSlug,
	)
	if err != nil {
		return errors.Wrap(err, "get custom event")
	}

	funcPath = strings.ReplaceAll(funcPath, "-", "_")

	query = `DELETE FROM custom_event WHERE id = $1`

	_, err = tx.Exec(ctx, query, req.Id)
	if err != nil {
		return errors.Wrap(err, "delete custom event")
	}

	query = `DELETE FROM field WHERE table_id = $1 AND slug = $2`

	_, err = tx.Exec(ctx, query, tableId, funcPath+"_disable")
	if err != nil {
		return errors.Wrap(err, "delete field")
	}

	query = `DELETE FROM action_permission WHERE custom_event_id = $1`

	_, err = tx.Exec(ctx, query, req.Id)
	if err != nil {
		return errors.Wrap(err, "delete action permission")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return errors.Wrap(err, "commit transaction")
	}

	return nil
}

func (c *customeEventRepo) UpdateByFunctionId(ctx context.Context, req *nb.UpdateByFunctionIdRequest) (err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "custom_event.UpdateByFunctionId")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var (
		query     = `UPDATE custom_event SET disable = true WHERE event_path = $1 RETURNING table_slug`
		tableSlug string
	)

	if err = tx.QueryRow(ctx, query, req.FunctionId).Scan(&tableSlug); err != nil {
		return errors.Wrap(err, "custom event disable")
	}

	req.FieldSlug = strings.ReplaceAll(req.FieldSlug, "-", "_")

	query = fmt.Sprintf(`UPDATE %s SET %s = true WHERE guid = $1`, tableSlug, req.FieldSlug)

	for _, id := range req.ObjectIds {
		_, err = tx.Exec(ctx, query, id)
		if err != nil {
			return errors.Wrap(err, "update table disable")
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return errors.Wrap(err, "commit transaction")
	}

	return nil
}

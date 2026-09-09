package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type fieldRepo struct {
	db           *psqlpool.Pool
	relationRepo storage.RelationRepoI
	logger       logger.LoggerI
}

func NewFieldRepo(db *psqlpool.Pool, relationRepo storage.RelationRepoI, logger logger.LoggerI) storage.FieldRepoI {
	return &fieldRepo{
		db:           db,
		relationRepo: relationRepo,
		logger:       logger,
	}
}

func (f *fieldRepo) Create(ctx context.Context, req *nb.CreateFieldRequest) (resp *nb.Field, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.Create")
	defer dbSpan.Finish()

	var (
		body                                  []byte
		fields                                = []models.SectionFields{}
		tableSlug, layoutId, tabId, sectionId string
		sectionCount                          int32
		ids                                   []string
		tableLabel, tableId                   string
		fieldAttributes                       = make(map[string]any)
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT id, slug, label FROM "table" WHERE `
	if _, err = uuid.Parse(req.TableId); err != nil {
		query += `slug = $1`
	} else {
		query += `id = $1`
	}
	if err := conn.Db.QueryRow(ctx, query, req.TableId).Scan(&tableId, &tableSlug, &tableLabel); err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select slug, label from table")
	}

	if req.Type == config.PERSON {
		var (
			fieldId = uuid.NewString()
		)

		relationAttributes, err := helper.ConvertMapToStruct(map[string]any{
			"label_en":              "Person",
			"label_to_en":           tableLabel,
			"table_editable":        false,
			"enable_multi_language": false,
		})
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to convert map to struct relation attributes")
		}

		_, err = f.relationRepo.Create(ctx, &nb.CreateRelationRequest{
			Id:                uuid.NewString(),
			TableFrom:         tableSlug,
			TableTo:           config.PERSON_TABLE_SLUG,
			Type:              config.MANY2ONE,
			ViewFields:        []string{"f54d8076-4972-4067-9a91-c178c02c4273"},
			RelationTableSlug: config.PERSON_TABLE_SLUG,
			AutoFilters:       []*nb.AutoFilter{},
			RelationFieldId:   fieldId,
			Attributes:        relationAttributes,
			RelationToFieldId: uuid.NewString(),
			ProjectId:         req.ProjectId,
		})
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to create relation")
		}

		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: fieldId, ProjectId: req.ProjectId})
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to begin transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	req.Slug = strings.ToLower(req.GetSlug())

	query = `INSERT INTO "field" (
		id,
		"table_id",
		"required",
		"slug",
		"label",
		"default",
		"type",
		"index",
		"attributes",
		"is_visible",
		autofill_field,
		autofill_table,
		"unique",
		"automatic",
		"is_search"
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	attributes, err := json.Marshal(req.Attributes)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to marshal attributes")
	}

	err = json.Unmarshal(attributes, &fieldAttributes)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to unmarshal attributes")
	}

	_, err = tx.Exec(ctx, query,
		req.GetId(),
		tableId,
		false,
		req.GetSlug(),
		req.GetLabel(),
		req.GetDefault(),
		req.GetType(),
		req.GetIndex(),
		attributes,
		req.GetIsVisible(),
		req.GetAutofillField(),
		req.GetAutofillTable(),
		req.GetUnique(),
		req.GetAutomatic(),
		true,
	)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to execute insert query")
	}

	query = `ALTER TABLE "` + tableSlug + `" ADD COLUMN ` + req.Slug + " " + helper.GetDataType(req.Type)

	_, err = tx.Exec(ctx, query)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to alter table")
	}

	if req.Unique {
		query = fmt.Sprintf(`ALTER TABLE IF EXISTS %s ADD CONSTRAINT %s_%s_unq UNIQUE(%s)`, tableSlug, tableSlug, req.Slug, req.Slug)
		_, err = tx.Exec(ctx, query)
		if err != nil {
			return nil, f.db.HandleDatabaseError(err, "Create field: failed to add unique constraint")
		}
	}

	query = `SELECT guid FROM "role"`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to select guid from role")
	}
	defer rows.Close()

	for rows.Next() {
		var id string

		err = rows.Scan(&id)
		if err != nil {
			return nil, f.db.HandleDatabaseError(err, "Create field: failed to scan id")
		}

		ids = append(ids, id)
	}

	query = `INSERT INTO "field_permission" (
		"edit_permission",
		"view_permission",
		"table_slug",
		"field_id",
		"label",
		role_id
	) VALUES (true, true, $1, $2, $3, $4)`

	for _, id := range ids {
		_, err = tx.Exec(ctx, query, tableSlug, req.Id, req.Label, id)
		if err != nil {
			return nil, f.db.HandleDatabaseError(err, "Create field: failed to insert field_permission")
		}
	}

	query = `SELECT id FROM "layout" WHERE table_id = $1`
	err = tx.QueryRow(ctx, query, tableId).Scan(&layoutId)
	if err != nil && err != pgx.ErrNoRows {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select id from layout")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	query = `SELECT id FROM "tab" WHERE "layout_id" = $1 and type = 'section'`
	err = tx.QueryRow(ctx, query, layoutId).Scan(&tabId)
	if err != nil && err != pgx.ErrNoRows {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select id from tab")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	query = `SELECT id, fields FROM "section" WHERE tab_id = $1 ORDER BY created_at DESC LIMIT 1`
	err = tx.QueryRow(ctx, query, tabId).Scan(&sectionId, &body)
	if err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select id, fields from section")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	queryCount := `SELECT COUNT(*) FROM "section" WHERE tab_id = $1`
	err = tx.QueryRow(ctx, queryCount, tabId).Scan(&sectionCount)
	if err != nil && err != pgx.ErrNoRows {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select count from section")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	if err := json.Unmarshal(body, &fields); err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failded to unmarshal fields")
	}

	if len(fields) < 3 {
		query := `UPDATE "section" SET fields = $2 WHERE id = $1`

		fields = append(fields, models.SectionFields{
			Id:    req.Id,
			Order: len(fields) + 1,
		})

		reqBody, err := json.Marshal(fields)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to marshal fields")
		}

		_, err = tx.Exec(ctx, query, sectionId, reqBody)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to execute update section query")
		}
	} else {
		query = `INSERT INTO "section" ("order", "column", label, table_id, tab_id, fields) VALUES ($1, $2, $3, $4, $5, $6)`

		sectionId = uuid.NewString()

		fields := []models.SectionFields{{Id: req.Id, Order: 1}}

		reqBody, err := json.Marshal(fields)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to insert section")
		}

		_, err = tx.Exec(ctx, query, sectionCount+1, "SINGLE", "Info", tableId, tabId, reqBody)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to execute insert section query")
		}
	}

	if req.Type == config.INCREMENT_ID {
		var maxValue int
		digitNumber := cast.ToInt(fieldAttributes["digit_number"])
		if digitNumber == 0 || digitNumber > 9 {
			maxValue = 999999999
		} else {
			maxValue = int(math.Pow10(digitNumber)) - 1
		}

		query = `INSERT INTO "incrementseqs" (field_slug, table_slug, max_value) VALUES ($1, $2, $3)`

		_, err = tx.Exec(ctx, query, req.Slug, tableSlug, maxValue)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to insert incementseqs query")
		}
	}

	query = `
		UPDATE "view"
		SET columns = array_append(columns, $1)
		WHERE table_slug = $2
	`
	_, err = tx.Exec(ctx, query, req.GetId(), tableSlug)
	if err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to execute update view query")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to commit transaction")
	}

	return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
}

func (f *fieldRepo) GetByID(ctx context.Context, req *nb.FieldPrimaryKey) (resp *nb.Field, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.GetByID")
	defer dbSpan.Finish()

	var (
		attributes     = []byte{}
		relationIdNull sql.NullString
		fiedlDefault   sql.NullString
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.Field{}
	query := `SELECT 
		id,
		"table_id",
		"required",
		"slug",
		"label",
		"default",
		"type",
		COALESCE("index", ''),
		"attributes",
		"is_visible",
		autofill_field,
		autofill_table,
		"unique",
		"automatic",
		relation_id,
		enable_multilanguage
	FROM "field" WHERE id = $1`

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&resp.TableId,
		&resp.Required,
		&resp.Slug,
		&resp.Label,
		&fiedlDefault,
		&resp.Type,
		&resp.Index,
		&attributes,
		&resp.IsVisible,
		&resp.AutofillField,
		&resp.AutofillTable,
		&resp.Unique,
		&resp.Automatic,
		&relationIdNull,
		&resp.EnableMultilanguage,
	)
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error getting field")
	}

	resp.RelationId = relationIdNull.String
	resp.Default = fiedlDefault.String

	if err := json.Unmarshal(attributes, &resp.Attributes); err != nil {
		return &nb.Field{}, errors.Wrap(err, "error unmarshaling attributes")
	}

	return resp, nil
}

func (f *fieldRepo) GetAll(ctx context.Context, req *nb.GetAllFieldsRequest) (resp *nb.GetAllFieldsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.GetAll")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.GetAllFieldsResponse{}

	getTable, err := helper.GetTableByIdSlug(ctx, models.GetTableByIdSlugReq{Conn: conn, Id: req.TableId, Slug: req.TableSlug})
	if err != nil {
		return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error getting table")
	}

	req.TableId = cast.ToString(getTable["id"])
	req.TableSlug = cast.ToString(getTable["slug"])

	query := `SELECT 
		"id",
		"table_id",
		"required",
		"slug",
		"label",
		"default",
		"type",
		"index",
		"attributes",
		"is_visible",
		autofill_field,
		autofill_table,
		"unique",
		"automatic",
		relation_id,
		enable_multilanguage
	FROM "field" WHERE table_id = $1 ORDER BY created_at DESC LIMIT $2 OFFSET $3`

	rows, err := conn.Query(ctx, query, req.TableId, req.Limit, req.Offset)
	if err != nil {
		return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error getting fields")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			field             = nb.Field{}
			attributes        sql.NullString
			autoFillFieldNull sql.NullString
			autoFillTableNull sql.NullString
			relationIdNull    sql.NullString
			defaultStr, index sql.NullString
		)

		err = rows.Scan(
			&field.Id,
			&field.TableId,
			&field.Required,
			&field.Slug,
			&field.Label,
			&defaultStr,
			&field.Type,
			&index,
			&attributes,
			&field.IsVisible,
			&autoFillFieldNull,
			&autoFillTableNull,
			&field.Unique,
			&field.Automatic,
			&relationIdNull,
			&field.EnableMultilanguage,
		)
		if err != nil {
			return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error scanning fields")
		}

		if attributes.Valid {
			err := json.Unmarshal([]byte(attributes.String), &field.Attributes)
			if err != nil {
				return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error unmarshaling attributes")
			}
		}

		field.AutofillField = autoFillFieldNull.String
		field.AutofillTable = autoFillTableNull.String
		field.RelationField = relationIdNull.String
		field.Default = defaultStr.String
		field.Index = index.String

		resp.Fields = append(resp.Fields, &field)
	}

	query = `SELECT COUNT(*) FROM "field" WHERE table_id = $1`

	err = conn.QueryRow(ctx, query, req.TableId).Scan(&resp.Count)
	if err != nil {
		return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error getting count")
	}

	if req.WithManyRelation {
		query = `
		SELECT table_from FROM "relation" WHERE 
			(table_to = $1 AND type = 'Many2One')
			OR
			(table_to = $1 AND type = 'Many2Many')
		`

		rows, err := conn.Query(ctx, query, req.TableSlug)
		if err != nil {
			return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error getting relation")
		}
		defer rows.Close()

		for rows.Next() {
			var tableFrom string

			err = rows.Scan(&tableFrom)
			if err != nil {
				return &nb.GetAllFieldsResponse{}, errors.Wrap(err, "error scanning table from")
			}
		}
	}

	return resp, nil
}

func (f *fieldRepo) Update(ctx context.Context, req *nb.Field) (resp *nb.Field, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.Update")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error creating transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	resp, err = f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error getting field")
	}

	if resp.IsSystem {
		return &nb.Field{}, fmt.Errorf("error you can't update this field its system field")
	}

	var (
		tableFilter, tableId, tableSlug string
		fieldId, fieldFilter            string
		fieldAttributes                 = make(map[string]any)
	)

	if _, err := uuid.Parse(req.TableId); err == nil {
		tableFilter = ` id = $1`
	} else {
		tableFilter = ` slug = $1`
	}

	query := fmt.Sprintf(`SELECT id, slug FROM "table" WHERE %s`, tableFilter)

	err = tx.QueryRow(ctx, query, req.TableId).Scan(&tableId, &tableSlug)
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error getting table slug")
	}

	if _, err := uuid.Parse(req.Id); err == nil {
		fieldFilter = ` id = $1`
	} else {
		fieldFilter = ` slug = $1`
	}

	query = fmt.Sprintf(`SELECT id FROM "field" WHERE %s`, fieldFilter)

	err = tx.QueryRow(ctx, query, req.Id).Scan(&fieldId)
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error getting table slug")
	}

	attributes, err := json.Marshal(req.Attributes)
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error marshaling attributes")
	}

	err = json.Unmarshal(attributes, &fieldAttributes)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to unmarshal attributes")
	}

	query = `UPDATE "field" SET
		"required" = $2,
		"slug" = $3,
		"label" = $4,
		"default" = $5,
		"type" = $6,
		"index" = $7,
		"attributes" = $8,
		"is_visible" = $9,
		autofill_field = $10,
		autofill_table = $11,
		"unique" = $12,
		"automatic" = $13,
		enable_multilanguage = $14
	WHERE id = $1`

	_, err = tx.Exec(ctx, query, fieldId,
		req.Required,
		req.Slug,
		req.Label,
		req.Default,
		req.Type,
		req.Index,
		attributes,
		req.IsVisible,
		req.AutofillField,
		req.AutofillTable,
		req.Unique,
		req.Automatic,
		req.EnableMultilanguage,
	)
	if err != nil {
		return &nb.Field{}, errors.Wrap(err, "error updating field")
	}

	if resp.Type != req.Type {
		query = fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN %s`, tableSlug, resp.Slug)

		_, err = tx.Exec(ctx, query)
		if err != nil {
			return &nb.Field{}, errors.Wrap(err, "error dropping column")
		}

		fieldType := helper.GetDataType(req.Type)

		query = fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s %s`, tableSlug, req.Slug, fieldType)

		_, err = tx.Exec(ctx, query)
		if err != nil {
			return &nb.Field{}, errors.Wrap(err, "error adding column")
		}
	}

	if resp.Slug != req.Slug {
		query = fmt.Sprintf(`ALTER TABLE "%s"
		RENAME COLUMN %s TO %s;`, tableSlug, resp.Slug, req.Slug)

		_, err = tx.Exec(ctx, query)
		if err != nil {
			return &nb.Field{}, errors.Wrap(err, "error renaming column")
		}
	}

	if !resp.Unique && req.Unique {
		query = fmt.Sprintf(`ALTER TABLE IF EXISTS %s ADD CONSTRAINT %s_%s_unq UNIQUE(%s)`, tableSlug, tableSlug, req.Slug, req.Slug)
		_, err = tx.Exec(ctx, query)
		if err != nil {
			return nil, f.db.HandleDatabaseError(err, "Create field: failed to add unique constraint")
		}
	}

	if resp.Unique && !req.Unique {
		query = fmt.Sprintf(`ALTER TABLE IF EXISTS "%s" DROP CONSTRAINT IF EXISTS %s_%s_unq`, tableSlug, tableSlug, resp.Slug)
		_, err = tx.Exec(ctx, query)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to add unique constraint")
		}
	}

	if req.Type == config.INCREMENT_ID {
		var maxValue int
		digitNumber := cast.ToInt(fieldAttributes["digit_number"])
		if digitNumber == 0 || digitNumber > 9 {
			maxValue = 999999999
		} else {
			maxValue = int(math.Pow10(digitNumber)) - 1
		}

		query = `UPDATE "incrementseqs" SET 
			max_value = $3
			WHERE field_slug = $1 AND table_slug = $2`

		_, err = tx.Exec(ctx, query, req.Slug, tableSlug, maxValue)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to insert incementseqs query")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return &nb.Field{}, errors.Wrap(err, "error committing transaction")
	}

	return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: fieldId, ProjectId: req.ProjectId})
}

func (f *fieldRepo) UpdateSearch(ctx context.Context, req *nb.SearchUpdateRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.UpdateSearch")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `UPDATE "field" SET is_search = $1 WHERE id = $2`

	for _, val := range req.Fields {
		_, err = tx.Exec(ctx, query, val.IsSearch, val.Id)
		if err != nil {
			return errors.Wrap(err, "error updating search")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "error committing transaction")
	}

	return nil
}

func (f *fieldRepo) Delete(ctx context.Context, req *nb.FieldPrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.Delete")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "error creating transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	var (
		isSystem, isExists   bool
		tableId, viewId      string
		columns, newColumns  []string
		tableSlug, fieldSlug string
		filterField, fieldId string
	)

	if _, err := uuid.Parse(req.Id); err == nil {
		filterField = ` f.id = $1 `
	} else {
		filterField = ` f.slug = $1 `
	}

	query := fmt.Sprintf(`SELECT f.id, f.is_system, f.table_id, f.slug
		FROM field f
		JOIN "table" c ON f.table_id = c.id
		WHERE %s AND c.slug = $2;
		`, filterField)

	err = tx.QueryRow(ctx, query, req.Id, req.TableSlug).Scan(&fieldId, &isSystem, &tableId, &fieldSlug)
	if err != nil {
		return errors.Wrap(err, "error getting field")
	}

	if isSystem {
		return errors.Wrap(err, "error you can't delete this field its system field")
	}

	query = `DELETE FROM "field_permission" WHERE field_id = $1`

	_, err = tx.Exec(ctx, query, fieldId)
	if err != nil && err != pgx.ErrNoRows {
		return errors.Wrap(err, "error deleting field permission")
	}

	query = `SELECT slug FROM "table" where id = $1`

	err = tx.QueryRow(ctx, query, tableId).Scan(&tableSlug)
	if err != nil {
		return errors.Wrap(err, "error getting table")
	}

	query = `SELECT id, columns FROM "view" WHERE table_slug = $1 `

	err = tx.QueryRow(ctx, query, tableSlug).Scan(&viewId, &columns)
	if err != nil {
		return errors.Wrap(err, "error getting view")
	}

	if len(columns) > 0 {
		for _, c := range columns {
			if c == fieldId {
				isExists = true
				continue
			} else {
				newColumns = append(newColumns, c)
			}
		}

		if isExists {
			query = `UPDATE "view" SET columns = $1 WHERE id = $2`

			_, err = tx.Exec(ctx, query, pq.Array(newColumns), viewId)
			if err != nil {
				return errors.Wrap(err, "error updating view")
			}
		}
	}

	query = `SELECT id, fields FROM "section" WHERE table_id = $1`

	sectionRows, err := tx.Query(ctx, query, tableId)
	if err != nil {
		return errors.Wrap(err, "error getting section")
	}
	defer sectionRows.Close()

	sections := []models.SectionBody{}

	for sectionRows.Next() {
		var (
			id         string
			fields     = []byte{}
			fieldsBody []map[string]any
		)

		err = sectionRows.Scan(&id, &fields)
		if err != nil {
			return errors.Wrap(err, "error scanning section")
		}

		if err := json.Unmarshal(fields, &fieldsBody); err != nil {
			return errors.Wrap(err, "error unmarshaling fields")
		}

		sections = append(sections, models.SectionBody{Id: id, Fields: fieldsBody})
	}

	query = `UPDATE "section" SET fields = $2 WHERE id = $1`

	for _, section := range sections {
		var (
			isExists  = false
			newFields []map[string]any
		)

		for i, field := range section.Fields {
			if cast.ToString(field["id"]) != fieldId {
				field["order"] = i + 1
				newFields = append(newFields, field)
			} else {
				isExists = true
			}
		}

		if isExists {
			fieldsBody, err := json.Marshal(newFields)
			if err != nil {
				return errors.Wrap(err, "error marshaling fields")
			}

			_, err = tx.Exec(ctx, query, section.Id, fieldsBody)
			if err != nil {
				return errors.Wrap(err, "error updating section")
			}
		}
	}

	query = `DELETE FROM "field" WHERE id = $1`

	_, err = tx.Exec(ctx, query, fieldId)
	if err != nil {
		return errors.Wrap(err, "error deleting field")
	}

	query = `
		UPDATE "relation"
			SET view_fields = array_remove(view_fields, $1)
		WHERE $1 = ANY(view_fields);`
	_, err = tx.Exec(ctx, query, fieldId)
	if err != nil {
		return errors.Wrap(err, "error deleting relation view fields")
	}

	query = fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN %s`, tableSlug, fieldSlug)

	_, err = tx.Exec(ctx, query)
	if err != nil {
		return errors.Wrap(err, "error dropping column")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "error committing transaction")
	}

	return nil
}

func (f *fieldRepo) FieldsWithPermissions(ctx context.Context, req *nb.FieldsWithRelationRequest) (resp *nb.FieldsWithRelationsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.FieldsWithPermissions")
	defer dbSpan.Finish()

	resp = &nb.FieldsWithRelationsResponse{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	getTable, err := helper.GetTableByIdSlug(ctx, models.GetTableByIdSlugReq{Conn: conn, Slug: req.TableSlug})
	if err != nil {
		return &nb.FieldsWithRelationsResponse{}, err
	}

	query := `SELECT slug, label FROM "field" WHERE table_id = $1 AND type <> 'LOOKUP'`
	rows, err := conn.Query(ctx, query, cast.ToString(getTable["id"]))
	if err != nil {
		return &nb.FieldsWithRelationsResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			slug  string
			label string
		)
		err = rows.Scan(
			&slug,
			&label,
		)
		if err != nil {
			return &nb.FieldsWithRelationsResponse{}, err
		}

		resp.Fields = append(resp.Fields, &nb.FieldNew{Slug: slug, Label: label})
	}

	query = `SELECT table_to, table_from, field_from FROM "relation" WHERE (table_to = $1 OR table_from = $1) AND type = 'Many2One'`
	rows, err = conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.FieldsWithRelationsResponse{}, err
	}
	defer rows.Close()
	for rows.Next() {
		var (
			tableTo   string
			tableFrom string
			fieldFrom string
			tableSlug string
		)
		err = rows.Scan(
			&tableTo,
			&tableFrom,
			&fieldFrom,
		)
		if err != nil {
			return &nb.FieldsWithRelationsResponse{}, err
		}

		relation := &nb.RelationNew{}
		if tableFrom == req.TableSlug {
			relation = &nb.RelationNew{
				Slug:  fieldFrom + "_data",
				Label: tableTo,
			}
			tableSlug = tableTo
		} else {
			relation = &nb.RelationNew{
				Slug:  tableFrom,
				Label: tableFrom,
			}
			tableSlug = tableFrom
		}

		getTable, err := helper.GetTableByIdSlug(ctx, models.GetTableByIdSlugReq{Conn: conn, Slug: tableSlug})
		if err != nil {
			return &nb.FieldsWithRelationsResponse{}, err
		}
		query := `SELECT slug, label FROM "field" WHERE table_id = $1 AND type <> 'LOOKUP'`
		rows, err := conn.Query(ctx, query, cast.ToString(getTable["id"]))
		if err != nil {
			return &nb.FieldsWithRelationsResponse{}, err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				slug  string
				label string
			)
			err = rows.Scan(
				&slug,
				&label,
			)
			if err != nil {
				return &nb.FieldsWithRelationsResponse{}, err
			}

			relation.Fields = append(relation.Fields, &nb.FieldNew{Slug: slug, Label: label})
		}

		query = `SELECT table_to, table_from, field_from FROM "relation" WHERE (table_to = $1 OR table_from = $1) AND type = 'Many2One'`
		rows, err = conn.Query(ctx, query, tableSlug)
		if err != nil {
			return &nb.FieldsWithRelationsResponse{}, err
		}
		defer rows.Close()
		for rows.Next() {
			var (
				tableTo    string
				tableFrom  string
				fieldFrom  string
				tableSlug2 string
			)
			err = rows.Scan(
				&tableTo,
				&tableFrom,
				&fieldFrom,
			)
			if err != nil {
				return &nb.FieldsWithRelationsResponse{}, err
			}
			relation2 := &nb.RelationNew{}
			if tableFrom == tableSlug {
				relation2 = &nb.RelationNew{
					Slug:  fieldFrom + "_data",
					Label: tableTo,
				}
				tableSlug2 = tableTo
			} else {
				relation2 = &nb.RelationNew{
					Slug:  tableFrom,
					Label: tableFrom,
				}
				tableSlug2 = tableFrom
			}

			getTable, err := helper.GetTableByIdSlug(ctx, models.GetTableByIdSlugReq{Conn: conn, Slug: tableSlug2})
			if err != nil {
				return &nb.FieldsWithRelationsResponse{}, err
			}
			query := `SELECT slug, label FROM "field" WHERE table_id = $1 AND type <> 'LOOKUP'`
			rows, err := conn.Query(ctx, query, cast.ToString(getTable["id"]))
			if err != nil {
				return &nb.FieldsWithRelationsResponse{}, err
			}
			defer rows.Close()

			for rows.Next() {
				var (
					slug  string
					label string
				)
				err = rows.Scan(
					&slug,
					&label,
				)
				if err != nil {
					return &nb.FieldsWithRelationsResponse{}, err
				}

				relation2.Fields = append(relation2.Fields, &nb.FieldNew{Slug: slug, Label: label})
			}
			relation.Relations = append(relation.Relations, relation2)
		}

		resp.Relations = append(resp.Relations, relation)
	}

	return resp, nil
}

func (f *fieldRepo) CreateWithTx(ctx context.Context, req *nb.CreateFieldRequest, tableSlug string, tx pgx.Tx) (resp *nb.Field, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.Create")
	defer dbSpan.Finish()

	var (
		body                       []byte
		fields                     = []models.SectionFields{}
		layoutId, tabId, sectionId string
		sectionCount               int32
		ids                        []string
	)

	if req.Type == config.PERSON {
		var (
			fieldId    = uuid.NewString()
			tableLabel string
			query      = `SELECT slug, label FROM "table" WHERE id = $1`
		)

		if err := tx.QueryRow(ctx, query, req.TableId).Scan(&tableSlug, &tableLabel); err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select slug, label from table")
		}

		relationAttributes, err := helper.ConvertMapToStruct(map[string]any{
			"label_en":              "Person",
			"label_to_en":           tableLabel,
			"table_editable":        false,
			"enable_multi_language": false,
		})
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to convert map to struct relation attributes")
		}

		_, err = f.relationRepo.Create(ctx, &nb.CreateRelationRequest{
			Id:                uuid.NewString(),
			TableFrom:         tableSlug,
			TableTo:           config.PERSON_TABLE_SLUG,
			Type:              config.MANY2ONE,
			ViewFields:        []string{"f54d8076-4972-4067-9a91-c178c02c4273"},
			RelationTableSlug: config.PERSON_TABLE_SLUG,
			AutoFilters:       []*nb.AutoFilter{},
			RelationFieldId:   fieldId,
			Attributes:        relationAttributes,
			RelationToFieldId: uuid.NewString(),
			ProjectId:         req.ProjectId,
		})
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to create relation")
		}

		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: fieldId, ProjectId: req.ProjectId})
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	req.Slug = strings.ToLower(req.GetSlug())

	query := `INSERT INTO "field" (
		id,
		"table_id",
		"required",
		"slug",
		"label",
		"default",
		"type",
		"index",
		"attributes",
		"is_visible",
		autofill_field,
		autofill_table,
		"unique",
		"automatic",
		"is_search"
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)`

	attributes, err := json.Marshal(req.Attributes)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to marshal attributes")
	}

	_, err = tx.Exec(ctx, query,
		req.GetId(),
		req.GetTableId(),
		false,
		req.GetSlug(),
		req.GetLabel(),
		req.GetDefault(),
		req.GetType(),
		req.GetIndex(),
		attributes,
		req.GetIsVisible(),
		req.GetAutofillField(),
		req.GetAutofillTable(),
		req.GetUnique(),
		req.GetAutomatic(),
		true,
	)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to execute insert query")
	}

	query = `ALTER TABLE "` + tableSlug + `" ADD COLUMN ` + req.Slug + " " + helper.GetDataType(req.Type)

	_, err = tx.Exec(ctx, query)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to alter table")
	}

	query = `SELECT guid FROM "role"`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return nil, f.db.HandleDatabaseError(err, "Create field: failed to select guid from role")
	}
	defer rows.Close()

	for rows.Next() {
		var id string

		err = rows.Scan(&id)
		if err != nil {
			return nil, f.db.HandleDatabaseError(err, "Create field: failed to scan id")
		}

		ids = append(ids, id)
	}

	query = `INSERT INTO "field_permission" (
		"edit_permission",
		"view_permission",
		"table_slug",
		"field_id",
		"label",
		role_id
	) VALUES (true, true, $1, $2, $3, $4)`

	for _, id := range ids {
		_, err = tx.Exec(ctx, query, tableSlug, req.Id, req.Label, id)
		if err != nil {
			return nil, f.db.HandleDatabaseError(err, "Create field: failed to insert field_permission")
		}
	}

	query = `SELECT id FROM "layout" WHERE table_id = $1`
	err = tx.QueryRow(ctx, query, req.TableId).Scan(&layoutId)
	if err != nil && err != pgx.ErrNoRows {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select id from layout")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	query = `SELECT id FROM "tab" WHERE "layout_id" = $1 and type = 'section'`
	err = tx.QueryRow(ctx, query, layoutId).Scan(&tabId)
	if err != nil && err != pgx.ErrNoRows {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select id from tab")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	query = `SELECT id, fields FROM "section" WHERE tab_id = $1 ORDER BY created_at DESC LIMIT 1`
	err = tx.QueryRow(ctx, query, tabId).Scan(&sectionId, &body)
	if err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select id, fields from section")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	queryCount := `SELECT COUNT(*) FROM "section" WHERE tab_id = $1`
	err = tx.QueryRow(ctx, queryCount, tabId).Scan(&sectionCount)
	if err != nil && err != pgx.ErrNoRows {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to select count from section")
	} else if err == pgx.ErrNoRows {
		return f.GetByID(ctx, &nb.FieldPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
	}

	if err := json.Unmarshal(body, &fields); err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failded to unmarshal fields")
	}

	if len(fields) < 3 {
		query := `UPDATE "section" SET fields = $2 WHERE id = $1`

		fields = append(fields, models.SectionFields{
			Id:    req.Id,
			Order: len(fields) + 1,
		})

		reqBody, err := json.Marshal(fields)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to marshal fields")
		}

		_, err = tx.Exec(ctx, query, sectionId, reqBody)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to execute update section query")
		}
	} else {
		query = `INSERT INTO "section" ("order", "column", label, table_id, tab_id, fields) VALUES ($1, $2, $3, $4, $5, $6)`

		sectionId = uuid.NewString()

		fields := []models.SectionFields{{Id: req.Id, Order: 1}}

		reqBody, err := json.Marshal(fields)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to insert section")
		}

		_, err = tx.Exec(ctx, query, sectionCount+1, "SINGLE", "Info", req.TableId, tabId, reqBody)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to execute insert section query")
		}
	}

	if req.Type == config.INCREMENT_ID {
		query = `INSERT INTO "incrementseqs" (field_slug, table_slug) VALUES ($1, $2)`

		_, err = tx.Exec(ctx, query, req.Slug, tableSlug)
		if err != nil {
			return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to insert incementseqs query")
		}

	}

	query = `
		UPDATE "view"
		SET columns = array_append(columns, $1)
		WHERE table_slug = $2
	`
	_, err = tx.Exec(ctx, query, req.GetId(), tableSlug)
	if err != nil {
		return &nb.Field{}, f.db.HandleDatabaseError(err, "Create field: failed to execute update view query")
	}

	return resp, nil
}

func (f *fieldRepo) ObtainRandomOne(ctx context.Context, req *nb.ObtainRandomRequest) (resp *nb.ObtainRandomResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "field.Delete")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT f.id
	FROM field f
	JOIN "table" c ON f.table_id = c.id
	WHERE c.slug = $1
	  AND f.slug NOT IN ('guid', 'created_at', 'updated_at', 'deleted_at')
	ORDER BY RANDOM()
	LIMIT 1`

	var fieldID string
	err = conn.QueryRow(ctx, query, req.GetTableSlug()).Scan(&fieldID)
	if err != nil {
		return nil, err
	}

	resp = &nb.ObtainRandomResponse{
		Id: fieldID,
	}

	return resp, nil
}

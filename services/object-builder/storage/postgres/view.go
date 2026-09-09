package postgres

import (
	"context"
	"database/sql"
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
	"github.com/spf13/cast"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type viewRepo struct {
	db *psqlpool.Pool
}

func NewViewRepo(db *psqlpool.Pool) storage.ViewRepoI {
	return &viewRepo{
		db: db,
	}
}

func (v viewRepo) Create(ctx context.Context, req *nb.CreateViewRequest) (resp *nb.View, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "view.Create")
	defer dbSpan.Finish()

	var (
		viewId      string = req.Id
		ids                = []string{}
		relationIds        = []string{}
		menuId      *string
		tableSlug   = req.TableSlug
	)
	resp = &nb.View{}

	if req.MenuId != "" {
		menuId = &req.MenuId
	}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.View{}, errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if req.Type == helper.VIEW_TYPES["BOARD"] {
		err = helper.BoardOrderChecker(ctx, models.BoardOrder{Tx: tx, TableSlug: req.TableSlug})
		if err != nil {
			return &nb.View{}, errors.Wrap(err, "failed to check board order")
		}
	}

	if viewId == "" {
		viewId = uuid.NewString()
	}

	if req.IsRelationView {
		tableSlug = req.RelationTableSlug
	}

	err = tx.QueryRow(ctx, `
        SELECT 
			ARRAY_AGG(DISTINCT f.id) 
        FROM "table" AS t
        JOIN field AS f ON t.id = f.table_id
        WHERE t.slug = $1 AND f.slug NOT IN ('guid')
    `, tableSlug).Scan(&ids)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get column ids")
	}

	err = tx.QueryRow(ctx, `
		SELECT ARRAY_AGG(DISTINCT r.id)
		FROM "relation" AS r
		WHERE r.table_from = $1
	`, tableSlug).Scan(&relationIds)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get relation ids")
	}

	ids = append(ids, relationIds...)

	attributes, err := protojson.Marshal(req.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal attributes")
	}

	_, err = tx.Exec(ctx, `
        INSERT INTO "view" (
			"id",
			"table_slug",
			"type",
			"group_fields",
			"view_fields",
			"disable_dates",
			"quick_filters",
			"name",
			"columns",
			"calendar_from_slug",
			"calendar_to_slug",
			"relation_table_slug",
			"relation_id",
			"updated_fields",
			"order",
			"name_uz",
			"name_en",
			"attributes",
			"menu_id",
			"is_relation_view"
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
    	`, viewId,
		req.TableSlug,
		req.Type,
		req.GroupFields,
		req.ViewFields,
		req.DisableDates,
		req.QuickFilters,
		req.Name,
		ids,
		req.CalendarFromSlug,
		req.CalendarToSlug,
		req.RelationTableSlug,
		req.RelationId,
		req.UpdatedFields,
		req.Order,
		req.NameUz,
		req.NameEn,
		attributes,
		menuId,
		req.IsRelationView,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert view")
	}

	roles, err := tx.Query(ctx, `SELECT guid FROM role`)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get roles")
	}
	defer roles.Close()

	var roleValues []any
	for roles.Next() {
		var roleID string
		if err := roles.Scan(&roleID); err != nil {
			return nil, errors.Wrap(err, "failed to scan role")
		}
		roleValues = append(roleValues, uuid.New().String(), viewId, roleID, true, true, true)
	}

	if len(roleValues) > 0 {
		valueStrings := make([]string, 0, len(roleValues)/6)
		for i := 0; i < len(roleValues); i += 6 {
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)", i+1, i+2, i+3, i+4, i+5, i+6))
		}

		_, err := tx.Exec(ctx, `
			INSERT INTO view_permission (guid, view_id, role_id, "view", "edit", "delete")
			VALUES `+strings.Join(valueStrings, ","),
			roleValues...)
		if err != nil {
			return nil, errors.Wrap(err, "failed to insert view permissions")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return v.GetSingle(ctx, &nb.ViewPrimaryKey{Id: viewId, ProjectId: req.ProjectId})
}

func (v viewRepo) GetList(ctx context.Context, req *nb.GetAllViewsRequest) (resp *nb.GetAllViewsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "view.GetList")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var (
		filterField      = "table_slug"
		filterValue      = req.MenuId
		m                = make(map[string]bool, 0)
		is          bool = true
	)
	if _, err := uuid.Parse(req.MenuId); err == nil {
		filterField = "menu_id"
		filterValue = req.MenuId
		is = false
	}

	resp = &nb.GetAllViewsResponse{}
	query := fmt.Sprintf(`
        SELECT 
			COUNT(*) OVER() AS count,
			"id",
			"table_slug",
			"type",
			"name",
			"disable_dates",
			"columns",
			"quick_filters",
			"view_fields",
			"group_fields",
			"calendar_from_slug",
			"calendar_to_slug",
			"status_field_slug",
			"relation_table_slug",
			"relation_id",
			"updated_fields",
			"attributes",
			"function_path",
			"order",
			"name_uz",
			"name_en",
			is_relation_view,
			menu_id,
			(SELECT label FROM "table" WHERE slug = "view".relation_table_slug) AS table_label
	    FROM view
        WHERE %s = $1
        ORDER BY "order" ASC
    `, filterField)

	rows, err := conn.Query(ctx, query, filterValue)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var (
		TableSlug          sql.NullString
		Type               sql.NullString
		Name               sql.NullString
		CalendarFromSlug   sql.NullString
		CalendarToSlug     sql.NullString
		StatusFieldSlug    sql.NullString
		RelationTableSlug  sql.NullString
		RelationId         sql.NullString
		FunctionPath       sql.NullString
		Order              sql.NullInt32
		NameUz             sql.NullString
		NameEn             sql.NullString
		tableLabel, menuId sql.NullString
	)
	for rows.Next() {
		var row nb.View
		attributes := []byte{}

		err = rows.Scan(
			&resp.Count,
			&row.Id,
			&TableSlug,
			&Type,
			&Name,
			&row.DisableDates,
			&row.Columns,
			&row.QuickFilters,
			&row.ViewFields,
			&row.GroupFields,
			&CalendarFromSlug,
			&CalendarToSlug,
			&StatusFieldSlug,
			&RelationTableSlug,
			&RelationId,
			&row.UpdatedFields,
			&row.Attributes,
			&FunctionPath,
			&Order,
			&NameUz,
			&NameEn,
			&row.IsRelationView,
			&menuId,
			&tableLabel,
		)
		if err != nil {
			return nil, err
		}

		if Type.String == "SECTION" {
			m[TableSlug.String] = true
		}

		resp.Views = append(resp.Views, &nb.View{
			Id:                row.Id,
			TableSlug:         TableSlug.String,
			Type:              Type.String,
			Name:              Name.String,
			DisableDates:      row.DisableDates,
			Columns:           row.Columns,
			QuickFilters:      row.QuickFilters,
			ViewFields:        row.ViewFields,
			GroupFields:       row.GroupFields,
			CalendarFromSlug:  CalendarFromSlug.String,
			CalendarToSlug:    CalendarToSlug.String,
			StatusFieldSlug:   StatusFieldSlug.String,
			RelationTableSlug: RelationTableSlug.String,
			RelationId:        RelationId.String,
			UpdatedFields:     row.UpdatedFields,
			FunctionPath:      FunctionPath.String,
			Order:             Order.Int32,
			NameUz:            NameUz.String,
			NameEn:            NameEn.String,
			Attributes:        row.Attributes,
			IsRelationView:    row.IsRelationView,
			TableLabel:        tableLabel.String,
			MenuId:            menuId.String,
		})

		if len(attributes) > 0 {
			err = json.Unmarshal(attributes, &row.Attributes)
			if err != nil {
				return nil, err
			}
		}

		permissionsQuery := `
			SELECT role_id
			FROM view_permission
			WHERE view_id = $1 AND role_id = $2
`
		rows, err := conn.Query(ctx, permissionsQuery, row.Id, req.RoleId)
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		if rows.Next() {
			attributesMap := row.Attributes.AsMap()
			attributesMap["view_permissions"] = true
			row.Attributes, _ = structpb.NewStruct(attributesMap)
		} else {
			attributesMap := row.Attributes.AsMap()
			attributesMap["view_permissions"] = false
			row.Attributes, _ = structpb.NewStruct(attributesMap)
		}

		encodedAttributes, err := json.Marshal(row.Attributes)
		if err != nil {
			return nil, err
		}

		var attributesMap map[string]any
		err = json.Unmarshal(encodedAttributes, &attributesMap)
		if err != nil {
			return nil, err
		}

		structAttributes, err := structpb.NewStruct(attributesMap)
		if err != nil {
			return nil, err
		}

		row.Attributes = structAttributes
	}

	if is && !m[req.MenuId] {
		resp.Views = append(resp.Views, &nb.View{
			Id:        uuid.NewString(),
			TableSlug: req.MenuId,
			Type:      "SECTION",
		})

		resp.Count++
	}

	return

}

func (v *viewRepo) GetSingle(ctx context.Context, req *nb.ViewPrimaryKey) (resp *nb.View, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "view.GetSingle")
	defer dbSpan.Finish()
	resp = &nb.View{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			"id",
			"table_slug",
			"type",
			"name",
			"disable_dates",
			"columns",
			"quick_filters",
			"users",
			"view_fields",
			"group_fields",
			"calendar_from_slug",
			"calendar_to_slug",
			"time_interval",
			"multiple_insert",
			"status_field_slug",
			"is_editable",
			"relation_table_slug",
			"relation_id",
			"updated_fields",
			"app_id",
			"table_label",
			"default_limit",
			"default_editable",
			"navigate",
			"function_path",
			"order",
			"name_uz",
			"name_en",
			"attributes",
			is_relation_view
		FROM "view" 
		WHERE id = $1`

	var (
		attributes        []byte
		TableSlug         sql.NullString
		Type              sql.NullString
		Name              sql.NullString
		CalendarFromSlug  sql.NullString
		CalendarToSlug    sql.NullString
		TimeInterval      sql.NullInt32
		StatusFieldSlug   sql.NullString
		RelationTableSlug sql.NullString
		RelationId        sql.NullString
		AppId             sql.NullString
		TableLabel        sql.NullString
		DefaultLimit      sql.NullString
		FunctionPath      sql.NullString
		Order             sql.NullInt32
		NameUz            sql.NullString
		NameEn            sql.NullString
	)

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&TableSlug,
		&Type,
		&Name,
		&resp.DisableDates,
		&resp.Columns,
		&resp.QuickFilters,
		&resp.Users,
		&resp.ViewFields,
		&resp.GroupFields,
		&CalendarFromSlug,
		&CalendarToSlug,
		&TimeInterval,
		&resp.MultipleInsert,
		&StatusFieldSlug,
		&resp.IsEditable,
		&RelationTableSlug,
		&RelationId,
		&resp.UpdatedFields,
		&AppId,
		&TableLabel,
		&DefaultLimit,
		&resp.DefaultEditable,
		&resp.Navigate,
		&FunctionPath,
		&Order,
		&NameUz,
		&NameEn,
		&attributes,
		&resp.IsRelationView,
	)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(attributes, &resp.Attributes); err != nil {
		return nil, err
	}

	resp = &nb.View{
		Id:                resp.Id,
		TableSlug:         TableSlug.String,
		Type:              Type.String,
		Name:              Name.String,
		DisableDates:      resp.DisableDates,
		Columns:           resp.Columns,
		QuickFilters:      resp.QuickFilters,
		Users:             resp.Users,
		ViewFields:        resp.ViewFields,
		GroupFields:       resp.GroupFields,
		CalendarFromSlug:  CalendarFromSlug.String,
		CalendarToSlug:    CalendarToSlug.String,
		TimeInterval:      TimeInterval.Int32,
		MultipleInsert:    resp.MultipleInsert,
		StatusFieldSlug:   StatusFieldSlug.String,
		IsEditable:        resp.IsEditable,
		RelationTableSlug: RelationTableSlug.String,
		RelationId:        RelationId.String,
		UpdatedFields:     resp.UpdatedFields,
		AppId:             AppId.String,
		TableLabel:        TableLabel.String,
		DefaultLimit:      DefaultLimit.String,
		DefaultEditable:   resp.DefaultEditable,
		Navigate:          resp.Navigate,
		FunctionPath:      FunctionPath.String,
		Order:             Order.Int32,
		NameUz:            NameUz.String,
		NameEn:            NameEn.String,
		Attributes:        resp.Attributes,
		IsRelationView:    resp.IsRelationView,
	}

	return resp, nil
}

func (v viewRepo) Update(ctx context.Context, req *nb.View) (resp *nb.View, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "view.Update")
	defer dbSpan.Finish()
	var (
		query = "UPDATE view SET "
		args  = []any{}
		i     = 1
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.View{}, errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if req.Type == helper.VIEW_TYPES["BOARD"] {
		err = helper.BoardOrderChecker(ctx, models.BoardOrder{Tx: tx, TableSlug: req.TableSlug})
		if err != nil {
			return &nb.View{}, errors.Wrap(err, "failed to check board order")
		}
	}
	attributes, err := json.Marshal(req.Attributes)
	if err != nil {
		return &nb.View{}, errors.Wrap(err, "failed to marshal attributes")
	}

	atrb, err := helper.ConvertStructToMap(req.Attributes)
	if err != nil {
		return &nb.View{}, errors.Wrap(err, "failed to convert struct to map")
	}

	groupFields := cast.ToStringSlice(atrb["group_by_columns"])

	secondMap := make(map[string]struct{}, len(groupFields))
	for _, item := range groupFields {
		secondMap[item] = struct{}{}
	}

	result := req.Columns[:0]
	for _, item := range req.Columns {
		if _, found := secondMap[item]; !found {
			result = append(result, item)
		}
	}

	req.Columns = groupFields
	req.Columns = append(req.Columns, result...)

	if req.TableSlug != "" {
		query += fmt.Sprintf("table_slug = $%d, ", i)
		args = append(args, req.TableSlug)
		i++
	}
	if req.Type != "" {
		query += fmt.Sprintf("type = $%d, ", i)
		args = append(args, req.Type)
		i++
	}

	if len(req.ViewFields) != 0 {
		query += fmt.Sprintf("view_fields = $%d, ", i)
		args = append(args, req.ViewFields)
		i++
	}

	if len(req.QuickFilters) != 0 {
		query += fmt.Sprintf("quick_filters = $%d, ", i)
		args = append(args, req.QuickFilters)
		i++
	}
	if req.Name != "" {
		query += fmt.Sprintf("name = $%d, ", i)
		args = append(args, req.Name)
		i++
	}

	if req.Attributes != nil {
		query += fmt.Sprintf("attributes = $%d, ", i)
		args = append(args, attributes)
		i++
	}

	query += fmt.Sprintf("calendar_from_slug = $%d, ", i)
	args = append(args, req.CalendarFromSlug)
	i++

	query += fmt.Sprintf("calendar_to_slug = $%d, ", i)
	args = append(args, req.CalendarToSlug)
	i++

	query += fmt.Sprintf("group_fields = $%d, ", i)
	args = append(args, req.GroupFields)
	i++

	query += fmt.Sprintf("columns = $%d, ", i)
	args = append(args, req.Columns)
	i++

	query = query[:len(query)-2] + fmt.Sprintf(" WHERE id = $%d", i)
	args = append(args, req.Id)

	_, err = tx.Exec(ctx, query, args...)
	if err != nil {
		return &nb.View{}, errors.Wrap(err, "failed to update view")
	}

	if err = tx.Commit(ctx); err != nil {
		return &nb.View{}, errors.Wrap(err, "failed to commit transaction")
	}

	return v.GetSingle(ctx, &nb.ViewPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
}

func (v *viewRepo) Delete(ctx context.Context, req *nb.ViewPrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "view.Delete")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	var (
		filter    string
		condition any
	)
	if req.Id != "" {
		filter = "id"
		condition = req.Id
	} else if req.TableSlug != "" {
		filter = "table_slug"
		condition = req.TableSlug
	}

	query := `
		SELECT
			id,
			table_slug
		FROM view
		WHERE ` + filter + ` = $1
	`

	var (
		id        sql.NullString
		tableSlug sql.NullString
	)
	row := conn.QueryRow(ctx, query, condition)
	err = row.Scan(&id, &tableSlug)
	if err != nil {
		return err
	}

	_, err = conn.Exec(ctx, fmt.Sprintf("DELETE FROM view WHERE %v = $1", filter), condition)
	if err != nil {
		return err
	}

	return nil
}

func (v viewRepo) UpdateViewOrder(ctx context.Context, req *nb.UpdateViewOrderRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "view.UpdateViewOrder")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var i int
	for _, id := range req.Ids {
		_, err := tx.Exec(ctx, `UPDATE view SET "order" = $1, updated_at = NOW() WHERE id = $2`, i, id)
		if err != nil {
			return errors.Wrap(err, "failed to update view order")
		}
		i++
	}

	err = tx.Commit(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}
	return nil
}

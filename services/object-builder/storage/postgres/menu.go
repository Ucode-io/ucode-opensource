package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

type menuRepo struct {
	db *psqlpool.Pool
}

func NewMenuRepo(db *psqlpool.Pool) storage.MenuRepoI {
	return &menuRepo{
		db: db,
	}
}

func (m *menuRepo) Create(ctx context.Context, req *nb.CreateMenuRequest) (resp *nb.Menu, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.Create")
	defer dbSpan.Finish()

	if !config.MENU_TYPES[req.Type] {
		return &nb.Menu{}, errors.New("unsupported menu type")
	}

	var (
		parentId        any = req.ParentId
		tableId         any = req.TableId
		microfrontendId any = req.MicrofrontendId

		tableViewId   = uuid.NewString()
		sectionViewId = uuid.NewString()
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.Menu{}, errors.Wrap(err, "failed to start transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if len(req.Id) == 0 {
		req.Id = uuid.NewString()
	}

	jsonAttr, err := json.Marshal(req.Attributes)
	if err != nil {
		return &nb.Menu{}, err
	}

	if len(req.ParentId) == 0 {
		parentId = nil
	}
	if len(req.MicrofrontendId) == 0 {
		microfrontendId = nil
	}
	if len(req.TableId) == 0 {
		tableId = nil
	}

	if req.ParentId == "undefined" {
		parentId = "c57eedc3-a954-4262-a0af-376c65b5a284"
	}

	query := `INSERT INTO "menu" (
		id,
		label,
		parent_id,
		table_id,
		type,
		icon,
		attributes,
		microfrontend_id
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`

	_, err = tx.Exec(ctx, query,
		req.Id,
		req.Label,
		parentId,
		tableId,
		req.Type,
		req.Icon,
		jsonAttr,
		microfrontendId,
	)
	if err != nil {
		return &nb.Menu{}, errors.Wrap(err, "failed to insert menu")
	}

	if req.NewRouter && req.Type == "TABLE" {
		var tableSlug string
		query = `SELECT slug from "table" WHERE id = $1`
		err = tx.QueryRow(ctx, query, req.TableId).Scan(&tableSlug)
		if err != nil {
			return &nb.Menu{}, errors.Wrap(err, "failed to get table slug")
		}

		var (
			ids         = []string{}
			relationIds = []string{}
		)

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

		query = `
			INSERT INTO "view" ("id", "table_slug", "type", "menu_id", "columns")
			VALUES 
				($1, $2, $3, $4, $7),
				($5, $2, $6, $4, $7)
		`
		_, err = tx.Exec(
			ctx,
			query,
			tableViewId,
			tableSlug,
			"TABLE",
			req.Id,
			sectionViewId,
			"SECTION",
			ids,
		)
	}

	query = `SELECT guid FROM "role"`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return &nb.Menu{}, errors.Wrap(err, "failed to get roles")
	}
	defer rows.Close()

	query = `INSERT INTO "menu_permission" (
		menu_id,
		role_id
	) VALUES ($1, $2)`

	viewPermissionQuery := `INSERT INTO view_permission(
		"guid", 
		"view_id", 
		"role_id", 
		"view", 
		"edit", 
		"delete"
	) VALUES 
		($1, $2, $3, true, true, true),
        ($4, $5, $6, true, true, true)
	`

	roleIds := []string{}

	for rows.Next() {
		var roleId string

		err := rows.Scan(&roleId)
		if err != nil {
			return &nb.Menu{}, errors.Wrap(err, "failed to scan role id")
		}

		roleIds = append(roleIds, roleId)
	}

	for _, roleId := range roleIds {
		_, err = tx.Exec(ctx, query, req.Id, roleId)
		if err != nil {
			return &nb.Menu{}, errors.Wrap(err, "failed to insert menu permission")
		}

		if req.NewRouter && req.Type == "TABLE" {
			_, err = tx.Exec(
				ctx,
				viewPermissionQuery,
				uuid.NewString(),
				tableViewId,
				roleId,
				uuid.NewString(),
				sectionViewId,
				roleId,
			)
			if err != nil {
				return &nb.Menu{}, errors.Wrap(err, "failed to insert view permission")
			}
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return &nb.Menu{}, errors.Wrap(err, "failed to commit transaction")
	}

	return m.GetById(ctx, &nb.MenuPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
}

func (m *menuRepo) CreateWithTx(ctx context.Context, req *nb.CreateMenuRequest, tx pgx.Tx) (resp *nb.Menu, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.Create")
	defer dbSpan.Finish()

	if !config.MENU_TYPES[req.Type] {
		return &nb.Menu{}, errors.New("unsupported menu type")
	}

	var (
		parentId        any = req.ParentId
		layoutId        any = req.LayoutId
		tableId         any = req.TableId
		microfrontendId any = req.MicrofrontendId
	)

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if len(req.Id) == 0 {
		req.Id = uuid.NewString()
	}

	jsonAttr, err := json.Marshal(req.Attributes)
	if err != nil {
		return &nb.Menu{}, err
	}

	if len(req.ParentId) == 0 {
		parentId = nil
	}
	if len(req.LayoutId) == 0 {
		layoutId = nil
	}
	if len(req.MicrofrontendId) == 0 {
		microfrontendId = nil
	}
	if len(req.TableId) == 0 {
		tableId = nil
	}

	if req.ParentId == "undefined" {
		parentId = config.MenuParentId
	}

	query := `INSERT INTO "menu" (
		id,
		label,
		parent_id,
		layout_id,
		table_id,
		type,
		icon,
		attributes,
		microfrontend_id
	) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	_, err = tx.Exec(ctx, query,
		req.Id,
		req.Label,
		parentId,
		layoutId,
		tableId,
		req.Type,
		req.Icon,
		jsonAttr,
		microfrontendId,
	)
	if err != nil {
		return &nb.Menu{}, errors.Wrap(err, "failed to insert menu")
	}

	query = `SELECT guid FROM "role"`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return &nb.Menu{}, errors.Wrap(err, "failed to get roles")
	}
	defer rows.Close()

	query = `INSERT INTO "menu_permission" (
		menu_id,
		role_id
	) VALUES ($1, $2)`

	roleIds := []string{}

	for rows.Next() {
		var roleId string

		err := rows.Scan(&roleId)
		if err != nil {
			return &nb.Menu{}, errors.Wrap(err, "failed to scan role id")
		}

		roleIds = append(roleIds, roleId)

	}

	for _, roleId := range roleIds {
		_, err = tx.Exec(ctx, query, req.Id, roleId)
		if err != nil {
			return &nb.Menu{}, errors.Wrap(err, "failed to insert menu permission")
		}
	}

	return &nb.Menu{Id: req.Id}, nil
}

func (m *menuRepo) GetById(ctx context.Context, req *nb.MenuPrimaryKey) (resp *nb.Menu, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetById")
	defer dbSpan.Finish()

	var (
		id, label, parentId, layoutId            sql.NullString
		tableId, menuType, icon, microfrontendId sql.NullString
		webpageId, guid, menuId, roleId          sql.NullString
		isVisible, isStatic, write, read         sql.NullBool
		update, delete, menuSettings             sql.NullBool
		tId, tLabel, tSlug                       sql.NullString
		tIcon                                    sql.NullString
		tDesc                                    sql.NullString
		tSubtitleFieldSlug                       sql.NullString
		tShowInMenu                              sql.NullBool
		tIsChanged                               sql.NullBool
		tIsSystem                                sql.NullBool
		tIsSoftDelete                            sql.NullBool
		tIsCached                                sql.NullBool
		tIsLoginTable                            sql.NullBool
		tIsOrderBy                               sql.NullBool
		order, tIsSectionColumnCount             sql.NullInt16
		tIsWithIncrementId                       sql.NullBool
		attrTableData, isChangedByHost           sql.NullString
		attrData                                 []byte
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			m."id",
			m."label",
			m."parent_id",
			m."layout_id",
			m."table_id",
			m."type",
			m."icon",
			m."microfrontend_id",
			m."is_visible",
			m."is_static",
			m."order",
			m."webpage_id",
			m."attributes",
			mp."guid",
			mp."menu_id",
			mp."role_id",
			mp."write",
			mp."read",
			mp."update",
			mp."delete",
			mp."menu_settings",
			t."id",
			t."label",
			t."slug",
			t."icon",
			t."description",
			t."show_in_menu",
			t."subtitle_field_slug",
			t."is_changed",
			t."is_system",
			t."soft_delete",
			t."is_cached",
			t."is_changed_by_host",
			t."is_login_table",
			t."attributes",
			t."order_by",
			t."section_column_count",
			t."with_increment_id"
		FROM 
			"menu" m
		LEFT JOIN 
			"menu_permission" mp ON m."id" = mp."menu_id"
		LEFT JOIN
			"table" t ON m."table_id" = t."id"
		WHERE 
			m."id" = $1
	`

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&id,
		&label,
		&parentId,
		&layoutId,
		&tableId,
		&menuType,
		&icon,
		&microfrontendId,
		&isVisible,
		&isStatic,
		&order,
		&webpageId,
		&attrData,
		&guid,
		&menuId,
		&roleId,
		&write,
		&read,
		&update,
		&delete,
		&menuSettings,
		&tId,
		&tLabel,
		&tSlug,
		&tIcon,
		&tDesc,
		&tShowInMenu,
		&tSubtitleFieldSlug,
		&tIsChanged,
		&tIsSystem,
		&tIsSoftDelete,
		&tIsCached,
		&isChangedByHost,
		&tIsLoginTable,
		&attrTableData,
		&tIsOrderBy,
		&tIsSectionColumnCount,
		&tIsWithIncrementId,
	)
	if err != nil {
		return &nb.Menu{}, err
	}

	var attrDataStruct *structpb.Struct
	if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
		return &nb.Menu{}, err
	}

	var attrTableStruct *structpb.Struct
	if attrTableData.Valid {
		if err := json.Unmarshal([]byte(attrTableData.String), &attrTableStruct); err != nil {
			return &nb.Menu{}, err
		}
	}

	permission := map[string]any{
		"guid":          guid.String,
		"menu_id":       menuId.String,
		"role_id":       roleId.String,
		"write":         write.Bool,
		"read":          read.Bool,
		"update":        update.Bool,
		"delete":        delete.Bool,
		"menu_settings": menuSettings.Bool,
	}
	permissionStruct, err := helper.ConvertMapToStruct(permission)
	if err != nil {
		return &nb.Menu{}, err
	}

	var isChangedByHostStruct *structpb.Struct
	if isChangedByHost.Valid {
		if err := json.Unmarshal([]byte(isChangedByHost.String), &isChangedByHostStruct); err != nil {
			return &nb.Menu{}, err
		}
	}

	table := map[string]any{
		"id":                   tId.String,
		"label":                tLabel.String,
		"slug":                 tSlug.String,
		"icon":                 tIcon.String,
		"description":          tDesc.String,
		"show_in_menu":         tShowInMenu.Bool,
		"subtitle_field_slug":  tSubtitleFieldSlug.String,
		"is_changed":           tIsChanged.Bool,
		"is_system":            tIsSystem.Bool,
		"soft_delete":          tIsSoftDelete.Bool,
		"is_cached":            tIsCached.Bool,
		"is_changed_by_host":   isChangedByHostStruct,
		"is_login_table":       tIsLoginTable.Bool,
		"attributes":           attrTableStruct,
		"order_by":             tIsOrderBy.Bool,
		"section_column_count": tIsSectionColumnCount.Int16,
		"with_increment_id":    tIsWithIncrementId.Bool,
	}
	tableStruct, err := helper.ConvertMapToStruct(table)
	if err != nil {
		return &nb.Menu{}, err
	}

	data := map[string]any{
		"permission": permissionStruct,
		"table":      tableStruct,
	}
	dataStruct, err := helper.ConvertMapToStruct(data)
	if err != nil {
		return &nb.Menu{}, err
	}

	return &nb.Menu{
		Id:              id.String,
		Label:           label.String,
		ParentId:        parentId.String,
		LayoutId:        layoutId.String,
		TableId:         tableId.String,
		Type:            menuType.String,
		Icon:            icon.String,
		MicrofrontendId: microfrontendId.String,
		IsVisible:       isVisible.Bool,
		IsStatic:        isStatic.Bool,
		Order:           int32(order.Int16),
		WebpageId:       webpageId.String,
		Attributes:      attrDataStruct,
		Data:            dataStruct,
	}, nil
}

func (m *menuRepo) GetByLabel(ctx context.Context, req *nb.MenuPrimaryKey) (resp *nb.GetAllMenusResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetByLabel")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.GetAllMenusResponse{}

	query := `
		SELECT
			m."id",
			m."label",
			m."parent_id",
			m."layout_id",
			m."table_id",
			m."type",
			m."icon",
			m."microfrontend_id",
			m."is_static"
		FROM "menu" m
		WHERE m.label = $1
`

	rows, err := conn.Query(ctx, query, req.Label)
	if err != nil {
		return &nb.GetAllMenusResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id              sql.NullString
			label           sql.NullString
			parentId        sql.NullString
			layoutId        sql.NullString
			tableId         sql.NullString
			menuType        sql.NullString
			icon            sql.NullString
			microfrontendId sql.NullString
			isStatic        sql.NullBool
		)

		err := rows.Scan(
			&id,
			&label,
			&parentId,
			&layoutId,
			&tableId,
			&menuType,
			&icon,
			&microfrontendId,
			&isStatic,
		)
		if err != nil {
			return &nb.GetAllMenusResponse{}, err
		}
		resp.Menus = append(resp.Menus, &nb.MenuForGetAll{
			Id:              id.String,
			Label:           label.String,
			ParentId:        parentId.String,
			LayoutId:        layoutId.String,
			TableId:         tableId.String,
			Type:            menuType.String,
			Icon:            icon.String,
			MicrofrontendId: microfrontendId.String,
			IsStatic:        isStatic.Bool,
		})
	}

	return resp, nil
}

func (m *menuRepo) GetAll(ctx context.Context, req *nb.GetAllMenusRequest) (resp *nb.GetAllMenusResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetAll")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	params := make(map[string]any)
	resp = &nb.GetAllMenusResponse{}
	query := `
		SELECT 
			m."id",
			m."label",
			m."parent_id",
			m."layout_id",
			m."table_id",
			m."type",
			m."icon",
			m."microfrontend_id",
			m."is_visible",
			m."is_static",
			m."order",
			m."webpage_id",
			m."attributes",

			mp."guid",
			mp."menu_id",
			mp."role_id",
			mp."write",
			mp."read",
			mp."update",
			mp."delete",
			mp."menu_settings",

			t."id",
			t."label",
			t."slug",
			t."icon",
			t."description",
			t."show_in_menu",
			t."subtitle_field_slug",
			t."is_changed",
			t."is_system",
			t."soft_delete",
			t."is_cached",
			t."is_changed_by_host",
			t."is_login_table",
			COALESCE(t."attributes", '{}'::jsonb) AS attributes,
			t."order_by",
			t."section_column_count",
			t."with_increment_id"
		FROM "menu" m
		LEFT JOIN 
			"menu_permission" mp ON m."id" = mp."menu_id"
		LEFT JOIN
			"table" t ON m."table_id" = t."id"
		WHERE 1=1 `

	whereStr := ""
	if req.TableId != "" {
		whereStr += fmt.Sprintf(` AND m.table_id = '%v' `, req.TableId)
	} else {
		if req.ParentId != "" {
			whereStr += fmt.Sprintf(` AND m.parent_id = '%v' `, req.ParentId)
		} else if req.ParentId == "undefined" {
			whereStr += fmt.Sprintf(` AND m.parent_id = '%v' `, "c57eedc3-a954-4262-a0af-376c65b5a284")
		} else {
			whereStr += fmt.Sprintf(` AND m.id = '%v'`, "c57eedc3-a954-4262-a0af-376c65b5a284")
		}
	}

	if req.RoleId != "" {
		whereStr += fmt.Sprintf(` AND mp.role_id = '%s'`, req.RoleId)
	}

	query += whereStr
	query += ` ORDER BY m."order" ASC`

	if req.Offset >= 0 {
		query += ` OFFSET :offset `
		params["offset"] = req.Offset
	}
	if req.Limit > 0 {
		query += ` LIMIT :limit `
		params["limit"] = req.Limit
	}

	query, args := helper.ReplaceQueryParams(query, params)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.GetAllMenusResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id              sql.NullString
			label           sql.NullString
			parentId        sql.NullString
			layoutId        sql.NullString
			tableId         sql.NullString
			menuType        sql.NullString
			icon            sql.NullString
			microfrontendId sql.NullString
			isVisible       sql.NullBool
			isStatic        sql.NullBool
			order           sql.NullInt16
			webpageId       sql.NullString
			attrData        []byte

			guid         sql.NullString
			menuId       sql.NullString
			roleId       sql.NullString
			write        sql.NullBool
			read         sql.NullBool
			update       sql.NullBool
			delete       sql.NullBool
			menuSettings sql.NullBool

			tId                   sql.NullString
			tLabel                sql.NullString
			tSlug                 sql.NullString
			tIcon                 sql.NullString
			tDesc                 sql.NullString
			tShowInMenu           sql.NullBool
			tSubtitleFieldSlug    sql.NullString
			tIsChanged            sql.NullBool
			tIsSystem             sql.NullBool
			tIsSoftDelete         sql.NullBool
			tIsCached             sql.NullBool
			tIsLoginTable         sql.NullBool
			tIsOrderBy            sql.NullBool
			tIsSectionColumnCount sql.NullInt16
			tIsWithIncrementId    sql.NullBool
			attrTableData         sql.NullString
			isChangedByHost       sql.NullString
		)

		err := rows.Scan(
			&id,
			&label,
			&parentId,
			&layoutId,
			&tableId,
			&menuType,
			&icon,
			&microfrontendId,
			&isVisible,
			&isStatic,
			&order,
			&webpageId,
			&attrData,

			&guid,
			&menuId,
			&roleId,
			&write,
			&read,
			&update,
			&delete,
			&menuSettings,

			&tId,
			&tLabel,
			&tSlug,
			&tIcon,
			&tDesc,
			&tShowInMenu,
			&tSubtitleFieldSlug,
			&tIsChanged,
			&tIsSystem,
			&tIsSoftDelete,
			&tIsCached,
			&isChangedByHost,
			&tIsLoginTable,
			&attrTableData,
			&tIsOrderBy,
			&tIsSectionColumnCount,
			&tIsWithIncrementId,
		)
		if err != nil {
			return &nb.GetAllMenusResponse{}, nil
		}

		var attrDataStruct *structpb.Struct
		if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
			return &nb.GetAllMenusResponse{}, err
		}

		var attrTableStruct *structpb.Struct
		if attrTableData.Valid {
			if err := json.Unmarshal([]byte(attrTableData.String), &attrTableStruct); err != nil {
				return &nb.GetAllMenusResponse{}, err
			}
		}

		permission := map[string]any{
			"guid":          guid.String,
			"menu_id":       menuId.String,
			"role_id":       roleId.String,
			"write":         write.Bool,
			"read":          read.Bool,
			"update":        update.Bool,
			"delete":        delete.Bool,
			"menu_settings": menuSettings.Bool,
		}
		permissionStruct, err := helper.ConvertMapToStruct(permission)
		if err != nil {
			return &nb.GetAllMenusResponse{}, err
		}

		var isChangedByHostStruct *structpb.Struct
		if isChangedByHost.Valid {
			if err := json.Unmarshal([]byte(isChangedByHost.String), &isChangedByHostStruct); err != nil {
				return &nb.GetAllMenusResponse{}, err
			}
		}

		table := map[string]any{
			"id":                   tId.String,
			"label":                tLabel.String,
			"slug":                 tSlug.String,
			"icon":                 tIcon.String,
			"description":          tDesc.String,
			"show_in_menu":         tShowInMenu.Bool,
			"subtitle_field_slug":  tSubtitleFieldSlug.String,
			"is_changed":           tIsChanged.Bool,
			"is_system":            tIsSystem.Bool,
			"soft_delete":          tIsSoftDelete.Bool,
			"is_cached":            tIsCached.Bool,
			"is_changed_by_host":   isChangedByHostStruct,
			"is_login_table":       tIsLoginTable.Bool,
			"attributes":           attrTableStruct,
			"order_by":             tIsOrderBy.Bool,
			"section_column_count": tIsSectionColumnCount.Int16,
			"with_increment_id":    tIsWithIncrementId.Bool,
		}
		tableStruct, err := helper.ConvertMapToStruct(table)
		if err != nil {
			return &nb.GetAllMenusResponse{}, err
		}

		data := map[string]any{
			"permission": permissionStruct,
			"table":      tableStruct,
			"microfrontend": map[string]any{
				"id": microfrontendId.String,
			},
		}
		dataStruct, err := helper.ConvertMapToStruct(data)
		if err != nil {
			return &nb.GetAllMenusResponse{}, err
		}

		resp.Menus = append(resp.Menus, &nb.MenuForGetAll{
			Id:              id.String,
			Label:           label.String,
			ParentId:        parentId.String,
			LayoutId:        layoutId.String,
			TableId:         tableId.String,
			Type:            menuType.String,
			Icon:            icon.String,
			MicrofrontendId: microfrontendId.String,
			IsStatic:        isStatic.Bool,
			WebpageId:       webpageId.String,
			Attributes:      attrDataStruct,
			Data:            dataStruct,
		})
	}

	query = `SELECT COUNT(*) FROM "menu"`

	err = conn.QueryRow(ctx, query).Scan(&resp.Count)
	if err != nil {
		return &nb.GetAllMenusResponse{}, err
	}

	return resp, nil
}

func (m *menuRepo) Update(ctx context.Context, req *nb.Menu) (resp *nb.Menu, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetAll")
	defer dbSpan.Finish()

	if !config.MENU_TYPES[req.Type] {
		return &nb.Menu{}, errors.New("unsupported menu type")
	}

	var (
		parentId any = req.ParentId
		layoutId any = req.LayoutId
		tableId  any = req.TableId
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.ParentId == "" {
		parentId = nil
	}
	if req.LayoutId == "" {
		layoutId = nil
	}
	if req.TableId == "" {
		tableId = nil
	}

	query := `UPDATE "menu" SET
		"label" = $1,
		"parent_id" = $2,
		"layout_id" = $3,
		"table_id" = $4,
		"type" = $5,
		"icon" = $6,
		"attributes" = $7,
		"updated_at" = now()
	WHERE id = $8
 	`

	_, err = conn.Exec(ctx, query, req.Label, parentId, layoutId, tableId, req.Type, req.Icon, req.Attributes, req.Id)
	if err != nil {
		return &nb.Menu{}, err
	}

	return m.GetById(ctx, &nb.MenuPrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
}

func (m *menuRepo) UpdateMenuOrder(ctx context.Context, req *nb.UpdateMenuOrderRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.UpdateMenuOrder")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	for i, menu := range req.Menus {
		_, err := conn.Exec(ctx, `UPDATE menu SET "order" = $1 WHERE id = $2`, i+1, menu.Id)
		if err != nil {
			return err
		}
	}

	return nil
}

func (m *menuRepo) Delete(ctx context.Context, req *nb.MenuPrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.Delete")
	defer dbSpan.Finish()

	if config.STATIC_MENU_IDS[req.Id] {
		return errors.New("cannot delete default menu")
	}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `DELETE from "menu" WHERE id = $1`

	_, err = conn.Exec(ctx, query, req.Id)
	if err != nil {
		return err
	}

	query = `DELETE from "menu_permission" WHERE menu_id = $1`

	_, err = conn.Exec(ctx, query, req.Id)
	if err != nil {
		return err
	}

	return nil
}

func (m *menuRepo) GetAllMenuSettings(ctx context.Context, req *nb.GetAllMenuSettingsRequest) (*nb.GetAllMenuSettingsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetAllMenuSettings")
	defer dbSpan.Finish()

	var (
		params = make(map[string]any)
		resp   = &nb.GetAllMenuSettingsResponse{}
		query  = `
		SELECT 
			"id",
			"icon_style",
			"icon_size"
		FROM "menu_setting"`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.Offset >= 0 {
		query += ` OFFSET :offset `
		params["offset"] = req.Offset
	}
	if req.Limit > 0 {
		query += ` LIMIT :limit `
		params["limit"] = req.Limit
	}

	query, args := helper.ReplaceQueryParams(query, params)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.GetAllMenuSettingsResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id        string
			iconStyle sql.NullString
			iconSize  sql.NullString
		)

		if err := rows.Scan(
			&id,
			&iconStyle,
			&iconSize,
		); err != nil {
			return nil, err
		}

		resp.MenuSettings = append(resp.MenuSettings, &nb.MenuSettings{
			Id:        id,
			IconStyle: iconStyle.String,
			IconSize:  iconSize.String,
		})
	}

	query = `SELECT COUNT(*) FROM "menu_setting"`

	if err = conn.QueryRow(ctx, query).Scan(&resp.Count); err != nil {
		return &nb.GetAllMenuSettingsResponse{}, err
	}

	return resp, nil
}

func (m *menuRepo) GetByIDMenuSettings(ctx context.Context, req *nb.MenuSettingPrimaryKey) (*nb.MenuSettings, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetAllMenuSettings")
	defer dbSpan.Finish()
	var (
		resp  = &nb.MenuSettings{}
		query = `
			SELECT 
				"id",
				"icon_style",
				"icon_size"	
			FROM "menu_setting"
			WHERE id = $1`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if err := conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&resp.IconStyle,
		&resp.IconSize,
	); err != nil {
		return resp, err
	}

	resp.MenuTemplateId = req.TemplateId

	return resp, nil

}

func (m *menuRepo) GetAllMenuTemplate(ctx context.Context, req *nb.GetAllMenuSettingsRequest) (*nb.GatAllMenuTemplateResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetAllMenuTemplate")
	defer dbSpan.Finish()

	var (
		resp  = &nb.GatAllMenuTemplateResponse{}
		query = `SELECT 
			id,
			background,
			active_background,
			text,
			active_text,
			title
		FROM "menu_templates"`
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	rows, err := conn.Query(ctx, query)
	if err != nil {
		return &nb.GatAllMenuTemplateResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			id                string
			background        sql.NullString
			active_background sql.NullString
			text              sql.NullString
			active_text       sql.NullString
			title             sql.NullString
		)

		err := rows.Scan(
			&id,
			&background,
			&active_background,
			&text,
			&active_text,
			&title,
		)
		if err != nil {
			return nil, err
		}

		resp.MenuTemplates = append(resp.MenuTemplates, &nb.MenuTemplate{
			Id:               id,
			Background:       background.String,
			ActiveBackground: active_background.String,
			Text:             text.String,
			ActiveText:       active_text.String,
			Title:            title.String,
		})
	}

	query = `SELECT COUNT(*) FROM "menu_templates"`

	if err = conn.QueryRow(ctx, query).Scan(&resp.Count); err != nil {
		return &nb.GatAllMenuTemplateResponse{}, err
	}

	return resp, nil
}

func (m *menuRepo) GetMenuTemplateWithEntities(ctx context.Context, req *nb.GetMenuTemplateRequest) (resp *nb.MenuTemplateWithEntities, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetMenuTemplateWithEntities")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: psqlpool.Get")
	}

	tableIdToMenuId := map[string]string{}

	menuQuery := `
		WITH RECURSIVE menu_hierarchy AS (
			-- Base case: select the root menu
			SELECT id, label, parent_id, table_id, 0 AS depth
			FROM menu
			WHERE id = $1
			UNION ALL
			-- Recursive case: select sub-menus
			SELECT m.id, m.label, m.parent_id, m.table_id, mh.depth + 1
			FROM menu m
			INNER JOIN menu_hierarchy mh ON m.parent_id = mh.id
		)
		SELECT id, label, parent_id, table_id
		FROM menu_hierarchy
		ORDER BY depth, id
	`
	rows, err := conn.Query(ctx, menuQuery, req.MenuId)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: menu rows")
	}
	defer rows.Close()

	var menus []*nb.ProjectMenuTemplate
	for rows.Next() {
		var (
			menu     = &nb.ProjectMenuTemplate{}
			parentId sql.NullString
			tableId  sql.NullString
		)
		err := rows.Scan(
			&menu.Id,
			&menu.Label,
			&parentId,
			&tableId,
		)
		if err != nil {
			return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: get menus")
		}
		if parentId.Valid && parentId.String != "" {
			menu.ParentId = parentId.String
		}
		if tableId.Valid && tableId.String != "" {
			menu.TableId = tableId.String
			if _, exists := tableIdToMenuId[tableId.String]; !exists {
				tableIdToMenuId[tableId.String] = menu.Id
			}
		}
		menus = append(menus, menu)
	}

	if rows.Err() != nil {
		return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: get menus err")
	}

	var tables []*nb.TableTemplate
	tableIds := make([]string, 0, len(tableIdToMenuId))
	for tableId := range tableIdToMenuId {
		tableIds = append(tableIds, tableId)
	}

	tableQuery := `
		SELECT id, label, slug
		FROM "table"
		WHERE id = ANY($1)
		ORDER BY id
	`
	rows, err = conn.Query(ctx, tableQuery, tableIds)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: get table rows")
	}
	defer rows.Close()

	for rows.Next() {
		table := &nb.TableTemplate{}
		err := rows.Scan(
			&table.Id,
			&table.Label,
			&table.Slug,
		)
		if err != nil {
			return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: get tables")
		}
		if menuId, exists := tableIdToMenuId[table.Id]; exists {
			table.MenuId = menuId
		} else {
			table.MenuId = ""
		}
		tables = append(tables, table)
	}

	var fields []*nb.FieldTemplate
	fieldQuery := `
		SELECT id, table_id, slug, label, "type"
		FROM field
		WHERE table_id = ANY($1)
		ORDER BY table_id, id
	`
	rows, err = conn.Query(ctx, fieldQuery, tableIds)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: get field rows")
	}
	defer rows.Close()

	for rows.Next() {
		field := &nb.FieldTemplate{}
		err := rows.Scan(
			&field.Id,
			&field.TableId,
			&field.Slug,
			&field.Label,
			&field.Type,
		)
		if err != nil {
			return nil, helper.HandleDatabaseError(err, m.db.Logger, "GetMenuTemplateWithEntities: get fields")
		}
		fields = append(fields, field)
	}

	menuTemplateWithEntities := &nb.MenuTemplateWithEntities{
		Menus:  menus,
		Tables: tables,
		Fields: fields,
	}

	return menuTemplateWithEntities, nil
}

func (m *menuRepo) GetMenuTree(ctx context.Context, req *nb.MenuPrimaryKey) (*nb.MenuTree, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "menu.GetMenuTree")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	// Use the get_menu_tree function that was created in the migration
	query := `SELECT get_menu_tree($1)`

	var menuTreeJSON []byte
	err = conn.QueryRow(ctx, query, req.Id).Scan(&menuTreeJSON)
	if err != nil {
		return nil, err
	}

	// Parse the JSON response into a map
	var menuTreeMap map[string]interface{}
	if err := json.Unmarshal(menuTreeJSON, &menuTreeMap); err != nil {
		return nil, err
	}

	// Convert the map to MenuTree struct
	menuTree, err := m.convertMapToMenuTree(menuTreeMap)
	if err != nil {
		return nil, err
	}

	return menuTree, nil
}

func (m *menuRepo) convertMapToMenuTree(data map[string]interface{}) (*nb.MenuTree, error) {
	menuTree := &nb.MenuTree{}

	// Convert basic fields
	if id, ok := data["id"].(string); ok {
		menuTree.Id = id
	}
	if label, ok := data["label"].(string); ok {
		menuTree.Label = label
	}
	if icon, ok := data["icon"].(string); ok {
		menuTree.Icon = icon
	}
	if menuType, ok := data["type"].(string); ok {
		menuTree.Type = menuType
	}
	if parentId, ok := data["parent_id"].(string); ok {
		menuTree.ParentId = parentId
	}
	if microfrontendId, ok := data["microfrontend_id"].(string); ok {
		menuTree.MicrofrontendId = microfrontendId
	}
	if webpageId, ok := data["webpage_id"].(string); ok {
		menuTree.WebpageId = webpageId
	}
	if wikiId, ok := data["wiki_id"].(string); ok {
		menuTree.WikiId = wikiId
	}
	if bucketPath, ok := data["bucket_path"].(string); ok {
		menuTree.BucketPath = bucketPath
	}
	if layoutId, ok := data["layout_id"].(string); ok {
		menuTree.LayoutId = layoutId
	}
	if tableId, ok := data["table_id"].(string); ok {
		menuTree.TableId = tableId
	}
	if menuSettingsId, ok := data["menu_settings_id"].(string); ok {
		menuTree.MenuSettingsId = menuSettingsId
	}

	// Convert boolean fields
	if isVisible, ok := data["is_visible"].(bool); ok {
		menuTree.IsVisible = isVisible
	}
	if isStatic, ok := data["is_static"].(bool); ok {
		menuTree.IsStatic = isStatic
	}

	// Convert numeric fields
	if order, ok := data["order"].(float64); ok {
		menuTree.Order = int32(order)
	}

	// Convert attributes and data fields
	if attributes, ok := data["attributes"].(map[string]interface{}); ok {
		attrStruct, err := helper.ConvertMapToStruct(attributes)
		if err == nil {
			menuTree.Attributes = attrStruct
		}
	}
	if menuData, ok := data["data"].(map[string]interface{}); ok {
		dataStruct, err := helper.ConvertMapToStruct(menuData)
		if err == nil {
			menuTree.Data = dataStruct
		}
	}

	// Convert children recursively
	if children, ok := data["children"].([]interface{}); ok {
		for _, child := range children {
			if childMap, ok := child.(map[string]interface{}); ok {
				childMenuTree, err := m.convertMapToMenuTree(childMap)
				if err == nil {
					menuTree.Children = append(menuTree.Children, childMenuTree)
				}
			}
		}
	}

	return menuTree, nil
}

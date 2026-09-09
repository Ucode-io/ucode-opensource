package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/structpb"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
)

type tableRepo struct {
	db           *psqlpool.Pool
	logger       logger.LoggerI
	menuRepo     storage.MenuRepoI
	fieldRepo    storage.FieldRepoI
	relationRepo storage.RelationRepoI
}

func NewTableRepo(
	db *psqlpool.Pool,
	menuRepo storage.MenuRepoI,
	fieldRepo storage.FieldRepoI,
	relationRepo storage.RelationRepoI,
	logger logger.LoggerI,
) storage.TableRepoI {
	return &tableRepo{
		db:           db,
		logger:       logger,
		menuRepo:     menuRepo,
		fieldRepo:    fieldRepo,
		relationRepo: relationRepo,
	}
}

func (t *tableRepo) Create(ctx context.Context, req *nb.CreateTableRequest) (resp *nb.CreateTableResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.Create")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	jsonAttr, err := json.Marshal(req.Attributes)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to marshal attributes")
	}

	query := `INSERT INTO "table" (
		id, "slug", "label", "icon",
		"description", "show_in_menu",
		"subtitle_field_slug", "is_cached",
		"with_increment_id", "soft_delete",
		"digit_number", "is_changed_by_host", "attributes", is_login_table
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`

	data, err := helper.ChangeHostname([]byte(`{}`))
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to change hostname")
	}

	var (
		tableId       = uuid.NewString()
		fieldId       = uuid.NewString()
		tabId         = uuid.NewString()
		sectionViewId = uuid.NewString()
		viewID        = uuid.NewString()
		roleIds       = []string{}
		menuId        any
		parentMenuId  any
	)

	if req.Id != "" {
		tableId = req.Id
	}

	if req.MenuId != "" {
		parentMenuId = req.MenuId
	}

	if req.LayoutId == "" {
		req.LayoutId = uuid.NewString()
	}

	_, err = tx.Exec(ctx, query,
		tableId, req.Slug, req.Label, req.Icon, req.Description,
		req.ShowInMenu, req.SubtitleFieldSlug, req.IsCached,
		req.GetIncrementId().GetWithIncrementId(), req.SoftDelete,
		req.GetIncrementId().GetDigitNumber(), data, jsonAttr, req.IsLoginTable,
	)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert table")
	}

	query = `INSERT INTO "field" ( "table_id", "slug", "label", "default", "type", "index", id) 
			 VALUES ($1, 'guid', 'ID', 'uuid_generate_v4()', 'UUID', true, $2)`

	_, err = tx.Exec(ctx, query, tableId, fieldId)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert field")
	}

	query = `CREATE TABLE IF NOT EXISTS "` + req.Slug + `" (
		guid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        deleted_at TIMESTAMP
	)`

	_, err = tx.Exec(ctx, query)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to create table")
	}

	if req.MenuId != "" {
		menuId = uuid.NewString()
		parentMenuId = req.MenuId
		query = `INSERT INTO "menu" (
			id,
			label,
			parent_id,
			table_id,
			type,
			attributes,
			icon
		) VALUES ($1, $2, $3, $4, $5, $6, $7)`

		_, err = tx.Exec(ctx, query,
			menuId,
			req.Label,
			parentMenuId,
			tableId,
			"TABLE",
			req.Attributes,
			req.Icon,
		)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert menu")
		}
	}

	query = `INSERT INTO "layout" (
		id, "table_id", "order", "label", "icon", "type", "is_default", menu_id, "attributes", "is_visible_section", "is_modal" ) 
		VALUES ($1, $2, 1, 'Layout', '', 'PopupLayout', true, $3, $4, false, true)`

	_, err = tx.Exec(ctx, query, req.LayoutId, tableId, menuId, []byte(`{}`))
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert layout")
	}

	query = `INSERT INTO "tab" ("id", "order", "label", "icon", "type", "layout_id", "table_slug") 
			 VALUES ($1, 1, 'Tab', '', 'section', $2, $3)`

	_, err = tx.Exec(ctx, query, tabId, req.LayoutId, req.Slug)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert tab")
	}

	query = `INSERT INTO "section" ("id", "order", "column", "label", "icon", "table_id", "tab_id") 
			 VALUES ($1, 1, 'SINGLE', 'Info', '', $2, $3)`

	_, err = tx.Exec(ctx, query, uuid.NewString(), tableId, tabId)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert section")
	}

	query = `INSERT INTO "view" ("id", "table_slug", "type", "menu_id", "order")
			 VALUES ($1, $2, $3, $4, 0), ($5, $2, $6, $4, -1)`

	_, err = tx.Exec(ctx, query, viewID, req.Slug, "TABLE", menuId, sectionViewId, "SECTION")
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert view")
	}

	menuQuery := `
			INSERT INTO "menu_permission" (
				menu_id,
				role_id
			) VALUES ($1, $2)`

	query = `SELECT guid FROM role`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to select role")
	}
	defer rows.Close()

	for rows.Next() {
		var id string

		err = rows.Scan(&id)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to scan role")
		}

		roleIds = append(roleIds, id)
	}

	query = `INSERT INTO view_permission (guid, view_id, role_id, "view", "edit", "delete") 
			VALUES ($1, $2, $3, $4, $5, $6), ($7, $8, $3, $4, $5, $6)`

	recordPermission := `INSERT INTO record_permission (
		guid, role_id, table_slug, is_have_condition, delete, write, update,
		read, pdf_action, add_field, language_btn, view_create, automation,
		settings, share_modal, add_filter, field_filter, fix_column, tab_group,
		columns, "group", excel_menu, search_button) 
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	for _, id := range roleIds {
		_, err = tx.Exec(ctx, query,
			uuid.NewString(), viewID, id, true, true, true, uuid.NewString(), sectionViewId,
		)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert view permission")
		}

		_, err = tx.Exec(ctx, recordPermission,
			uuid.NewString(), id, req.Slug, true, "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes",
			"Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes",
		)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert record permission")
		}

		if req.MenuId != "" {
			_, err = tx.Exec(ctx, menuQuery, menuId, id)
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert menu permission")
			}
		}

	}

	if req.IsLoginTable {
		var (
			columns                                    []string
			clientTypeRelationCount, roleRelationCount int32
		)
		attributesMap, err := helper.ConvertStructToMap(req.Attributes)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "convert attributes struct to map")
		}

		attributesAuthInfo, ok := attributesMap["auth_info"].(map[string]any)
		if !ok {
			return &nb.CreateTableResponse{}, errors.New("auth_info does not exist")
		}

		loginStrategy, ok := attributesAuthInfo["login_strategy"].([]any)
		if !ok {
			return &nb.CreateTableResponse{}, errors.New("login_strategy does not exist")
		}

		withLastActivity := cast.ToBool(attributesMap[config.LAST_ACTIVITY])
		if withLastActivity {
			lastActivityAttributes, err := helper.ConvertMapToStruct(
				map[string]any{
					"attributes": map[string]any{
						"fields": map[string]any{
							"label_en": map[string]any{
								"stringValue": "Last activity",
								"kind":        "stringValue",
							},
						},
					},
				})
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct phone field attributes")
			}

			_, err = helper.UpsertLoginTableField(ctx, models.Field{
				Tx:         tx,
				Label:      "Last activity",
				Slug:       config.LAST_ACTIVITY,
				Type:       "DATE_TIME",
				TableId:    tableId,
				TableSlug:  req.Slug,
				Required:   false,
				ShowLabel:  true,
				Attributes: lastActivityAttributes,
				Default:    "",
				Index:      "string",
			})
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert last_activity field")
			}

		}

		var authInfo = map[string]any{
			"client_type_id": "client_type_id",
			"login_strategy": loginStrategy,
			"role_id":        "role_id",
		}

		for _, value := range loginStrategy {
			strategy := cast.ToString(value)
			switch cast.ToString(strategy) {
			case "phone":
				phoneAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Phone", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct phone field attributes")
				}

				passwordAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Password", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct password field attributes")
				}

				phoneFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Phone",
					Slug:       "phone",
					Type:       "INTERNATION_PHONE",
					TableId:    tableId,
					TableSlug:  req.Slug,
					ShowLabel:  true,
					Required:   false,
					Attributes: phoneAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert phone field")
				}

				passwordFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    tableId,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert password field")
				}

				columns = append(columns, passwordFieldId, phoneFieldId)
				authInfo["phone"] = "phone"
				authInfo["password"] = "password"
			case "login":
				loginAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Login", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct login field attributes")
				}

				passwordAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Password", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct password field attributes")
				}

				loginFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Login",
					Slug:       "login",
					Type:       "SINGLE_LINE",
					TableId:    tableId,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: loginAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert login field")
				}

				passwordFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    tableId,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert password field")
				}

				columns = append(columns, loginFieldId, passwordFieldId)
				authInfo["login"] = "login"
				authInfo["password"] = "password"
			case "email":
				emailAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Email", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct email field attributes")
				}

				emailFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Email",
					Slug:       "email",
					Type:       "EMAIL",
					TableId:    tableId,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: emailAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert email field")
				}

				passwordAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Password", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct password field attributes")
				}

				passwordFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    tableId,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert password field")
				}

				columns = append(columns, emailFieldId, passwordFieldId)
				authInfo["email"] = "email"
				authInfo["password"] = "password"
			case "e-imzo":
				tinAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "TIN", "kind": "integerValue"}}},
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct tin field attributes")
				}

				tinFieldId, err := helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "TIN",
					Slug:       "tin",
					Type:       "SINGLE_LINE",
					TableId:    tableId,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: tinAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.CreateTableResponse{}, errors.Wrap(err, "when upsert tin field")
				}

				columns = append(columns, tinFieldId)
				authInfo["tin"] = "tin"
			default:
				return &nb.CreateTableResponse{}, errors.New("Unknown strategy: " + cast.ToString(strategy))
			}
		}

		query = `SELECT COUNT(id) 
			FROM "relation" 
			WHERE table_from = $1 AND field_from = 'client_type_id' AND table_to = 'client_type' AND field_to = 'id'`

		err = tx.QueryRow(ctx, query, req.Slug).Scan(&clientTypeRelationCount)
		if err != nil && err != pgx.ErrNoRows {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "when get count client type relaion")
		}

		if clientTypeRelationCount == 0 {
			relationId := uuid.NewString()
			clientTypeAttributes, err := helper.ConvertMapToStruct(map[string]any{
				"label_en":              "Client Type",
				"label_to_en":           req.Label,
				"table_editable":        false,
				"enable_multi_language": false,
			})
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert ")
			}

			_, err = helper.CreateRelationWithTx(ctx, &models.CreateRelationRequest{
				Tx:                tx,
				Id:                relationId,
				TableFrom:         req.Slug,
				TableTo:           config.CLIENT_TYPE,
				Type:              config.MANY2ONE,
				ViewFields:        []string{"04d0889a-b9ba-4f5c-8473-c8447aab350d"},
				RelationTableSlug: config.CLIENT_TYPE,
				Attributes:        clientTypeAttributes,
				AutoFilters:       []*nb.AutoFilter{{FieldTo: "", FieldFrom: ""}},
				RelationFieldId:   uuid.NewString(),
			})
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "when create relation")
			}

			columns = append(columns, relationId)
		}

		query = `SELECT COUNT(id) 
				FROM "relation" 
				WHERE table_from = $1 AND field_from = 'role_id' AND table_to = 'role' AND field_to = 'id'`

		err = tx.QueryRow(ctx, query, req.Slug).Scan(&roleRelationCount)
		if err != nil && err != pgx.ErrNoRows {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "when get count client type relaion")
		}

		if roleRelationCount == 0 {
			relationId := uuid.NewString()

			roleAttributes, err := helper.ConvertMapToStruct(map[string]any{
				"label_en":              "Role",
				"label_to_en":           req.Label,
				"table_editable":        false,
				"enable_multi_language": false,
			})
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert role attributes")
			}

			_, err = helper.CreateRelationWithTx(ctx, &models.CreateRelationRequest{
				Tx:                tx,
				Id:                relationId,
				TableFrom:         req.Slug,
				TableTo:           config.ROLE,
				Type:              config.MANY2ONE,
				ViewFields:        []string{"c12adfef-2991-4c6a-9dff-b4ab8810f0df"},
				RelationTableSlug: config.ROLE,
				Attributes:        roleAttributes,
				AutoFilters:       []*nb.AutoFilter{{FieldTo: "client_type_id", FieldFrom: "client_type_id"}},
				RelationFieldId:   uuid.NewString(),
			})
			if err != nil {
				return &nb.CreateTableResponse{}, errors.Wrap(err, "when create relation")
			}

			columns = append(columns, relationId)
		}

		query = `
			UPDATE "view" SET 
				columns = $2
			WHERE id = $1`
		_, err = tx.Exec(ctx, query, viewID, columns)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.New("can't update view columns")
		}

		var attMap = map[string]any{
			"auth_info": authInfo,
			"label":     req.Label,
			"label_en":  req.Label,
		}

		if withLastActivity {
			attMap["last_activity"] = true
		}

		req.Attributes, err = helper.ConvertMapToStruct(attMap)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "when convert to struct auth_info")
		}

		query = `UPDATE "table" SET attributes = $2 WHERE id = $1`

		_, err = tx.Exec(ctx, query, tableId, req.Attributes)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "when update table attributes")
		}

		query = `INSERT INTO "field" (id, table_id, slug, label, type, is_visible, is_system, attributes) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    		ON CONFLICT (table_id, slug)
    		DO NOTHING`

		_, err = tx.Exec(ctx, query,
			uuid.NewString(), tableId, "user_id_auth",
			"User ID Auth", "UUID", false, true,
			`{"label_en":"UserIdAuth","label":"UserIdAuth","defaultValue":""}`,
		)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert field")
		}

		query = `ALTER TABLE IF EXISTS "` + req.GetSlug() + `" ADD COLUMN IF NOT EXISTS ` + ` user_id_auth` + ` UUID`
		_, _ = tx.Exec(ctx, query)
	}

	if err := tx.Commit(ctx); err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to commit transaction")
	}

	resp = &nb.CreateTableResponse{
		Id:                tableId,
		Label:             req.Label,
		Slug:              req.Slug,
		ShowInMenu:        req.ShowInMenu,
		Icon:              req.Icon,
		SubtitleFieldSlug: req.SubtitleFieldSlug,
		IsCached:          req.IsCached,
		DefaultEditable:   req.DefaultEditable,
		SoftDelete:        req.SoftDelete,
	}

	resp.Fields = append(resp.Fields,
		&nb.Field{
			Id:      fieldId,
			TableId: tableId,
			Slug:    "guid",
			Label:   "ID",
			Default: "uuid_generate_v4()",
			Type:    "UUID",
			Index:   "true",
		},
	)

	return resp, nil
}

func (t *tableRepo) GetByID(ctx context.Context, req *nb.TablePrimaryKey) (*nb.Table, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.GetByID")
	defer dbSpan.Finish()

	var (
		filter string = "id = $1"
		resp          = &nb.Table{IncrementId: &nb.IncrementID{}}
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	_, err = uuid.Parse(req.Id)
	if err != nil {
		filter = "slug = $1"
	}

	query := `SELECT 
		id,
		"slug",
		"label",
		"icon",
		"description",
		"show_in_menu",
		"subtitle_field_slug",
		"is_cached",
		"with_increment_id",
		"soft_delete",
		"order_by",
		"digit_number",
		"attributes",
		is_login_table
	FROM "table" WHERE ` + filter

	var attrData []byte

	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&resp.Id,
		&resp.Slug,
		&resp.Label,
		&resp.Icon,
		&resp.Description,
		&resp.ShowInMenu,
		&resp.SubtitleFieldSlug,
		&resp.IsCached,
		&resp.IncrementId.WithIncrementId,
		&resp.SoftDelete,
		&resp.OrderBy,
		&resp.IncrementId.DigitNumber,
		&attrData,
		&resp.IsLoginTable,
	)

	if err == pgx.ErrNoRows {
		return resp, nil
	} else if err != nil {
		return resp, errors.Wrap(err, "table get by id scan")
	}

	resp.Exists = true
	var attrDataStruct *structpb.Struct
	if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
		return &nb.Table{}, errors.Wrap(err, "unmarchal structpb table get by id")
	}

	resp.Attributes = attrDataStruct

	return resp, nil
}

func (t *tableRepo) GetAll(ctx context.Context, req *nb.GetAllTablesRequest) (resp *nb.GetAllTablesResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.GetAll")
	defer dbSpan.Finish()

	var (
		params = make(map[string]any)
		query  = `SELECT 
			id,
			"slug",
			"label",
			"icon",
			"description",
			"show_in_menu",
			"subtitle_field_slug",
			"is_changed",
			"with_increment_id",
			"soft_delete",
			"order_by",
			"digit_number",
			"attributes",
			is_login_table
		FROM "table" WHERE (is_system = false OR (slug = 'role' OR slug = 'client_type' OR slug = 'person' OR slug = 'sms_template')) `
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.GetAllTablesResponse{}

	if req.Search != "" {
		query += ` AND label ~* :label `
		params["label"] = req.Search
	}

	if req.IsLoginTable {
		query += ` AND is_login_table = true `
	}

	query += ` ORDER BY created_at DESC `

	if req.Limit != 0 && req.Limit > 0 {
		query += ` LIMIT :limit `
		params["limit"] = req.Limit
	}

	if req.Offset >= 0 {
		query += ` OFFSET :offset `
		params["offset"] = req.Offset
	}

	query, args := helper.ReplaceQueryParams(query, params)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.GetAllTablesResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		table := &nb.Table{
			IncrementId: &nb.IncrementID{},
		}

		var attrData []byte

		err = rows.Scan(
			&table.Id,
			&table.Slug,
			&table.Label,
			&table.Icon,
			&table.Description,
			&table.ShowInMenu,
			&table.SubtitleFieldSlug,
			&table.IsCached,
			&table.IncrementId.WithIncrementId,
			&table.SoftDelete,
			&table.OrderBy,
			&table.IncrementId.DigitNumber,
			&attrData,
			&table.IsLoginTable,
		)
		if err != nil {
			return &nb.GetAllTablesResponse{}, err
		}

		var attrDataStruct *structpb.Struct
		if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
			return &nb.GetAllTablesResponse{}, err
		}

		table.Attributes = attrDataStruct

		resp.Tables = append(resp.Tables, table)
	}

	query = `SELECT COUNT(*) FROM "table"  WHERE (is_system = false OR (slug = 'role' OR slug = 'client_type' OR slug = 'person' OR slug = 'sms_template'))`

	err = conn.QueryRow(ctx, query).Scan(&resp.Count)
	if err != nil {
		return &nb.GetAllTablesResponse{}, err
	}

	return resp, nil
}

func (t *tableRepo) Update(ctx context.Context, req *nb.UpdateTableRequest) (resp *nb.Table, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.Update")
	defer dbSpan.Finish()

	var (
		oldAttributes []byte
		isLoginTable  sql.NullBool
		guids         = []string{}
		createQuery   = `INSERT INTO "record_permission" (table_slug, role_id, read, update, write, delete, is_have_condition) 
						VALUES ($1, $2, 'Yes', 'Yes', 'Yes', 'Yes', false)`
		clientTypeRelationCount, roleRelationCount int32
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return &nb.Table{}, errors.Wrap(err, "failed to begin transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `SELECT is_login_table, attributes FROM "table" WHERE id = $1`

	err = tx.QueryRow(ctx, query, req.Id).Scan(&isLoginTable, &oldAttributes)
	if err != nil {
		return &nb.Table{}, errors.Wrap(err, "when get is_login_table")
	}

	query = `UPDATE "table" SET 
		"label" = $2,
		"icon" = $3,
		"description" = $4,
		"show_in_menu" = $5,
		"subtitle_field_slug" = $6,
		"is_cached" = $7,
		"with_increment_id" = $8,
		"soft_delete" = $9,
		"order_by" = $10,
		"digit_number" = $11,
		"attributes" = $12,
		is_login_table = $13
	WHERE id = $1`

	_, err = tx.Exec(ctx, query, req.Id,
		req.Label,
		req.Icon,
		req.Description,
		req.ShowInMenu,
		req.SubtitleFieldSlug,
		req.IsCached,
		req.IncrementId.WithIncrementId,
		req.SoftDelete,
		req.OrderBy,
		req.IncrementId.DigitNumber,
		req.Attributes,
		req.IsLoginTable,
	)
	if err != nil {
		return &nb.Table{}, errors.Wrap(err, "failed to update table")
	}

	query = `SELECT guid FROM "role" `

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return &nb.Table{}, errors.Wrap(err, "failed to select role")
	}
	defer rows.Close()

	query = `SELECT COUNT(*) FROM "record_permission" WHERE table_slug = $1 AND role_id = $2`

	for rows.Next() {
		var guid string

		if err = rows.Scan(&guid); err != nil {
			return &nb.Table{}, errors.Wrap(err, "failed to scan role")
		}

		guids = append(guids, guid)
	}

	for _, guid := range guids {
		var count = 0

		if err = tx.QueryRow(ctx, query, req.Slug, guid).Scan(&count); err != nil {
			return &nb.Table{}, errors.Wrap(err, "failed to select count")
		}

		if count == 0 {
			_, err = tx.Exec(ctx, createQuery, req.Slug, guid)
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "failed to insert record permission")
			}
		}
	}

	loginStrategyMap := helper.GetLoginStrategyMap(ctx, oldAttributes)
	if req.IsLoginTable {
		attributesMap, err := helper.ConvertStructToMap(req.Attributes)
		if err != nil {
			return &nb.Table{}, errors.Wrap(err, "convert attributes struct to map")
		}

		attributesAuthInfo, ok := attributesMap["auth_info"].(map[string]any)
		if !ok {
			return &nb.Table{}, errors.New("auth_info does not exist")
		}

		loginStrategy, ok := attributesAuthInfo["login_strategy"].([]any)
		if !ok {
			return &nb.Table{}, errors.New("login_strategy does not exist")
		}

		withLastActivity := cast.ToBool(attributesMap[config.LAST_ACTIVITY])
		if withLastActivity {
			lastActivityAttributes, err := helper.ConvertMapToStruct(
				map[string]any{
					"attributes": map[string]any{
						"fields": map[string]any{
							"label_en": map[string]any{
								"stringValue": "Last activity",
								"kind":        "stringValue",
							},
						},
					},
				})
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "when convert to struct phone field attributes")
			}

			_, err = helper.UpsertLoginTableField(ctx, models.Field{
				Tx:         tx,
				Label:      "Last activity",
				Slug:       config.LAST_ACTIVITY,
				Type:       "DATE_TIME",
				TableId:    req.Id,
				TableSlug:  req.Slug,
				Required:   false,
				ShowLabel:  true,
				Attributes: lastActivityAttributes,
				Default:    "",
				Index:      "string",
			})
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "when upsert last_activity field")
			}

		}

		var authInfo = map[string]any{
			"role_id":        "role_id",
			"client_type_id": "client_type_id",
			"login_strategy": loginStrategy,
		}

		for _, value := range loginStrategy {
			strategy := cast.ToString(value)
			oldVal, exist := loginStrategyMap[strategy]

			switch cast.ToString(strategy) {
			case "phone":
				if exist {
					authInfo["phone"] = oldVal
					authInfo["password"] = "password"
					continue
				}

				phoneAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Phone", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when convert to struct phone field attributes")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Phone",
					Slug:       "phone",
					Type:       "INTERNATION_PHONE",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: phoneAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert phone field")
				}

				passwordAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Password", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when convert to struct password field attributes")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert password field")
				}

				authInfo["phone"] = "phone"
				authInfo["password"] = "password"
			case "login":
				if exist {
					authInfo["login"] = oldVal
					authInfo["password"] = "password"
					continue
				}

				loginAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Login", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when convert to struct login field attributes")
				}

				passwordAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Password", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when convert to struct password field attributes")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert password field")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Login",
					Slug:       "login",
					Type:       "SINGLE_LINE",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: loginAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert login field")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert password field")
				}

				authInfo["login"] = "login"
				authInfo["password"] = "password"
			case "email":
				if exist {
					authInfo["email"] = oldVal
					authInfo["password"] = "password"
					continue
				}

				emailAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Email", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when convert to struct email field attributes")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Email",
					Slug:       "email",
					Type:       "EMAIL",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: emailAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert email field")
				}

				passwordAttributes, err := helper.ConvertMapToStruct(map[string]any{
					"attributes": map[string]any{"fields": map[string]any{"label_en": map[string]any{"stringValue": "Password", "kind": "stringValue"}}},
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when convert to struct password field attributes")
				}

				_, err = helper.UpsertLoginTableField(ctx, models.Field{
					Tx:         tx,
					Label:      "Password",
					Slug:       "password",
					Type:       "PASSWORD",
					TableId:    req.Id,
					TableSlug:  req.Slug,
					Required:   false,
					ShowLabel:  true,
					Attributes: passwordAttributes,
					Default:    "",
					Index:      "string",
				})
				if err != nil {
					return &nb.Table{}, errors.Wrap(err, "when upsert password field")
				}

				authInfo["password"] = "password"
				authInfo["email"] = "email"
			default:
				return &nb.Table{}, errors.New("Unknown strategy: " + cast.ToString(strategy))
			}
		}

		query = `SELECT COUNT(id) 
			FROM "relation" 
			WHERE table_from = $1 AND field_from = 'client_type_id' AND table_to = 'client_type' AND field_to = 'id'`

		err = tx.QueryRow(ctx, query, req.Slug).Scan(&clientTypeRelationCount)
		if err != nil && err != pgx.ErrNoRows {
			return &nb.Table{}, errors.Wrap(err, "when get count client type relaion")
		}

		if clientTypeRelationCount == 0 {
			clientTypeAttributes, err := helper.ConvertMapToStruct(map[string]any{
				"label_en":              "Client Type",
				"label_to_en":           req.Label,
				"table_editable":        false,
				"enable_multi_language": false,
			})
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "when convert ")
			}

			_, err = helper.CreateRelationWithTx(ctx, &models.CreateRelationRequest{
				Tx:                tx,
				Id:                uuid.NewString(),
				TableFrom:         req.Slug,
				TableTo:           config.CLIENT_TYPE,
				Type:              config.MANY2ONE,
				ViewFields:        []string{"04d0889a-b9ba-4f5c-8473-c8447aab350d"},
				RelationTableSlug: config.CLIENT_TYPE,
				Attributes:        clientTypeAttributes,
				AutoFilters:       []*nb.AutoFilter{{FieldTo: "", FieldFrom: ""}},
				RelationFieldId:   uuid.NewString(),
			})
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "when create relation")
			}
		}

		query = `SELECT COUNT(id) FROM "relation" 
				WHERE table_from = $1 AND field_from = 'role_id' AND table_to = 'role' AND field_to = 'id'`

		err = tx.QueryRow(ctx, query, req.Slug).Scan(&roleRelationCount)
		if err != nil && err != pgx.ErrNoRows {
			return &nb.Table{}, errors.Wrap(err, "when get count client type relaion")
		}

		if roleRelationCount == 0 {
			roleAttributes, err := helper.ConvertMapToStruct(map[string]any{
				"label_en":              "Role",
				"label_to_en":           req.Label,
				"table_editable":        false,
				"enable_multi_language": false,
			})
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "when convert role attributes")
			}

			_, err = helper.CreateRelationWithTx(ctx, &models.CreateRelationRequest{
				Tx:                tx,
				Id:                uuid.NewString(),
				TableFrom:         req.Slug,
				TableTo:           config.ROLE,
				Type:              config.MANY2ONE,
				ViewFields:        []string{"c12adfef-2991-4c6a-9dff-b4ab8810f0df"},
				RelationTableSlug: config.ROLE,
				Attributes:        roleAttributes,
				AutoFilters:       []*nb.AutoFilter{{FieldTo: "client_type_id", FieldFrom: "client_type_id"}},
				RelationFieldId:   uuid.NewString(),
			})
			if err != nil {
				return &nb.Table{}, errors.Wrap(err, "when create relation")
			}
		}

		var attMap = map[string]any{"auth_info": authInfo, "label": req.Label, "label_en": req.Label}

		if withLastActivity {
			attMap["last_activity"] = true
		}

		req.Attributes, err = helper.ConvertMapToStruct(attMap)
		if err != nil {
			return &nb.Table{}, errors.Wrap(err, "when convert to struct auth_info")
		}

		query = `UPDATE "table" SET attributes = $2 WHERE id = $1`

		_, err = tx.Exec(ctx, query, req.Id, req.Attributes)
		if err != nil {
			return &nb.Table{}, errors.Wrap(err, "when update table attributes")
		}

		query = `INSERT INTO "field" ( id, table_id, slug, label, type, is_visible, is_system, attributes) 
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
    		ON CONFLICT (table_id, slug)
    		DO NOTHING`

		_, err = tx.Exec(ctx, query,
			uuid.NewString(), req.Id, "user_id_auth",
			"User ID Auth", "UUID", false, true,
			`{"label_en":"UserIdAuth","label":"UserIdAuth","defaultValue":""}`,
		)
		if err != nil {
			return &nb.Table{}, errors.Wrap(err, "failed to insert field")
		}

		query = `ALTER TABLE IF EXISTS "` + req.GetSlug() + `" ADD COLUMN IF NOT EXISTS ` + ` user_id_auth` + ` UUID`
		_, _ = tx.Exec(ctx, query)
	}

	if err := tx.Commit(ctx); err != nil {
		return &nb.Table{}, errors.Wrap(err, "failed to commit transaction")
	}

	return t.GetByID(ctx, &nb.TablePrimaryKey{Id: req.Id, ProjectId: req.ProjectId})
}

func (t *tableRepo) Delete(ctx context.Context, req *nb.TablePrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.Delete")
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
		_ = tx.Rollback(ctx)
	}()

	var (
		query             = `SELECT is_system FROM "table" WHERE id = $1`
		slug              string
		isSystem          sql.NullBool
		layoutIds, tabIds []string
		relationIds       []string
	)

	err = tx.QueryRow(ctx, query, req.Id).Scan(&isSystem)
	if err != nil {
		return errors.Wrap(err, "failed select from table")
	}

	if isSystem.Valid && isSystem.Bool {
		return errors.New("system table can not be deleted")
	}

	query = `DELETE FROM "table" WHERE id = $1 RETURNING slug`

	err = tx.QueryRow(ctx, query, req.Id).Scan(&slug)
	if err != nil {
		return errors.Wrap(err, "failed to delete table")
	}

	query = fmt.Sprintf(`DROP TABLE IF EXISTS "%s"`, slug)

	_, err = tx.Exec(ctx, query)
	if err != nil {
		return helper.HandleDatabaseError(err, t.logger, "Create table: failed to drop table")
	}

	query = `SELECT id FROM layout WHERE table_id = $1`
	rows, err := tx.Query(ctx, query, req.Id)
	if err != nil {
		return errors.Wrap(err, "failed to select layout")
	}
	defer rows.Close()

	for rows.Next() {
		var layoutId string
		if err = rows.Scan(&layoutId); err != nil {
			return errors.Wrap(err, "failed to scan layout")
		}

		layoutIds = append(layoutIds, layoutId)
	}

	query = `SELECT id FROM tab WHERE layout_id = ANY($1)`
	rows, err = tx.Query(ctx, query, layoutIds)
	if err != nil {
		return errors.Wrap(err, "failed to select tab")
	}
	defer rows.Close()

	for rows.Next() {
		var tabId string
		if err = rows.Scan(&tabId); err != nil {
			return errors.Wrap(err, "failed to scan tab")
		}
		tabIds = append(tabIds, tabId)
	}

	query = `DELETE FROM section WHERE tab_id = ANY($1)`
	_, err = tx.Exec(ctx, query, tabIds)
	if err != nil {
		return errors.Wrap(err, "failed to delete from section")
	}

	query = `DELETE FROM tab WHERE id = ANY($1)`
	_, err = tx.Exec(ctx, query, tabIds)
	if err != nil {
		return errors.Wrap(err, "failed to delete from tab")
	}

	query = `DELETE FROM layout WHERE id = ANY($1)`
	_, err = tx.Exec(ctx, query, layoutIds)
	if err != nil {
		return errors.Wrap(err, "failed to delete from layout")
	}

	query = `SELECT id FROM relation WHERE table_from = $1 OR table_to = $1`
	rows, err = tx.Query(ctx, query, slug)
	if err != nil {
		return errors.Wrap(err, "failed to select relation")
	}
	defer rows.Close()

	for rows.Next() {
		var relationId string
		if err = rows.Scan(&relationId); err != nil {
			return errors.Wrap(err, "failed to scan relation")
		}
		relationIds = append(relationIds, relationId)
	}

	if len(relationIds) > 0 {
		query = `DELETE FROM relation WHERE id = ANY($1)`
		_, err = tx.Exec(ctx, query, relationIds)
		if err != nil {
			return errors.Wrap(err, "failed to delete from relation")
		}

		query = `DELETE FROM field WHERE relation_id = ANY($1)`
		_, err = tx.Exec(ctx, query, relationIds)
		if err != nil {
			return errors.Wrap(err, "failed to delete from field")
		}
	}

	query = `DELETE FROM field WHERE table_id = $1`
	_, err = tx.Exec(ctx, query, req.Id)
	if err != nil {
		return errors.Wrap(err, "failed to delete from field")
	}

	query = `DELETE FROM "field_permission" WHERE table_slug = $1`
	_, err = tx.Exec(ctx, query, slug)
	if err != nil {
		return errors.Wrap(err, "failed to delete from field_permission")
	}

	query = `DELETE FROM "record_permission" WHERE table_slug = $1`
	_, err = tx.Exec(ctx, query, slug)
	if err != nil {
		return errors.Wrap(err, "failed to delete from record_permission")
	}

	query = `DELETE FROM "view" WHERE table_slug = $1`
	_, err = tx.Exec(ctx, query, slug)
	if err != nil {
		return errors.Wrap(err, "failed to delete from view")
	}

	query = `DELETE FROM "menu" WHERE table_id = $1`
	_, err = tx.Exec(ctx, query, req.Id)
	if err != nil {
		return errors.Wrap(err, "failed to delete from menu")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "failed to commit transaction")
	}

	return nil
}

func (t *tableRepo) GetTablesByLabel(ctx context.Context, req *nb.GetTablesByLabelReq) (resp *nb.GetAllTablesResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.GetTablesByLabel")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.GetAllTablesResponse{}

	query := `SELECT 
		id,
		"slug",
		"label",
		"icon",
		"description",
		"show_in_menu",
		"subtitle_field_slug",
		"is_changed",
		"with_increment_id",
		"soft_delete",
		"order_by",
		"digit_number",
		"attributes",
		is_login_table
	FROM "table" WHERE (is_system = false OR (slug = 'role' OR slug = 'client_type')) AND label = $1`

	query += ` ORDER BY created_at DESC `

	rows, err := conn.Query(ctx, query, req.Label)
	if err != nil {
		return &nb.GetAllTablesResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		table := &nb.Table{
			IncrementId: &nb.IncrementID{},
		}

		var attrData []byte

		err := rows.Scan(
			&table.Id,
			&table.Slug,
			&table.Label,
			&table.Icon,
			&table.Description,
			&table.ShowInMenu,
			&table.SubtitleFieldSlug,
			&table.IsCached,
			&table.IncrementId.WithIncrementId,
			&table.SoftDelete,
			&table.OrderBy,
			&table.IncrementId.DigitNumber,
			&attrData,
			&table.IsLoginTable,
		)
		if err != nil {
			return &nb.GetAllTablesResponse{}, err
		}

		var attrDataStruct *structpb.Struct
		if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
			return &nb.GetAllTablesResponse{}, err
		}

		table.Attributes = attrDataStruct

		resp.Tables = append(resp.Tables, table)
	}

	return resp, nil
}

func (t *tableRepo) GetChart(ctx context.Context, req *nb.ChartPrimaryKey) (resp *nb.GetChartResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.ChartPrimaryKey")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var (
		tableIds   []string
		tableSlugs []string
	)

	tables := map[string]*nb.Table{}
	rows, err := conn.Query(ctx, `
        SELECT id, label, slug 
        FROM public.table 
        WHERE deleted_at IS NULL 
            AND (is_system = false OR slug IN ('role', 'client_type'))
    `)
	if err != nil {
		return &nb.GetChartResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		table := &nb.Table{}
		if err = rows.Scan(&table.Id, &table.Label, &table.Slug); err != nil {
			return &nb.GetChartResponse{}, err
		}

		tableIds = append(tableIds, table.Id)
		tableSlugs = append(tableSlugs, table.Slug)

		tables[table.Id] = table
	}

	fields := map[string][]*nb.Field{}
	rows, err = conn.Query(ctx, `
        SELECT table_id, slug, type 
        FROM public.field 
        WHERE deleted_at IS NULL
		AND table_id = ANY($1)
	`, tableIds)

	if err != nil {
		return &nb.GetChartResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var tableID, slug, fieldType string
		if err = rows.Scan(&tableID, &slug, &fieldType); err != nil {
			return &nb.GetChartResponse{}, err
		}
		fields[tableID] = append(fields[tableID], &nb.Field{Slug: slug, Type: helper.FIELD_TYPES[fieldType]})
	}

	relations := []*models.RelationForView{}
	rows, err = conn.Query(ctx, `
        SELECT table_from, table_to, field_from, field_to, type 
        FROM public.relation 
        WHERE deleted_at IS NULL AND is_system = false
		AND (table_from = ANY($1) OR table_to = ANY($1))
		GROUP BY table_from,table_to, field_from, field_to, type
    `, tableSlugs)
	if err != nil {
		return &nb.GetChartResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var tableFrom, tableTo, fieldFrom, fieldTo, relType string
		if err = rows.Scan(&tableFrom, &tableTo, &fieldFrom, &fieldTo, &relType); err != nil {
			return &nb.GetChartResponse{}, err
		}
		relations = append(relations, &models.RelationForView{
			TableFrom: tableFrom,
			TableTo:   tableTo,
			FieldFrom: fieldFrom,
			FieldTo:   fieldTo,
			Type:      relType,
		})
	}

	type tableOutput struct {
		slug   string
		fields []string
	}
	tableOutputs := make(map[string]tableOutput, len(tables))

	for id, table := range tables {
		fieldEntries := make([]string, 0, len(fields[id]))
		for _, f := range fields[id] {
			fieldEntries = append(fieldEntries, fmt.Sprintf("  %s %s",
				strings.ReplaceAll(f.Slug, "-", "_"),
				f.Type))
		}
		tableOutputs[id] = tableOutput{
			slug:   strings.ReplaceAll(table.Slug, "-", "_"),
			fields: fieldEntries,
		}
	}

	var sb strings.Builder

	for _, output := range tableOutputs {
		sb.WriteString(fmt.Sprintf("Table %s {\n%s\n}\n\n",
			output.slug,
			strings.Join(output.fields, "\n")))
	}

	for _, r := range relations {
		if r.Type == config.RECURSIVE {
			sb.WriteString(fmt.Sprintf("Ref: %s.%s > %s.%s\n",
				strings.ReplaceAll(r.TableFrom, "-", "_"),
				strings.ReplaceAll(r.FieldTo, "-", "_"),
				strings.ReplaceAll(r.TableTo, "-", "_"),
				"guid"))
		} else {
			sb.WriteString(fmt.Sprintf("Ref: %s.%s > %s.%s\n",
				strings.ReplaceAll(r.TableFrom, "-", "_"),
				strings.ReplaceAll(r.FieldFrom, "-", "_"),
				strings.ReplaceAll(r.TableTo, "-", "_"),
				"guid"))
		}
	}

	return &nb.GetChartResponse{
		Dbml: sb.String(),
	}, nil
}

// IMPORT TABLES
func (t *tableRepo) CreateConnectionAndSchema(ctx context.Context, req *nb.CreateConnectionAndSchemaReq) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.CreateConnectionAndSchema")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.ProjectId)
	if err != nil {
		return err
	}

	pool, err := pgxpool.New(ctx, req.ConnectionString)
	if err != nil {
		return err
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return err
	}

	schema, err := extractSchema(ctx, pool)
	if err != nil {
		return err
	}

	err = storeSchema(ctx, conn.Db, req.ConnectionString, req.Name, schema)
	if err != nil {
		return err
	}

	return nil
}

func extractSchema(ctx context.Context, pool *pgxpool.Pool) ([]models.TableSchema, error) {
	rows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (t.table_name, c.column_name)
			t.table_name,
			c.column_name,
			c.udt_name,
			BOOL_OR(tc.constraint_type = 'PRIMARY KEY') AS is_primary,
			BOOL_OR(tc.constraint_type = 'FOREIGN KEY') AS is_foreign,
			MAX(kcu.table_name) FILTER (WHERE tc.constraint_type = 'FOREIGN KEY') AS rel_table_from,
			MAX(ccu.table_name) FILTER (WHERE tc.constraint_type = 'FOREIGN KEY') AS rel_table_to
		FROM information_schema.tables t
		LEFT JOIN information_schema.columns c ON t.table_name = c.table_name
		LEFT JOIN information_schema.key_column_usage kcu ON c.table_name = kcu.table_name AND c.column_name = kcu.column_name
		LEFT JOIN information_schema.table_constraints tc ON kcu.constraint_name = tc.constraint_name
		LEFT JOIN information_schema.constraint_column_usage ccu ON tc.constraint_name = ccu.constraint_name
		WHERE t.table_schema NOT IN ('pg_catalog', 'information_schema')
		AND t.table_type = 'BASE TABLE'
		GROUP BY t.table_name, c.column_name, c.udt_name, c.ordinal_position
		ORDER BY t.table_name, c.column_name, c.ordinal_position
    `)
	if err != nil {
		return nil, fmt.Errorf("failed to query schema: %w", err)
	}
	defer rows.Close()

	schemas := make(map[string]*models.TableSchema)
	skipFields := map[string]bool{
		"created_at": true,
		"updated_at": true,
		"deleted_at": true,
		"guid":       true,
	}
	duplicateRels := map[string]bool{}

	for rows.Next() {
		var (
			tableName, columnName, dataType string
			isPrimary, isForeign            sql.NullBool
			relTableFrom, relTableTo        sql.NullString
		)

		if err := rows.Scan(
			&tableName, &columnName, &dataType, &isPrimary, &isForeign,
			&relTableFrom, &relTableTo,
		); err != nil {
			return nil, fmt.Errorf("failed to scan schema row: %w", err)
		}

		if _, ok := schemas[tableName]; !ok {
			schemas[tableName] = &models.TableSchema{Name: tableName}
		}

		if skipFields[columnName] {
			continue
		}

		if isForeign.Bool && relTableFrom.Valid && relTableTo.Valid {
			relKey := fmt.Sprintf("%s_%s", relTableFrom.String, relTableTo.String)
			if _, exists := duplicateRels[relKey]; exists {
				continue
			}
			duplicateRels[relKey] = true
			schemas[tableName].Relations = append(schemas[tableName].Relations, models.RelationInfo{
				TableFrom: relTableFrom.String,
				TableTo:   relTableTo.String,
			})
		} else {
			schemas[tableName].Columns = append(schemas[tableName].Columns, models.ColumnInfo{
				Name:     columnName,
				DataType: dataType,
			})
		}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("row iteration error: %w", err)
	}

	dependencies := make(map[string]map[string]bool)
	dependents := make(map[string]map[string]bool)
	allTables := make(map[string]bool)

	for tableName := range schemas {
		allTables[tableName] = true
		dependencies[tableName] = make(map[string]bool)
		dependents[tableName] = make(map[string]bool)
	}

	for tableName, schema := range schemas {
		for _, rel := range schema.Relations {
			if rel.TableTo != tableName {
				if _, exists := schemas[rel.TableTo]; exists {
					dependencies[tableName][rel.TableTo] = true
					dependents[rel.TableTo][tableName] = true
				}
			}
		}
	}

	var result []models.TableSchema
	processed := make(map[string]bool)

	for tableName := range allTables {
		if len(dependencies[tableName]) == 0 {
			result = append(result, *schemas[tableName])
			processed[tableName] = true
		}
	}

	for len(processed) < len(allTables) {
		progress := false
		for tableName := range allTables {
			if processed[tableName] {
				continue
			}

			allDepsProcessed := true
			for dep := range dependencies[tableName] {
				if !processed[dep] {
					allDepsProcessed = false
					break
				}
			}

			if allDepsProcessed {
				result = append(result, *schemas[tableName])
				processed[tableName] = true
				progress = true
			}
		}

		if !progress {
			var circularTables []string
			for tableName := range allTables {
				if !processed[tableName] {
					circularTables = append(circularTables, tableName)
				}
			}
			return nil, fmt.Errorf("circular dependency detected involving tables: %v", circularTables)
		}
	}

	return result, nil
}

func storeSchema(ctx context.Context, pool *pgxpool.Pool, connStr, name string, schemas []models.TableSchema) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	var connectionID string
	err = tx.QueryRow(ctx, `
        INSERT INTO tracked_connections (name, connection_string)
        VALUES ($1, $2)
        ON CONFLICT (connection_string) DO UPDATE
        SET name = EXCLUDED.name
        RETURNING id
    `, name, connStr).Scan(&connectionID)
	if err != nil {
		return fmt.Errorf("failed to insert tracked connection: %w", err)
	}

	batch := &pgx.Batch{}
	tableNames := make([]string, 0, len(schemas))

	for _, table := range schemas {
		tableNames = append(tableNames, table.Name)

		fields := make([]map[string]string, 0, len(table.Columns))
		for _, col := range table.Columns {
			fields = append(fields, map[string]string{
				"name": col.Name,
				"type": col.DataType,
			})
		}
		fieldsJSON, err := json.Marshal(fields)
		if err != nil {
			return fmt.Errorf("failed to marshal fields for table %s: %w", table.Name, err)
		}

		relations := make([]map[string]string, 0, len(table.Relations))
		for _, rel := range table.Relations {
			relations = append(relations, map[string]string{
				"table_from": rel.TableFrom,
				"table_to":   rel.TableTo,
			})
		}
		relationsJSON, err := json.Marshal(relations)
		if err != nil {
			return fmt.Errorf("failed to marshal relations for table %s: %w", table.Name, err)
		}

		batch.Queue(`
            INSERT INTO tracked_tables (connection_id, table_name, fields, relations)
            VALUES ($1, $2, $3, $4)
            ON CONFLICT (connection_id, table_name) DO UPDATE
            SET fields = EXCLUDED.fields, relations = EXCLUDED.relations
        `, connectionID, table.Name, fieldsJSON, relationsJSON)
	}

	br := tx.SendBatch(ctx, batch)
	defer br.Close()

	for i := range batch.Len() {
		_, err := br.Exec()
		if err != nil {
			return fmt.Errorf("failed to insert tracked table %s: %w", tableNames[i], err)
		}
	}

	if err := br.Close(); err != nil {
		return fmt.Errorf("failed to close batch results: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

func (t *tableRepo) GetTrackedUntrackedTables(ctx context.Context, req *nb.GetTrackedUntrackedTablesReq) (resp *nb.GetTrackedUntrackedTableResp, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.GetTrackedUntrackedTables")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			id,
			table_name,
			is_tracked
		FROM tracked_tables
		WHERE connection_id = $1
	`

	trackedTables := &nb.GetTrackedUntrackedTableResp{}
	rows, err := conn.Query(ctx, query, req.ConnectionId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		table := &nb.TrackedUntrackedTable{}

		err := rows.Scan(
			&table.Id,
			&table.TableName,
			&table.IsTracked,
		)
		if err != nil {
			return nil, err
		}

		trackedTables.Tables = append(trackedTables.Tables, table)
	}

	return trackedTables, nil
}

func (t *tableRepo) GetTrackedConnections(ctx context.Context, req *nb.GetTrackedConnectionsReq) (resp *nb.GetTrackedConnectionsResp, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.GetTrackedConnections")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			id,
			name,
			connection_string
		FROM tracked_connections
	`

	trackedConnections := &nb.GetTrackedConnectionsResp{}
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		connection := &nb.TrackedConnection{}

		err := rows.Scan(
			&connection.Id,
			&connection.Name,
			&connection.ConnectionString,
		)
		if err != nil {
			return nil, err
		}

		trackedConnections.Connections = append(trackedConnections.Connections, connection)
	}

	return trackedConnections, nil
}

func (t *tableRepo) TrackTables(ctx context.Context, req *nb.TrackedTablesByIdsReq) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "section.TrackedTablesByIds")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, `
		SELECT 
			"id",
			"table_name",
			"fields",
			"relations"
		FROM "tracked_tables"
		WHERE connection_id = $1 AND id = ANY($2)
	`, req.ConnectionId, req.TableIds)
	if err != nil {
		return err
	}
	defer rows.Close()

	tables := &nb.TrackedTablesByIdsResp{}
	for rows.Next() {
		table := &nb.TrackedUntrackedTable{}
		fields := []byte{}
		relations := []byte{}

		err = rows.Scan(
			&table.Id,
			&table.TableName,
			&fields,
			&relations,
		)
		if err != nil {
			return err
		}

		var fieldResps []*nb.FieldForTrackedUntrackedTable
		if err := json.Unmarshal(fields, &fieldResps); err != nil {
			return err
		}

		var relationResps []*nb.RelationForTrackedUntrackedTable
		if err := json.Unmarshal(relations, &relationResps); err != nil {
			return err
		}

		table.Fields = fieldResps
		table.Relations = relationResps
		tables.Tables = append(tables.Tables, table)
	}

	skipTables := map[string]bool{
		"schema_migrations": true,
	}

	for _, table := range tables.Tables {
		if skipTables[table.TableName] {
			continue
		}

		tableResp, err := t.CreateWithTx(ctx, &nb.CreateTableRequest{
			Label:      table.TableName,
			Slug:       table.TableName,
			ShowInMenu: true,
			ViewId:     uuid.NewString(),
			LayoutId:   uuid.NewString(),
			Attributes: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"label_en": structpb.NewStringValue(table.TableName),
				},
			},
			ProjectId: req.ProjectId,
		}, tx)
		if err != nil {
			return err
		}

		_, err = t.menuRepo.CreateWithTx(ctx, &nb.CreateMenuRequest{
			Label:    table.TableName,
			TableId:  tableResp.Id,
			Type:     "TABLE",
			ParentId: config.MenuParentId,
			Attributes: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"label_en": structpb.NewStringValue(table.TableName),
				},
			},
			ProjectId: req.ProjectId,
		}, tx)
		if err != nil {
			return err
		}

		for _, field := range table.Fields {
			_, err := t.fieldRepo.CreateWithTx(ctx, &nb.CreateFieldRequest{
				Id:      uuid.NewString(),
				TableId: tableResp.Id,
				Type:    helper.GetCustomToPostgres(field.Type),
				Label:   field.Name,
				Slug:    field.Name,
				Attributes: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"label_en": structpb.NewStringValue(field.Name),
					},
				},
				ProjectId: req.ProjectId,
			}, table.TableName, tx)
			if err != nil {
				return err
			}
		}

		for _, relation := range table.Relations {
			_, err := t.relationRepo.CreateWithTx(ctx, &nb.CreateRelationRequest{
				Id:        uuid.NewString(),
				Type:      "Many2One",
				TableFrom: relation.TableFrom,
				TableTo:   relation.TableTo,
				Attributes: &structpb.Struct{
					Fields: map[string]*structpb.Value{
						"label_en":    structpb.NewStringValue(relation.TableTo),
						"label_to_en": structpb.NewStringValue(relation.TableFrom),
					},
				},
				RelationFieldId:   uuid.NewString(),
				RelationToFieldId: uuid.NewString(),
				ProjectId:         req.ProjectId,
			}, tx)
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New(fmt.Sprintf("First track table %v", relation.TableTo))
			} else if err != nil {
				return err
			}
		}
	}

	query := `UPDATE tracked_tables SET is_tracked = true WHERE id = ANY($1) AND connection_id = $2`
	_, err = tx.Exec(ctx, query, req.TableIds, req.ConnectionId)
	if err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return err
	}

	return err
}

func (t *tableRepo) UntrackTableById(ctx context.Context, req *nb.UntrackTableByIdReq) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "section.UntrackTableById")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `
		SELECT
			id
		FROM "table"
		WHERE slug = (
			SELECT 
				table_name 
			FROM 
				tracked_tables
			WHERE id = $1 AND connection_id = $2
		)
	`

	var tableId string
	err = conn.QueryRow(ctx, query, req.TableId, req.ConnectionId).Scan(
		&tableId,
	)
	if err != nil {
		return err
	}

	query = `UPDATE tracked_tables SET is_tracked = false WHERE id = $1`
	_, err = conn.Exec(ctx, query, req.TableId)
	if err != nil {
		return err
	}

	return t.Delete(ctx, &nb.TablePrimaryKey{
		Id:        tableId,
		ProjectId: req.ProjectId,
	})
}

func (t *tableRepo) CreateWithTx(ctx context.Context, req *nb.CreateTableRequest, tx pgx.Tx) (resp *nb.CreateTableResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "table.Create")
	defer dbSpan.Finish()

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	jsonAttr, err := json.Marshal(req.Attributes)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to marshal attributes")
	}

	query := `INSERT INTO "table" (
		id, "slug", "label", "icon",
		"description", "show_in_menu",
		"subtitle_field_slug", "is_cached",
		"with_increment_id", "soft_delete",
		"digit_number", "is_changed_by_host", "attributes", is_login_table
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`

	data, err := helper.ChangeHostname([]byte(`{}`))
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to change hostname")
	}

	var tableId = uuid.NewString()

	_, err = tx.Exec(ctx, query,
		tableId, req.Slug, req.Label, req.Icon, req.Description,
		req.ShowInMenu, req.SubtitleFieldSlug, req.IsCached,
		req.GetIncrementId().GetWithIncrementId(), req.SoftDelete,
		req.GetIncrementId().GetDigitNumber(), data, jsonAttr, req.IsLoginTable,
	)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert table")
	}

	var fieldId = uuid.NewString()

	query = `INSERT INTO "field" ( "table_id", "slug", "label", "default", "type", "index", id) 
			 VALUES ($1, 'guid', 'ID', 'uuid_generate_v4()', 'UUID', true, $2)`

	_, err = tx.Exec(ctx, query, tableId, fieldId)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert field")
	}

	query = `CREATE TABLE IF NOT EXISTS "` + req.Slug + `" (
		guid UUID PRIMARY KEY DEFAULT gen_random_uuid(),
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        deleted_at TIMESTAMP
	)`

	_, err = tx.Exec(ctx, query)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to create table")
	}

	query = `INSERT INTO "layout" (
		id, "table_id", "order", "label", "icon", "type", "is_default", "attributes", "is_visible_section", "is_modal" ) 
		VALUES ($1, $2, 1, 'Layout', '', 'PopupLayout', true, $3, false, true)`

	_, err = tx.Exec(ctx, query, req.LayoutId, tableId, []byte(`{}`))
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert layout")
	}

	var tabId = uuid.NewString()

	query = `INSERT INTO "tab" ("id", "order", "label", "icon", "type", "layout_id", "table_slug") 
			 VALUES ($1, 1, 'Tab', '', 'section', $2, $3)`

	_, err = tx.Exec(ctx, query, tabId, req.LayoutId, req.Slug)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert tab")
	}

	query = `INSERT INTO "section" ("id", "order", "column", "label", "icon", "table_id", "tab_id") 
			 VALUES ($1, 1, 'SINGLE', 'Info', '', $2, $3)`

	_, err = tx.Exec(ctx, query, uuid.NewString(), tableId, tabId)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert section")
	}

	var viewID = uuid.NewString()

	query = `INSERT INTO "view" ("id", "table_slug", "type" )
			 VALUES ($1, $2, $3)`

	_, err = tx.Exec(ctx, query, viewID, req.Slug, "TABLE")
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert view")
	}

	var roleIds = []string{}
	query = `SELECT guid FROM role`

	rows, err := tx.Query(ctx, query)
	if err != nil {
		return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to select role")
	}
	defer rows.Close()

	for rows.Next() {
		var id string

		err = rows.Scan(&id)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to scan role")
		}

		roleIds = append(roleIds, id)
	}

	query = `INSERT INTO view_permission (guid, view_id, role_id, "view", "edit", "delete") 
			VALUES ($1, $2, $3, $4, $5, $6)`

	recordPermission := `INSERT INTO record_permission (
		guid, role_id, table_slug, is_have_condition, delete, write, update,
		read, pdf_action, add_field, language_btn, view_create, automation,
		settings, share_modal, add_filter, field_filter, fix_column, tab_group,
		columns, "group", excel_menu, search_button) 
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	for _, id := range roleIds {
		_, err = tx.Exec(ctx, query,
			uuid.NewString(), viewID, id, true, true, true,
		)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert view permission")
		}

		_, err = tx.Exec(ctx, recordPermission,
			uuid.NewString(), id, req.Slug, true, "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes",
			"Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes", "Yes",
		)
		if err != nil {
			return &nb.CreateTableResponse{}, errors.Wrap(err, "failed to insert record permission")
		}
	}

	resp = &nb.CreateTableResponse{
		Id:                tableId,
		Label:             req.Label,
		Slug:              req.Slug,
		ShowInMenu:        req.ShowInMenu,
		Icon:              req.Icon,
		SubtitleFieldSlug: req.SubtitleFieldSlug,
		IsCached:          req.IsCached,
		DefaultEditable:   req.DefaultEditable,
		SoftDelete:        req.SoftDelete,
	}

	resp.Fields = append(resp.Fields,
		&nb.Field{
			Id:      fieldId,
			TableId: tableId,
			Slug:    "guid",
			Label:   "ID",
			Default: "uuid_generate_v4()",
			Type:    "UUID",
			Index:   "true",
		},
	)

	return resp, nil
}

func (t *tableRepo) GetProjectTablesCount(ctx context.Context, projectId string) (int32, error) {
	conn, err := psqlpool.Get(projectId)
	if err != nil {
		return 0, err
	}

	var count int32
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM "table" WHERE is_system = false`).Scan(&count)
	return count, err
}

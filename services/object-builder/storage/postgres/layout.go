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

	"maps"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type layoutRepo struct {
}

func NewLayoutRepo() storage.LayoutRepoI {
	return &layoutRepo{}
}

func (l *layoutRepo) Update(ctx context.Context, req *nb.LayoutRequest) (resp *nb.LayoutResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.Update")
	defer dbSpan.Finish()

	var (
		roleGUID, layoutId                         string
		roles                                      []string
		mapTabs, mapSections                       = make(map[string]int), make(map[string]int)
		existingPermissions                        = make(map[string]bool)
		deletedSectionIds, deletedTabIds           []string
		relationIds, tab_ids                       []string
		bulkWriteTabValues, bulkWriteSectionValues []any
		insertManyRelationExist                    bool
		tabArgs                                    = 1
		sectionArgs                                = 1

		insertManyRelationQuery = `INSERT INTO view_relation_permission (
			"role_id", "table_slug", "relation_id", "view_permission", 
			"create_permission", "edit_permission", "delete_permission"
			) VALUES `
		bulkWriteTabQuery = `INSERT INTO "tab" (
				"id", "label", "layout_id",  "type",
				"order", "icon", relation_id, "attributes", "view_type"
			) VALUES `
		bulkWriteSectionQuery = `INSERT INTO "section" (
				"id", "tab_id", "label", "order", "icon", 
				"column", "is_summary_section", "fields", "table_id", "attributes"
			) VALUES `
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "error starting transaction")
	}
	defer tx.Rollback(ctx)

	resp = &nb.LayoutResponse{}
	rows, err := tx.Query(ctx, "SELECT guid FROM role")
	if err != nil {
		return nil, errors.Wrap(err, "error fetching roles")
	}
	defer rows.Close()

	for rows.Next() {
		if err := rows.Scan(&roleGUID); err != nil {
			return nil, errors.Wrap(err, "error scanning role GUID")
		}
		roles = append(roles, roleGUID)
	}

	layoutId = req.Id
	if req.Id == "" {
		layoutId = uuid.New().String()
	}

	result, err := helper.TableVer(ctx, models.TableVerReq{Tx: tx, Id: req.TableId})
	if err != nil {
		return nil, errors.Wrap(err, "error verifying table")
	}

	if _, ok := result["slug"].(string); !ok {
		return nil, errors.New("tableSlug not found or not a string")
	}

	attributesJSON, err := json.Marshal(req.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "error marshaling attributes to JSON")
	}

	query := `
        INSERT INTO "layout" (
            "id", "label", "order", "type", "icon", "is_default", 
            "is_modal", "is_visible_section",
             "table_id", "menu_id", "attributes"
        ) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
        ON CONFLICT (id) DO UPDATE
        SET 
            "label" = EXCLUDED.label,
            "order" = EXCLUDED.order,
            "type" = EXCLUDED.type,
            "icon" = EXCLUDED.icon,
            "is_default" = EXCLUDED.is_default,
            "is_modal" = EXCLUDED.is_modal,
            "is_visible_section" = EXCLUDED.is_visible_section,
            "table_id" = EXCLUDED.table_id,
            "menu_id" = EXCLUDED.menu_id,
			"attributes" = EXCLUDED.attributes
    `
	_, err = tx.Exec(ctx, query,
		layoutId, req.Label, req.Order, req.Type, req.Icon,
		req.IsDefault, req.IsModal, req.IsVisibleSection,
		req.TableId, req.MenuId, attributesJSON)
	if err != nil {
		return nil, errors.Wrap(err, "error inserting layout")
	}

	if req.IsDefault {
		_, err = tx.Exec(ctx, `
            UPDATE layout
            SET is_default = false
            WHERE table_id = $1 AND id != $2
        `, req.TableId, layoutId)
		if err != nil {
			return nil, errors.Wrap(err, "error updating layout")
		}
	}

	rows, err = tx.Query(ctx, "SELECT id FROM tab WHERE layout_id = $1", layoutId)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching tabs")
	}
	defer rows.Close()

	for rows.Next() {
		var tabId string
		if err := rows.Scan(&tabId); err != nil {
			return nil, errors.Wrap(err, "error scanning tab ID")
		}
		mapTabs[tabId] = 1
		tab_ids = append(tab_ids, tabId)
	}

	rows, err = tx.Query(ctx, "SELECT id FROM section WHERE tab_id = ANY($1)", pq.Array(tab_ids))
	if err != nil {
		return nil, errors.Wrap(err, "error fetching sections")
	}
	defer rows.Close()

	for rows.Next() {
		var sectionId string
		if err := rows.Scan(&sectionId); err != nil {
			return nil, errors.Wrap(err, "error scanning section ID")
		}
		mapSections[sectionId] = 1
	}

	for i := range req.Tabs {
		tab := req.Tabs[i]
		if tab.Id == "" {
			tab.Id = uuid.New().String()
		}
		if tab.Type == "relation" {
			relationIds = append(relationIds, tab.RelationId)
		}

		if _, ok := mapTabs[tab.Id]; ok {
			mapTabs[tab.Id] = 2
		}
		attributesJSON, err := json.Marshal(tab.Attributes)
		if err != nil {
			return nil, fmt.Errorf("error marshaling attributes to JSON: %w", err)
		}

		if tab.RelationId != "" {
			bulkWriteTabQuery += fmt.Sprintf(`($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d) `,
				tabArgs, tabArgs+1, tabArgs+2, tabArgs+3, tabArgs+4, tabArgs+5, tabArgs+6, tabArgs+7, tabArgs+8,
			)
			bulkWriteTabValues = append(bulkWriteTabValues, tab.Id, tab.Label, layoutId, tab.Type, i+1, tab.Icon, tab.RelationId, string(attributesJSON), tab.ViewType)
			tabArgs += 9

			if i != len(req.Tabs)-1 {
				bulkWriteTabQuery += ","
			}

		} else {
			bulkWriteTabQuery += fmt.Sprintf(`($%d, $%d, $%d, $%d, $%d, $%d, NULL, $%d, $%d) `,
				tabArgs, tabArgs+1, tabArgs+2, tabArgs+3, tabArgs+4, tabArgs+5, tabArgs+6, tabArgs+7,
			)
			bulkWriteTabValues = append(bulkWriteTabValues, tab.Id, tab.Label, layoutId, tab.Type, i+1, tab.Icon, string(attributesJSON), tab.ViewType)
			tabArgs += 8

			if i != len(req.Tabs)-1 {
				bulkWriteTabQuery += ","
			}

		}

		for i, section := range tab.Sections {
			if section.Id == "" {
				section.Id = uuid.New().String()
			}

			if _, ok := mapSections[section.Id]; ok {
				mapSections[section.Id] = 2
			}

			jsonFields := []byte(`[]`)

			if section.Fields != nil {
				jsonFields, err = json.Marshal(section.Fields)
				if err != nil {
					return nil, errors.Wrap(err, "error marshaling section fields to JSON")
				}
			}

			attributes := []byte(`{}`)

			if section.Attributes != nil {
				attributes, err = json.Marshal(section.Attributes)
				if err != nil {
					return nil, fmt.Errorf("error marshaling section attributes to JSON: %w", err)
				}
			}

			bulkWriteSectionQuery += fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				sectionArgs, sectionArgs+1, sectionArgs+2, sectionArgs+3, sectionArgs+4,
				sectionArgs+5, sectionArgs+6, sectionArgs+7, sectionArgs+8, sectionArgs+9)

			sectionArgs += 10

			bulkWriteSectionValues = append(bulkWriteSectionValues, section.Id, tab.Id, section.Label, i,
				section.Icon, section.Column, section.IsSummarySection, jsonFields, req.TableId, attributes)

			if i != len(tab.Sections)-1 {
				bulkWriteSectionQuery += ","
			}
		}

	}

	for key, value := range mapTabs {
		if value == 1 {
			deletedTabIds = append(deletedTabIds, key)
		}
	}

	for key, value := range mapSections {
		if value == 1 {
			deletedSectionIds = append(deletedSectionIds, key)
		}
	}

	query = `SELECT role_id, table_slug, relation_id
		FROM view_relation_permission
		WHERE role_id = ANY($1) AND table_slug = $2 AND relation_id = ANY($3)
	`

	rows, err = tx.Query(ctx, query, roles, req.TableId, relationIds)
	if err != nil {
		return nil, errors.Wrap(err, "error when get view_relation_permission")
	}

	defer rows.Close()
	for rows.Next() {
		var roleID, tableSlug, relationID sql.NullString
		if err := rows.Scan(&roleID, &tableSlug, &relationID); err != nil {
			return nil, errors.Wrap(err, "error scanning relation permissions")
		}
		key := fmt.Sprintf("%s_%s_%s", roleID.String, tableSlug.String, relationID.String)
		existingPermissions[key] = true
	}

	for _, roleId := range roles {
		for _, relationID := range relationIds {
			key := fmt.Sprintf("%s_%s_%s", roleId, req.TableId, relationID)
			if !existingPermissions[key] {
				if insertManyRelationExist {
					insertManyRelationQuery += ","
				}
				insertManyRelationQuery += fmt.Sprintf("('%s', '%s', '%s', true, true, true, true)", roleId, req.TableId, relationID)
				insertManyRelationExist = true
			}
		}
	}

	if insertManyRelationExist {
		_, err := tx.Exec(ctx, insertManyRelationQuery)
		if err != nil {
			return nil, errors.Wrap(err, "error when insert many view_relation_permission")
		}
	}

	if len(deletedTabIds) > 0 {
		_, err := tx.Exec(ctx, "DELETE FROM tab WHERE id = ANY($1)", pq.Array(deletedTabIds))
		if err != nil {
			return nil, errors.Wrap(err, "error deleting tabs")
		}
	}

	if len(deletedSectionIds) > 0 {
		_, err := tx.Exec(ctx, "DELETE FROM section WHERE id = ANY($1)", pq.Array(deletedSectionIds))
		if err != nil {
			return nil, errors.Wrap(err, "error deleting sections")
		}
	}

	if len(bulkWriteTabValues) > 0 {
		bulkWriteTabQuery += ` ON CONFLICT (id) DO UPDATE
			SET
				"label" = EXCLUDED.label,
				"layout_id" = EXCLUDED.layout_id,
				"type" = EXCLUDED.type,
				"order" = EXCLUDED.order,
				"icon" = EXCLUDED.icon,
				"relation_id" = EXCLUDED.relation_id,
				"attributes" = EXCLUDED.attributes,
				"view_type" = EXCLUDED.view_type`

		_, err := tx.Exec(ctx, bulkWriteTabQuery, bulkWriteTabValues...)
		if err != nil {
			return nil, errors.Wrap(err, "error executing bulkWriteTab query")
		}
	}

	if len(bulkWriteSectionValues) > 0 {
		bulkWriteSectionQuery += ` ON CONFLICT (id) DO UPDATE
			SET
				"tab_id" = EXCLUDED.tab_id,
				"label" = EXCLUDED.label,
				"order" = EXCLUDED.order,
				"icon" = EXCLUDED.icon,
				"column" = EXCLUDED.column,
				"is_summary_section" = EXCLUDED.is_summary_section,
				"fields" = EXCLUDED.fields,
				"table_id" = EXCLUDED.table_id,
				"attributes" = EXCLUDED.attributes`

		_, err = tx.Exec(ctx, bulkWriteSectionQuery, bulkWriteSectionValues...)
		if err != nil {
			return nil, errors.Wrap(err, "error executing bulkWriteSection query")
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, "error committing transaction")
	}

	if req.WithoutResponse {
		return &nb.LayoutResponse{Id: layoutId}, nil
	}

	return l.GetByID(ctx, &nb.LayoutPrimaryKey{Id: layoutId, ProjectId: req.ProjectId, RoleId: roleGUID})
}

func (l *layoutRepo) GetAll(ctx context.Context, req *nb.GetListLayoutRequest) (resp *nb.GetListLayoutResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.GetAll")
	defer dbSpan.Finish()

	resp = &nb.GetListLayoutResponse{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.TableId == "" {
		table, err := helper.TableVer(ctx, models.TableVerReq{Conn: conn, Slug: req.TableSlug, Id: req.TableId})
		if err != nil {
			return nil, err
		}
		req.TableId = cast.ToString(table["id"])
		req.TableSlug = cast.ToString(table["slug"])
	}

	payload := make(map[string]any)
	payload["table_id"] = req.TableId
	if req.IsDefault {
		payload["is_default"] = true
	}
	if req.MenuId != "" {
		payload["menu_id"] = req.MenuId
	}
	var menu_id sql.NullString
	query := `
		SELECT
			id,
			label,
			"order",
			"type",
			icon,
			is_default,
			is_modal,
			is_visible_section,
			attributes,
			table_id,
			menu_id
		FROM layout
		WHERE table_id = $1`

	var args []any
	args = append(args, payload["table_id"])

	if menuID, ok := payload["menu_id"]; ok {
		query += ` AND menu_id = $2`
		args = append(args, menuID)
	}

	query += ` ORDER BY created_at DESC`

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	layouts := make([]*nb.LayoutResponse, 0)

	for rows.Next() {
		layout := nb.LayoutResponse{}
		err := rows.Scan(&layout.Id, &layout.Label, &layout.Order, &layout.Type,
			&layout.Icon, &layout.IsDefault, &layout.IsModal,
			&layout.IsVisibleSection, &layout.Attributes, &layout.TableId, &menu_id)
		if err != nil {
			return nil, err
		}
		if menu_id.Valid {
			layout.MenuId = menu_id.String
		}
		layouts = append(layouts, &layout)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	var layoutIDs []string
	for _, layout := range layouts {
		summaryFields := make([]*nb.FieldResponse, 0)
		if layout.SummaryFields == nil {
			layout.SummaryFields = []*nb.FieldResponse{}
		}
		if len(layout.SummaryFields) > 0 {
			for _, fieldReq := range layout.SummaryFields {
				field := &nb.FieldResponse{}

				if strings.Contains(fieldReq.Id, "#") {
					field.Id = fieldReq.Id
					field.Label = fieldReq.Label
					field.Order = fieldReq.Order
					field.RelationType = fieldReq.RelationType
					relationID := strings.Split(fieldReq.Id, "#")[1]
					var fieldResp nb.Field
					err := conn.QueryRow(ctx, "SELECT slug, required FROM field WHERE relation_id = $1 AND table_id = $2", relationID, req.TableId).Scan(&fieldResp.Slug, &fieldResp.Required)
					if err != nil {
						if err != pgx.ErrNoRows {
							return nil, err
						}
					}

					var relation nb.RelationForGetAll
					err = conn.QueryRow(ctx, `SELECT
				"id",
				"table_from",
				"table_to",
				"field_from",
				"field_to",
				"type",
				"view_fields",
				"relation_field_slug",
				"dynamic_tables",
				"editable",
				"is_user_id_default",
				"cascadings",
				"is_system",
				"object_id_from_jwt",
				"cascading_tree_table_slug",
				"cascading_tree_field_slug",
				"dynamic_tables" FROM relation WHERE id = $1`, relationID).Scan(&relationID,
						&relation.TableFrom,
						&relation.TableTo,
						&relation.FieldFrom,
						&relation.FieldTo,
						&relation.Type,
						&relation.ViewFields,
						&relation.RelationFieldSlug,
						&relation.DynamicTables,
						&relation.IsEditable,
						&relation.IsUserIdDefault,
						&relation.Cascadings,
						&relation.IsSystem,
						&relation.ObjectIdFromJwt,
						&relation.CascadingTreeTableSlug,
						&relation.CascadingTreeFieldSlug,
						&relation.DynamicTables)

					if err != nil {
						if err == pgx.ErrNoRows {
							continue
						}
						return nil, err
					}
					viewOfRelation := nb.View{}
					err = conn.QueryRow(ctx, "SELECT view_fields, function_path, attributes, is_editable FROM view WHERE relation_id = $1 AND relation_table_slug = $2", relation.Id, req.TableSlug).Scan(viewOfRelation.ViewFields, &viewOfRelation.FunctionPath, &viewOfRelation.Attributes, &viewOfRelation.IsEditable)
					if err != nil {
						if err != pgx.ErrNoRows {
							return nil, err
						}
					}
					var viewFieldIds []*nb.Field
					if len(viewOfRelation.ViewFields) > 0 {
						viewFieldIds = make([]*nb.Field, len(viewOfRelation.ViewFields))
						for i, id := range viewOfRelation.ViewFields {
							viewFieldIds[i] = &nb.Field{Id: id}
						}
					}

					var fieldAsAttribute []string

					if relation.Id != "" {
						for _, fieldID := range viewFieldIds {
							var field nb.FieldResponse
							err := conn.QueryRow(ctx, "SELECT slug, enable_multilanguage FROM field WHERE id = $1", fieldID).Scan(&field.Slug, &field.EnableMultilanguage)
							if err != nil {
								if err == pgx.ErrNoRows {
									continue
								}
								return nil, err
							}

							if req.LanguageSetting != "" && field.EnableMultilanguage {
								if strings.HasSuffix(field.Slug, "_"+req.LanguageSetting) {
									fieldAsAttribute = append(fieldAsAttribute, field.Slug)
								} else {
									continue
								}
							} else {
								fieldAsAttribute = append(fieldAsAttribute, field.Slug)
							}
						}

						tableFields := []nb.Field{}
						rows, err = conn.Query(ctx, "SELECT id, auto_fill_table, auto_fill_field FROM field WHERE table_id = $1", &tableFields, req.TableId)
						if err != nil {
							return nil, err
						}
						defer rows.Close()

						autofillFields := []map[string]any{}
						for i := range tableFields {
							field := &tableFields[i]
							autoFillTable := field.AutofillTable
							splitedAutoFillTable := []string{}
							if strings.Contains(field.AutofillTable, "#") {
								splitedAutoFillTable = strings.Split(field.AutofillTable, "#")
								autoFillTable = splitedAutoFillTable[0]
							}
							if field.AutofillField != "" && autoFillTable != "" && autoFillTable == strings.Split(fieldReq.Id, "#")[0] {
								autofill := map[string]any{
									"field_from": field.AutofillField,
									"field_to":   field.Slug,
									"automatic":  field.Automatic,
								}
								if fieldResp.Slug == splitedAutoFillTable[1] {
									autofillFields = append(autofillFields, autofill)
								}
							}
						}

						originalAttributes := make(map[string]any)
						dynamicTables := []string{}
						if relation.Type == "Many2Dynamic" {
							for _, dynamicTable := range relation.DynamicTables {
								dynamicTableInfo, err := helper.TableVer(ctx, models.TableVerReq{Slug: dynamicTable.TableSlug})
								if err != nil {
									return nil, err
								}
								viewFieldsOfDynamicRelation := dynamicTable.ViewFields
								var viewOfDynamicRelation nb.View
								err = conn.QueryRow(ctx, "SELECT id, relation_id, relation_table_slug FROM view WHERE relation_id = $1 AND relation_table_slug = $2", relation.Id, dynamicTable.TableSlug).Scan(&viewOfDynamicRelation.Id, &viewOfDynamicRelation.RelationId, &viewOfDynamicRelation.RelationTableSlug)
								if err != nil {
									if err != pgx.ErrNoRows {
										return nil, err
									}
								}
								if err != nil {
									return nil, err
								}
								if len(viewOfDynamicRelation.ViewFields) > 0 {
									viewFieldsOfDynamicRelation = viewOfDynamicRelation.ViewFields
								}

								dynamicTableToAttribute := make(map[string]any)
								viewFieldsInDynamicTable := []string{}
								for _, fieldID := range viewFieldsOfDynamicRelation {
									field := &nb.Field{}
									err := conn.QueryRow(ctx, "SELECT slug, enable_multilanguage FROM field WHERE id = $1", fieldID).Scan(&field.Slug, &field.EnableMultilanguage)
									if err != nil {
										if err == pgx.ErrNoRows {
											continue
										}
										return nil, err
									}
									fieldAsAttribute := []string{}
									if req.LanguageSetting != "" && field.EnableMultilanguage {
										if strings.HasSuffix(field.Slug, "_"+req.LanguageSetting) {
											fieldAsAttribute = append(fieldAsAttribute, field.Slug)
										} else {
											continue
										}
									} else {
										fieldAsAttribute = append(fieldAsAttribute, field.Slug)
									}
									viewFieldsInDynamicTable = append(viewFieldsInDynamicTable, fieldAsAttribute...)
								}
								dynamicTableToAttribute["table"] = dynamicTableInfo
								dynamicTableToAttribute["viewFields"] = viewFieldsInDynamicTable

								if field != nil {
									if field.Attributes != nil {
										attributesBytes, err := field.Attributes.MarshalJSON()
										if err != nil {
											return nil, err
										}
										err = json.Unmarshal(attributesBytes, &field)
										if err != nil {
											return nil, err
										}
									}

									if req.LanguageSetting != "" && field.EnableMultilanguage {
										if strings.HasSuffix(field.Slug, "_"+req.LanguageSetting) {
											viewFieldsInDynamicTable = append(viewFieldsInDynamicTable, field.Slug)
										} else {
											continue
										}
									} else {
										viewFieldsInDynamicTable = append(viewFieldsInDynamicTable, field.Slug)
									}
								}
								dynamicTableToAttribute["view_fields"] = viewFieldsInDynamicTable
								dynamicTables = append(dynamicTables, fmt.Sprintf("%v", dynamicTableToAttribute))
							}

							originalAttributes = make(map[string]any)

							originalAttributes["autofill"] = autofillFields
							originalAttributes["view_fields"] = fieldAsAttribute
							originalAttributes["auto_filters"] = relation.AutoFilters
							originalAttributes["relation_field_slug"] = relation.RelationFieldSlug
							originalAttributes["dynamic_tables"] = dynamicTables
							originalAttributes["is_user_id_default"] = relation.IsUserIdDefault
							originalAttributes["object_id_from_jwt"] = relation.ObjectIdFromJwt
							originalAttributes["cascadings"] = relation.Cascadings
							originalAttributes["cascading_tree_table_slug"] = relation.CascadingTreeTableSlug
							originalAttributes["cascading_tree_field_slug"] = relation.CascadingTreeFieldSlug
							originalAttributes["function_path"] = viewOfRelation.FunctionPath
						} else {
							originalAttributes["autofill"] = autofillFields
							originalAttributes["view_fields"] = fieldAsAttribute
							originalAttributes["auto_filters"] = relation.AutoFilters
							originalAttributes["relation_field_slug"] = relation.RelationFieldSlug
							originalAttributes["dynamic_tables"] = dynamicTables
							originalAttributes["is_user_id_default"] = relation.IsUserIdDefault
							originalAttributes["object_id_from_jwt"] = relation.ObjectIdFromJwt
							originalAttributes["cascadings"] = relation.Cascadings
							originalAttributes["cascading_tree_table_slug"] = relation.CascadingTreeTableSlug
							originalAttributes["cascading_tree_field_slug"] = relation.CascadingTreeFieldSlug
							originalAttributes["function_path"] = viewOfRelation.FunctionPath

							for k, v := range viewOfRelation.Attributes.AsMap() {
								originalAttributes[k] = v
							}

						}
						if len(viewOfRelation.DefaultValues) > 0 {
							originalAttributes["default_values"] = viewOfRelation.DefaultValues
						}
						originalAttributes["creatable"] = viewOfRelation.Creatable

						originalAttributesJSON, err := json.Marshal(originalAttributes)
						if err != nil {
							return nil, err
						}
						var encodedAttributes []byte
						err = json.Unmarshal(originalAttributesJSON, &encodedAttributes)
						if err != nil {
							return nil, err
						}
						var attributes structpb.Struct
						err = protojson.Unmarshal(encodedAttributes, &attributes)
						if err != nil {
							return nil, err
						}
						field.Attributes = &attributes
						summaryFields = append(summaryFields, field)
						if strings.Contains(fieldReq.Id, "@") {
							field.Id = fieldReq.Id
						} else {
							guid := fieldReq.Id
							fieldQuery := `
							SELECT
								id,
								table_id,
								required,
								"slug",
								"label",
								"default",
								"type",
								"index"

							FROM "field"
							WHERE id = $1
						`
							err := conn.QueryRow(ctx, fieldQuery, guid).Scan(
								&field.Id,
								&field.TableId,
								&field.Required,
								&field.Slug,
								&field.Label,
								&field.Default,
								&field.Type,
								&field.Index,
							)
							if err != nil {
								return nil, err
							}
							if field != nil {
								field.Order = fieldReq.Order
								field.Column = fieldReq.Column
								field.Id = fieldReq.Id
								field.RelationType = fieldReq.RelationType
								summaryFields = append(summaryFields, field)
							}
							summaryFields = append(summaryFields, field)
						}
					}
				}
			}
		}
		fieldsWithPermissions, err := helper.AddPermissionToField(ctx, conn, summaryFields, req.RoleId, req.TableSlug, req.ProjectId)
		if err != nil {
			return nil, err
		}
		layout.SummaryFields = fieldsWithPermissions
		layoutIDs = append(layoutIDs, layout.Id)

		var (
			label      sql.NullString
			order      sql.NullInt32
			icon       sql.NullString
			relationId sql.NullString
		)
		tabs := make([]*nb.TabResponse, 0)
		sqlQuery := `
		SELECT
			"id",
			"type",
			"order",
			"label",
			"icon",
			"layout_id",
			"relation_id",
			"attributes"
		FROM "tab"
		WHERE "layout_id"::varchar = ANY($1)
		ORDER BY t."order"
		`

		rows, err := conn.Query(ctx, sqlQuery, pq.Array(layoutIDs))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var tab nb.TabResponse
			err := rows.Scan(&tab.Id, &tab.Type, &order, &label, &icon, &tab.LayoutId, &relationId, &tab.Attributes)
			if err != nil {
				return nil, err
			}
			if label.Valid {
				tab.Label = label.String
			}
			if order.Valid {
				tab.Order = order.Int32
			}

			if icon.Valid {
				tab.Icon = icon.String
			}

			if relationId.Valid {
				tab.RelationId = relationId.String
			}

			tabs = append(tabs, &tab)
		}
		if rows.Err() != nil {
			return nil, rows.Err()
		}
		layout.Tabs = tabs

		sectionRepo := NewSectionRepo(conn)
		relationRepo := NewRelationRepo(conn)

		for _, tab := range tabs {
			if tab.Type == "section" {
				sections, err := sectionRepo.GetAll(ctx, &nb.GetAllSectionsRequest{
					ProjectId: req.ProjectId,
					RoleId:    req.RoleId,
					TableSlug: req.TableSlug,
					TableId:   req.TableId,
					TabId:     tab.Id,
				})
				if err != nil {
					return nil, err
				}
				tab.Sections = sections.Sections
			} else if tab.Type == "relation" && tab.RelationId != "" {

				relation, err := relationRepo.GetSingleViewForRelation(ctx, models.ReqForViewRelation{
					Id:        tab.RelationId,
					ProjectId: req.ProjectId,
					RoleId:    req.RoleId,
					TableSlug: req.TableSlug,
				})
				if err != nil {
					return nil, err
				}

				var newRelation nb.RelationForSection
				newRelation.Id = relation.Id
				newRelation.Type = relation.Type

				tab.Relation = &newRelation
			}
		}
		layout.Tabs = tabs
	}

	resp.Layouts = layouts

	return resp, nil
}

func (l *layoutRepo) RemoveLayout(ctx context.Context, req *nb.LayoutPrimaryKey) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.RemoveLayout")
	defer dbSpan.Finish()

	var (
		tabIDs []string
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "error starting transaction")
	}
	defer tx.Rollback(ctx)

	rows, err := tx.Query(ctx, "SELECT id FROM tab WHERE layout_id = $1", req.Id)
	if err != nil {
		return errors.Wrap(err, "error querying tabs")
	}
	defer rows.Close()

	for rows.Next() {
		var tabID string
		if err := rows.Scan(&tabID); err != nil {
			return errors.Wrap(err, "error scanning tab id")
		}
		tabIDs = append(tabIDs, tabID)
	}
	if err := rows.Err(); err != nil {
		return errors.Wrap(err, "error iterating over tab rows")
	}

	if _, err := tx.Exec(ctx, "DELETE FROM section WHERE tab_id = ANY($1)", pq.Array(tabIDs)); err != nil {
		return errors.Wrap(err, "error deleting sections")
	}

	if _, err := tx.Exec(ctx, "DELETE FROM tab WHERE id = ANY($1)", pq.Array(tabIDs)); err != nil {
		return errors.Wrap(err, "error deleting tabs")
	}

	if _, err := tx.Exec(ctx, "DELETE FROM layout WHERE id = $1", req.Id); err != nil {
		return errors.Wrap(err, "error deleting layout")
	}

	if err := tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "error committing transaction")
	}

	return nil
}

func (l *layoutRepo) getLayoutData(ctx context.Context, conn *psqlpool.Pool, query string, args ...interface{}) (*nb.LayoutResponse, error) {
	var (
		layout = &nb.LayoutResponse{}
		body   = []byte{}
	)

	if err := conn.QueryRow(ctx, query, args...).Scan(&body); err != nil {
		return nil, errors.Wrap(err, "error querying layout")
	}

	if err := json.Unmarshal(body, layout); err != nil {
		return nil, errors.Wrap(err, "error unmarshalling layout")
	}

	return layout, nil
}

func (l *layoutRepo) enrichLayoutWithTabsAndFields(ctx context.Context, conn *psqlpool.Pool, layout *nb.LayoutResponse, roleId, tableSlug string) error {
	fields, fieldsAutofillMap, err := l.getFieldsWithPermissions(ctx, conn, tableSlug, roleId)
	if err != nil {
		return err
	}

	for _, tab := range layout.Tabs {
		switch tab.Type {
		case "section":
			section, err := GetSections(ctx, conn, tab.Id, roleId, tableSlug, fields, fieldsAutofillMap)
			if err != nil {
				return err
			}
			tab.Sections = section
		case "relation":
			relation, err := GetRelation(ctx, conn, tab.RelationId)
			if err != nil {
				return err
			}
			if relation != nil {
				relation.Attributes = tab.Attributes
				relation.RelationTableSlug = relation.TableFrom.Slug
				tab.Relation = relation
			}
		}
	}

	return nil
}

func (l *layoutRepo) getFieldsWithPermissions(ctx context.Context, conn *psqlpool.Pool, tableSlug, roleId string) (map[string]*nb.FieldResponse, map[string]models.AutofillField, error) {
	var (
		fields            = make(map[string]*nb.FieldResponse)
		fieldsAutofillMap = make(map[string]models.AutofillField)
	)

	fieldQuery := `SELECT 
		f.id,
		f.type,
		f.index,
		f.label,
		f.slug,
		f.table_id,
		f.attributes,
		f.autofill_field,
		f.autofill_table,
		f.automatic,
		f.required,
		fp.guid,
		fp.field_id,
		fp.role_id,
		fp.table_slug,
		fp.label,
		fp.view_permission,
		fp.edit_permission
	FROM "field" f 
	JOIN "table" t ON t.id = f.table_id 
	LEFT JOIN "field_permission" fp ON fp.field_id = f.id
	WHERE t.slug = $1 AND fp.role_id = $2`

	rows, err := conn.Query(ctx, fieldQuery, tableSlug, roleId)
	if err != nil {
		return nil, nil, errors.Wrap(err, "error querying fields")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			field                 = nb.FieldResponse{}
			att                   = []byte{}
			indexNull             sql.NullString
			fPermission           = models.FieldPermission{}
			autofillField         sql.NullString
			autofillTable, fpGuid sql.NullString
			autofillAutomatic     sql.NullBool
			attributes            = make(map[string]any)
		)

		err = rows.Scan(
			&field.Id,
			&field.Type,
			&indexNull,
			&field.Label,
			&field.Slug,
			&field.TableId,
			&att,
			&autofillField,
			&autofillTable,
			&autofillAutomatic,
			&field.Required,
			&fpGuid,
			&fPermission.FieldId,
			&fPermission.RoleId,
			&fPermission.TableSlug,
			&fPermission.Label,
			&fPermission.ViewPermission,
			&fPermission.EditPermission,
		)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error scanning field")
		}

		if err := json.Unmarshal(att, &attributes); err != nil {
			return nil, nil, errors.Wrap(err, "error unmarshalling attributes")
		}

		fPermission.Guid = fpGuid.String

		attributes["field_permission"] = fPermission
		attributes["autofill_field"] = autofillField.String
		attributes["autofill_table"] = autofillTable.String
		attributes["automatic"] = autofillAutomatic.Bool

		atr, err := helper.ConvertMapToStruct(attributes)
		if err != nil {
			return nil, nil, errors.Wrap(err, "error converting attributes")
		}

		field.Attributes = atr
		field.Index = indexNull.String

		if autofillField.String != "" && autofillTable.String != "" {
			splitAutofillTable := strings.Split(autofillTable.String, "#")
			if len(splitAutofillTable) >= 2 {
				relationFieldSlug := splitAutofillTable[1]
				fieldsAutofillMap[relationFieldSlug] = models.AutofillField{
					FieldFrom: autofillField.String,
					FieldTo:   field.Slug,
					TableSlug: tableSlug,
					Automatic: autofillAutomatic.Bool,
				}
			}
		}
		fields[field.Id] = &field
	}

	return fields, fieldsAutofillMap, nil
}

func (l *layoutRepo) GetByID(ctx context.Context, req *nb.LayoutPrimaryKey) (*nb.LayoutResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.GetByID")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, errors.Wrap(err, "error getting connection from pool")
	}

	query := `SELECT jsonb_build_object (
		'id', l.id,
		'label', l.label,
		'order', l."order",
		'table_id', l.table_id,
		'type', l."type",
		'is_default', l.is_default,
		'is_modal', l.is_modal,
		'is_visible_section', l.is_visible_section,
		'tabs', (
			SELECT jsonb_agg(
				jsonb_build_object(
					'id', t.id,
					'label', t.label,
					'layout_id', t.layout_id,
					'type', t.type,
					'order', t."order",
					'relation_id', t.relation_id::varchar,
					'attributes', t.attributes,
					'view_type', t.view_type
				)
				ORDER BY t."order" ASC
			)
			FROM tab t 
			WHERE t.layout_id = l.id
		)
	) AS data 
	FROM layout l 
	WHERE l.id = $1`

	layout, err := l.getLayoutData(ctx, conn, query, req.Id)
	if err != nil {
		return nil, err
	}

	// Get table slug for the layout
	var tableSlug string
	err = conn.QueryRow(ctx, `SELECT slug FROM "table" WHERE id = $1`, layout.TableId).Scan(&tableSlug)
	if err != nil {
		return nil, errors.Wrap(err, "error getting table slug")
	}

	err = l.enrichLayoutWithTabsAndFields(ctx, conn, layout, req.RoleId, tableSlug)
	if err != nil {
		return nil, err
	}

	return layout, nil
}

func (l *layoutRepo) GetSingleLayout(ctx context.Context, req *nb.GetSingleLayoutRequest) (*nb.LayoutResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.GetSingleLayout")
	defer dbSpan.Finish()

	if req.MenuId == "" {
		return nil, errors.New("menu_id is required")
	}

	if req.TableId == "" && req.TableSlug == "" {
		return nil, errors.New("either table_id or table_slug is required")
	}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, errors.Wrap(err, "error getting connection from pool")
	}

	if req.TableId == "" {
		err := conn.QueryRow(ctx, `SELECT id FROM "table" WHERE slug = $1`, req.TableSlug).Scan(&req.TableId)
		if err != nil {
			return nil, errors.Wrap(err, "error getting table_id")
		}
	}

	// Check if layout exists for the menu
	var count int
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM "layout" WHERE table_id = $1 AND menu_id = $2`,
		req.TableId, req.MenuId).Scan(&count)
	if err != nil {
		return nil, errors.Wrap(err, "error checking layout existence")
	}

	baseQuery := `SELECT jsonb_build_object (
		'id', l.id,
		'label', l.label,
		'order', l."order",
		'table_id', l.table_id,
		'type', l."type",
		'is_default', l.is_default,
		'is_modal', l.is_modal,
		'is_visible_section', l.is_visible_section,
		'tabs', (
			SELECT jsonb_agg(
				jsonb_build_object(
					'id', t.id,
					'label', t.label,
					'layout_id', t.layout_id,
					'type', t.type,
					'order', t."order",
					'relation_id', t.relation_id::varchar,
					'attributes', t.attributes,
					'view_type', t.view_type
				)
				ORDER BY t."order" ASC
			)
			FROM tab t 
			WHERE t.layout_id = l.id
		)
	) AS data 
	FROM layout l`

	var layout *nb.LayoutResponse
	if count == 0 {
		// Get default layout
		query := baseQuery + ` WHERE l.table_id = $1 AND l.is_default = true`
		layout, err = l.getLayoutData(ctx, conn, query, req.TableId)
	} else {
		// Get layout for specific menu
		query := baseQuery + ` WHERE l.table_id = $1 AND l.menu_id = $2`
		layout, err = l.getLayoutData(ctx, conn, query, req.TableId, req.MenuId)
	}

	if err != nil {
		return nil, err
	}

	err = l.enrichLayoutWithTabsAndFields(ctx, conn, layout, req.RoleId, req.TableSlug)
	if err != nil {
		return nil, err
	}

	return layout, nil
}

func (l *layoutRepo) GetAllV2(ctx context.Context, req *nb.GetListLayoutRequest) (resp *nb.GetListLayoutResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.GetAllV2")
	defer dbSpan.Finish()

	resp = &nb.GetListLayoutResponse{}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT jsonb_build_object (
		'id', l.id,
		'label', l.label,
		'order', l."order",
		'table_id', l.table_id,
		'type', l."type",
		'is_default', l.is_default,
		'is_modal', l.is_modal,
		'is_visible_section', l.is_visible_section,
		'tabs', (
			SELECT jsonb_agg(
					jsonb_build_object(
						'id', t.id,
						'label', t.label,
						'layout_id', t.layout_id,
						'type', t.type,
						'order', t."order",
						'relation_id', t.relation_id::varchar,
						'attributes', t.attributes
					)
					ORDER BY t."order"
				)
			FROM tab t 
			WHERE t.layout_id = l.id
		)
	) AS DATA 
	FROM layout l 
	JOIN "table" ta ON ta.id = l.table_id
	WHERE ta.slug = $1
	GROUP BY l.id
	ORDER BY l."order" ASC;
	`

	layoutRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error querying layout")
	}
	defer layoutRows.Close()

	for layoutRows.Next() {
		var (
			layout = nb.LayoutResponse{}
			body   = []byte{}
		)

		err = layoutRows.Scan(
			&body,
		)
		if err != nil {
			return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error scanning layout")
		}

		if err := json.Unmarshal(body, &layout); err != nil {
			return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error unmarshalling layout")
		}

		resp.Layouts = append(resp.Layouts, &layout)
	}

	fieldQuery := `SELECT 
		f.id,
		f.type,
		f.index,
		f.label,
		f.slug,
		f.table_id,
		f.attributes
	FROM "field" f 
	JOIN "table" t ON t.id = f.table_id 
	WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, fieldQuery, req.TableSlug)
	if err != nil {
		return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error querying field")
	}
	defer fieldRows.Close()

	fields := make(map[string]*nb.FieldResponse)

	for fieldRows.Next() {
		var (
			field     = nb.FieldResponse{}
			att       = []byte{}
			indexNull sql.NullString
		)

		err = fieldRows.Scan(
			&field.Id,
			&field.Type,
			&indexNull,
			&field.Label,
			&field.Slug,
			&field.TableId,
			&att,
		)
		if err != nil {
			return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error scanning field")
		}

		field.Index = indexNull.String

		fields[field.Id] = &field
	}

	for _, layout := range resp.Layouts {
		for _, tab := range layout.Tabs {
			if tab.Type == "section" {
				section, err := GetSections(ctx, conn, tab.Id, "", "", fields, map[string]models.AutofillField{})
				if err != nil {
					return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error getting sections")
				}
				tab.Sections = section
			} else if tab.Type == "relation" {
				relation, err := GetRelation(ctx, conn, tab.RelationId)
				if err != nil {
					return &nb.GetListLayoutResponse{}, errors.Wrap(err, "error getting relation")
				}

				if relation == nil {
					continue
				}
				relation.Attributes = tab.Attributes
				relation.RelationTableSlug = relation.TableFrom.Slug
				tab.Relation = relation
			}
		}
	}

	return resp, nil
}

func GetSections(ctx context.Context, conn *psqlpool.Pool, tabId, roleId, tableSlug string, fields map[string]*nb.FieldResponse, fieldsAutofillMap map[string]models.AutofillField) ([]*nb.SectionResponse, error) {
	var (
		sections          = []*nb.SectionResponse{}
		relationFiledsMap = make(map[string]models.SectionRelation)
		relationsIds      = []string{}

		sectionQuery = `SELECT 
				id,
				"order",
				fields,
				attributes
			FROM "section" WHERE tab_id = $1 ORDER BY "order" ASC`

		relationFieldQuery = `SELECT
				r."id",
				COALESCE(r."auto_filters", '[{}]') AS "auto_filters",
				r."view_fields",
				r."type"
			FROM "relation" r
			WHERE r."table_from" = $1`

		viewQuery = `SELECT creatable, relation_id FROM "view" WHERE relation_id = ANY($1)`
	)

	relationFieldsRows, err := conn.Query(ctx, relationFieldQuery, tableSlug)
	if err != nil {
		return nil, errors.Wrap(err, "when querying relation")
	}

	defer relationFieldsRows.Close()

	for relationFieldsRows.Next() {
		var (
			id             sql.NullString
			autoFilterByte []byte
			autoFilters    []map[string]any
			viewFields     []string
			relationType   string
		)

		if err = relationFieldsRows.Scan(&id, &autoFilterByte, &viewFields, &relationType); err != nil {
			return nil, errors.Wrap(err, "when scaning")
		}

		if err = json.Unmarshal(autoFilterByte, &autoFilters); err != nil {
			return nil, errors.Wrap(err, "error unmarshal")
		}

		relationFiledsMap[id.String] = models.SectionRelation{
			Id:          id.String,
			ViewFields:  viewFields,
			Autofilters: autoFilters,
			Type:        relationType,
		}
		relationsIds = append(relationsIds, id.String)
	}

	viewRows, err := conn.Query(ctx, viewQuery, relationsIds)
	if err != nil {
		return nil, errors.Wrap(err, "when querying view")
	}

	defer viewRows.Close()

	for viewRows.Next() {
		var creatable sql.NullBool
		var relationId sql.NullString
		if err = viewRows.Scan(&creatable, &relationId); err != nil {
			return nil, errors.Wrap(err, "when scaning")
		}

		relationField := relationFiledsMap[relationId.String]
		relationField.Creatable = creatable.Bool
		relationFiledsMap[relationId.String] = relationField
	}

	sectionRows, err := conn.Query(ctx, sectionQuery, tabId)
	if err != nil {
		return nil, errors.Wrap(err, "error querying section")
	}
	defer sectionRows.Close()

	for sectionRows.Next() {
		var (
			section    = nb.SectionResponse{}
			fieldBody  = []models.SectionFields{}
			body       = []byte{}
			attributes = []byte{}
		)

		err = sectionRows.Scan(
			&section.Id,
			&section.Order,
			&body,
			&attributes,
		)
		if err != nil {
			return nil, errors.Wrap(err, "error scanning section")
		}

		if err := json.Unmarshal(body, &fieldBody); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling section")
		}

		if err := json.Unmarshal(attributes, &section.Attributes); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling section attributes")
		}

		for i, f := range fieldBody {
			if strings.Contains(f.Id, "#") {
				fBody := []nb.FieldResponse{}

				if err := json.Unmarshal(body, &fBody); err != nil {
					return nil, errors.Wrap(err, "error unmarshalling field body")
				}

				temp, err := helper.ConvertStructToMap(fBody[i].Attributes)
				if err != nil {
					return nil, errors.Wrap(err, "error converting struct to map")
				}

				if temp == nil {
					temp = make(map[string]any)
				}

				fieldsSlice := cast.ToSlice(temp["fields"])
				if fieldsSlice != nil {
					attributes := cast.ToStringMap(cast.ToStringMap(fieldsSlice[0])["attributes"])
					maps.Copy(temp, attributes)
				}
				var isTab = cast.ToBool(temp["isTab"])

				newAttributes, err := helper.ConvertMapToStruct(temp)
				if err != nil {
					return nil, errors.Wrap(err, "error converting map to struct")
				}

				fBody[i].Attributes = newAttributes

				if roleId != "" && !isTab {
					var (
						relationId        = strings.Split(f.Id, "#")[1]
						relationTableSlug = strings.Split(f.Id, "#")[0]
						fieldId           = ""
						required          bool
						fieldAttributes   = []byte{}
						queryF            = `SELECT f.id, f.attributes, f.required FROM "field" f JOIN "table" t ON t.id = f.table_id WHERE f.relation_id = $1 AND t.slug = $2`
					)

					if err = conn.QueryRow(ctx, queryF, relationId, tableSlug).Scan(&fieldId, &fieldAttributes, &required); err != nil {
						return nil, errors.Wrap(err, "error querying field")
					}
					if err := json.Unmarshal(fieldAttributes, &fBody[i].Attributes); err != nil {
						return nil, errors.Wrap(err, "error unmarshalling section attributes")
					}

					var (
						field          = fields[fieldId]
						autoFilters    = []map[string]any{}
						viewFieldsBody = []map[string]any{}
						viewFields     = []string{}
						creatable      bool
						relationType   string
					)

					if value, ok := relationFiledsMap[relationId]; ok {
						autoFilters = value.Autofilters
						viewFields = value.ViewFields
						creatable = value.Creatable
						relationType = value.Type
					}

					for _, id := range viewFields {
						var (
							slug   = ""
							queryF = `SELECT slug FROM field WHERE id = $1`
						)

						if err = conn.QueryRow(ctx, queryF, id).Scan(&slug); err != nil {
							return nil, errors.Wrap(err, "error get field")
						}

						viewFieldsBody = append(viewFieldsBody, map[string]any{
							"slug": slug,
						})
					}

					attributes, err := helper.ConvertStructToMap(fBody[i].Attributes)
					if err != nil {
						return nil, errors.Wrap(err, "error converting struct to map")
					}

					permission := models.FieldPermission{}

					query := `SELECT
						guid,
						field_id,
						role_id,
						table_slug,
						label,
						view_permission,
						edit_permission
					FROM field_permission WHERE field_id = $1 AND role_id = $2`

					err = conn.QueryRow(ctx, query, fieldId, roleId).Scan(
						&permission.Guid,
						&permission.FieldId,
						&permission.RoleId,
						&permission.TableSlug,
						&permission.Label,
						&permission.ViewPermission,
						&permission.EditPermission,
					)
					if err != nil {
						return nil, errors.Wrap(err, "error querying field permission")
					}

					attributes["field_hide_layout"] = cast.ToBool(f.Attributes["field_hide_layout"])
					attributes["creatable"] = creatable
					attributes["field_permission"] = permission
					attributes["auto_filters"] = autoFilters
					attributes["view_fields"] = viewFieldsBody
					if v, ok := fieldsAutofillMap[relationTableSlug+"_id"]; ok {
						attributes["autofill"] = []any{v}
					}

					bodyAtt, err := helper.ConvertMapToStruct(attributes)
					if err != nil {
						return nil, errors.Wrap(err, "error converting map to struct")
					}

					fBody[i].Slug = field.Slug
					fBody[i].Attributes = bodyAtt
					fBody[i].Type = field.Type
					fBody[i].Label = field.Label
					fBody[i].RelationType = relationType
					fBody[i].Required = required
				}

				section.Fields = append(section.Fields, &fBody[i])
			} else {
				fBody := fields[f.Id]

				if fBody != nil {
					attr := fBody.Attributes.AsMap()
					attr["field_hide_layout"] = cast.ToBool(f.Attributes["field_hide_layout"])

					bodyAtt, err := helper.ConvertMapToStruct(attr)
					if err != nil {
						return nil, errors.Wrap(err, "error converting map to struct")
					}

					fBody.Attributes = bodyAtt
					fBody.Order = int32(f.Order)
				}

				section.Fields = append(section.Fields, fBody)
			}

		}

		sections = append(sections, &section)
	}

	return sections, nil
}

func GetRelation(ctx context.Context, conn *psqlpool.Pool, relationId string) (*nb.RelationForSection, error) {
	query := `SELECT
		r.id,
		r.type,
		r.view_fields,

		COALESCE(t1.id::varchar, ''),
		COALESCE(t1.label, ''),
		COALESCE(t1.slug, ''),
		COALESCE(t1.show_in_menu, false),
		COALESCE(t1.icon, ''),

		COALESCE(t2.id::varchar, ''),
		COALESCE(t2.label, ''),
		COALESCE(t2.slug, ''),
		COALESCE(t2.show_in_menu, false),
		COALESCE(t2.icon, '')
	FROM "relation" r 
	LEFT JOIN "table" t1 ON t1.slug = table_from
	LEFT JOIN "table" t2 ON t2.slug = table_to
	WHERE r.id = $1`

	relation := nb.RelationForSection{
		TableFrom: &nb.TableForSection{},
		TableTo:   &nb.TableForSection{},
	}
	viewFields := []string{}

	err := conn.QueryRow(ctx, query, relationId).Scan(
		&relation.Id,
		&relation.Type,
		&viewFields,

		&relation.TableFrom.Id,
		&relation.TableFrom.Label,
		&relation.TableFrom.Slug,
		&relation.TableFrom.ShowInMenu,
		&relation.TableFrom.Icon,
		&relation.TableTo.Id,
		&relation.TableTo.Label,
		&relation.TableTo.Slug,
		&relation.TableTo.ShowInMenu,
		&relation.TableTo.Icon,
	)
	if err != nil {
		return &nb.RelationForSection{}, errors.Wrap(err, "relation query scan")
	}

	query = `SELECT
		id,
		type,
		index,
		label,
		slug,
		table_id,
		attributes,
		is_search
	FROM "field" WHERE id IN ($1)`

	fieldRows, err := conn.Query(ctx, query, pq.Array(viewFields))
	if err != nil {
		return &nb.RelationForSection{}, errors.Wrap(err, "field query")
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		att := []byte{}
		field := nb.Field{}

		err = fieldRows.Scan(
			&field.Id,
			&field.Type,
			&field.Index,
			&field.Label,
			&field.TableId,
			&att,
			&field.IsSearch,
		)
		if err != nil {
			return &nb.RelationForSection{}, errors.Wrap(err, "field scan")
		}
		if err := json.Unmarshal(att, &field.Attributes); err != nil {
			return &nb.RelationForSection{}, errors.Wrap(err, "unmarshal attributes")
		}

		relation.ViewFields = append(relation.ViewFields, &field)
	}

	permission := models.RelationFields{}

	query = `SELECT 
		guid,
		role_id,
		relation_id,
		table_slug,
		view_permission,
		create_permission,
		edit_permission,
		delete_permission
	FROM "view_relation_permission" WHERE relation_id = $1`

	err = conn.QueryRow(ctx, query, relationId).Scan(
		&permission.Guid,
		&permission.RoleId,
		&permission.RelationId,
		&permission.TableSlug,
		&permission.ViewPermission,
		&permission.CreatePermission,
		&permission.EditPermission,
		&permission.DeletePermission,
	)

	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return &nb.RelationForSection{}, errors.Wrap(err, "view relation")
	}

	marshledInputMap, err := json.Marshal(permission)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return &nb.RelationForSection{}, errors.Wrap(err, "marshal permission")
	}
	err = protojson.Unmarshal(marshledInputMap, outputStruct)
	if err != nil {
		return &nb.RelationForSection{}, errors.Wrap(err, "unmarshal output")
	}

	relation.Permission = outputStruct

	return &relation, nil
}

// GetLayoutByTableID retrieves all layouts with tabs and sections by table ID using pure SQL
func (l *layoutRepo) GetLayoutByTableID(ctx context.Context, tableID string, projectID string) ([]*nb.LayoutResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "layout.GetLayoutByTableID")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(projectID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting connection from pool")
	}

	query := `SELECT
		l.id,
		l.table_id,
		l."order",
		l.label,
		l.icon,
		l.type,
		l.is_default,
		l.is_visible_section,
		l.is_modal,
		COALESCE(l.menu_id, ''),
		COALESCE(
			jsonb_agg(
				jsonb_build_object(
					'id', t.id,
					'order', t."order",
					'label', t.label,
					'icon', t.icon,
					'type', t.type,
					'layout_id', t.layout_id,
					'relation_id', t.relation_id,
					'table_slug', t.table_slug,
					'attributes', t.attributes,
					'view_type', t.view_type,
					'sections', COALESCE(
						(
							SELECT jsonb_agg(
								jsonb_build_object(
									'id', s.id,
									'order', s."order",
									'column', s."column",
									'label', s.label,
									'icon', s.icon,
									'is_summary_section', s.is_summary_section,
									'fields', s.fields,
									'table_id', s.table_id,
									'tab_id', s.tab_id,
									'attributes', s.attributes
								)
							)
							FROM section s
							WHERE s.tab_id = t.id
						), '[]'::jsonb
					)
				)
			) FILTER (WHERE t.id IS NOT NULL), '[]'::jsonb
		) AS tabs
	FROM layout l
	LEFT JOIN tab t ON t.layout_id = l.id
	WHERE l.table_id = $1
	GROUP BY l.id, l.table_id, l."order", l.label, l.icon, l.type, l.is_default, l.is_visible_section, l.is_modal, l.menu_id, l.attributes`

	rows, err := conn.Query(ctx, query, tableID)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	var layouts []*nb.LayoutResponse
	for rows.Next() {
		var (
			id, tableID, label, icon, layoutType, menuID string
			order                                        int
			isDefault, isVisibleSection, isModal         bool
			tabsJSON                                     []byte
		)

		err := rows.Scan(
			&id,
			&tableID,
			&order,
			&label,
			&icon,
			&layoutType,
			&isDefault,
			&isVisibleSection,
			&isModal,
			&menuID,
			// &attributes,
			&tabsJSON,
		)
		if err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}

		// Parse tabs JSON
		var tabs []*nb.TabResponse
		if len(tabsJSON) > 0 {
			if err := json.Unmarshal(tabsJSON, &tabs); err != nil {
				return nil, errors.Wrap(err, "error unmarshaling tabs")
			}
		}

		layout := &nb.LayoutResponse{
			Id:               id,
			TableId:          tableID,
			Order:            int32(order),
			Label:            label,
			Icon:             icon,
			Type:             layoutType,
			IsDefault:        isDefault,
			IsVisibleSection: isVisibleSection,
			IsModal:          isModal,
			MenuId:           menuID,
			// Attributes:       attributesStruct,
			Tabs: tabs,
		}

		layouts = append(layouts, layout)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	if len(layouts) == 0 {
		return []*nb.LayoutResponse{}, nil
	}

	// Return all layouts found for the table
	return layouts, nil
}

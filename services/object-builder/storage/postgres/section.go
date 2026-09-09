package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"strings"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/jackc/pgx/v5"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

type sectionRepo struct {
	db *psqlpool.Pool
}

func NewSectionRepo(db *psqlpool.Pool) storage.SectionRepoI {
	return &sectionRepo{
		db: db,
	}
}

func (s *sectionRepo) GetViewRelation(ctx context.Context, req *nb.GetAllSectionsRequest) (resp *nb.GetViewRelationResponse, err error) {

	return &nb.GetViewRelationResponse{}, nil
}

func (s *sectionRepo) GetAll(ctx context.Context, req *nb.GetAllSectionsRequest) (resp *nb.GetAllSectionsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "section.GetAll")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.GetAllSectionsResponse{}
	section := nb.SectionResponse{}
	var tableID string
	if req.TableId == "" {
		err := conn.QueryRow(ctx, `SELECT id, "slug" FROM "table" WHERE "slug" = $1`, req.TableSlug).Scan(&tableID, &req.TableSlug)
		if err != nil {
			return nil, err
		}

	}

	rows, err := conn.Query(ctx, `
		SELECT 
			"id",
			"order",
			"column",
			"label",
			"icon",
			"is_summary_section",
			"fields"
			FROM "section"
		WHERE tab_id = $1`, req.TabId)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sections := make([]*nb.SectionResponse, 0)
	for rows.Next() {
		var fields = []byte{}
		var label sql.NullString

		err = rows.Scan(
			&section.Id,
			&section.Order,
			&section.Column,
			&label,
			&section.Icon,
			&section.IsSummarySection,
			&fields,
		)

		var fieldResponses []*nb.FieldResponse
		if err := json.Unmarshal(fields, &fieldResponses); err != nil {
			return nil, err
		}

		if label.Valid {
			section.Label = label.String
		}

		section.Fields = fieldResponses
		sections = append(sections, &section)
	}
	var fieldAsAttribute []*nb.Field
	fieldRes := []*nb.FieldResponse{}

	sectionResponses := make([]*nb.SectionResponse, 0)
	for _, section := range sections {
		for _, fieldReq := range section.Fields {
			if strings.Contains(fieldReq.Id, "#") {
				relationID := strings.Split(fieldReq.Id, "#")[1]
				var fieldResp nb.Field
				err := conn.QueryRow(ctx, "SELECT relation_id, table_id FROM field WHERE relation_id = $1 AND table_id = $2", relationID, req.TableId).Scan(&fieldResp.RelationId, &fieldResp.TableId)
				if err != nil {
					return nil, err
				}

				var relation nb.RelationForGetAll
				err = conn.QueryRow(ctx, `SELECT id, view_fields FROM "relation" WHERE id = $1`, relationID).Scan(&relation.Id)
				if err != nil {
					return nil, err
				}
				var viewOfRelation nb.View
				err = conn.QueryRow(ctx, "SELECT id, view_fields, dynamic_tables, is_editable, function_path,  FROM view WHERE relation_id = $1", relation.Id).Scan(&viewOfRelation.Id, &viewOfRelation.ViewFields)
				if err != nil {
					return nil, err
				}
				var viewFieldIds []string
				for _, field := range relation.ViewFields {
					viewFieldIds = append(viewFieldIds, field.Id)

					if len(viewOfRelation.ViewFields) > 0 {
						viewFieldIds = viewOfRelation.ViewFields
					}
				}

				for _, fieldID := range viewFieldIds {
					var field nb.Field

					err := conn.QueryRow(ctx, `SELECT 
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
						"is_system",
						"is_search",
						"autofill_field",
						"autofill_table",
						"relation_id",
						"unique",
						"automatic",
						"enable_multilanguage" 
					 FROM field WHERE id = $1`, fieldID).Scan(

						&field.Id,
						&field.TableId,
						&field.Required,
						&field.Slug,
						&field.Label,
						&field.Default,
						&field.Type,
						&field.Index,
						&field.Attributes,
						&field.IsVisible,
						&field.IsSystem,
						&field.IsSearch,
						&field.AutofillField,
						&field.AutofillTable,
						&field.RelationId,
						&field.Unique,
						&field.Automatic,
						&field.EnableMultilanguage,
					)
					if err != nil {
						return nil, err
					}
					if field.Id != "" {
						if req.LanguageSetting != "" && field.EnableMultilanguage {
							if strings.HasSuffix(field.Slug, "_"+req.LanguageSetting) {
								fieldAsAttribute = append(fieldAsAttribute, &field)
							} else {
								continue
							}
						} else {
							fieldAsAttribute = append(fieldAsAttribute, &field)
						}
					}
				}

				var tableFields []*nb.Field
				rows, err := conn.Query(ctx, "SELECT id, autofill_table, autofill_field, slug FROM field WHERE table_slug = $1", req.TableSlug)
				if err != nil {
					return nil, err
				}
				defer rows.Close()

				for rows.Next() {
					field := nb.Field{}
					err := rows.Scan(&field.Id, &field.AutofillTable, &field.AutofillField, &field.Slug)
					if err != nil {
						return nil, err
					}
					tableFields = append(tableFields, &field)
				}
				autofillFields := []map[string]any{}
				for _, field := range tableFields {
					autoFillTable := field.AutofillTable
					splitedAutoFillTable := make([]string, 0)
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

						originalAttributes := make(map[string]any)
						dynamicTables := []string{}
						var viewField nb.FieldResponse

						if relation.Type == "Many2Dynamic" {

							for _, dynamicTable := range relation.DynamicTables {
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
								viewFieldsInDynamicTable := []string{}
								for _, fieldID := range viewFieldsOfDynamicRelation {
									err := conn.QueryRow(ctx, "SELECT id, table_id, slug, label, attributes FROM field WHERE id = $1", fieldID).Scan(&viewField.Id, &viewField.TableId, &viewField.Slug, &viewField.Label, &viewField.Attributes)
									if err != nil {
										return nil, err
									}
									if viewField.Attributes != nil {
										attributesBytes, err := viewField.Attributes.MarshalJSON()
										if err != nil {
											return nil, err
										}
										err = json.Unmarshal(attributesBytes, &viewField.Attributes)
										if err != nil {
											return nil, err

										}
										if req.LanguageSetting != "" && viewField.EnableMultilanguage {
											if strings.HasSuffix(viewField.Slug, "_"+req.LanguageSetting) {
												viewFieldsInDynamicTable = append(viewFieldsInDynamicTable, viewField.Slug)
											} else {
												continue
											}
										} else {
											viewFieldsInDynamicTable = append(viewFieldsInDynamicTable, viewField.Slug)
										}
									}
									dynamicTableToAttribute := make(map[string]any)
									dynamicTableToAttribute["view_fields"] = viewFieldsInDynamicTable

									originalAttributes := make(map[string]any)
									if viewOfDynamicRelation.Attributes != nil {
										attributesBytes, err := viewOfDynamicRelation.Attributes.MarshalJSON()
										if err != nil {
											return nil, err
										}
										err = json.Unmarshal(attributesBytes, &originalAttributes)
										if err != nil {
											return nil, err
										}
									}
								}

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
						originalAttributes["default_values"] = viewOfRelation.DefaultValues
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
						fieldRes = append(fieldRes, &nb.FieldResponse{
							Id:                  field.Id,
							TableId:             field.TableId,
							Required:            field.Required,
							Slug:                field.Slug,
							Label:               field.Label,
							Default:             field.Default,
							Type:                field.Type,
							Index:               field.Index,
							EnableMultilanguage: field.EnableMultilanguage,
							Attributes: &structpb.Struct{
								Fields: field.Attributes.Fields,
							},
						})

						if strings.Contains(fieldReq.Id, "@") {
							field.Id = fieldReq.Id
						} else {
							guid := fieldReq.Id
							fieldQuery := `
								SELECT
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
									"is_system",
									"is_search",
									"autofill_field",
									"autofill_table",
									"relation_id",
									"unique",
									"automatic",
									"enable_multilanguage",

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
								&field.Attributes,
								&field.IsVisible,
								&field.IsSystem,
								&field.IsSearch,
								&field.AutofillField,
								&field.AutofillTable,
								&field.RelationId,
								&field.Unique,
								&field.Automatic,
								&field.EnableMultilanguage,
							)
							if err != nil {
								return nil, err
							}
							if field != nil {
								viewField.IsVisibleLayout = fieldReq.IsVisibleLayout
								viewField.Order = fieldReq.Order
								viewField.Id = fieldReq.Id
								viewField.RelationType = fieldReq.RelationType
								viewField.Label = fieldReq.Label
								viewField.Attributes = field.Attributes
								fieldRes = append(fieldRes, &nb.FieldResponse{
									Id:              viewField.Id,
									Column:          viewField.Column,
									RelationType:    viewField.RelationType,
									Order:           viewField.Order,
									IsVisibleLayout: viewField.IsVisibleLayout,
									Label:           viewField.Label,
									Attributes:      viewField.Attributes,
								})
							}
							fieldRes = append(fieldRes, &nb.FieldResponse{
								Id:              viewField.Id,
								Column:          viewField.Column,
								RelationType:    viewField.RelationType,
								Order:           viewField.Order,
								IsVisibleLayout: viewField.IsVisibleLayout,
								Label:           viewField.Label,
								Attributes:      viewField.Attributes,
							})
						}
						var fieldResponse []models.Field
						err = helper.MarshalToStruct(&fieldRes, &fieldResponse)
						if err != nil {
							return nil, err
						}

						fieldsWithPermissions, err := helper.AddPermissionToFieldv2(ctx, conn, fieldResponse, req.TableId, req.RoleId)
						if err != nil {
							return nil, err
						}

						section.Fields = make([]*nb.FieldResponse, len(fieldsWithPermissions))
						for i, field := range fieldsWithPermissions {
							section.Fields[i] = &nb.FieldResponse{
								Id:              field.Id,
								Column:          field.Column,
								RelationType:    field.RelationType,
								Order:           field.Order,
								IsVisibleLayout: field.IsVisibleLayout,
								Label:           field.Label,
								Attributes:      field.Attributes,
							}

						}
					}
				}
			}

		}
	}
	resp.Sections = sectionResponses

	return resp, nil
}

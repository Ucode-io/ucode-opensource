package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

type relationRepo struct {
	db *psqlpool.Pool
}

func NewRelationRepo(db *psqlpool.Pool) storage.RelationRepoI {
	return &relationRepo{
		db: db,
	}
}

func (r *relationRepo) Create(ctx context.Context, data *nb.CreateRelationRequest) (resp *nb.CreateRelationRequest, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.Create")
	defer dbSpan.Finish()

	var (
		fieldFrom, fieldTo, recursiveFieldId string
		table                                *nb.Table
		autoFilters                          []byte
	)

	conn, err := psqlpool.Get(data.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.CreateRelationRequest{}

	if len(data.Id) == 0 {
		data.Id = uuid.New().String()
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

	roles, err := helper.RolesFind(ctx, models.RelationHelper{Tx: tx})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find roles")
	}

	switch data.Type {
	case config.MANY2DYNAMIC:
		fieldFrom = data.RelationFieldSlug
	case config.MANY2MANY:
		fieldFrom = data.TableTo + "_ids"
		fieldTo = data.TableFrom + "_ids"
		tableTo, err := helper.TableFindOneTx(ctx, tx, data.TableTo)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_to")
		}
		table = tableTo

		exists, result, err := helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: fieldFrom,
			TableID:   table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err := helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx,
			Field: &nb.CreateFieldRequest{
				Id:         data.RelationFieldId,
				TableId:    tableTo.Id,
				Slug:       fieldTo,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableTo,
				Type:       "LOOKUPS",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableTo,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}

		tableFrom, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		table = tableFrom

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}
		if layout != nil {
			tab, err := helper.TabFindOne(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tab")
			}
			if tab == nil {
				tab, err = helper.TabCreate(ctx, models.RelationHelper{
					Tx:        tx,
					Order:     1,
					Label:     "Tab",
					Type:      "section",
					TableSlug: table.GetSlug(),
					LayoutID:  layout.GetId(),
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create tab")
				}
			}

			sections, err := helper.SectionFind(ctx, models.RelationHelper{
				Tx:    tx,
				TabID: tab.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find sections")
			}

			if len(sections) == 0 {
				fields := []*nb.FieldForSection{
					{
						Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
						Order:           1,
						FieldName:       "",
						RelationType:    config.MANY2MANY,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
				}
				err = helper.SectionCreate(ctx, models.RelationHelper{
					Tx:      tx,
					Order:   len(sections) + 1,
					Fields:  fields,
					TableID: table.Id,
					TabID:   tab.Id,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create section")
				}
			}

			if len(sections) > 0 {
				countColumns := 0
				if len(sections[0].Fields) > 0 {
					countColumns = len(sections[0].Fields)
				}

				sectionColumnCount := 3
				if table.SectionColumnCount != 0 {
					sectionColumnCount = int(table.SectionColumnCount)
				}

				if countColumns < int(sectionColumnCount) {
					fields := []*nb.FieldForSection{}

					fields = append(fields, sections[0].Fields...)

					fields = append(fields, &nb.FieldForSection{
						Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
						Order:           int32(countColumns) + 1,
						FieldName:       "",
						RelationType:    config.MANY2MANY,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
					)

					err = helper.SectionFindOneAndUpdate(ctx, models.RelationHelper{
						Tx:        tx,
						SectionID: sections[0].Id,
						Fields:    fields,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to find one and update section")
					}
				} else {
					fields := []*nb.FieldForSection{
						{
							Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
							Order:           1,
							FieldName:       "",
							RelationType:    config.MANY2MANY,
							IsVisibleLayout: true,
							ShowLabel:       true,
							Attributes:      data.Attributes,
						},
					}
					err = helper.SectionCreate(ctx, models.RelationHelper{
						Tx:      tx,
						Fields:  fields,
						TableID: table.Id,
						TabID:   tab.Id,
						Order:   len(sections) + 1,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to create section")
					}
				}

			}
		}

		exists, result, err = helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: fieldFrom,
			TableID:   tableFrom.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err = helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx,
			Field: &nb.CreateFieldRequest{
				Id:         data.RelationToFieldId,
				TableId:    tableFrom.Id,
				Slug:       fieldFrom,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableTo,
				Type:       "LOOKUPS",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableFrom,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}
	case config.MANY2ONE:
		fieldFrom = data.TableTo + "_id"
		fieldTo = "id"
		table, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		exists, result, err := helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: fieldFrom,
			TableID:   table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err := helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx, Field: &nb.CreateFieldRequest{
				Id:         data.RelationFieldId,
				TableId:    table.Id,
				Slug:       fieldFrom,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableTo,
				Type:       "LOOKUP",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}

		if layout != nil {
			tab, err := helper.TabFindOne(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.GetId(),
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tab")
			}

			if tab == nil {
				tab, err = helper.TabCreate(ctx, models.RelationHelper{
					Tx:        tx,
					LayoutID:  layout.GetId(),
					TableSlug: table.GetSlug(),
					Order:     1,
					Label:     "Tab",
					Type:      "section",
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create tab")
				}
			}

			sections, err := helper.SectionFind(ctx, models.RelationHelper{
				Tx:    tx,
				TabID: tab.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find sections")
			}

			if len(sections) == 0 {
				fields := []*nb.FieldForSection{
					{
						Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
						Order:           1,
						FieldName:       "",
						RelationType:    config.MANY2ONE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
				}
				err = helper.SectionCreate(ctx, models.RelationHelper{
					Tx:      tx,
					Order:   len(sections) + 1,
					Fields:  fields,
					TableID: table.Id,
					TabID:   tab.Id,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create section")
				}
			}

			if len(sections) > 0 {
				countColumns := 0
				if len(sections[0].Fields) > 0 {
					countColumns = len(sections[0].Fields)
				}

				sectionColumnCount := 3
				if table.SectionColumnCount != 0 {
					sectionColumnCount = int(table.SectionColumnCount)
				}

				if countColumns < int(sectionColumnCount) {
					fields := []*nb.FieldForSection{}

					fields = append(fields, sections[0].Fields...)

					fields = append(fields, &nb.FieldForSection{
						Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
						Order:           int32(countColumns) + 1,
						FieldName:       "",
						RelationType:    config.MANY2ONE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
					)

					err = helper.SectionFindOneAndUpdate(ctx, models.RelationHelper{
						Tx:        tx,
						SectionID: sections[0].Id,
						Fields:    fields,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to find one and update section")
					}
				} else {
					fields := []*nb.FieldForSection{
						{
							Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
							Order:           1,
							FieldName:       "",
							RelationType:    config.MANY2ONE,
							IsVisibleLayout: true,
							ShowLabel:       true,
							Attributes:      data.Attributes,
						},
					}
					err = helper.SectionCreate(ctx, models.RelationHelper{
						Tx:      tx,
						Fields:  fields,
						TableID: table.Id,
						TabID:   tab.Id,
						Order:   len(sections) + 1,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to create section")
					}
				}
			}
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableFrom,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}
	case config.RECURSIVE:
		recursiveFieldId = data.TableFrom + "_id"
		fieldFrom = "id"
		fieldTo = data.TableFrom + "_id"
		table, err = helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		exists, result, err := helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: recursiveFieldId,
			TableID:   table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err := helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx,
			Field: &nb.CreateFieldRequest{
				Id:         data.RelationFieldId,
				TableId:    table.Id,
				Slug:       recursiveFieldId,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableFrom,
				Type:       "LOOKUP",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}

		if layout != nil {
			tab, err := helper.TabFindOne(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.GetId(),
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tab")
			}

			if tab == nil {
				tab, err = helper.TabCreate(ctx, models.RelationHelper{
					Tx:        tx,
					LayoutID:  layout.GetId(),
					TableSlug: table.GetSlug(),
					Order:     1,
					Label:     "Tab",
					Type:      "section",
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create tab")
				}
			}

			sections, err := helper.SectionFind(ctx, models.RelationHelper{
				Tx:    tx,
				TabID: tab.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find sections")
			}

			if len(sections) == 0 {
				fields := []*nb.FieldForSection{
					{
						Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
						Order:           1,
						FieldName:       "",
						RelationType:    config.RECURSIVE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
				}
				err = helper.SectionCreate(ctx, models.RelationHelper{
					Tx:      tx,
					Order:   len(sections) + 1,
					Fields:  fields,
					TableID: table.Id,
					TabID:   tab.Id,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create section")
				}
			}

			if len(sections) > 0 {
				countColumns := 0
				if len(sections[0].Fields) > 0 {
					countColumns = len(sections[0].Fields)
				}

				sectionColumnCount := 3
				if table.SectionColumnCount != 0 {
					sectionColumnCount = int(table.SectionColumnCount)
				}

				if countColumns < int(sectionColumnCount) {
					fields := []*nb.FieldForSection{}

					fields = append(fields, sections[0].Fields...)

					fields = append(fields, &nb.FieldForSection{

						Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
						Order:           int32(countColumns) + 1,
						FieldName:       "",
						RelationType:    config.RECURSIVE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
					)

					err = helper.SectionFindOneAndUpdate(ctx, models.RelationHelper{
						Tx:        tx,
						SectionID: sections[0].Id,
						Fields:    fields,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to find one and update section")
					}
				} else {
					fields := []*nb.FieldForSection{
						{
							Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
							Order:           1,
							FieldName:       "",
							RelationType:    config.RECURSIVE,
							IsVisibleLayout: true,
							ShowLabel:       true,
							Attributes:      data.Attributes,
						},
					}
					err = helper.SectionCreate(ctx, models.RelationHelper{
						Tx:      tx,
						Fields:  fields,
						TableID: table.Id,
						TabID:   tab.Id,
						Order:   len(sections) + 1,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to create section")
					}
				}
			}
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableFrom,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}
	}

	query := `
		INSERT INTO "relation" (
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
			"is_system", 
			"object_id_from_jwt",
			"cascading_tree_table_slug", 
			"cascading_tree_field_slug",
			"auto_filters" 
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING 
			"id", 
			"type",
			"relation_field_slug", 
			"dynamic_tables", 
			"editable",
			"is_user_id_default", 
			"object_id_from_jwt",
			"cascading_tree_table_slug", 
			"cascading_tree_field_slug"`

	if data.AutoFilters != nil || len(data.AutoFilters) == 0 {
		autoFilters, err = json.Marshal(data.AutoFilters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal")
		}
	} else {
		autoFilters = []byte(`[{}]`)
	}

	err = tx.QueryRow(ctx, query,
		data.Id,
		data.TableFrom,
		data.TableTo,
		fieldFrom,
		fieldTo,
		data.Type,
		data.ViewFields,
		data.RelationFieldSlug,
		data.DynamicTables,
		data.Editable,
		data.IsUserIdDefault,
		false,
		data.ObjectIdFromJwt,
		data.CascadingTreeTableSlug,
		data.CascadingTreeFieldSlug,
		autoFilters,
	).Scan(
		&resp.Id,
		&resp.Type,
		&resp.RelationFieldSlug,
		&resp.DynamicTables,
		&resp.Editable,
		&resp.IsUserIdDefault,
		&resp.ObjectIdFromJwt,
		&resp.CascadingTreeTableSlug,
		&resp.CascadingTreeFieldSlug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert relation")
	}

	if resp.Type != config.MANY2DYNAMIC {
		tableTo, err := helper.TableFindOneTx(ctx, tx, data.TableTo)
		if err != nil {
			return nil, err
		}
		tableFrom, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		viewRequest := &nb.CreateViewRequest{
			Id:         uuid.NewString(),
			Type:       data.ViewType,
			RelationId: resp.Id,
			Attributes: data.Attributes,
			TableSlug:  "",
			GroupFields: func() []string {
				if len(data.GroupFields) == 0 {
					return []string{}
				}
				return data.GroupFields
			}(),
			ViewFields: func() []string {
				if len(data.ViewFields) == 0 {
					return []string{}
				}
				return data.ViewFields
			}(),
			MainField:    "",
			QuickFilters: data.QuickFilters,
			Users:        []string{},
			Name:         "",
			Columns: func() []string {
				if len(data.Columns) == 0 {
					return []string{}
				}
				return data.Columns
			}(),
			MultipleInsert:      data.MultipleInsert,
			IsEditable:          data.IsEditable,
			RelationTableSlug:   data.RelationFieldSlug,
			MultipleInsertField: data.MultipleInsertField,
			UpdatedFields: func() []string {
				if len(data.UpdatedFields) == 0 {
					return []string{}
				}
				return data.UpdatedFields
			}(),
			DefaultLimit:    data.DefaultLimit,
			DefaultEditable: data.DefaultEditable,
		}

		err = helper.ViewCreate(ctx, models.RelationHelper{
			Tx:   tx,
			View: viewRequest,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create view")
		}

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: tableTo.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}

		if layout != nil {
			tabs, err := helper.TabFind(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tabs")
			}

			var label string
			if tableFrom != nil && tableFrom.Label != "" {
				label = tableFrom.Label
			} else {
				label = "Relation tab" + data.TableFrom
			}

			_, err = helper.TabCreate(ctx, models.RelationHelper{
				Tx:         tx,
				Order:      len(tabs) + 1,
				Label:      label,
				Type:       "relation",
				LayoutID:   layout.Id,
				RelationID: resp.Id,
				Attributes: data.Attributes,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to create tab")
			}

			err = helper.ViewRelationPermission(ctx, models.RelationHelper{
				Tx:         tx,
				TableSlug:  tableTo.Slug,
				RelationID: resp.Id,
				RoleIDs:    roles,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to create view relation permission")
			}
		}
	}

	err = helper.ExecRelation(ctx, models.RelationHelper{
		Tx:           tx,
		TableFrom:    data.TableFrom,
		TableTo:      data.TableTo,
		FieldFrom:    fieldFrom,
		FieldTo:      fieldTo,
		RelationType: data.Type,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to exec relation")
	}

	resp.Attributes = data.Attributes

	query = `
		UPDATE "view"
		SET columns = array_append(columns, $1)
		WHERE table_slug = $2
	`
	_, err = tx.Exec(ctx, query, resp.Id, data.TableFrom)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exec update view")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return resp, nil
}

func (r *relationRepo) CreateWithTx(ctx context.Context, data *nb.CreateRelationRequest, tx pgx.Tx) (resp *nb.CreateRelationRequest, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.Create")
	defer dbSpan.Finish()

	var (
		fieldFrom, fieldTo, recursiveFieldId string
		table                                *nb.Table
		autoFilters                          []byte
	)

	resp = &nb.CreateRelationRequest{}

	if len(data.Id) == 0 {
		data.Id = uuid.New().String()
	}

	roles, err := helper.RolesFind(ctx, models.RelationHelper{Tx: tx})
	if err != nil {
		return nil, errors.Wrap(err, "failed to find roles")
	}

	switch data.Type {
	case config.MANY2MANY:
		fieldFrom = data.TableTo + "_ids"
		fieldTo = data.TableFrom + "_ids"
		tableTo, err := helper.TableFindOneTx(ctx, tx, data.TableTo)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_to")
		}
		table = tableTo

		exists, result, err := helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: fieldFrom,
			TableID:   table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err := helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx,
			Field: &nb.CreateFieldRequest{
				Id:         data.RelationFieldId,
				TableId:    tableTo.Id,
				Slug:       fieldTo,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableTo,
				Type:       "LOOKUPS",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableTo,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}

		tableFrom, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		table = tableFrom

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}
		if layout != nil {
			tab, err := helper.TabFindOne(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tab")
			}
			if tab == nil {
				tab, err = helper.TabCreate(ctx, models.RelationHelper{
					Tx:        tx,
					Order:     1,
					Label:     "Tab",
					Type:      "section",
					TableSlug: table.GetSlug(),
					LayoutID:  layout.GetId(),
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create tab")
				}
			}

			sections, err := helper.SectionFind(ctx, models.RelationHelper{
				Tx:    tx,
				TabID: tab.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find sections")
			}

			if len(sections) == 0 {
				fields := []*nb.FieldForSection{
					{
						Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
						Order:           1,
						FieldName:       "",
						RelationType:    config.MANY2MANY,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
				}
				err = helper.SectionCreate(ctx, models.RelationHelper{
					Tx:      tx,
					Order:   len(sections) + 1,
					Fields:  fields,
					TableID: table.Id,
					TabID:   tab.Id,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create section")
				}
			}

			if len(sections) > 0 {
				countColumns := 0
				if len(sections[0].Fields) > 0 {
					countColumns = len(sections[0].Fields)
				}

				sectionColumnCount := 3
				if table.SectionColumnCount != 0 {
					sectionColumnCount = int(table.SectionColumnCount)
				}

				if countColumns < int(sectionColumnCount) {
					fields := []*nb.FieldForSection{}

					fields = append(fields, sections[0].Fields...)

					fields = append(fields, &nb.FieldForSection{
						Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
						Order:           int32(countColumns) + 1,
						FieldName:       "",
						RelationType:    config.MANY2MANY,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
					)

					err = helper.SectionFindOneAndUpdate(ctx, models.RelationHelper{
						Tx:        tx,
						SectionID: sections[0].Id,
						Fields:    fields,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to find one and update section")
					}
				} else {
					fields := []*nb.FieldForSection{
						{
							Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
							Order:           1,
							FieldName:       "",
							RelationType:    config.MANY2MANY,
							IsVisibleLayout: true,
							ShowLabel:       true,
							Attributes:      data.Attributes,
						},
					}
					err = helper.SectionCreate(ctx, models.RelationHelper{
						Tx:      tx,
						Fields:  fields,
						TableID: table.Id,
						TabID:   tab.Id,
						Order:   len(sections) + 1,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to create section")
					}
				}

			}
		}

		exists, result, err = helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: fieldFrom,
			TableID:   tableFrom.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err = helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx,
			Field: &nb.CreateFieldRequest{
				Id:         data.RelationToFieldId,
				TableId:    tableFrom.Id,
				Slug:       fieldFrom,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableTo,
				Type:       "LOOKUPS",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableFrom,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}
	case config.MANY2ONE:
		fieldFrom = data.TableTo + "_id"
		fieldTo = "id"
		table, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		exists, result, err := helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: fieldFrom,
			TableID:   table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err := helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx, Field: &nb.CreateFieldRequest{
				Id:         data.RelationFieldId,
				TableId:    table.Id,
				Slug:       fieldFrom,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableTo,
				Type:       "LOOKUP",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}

		if layout != nil {
			tab, err := helper.TabFindOne(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.GetId(),
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tab")
			}

			if tab == nil {
				tab, err = helper.TabCreate(ctx, models.RelationHelper{
					Tx:        tx,
					LayoutID:  layout.GetId(),
					TableSlug: table.GetSlug(),
					Order:     1,
					Label:     "Tab",
					Type:      "section",
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create tab")
				}
			}

			sections, err := helper.SectionFind(ctx, models.RelationHelper{
				Tx:    tx,
				TabID: tab.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find sections")
			}

			if len(sections) == 0 {
				fields := []*nb.FieldForSection{
					{
						Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
						Order:           1,
						FieldName:       "",
						RelationType:    config.MANY2ONE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
				}
				err = helper.SectionCreate(ctx, models.RelationHelper{
					Tx:      tx,
					Order:   len(sections) + 1,
					Fields:  fields,
					TableID: table.Id,
					TabID:   tab.Id,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create section")
				}
			}

			if len(sections) > 0 {
				countColumns := 0
				if len(sections[0].Fields) > 0 {
					countColumns = len(sections[0].Fields)
				}

				sectionColumnCount := 3
				if table.SectionColumnCount != 0 {
					sectionColumnCount = int(table.SectionColumnCount)
				}

				if countColumns < int(sectionColumnCount) {
					fields := []*nb.FieldForSection{}

					fields = append(fields, sections[0].Fields...)

					fields = append(fields, &nb.FieldForSection{
						Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
						Order:           int32(countColumns) + 1,
						FieldName:       "",
						RelationType:    config.MANY2ONE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
					)

					err = helper.SectionFindOneAndUpdate(ctx, models.RelationHelper{
						Tx:        tx,
						SectionID: sections[0].Id,
						Fields:    fields,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to find one and update section")
					}
				} else {
					fields := []*nb.FieldForSection{
						{
							Id:              fmt.Sprintf("%s#%s", data.TableTo, data.Id),
							Order:           1,
							FieldName:       "",
							RelationType:    config.MANY2ONE,
							IsVisibleLayout: true,
							ShowLabel:       true,
							Attributes:      data.Attributes,
						},
					}
					err = helper.SectionCreate(ctx, models.RelationHelper{
						Tx:      tx,
						Fields:  fields,
						TableID: table.Id,
						TabID:   tab.Id,
						Order:   len(sections) + 1,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to create section")
					}
				}
			}
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableFrom,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}
	case config.RECURSIVE:
		recursiveFieldId = data.TableFrom + "_id"
		fieldFrom = "id"
		fieldTo = data.TableFrom + "_id"
		table, err = helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		exists, result, err := helper.CheckRelationFieldExists(ctx, models.RelationHelper{
			Tx:        tx,
			FieldName: recursiveFieldId,
			TableID:   table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to check relation field exists")
		}
		if exists {
			fieldFrom = result
		}

		field, err := helper.UpsertField(ctx, models.RelationHelper{
			Tx: tx,
			Field: &nb.CreateFieldRequest{
				Id:         data.RelationFieldId,
				TableId:    table.Id,
				Slug:       recursiveFieldId,
				Label:      "FROM " + data.TableFrom + " TO " + data.TableFrom,
				Type:       "LOOKUPS",
				RelationId: data.Id,
				Attributes: data.Attributes,
			},
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to upsert field")
		}

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: table.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}

		if layout != nil {
			tab, err := helper.TabFindOne(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.GetId(),
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tab")
			}

			if tab == nil {
				tab, err = helper.TabCreate(ctx, models.RelationHelper{
					Tx:        tx,
					LayoutID:  layout.GetId(),
					TableSlug: table.GetSlug(),
					Order:     1,
					Label:     "Tab",
					Type:      "section",
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create tab")
				}
			}

			sections, err := helper.SectionFind(ctx, models.RelationHelper{
				Tx:    tx,
				TabID: tab.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find sections")
			}

			if len(sections) == 0 {
				fields := []*nb.FieldForSection{
					{
						Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
						Order:           1,
						FieldName:       "",
						RelationType:    config.RECURSIVE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
				}
				err = helper.SectionCreate(ctx, models.RelationHelper{
					Tx:      tx,
					Order:   len(sections) + 1,
					Fields:  fields,
					TableID: table.Id,
					TabID:   tab.Id,
				})
				if err != nil {
					return nil, errors.Wrap(err, "failed to create section")
				}
			}

			if len(sections) > 0 {
				countColumns := 0
				if len(sections[0].Fields) > 0 {
					countColumns = len(sections[0].Fields)
				}

				sectionColumnCount := 3
				if table.SectionColumnCount != 0 {
					sectionColumnCount = int(table.SectionColumnCount)
				}

				if countColumns < int(sectionColumnCount) {
					fields := []*nb.FieldForSection{}

					fields = append(fields, sections[0].Fields...)

					fields = append(fields, &nb.FieldForSection{

						Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
						Order:           int32(countColumns) + 1,
						FieldName:       "",
						RelationType:    config.RECURSIVE,
						IsVisibleLayout: true,
						ShowLabel:       true,
						Attributes:      data.Attributes,
					},
					)

					err = helper.SectionFindOneAndUpdate(ctx, models.RelationHelper{
						Tx:        tx,
						SectionID: sections[0].Id,
						Fields:    fields,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to find one and update section")
					}
				} else {
					fields := []*nb.FieldForSection{
						{
							Id:              fmt.Sprintf("%s#%s", data.TableFrom, data.Id),
							Order:           1,
							FieldName:       "",
							RelationType:    config.RECURSIVE,
							IsVisibleLayout: true,
							ShowLabel:       true,
							Attributes:      data.Attributes,
						},
					}
					err = helper.SectionCreate(ctx, models.RelationHelper{
						Tx:      tx,
						Fields:  fields,
						TableID: table.Id,
						TabID:   tab.Id,
						Order:   len(sections) + 1,
					})
					if err != nil {
						return nil, errors.Wrap(err, "failed to create section")
					}
				}
			}
		}

		err = helper.RelationFieldPermission(ctx, models.RelationHelper{
			Tx:        tx,
			FieldID:   field.Id,
			TableSlug: data.TableFrom,
			Label:     "FROM " + data.TableFrom + " TO " + data.TableTo,
			RoleIDs:   roles,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create relation field permission")
		}
	}

	query := `
		INSERT INTO "relation" (
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
			"is_system", 
			"object_id_from_jwt",
			"cascading_tree_table_slug", 
			"cascading_tree_field_slug",
			"auto_filters" 
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16)
		RETURNING 
			"id", 
			"type",
			"relation_field_slug", 
			"dynamic_tables", 
			"editable",
			"is_user_id_default", 
			"object_id_from_jwt",
			"cascading_tree_table_slug", 
			"cascading_tree_field_slug"`

	if data.AutoFilters != nil || len(data.AutoFilters) == 0 {
		autoFilters, err = json.Marshal(data.AutoFilters)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal")
		}
	} else {
		autoFilters = []byte(`[{}]`)
	}

	err = tx.QueryRow(ctx, query,
		data.Id,
		data.TableFrom,
		data.TableTo,
		fieldFrom,
		fieldTo,
		data.Type,
		data.ViewFields,
		data.RelationFieldSlug,
		data.DynamicTables,
		data.Editable,
		data.IsUserIdDefault,
		false,
		data.ObjectIdFromJwt,
		data.CascadingTreeTableSlug,
		data.CascadingTreeFieldSlug,
		autoFilters,
	).Scan(
		&resp.Id,
		&resp.Type,
		&resp.RelationFieldSlug,
		&resp.DynamicTables,
		&resp.Editable,
		&resp.IsUserIdDefault,
		&resp.ObjectIdFromJwt,
		&resp.CascadingTreeTableSlug,
		&resp.CascadingTreeFieldSlug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to insert relation")
	}

	if resp.Type != config.MANY2DYNAMIC {
		tableTo, err := helper.TableFindOneTx(ctx, tx, data.TableTo)
		if err != nil {
			return nil, err
		}
		tableFrom, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
		if err != nil {
			return nil, errors.Wrap(err, "failed to find table_from")
		}

		viewRequest := &nb.CreateViewRequest{
			Id:         uuid.NewString(),
			Type:       data.ViewType,
			RelationId: resp.Id,
			Attributes: data.Attributes,
			TableSlug:  "",
			GroupFields: func() []string {
				if len(data.GroupFields) == 0 {
					return []string{}
				}
				return data.GroupFields
			}(),
			ViewFields: func() []string {
				if len(data.ViewFields) == 0 {
					return []string{}
				}
				return data.ViewFields
			}(),
			MainField:    "",
			QuickFilters: data.QuickFilters,
			Users:        []string{},
			Name:         "",
			Columns: func() []string {
				if len(data.Columns) == 0 {
					return []string{}
				}
				return data.Columns
			}(),
			MultipleInsert:      data.MultipleInsert,
			IsEditable:          data.IsEditable,
			RelationTableSlug:   data.RelationFieldSlug,
			MultipleInsertField: data.MultipleInsertField,
			UpdatedFields: func() []string {
				if len(data.UpdatedFields) == 0 {
					return []string{}
				}
				return data.UpdatedFields
			}(),
			DefaultLimit:    data.DefaultLimit,
			DefaultEditable: data.DefaultEditable,
		}

		err = helper.ViewCreate(ctx, models.RelationHelper{
			Tx:   tx,
			View: viewRequest,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create view")
		}

		layout, err := helper.LayoutFindOne(ctx, models.RelationHelper{
			Tx:      tx,
			TableID: tableTo.Id,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to find layout")
		}

		if layout != nil {
			tabs, err := helper.TabFind(ctx, models.RelationHelper{
				Tx:       tx,
				LayoutID: layout.Id,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to find tabs")
			}

			var label string
			if tableFrom != nil && tableFrom.Label != "" {
				label = tableFrom.Label
			} else {
				label = "Relation tab" + data.TableFrom
			}

			_, err = helper.TabCreate(ctx, models.RelationHelper{
				Tx:         tx,
				Order:      len(tabs) + 1,
				Label:      label,
				Type:       "relation",
				LayoutID:   layout.Id,
				RelationID: resp.Id,
				Attributes: data.Attributes,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to create tab")
			}

			err = helper.ViewRelationPermission(ctx, models.RelationHelper{
				Tx:         tx,
				TableSlug:  tableTo.Slug,
				RelationID: resp.Id,
				RoleIDs:    roles,
			})
			if err != nil {
				return nil, errors.Wrap(err, "failed to create view relation permission")
			}
		}
	}

	err = helper.ExecRelation(ctx, models.RelationHelper{
		Tx:           tx,
		TableFrom:    data.TableFrom,
		TableTo:      data.TableTo,
		FieldFrom:    fieldFrom,
		FieldTo:      fieldTo,
		RelationType: data.Type,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to exec relation")
	}

	resp.Attributes = data.Attributes

	query = `
		UPDATE "view"
		SET columns = array_append(columns, $1)
		WHERE table_slug = $2
	`
	_, err = tx.Exec(ctx, query, resp.Id, data.TableFrom)
	if err != nil {
		return nil, errors.Wrap(err, "failed to exec update view")
	}

	return resp, nil
}

func (r *relationRepo) GetByID(ctx context.Context, data *nb.RelationPrimaryKey) (resp *nb.RelationForGetAll, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.GetByID")
	defer dbSpan.Finish()

	var (
		query = `SELECT
				r.id,
				r.table_from,
				r.table_to,
				r.field_from,
				r.field_to,
				r.type,
				r.relation_field_slug,
				r.dynamic_tables,
				r.editable,
				r.is_user_id_default,
				r.is_system,
				r.object_id_from_jwt,
				COALESCE(r.cascading_tree_table_slug, '') AS cascading_tree_table_slug,
				COALESCE(r.cascading_tree_field_slug, '') AS cascading_tree_field_slug,
				r.view_fields,
				r.auto_filters
			FROM
				relation r
			WHERE  r.id = $1`

		tableFromSlug, tableToSlug string
		dynamicTables              sql.NullString
		autoFilters                []byte
		viewFields                 []string
	)

	conn, err := psqlpool.Get(data.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp = &nb.RelationForGetAll{}

	err = conn.QueryRow(ctx, query, data.Id).Scan(
		&resp.Id,
		&tableFromSlug,
		&tableToSlug,
		&resp.FieldFrom,
		&resp.FieldTo,
		&resp.Type,
		&resp.RelationFieldSlug,
		&dynamicTables,
		&resp.Editable,
		&resp.IsUserIdDefault,
		&resp.IsSystem,
		&resp.ObjectIdFromJwt,
		&resp.CascadingTreeTableSlug,
		&resp.CascadingTreeFieldSlug,
		&viewFields,
		&autoFilters,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error when scan relation queryRow")
	}

	if len(autoFilters) == 0 {
		autoFilters = []byte(`[{}]`)
	}

	if err := json.Unmarshal(autoFilters, &resp.AutoFilters); err != nil {
		return nil, errors.Wrap(err, "error when unmarshl autofilter")
	}

	if len(viewFields) > 0 {
		query = `
			SELECT 
				"id",
				"table_id",
				"required",
				"slug",
				"label",
				"default",
				"type",
				"index",
				"is_visible",
				autofill_field,
				autofill_table,
				"unique",
				"automatic",
				relation_id
			FROM "field" WHERE id = ANY($1)`

		rows, err := conn.Query(ctx, query, pq.Array(viewFields))
		if err != nil {
			return nil, errors.Wrap(err, "error when exec query viewfields")
		}
		defer rows.Close()

		for rows.Next() {
			var (
				field             = nb.Field{}
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
				&field.IsVisible,
				&autoFillFieldNull,
				&autoFillTableNull,
				&field.Unique,
				&field.Automatic,
				&relationIdNull,
			)
			if err != nil {
				return nil, errors.Wrap(err, "error when scan query viewfields")
			}

			field.AutofillField = autoFillFieldNull.String
			field.AutofillTable = autoFillTableNull.String
			field.RelationField = relationIdNull.String
			field.Default = defaultStr.String
			field.Index = index.String

			resp.ViewFields = append(resp.ViewFields, &field)
		}
	}

	if dynamicTables.Valid {
		err = json.Unmarshal([]byte(dynamicTables.String), &resp.DynamicTables)
		if err != nil {
			return resp, errors.Wrap(err, "error whan unmarshl dynamicTable")
		}
	}

	tableFrom, err := helper.TableFindOne(ctx, conn, tableFromSlug)
	if err != nil {
		return nil, errors.Wrap(err, "error when tableFrom findOne")
	}

	tableTo, err := helper.TableFindOne(ctx, conn, tableToSlug)
	if err != nil {
		return nil, errors.Wrap(err, "error when tableTo findOne")
	}

	resp.TableFrom = tableFrom
	resp.TableTo = tableTo

	view, err := helper.ViewFindOne(ctx, models.RelationHelper{
		Conn:       conn,
		RelationID: resp.Id,
	})
	if err != nil {
		return nil, errors.Wrap(err, "error when view findOne")
	}

	if view != nil {
		resp.Title = view.Name
		resp.Columns = view.Columns
		resp.QuickFilters = view.QuickFilters
		resp.GroupFields = view.GroupFields
		resp.IsEditable = view.IsEditable
		resp.RelationTableSlug = view.RelationTableSlug
		resp.ViewType = view.Type
		resp.Id = view.RelationId
		resp.DefaultValues = view.DefaultValues
		resp.DefaultLimit = view.DefaultLimit
		resp.MultipleInsert = view.MultipleInsert
		resp.MultipleInsertField = view.MultipleInsertField
		resp.UpdatedFields = view.UpdatedFields
		resp.Creatable = view.Creatable
		resp.DefaultEditable = view.DefaultEditable
		resp.FunctionPath = view.FunctionPath
		resp.Attributes = view.Attributes
	}

	return resp, nil
}

func (r *relationRepo) GetList(ctx context.Context, data *nb.GetAllRelationsRequest) (resp *nb.GetAllRelationsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.GetList")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(data.GetProjectId())
	if err != nil {
		return nil, err
	}

	if data.TableSlug == "" {
		table, err := helper.TableFindOne(ctx, conn, data.TableId)
		if err != nil {
			return nil, err
		}
		data.TableSlug = table.Slug
	}

	var (
		relations     []*nb.RelationForGetAll
		tableToFilter string
	)

	resp = &nb.GetAllRelationsResponse{}

	params := make(map[string]any)
	params["table_slug"] = data.TableSlug

	if !data.DisableTableTo {
		tableToFilter = ` OR r.table_from = :table_slug `
	}

	query := fmt.Sprintf(`
		SELECT
    		r.id,
    		r.table_from,
    		r.table_to,
    		r.field_from,
    		r.field_to,
    		r.type,
    		r.relation_field_slug,
    		r.dynamic_tables,
    		r.editable,
    		r.is_user_id_default,
    		r.is_system,
    		r.object_id_from_jwt,
    		COALESCE(r.cascading_tree_table_slug, '') AS cascading_tree_table_slug,
    		COALESCE(r.cascading_tree_field_slug, '') AS cascading_tree_field_slug,
    		jsonb_agg(field.*) AS view_fields
		FROM
		    relation r
		INNER JOIN
		    field ON r.id = field.relation_id
		WHERE  r.table_to = :table_slug %s
			OR r.dynamic_tables->>'table_slug' = :table_slug
		GROUP BY r.id `, tableToFilter)

	query += ` ORDER BY r.created_at DESC `

	if data.Limit > 0 {
		query += ` LIMIT :limit `
		params["limit"] = data.Limit
	}

	if data.Offset >= 0 {
		query += ` OFFSET :offset `
		params["offset"] = data.Offset
	}

	query, args := helper.ReplaceQueryParams(query, params)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			viewFields, dynamicTables sql.NullString
		)
		relation := &nb.RelationForGetAll{
			TableFrom: &nb.Table{},
			TableTo:   &nb.Table{},
		}

		err := rows.Scan(
			&relation.Id,
			&relation.TableFrom.Slug,
			&relation.TableTo.Slug,
			&relation.FieldFrom,
			&relation.FieldTo,
			&relation.Type,
			&relation.RelationFieldSlug,
			&dynamicTables,
			&relation.Editable,
			&relation.IsUserIdDefault,
			&relation.IsSystem,
			&relation.ObjectIdFromJwt,
			&relation.CascadingTreeTableSlug,
			&relation.CascadingTreeFieldSlug,
			&viewFields,
		)
		if err != nil {
			return resp, err
		}

		if viewFields.Valid {
			err = json.Unmarshal([]byte(viewFields.String), &relation.ViewFields)
			if err != nil {
				return resp, err
			}
		}

		if dynamicTables.Valid && dynamicTables.String != "{}" {
			err = json.Unmarshal([]byte(dynamicTables.String), &relation.DynamicTables)
			if err != nil {
				return resp, err
			}
		}

		relations = append(relations, relation)
	}

	if len(relations) == 0 {
		return resp, nil
	}

	for i := 0; i < len(relations); i++ {

		tableFrom, err := helper.TableFindOne(ctx, conn, relations[i].TableFrom.Slug)
		if err != nil {
			return resp, err
		}

		relations[i].TableFrom = tableFrom

		tableTo, err := helper.TableFindOne(ctx, conn, relations[i].TableTo.Slug)
		if err != nil {
			return resp, err
		}

		relations[i].TableTo = tableTo

		view, err := helper.ViewFindOne(ctx, models.RelationHelper{
			Conn:       conn,
			RelationID: relations[i].Id,
		})
		if err != nil {
			return resp, err
		}

		if view != nil {
			relations[i].Title = view.Name
			relations[i].Columns = view.Columns
			relations[i].QuickFilters = view.QuickFilters
			relations[i].GroupFields = view.GroupFields
			relations[i].IsEditable = view.IsEditable
			relations[i].RelationTableSlug = view.RelationTableSlug
			relations[i].ViewType = view.Type
			relations[i].Id = view.RelationId
			relations[i].DefaultValues = view.DefaultValues
			relations[i].DefaultLimit = view.DefaultLimit
			relations[i].MultipleInsert = view.MultipleInsert
			relations[i].MultipleInsertField = view.MultipleInsertField
			relations[i].UpdatedFields = view.UpdatedFields
			relations[i].Creatable = view.Creatable
			relations[i].DefaultEditable = view.DefaultEditable
			relations[i].FunctionPath = view.FunctionPath
			relations[i].Attributes = view.Attributes
		}
	}

	query = `SELECT COUNT(*) FROM "relation" WHERE table_from = $1`

	err = conn.QueryRow(ctx, query, data.TableSlug).Scan(&resp.Count)
	if err != nil {
		return resp, err
	}

	resp.Relations = relations

	return resp, nil
}

func (r *relationRepo) Update(ctx context.Context, data *nb.UpdateRelationRequest) (resp *nb.RelationForGetAll, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.Update")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(data.GetProjectId())
	if err != nil {
		return nil, err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to start transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	if data.RelationTableSlug == "" {
		return resp, errors.New("relation table slug is required")
	}

	query := `
	UPDATE "relation"
	SET 
		"table_from" = $2, 
		"table_to" = $3, 
		"type" = $4,
		"view_fields" = $5, 
		"relation_field_slug" = $6, 
		"dynamic_tables" = $7, 
		"editable" = $8,
		"is_user_id_default" = $9, 
		"is_system" = $10, 
		"object_id_from_jwt" = $11,
		"cascading_tree_table_slug" = $12, 
		"cascading_tree_field_slug" = $13,
		"auto_filters" = $14
	WHERE "id" = $1 AND "type" = $4
	RETURNING 
		"id", 
		"type",
		"relation_field_slug", 
		"dynamic_tables", 
		"editable",
		"is_user_id_default", 
		"object_id_from_jwt",
		"cascading_tree_table_slug", 
		"cascading_tree_field_slug"`

	row, err := tx.Exec(ctx, query,
		data.Id,
		data.TableFrom,
		data.TableTo,
		data.Type,
		data.ViewFields,
		data.RelationFieldSlug,
		data.DynamicTables,
		data.Editable,
		data.IsUserIdDefault,
		false,
		data.ObjectIdFromJwt,
		data.CascadingTreeTableSlug,
		data.CascadingTreeFieldSlug,
		data.AutoFilters,
	)
	if err != nil {
		return nil, errors.Wrap(err, "failed to update relation")
	}

	if row.RowsAffected() == 0 {
		return nil, errors.New("relation type cannot be changed")
	}

	resp = &nb.RelationForGetAll{
		Id:                     data.Id,
		Type:                   data.Type,
		RelationFieldSlug:      data.RelationFieldSlug,
		DynamicTables:          data.DynamicTables,
		Editable:               data.Editable,
		IsUserIdDefault:        data.IsUserIdDefault,
		ObjectIdFromJwt:        data.ObjectIdFromJwt,
		CascadingTreeTableSlug: data.CascadingTreeTableSlug,
		CascadingTreeFieldSlug: data.CascadingTreeFieldSlug,
	}

	tableTo, err := helper.TableFindOneTx(ctx, tx, data.TableTo)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find table_to")
	}
	tableFrom, err := helper.TableFindOneTx(ctx, tx, data.TableFrom)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find table_from")
	}

	jsonAttr, err := json.Marshal(data.Attributes)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal")
	}

	updateField := `UPDATE "field" SET 
		attributes=$2, 
		required=$3 
	WHERE relation_id=$1`
	_, err = tx.Exec(ctx, updateField, data.Id, string(jsonAttr), data.Required)
	if err != nil {
		return nil, errors.Wrap(err, "cannot update field")
	}

	if len(data.ViewFields) > 0 {
		query = `
			SELECT 
				"id",
				"table_id",
				"required",
				"slug",
				"label",
				"default",
				"type",
				"index",
				"is_visible",
				autofill_field,
				autofill_table,
				"unique",
				"automatic",
				relation_id
			FROM "field" WHERE id = ANY($1)`

		rows, err := tx.Query(ctx, query, pq.Array(data.ViewFields))
		if err != nil {
			return nil, err
		}
		defer rows.Close()

		for rows.Next() {
			var (
				field             = nb.Field{}
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
				&field.IsVisible,
				&autoFillFieldNull,
				&autoFillTableNull,
				&field.Unique,
				&field.Automatic,
				&relationIdNull,
			)
			if err != nil {
				return nil, err
			}

			field.AutofillField = autoFillFieldNull.String
			field.AutofillTable = autoFillTableNull.String
			field.RelationField = relationIdNull.String
			field.Default = defaultStr.String
			field.Index = index.String

			resp.ViewFields = append(resp.ViewFields, &field)
		}
	}

	isViewExists, err := helper.ViewFindOneTx(ctx, models.RelationHelper{
		Tx:         tx,
		RelationID: data.Id,
	})

	query = `
        UPDATE view_relation_permission SET 
			label = $1
        WHERE relation_id = $2 AND table_slug = $3
    `

	_, err = tx.Exec(ctx, query, data.Title, data.Id, data.RelationTableSlug)
	if err != nil {
		return resp, errors.Wrap(err, "failed to update view relation permissions")
	}

	if isViewExists != nil {
		query := `
        UPDATE view SET 
            name = $2,
            quick_filters = $3,
            group_fields = $4,
            columns = $5,
            is_editable = $6,
            relation_table_slug = $7,
            type = $8,
            summaries = $9,
            default_values = $10,
            action_relations = $11,
            default_limit = $12,
            multiple_insert = $13,
            updated_fields = $14,
            default_editable = $15,
            creatable = $16,
            view_fields = $17,
            attributes = $18
        WHERE 
            relation_id = $1
    `

		_, err = tx.Exec(ctx, query,
			data.Id,
			data.Title,
			data.QuickFilters,
			func() []string {
				if len(data.GroupFields) == 0 {
					return nil
				}
				return data.GroupFields
			}(),
			func() []string {
				if len(data.Columns) == 0 {
					return nil
				}
				return data.Columns
			}(),
			data.IsEditable,
			data.RelationTableSlug,
			data.ViewType,
			data.Summaries,
			data.DefaultValues,
			data.ActionRelations,
			data.DefaultLimit,
			data.MultipleInsert,
			func() []string {
				if len(data.UpdatedFields) == 0 {
					return nil
				}
				return data.UpdatedFields
			}(),
			data.DefaultEditable,
			data.Creatable,
			func() []string {
				if len(data.ViewFields) == 0 {
					return nil
				}
				return data.ViewFields
			}(),
			data.Attributes,
		)
		if err != nil {
			return resp, errors.Wrap(err, "failed to update view")
		}
	} else {
		viewRequest := &nb.CreateViewRequest{
			Id:         uuid.NewString(),
			Type:       data.ViewType,
			RelationId: data.Id,
			Name:       data.Title,
			Attributes: data.Attributes,
			TableSlug:  "",
			GroupFields: func() []string {
				if len(data.GroupFields) == 0 {
					return []string{}
				}
				return data.GroupFields
			}(),
			ViewFields: func() []string {
				if len(data.ViewFields) == 0 {
					return []string{}
				}
				return data.ViewFields
			}(),
			QuickFilters: data.QuickFilters,
			Users:        []string{},
			Columns: func() []string {
				if len(data.Columns) == 0 {
					return []string{}
				}
				return data.Columns
			}(),
			MultipleInsert:    data.MultipleInsert,
			IsEditable:        data.IsEditable,
			RelationTableSlug: data.RelationFieldSlug,
			UpdatedFields: func() []string {
				if len(data.UpdatedFields) == 0 {
					return []string{}
				}
				return data.UpdatedFields
			}(),
			DefaultLimit:    data.DefaultLimit,
			DefaultEditable: data.DefaultEditable,
		}

		err = helper.ViewCreate(ctx, models.RelationHelper{
			Tx:   tx,
			View: viewRequest,
		})
		if err != nil {
			return nil, errors.Wrap(err, "failed to create view")
		}
	}

	resp.Attributes = data.Attributes
	resp.TableFrom = tableFrom
	resp.TableTo = tableTo

	err = tx.Commit(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to commit transaction")
	}

	return resp, nil
}

func (r *relationRepo) Delete(ctx context.Context, data *nb.RelationPrimaryKey) (err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.Delete")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(data.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to start transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `
		SELECT
    		r.id,
    		r.table_from,
    		r.table_to,
    		r.field_from,
    		r.field_to,
    		r.type,
    		r.relation_field_slug,
    		r.editable,
    		r.is_user_id_default,
    		r.is_system,
    		r.object_id_from_jwt,
    		COALESCE(r.cascading_tree_table_slug, '') AS cascading_tree_table_slug,
    		COALESCE(r.cascading_tree_field_slug, '') AS cascading_tree_field_slug
		FROM
		    relation r
		WHERE  r.id = $1`

	var (
		tableFromSlug, tableToSlug string
		columns                    []string
		isExists                   bool
	)

	relation := &nb.RelationForGetAll{}

	err = tx.QueryRow(ctx, query, data.Id).Scan(
		&relation.Id,
		&tableFromSlug,
		&tableToSlug,
		&relation.FieldFrom,
		&relation.FieldTo,
		&relation.Type,
		&relation.RelationFieldSlug,
		&relation.Editable,
		&relation.IsUserIdDefault,
		&relation.IsSystem,
		&relation.ObjectIdFromJwt,
		&relation.CascadingTreeTableSlug,
		&relation.CascadingTreeFieldSlug,
	)
	if err != nil {
		return errors.Wrap(err, "relation not found")
	}

	if relation.IsSystem {
		return errors.New("system relations cannot be deleted")
	}

	field, err := helper.FieldFindOne(ctx, models.RelationHelper{
		Tx:         tx,
		RelationID: data.Id,
	})
	if err != nil {
		return errors.Wrap(err, "failed to find field")
	}

	if field == nil {
		return errors.New("field not found")
	}

	switch relation.Type {
	case config.MANY2MANY:
		table, err := helper.TableFindOneTx(ctx, tx, tableToSlug)
		if err != nil {
			return errors.Wrap(err, "failed to find table")
		}

		err = helper.FieldFindOneDelete(ctx, models.RelationHelper{
			Tx:         tx,
			FieldName:  relation.FieldTo,
			TableID:    table.Id,
			RelationID: relation.Id,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete field")
		}

		table, err = helper.TableFindOneTx(ctx, tx, tableFromSlug)
		if err != nil {
			return errors.Wrap(err, "failed to find table")
		}

		err = helper.FieldFindOneDelete(ctx, models.RelationHelper{
			Tx:         tx,
			FieldName:  relation.FieldFrom,
			TableID:    table.Id,
			RelationID: relation.Id,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete field")
		}

		err = helper.RemoveFromLayout(ctx, models.RelationLayout{
			Tx:         tx,
			TableId:    table.Id,
			RelationId: relation.Id,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete from section")
		}
	case config.RECURSIVE:
		table, err := helper.TableFindOneTx(ctx, tx, tableFromSlug)
		if err != nil {
			return errors.Wrap(err, "failed to find table")
		}

		err = helper.FieldFindOneDelete(ctx, models.RelationHelper{
			Tx:         tx,
			FieldName:  relation.FieldTo,
			TableID:    table.Id,
			RelationID: relation.Id,
			TableSlug:  tableFromSlug,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete field")
		}

		err = helper.RemoveFromLayout(ctx, models.RelationLayout{
			Tx:         tx,
			TableId:    table.Id,
			RelationId: relation.Id,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete from section")
		}
	default:
		table, err := helper.TableFindOneTx(ctx, tx, tableFromSlug)
		if err != nil {
			return errors.Wrap(err, "failed to find table")
		}

		err = helper.FieldFindOneDelete(ctx, models.RelationHelper{
			Tx:         tx,
			FieldName:  relation.FieldFrom,
			TableID:    table.Id,
			RelationID: relation.Id,
			TableSlug:  tableFromSlug,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete field")
		}

		err = helper.RemoveFromLayout(ctx, models.RelationLayout{
			Tx:         tx,
			TableId:    table.Id,
			RelationId: relation.Id,
		})
		if err != nil {
			return errors.Wrap(err, "failed to delete from section")
		}
	}

	viewDeleteQuery := `DELETE FROM view WHERE relation_id = $1`
	_, err = tx.Exec(ctx, viewDeleteQuery, data.Id)
	if err != nil {
		return errors.Wrap(err, "failed to delete views")
	}

	existsColumnView, err := helper.ViewFindOneByTableSlug(ctx, models.RelationHelper{
		Tx:        tx,
		TableSlug: tableFromSlug,
	})
	if err != nil {
		return errors.Wrap(err, "failed to find column view")
	}

	if existsColumnView != nil && len(existsColumnView.Columns) > 0 {
		for _, id := range existsColumnView.Columns {
			if id == field.Id || id == field.RelationId {
				isExists = true
				continue
			} else if id != "" {
				columns = append(columns, id)
			}
		}

		if isExists {
			viewFindOneAndUpdate := `
				UPDATE view SET 
					columns = $1
				WHERE id = $2`
			_, err = tx.Exec(ctx, viewFindOneAndUpdate, columns, existsColumnView.Id)
			if err != nil {
				return errors.Wrap(err, "failed to update view")
			}
		}
	}

	query = `DELETE FROM relation WHERE id = $1`
	rows, err := tx.Exec(ctx, query, data.Id)
	if err != nil {
		return errors.Wrap(err, "failed to delete relation")
	}

	if rows.RowsAffected() == 0 {
		return errors.New("no rows affected")
	}

	err = helper.TabDeleteMany(ctx, models.RelationHelper{
		Tx:         tx,
		RelationID: data.Id,
	})
	if err != nil {
		return errors.Wrap(err, "failed to delete tabs")
	}

	err = helper.RemoveRelation(ctx, models.RelationHelper{
		Tx:           tx,
		TableFrom:    tableFromSlug,
		FieldName:    field.Slug,
		FieldFrom:    relation.FieldFrom,
		FieldTo:      relation.FieldTo,
		TableTo:      tableToSlug,
		RelationType: relation.Type,
	})
	if err != nil {
		return errors.Wrap(err, "remove relation")
	}

	err = tx.Commit(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to commit")
	}

	return nil
}

func (r *relationRepo) GetSingleViewForRelation(ctx context.Context, req models.ReqForViewRelation) (resp *nb.RelationForGetAll, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.GetSingleViewForRelation")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.ProjectId)
	if err != nil {
		return nil, err
	}

	var tableId string

	resp = &nb.RelationForGetAll{}
	table, err := helper.TableVer(ctx, models.TableVerReq{Conn: conn, Slug: req.TableSlug})
	if err != nil {
		return resp, err
	}
	if table != nil {
		cast.ToString(table["id"])
		table["id"] = tableId
	}

	fieldResp := &nb.Field{}

	query := `
    SELECT 
		r.id,
		r.table_from,
		r.table_to,
		r.field_from,
		r.field_to,
		r.type,
		r.view_fields,
		r.relation_field_slug,
		r.dynamic_tables,
		r.editable,
		r.is_user_id_default,
		r.cascadings,
		r.is_system,
		r.object_id_from_jwt,
		r.cascading_tree_table_slug,
		r.cascading_tree_field_slug,
		r.dynamic_tables,

        f.table_id AS table_id ,
        f.required AS required ,
        f.slug AS slug ,
        f.label AS label ,
        f.default AS default ,
        f.type AS type ,
        f.index AS index ,
        f.is_visible AS is_visible ,
        f.is_search AS is_search ,
        f.autofill_field AS autofill_field ,
        f.autofill_table AS autofill_table ,
        f.relation_id AS relation_id ,
        f.unique AS unique ,
        f.automatic AS automatic ,
        f.enable_multilanguage AS enable_multilanguage ,
		f.attributes AS attributes
    FROM 
        relation r
    LEFT JOIN 
        field f ON f.relation_id = r.id
    WHERE 
        r.id = $1
    ORDER BY 
        r.created_at DESC
    LIMIT 1
`

	rows, err := conn.Query(ctx, query, req.Id)
	if err != nil {
		return resp, err
	}
	defer rows.Close()

	var attributes []byte

	var (
		defaultNull         sql.NullString
		index               sql.NullString
		AutofillField       sql.NullString
		AutofillTable       sql.NullString
		RelationId          sql.NullString
		Unique              sql.NullBool
		Automatic           sql.NullBool
		EnableMultilanguage sql.NullBool
		typeNull            sql.NullString
		view_fields         sql.NullString
		relation_field_slug sql.NullString
		editable            sql.NullBool
		field_from          sql.NullString
		field_to            sql.NullString

		cascading_tree_field_slug sql.NullString
		cascading_tree_table_slug sql.NullString
		object_id_from_jwt        sql.NullBool
	)
	for rows.Next() {
		if err := rows.Scan(
			&resp.Id,
			&resp.TableFrom,
			&resp.TableTo,
			&typeNull,
			&view_fields,
			&relation_field_slug,
			&editable,

			&fieldResp.TableId,
			&fieldResp.Required,
			&fieldResp.Slug,
			&fieldResp.Label,
			&defaultNull,
			&fieldResp.Type,
			&index,
			&fieldResp.IsVisible,
			&fieldResp.IsSearch,
			&AutofillField,
			&AutofillTable,
			&RelationId,
			&Unique,
			&Automatic,
			&EnableMultilanguage,
			&attributes,
		); err != nil {
			return resp, err
		}

		err = json.Unmarshal(attributes, &fieldResp.Attributes)
		if err != nil {
			return resp, err
		}

		if field_from.Valid {
			resp.FieldFrom = field_from.String
		}
		if field_to.Valid {
			resp.FieldTo = field_to.String
		}

		if typeNull.Valid {
			resp.Type = typeNull.String
		}

		if relation_field_slug.Valid {
			resp.RelationFieldSlug = relation_field_slug.String
		}
		if editable.Valid {
			resp.Editable = editable.Bool
		}

		if view_fields.Valid {
			err = json.Unmarshal([]byte(view_fields.String), &resp.ViewFields)
			if err != nil {
				return resp, err
			}
		}

		if cascading_tree_field_slug.Valid {
			resp.CascadingTreeFieldSlug = cascading_tree_field_slug.String
		}
		if cascading_tree_table_slug.Valid {
			resp.CascadingTreeTableSlug = cascading_tree_table_slug.String
		}
		if object_id_from_jwt.Valid {
			resp.ObjectIdFromJwt = object_id_from_jwt.Bool
		}

	}

	if resp.Id == "" {
		return resp, errors.New("no data found")
	}
	if err := rows.Err(); err != nil {
		return resp, err
	}

	tableFrom, err := helper.TableVer(ctx, models.TableVerReq{
		Conn: conn,
		Slug: req.TableSlug,
	})
	if err != nil {
		return resp, err
	}

	if resp.Type == config.MANY2DYNAMIC {
		for _, dynamicTable := range resp.DynamicTables {

			if dynamicTable.TableSlug == req.TableSlug || cast.ToString(table["slug"]) == req.TableSlug {
				tableTo, err := helper.TableVer(ctx, models.TableVerReq{
					Conn: conn,
					Slug: dynamicTable.TableSlug,
				})
				if err != nil {
					return resp, err
				}
				view := &nb.View{}
				err = conn.QueryRow(ctx, `
					SELECT * FROM view
					WHERE relation_id = $1
				`, req.Id).Scan(
					&view.RelationId,
					&view.TableSlug,
				)
				if err != nil {
					return resp, err
				}

				viewFieldsInDynamicTable := []nb.Field{}
				for _, fieldID := range dynamicTable.ViewFields {

					query := `SELECT id, slug, table_id , attributes,  FROM field WHERE id = $1`
					view_field := &nb.Field{}
					err = conn.QueryRow(ctx, query, fieldID).Scan(
						&view_field.Id,
						&view_field.Slug,
						&view_field.TableId,
						&view_field.Attributes,
					)
					if err != nil {
						return resp, err
					}

					viewFieldsInDynamicTable = append(viewFieldsInDynamicTable, nb.Field{
						Id:         view_field.Id,
						Slug:       view_field.Slug,
						TableId:    view_field.TableId,
						Attributes: view_field.Attributes,
					})

					responseRelation := map[string]any{
						"id":                        resp.Id,
						"table_from":                tableFrom,
						"table_to":                  tableTo,
						"type":                      resp.Type,
						"view_fields":               viewFieldsInDynamicTable,
						"editable":                  resp.Editable,
						"dynamic_tables":            resp.DynamicTables,
						"relation_field_slug":       resp.RelationFieldSlug,
						"auto_filters":              resp.AutoFilters,
						"is_user_id_default":        resp.IsUserIdDefault,
						"cascadings":                resp.Cascadings,
						"object_id_from_jwt":        resp.ObjectIdFromJwt,
						"cascading_tree_table_slug": resp.CascadingTreeTableSlug,
						"cascading_tree_field_slug": resp.CascadingTreeFieldSlug,
					}

					responseRelation["title"] = view.Name
					responseRelation["columns"] = view.Columns
					responseRelation["quick_filters"] = view.QuickFilters
					responseRelation["group_fields"] = view.GroupFields
					responseRelation["is_editable"] = view.IsEditable
					responseRelation["relation_table_slug"] = view.RelationTableSlug
					responseRelation["view_type"] = view.Type
					responseRelation["summaries"] = view.Summaries
					responseRelation["relation_id"] = view.RelationId
					responseRelation["default_values"] = view.DefaultValues
					responseRelation["action_relations"] = view.ActionRelations
					responseRelation["default_limit"] = view.DefaultLimit
					responseRelation["multiple_insert"] = view.MultipleInsert
					responseRelation["updated_fields"] = view.UpdatedFields
					responseRelation["attributes"] = view.Attributes
				}
			}
		}
	}

	tableTo, err := helper.TableVer(ctx, models.TableVerReq{
		Conn: conn,
		Slug: req.TableSlug,
	})
	if err != nil {
		return resp, err
	}

	query = `
		SELECT relation_id, table_slug, creatable FROM view
		WHERE relation_id = $1
	`
	view := &nb.View{}
	err = conn.QueryRow(ctx, query, req.Id).Scan(
		&view.RelationId,
		&view.TableSlug,
		&view.Creatable,
	)
	if err != nil {
		return resp, err
	}
	responseRelation := map[string]any{
		"id":                        resp.Id,
		"table_from":                tableFrom,
		"table_to":                  tableTo,
		"type":                      resp.Type,
		"view_fields":               resp.ViewFields,
		"editable":                  resp.Editable,
		"dynamic_tables":            resp.DynamicTables,
		"relation_field_slug":       resp.RelationFieldSlug,
		"auto_filters":              resp.AutoFilters,
		"is_user_id_default":        resp.IsUserIdDefault,
		"cascadings":                resp.Cascadings,
		"object_id_from_jwt":        resp.ObjectIdFromJwt,
		"cascading_tree_table_slug": resp.CascadingTreeTableSlug,
		"cascading_tree_field_slug": resp.CascadingTreeFieldSlug,
	}
	responseRelation["title"] = view.Name
	responseRelation["columns"] = view.Columns
	responseRelation["quick_filters"] = view.QuickFilters
	responseRelation["group_fields"] = view.GroupFields
	responseRelation["is_editable"] = view.IsEditable
	responseRelation["relation_table_slug"] = view.RelationTableSlug
	responseRelation["view_type"] = view.Type
	responseRelation["summaries"] = view.Summaries
	responseRelation["relation_id"] = view.RelationId
	responseRelation["default_values"] = view.DefaultValues
	responseRelation["action_relations"] = view.ActionRelations
	responseRelation["default_limit"] = view.DefaultLimit
	responseRelation["multiple_insert"] = view.MultipleInsert
	responseRelation["updated_fields"] = view.UpdatedFields
	responseRelation["attributes"] = view.Attributes
	responseRelation["creatable"] = view.Creatable

	resp.Creatable = view.Creatable
	return resp, nil
}

func (r *relationRepo) GetIds(ctx context.Context, req *nb.GetIdsReq) (resp *nb.GetIdsResp, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.GetIds")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.ProjectId)
	if err != nil {
		return nil, err
	}

	resp = &nb.GetIdsResp{
		Ids: make([]string, 0),
	}

	query := `
		SELECT id FROM relation WHERE table_from = $1 AND table_to = $2
	`

	rows, err := conn.Query(ctx, query, req.TableFrom, req.TableTo)
	if err != nil {
		return nil, errors.Wrap(err, "error when query")
	}
	defer rows.Close()

	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, errors.Wrap(err, "error when scan")
		}
		resp.Ids = append(resp.Ids, id)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error when rows")
	}

	return resp, nil
}

// GetRelationsByTableFrom retrieves relations by table_from using pure SQL
func (r *relationRepo) GetRelationsByTableFrom(ctx context.Context, projectID string, tableFrom string) ([]*nb.CreateRelationRequest, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "relation.GetRelationsByTableFrom")
	defer dbSpan.Finish()

	if projectID == "" {
		return nil, errors.New("project_id is required")
	}
	if tableFrom == "" {
		return nil, errors.New("table_from is required")
	}

	conn, err := psqlpool.Get(projectID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting connection from pool")
	}

	query := `SELECT 
		id,
		table_from,
		table_to,
		field_from,
		field_to,
		type,
		view_fields,
		relation_field_slug,
		editable,
		is_user_id_default,
		cascadings,
		is_system,
		object_id_from_jwt,
		cascading_tree_table_slug,
		cascading_tree_field_slug,
		dynamic_tables,
		auto_filters
	FROM relation
	WHERE table_from = $1`

	rows, err := conn.Query(ctx, query, tableFrom)
	if err != nil {
		return nil, errors.Wrap(err, "error executing query")
	}
	defer rows.Close()

	var relations []*nb.CreateRelationRequest
	for rows.Next() {
		var (
			id, tableFrom, tableTo, fieldFrom, fieldTo, relationType, relationFieldSlug string
			cascadingTreeTableSlug, cascadingTreeFieldSlug                              sql.NullString
			editable, isUserIdDefault, isSystem, objectIdFromJwt                        bool
			cascadings, dynamicTables, autoFilters                                      []byte
			viewFields                                                                  []string
		)

		err := rows.Scan(
			&id,
			&tableFrom,
			&tableTo,
			&fieldFrom,
			&fieldTo,
			&relationType,
			&viewFields,
			&relationFieldSlug,
			&editable,
			&isUserIdDefault,
			&cascadings,
			&isSystem,
			&objectIdFromJwt,
			&cascadingTreeTableSlug,
			&cascadingTreeFieldSlug,
			&dynamicTables,
			&autoFilters,
		)
		if err != nil {
			return nil, errors.Wrap(err, "error scanning row")
		}

		// Parse auto filters
		var autoFiltersArray []*nb.AutoFilter
		if len(autoFilters) > 0 {
			if err := json.Unmarshal(autoFilters, &autoFiltersArray); err != nil {
				return nil, errors.Wrap(err, "error unmarshaling auto filters")
			}
		}

		// cascadings / dynamic_tables were read from the row but previously dropped,
		// so cascading relations and dynamic tables were silently lost on any reader
		// of this method (including the template copier). Parse them the same
		// way as auto_filters and carry them through.
		//
		// These columns are sometimes persisted as a JSON object ("{}") instead of
		// an array, so unmarshaling into a slice fails. Older readers dropped these
		// fields entirely, so tolerate any non-array shape by treating it as empty
		// rather than failing the whole read — a strict error here aborts the
		// template copier mid-clone and leaves the child project empty.
		var cascadingsArray []*nb.Cascading
		if len(cascadings) > 0 {
			if err := json.Unmarshal(cascadings, &cascadingsArray); err != nil {
				log.Printf("relation %s: ignoring malformed cascadings: %v", id, err)
				cascadingsArray = nil
			}
		}

		var dynamicTablesArray []*nb.DynamicTable
		if len(dynamicTables) > 0 {
			if err := json.Unmarshal(dynamicTables, &dynamicTablesArray); err != nil {
				log.Printf("relation %s: ignoring malformed dynamic_tables: %v", id, err)
				dynamicTablesArray = nil
			}
		}

		relation := &nb.CreateRelationRequest{
			Id:                     id,
			TableFrom:              tableFrom,
			TableTo:                tableTo,
			Type:                   relationType,
			ViewFields:             viewFields,
			FieldFrom:              fieldFrom,
			FieldTo:                fieldTo,
			Editable:               editable,
			IsUserIdDefault:        isUserIdDefault,
			ObjectIdFromJwt:        objectIdFromJwt,
			RelationFieldSlug:      relationFieldSlug,
			AutoFilters:            autoFiltersArray,
			Cascadings:             cascadingsArray,
			DynamicTables:          dynamicTablesArray,
			CascadingTreeTableSlug: cascadingTreeTableSlug.String,
			CascadingTreeFieldSlug: cascadingTreeFieldSlug.String,
		}

		relations = append(relations, relation)
	}

	if err = rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating rows")
	}

	return relations, nil
}

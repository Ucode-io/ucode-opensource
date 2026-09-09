package helper

import (
	"context"
	"database/sql"
	"strings"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"

	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

func GetAutomaticFilter(ctx context.Context, req models.GetAutomaticFilterRequest) (map[string]any, error) {
	var (
		many2ManyRelation    bool
		automaticFilterQuery string
		filter               = make(map[string]any)
	)

	automaticFilterQuery = `
	SELECT
		table_slug,
		custom_field,
		object_field,
		not_use_in_tab
	FROM automatic_filter
	WHERE method = 'read' AND role_id = $1 AND table_slug = $2 AND deleted_at IS NULL`

	rows, err := req.Conn.Query(ctx, automaticFilterQuery, req.RoleIdFromToken, req.TableSlug)
	if err != nil {
		return req.Params, errors.Wrap(err, "when get automaticFilter rows")
	}
	defer rows.Close()
	for rows.Next() {
		var (
			autofilter nb.RoleWithAppTablePermissions_Table_AutomaticFilter
		)

		if err := rows.Scan(
			&autofilter.TableSlug,
			&autofilter.CustomField,
			&autofilter.ObjectField,
			&autofilter.NotUseInTab,
		); err != nil {
			return req.Params, errors.Wrap(err, "when scan automaticFilter rows")
		}

		if len(autofilter.TableSlug) != 0 && !autofilter.NotUseInTab {
			if strings.Contains(autofilter.ObjectField, "#") {
				var (
					splitedElement     = strings.Split(autofilter.ObjectField, "#")
					reltype            sql.NullString
					fieldFrom, fieldTo sql.NullString
				)
				autofilter.ObjectField = splitedElement[0]

				relquery := `SELECT type, field_from, field_to FROM "relation" WHERE id = $1`
				if err := req.Conn.QueryRow(ctx, relquery, splitedElement[1]).Scan(&reltype, &fieldFrom, &fieldTo); err != nil {
					return req.Params, errors.Wrap(err, "when get automaticFilter relation")

				}

				switch reltype.String {
				case "Many2One":
					autofilter.CustomField = fieldFrom.String
				}
			}
			if autofilter.CustomField == "user_id" {
				if autofilter.ObjectField != req.TableSlug {
					if !many2ManyRelation {
						filter[autofilter.ObjectField+"_id"] = req.Params["user_id_from_token"]
					} else {
						filter[autofilter.ObjectField+"ids"] = req.Params["user_id_from_token"]
					}
				}
			} else {
				if len(autofilter.CustomField) >= 3 {
					var connectionTableSlug = autofilter.CustomField[:len(autofilter.CustomField)-3]
					var objFromAuth = FindOneTableFromParams(cast.ToSlice(req.Params["tables"]), autofilter.ObjectField)
					if objFromAuth != nil {
						if connectionTableSlug != req.TableSlug {
							if !many2ManyRelation {
								filter[autofilter.CustomField] = objFromAuth["object_id"]
							}
						}
					} else {
						filter[autofilter.CustomField] = req.UserIdFromToken
					}
				}
			}
		}
	}

	req.Params["auto_filter"] = filter

	return req.Params, nil
}

package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/util"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	pa "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/auth_service"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/grpc/client"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/Masterminds/squirrel"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/lib/pq"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	excel "github.com/xuri/excelize/v2"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
)

func convertToTitle(columnNumber int) string {
	columnNumber += 1
	title := ""
	for columnNumber > 0 {
		columnNumber--
		title = string('A'+byte(columnNumber%26)) + title
		columnNumber /= 26
	}

	return title
}

func formatExcelDateValue(value any) (string, error) {
	dateValue := strings.TrimSpace(cast.ToString(value))
	if dateValue == "" {
		return "", nil
	}

	timeF, err := time.Parse("2006-01-02", strings.Split(dateValue, " ")[0])
	if err != nil {
		return "", err
	}

	return timeF.Format("02.01.2006"), nil
}

type objectBuilderRepo struct {
	logger     logger.LoggerI
	grpcClient client.ServiceManagerI
}

func NewObjectBuilder(logger logger.LoggerI, grpcClient client.ServiceManagerI) storage.ObjectBuilderRepoI {
	return &objectBuilderRepo{
		logger:     logger,
		grpcClient: grpcClient,
	}
}

func (o *objectBuilderRepo) GetList(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetList")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.TableSlug == "client_type" {
		query := `
			SELECT
				"guid",
				"project_id",
				"name",
				"self_register",
				"self_recover",
				"client_platform_ids",
				"confirm_by",
				"is_system"
			FROM client_type
		`

		rows, err := conn.Query(ctx, query)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while getting client types")
		}
		defer rows.Close()

		data := make([]models.ClientType, 0)
		for rows.Next() {
			var clientType models.ClientType

			err = rows.Scan(
				&clientType.Guid,
				&clientType.ProjectId,
				&clientType.Name,
				&clientType.SelfRegister,
				&clientType.SelfRecover,
				&clientType.ClientPlatformIds,
				&clientType.ConfirmBy,
				&clientType.IsSystem,
			)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning client types")
			}

			data = append(data, clientType)
		}

		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while marshalling client types")
		}

		var dataStruct structpb.Struct
		jsonBytes = []byte(fmt.Sprintf(`{"response": %s}`, jsonBytes))

		err = json.Unmarshal(jsonBytes, &dataStruct)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling client types")
		}

		return &nb.CommonMessage{
			TableSlug:     req.TableSlug,
			ProjectId:     req.ProjectId,
			Data:          &dataStruct,
			VersionId:     req.VersionId,
			CustomMessage: req.CustomMessage,
			IsCached:      req.IsCached,
		}, err
	} else {
		query := `
			SELECT
				"guid",
				"project_id",
				"name",
				"client_platform_id",
				"client_type_id",
				"is_system"
			FROM "role"
		`

		rows, err := conn.Query(ctx, query)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while getting roles")
		}
		defer rows.Close()

		data := make([]models.Role, 0)
		for rows.Next() {
			var role models.Role

			err = rows.Scan(
				&role.Guid,
				&role.ProjectId,
				&role.Name,
				&role.ClientPlatformId,
				&role.ClientTypeId,
				&role.IsSystem,
			)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning roles")
			}

			data = append(data, role)
		}

		jsonBytes, err := json.Marshal(data)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while marshalling roles")
		}

		var dataStruct structpb.Struct
		jsonBytes = []byte(fmt.Sprintf(`{"response": %s}`, jsonBytes))

		err = json.Unmarshal(jsonBytes, &dataStruct)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling roles")
		}

		return &nb.CommonMessage{
			TableSlug:     req.TableSlug,
			ProjectId:     req.ProjectId,
			Data:          &dataStruct,
			VersionId:     req.VersionId,
			CustomMessage: req.CustomMessage,
			IsCached:      req.IsCached,
		}, err
	}
}

func (o *objectBuilderRepo) GetListConnection(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetListConnection")
	defer dbSpan.Finish()
	var (
		clientTypeId = cast.ToString(req.Data.AsMap()["client_type_id_from_token"])
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT
			"guid",
			"table_slug",
			"view_slug",
			"view_label",
			"name",
			"type",
			"icon",
			"main_table_slug",
			"field_slug",
			"client_type_id"
		FROM "connections" WHERE deleted_at IS NULL AND client_type_id = $1
	`

	rows, err := conn.Query(ctx, query, clientTypeId)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer rows.Close()

	data := make([]models.Connection, 0)
	for rows.Next() {
		var (
			guid          sql.NullString
			tableSlug     sql.NullString
			viewSlug      sql.NullString
			viewLabel     sql.NullString
			name          sql.NullString
			ftype         sql.NullString
			icon          sql.NullString
			mainTableSlug sql.NullString
			fieldSlug     sql.NullString
			clientTypeId  sql.NullString
		)
		err = rows.Scan(
			&guid,
			&tableSlug,
			&viewSlug,
			&viewLabel,
			&name,
			&ftype,
			&icon,
			&mainTableSlug,
			&fieldSlug,
			&clientTypeId,
		)
		if err != nil {
			return &nb.CommonMessage{}, err
		}

		data = append(data, models.Connection{
			Guid:          guid.String,
			TableSlug:     tableSlug.String,
			ViewSlug:      viewSlug.String,
			ViewLabel:     viewLabel.String,
			Name:          name.String,
			Type:          ftype.String,
			Icon:          icon.String,
			MainTableSlug: mainTableSlug.String,
			FieldSlug:     fieldSlug.String,
			ClientTypeId:  clientTypeId.String,
		})
	}

	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	var dataStruct structpb.Struct
	jsonBytes = []byte(fmt.Sprintf(`{"response": %s}`, jsonBytes))

	err = json.Unmarshal(jsonBytes, &dataStruct)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	return &nb.CommonMessage{
		TableSlug:     req.TableSlug,
		ProjectId:     req.ProjectId,
		Data:          &dataStruct,
		VersionId:     req.VersionId,
		CustomMessage: req.CustomMessage,
		IsCached:      req.IsCached,
	}, err
}

func (o *objectBuilderRepo) GetTableDetails(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetTableDetails")
	defer dbSpan.Finish()

	var (
		fields, relationsFields, decodedFields []models.Field
		views                                  = []models.View{}
		fieldsAutofillMap                      = make(map[string]models.AutofillField)
		params                                 = make(map[string]any)
		table                                  = &nb.Table{}
		tAttributes                            = []byte{} // Default empty JSON object
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error marshalling request data")
	}
	if err := json.Unmarshal(body, &params); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error unmarshalling request data")
	}

	query := `SELECT 
		f.id,
		f."table_id",
		f."required",
		f."slug",
		f."label",
		f."default",
		f."type",
		f."index",
		f."attributes",
		f."is_visible",
		f.autofill_field,
		f.autofill_table,
		f."unique",
		f."automatic",
		f.relation_id,
		f."is_search",
		f.enable_multilanguage
	FROM "field" as f 
	JOIN "table" as t ON f.table_id = t.id 
	WHERE t.slug = $1`

	rows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting fields by table slug")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			field                            = models.Field{}
			attributes                       = []byte{}
			relationIdNull, autofillField    sql.NullString
			defaultStr, index, autofillTable sql.NullString
			atr                              = make(map[string]any)
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
			&autofillField,
			&autofillTable,
			&field.Unique,
			&field.Automatic,
			&relationIdNull,
			&field.IsSearch,
			&field.EnableMultilanguage,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning fields")
		}

		field.RelationId = relationIdNull.String
		field.AutofillField = autofillField.String
		field.AutofillTable = autofillTable.String
		field.Default = defaultStr.String
		field.Index = index.String
		newAtrb := make(map[string]any)

		if len(field.AutofillField) != 0 && len(field.AutofillTable) > 1 {
			var relationFieldSlug = strings.Split(autofillTable.String, "#")[1]

			fieldsAutofillMap[relationFieldSlug] = models.AutofillField{
				FieldFrom: autofillField.String,
				FieldTo:   field.Slug,
				TableSlug: req.TableSlug,
				Automatic: field.Automatic,
			}
		}

		if err := json.Unmarshal(attributes, &atr); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
		}

		if field.Type == "LOOKUP" || field.Type == "LOOKUPS" {
			view, err := helper.ViewFindOne(ctx, models.RelationHelper{
				Conn:       conn,
				RelationID: field.RelationId,
			})
			if err != nil {
				return resp, errors.Wrap(err, "error while getting view by relation id")
			}

			if view == nil {
				continue
			}

			newAtrb, err = helper.ConvertStructToMap(view.Attributes)
			if err != nil {
				return resp, errors.Wrap(err, "error while converting struct to map")
			}

			query := `
				SELECT
					"view_fields",
					"table_from",
					"table_to",
					"object_id_from_jwt",
					"is_user_id_default",
					COALESCE(r."auto_filters", '[{}]') AS "auto_filters"
				FROM "relation" r
				WHERE id = $1
			`

			relationRows, err := conn.Query(ctx, query, field.RelationId)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "error while getting relation by id")
			}
			defer relationRows.Close()

			for relationRows.Next() {
				var (
					viewFields                       []string
					tableFrom, tableTo               string
					objectIdFromJWT, isUserIdDefault bool
					fieldObjects                     []models.Field
					autoFilterByte                   []byte
					autoFilters                      []map[string]any
				)

				err = relationRows.Scan(
					&viewFields,
					&tableFrom,
					&tableTo,
					&objectIdFromJWT,
					&isUserIdDefault,
					&autoFilterByte,
				)
				if err != nil {
					return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning relation rows")
				}

				if err = json.Unmarshal(autoFilterByte, &autoFilters); err != nil {
					return nil, errors.Wrap(err, "error unmarshal")
				}

				atr["relation_data"] = map[string]any{
					"view_fields": viewFields,
				}
				atr["auto_filters"] = autoFilters
				atr["object_id_from_jwt"] = objectIdFromJWT
				atr["is_user_id_default"] = isUserIdDefault

				if tableFrom != req.TableSlug {
					field.TableSlug = tableFrom
				} else {
					field.TableSlug = tableTo
				}

				for _, id := range viewFields {
					var field models.Field

					query = `
						SELECT 
							f.id,
							f."table_id",
							f."required",
							f."slug",
							f."label",
							f."default",
							f."type",
							f."index",
							f."attributes",
							f."is_visible",
							f.autofill_field,
							f.autofill_table,
							f."unique",
							f."automatic",
							f.relation_id,
							f."is_search",
							f.enable_multilanguage
						FROM "field" as f 
						WHERE f.id = $1
					`

					err = conn.QueryRow(ctx, query, id).Scan(
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
						&autofillField,
						&autofillTable,
						&field.Unique,
						&field.Automatic,
						&relationIdNull,
						&field.IsSearch,
						&field.EnableMultilanguage,
					)
					if err != nil {
						return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning fields")
					}

					field.RelationId = relationIdNull.String
					field.AutofillField = autofillField.String
					field.AutofillTable = autofillTable.String
					field.Default = defaultStr.String
					field.Index = index.String

					if err := json.Unmarshal(attributes, &field.Attributes); err != nil {
						return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
					}

					fieldObjects = append(fieldObjects, field)
				}

				field.ViewFields = fieldObjects
			}
		}

		maps.Copy(atr, newAtrb)

		attributes, _ = json.Marshal(atr)

		if err := json.Unmarshal(attributes, &field.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
		}

		fields = append(fields, field)
	}

	query = `
		SELECT
			id,
			slug,
			label,
			soft_delete,
			is_cached,
			is_login_table,
			attributes,
			order_by
		FROM "table" 
		WHERE slug = $1 AND deleted_at IS NULL`
	if err = conn.QueryRow(ctx, query, req.TableSlug).Scan(
		&table.Id,
		&table.Slug,
		&table.Label,
		&table.SoftDelete,
		&table.IsCached,
		&table.IsLoginTable,
		&tAttributes,
		&table.OrderBy,
	); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting table details")
	}

	var attrDataStruct *structpb.Struct
	if err := json.Unmarshal(tAttributes, &attrDataStruct); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "unmarchal structpb table get by id")
	}

	table.Attributes = attrDataStruct

	viewFilterField := ` table_slug `
	viewFilterValue := req.TableSlug
	if cast.ToBool(params["main_table"]) {
		viewFilterField = ` menu_id `
		viewFilterValue = cast.ToString(params["menu_id"])
	}

	query = fmt.Sprintf(`SELECT 
		"id",
		"attributes",
		"table_slug",
		"type",
		"columns",
		"order",
		COALESCE("time_interval", 0),
		COALESCE("group_fields"::varchar[], '{}'),
		"name",
		"quick_filters",
		"users",
		"view_fields",
		"calendar_from_slug",
		"calendar_to_slug",
		"multiple_insert",
		"status_field_slug",
		"is_editable",
		"relation_table_slug",
		"relation_id",
		"updated_fields",
		"table_label",
		"default_limit",
		"default_editable",
		"name_uz",
		"name_en",
		is_relation_view
	FROM "view" WHERE %s = $1 ORDER BY "order" ASC`, viewFilterField)

	viewRows, err := conn.Query(ctx, query, viewFilterValue)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting views by table slug")
	}
	defer viewRows.Close()

	for viewRows.Next() {
		var (
			attributes                               []byte
			view                                     = models.View{}
			Name, CalendarFromSlug                   sql.NullString
			RelationTableSlug, StatusFieldSlug       sql.NullString
			RelationId                               sql.NullString
			TableLabel, DefaultLimit, CalendarToSlug sql.NullString
			NameUz, NameEn, QuickFilters             sql.NullString
		)

		err := viewRows.Scan(
			&view.Id,
			&attributes,
			&view.TableSlug,
			&view.Type,
			&view.Columns,
			&view.Order,
			&view.TimeInterval,
			&view.GroupFields,
			&Name,
			&QuickFilters,
			&view.Users,
			&view.ViewFields,
			&CalendarFromSlug,
			&CalendarToSlug,
			&view.MultipleInsert,
			&StatusFieldSlug,
			&view.IsEditable,
			&RelationTableSlug,
			&RelationId,
			&view.UpdatedFields,
			&TableLabel,
			&DefaultLimit,
			&view.DefaultEditable,
			&NameUz,
			&NameEn,
			&view.IsRelationView,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning views")
		}

		view.Name = Name.String
		view.CalendarFromSlug = CalendarFromSlug.String
		view.CalendarToSlug = CalendarToSlug.String
		view.StatusFieldSlug = StatusFieldSlug.String
		view.RelationTableSlug = RelationTableSlug.String
		view.RelationId = RelationId.String
		view.TableLabel = TableLabel.String
		view.DefaultLimit = DefaultLimit.String
		view.NameUz = NameUz.String
		view.NameEn = NameEn.String

		if QuickFilters.Valid {
			err = json.Unmarshal([]byte(QuickFilters.String), &view.QuickFilters)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling quick filters")
			}
		}

		if view.Columns == nil {
			view.Columns = []string{}
		}

		if err := json.Unmarshal(attributes, &view.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling view attributes")
		}

		views = append(views, view)
	}

	if util.IsValidUUID(cast.ToString(params["role_id_from_token"])) {
		query = `SELECT 
		"guid",
		"role_id",
		"view_id",
		"view",
		"edit",
		"delete"
	FROM "view_permission" WHERE "view_id" = $1 AND "role_id" = $2`

		for _, view := range views {
			vp := models.ViewPermission{}

			err = conn.QueryRow(ctx, query, view.Id, cast.ToString(params["role_id_from_token"])).Scan(
				&vp.Guid,
				&vp.RoleId,
				&vp.ViewId,
				&vp.View,
				&vp.Edit,
				&vp.Delete,
			)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning view permissions")
			}

			view.Attributes["view_permission"] = vp
		}

		fields, _, err = helper.AddPermissionToField1(ctx,
			models.AddPermissionToFieldRequest{
				Conn:      conn,
				Fields:    fields,
				RoleId:    cast.ToString(params["role_id_from_token"]),
				TableSlug: req.TableSlug,
			},
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while adding permissions to fields")
		}
	}

	for _, element := range fields {
		if element.Attributes != nil && !(element.Type == "LOOKUP" || element.Type == "LOOKUPS" || element.Type == "DYNAMIC") {
			decodedFields = append(decodedFields, element)
		} else {
			elementField := element

			atrb, err := helper.ConvertStructToMap(element.Attributes)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "error while converting struct to map")
			}

			tempViewFields := cast.ToSlice(atrb["view_fields"])

			if len(tempViewFields) > 0 {
				body, err := json.Marshal(tempViewFields)
				if err != nil {
					return &nb.CommonMessage{}, errors.Wrap(err, "error while marshalling view fields")
				}
				if err := json.Unmarshal(body, &elementField.ViewFields); err != nil {
					return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling view fields")
				}
			}

			atrb, err = helper.ConvertStructToMap(elementField.Attributes)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "convert struct to map")
			}

			atrb["enable_multilanguage"] = elementField.EnableMultilanguage
			atrb["enable_multi_language"] = elementField.EnableMultilanguage

			if v, ok := fieldsAutofillMap[elementField.Slug]; ok {
				atrb["autofill"] = []any{v}
			}

			strc, err := helper.ConvertMapToStruct(atrb)
			if err != nil {
				return &nb.CommonMessage{}, errors.Wrap(err, "convert map to struct")
			}

			elementField.Attributes = strc

			decodedFields = append(decodedFields, elementField)
		}
	}

	repsonse := map[string]any{
		"fields":          decodedFields,
		"views":           views,
		"relation_fields": relationsFields,
		"table_info":      table,
	}

	newResp, err := helper.ConvertMapToStruct(repsonse)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while converting map to struct")
	}

	return &nb.CommonMessage{
		TableSlug: req.TableSlug,
		Data:      newResp,
	}, nil
}

func (o *objectBuilderRepo) GetAll(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetAll")
	defer dbSpan.Finish()

	var (
		params                       = make(map[string]any)
		views                        = []models.View{}
		fieldsMap                    = make(map[string]models.Field)
		fields, decodedFields        []models.Field
		relationsMap                 = make(map[string]models.RelationBody)
		fieldsM                      = make(map[string]any)
		searchFields                 []string
		count, counter, argCount     = 0, 0, 1
		filter, limit, offset        = " WHERE deleted_at IS NULL ", " LIMIT 20 ", " OFFSET 0"
		args, result                 []any
		order                        = " ORDER BY a.created_at DESC "
		autoFilters, searchCondition string
		getQuery                     = `SELECT jsonb_build_object( `
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while marshalling request data")
	}
	if err := json.Unmarshal(paramBody, &params); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling request data")
	}

	roleIdFromToken := cast.ToString(params["role_id_from_token"])
	userIdFromToken := cast.ToString(params["user_id_from_token"])

	query := `
		SELECT 
			f.id,
			f."table_id",
			t.slug,
			f."required",
			f."slug",
			f."label",
			f."default",
			f."type",
			f."index",
			f."attributes",
			f."is_visible",
			f.autofill_field,
			f.autofill_table,
			f."unique",
			f."automatic",
			f.relation_id,
			f.is_search
		FROM "field" as f 
		JOIN "table" as t ON t."id" = f."table_id"
		LEFT JOIN "relation" r ON r.id = f.relation_id
		WHERE t."slug" = $1
	`

	rows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting fields by table slug")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			field                            = models.Field{}
			attributes                       = []byte{}
			relationIdNull, autofillField    sql.NullString
			defaultStr, index, autofillTable sql.NullString
			atrb                             = make(map[string]any)
			isSearch                         bool
		)

		err = rows.Scan(
			&field.Id,
			&field.TableId,
			&field.TableSlug,
			&field.Required,
			&field.Slug,
			&field.Label,
			&defaultStr,
			&field.Type,
			&index,
			&attributes,
			&field.IsVisible,
			&autofillField,
			&autofillTable,
			&field.Unique,
			&field.Automatic,
			&relationIdNull,
			&isSearch,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning fields")
		}

		field.RelationId = relationIdNull.String
		field.AutofillField = autofillField.String
		field.AutofillTable = autofillTable.String
		field.Default = defaultStr.String
		field.Index = index.String

		if err := json.Unmarshal(attributes, &atrb); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
		}

		attributes, _ = json.Marshal(atrb)

		if err := json.Unmarshal(attributes, &field.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
		}

		if field.Type == "DATE_TIME_WITHOUT_TIME_ZONE" {
			getQuery += fmt.Sprintf(`'%s', TO_CHAR(a.%s, 'DD.MM.YYYY HH24:MI'),`, field.Slug, field.Slug)
			fieldsM[field.Slug] = field.Type
			continue
		}

		if counter >= 30 {
			getQuery = strings.TrimRight(getQuery, ",")
			getQuery += `) || jsonb_build_object( `
			counter = 0
		}
		getQuery += fmt.Sprintf(`'%s', a.%s,`, field.Slug, field.Slug)
		fieldsM[field.Slug] = field.Type

		if helper.FIELD_TYPES[field.Type] == "VARCHAR" && isSearch {
			searchFields = append(searchFields, field.Slug)
		}

		counter++

		fields = append(fields, field)
		fieldsMap[field.Slug] = field
	}

	fieldsWithPermissions, _, err := helper.AddPermissionToField1(ctx, models.AddPermissionToFieldRequest{Conn: conn, RoleId: roleIdFromToken, TableSlug: req.TableSlug, Fields: fields})
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while adding permissions to fields")
	}

	rquery := `SELECT 
			f.id,
			f."table_id",
			t.slug,
			f."required",
			f."slug",
			f."label",
			f."default",
			f."type",
			f."index",
			f."attributes",
			f."is_visible",
			f.autofill_field,
			f.autofill_table,
			f."unique",
			f."automatic",
			f.relation_id 
	
	FROM field f 
	JOIN "table" t ON t.id = f.table_id
	JOIN relation r ON r.id = $1 WHERE f.id::text = ANY(r.view_fields)`

	reqlationQ := `
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
		r.cascading_tree_table_slug,
		r.cascading_tree_field_slug,
		r.view_fields
	FROM
		relation r
	WHERE  r.id = $1`

	for _, el := range fieldsWithPermissions {
		if el.Attributes != nil && !(el.Type == "LOOKUP" || el.Type == "LOOKUPS" || el.Type == "DYNAMIC") {
			decodedFields = append(decodedFields, el)
		} else {
			elementField := el
			viewFields := []models.Field{}

			if el.RelationId != "" {
				relation := models.RelationBody{}

				err = conn.QueryRow(ctx, reqlationQ, el.RelationId).Scan(
					&relation.Id,
					&relation.TableFrom,
					&relation.TableTo,
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
					&relation.ViewFields,
				)

				if err != nil {
					if !strings.Contains(err.Error(), "no rows") {
						return nil, errors.Wrap(err, "error while scanning relation")
					}
				} else {
					relationsMap[relation.Id] = relation
					elementField.RelationData = relation

					if relation.TableFrom != req.TableSlug {
						elementField.TableSlug = relation.TableFrom
					} else {
						elementField.TableSlug = relation.TableTo
					}

					frows, err := conn.Query(ctx, rquery, el.RelationId)
					if err != nil {
						return &nb.CommonMessage{}, errors.Wrap(err, "error while getting relation fields")
					}
					defer frows.Close()

					for frows.Next() {
						var (
							vf                               = models.Field{}
							attributes                       = []byte{}
							relationIdNull, autofillField    sql.NullString
							defaultStr, index, autofillTable sql.NullString
						)

						err = frows.Scan(
							&vf.Id,
							&vf.TableId,
							&vf.TableSlug,
							&vf.Required,
							&vf.Slug,
							&vf.Label,
							&defaultStr,
							&vf.Type,
							&index,
							&attributes,
							&vf.IsVisible,
							&autofillField,
							&autofillTable,
							&vf.Unique,
							&vf.Automatic,
							&relationIdNull,
						)
						if err != nil {
							return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning relation fields")
						}

						vf.RelationId = relationIdNull.String
						vf.AutofillField = autofillField.String
						vf.AutofillTable = autofillTable.String
						vf.Default = defaultStr.String
						vf.Index = index.String

						if err := json.Unmarshal(attributes, &vf.Attributes); err != nil {
							return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling relation field attributes")
						}

						viewFields = append(viewFields, vf)
					}
				}

			}

			elementField.ViewFields = viewFields
			decodedFields = append(decodedFields, elementField)
		}
	}

	query = `SELECT 
		"id",
		"attributes",
		"table_slug",
		"type",
		"columns",
		COALESCE("group_fields"::varchar[], '{}'),
		"name",
		"view_fields",
		"calendar_from_slug",
		"calendar_to_slug",
		"relation_table_slug",
		"relation_id",
		"name_uz",
		"name_en"
	FROM "view" WHERE "table_slug" = $1 ORDER BY "order" ASC`

	viewRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting views by table slug")
	}
	defer viewRows.Close()

	for viewRows.Next() {
		var (
			attributes                             []byte
			view                                   = models.View{}
			Name, CalendarFromSlug, CalendarToSlug sql.NullString
			RelationTableSlug, RelationId          sql.NullString
			NameUz, NameEn                         sql.NullString
		)

		err := viewRows.Scan(
			&view.Id,
			&attributes,
			&view.TableSlug,
			&view.Type,
			&view.Columns,
			&view.GroupFields,
			&Name,
			&view.ViewFields,
			&CalendarFromSlug,
			&CalendarToSlug,
			&RelationTableSlug,
			&RelationId,
			&NameUz,
			&NameEn,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning views")
		}

		view.Name = Name.String
		view.CalendarFromSlug = CalendarFromSlug.String
		view.CalendarToSlug = CalendarToSlug.String
		view.RelationTableSlug = RelationTableSlug.String
		view.RelationId = RelationId.String
		view.NameUz = NameUz.String
		view.NameEn = NameEn.String

		if view.Columns == nil {
			view.Columns = []string{}
		}

		if err := json.Unmarshal(attributes, &view.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling view attributes")
		}

		views = append(views, view)
	}

	recordPermission, err := helper.GetRecordPermission(ctx, models.GetRecordPermissionRequest{
		Conn:      conn,
		TableSlug: req.TableSlug,
		RoleId:    roleIdFromToken,
	})
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "when get recordPermission")
	}

	getQuery = strings.TrimRight(getQuery, ",")
	getQuery += fmt.Sprintf(`) AS DATA FROM "%s" a`, req.TableSlug)

	if recordPermission.IsHaveCondition {
		params, err = helper.GetAutomaticFilter(ctx, models.GetAutomaticFilterRequest{
			Conn:            conn,
			Params:          params,
			RoleIdFromToken: roleIdFromToken,
			UserIdFromToken: userIdFromToken,
			TableSlug:       req.TableSlug,
		})
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "when get GetAutomaticFilter")
		}
	}

	for key, val := range params {
		if key == "limit" {
			limit = fmt.Sprintf(" LIMIT %d ", cast.ToInt(val))
		} else if key == "offset" {
			offset = fmt.Sprintf(" OFFSET %d ", cast.ToInt(val))
		} else if key == "order" {
			orders := cast.ToStringMap(val)
			counter := 0

			if len(orders) > 0 {
				order = " ORDER BY "
			}

			for k, v := range orders {
				if k == "created_at" {
					continue
				}
				oType := " DESC"
				if cast.ToInt(v) == 1 {
					oType = " ASC"
				}

				if counter == 0 {
					order += fmt.Sprintf(" a.%s"+oType, k)
				} else {
					order += fmt.Sprintf(", a.%s"+oType, k)
				}
				counter++
			}
		} else if key == "auto_filter" {
			var counter int
			filters := cast.ToStringMap(val)
			for k, v := range filters {
				if counter == 0 {
					autoFilters += fmt.Sprintf(" AND (a.%s = $%d", k, argCount)
				} else {
					autoFilters += fmt.Sprintf(" OR a.%s = $%d", k, argCount)
				}

				if counter == len(filters)-1 {
					autoFilters += " )"
				}
				args = append(args, v)
				argCount++
				counter++
			}
		} else {
			if _, ok := fieldsM[key]; ok {
				switch valTyped := val.(type) {
				case []string:
					filter += fmt.Sprintf(" AND a.%s IN($%d) ", key, argCount)
					args = append(args, pq.Array(valTyped))
					argCount++
				case int, float32, float64, int32, bool:
					filter += fmt.Sprintf(" AND a.%s = $%d ", key, argCount)
					args = append(args, valTyped)
					argCount++
				case []any:
					if fieldsM[key] == "MULTISELECT" {
						filter += fmt.Sprintf(" AND a.%s && $%d", key, argCount)
					} else {
						filter += fmt.Sprintf(" AND a.%s = ANY($%d) ", key, argCount)
					}
					args = append(args, pq.Array(valTyped))
					argCount++
				case map[string]any:
					for k, v := range valTyped {
						switch k {
						case "$gt":
							filter += fmt.Sprintf(" AND a.%s > $%d ", key, argCount)
						case "$gte":
							filter += fmt.Sprintf(" AND a.%s >= $%d ", key, argCount)
						case "$lt":
							filter += fmt.Sprintf(" AND a.%s < $%d ", key, argCount)
						case "$lte":
							filter += fmt.Sprintf(" AND a.%s <= $%d ", key, argCount)
						case "$in":
							filter += fmt.Sprintf(" AND a.%s::VARCHAR = ANY($%d)", key, argCount)
						}
						args = append(args, v)
						argCount++
					}
				default:
					if strings.Contains(key, "_id") || key == "guid" {
						if req.TableSlug == "client_type" {
							filter += " AND a.guid = ANY($1::uuid[]) "
							args = append(args, pq.Array(cast.ToStringSlice(val)))
						} else {
							filter += fmt.Sprintf(" AND a.%s = $%d ", key, argCount)
							args = append(args, val)
							argCount++
						}
					} else {
						val = escapeSpecialCharacters(cast.ToString(val))
						filter += fmt.Sprintf(" AND a.%s ~* $%d ", key, argCount)
						args = append(args, val)
						argCount++
					}
				}
			}
		}
	}

	searchValue := cast.ToString(params["search"])
	if len(searchValue) > 0 {
		searchValue = escapeSpecialCharacters(searchValue)

		for idx, val := range searchFields {
			if idx == 0 {
				filter += " AND ("
				searchCondition = ""
			} else {
				searchCondition = " OR "
			}
			filter += fmt.Sprintf(" %s a.%s ~* $%d ", searchCondition, val, argCount)

			args = append(args, searchValue)
			argCount++

			if idx == len(searchFields)-1 {
				filter += " ) "
			}
		}
	}

	getQuery += filter + autoFilters + order + limit + offset

	rows, err = conn.Query(ctx, getQuery, args...)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting rows")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			data any
			temp = make(map[string]any)
		)

		values, err := rows.Values()
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while getting values")
		}

		for i, value := range values {
			temp[rows.FieldDescriptions()[i].Name] = value
			data = temp["data"]
		}

		result = append(result, data)
	}

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" AS a %s`, req.TableSlug, filter+autoFilters)
	err = conn.QueryRow(ctx, countQuery, args...).Scan(&count)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting count")
	}

	repsonse := map[string]any{
		"fields":   decodedFields,
		"views":    views,
		"count":    count,
		"response": result,
	}

	newResp, err := helper.ConvertMapToStruct(repsonse)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while converting map to struct")
	}

	return &nb.CommonMessage{
		TableSlug:     req.TableSlug,
		ProjectId:     req.ProjectId,
		Data:          newResp,
		IsCached:      req.IsCached,
		CustomMessage: req.CustomMessage,
	}, nil
}

func (o *objectBuilderRepo) GetList2(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetList2")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	if req.TableSlug == "template" {
		response := map[string]any{
			"count":    0,
			"response": []string{},
		}

		responseStruct, err := helper.ConvertMapToStruct(response)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while converting map to struct")
		}

		return &nb.CommonMessage{Data: responseStruct, TableSlug: req.TableSlug}, nil
	}

	var (
		params       = make(map[string]any)
		searchFields = []string{}
		fields       = make(map[string]models.Field)
	)

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while marshalling request data")
	}
	if err := json.Unmarshal(paramBody, &params); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling request data")
	}

	query := `
		SELECT 
			f.type, 
			f.slug, 
			f.attributes,
			f.is_search
		FROM "field" f 
		JOIN "table" t ON t.id = f.table_id 
		WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting fields by table slug")
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			fBody = models.Field{}
			attrb = []byte{}
		)

		err = fieldRows.Scan(
			&fBody.Type,
			&fBody.Slug,
			&attrb,
			&fBody.IsSearch,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning fields")
		}

		if fBody.IsSearch && helper.FIELD_TYPES[fBody.Type] == "VARCHAR" {
			searchFields = append(searchFields, fBody.Slug)
		}

		if err := json.Unmarshal(attrb, &fBody.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
		}

		fields[fBody.Slug] = fBody
	}

	withTypes, _ := params["with_types"].(bool)
	delete(params, "with_types")

	items, count, err := helper.GetItemsGetList(ctx, conn, models.GetItemsBody{
		TableSlug:    req.TableSlug,
		Params:       params,
		FieldsMap:    fields,
		SearchFields: searchFields,
	})
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting items")
	}

	if req.TableSlug == "user" && len(items) > 0 && o.grpcClient != nil {
		userIdAuths := make([]string, 0, len(items))
		for _, item := range items {
			if uid, ok := item["user_id_auth"]; ok && uid != nil && uid != "" {
				userIdAuths = append(userIdAuths, cast.ToString(uid))
			}
		}

		if len(userIdAuths) > 0 {
			authResp, err := o.grpcClient.UserService().GetUserListByIDs(ctx, &pa.UserPrimaryKeyList{
				Ids:       userIdAuths,
				ProjectId: req.CompanyProjectId,
			})
			if err == nil && authResp != nil {
				authMap := make(map[string]map[string]any, len(authResp.Users))
				for _, u := range authResp.Users {
					authMap[u.GetId()] = map[string]any{
						"login":    u.GetLogin(),
						"email":    u.GetEmail(),
						"phone":    u.GetPhone(),
						"password": u.GetPassword(),
					}
				}

				for i, item := range items {
					uid := cast.ToString(item["user_id_auth"])
					if a, ok := authMap[uid]; ok {
						items[i]["login"] = a["login"]
						items[i]["email"] = a["email"]
						items[i]["phone"] = a["phone"]
						items[i]["password"] = a["password"]
					}
				}
			}
		}
	}

	// Formula calculations are now handled in CREATE/UPDATE operations
	// Formula values should already be stored in the database

	response := map[string]any{
		"count":    count,
		"response": items,
	}

	if withTypes {
		types := make(map[string]string)
		typeRows, err := conn.Query(ctx,
			`SELECT column_name, udt_name FROM information_schema.columns WHERE table_name = $1`,
			req.TableSlug,
		)
		if err == nil {
			defer typeRows.Close()
			for typeRows.Next() {
				var colName, udtName string
				if err := typeRows.Scan(&colName, &udtName); err == nil {
					types[colName] = udtName
				}
			}
		}
		response["types"] = types
	}

	itemsStruct, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while converting map to struct")
	}

	return &nb.CommonMessage{
		TableSlug:     req.TableSlug,
		ProjectId:     req.ProjectId,
		Data:          itemsStruct,
		IsCached:      req.IsCached,
		CustomMessage: req.CustomMessage,
	}, nil
}

func (o *objectBuilderRepo) GetListSlim(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetListSlim")
	defer dbSpan.Finish()

	var (
		params = make(map[string]any)
		fields = make(map[string]models.Field)
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while marshalling request data")
	}
	if err := json.Unmarshal(paramBody, &params); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling request data")
	}

	query := `SELECT f.type, f.slug, f.attributes FROM "field" f JOIN "table" t ON t.id = f.table_id WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting fields by table slug")
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			fBody = models.Field{}
			attrb = []byte{}
		)

		err = fieldRows.Scan(
			&fBody.Type,
			&fBody.Slug,
			&attrb,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while scanning fields")
		}

		if err := json.Unmarshal(attrb, &fBody.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while unmarshalling field attributes")
		}

		fields[fBody.Slug] = fBody
	}

	items, count, err := helper.GetItems(ctx, conn, models.GetItemsBody{
		TableSlug: req.TableSlug,
		Params:    params,
		FieldsMap: fields,
	})
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting items")
	}

	// Formula calculations are now handled in CREATE/UPDATE operations
	// Formula values should already be stored in the database

	response := map[string]any{
		"count":    count,
		"response": items,
	}

	itemsStruct, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while converting map to struct")
	}

	return &nb.CommonMessage{
		TableSlug:     req.TableSlug,
		ProjectId:     req.ProjectId,
		Data:          itemsStruct,
		IsCached:      req.IsCached,
		CustomMessage: req.CustomMessage,
	}, nil
}

func (o *objectBuilderRepo) GetListInExcel(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {

	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetListInExcel")
	defer dbSpan.Finish()

	var (
		params       = make(map[string]any)
		fields       = make(map[string]models.Field)
		fieldsArr    = []models.Field{}
		searchFields = make([]string, 0)
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	if err = json.Unmarshal(paramBody, &params); err != nil {
		return &nb.CommonMessage{}, err
	}

	params["with_relations"] = true

	var (
		fieldIds  = cast.ToStringSlice(params["field_ids"])
		language  = cast.ToString(params["language"])
		utcOffset = cast.ToFloat64(params["utc_offset"])
	)

	if utcOffset == 0 {
		utcOffset = 5
	}

	delete(params, "field_ids")

	query := `
		SELECT 
			f.type, 
			f.slug, 
			f.attributes, 
			f.label,
			f.is_search,
			(
				SELECT jsonb_agg(jsonb_build_object(
					'slug', f2.slug,
					'enable_multilanguage', f2.enable_multilanguage
				))
				FROM field f2
				WHERE f2.id = ANY(r.view_fields::uuid[])
			) AS view_fields
		FROM 
			"field" f
		LEFT JOIN 
			"relation" r ON f.relation_id = r.id
		WHERE 
			f.id = ANY($1) OR f.relation_id = ANY($2)
		ORDER BY
			COALESCE(
				array_position($1::uuid[], f.id),
				array_position($1::uuid[], f.relation_id)
			)
	`

	fieldRows, err := conn.Query(ctx, query, pq.Array(fieldIds), pq.Array(fieldIds))
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			fBody      = models.Field{}
			attrb      = []byte{}
			viewFields = []models.Field{}
		)

		err = fieldRows.Scan(
			&fBody.Type,
			&fBody.Slug,
			&attrb,
			&fBody.Label,
			&fBody.IsSearch,
			&viewFields,
		)
		if err != nil {
			return &nb.CommonMessage{}, err
		}

		if err = json.Unmarshal(attrb, &fBody.Attributes); err != nil {
			return &nb.CommonMessage{}, err
		}

		if fBody.IsSearch && helper.FIELD_TYPES[fBody.Type] == "VARCHAR" {
			searchFields = append(searchFields, fBody.Slug)
		}

		fBody.ViewFields = viewFields
		fields[fBody.Slug] = fBody
		fieldsArr = append(fieldsArr, fBody)
	}

	params["limit"] = config.MAX_EXCEL_LIMIT
	delete(params, "language")

	items, _, err := helper.GetItemsGetList(
		ctx, conn,
		models.GetItemsBody{
			TableSlug:    req.TableSlug,
			Params:       params,
			FieldsMap:    fields,
			SearchFields: searchFields,
		},
	)

	if err != nil {
		return &nb.CommonMessage{}, err
	}

	var file = excel.NewFile()
	sheetName := file.GetSheetName(0)

	for i, field := range fieldsArr {

		attributesByte, err1 := field.Attributes.MarshalJSON()
		if err1 != nil {
			return &nb.CommonMessage{}, fmt.Errorf("error while marshalling field attributes: %w", err)
		}

		var (
			attributesMap = make(map[string]any)
			label         = "label"
			ok            bool
		)

		err = json.Unmarshal(attributesByte, &attributesMap)
		if err != nil {
			return &nb.CommonMessage{}, fmt.Errorf("error while unmarshalling field attributes: %w", err)
		}

		if len(language) > 0 {
			label = fmt.Sprintf("label_%s", language)

			if _, ok = attributesMap[label].(string); ok {
				err = file.SetCellValue(sheetName, convertToTitle(i)+"1", attributesMap[label])
				if err != nil {
					return &nb.CommonMessage{}, err
				}

				continue
			}
		}

		if _, ok = attributesMap["label"].(string); ok {
			err = file.SetCellValue(sheetName, convertToTitle(i)+"1", attributesMap["label"])
			if err != nil {
				return &nb.CommonMessage{}, err
			}

			continue
		}

		err = file.SetCellValue(sheetName, convertToTitle(i)+"1", field.Label)
		if err != nil {
			return &nb.CommonMessage{}, err
		}
	}

	for i, item := range items {
		letterCount := 0
		column := fmt.Sprint(i + 2)

		for _, f := range fieldsArr {
			_, ok := fields[f.Slug]
			f.Slug = strings.ToLower(f.Slug)
			if ok {
				if f.Type == "MULTI_LINE" {
					re := regexp.MustCompile(`<[^>]+>`)

					result := re.ReplaceAllString(cast.ToString(item[f.Slug]), "")

					item[f.Slug] = result
				} else if f.Type == "DATE" {
					formattedDate, err := formatExcelDateValue(item[f.Slug])
					if err != nil {
						return &nb.CommonMessage{}, err
					}

					item[f.Slug] = formattedDate
				} else if f.Type == "DATE_TIME" {
					t, err := time.Parse(config.DateTimeWithZone, cast.ToString(item[f.Slug]))
					if err != nil {
						item[f.Slug] = ""
					} else {
						item[f.Slug] = t.Add(time.Duration(utcOffset) * time.Hour).Format(time.DateTime)
					}
				} else if f.Type == "DATE_TIME_WITHOUT_TIME_ZONE" {
					t, err := time.Parse("2006-01-02 15:04:05", strings.Split(cast.ToString(item[f.Slug]), ".")[0])
					if err != nil {
						item[f.Slug] = ""
					} else {
						item[f.Slug] = t.Add(time.Duration(utcOffset) * time.Hour).Format(time.DateTime)
					}
				} else if f.Type == "MULTISELECT" {
					attributes, err := helper.ConvertStructToMap(f.Attributes)
					if err != nil {
						return &nb.CommonMessage{}, err
					}

					multiselectValue := ""

					_, ok = attributes["options"]
					if ok {
						options := cast.ToSlice(attributes["options"])
						isMultiSelect := cast.ToBool(attributes["is_multiselect"])
						values := cast.ToStringSlice(item[f.Slug])

						if len(options) > 0 && len(values) > 0 {
							for _, val := range values {
								for _, op := range options {
									opt := cast.ToStringMap(op)
									if val == cast.ToString(opt["value"]) {
										if isMultiSelect {
											if cast.ToString(opt["label_"+language]) != "" {
												multiselectValue += cast.ToString(opt["label_"+language]) + ","
											} else if cast.ToString(opt["label"]) != "" {
												multiselectValue += cast.ToString(opt["label"]) + ","

											} else {
												multiselectValue += cast.ToString(opt["value"]) + ","
											}
										} else {
											if cast.ToString(opt["label_"+language]) != "" {
												multiselectValue = cast.ToString(opt["label_"+language]) + ","
											} else if cast.ToString(opt["label"]) != "" {
												multiselectValue = cast.ToString(opt["label"]) + ","

											} else {
												multiselectValue = cast.ToString(opt["value"]) + ","
											}
										}
									}
								}
							}
						}
					}

					item[f.Slug] = strings.TrimRight(multiselectValue, ",")
				} else if f.Type == "LOOKUP" {
					var overall string
					for _, v := range f.ViewFields {
						lookupData, ok := item[f.Slug+"_data"].(map[string]any)
						if !ok {
							continue
						}

						if v.EnableMultilanguage {
							splitted := strings.Split(v.Slug, "_")
							if len(splitted) > 0 {
								lang := splitted[len(splitted)-1]
								if language == lang {
									overall += cast.ToString(lookupData[v.Slug])
								}
							}
						} else {
							overall += cast.ToString(lookupData[v.Slug])
						}

						overall += " "
					}
					item[f.Slug] = overall
				}

				err = file.SetCellValue(sheetName, convertToTitle(letterCount)+column, item[f.Slug])
				if err != nil {
					return &nb.CommonMessage{}, err
				}
				letterCount++
			}
		}
	}

	var (
		filename        = fmt.Sprintf("report_%d.xlsx", time.Now().Unix())
		filepath        = "./" + filename
		cfg             = config.Load()
		endpoint        = cfg.MinioHost
		accessKeyID     = cfg.MinioAccessKeyID
		secretAccessKey = cfg.MinioSecretKey
	)

	err = file.SaveAs(filename)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	minioClient, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: cfg.MinioSSL,
	})
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	_, err = minioClient.FPutObject(
		ctx,
		req.GetProjectId(),
		filename,
		filepath,
		minio.PutObjectOptions{ContentType: ""},
	)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	err = os.Remove(filepath)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	link := fmt.Sprintf("%s/%s/%s", endpoint, req.GetProjectId(), filename)
	respExcel := map[string]string{
		"link": link,
	}

	marshledInputMap, err := json.Marshal(respExcel)
	outputStruct := &structpb.Struct{}
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	err = protojson.Unmarshal(marshledInputMap, outputStruct)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	return &nb.CommonMessage{TableSlug: req.TableSlug, Data: outputStruct}, nil
}

func (o *objectBuilderRepo) GroupByColumns(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GroupByColumns")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, errors.Wrap(err, "failed to get database connection")
	}

	// Parse request data
	reqData, err := helper.ConvertStructToMap(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to convert request data")
	}

	viewID, ok := reqData["builder_service_view_id"]
	if !ok {
		return &nb.CommonMessage{}, errors.New("builder_service_view_id is required")
	}

	// Get view configuration
	groupFields, err := o.getViewGroupFields(ctx, conn, cast.ToString(viewID))
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to get view group fields")
	}

	if len(groupFields) == 0 {
		return &nb.CommonMessage{}, helper.HandleDatabaseError(
			errors.New("group_by_columns is required"),
			o.logger,
			"group_by_columns is required",
		)
	}

	// Get table fields mapping
	fieldMap, fieldSlugMap, slugRelation, err := o.getTableFields(ctx, conn, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to get table fields")
	}

	// Map group field IDs to slugs
	groupFieldSlugs := o.mapGroupFieldIdsToSlugs(groupFields, fieldSlugMap)

	// Build hierarchical query with related data included
	query, err := o.buildHierarchicalGroupQueryWithRelatedData(conn, req.TableSlug, groupFieldSlugs, fieldMap, slugRelation)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to build hierarchical query")
	}

	// Execute query
	rows, err := conn.Query(ctx, query)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to execute group by query")
	}
	defer rows.Close()

	// Process results
	results, err := o.processGroupByResults(rows)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to process group by results")
	}

	// Prepare response
	response := map[string]any{
		"response": results,
	}

	responseStruct, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "failed to convert response to struct")
	}

	return &nb.CommonMessage{
		Data:      responseStruct,
		TableSlug: req.TableSlug,
	}, nil
}

// getViewGroupFields retrieves group fields from view configuration
func (o *objectBuilderRepo) getViewGroupFields(ctx context.Context, conn *psqlpool.Pool, viewID string) ([]string, error) {
	query := `SELECT attributes, group_fields FROM view WHERE id = $1`

	var attributesBytes []byte
	var groupFields []string

	err := conn.QueryRow(ctx, query, viewID).Scan(&attributesBytes, &groupFields)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get view attributes")
	}

	var viewAttributes map[string]any
	if err = json.Unmarshal(attributesBytes, &viewAttributes); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal view attributes")
	}

	// Try to get group_by_columns from attributes first
	if groupByColumns, ok := viewAttributes["group_by_columns"]; ok {
		if columns, ok := groupByColumns.([]any); ok {
			result := make([]string, len(columns))
			for i, col := range columns {
				result[i] = cast.ToString(col)
			}
			return result, nil
		}
	}

	// Fallback to group_fields if group_by_columns is not available
	return groupFields, nil
}

// getTableFields retrieves field information for the table
func (o *objectBuilderRepo) getTableFields(ctx context.Context, conn *psqlpool.Pool, tableSlug string) (map[string]string, map[string]string, map[string]string, error) {
	query := `SELECT f.id, f.type, f.slug, COALESCE(f.relation_id::varchar, ''), COALESCE(r.table_to::varchar, '')
			  FROM field f 
			  JOIN "table" t ON f.table_id = t.id
			  LEFT JOIN "relation" r ON f.relation_id = r.id
			  WHERE t.slug = $1`

	rows, err := conn.Query(ctx, query, tableSlug)
	if err != nil {
		return nil, nil, nil, errors.Wrap(err, "failed to get table fields")
	}
	defer rows.Close()

	var (
		fieldMap        = make(map[string]string) // slug -> type
		fieldSlugMap    = make(map[string]string) // id -> slug
		slugRelationMap = make(map[string]string) // slug -> relation
	)

	for rows.Next() {
		var id, fieldType, slug, relationID, relationTable string

		err = rows.Scan(&id, &fieldType, &slug, &relationID, &relationTable)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to scan field")
		}

		// Use relation ID if available

		if relationID != "" {
			id = relationID
		}

		fieldMap[slug] = fieldType
		fieldSlugMap[id] = slug
		slugRelationMap[slug] = relationTable
	}

	return fieldMap, fieldSlugMap, slugRelationMap, nil
}

// mapGroupFieldIdsToSlugs maps group field IDs to their corresponding slugs
func (o *objectBuilderRepo) mapGroupFieldIdsToSlugs(groupFields []string, fieldSlugMap map[string]string) []string {
	result := make([]string, len(groupFields))
	for i, fieldID := range groupFields {
		if slug, exists := fieldSlugMap[fieldID]; exists {
			result[i] = slug
		} else {
			result[i] = fieldID // fallback to original if mapping not found
		}
	}
	return result
}

// buildHierarchicalGroupQueryWithRelatedData builds a hierarchical SQL query for grouping with related data included
func (o *objectBuilderRepo) buildHierarchicalGroupQueryWithRelatedData(conn *psqlpool.Pool, tableSlug string, groupFields []string, fieldMap, fieldRelation map[string]string) (string, error) {
	if len(groupFields) == 0 {
		return "", errors.New("no group fields provided")
	}

	// Get all field slugs for the data aggregation
	allFields := make([]string, 0, len(fieldMap))
	for fieldSlug := range fieldMap {
		allFields = append(allFields, fieldSlug)
	}

	// Build the innermost query with related data included
	innerQuery := o.buildInnerGroupQueryWithRelatedData(conn, tableSlug, groupFields, allFields, fieldMap, fieldRelation)

	// Build hierarchical structure if multiple group fields
	if len(groupFields) == 1 {
		return innerQuery, nil
	}

	return o.buildHierarchicalStructure(innerQuery, groupFields), nil
}

// buildInnerGroupQueryWithRelatedData builds the innermost query that groups by all specified fields and includes related data
func (o *objectBuilderRepo) buildInnerGroupQueryWithRelatedData(conn *psqlpool.Pool, tableSlug string, groupFields, allFields []string, fieldTypes, fieldRelation map[string]string) string {
	var query strings.Builder

	// SELECT clause with group fields
	query.WriteString("SELECT ")
	query.WriteString(strings.Join(groupFields, ", "))

	// Add JSON aggregation for all fields including related data
	query.WriteString(", jsonb_agg(jsonb_build_object( ")

	for i, field := range allFields {
		if i > 0 {
			query.WriteString(", ")
		}
		query.WriteString(fmt.Sprintf("'%s', %s", field, field))

		// Add related data as subquery for _id fields (skip folder_id)
		if fieldTypes[field] == "LOOKUP" {
			relatedTable := fieldRelation[field]

			query.WriteString(fmt.Sprintf(", '%s_data', (SELECT row_to_json(rel) FROM \"%s\" rel WHERE rel.guid = %s LIMIT 1)",
				field, relatedTable, field))
		}
	}

	query.WriteString(")) as data ")
	query.WriteString(fmt.Sprintf("FROM \"%s\" ", tableSlug))
	query.WriteString(fmt.Sprintf("GROUP BY %s", strings.Join(groupFields, ", ")))

	return query.String()
}

// buildHierarchicalStructure builds the hierarchical grouping structure
func (o *objectBuilderRepo) buildHierarchicalStructure(innerQuery string, groupFields []string) string {
	// Reverse the group fields for hierarchical building
	reversedFields := make([]string, len(groupFields))
	copy(reversedFields, groupFields)
	for i, j := 0, len(reversedFields)-1; i < j; i, j = i+1, j-1 {
		reversedFields[i], reversedFields[j] = reversedFields[j], reversedFields[i]
	}

	currentQuery := innerQuery
	lastField := reversedFields[0]

	for i := 1; i < len(reversedFields); i++ {
		var query strings.Builder

		query.WriteString("SELECT ")

		// Add group fields for this level
		groupClause := ""
		for j := i; j < len(reversedFields); j++ {
			if j > i {
				query.WriteString(", ")
			}
			query.WriteString(reversedFields[j])
			groupClause += reversedFields[j] + ","
		}
		groupClause = strings.TrimRight(groupClause, ",")

		// Add JSON aggregation
		query.WriteString(fmt.Sprintf(", jsonb_agg(jsonb_build_object( '%s', %s, 'data', data)) as data ", lastField, lastField))
		query.WriteString(fmt.Sprintf("FROM ( %s ) subquery ", currentQuery))
		query.WriteString(fmt.Sprintf("GROUP BY %s", groupClause))

		currentQuery = query.String()
		lastField = reversedFields[i]
	}

	return currentQuery
}

// processGroupByResults processes the query results and converts them to the expected format
func (o *objectBuilderRepo) processGroupByResults(rows pgx.Rows) ([]map[string]any, error) {
	var results []map[string]any

	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get row values")
		}

		rowData := make(map[string]any)
		fieldDescriptions := rows.FieldDescriptions()

		for i, value := range values {
			fieldName := string(fieldDescriptions[i].Name)

			// Convert UUID fields
			if strings.Contains(fieldName, "_id") || fieldName == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = helper.ConvertGuid(arr)
				}
			}

			rowData[fieldName] = value
		}

		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil {
		return nil, errors.Wrap(err, "error iterating over rows")
	}

	return results, nil
}

func (o *objectBuilderRepo) UpdateWithParams(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.UpdateWithParams")
	defer dbSpan.Finish()

	var (
		fields   []string
		argCount = 1
		args     = []any{}
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	data, err := helper.ConvertStructToMap(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	params := cast.ToStringMap(data["params"])
	delete(data, "params")

	queryField := `SELECT f.slug FROM field f JOIN "table" t ON t.id = f.table_id WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, queryField, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, err
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		slug := ""

		err = fieldRows.Scan(&slug)
		if err != nil {
			return &nb.CommonMessage{}, err
		}
		fields = append(fields, slug)
	}

	query := fmt.Sprintf(`UPDATE %s SET `, req.TableSlug)

	filter := " WHERE 1=1 "

	for _, slug := range fields {
		arg, ok := data[slug]
		if ok {
			query += fmt.Sprintf(` %s = $%d,`, slug, argCount)
			argCount++
			args = append(args, arg)
		}

		val, ok := params[slug]
		if ok {
			switch val.(type) {
			case map[string]any:
				newOrder := cast.ToStringMap(val)

				for k, v := range newOrder {
					switch v.(type) {
					case string:
						if cast.ToString(v) == "" {
							continue
						}
					}

					switch k {
					case "$gt":
						filter += fmt.Sprintf(" AND %s > $%d ", slug, argCount)
					case "$gte":
						filter += fmt.Sprintf(" AND %s >= $%d ", slug, argCount)
					case "$lt":
						filter += fmt.Sprintf(" AND %s < $%d ", slug, argCount)
					case "$lte":
						filter += fmt.Sprintf(" AND %s <= $%d ", slug, argCount)
					}

					args = append(args, v)
					argCount++
				}
			default:
				filter += fmt.Sprintf(` AND %s = $%d`, slug, argCount)
				argCount++
				args = append(args, val)
			}
		}
	}

	query = strings.TrimRight(query, ",")
	query = query + filter

	_, err = conn.Exec(ctx, query, args...)
	if err != nil {
		return &nb.CommonMessage{}, err
	}

	return &nb.CommonMessage{
		TableSlug: req.TableSlug,
	}, nil
}

// QueryBuilder holds the state for building SQL queries

// GetListV2 retrieves a list of records with filtering and pagination
func (o *objectBuilderRepo) GetListV2(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetListV2")
	defer dbSpan.Finish()

	// Get database connection
	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	// Initialize query builder
	qb := NewQueryBuilder()

	// Get parameters from request
	params, err := helper.ConvertStructToMap(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "when convert struct to map")
	}

	// Get role ID from token
	roleIdFromToken := cast.ToString(params["role_id_from_token"])
	userIdFromToken := cast.ToString(params["user_id_from_token"])

	// Get fields for the table
	fquery := `SELECT 
		f.slug, 
		f.type, 
		t.order_by, 
		f.is_search,
		t.is_cached 
	FROM field f 
	JOIN "table" t ON t.id = f.table_id 
	WHERE t.slug = $1`
	fieldRows, err := conn.Query(ctx, fquery, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting fields by table slug")
	}
	defer fieldRows.Close()

	// Build field query
	if err := qb.buildFieldQuery(fieldRows); err != nil {
		return &nb.CommonMessage{}, err
	}

	// Add relations if requested
	withRelations, ok := params["with_relations"]
	if cast.ToBool(withRelations) || !ok {
		qb.buildRelationsQuery()
	}

	// Get record permissions
	recordPermission, err := helper.GetRecordPermission(ctx, models.GetRecordPermissionRequest{
		Conn:      conn,
		TableSlug: req.TableSlug,
		RoleId:    roleIdFromToken,
	})
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "when get recordPermission")
	}

	// Apply automatic filters if needed
	if recordPermission.IsHaveCondition {
		params, err = helper.GetAutomaticFilter(ctx, models.GetAutomaticFilterRequest{
			Conn:            conn,
			Params:          params,
			RoleIdFromToken: roleIdFromToken,
			UserIdFromToken: userIdFromToken,
			TableSlug:       req.TableSlug,
		})
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "when GetAutomaticFilter")
		}
	}

	// Extract with_types before filters so it's not treated as a column filter
	withTypes, _ := params["with_types"].(bool)
	delete(params, "with_types")

	// Apply filters and search
	qb.applyFilters(params)
	qb.buildSearchFilter(cast.ToString(params["search"]))

	additionalRequest := cast.ToStringMap(params["additional_request"])
	if len(additionalRequest) > 0 {
		qb.additionalField = cast.ToString(additionalRequest["additional_field"])
		qb.additionalValues = cast.ToSlice(additionalRequest["additional_values"])
	}

	// Build final query
	finalQuery := qb.finalizeQuery(req.TableSlug)

	// Execute query
	rows, err := conn.Query(ctx, finalQuery, qb.args...)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting rows")
	}
	defer rows.Close()

	if len(qb.additionalValues) > 0 {
		qb.args = qb.args[:len(qb.args)-1]
	}

	// Process results
	var result []any
	for rows.Next() {
		var (
			data any
			temp = make(map[string]any)
		)

		values, err := rows.Values()
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "error while getting values")
		}

		for i, value := range values {
			temp[rows.FieldDescriptions()[i].Name] = value
			data = temp["data"]
		}

		result = append(result, data)
	}

	var count int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "%s" AS a %s`, req.TableSlug, qb.filter+qb.autoFilters)
	err = conn.QueryRow(ctx, countQuery, qb.args...).Scan(&count)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while getting count")
	}

	rr := map[string]any{
		"response": result,
		"count":    count,
	}

	if withTypes {
		types := make(map[string]string)
		typeRows, err := conn.Query(ctx,
			`SELECT column_name, udt_name FROM information_schema.columns WHERE table_name = $1`,
			req.TableSlug,
		)
		if err == nil {
			defer typeRows.Close()
			for typeRows.Next() {
				var colName, udtName string
				if err := typeRows.Scan(&colName, &udtName); err == nil {
					types[colName] = udtName
				}
			}
		}
		rr["types"] = types
	}

	response, _ := helper.ConvertMapToStruct(rr)

	return &nb.CommonMessage{
		Data:     response,
		IsCached: qb.isCached,
	}, nil
}

func (o *objectBuilderRepo) GetSingleSlim(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetSingleSlim")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	data, err := helper.ConvertStructToMap(req.Data)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "convert req data")
	}

	output, err := helper.GetItem(ctx, conn, req.TableSlug, cast.ToString(data["id"]), false)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "get item by id")
	}

	query := `SELECT 
		f."id",
		f."table_id",
		f."required",
		f."slug",
		f."label",
		f."default",
		f."type",
		f."index",
		f."attributes",
		f."is_visible",
		f.autofill_field,
		f.autofill_table,
		f."unique",
		f."automatic",
		f.relation_id
	FROM "field" as f JOIN "table" as t ON t.id = f.table_id WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "get fields by table slug")
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			field                          = models.Field{}
			atr                            = []byte{}
			autoFillField, autoFillTable   sql.NullString
			relationId, defaultNull, index sql.NullString
		)

		err = fieldRows.Scan(
			&field.Id,
			&field.TableId,
			&field.Required,
			&field.Slug,
			&field.Label,
			&defaultNull,
			&field.Type,
			&index,
			&atr,
			&field.IsVisible,
			&autoFillField,
			&autoFillTable,
			&field.Unique,
			&field.Automatic,
			&relationId,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "scan fields")
		}

		if err := json.Unmarshal(atr, &field.Attributes); err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "unmarshal attributes")
		}
	}

	var (
		attributeTableFromSlugs, attributeTableFromRelationIds []string
		relationFieldTablesMap                                 = make(map[string]any)
		relationFieldTableIds                                  = []string{}
	)

	query = `SELECT id, slug FROM "table" WHERE slug IN ($1)`

	tableRows, err := conn.Query(ctx, query, pq.Array(attributeTableFromSlugs))
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "get tables by slugs")
	}
	defer tableRows.Close()

	for tableRows.Next() {
		table := models.Table{}

		err = tableRows.Scan(&table.Id, &table.Slug)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "scan tables")
		}

		relationFieldTableIds = append(relationFieldTableIds, table.Id)
		relationFieldTablesMap[table.Slug] = table
	}

	query = `SELECT slug, table_id, relation_id FROM "field" WHERE relation_id IN ($1) AND table_id IN ($2)`

	relationFieldRows, err := conn.Query(ctx, query, pq.Array(attributeTableFromRelationIds), pq.Array(relationFieldTableIds))
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "get relation fields")
	}
	defer relationFieldRows.Close()

	relationFieldsMap := make(map[string]string)

	for relationFieldRows.Next() {
		field := models.Field{}

		err = relationFieldRows.Scan(
			&field.Slug,
			&field.TableId,
			&field.RelationId,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "scan relation fields")
		}

		relationFieldsMap[field.RelationId+"_"+field.TableId] = field.Slug
	}

	query = `SELECT id, type, field_from FROM "relation" WHERE id IN ($1)`

	dynamicRows, err := conn.Query(ctx, query, pq.Array(attributeTableFromRelationIds))
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "get dynamic rows")
	}
	defer dynamicRows.Close()

	dynamicRelationsMap := make(map[string]models.Relation)

	for dynamicRows.Next() {
		relation := models.Relation{}

		err = dynamicRows.Scan(
			&relation.Id,
			&relation.Type,
			&relation.FieldFrom,
		)
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "scan dynamic rows")
		}

		dynamicRelationsMap[relation.Id] = relation
	}

	// Formula calculations are now handled in CREATE/UPDATE operations
	// Formula values should already be stored in the database

	response := make(map[string]any)
	response["response"] = output

	newBody, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "convert map to struct")
	}

	return &nb.CommonMessage{
		ProjectId: req.ProjectId,
		TableSlug: req.TableSlug,
		Data:      newBody,
	}, err
}

func (o *objectBuilderRepo) GetListAggregation(ctx context.Context, req *nb.CommonMessage) (resp *nb.CommonMessage, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetListAggregation")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	var (
		sb          = squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
		queryParams models.QueryParams
		query       string
		args        []any
	)

	dataMap := req.Data.AsMap()
	jsonBytes, err := json.Marshal(dataMap)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "marshal req.Data to JSON")
	}

	err = json.Unmarshal(jsonBytes, &queryParams)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "unmarshal query params")
	}

	switch queryParams.Operation {
	case "SELECT":
		query, args, err = executeSelect(queryParams, sb)
	case "UPDATE":
		query, args, err = executeUpdate(queryParams, sb)
	default:
		return &nb.CommonMessage{}, errors.New("operation not found")
	}

	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "execute select")
	}

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "query execution")
	}
	defer rows.Close()

	columns := make([]string, len(rows.FieldDescriptions()))
	for i, fd := range rows.FieldDescriptions() {
		columns[i] = string(fd.Name)
	}

	var results []map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return &nb.CommonMessage{}, errors.Wrap(err, "get values")
		}

		rowData := make(map[string]any)
		for i, col := range columns {
			if value, ok := values[i].([16]uint8); ok { // uuid
				rowData[col] = uuid.UUID(value).String()
				continue
			}
			rowData[col] = values[i]
		}
		results = append(results, rowData)
	}

	if err := rows.Err(); err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "rows error")
	}

	repsonse := map[string]any{
		"data": results,
	}

	newResp, err := helper.ConvertMapToStruct(repsonse)
	if err != nil {
		return &nb.CommonMessage{}, errors.Wrap(err, "error while converting map to struct")
	}

	return &nb.CommonMessage{
		Data:      newResp,
		ProjectId: req.ProjectId,
	}, nil
}

func (o *objectBuilderRepo) GetBoardStructure(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to get database connection")
	}

	params, paramsMap, err := parseBoardParams(req)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to parse request")
	}

	var (
		groupByField    = params.GroupBy.Field
		subgroupByField = params.SubgroupBy.Field
		hasSubgroup     = subgroupByField != ""
		groups          = make([]models.BoardGroup, 0)
		subgroups       = make([]models.BoardSubgroup, 0)
		response        = map[string]any{}
		noGroupValue    = "Unassigned"
	)

	if groupByField == "" {
		return nil, helper.HandleDatabaseError(errors.New("group_by field is required"), o.logger, "GetBoardStructure: Group by is required")
	}

	whereClause, whereArgs, err := o.buildBoardWhereClause(ctx, conn, req.TableSlug, paramsMap, params.Search, params.ViewFields, 1)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to build filters")
	}

	query := fmt.Sprintf(`
		SELECT
			unnest(ARRAY[a.%s]::TEXT[]) AS name,
			COUNT(*) AS count
		FROM %s a
		WHERE %s
		GROUP BY 1
	`,
		pq.QuoteIdentifier(groupByField),
		pq.QuoteIdentifier(req.TableSlug),
		whereClause,
	)
	rows, err := conn.Query(ctx, query, whereArgs...)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to query db")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			name  sql.NullString
			count int
		)
		if err := rows.Scan(&name, &count); err != nil {
			return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to scan row")
		}
		if !name.Valid {
			name.String = noGroupValue
		}
		groups = append(groups, models.BoardGroup{
			Name:  name.String,
			Count: count,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Error after row iteration")
	}

	if hasSubgroup {
		query := fmt.Sprintf(`
				SELECT %s, COUNT(*) as count 
				FROM %s a
				WHERE %s
				GROUP BY %s 
			`,
			"a."+pq.QuoteIdentifier(subgroupByField),
			pq.QuoteIdentifier(req.TableSlug),
			whereClause,
			"a."+pq.QuoteIdentifier(subgroupByField),
		)
		rows, err := conn.Query(ctx, query, whereArgs...)
		if err != nil {
			return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to query db")
		}
		defer rows.Close()

		for rows.Next() {
			var (
				name  sql.NullString
				count int
			)
			if err := rows.Scan(&name, &count); err != nil {
				return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to scan row")
			}
			if !name.Valid {
				name.String = noGroupValue
			}
			subgroups = append(subgroups, models.BoardSubgroup{
				Name:  name.String,
				Count: count,
			})
		}
		if err := rows.Err(); err != nil {
			return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Error after row iteration")
		}
	}

	response["groups"] = groups
	response["subgroups"] = subgroups

	respData, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardStructure: Failed to convert response")
	}

	return &nb.CommonMessage{
		TableSlug: req.TableSlug,
		ProjectId: req.ProjectId,
		Data:      respData,
	}, nil
}

func (o *objectBuilderRepo) GetBoardData(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to get database connection")
	}

	params, paramsMap, err := parseBoardParams(req)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to parse request")
	}

	var (
		groupByField    = params.GroupBy.Field
		subgroupByField = params.SubgroupBy.Field
		hasSubgroup     = subgroupByField != ""
		fields          = params.Fields
		viewFields      = params.ViewFields
		limit           = params.Limit
		offset          = params.Offset
		search          = params.Search
		orderBy         = "created_at"
		noGroupValue    = "Unassigned"

		groups    = make(map[string][]any)
		subgroups = make(map[string]map[string][]any)
		response  = map[string]any{}
	)

	if len(fields) == 0 {
		return nil, helper.HandleDatabaseError(errors.New("fields are required"), o.logger, "GetBoardData: Fields are required")
	}
	if groupByField == "" {
		return nil, helper.HandleDatabaseError(errors.New("group_by field is required"), o.logger, "GetBoardData: Group by is required")
	}
	if hasSubgroup {
		orderBy = subgroupByField
	}

	lookupFields, relatedTables, err := getLookupFields(ctx, conn, fields, req.TableSlug)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to get lookup fields")
	}

	var sb strings.Builder
	baseFields := joinColumnsWithPrefix(fields, "a.")

	sb.WriteString(baseFields)

	for i, field := range lookupFields {
		relationAlias := fmt.Sprintf("related_table%d", i+1)
		sb.WriteString(fmt.Sprintf(`, 
        (SELECT row_to_json(%s) 
         FROM "%s" %s 
         WHERE %s.guid = a.%s
        ) AS %s_data`,
			relationAlias,
			relatedTables[i],
			relationAlias,
			relationAlias,
			field,
			field,
		))
	}

	whereClause, whereArgs, err := o.buildBoardWhereClause(ctx, conn, req.TableSlug, paramsMap, search, viewFields, 3)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to build filters")
	}
	queryParams := append([]any{offset, limit}, whereArgs...)

	query := buildBoardDataQuery(req.TableSlug, sb.String(), whereClause, orderBy)

	rows, err := conn.Query(ctx, query, queryParams...)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to query db")
	}
	defer rows.Close()

	fieldDescriptions := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescriptions))
	for i, fd := range fieldDescriptions {
		columns[i] = string(fd.Name)
	}

	for rows.Next() {
		row, err := processRow(rows, columns)
		if err != nil {
			return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to process row")
		}

		groupValues := toStringSlice(row[groupByField], noGroupValue)

		var subgroupValues []string
		if hasSubgroup {
			subgroupValues = toStringSlice(row[subgroupByField], noGroupValue)
		}

		for _, groupValue := range groupValues {
			if hasSubgroup {
				for _, subgroupValue := range subgroupValues {
					if _, exists := subgroups[subgroupValue]; !exists {
						subgroups[subgroupValue] = make(map[string][]any)
					}
					subgroups[subgroupValue][groupValue] = append(subgroups[subgroupValue][groupValue], row)
				}
			} else {
				groups[groupValue] = append(groups[groupValue], row)
			}
		}
	}

	var count int
	countWhereClause, countWhereArgs, err := o.buildBoardWhereClause(ctx, conn, req.TableSlug, paramsMap, search, viewFields, 1)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to build count filters")
	}
	countQuery := buildBoardCountQuery(req.TableSlug, countWhereClause)
	err = conn.QueryRow(ctx, countQuery, countWhereArgs...).Scan(&count)
	if err != nil {
		return &nb.CommonMessage{}, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to get count")
	}

	response["response"] = groups
	response["count"] = count
	if hasSubgroup {
		response["response"] = subgroups
	}

	respData, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return nil, helper.HandleDatabaseError(err, o.logger, "GetBoardData: Failed to convert response")
	}

	return &nb.CommonMessage{
		TableSlug: req.TableSlug,
		ProjectId: req.ProjectId,
		Data:      respData,
	}, nil
}

func parseBoardParams(req *nb.CommonMessage) (*models.BoardDataParams, map[string]any, error) {
	params := &models.BoardDataParams{}
	paramBody, err := json.Marshal(req.Data)
	if err != nil {
		return nil, nil, err
	}
	if err := json.Unmarshal(paramBody, params); err != nil {
		return nil, nil, err
	}

	paramsMap := map[string]any{}
	if err := json.Unmarshal(paramBody, &paramsMap); err != nil {
		return nil, nil, err
	}
	if paramsMap == nil {
		paramsMap = map[string]any{}
	}

	return params, paramsMap, nil
}

func (o *objectBuilderRepo) buildBoardWhereClause(ctx context.Context, conn *psqlpool.Pool, tableSlug string, params map[string]any, search string, viewFields []string, startParamIdx int) (string, []any, error) {
	roleIdFromToken := cast.ToString(params["role_id_from_token"])
	userIdFromToken := cast.ToString(params["user_id_from_token"])

	recordPermission, err := helper.GetRecordPermission(ctx, models.GetRecordPermissionRequest{
		Conn:      conn,
		TableSlug: tableSlug,
		RoleId:    roleIdFromToken,
	})
	if err != nil {
		return "", nil, errors.Wrap(err, "when get recordPermission")
	}

	if recordPermission.IsHaveCondition {
		params, err = helper.GetAutomaticFilter(ctx, models.GetAutomaticFilterRequest{
			Conn:            conn,
			Params:          params,
			RoleIdFromToken: roleIdFromToken,
			UserIdFromToken: userIdFromToken,
			TableSlug:       tableSlug,
		})
		if err != nil {
			return "", nil, errors.Wrap(err, "when get automatic filter")
		}
	}

	tableColumns, err := getBoardTableColumnSet(ctx, conn, tableSlug)
	if err != nil {
		return "", nil, err
	}

	whereClause, args := buildBoardWhereClauseFromParams(params, tableColumns, search, viewFields, startParamIdx)
	return whereClause, args, nil
}

func getBoardTableColumnSet(ctx context.Context, conn *psqlpool.Pool, tableSlug string) (map[string]bool, error) {
	query := `
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = current_schema() AND table_name = $1
	`

	rows, err := conn.Query(ctx, query, tableSlug)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tableColumns := map[string]bool{}
	for rows.Next() {
		var columnName string
		if err := rows.Scan(&columnName); err != nil {
			return nil, err
		}
		tableColumns[columnName] = true
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return tableColumns, nil
}

func buildBoardWhereClauseFromParams(params map[string]any, tableColumns map[string]bool, search string, viewFields []string, startParamIdx int) (string, []any) {
	var (
		whereBuilder strings.Builder
		args         []any
		paramIdx     = startParamIdx
	)

	whereBuilder.WriteString("a.deleted_at IS NULL")

	for _, key := range sortedBoardParamKeys(params) {
		value := params[key]
		if isBoardServiceField(key) || !tableColumns[key] {
			continue
		}

		switch valueTyped := value.(type) {
		case []any:
			whereBuilder.WriteString(fmt.Sprintf(" AND a.%s::TEXT = ANY($%d::TEXT[])", pq.QuoteIdentifier(key), paramIdx))
			args = append(args, pq.Array(cast.ToStringSlice(valueTyped)))
			paramIdx++
		case []string:
			whereBuilder.WriteString(fmt.Sprintf(" AND a.%s::TEXT = ANY($%d::TEXT[])", pq.QuoteIdentifier(key), paramIdx))
			args = append(args, pq.Array(valueTyped))
			paramIdx++
		}
	}

	autoFilters := cast.ToStringMap(params["auto_filter"])
	autoFilterKeys := make([]string, 0, len(autoFilters))
	for key := range autoFilters {
		if tableColumns[key] {
			autoFilterKeys = append(autoFilterKeys, key)
		}
	}
	sort.Strings(autoFilterKeys)
	if len(autoFilterKeys) > 0 {
		whereBuilder.WriteString(" AND (")
		for idx, key := range autoFilterKeys {
			if idx > 0 {
				whereBuilder.WriteString(" OR ")
			}
			whereBuilder.WriteString(fmt.Sprintf("a.%s::TEXT = $%d::TEXT", pq.QuoteIdentifier(key), paramIdx))
			args = append(args, cast.ToString(autoFilters[key]))
			paramIdx++
		}
		whereBuilder.WriteString(")")
	}

	if search != "" {
		searchFields := make([]string, 0, len(viewFields))
		for _, field := range viewFields {
			if tableColumns[field] {
				searchFields = append(searchFields, field)
			}
		}
		if len(searchFields) > 0 {
			whereBuilder.WriteString(" AND (")
			for idx, field := range searchFields {
				if idx > 0 {
					whereBuilder.WriteString(" OR ")
				}
				whereBuilder.WriteString(fmt.Sprintf("a.%s::TEXT ~* $%d::TEXT", pq.QuoteIdentifier(field), paramIdx))
			}
			whereBuilder.WriteString(")")
			args = append(args, search)
		}
	}

	return whereBuilder.String(), args
}

func buildBoardCountQuery(tableSlug, whereClause string) string {
	return fmt.Sprintf(`SELECT COUNT(*) FROM %s a WHERE %s`, pq.QuoteIdentifier(tableSlug), whereClause)
}

func buildBoardDataQuery(tableSlug, selectClause, whereClause, orderBy string) string {
	return fmt.Sprintf(`
			SELECT
				%s
			FROM %s a
			WHERE %s
			ORDER BY a.%s DESC, a.board_order ASC, a.created_at DESC
			OFFSET $1::INT
			LIMIT $2::INT
		`,
		selectClause,
		pq.QuoteIdentifier(tableSlug),
		whereClause,
		pq.QuoteIdentifier(orderBy),
	)
}

func sortedBoardParamKeys(params map[string]any) []string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func isBoardServiceField(field string) bool {
	switch field {
	case "tables",
		"fields",
		"view_fields",
		"group_by",
		"subgroup_by",
		"limit",
		"offset",
		"search",
		"role_id_from_token",
		"user_id_from_token",
		"client_type_id_from_token",
		"auto_filter":
		return true
	default:
		return false
	}
}

func (o *objectBuilderRepo) UserActivity(ctx context.Context, req *nb.UserActivityReqeust) error {

	if req.GetLoginTable() == config.USER {
		return nil
	}

	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return helper.HandleDatabaseError(err, o.logger, "UserActivity: Failed to connect to db")
	}

	var (
		attributes    []byte
		attributesMap map[string]any
		now           = time.Now()

		query = `SELECT attributes FROM "table" WHERE slug = $1`
	)

	err = conn.QueryRow(ctx, query, req.GetLoginTable()).Scan(&attributes)
	if err != nil {
		return helper.HandleDatabaseError(err, o.logger, "UserActivity: Failed to get table attributes")
	}

	err = json.Unmarshal(attributes, &attributesMap)
	if err != nil {
		return helper.HandleDatabaseError(err, o.logger, "UserActivity: Failed to unmarshal attributes")
	}

	if cast.ToBool(attributesMap[config.LAST_ACTIVITY]) {

		query = fmt.Sprintf(`UPDATE "%s" SET %s = $2 WHERE guid = $1`, req.GetLoginTable(), config.LAST_ACTIVITY)
		_, err = conn.Exec(ctx, query, req.GetUserId(), now)
		if err != nil {
			return helper.HandleDatabaseError(err, o.logger, "UserActivity: Failed to update last activity")
		}
	}

	return nil
}

func (o *objectBuilderRepo) GetResourceUsage(ctx context.Context, req *nb.GetResourceUsageRequest) (*nb.GetResourceUsageResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "object_builder.GetResourceUsage")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	resp := &nb.GetResourceUsageResponse{}

	// 1. Functions count
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM "function" WHERE type = 'KNATIVE' AND deleted_at IS NULL`).Scan(&resp.FunctionsCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count functions: %w", err)
	}

	// 2. Microfrontends count
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM "function" WHERE type = 'MICRO_FRONTEND' AND deleted_at IS NULL`).Scan(&resp.MicrofrontendsCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count microfrontends: %w", err)
	}

	// 3. Asset size
	err = conn.QueryRow(ctx, `SELECT COALESCE(SUM(file_size), 0) FROM "file" WHERE deleted_at IS NULL`).Scan(&resp.AssetSize)
	if err != nil {
		return nil, fmt.Errorf("failed to sum asset size: %w", err)
	}

	// 4. Database size
	err = conn.QueryRow(ctx, `SELECT pg_database_size(current_database())`).Scan(&resp.DatabaseSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get database size: %w", err)
	}

	// 5. Items count (approximate but fast)
	err = conn.QueryRow(ctx, `SELECT COALESCE(SUM(n_live_tup), 0) FROM pg_stat_user_tables WHERE schemaname = 'public'`).Scan(&resp.ItemsCount)
	if err != nil {
		o.logger.Error("failed to get items count from pg_stat_user_tables", logger.Error(err))
		resp.ItemsCount = 0
		err = nil
	}

	// 6. Tables count
	err = conn.QueryRow(ctx, `SELECT COUNT(*) FROM "table" WHERE deleted_at IS NULL`).Scan(&resp.TablesCount)
	if err != nil {
		return nil, fmt.Errorf("failed to count tables: %w", err)
	}

	return resp, nil
}

func (o *objectBuilderRepo) ExecuteSQL(ctx context.Context, req *nb.ExecuteSQLRequest) (*nb.ExecuteSQLResponse, error) {
	conn, err := psqlpool.Get(req.GetResourceEnvId())
	if err != nil {
		return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> psqlpool.Get] Ошибка подключения к БД: %v", err)}, nil
	}

	var (
		args = make([]any, len(req.GetParams()))
		tx   pgx.Tx
		rows pgx.Rows
	)
	for i, p := range req.GetParams() {
		args[i] = p
	}

	if req.GetInTransaction() {
		tx, err = conn.Begin(ctx)
		if err != nil {
			return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> Begin] Не удалось начать транзакцию: %v", err)}, nil
		}

		defer func() {
			if txErr := tx.Rollback(ctx); txErr != nil && txErr != pgx.ErrTxClosed {
				o.logger.Error("ExecuteSQL: Failed to rollback transaction", logger.Error(txErr))
			}
		}()
	}

	if tx != nil {
		rows, err = tx.Query(ctx, req.GetSql(), args...)
	} else {
		rows, err = conn.Query(ctx, req.GetSql(), args...)
	}

	if err != nil {
		return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> Query] Ошибка выполнения SQL: %v | Query: %s", err, req.GetSql())}, nil
	}
	defer rows.Close()

	var pgOIDNames = map[uint32]string{
		16: "bool", 17: "bytea", 18: "char", 20: "int8", 21: "int2", 23: "int4",
		25: "text", 26: "oid", 114: "json", 142: "xml", 700: "float4", 701: "float8",
		869: "inet", 1042: "bpchar", 1043: "varchar", 1082: "date", 1083: "time",
		1114: "timestamp", 1184: "timestamptz", 1186: "interval", 1700: "numeric",
		2950: "uuid", 3802: "jsonb",
	}

	var (
		resultRows   []*structpb.Struct
		rowsAffected int64

		fields = rows.FieldDescriptions()
		types  = make(map[string]string, len(fields))
	)

	for _, field := range fields {
		if name, ok := pgOIDNames[field.DataTypeOID]; ok {
			types[field.Name] = name
		} else {
			types[field.Name] = "unknown"
		}
	}

	for rows.Next() {
		rowsAffected++

		vals, err := rows.Values()
		if err != nil {
			return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> rows.Values] Ошибка чтения значений строки: %v", err)}, nil
		}

		rowMap := make(map[string]any)

		for i, field := range fields {
			colName := field.Name
			val := vals[i]

			if val == nil {
				rowMap[colName] = nil
				continue
			}

			switch v := val.(type) {
			case time.Time:
				rowMap[colName] = v.Format(time.RFC3339)
			case [16]byte:
				rowMap[colName] = uuid.UUID(v).String()
			case []byte:
				rowMap[colName] = string(v)
			default:
				rowMap[colName] = v
			}
		}

		structPb, err := structpb.NewStruct(rowMap)
		if err != nil {
			return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> structpb.NewStruct] Ошибка конвертации данных в protobuf: %v", err)}, nil
		}

		resultRows = append(resultRows, structPb)
	}

	if err = rows.Err(); err != nil {
		return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> rows.Err] Ошибка при итерации результата: %v", err)}, nil
	}

	commandTag := rows.CommandTag()
	if commandTag.RowsAffected() > 0 && rowsAffected == 0 {
		rowsAffected = commandTag.RowsAffected()
	}

	if tx != nil {
		if err := tx.Commit(ctx); err != nil {
			return &nb.ExecuteSQLResponse{Error: fmt.Sprintf("[ExecuteSQL -> Commit] Не удалось закоммитить транзакцию: %v", err)}, nil
		}
	}

	return &nb.ExecuteSQLResponse{
		Rows:         resultRows,
		RowsAffected: rowsAffected,
		Types:        types,
	}, nil
}

func executeSelect(params models.QueryParams, sb squirrel.StatementBuilderType) (string, []any, error) {
	query := sb.Select(params.Columns...).From(params.Table)

	for _, join := range params.Joins {
		switch join.Type {
		case "LEFT":
			query = query.LeftJoin(join.Table + " ON " + join.Condition)
		case "RIGHT":
			query = query.RightJoin(join.Table + " ON " + join.Condition)
		case "INNER":
			query = query.InnerJoin(join.Table + " ON " + join.Condition)
		default:
			query = query.Join(join.Table + " ON " + join.Condition)
		}
	}

	if params.Where != "" {
		query = query.Where(params.Where)
	}

	if len(params.GroupBy) > 0 {
		query = query.GroupBy(params.GroupBy...)
	}

	if params.Having != "" {
		query = query.Having(params.Having)
	}

	if len(params.OrderBy) > 0 {
		query = query.OrderBy(params.OrderBy...)
	}

	if params.Limit > 0 {
		query = query.Limit(params.Limit)
	}
	if params.Offset > 0 {
		query = query.Offset(params.Offset)
	}

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil, err
	}

	return sql, args, nil
}

func executeUpdate(params models.QueryParams, sb squirrel.StatementBuilderType) (string, []any, error) {
	if len(params.Data) == 0 {
		return "", nil, errors.New("no data provided for update")
	}

	query := sb.Update(params.Table).SetMap(params.Data)

	if params.Where != "" {
		query = query.Where(params.Where)
	}

	query = query.Suffix("RETURNING " + strings.Join(params.Columns, ", "))

	sql, args, err := query.ToSql()
	if err != nil {
		return "", nil, err
	}

	return sql, args, nil
}

func toStringSlice(value any, noGroupValue string) []string {
	switch v := value.(type) {
	case string:
		if v == "" {
			return []string{noGroupValue}
		}
		return []string{v}
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return []string{fmt.Sprint(v)}
	case []any:
		slice := cast.ToStringSlice(v)
		if len(slice) == 0 {
			return []string{noGroupValue}
		}
		return slice
	default:
		return []string{noGroupValue}
	}
}

func (o *objectBuilderRepo) GetTableSchema(ctx context.Context, req *nb.CommonMessage) (*nb.CommonMessage, error) {
	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	// Constraints per column: map attnum -> list of constraints
	// For FK constraints also fetch referenced table and first referenced column
	conRows, err := conn.Query(ctx, `
		SELECT
			c.conname,
			c.contype::text,
			pg_get_constraintdef(c.oid) AS definition,
			c.conkey,
			COALESCE(ref.relname, '')                                                                          AS ref_table,
			COALESCE((SELECT a.attname FROM pg_attribute a WHERE a.attrelid = c.confrelid AND a.attnum = c.confkey[1] LIMIT 1), '') AS ref_column
		FROM pg_constraint c
		LEFT JOIN pg_class ref ON ref.oid = c.confrelid
		WHERE c.conrelid = $1::regclass`,
		req.TableSlug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error querying constraints")
	}

	type constraintInfo struct {
		Name  string
		Label string
	}
	// attnum is 1-based; conkey can list multiple columns for composite constraints
	colConstraints := make(map[int16][]constraintInfo)
	var allConstraints []any
	for conRows.Next() {
		var name, ctype, def, refTable, refColumn string
		var conkey []int16
		if err := conRows.Scan(&name, &ctype, &def, &conkey, &refTable, &refColumn); err != nil {
			conRows.Close()
			return nil, errors.Wrap(err, "error scanning constraint")
		}

		var label string
		switch ctype {
		case "p":
			label = "PK"
		case "f":
			label = "FK -> " + refTable + "." + refColumn
		case "u":
			label = "UNIQUE"
		case "c":
			label = "CHECK"
		default:
			label = def
		}

		allConstraints = append(allConstraints, map[string]any{
			"name":  name,
			"label": label,
		})
		ci := constraintInfo{Name: name, Label: label}
		for _, attnum := range conkey {
			colConstraints[attnum] = append(colConstraints[attnum], ci)
		}
	}
	conRows.Close()

	// Columns — same output as \d: full type string, nullable, default
	// Also fetch attnum so we can look up per-column constraints
	colRows, err := conn.Query(ctx, `
		SELECT
			a.attnum,
			a.attname                                                      AS column_name,
			pg_catalog.format_type(a.atttypid, a.atttypmod)               AS data_type,
			CASE WHEN a.attnotnull THEN 'NO' ELSE 'YES' END                AS is_nullable,
			pg_catalog.pg_get_expr(d.adbin, d.adrelid)                    AS column_default,
			COALESCE(f.id::text, '')                                       AS field_id
		FROM pg_catalog.pg_attribute a
		LEFT JOIN pg_catalog.pg_attrdef d ON a.attrelid = d.adrelid AND a.attnum = d.adnum
		LEFT JOIN "field" f ON f.slug = a.attname
			AND f.table_id = (SELECT id FROM "table" WHERE slug = $1 LIMIT 1)
		WHERE a.attrelid = $1::regclass
		  AND a.attnum > 0
		  AND NOT a.attisdropped
		ORDER BY a.attnum`,
		req.TableSlug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error querying columns")
	}
	defer colRows.Close()

	var columns []any
	for colRows.Next() {
		var attnum int16
		var name, dataType, nullable string
		var colDefault *string
		var fieldID string
		if err := colRows.Scan(&attnum, &name, &dataType, &nullable, &colDefault, &fieldID); err != nil {
			return nil, errors.Wrap(err, "error scanning column")
		}

		var colCons []any
		for _, ci := range colConstraints[attnum] {
			colCons = append(colCons, map[string]any{
				"name":  ci.Name,
				"label": ci.Label,
			})
		}

		columns = append(columns, map[string]any{
			"id":          fieldID,
			"name":        name,
			"type":        dataType,
			"nullable":    nullable,
			"default":     colDefault,
			"constraints": colCons,
		})
	}

	// Indexes
	idxRows, err := conn.Query(ctx, `
		SELECT indexname, indexdef
		FROM pg_indexes
		WHERE tablename = $1 AND schemaname = 'public'`,
		req.TableSlug,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error querying indexes")
	}
	defer idxRows.Close()

	var indexes []any
	for idxRows.Next() {
		var name, def string
		if err := idxRows.Scan(&name, &def); err != nil {
			return nil, errors.Wrap(err, "error scanning index")
		}
		indexes = append(indexes, map[string]any{
			"name":       name,
			"definition": def,
		})
	}

	response := map[string]any{
		"table_name":  req.TableSlug,
		"columns":     columns,
		"indexes":     indexes,
		"constraints": allConstraints,
	}

	responseStruct, err := helper.ConvertMapToStruct(response)
	if err != nil {
		return nil, errors.Wrap(err, "error converting to struct")
	}

	return &nb.CommonMessage{Data: responseStruct}, nil
}

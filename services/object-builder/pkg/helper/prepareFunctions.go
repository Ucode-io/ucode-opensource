package helper

import (
	"context"
	"database/sql"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/security"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/util"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
)

func PrepareToCreateInObjectBuilder(ctx context.Context, conn pgx.Tx, req *nb.CommonMessage, reqBody models.CreateBody) (map[string]any, []map[string]any, error) {
	var (
		fieldM     = reqBody.FieldMap
		tableSlugs = reqBody.TableSlugs
		fields     = reqBody.Fields
	)

	data, err := ConvertStructToMap(req.Data)
	if err != nil {
		return map[string]any{}, []map[string]any{}, err
	}

	response := data

	// * RANDOM_NUMBER
	{
		randomNumbers, ok := fieldM["RANDOM_NUMBERS"]
		if ok {
			for {
				randNum := GenerateRandomNumber(cast.ToString(randomNumbers.Attributes["prefix"]), cast.ToInt(randomNumbers.Attributes["digit_number"]))

				isExists, err := IsExistsWithTx(ctx, conn, models.IsExistsBody{TableSlug: req.TableSlug, FieldSlug: randomNumbers.Slug, FieldValue: randNum})
				if err != nil {
					return map[string]any{}, []map[string]any{}, err
				}

				if !isExists {
					response[randomNumbers.Slug] = randNum
					break
				}
			}

		}
	}

	// * RANDOM_TEXT
	{
		randomText, ok := fieldM["RANDOM_TEXT"]
		if ok {
			for {
				randText := GenerateRandomString(cast.ToString(randomText.Attributes["prefix"]), cast.ToInt(randomText.Attributes["digit_number"]))
				isExists, err := IsExistsWithTx(ctx, conn, models.IsExistsBody{TableSlug: req.TableSlug, FieldSlug: randomText.Slug, FieldValue: randText})
				if err != nil {
					return map[string]any{}, []map[string]any{}, err
				}

				if randText != "" {
					if !isExists {
						response[randomText.Slug] = randText
						break
					}
				}
			}

		}
	}

	// * RANDOM_UUID
	{
		randomUuid, ok := fieldM["RANDOM_UUID"]
		if ok {
			response[randomUuid.Slug] = uuid.NewString()
		}
	}

	// * MANUAL_STRING
	{
		manual, ok := fieldM["MANUAL_STRING"]
		if ok {
			fields, err := ConvertStructToMap(req.Data)
			if err != nil {
				return map[string]any{}, []map[string]any{}, err
			}

			text := cast.ToString(manual.Attributes["formula"])

			for _, slug := range tableSlugs {
				var (
					value any
				)

				switch v := fields[slug].(type) {
				case bool:
					value = strconv.FormatBool(v)
				case string:
					value = v
				case []any:
					if len(v) > 0 {
						if str, ok := v[0].(string); ok {
							value = str
						}
					}
				case int, float64:
					value = v
				default:
					value = v
				}

				text = strings.ReplaceAll(text, slug, cast.ToString(value))
			}

			response[manual.Slug] = text
		}
	}

	// * INCREMENT_ID
	{
		incrementField, ok := fieldM["INCREMENT_ID"]
		if ok {
			var incrementBy, maxValue int

			query := `UPDATE "incrementseqs" SET increment_by = increment_by + 1  WHERE table_slug = $1 AND field_slug = $2 RETURNING increment_by AS old_value, max_value`

			err = conn.QueryRow(ctx, query, req.TableSlug, incrementField.Slug).Scan(&incrementBy, &maxValue)
			if err != nil {
				return map[string]any{}, []map[string]any{}, err
			}

			width := len(strconv.Itoa(maxValue))
			value := fmt.Sprintf("%0*d", width, incrementBy)

			prefix := cast.ToString(incrementField.Attributes["prefix"])
			if len(prefix) > 0 {
				response[incrementField.Slug] = cast.ToString(incrementField.Attributes["prefix"]) + "-" + value
			} else {
				response[incrementField.Slug] = value
			}
		}
	}

	// * INCREMENT_NUMBER
	{
		incrementNum, ok := fieldM["INCREMENT_NUMBER"]
		if ok {
			delete(response, incrementNum.Slug)
		}
	}

	{
		if password, ok := fieldM["PASSWORD"]; ok {
			if passwordData, ok := data[password.Slug]; ok {
				err = util.ValidStrongPassword(cast.ToString(passwordData))
				if err != nil {
					return map[string]any{}, []map[string]any{}, err
				}
				hashedPassword, err := security.HashPasswordBcrypt(cast.ToString(passwordData))
				if err != nil {
					return map[string]any{}, []map[string]any{}, err
				}

				response[password.Slug] = hashedPassword
			}
		}
	}

	// * AUTOFILL
	{
		for _, field := range fields {
			attributes, err := ConvertStructToMap(field.Attributes)
			if err != nil {
				return map[string]any{}, []map[string]any{}, err
			}

			if field.AutofillField != "" && field.AutofillTable != "" {

				splitArr := strings.Split(field.AutofillTable, "#")

				slug := splitArr[0]
				if !strings.Contains(slug, "_id") {
					slug += "_id"
				}

				if IsEmpty(response[slug]) {
					continue
				}

				var (
					query    = fmt.Sprintf(`SELECT %s FROM "%s" WHERE guid = '%s'`, field.AutofillField, splitArr[0], response[slug])
					autofill any
				)

				if err = conn.QueryRow(ctx, query).Scan(&autofill); err != nil {
					return map[string]any{}, []map[string]any{}, err
				}

				if autofill != nil {
					response[field.Slug] = autofill
					continue
				}
			}

			_, ok := response[field.Slug]
			ok2 := !IsEmpty(response[field.Slug])

			defaultValues := cast.ToSlice(attributes["default_values"])
			ftype := FIELD_TYPES[field.Type]
			if ok2 && !ok {
				if ftype == "FLOAT" {
					response[field.Slug] = cast.ToInt(attributes["defaultValue"])
				} else if field.Type == "DATE_TIME" || field.Type == "DATE" {
					response[field.Slug] = time.Now().Format(time.RFC3339)
				} else if field.Type == "SWITCH" {
					defaultValue := strings.ToLower(cast.ToString(attributes["defaultValue"]))

					switch defaultValue {
					case "true":
						response[field.Slug] = true
					case "false":
						response[field.Slug] = false
					}
				} else if ftype == "TEXT[]" {
					response[field.Slug] = "{}"
				} else if field.Type == "FORMULA_FRONTEND" {
					continue
				} else {
					response[field.Slug] = attributes["defaultValue"]
				}
			} else if len(defaultValues) > 0 && !ok {
				response[field.Slug] = defaultValues[0]
			} else if field.Type == "FORMULA_FRONTEND" {
				response[field.Slug] = cast.ToString(response[field.Slug])
			} else if IsEmpty(response[field.Slug]) {
				delete(response, field.Slug)
			}
		}
	}

	query := `SELECT table_to, table_from FROM "relation" WHERE id = $1`

	appendMany2ManyObjects := []map[string]any{}
	// * AppendMany2ManyObjects
	{
		for _, field := range fields {
			var (
				tableTo, tableFrom string
			)

			if field.Type == "LOOKUPS" {
				if strings.Contains(field.Slug, "_ids") {

					_, ok := response[field.Slug]
					if ok {
						err = conn.QueryRow(ctx, query, field.RelationId).Scan(&tableTo, &tableFrom)
						if err != nil {
							return map[string]any{}, []map[string]any{}, err
						}

						// appendMany2Many := make(map[string]any)

						appendMany2Many := map[string]any{
							"project_id": req.ProjectId,
							"id_from":    response["guid"],
							"id_to":      response[field.Slug],
							"table_from": req.TableSlug,
						}
						if tableTo == req.TableSlug {
							appendMany2Many["table_to"] = tableFrom
						} else if tableFrom == req.TableSlug {
							appendMany2Many["table_to"] = tableTo
						}

						appendMany2ManyObjects = append(appendMany2ManyObjects, appendMany2Many)
					}
				}
			}
		}
	}

	return response, appendMany2ManyObjects, nil
}

func PrepareToUpdateInObjectBuilder(ctx context.Context, req *nb.CommonMessage, conn pgx.Tx) (map[string]any, map[string]any, error) {
	data, err := ConvertStructToMap(req.Data)
	if err != nil {
		return map[string]any{}, nil, errors.Wrap(err, "pgx.Tx ConvertStructToMap")
	}

	oldData, err := GetItemWithTx(ctx, conn, req.TableSlug, cast.ToString(data["guid"]), false)
	if err != nil {
		return map[string]any{}, nil, errors.Wrap(err, "")
	}

	var (
		fieldTypes      = make(map[string]string)
		dataToAnalytics = make(map[string]any)
	)

	query := `SELECT 
			f."id", 
			f."type", 
			f."attributes", 
			f."slug", 
			f."relation_id", 
			f."required" 
		FROM "field" AS f 
		JOIN "table" AS t ON t.id = f.table_id 
		WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return map[string]any{}, nil, errors.Wrap(err, "")
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			field      = models.Field{}
			attributes = []byte{}
			relationId sql.NullString
		)

		err = fieldRows.Scan(
			&field.Id,
			&field.Type,
			&attributes,
			&field.Slug,
			&relationId,
			&field.Required,
		)
		if err != nil {
			return map[string]any{}, nil, errors.Wrap(err, "error in fieldRows.Scan")
		}

		fType := FIELD_TYPES[field.Type]
		fieldTypes[field.Slug] = fType

		switch field.Type {
		case "LOOKUPS":
			var deletedIds []string

			if _, ok := data[field.Slug]; ok {
				olderArr := cast.ToStringSlice(oldData[field.Slug])
				newArr := cast.ToStringSlice(data[field.Slug])

				if len(newArr) > 0 {
					for _, oldVal := range olderArr {
						var found bool
						if slices.Contains(newArr, oldVal) {
							found = true
						}
						if !found && !Contains(deletedIds, oldVal) {
							deletedIds = append(deletedIds, oldVal)
						}
					}
				}
			}

			dataToAnalytics[field.Slug] = data[field.Slug]
		case "MULTISELECT":
			val, ok := data[field.Slug]
			if field.Required && (!ok || len(cast.ToSlice(val)) == 0) {
				return map[string]any{}, nil, errors.Wrap(err, "error")
			}
		}
	}

	fieldTypes["guid"] = "String"

	return data, oldData, nil
}

func PrepareToUpdateInObjectBuilderFromAuth(ctx context.Context, req *nb.CommonMessage, conn pgx.Tx) (map[string]any, error) {
	data, err := ConvertStructToMap(req.Data)
	if err != nil {
		return map[string]any{}, errors.Wrap(err, "pgx.Tx ConvertStructToMap")
	}

	oldData, err := GetItemWithTx(ctx, conn, req.TableSlug, cast.ToString(data["id"]), true)
	if err != nil {
		return map[string]any{}, errors.Wrap(err, "")
	}

	var (
		fieldTypes      = make(map[string]string)
		dataToAnalytics = make(map[string]any)
	)

	query := `SELECT f."id", f."type", f."attributes", f."slug", f."relation_id", f."required" FROM "field" as f JOIN "table" as t ON t.id = f.table_id WHERE t.slug = $1`

	fieldRows, err := conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return map[string]any{}, errors.Wrap(err, "")
	}
	defer fieldRows.Close()

	for fieldRows.Next() {
		var (
			field      = models.Field{}
			attributes = []byte{}
			relationId sql.NullString
		)

		err = fieldRows.Scan(
			&field.Id,
			&field.Type,
			&attributes,
			&field.Slug,
			&relationId,
			&field.Required,
		)
		if err != nil {
			return map[string]any{}, errors.Wrap(err, "error in fieldRows.Scan")
		}

		fType := FIELD_TYPES[field.Type]
		fieldTypes[field.Slug] = fType

		if field.Type == "LOOKUPS" {
			var deletedIds []string

			if _, ok := data[field.Slug]; ok {
				olderArr := cast.ToStringSlice(oldData[field.Slug])
				newArr := cast.ToStringSlice(data[field.Slug])

				if len(newArr) > 0 {
					for _, val := range newArr {
						for _, oldVal := range olderArr {
							if val == oldVal {
								break
							}
						}
					}

					for _, oldVal := range olderArr {
						var found bool
						for _, val := range newArr {
							if oldVal == val {
								found = true
								break
							}
						}
						if !found && !Contains(deletedIds, oldVal) {
							deletedIds = append(deletedIds, oldVal)
						}
					}
				}
			}

			dataToAnalytics[field.Slug] = data[field.Slug]
		} else if field.Type == "MULTISELECT" {
			val, ok := data[field.Slug]
			if field.Required && (!ok || len(cast.ToSlice(val)) == 0) {
				return map[string]any{}, errors.Wrap(err, "error")
			}
		}
	}

	fieldTypes["guid"] = "String"

	return data, nil
}

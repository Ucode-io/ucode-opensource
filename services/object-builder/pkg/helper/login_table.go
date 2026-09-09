package helper

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/spf13/cast"
)

func UpsertLoginTableField(ctx context.Context, req models.Field) (string, error) {
	var (
		tx                 = req.Tx
		fieldId, fieldType string
		query              = `SELECT id, type FROM field where slug = $1 AND table_id = $2`
	)

	attributes, err := json.Marshal(req.Attributes)
	if err != nil {
		return fieldId, err
	}

	err = tx.QueryRow(ctx, query, req.Slug, req.TableId).Scan(&fieldId, &fieldType)
	if err != nil && err != pgx.ErrNoRows {
		return fieldId, err
	} else if err == pgx.ErrNoRows {
		fieldId = uuid.NewString()

		query = `INSERT INTO "field" (
			id,
			"table_id",
			"required",
			"slug",
			"label",
			"default",
			"type",
			"attributes",
			"index"
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

		_, err = tx.Exec(ctx, query, fieldId, req.TableId, req.Required, req.Slug, req.Label, req.Default, req.Type, attributes, req.Index)
		if err != nil {
			return fieldId, err
		}

		var (
			body                                  []byte
			ids, valueStrings                     []string
			values                                []any
			tableSlug, layoutId, tabId, sectionId string
			sectionCount                          int32
			fields                                = []models.SectionFields{}
		)

		tableSlug = req.TableSlug

		query = fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", tableSlug, req.Slug, GetDataType(req.Type))

		_, err = tx.Exec(ctx, query)
		if err != nil {
			return fieldId, err
		}

		query = `SELECT guid FROM "role"`

		rows, err := tx.Query(ctx, query)
		if err != nil {
			return fieldId, err
		}

		defer rows.Close()

		for rows.Next() {
			var id string

			err = rows.Scan(&id)
			if err != nil {
				return fieldId, err
			}

			ids = append(ids, id)
		}

		query = `INSERT INTO "field_permission" (
    		"edit_permission",
    		"view_permission",
    		"table_slug",
    		"field_id",
    		"label",
    		"role_id"
		) VALUES 
		`

		for i, id := range ids {
			valueStrings = append(valueStrings, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
				1+i*6, 2+i*6, 3+i*6, 4+i*6, 5+i*6, 6+i*6))

			// Add the corresponding values for each column
			values = append(values, true, true, tableSlug, fieldId, req.Label, id)
		}

		query += strings.Join(valueStrings, ", ")

		_, err = tx.Exec(ctx, query, values...)
		if err != nil {
			return fieldId, err
		}

		query = `SELECT id FROM "layout" WHERE table_id = $1`
		err = tx.QueryRow(ctx, query, req.TableId).Scan(&layoutId)
		if err != nil && err != pgx.ErrNoRows {
			return fieldId, err
		} else if err == pgx.ErrNoRows {
			return fieldId, nil
		}

		query = `SELECT id FROM "tab" WHERE "layout_id" = $1 and type = 'section'`
		err = tx.QueryRow(ctx, query, layoutId).Scan(&tabId)
		if err != nil && err != pgx.ErrNoRows {
			return fieldId, err
		} else if err == pgx.ErrNoRows {
			return fieldId, nil
		}

		query = `SELECT id, fields FROM "section" WHERE tab_id = $1 ORDER BY created_at DESC LIMIT 1`
		err = tx.QueryRow(ctx, query, tabId).Scan(&sectionId, &body)
		if err != nil {
			return fieldId, err
		} else if err == pgx.ErrNoRows {
			return fieldId, nil
		}

		queryCount := `SELECT COUNT(*) FROM "section" WHERE tab_id = $1`
		err = tx.QueryRow(ctx, queryCount, tabId).Scan(&sectionCount)
		if err != nil && err != pgx.ErrNoRows {
			return fieldId, err
		} else if err == pgx.ErrNoRows {
			return fieldId, nil
		}

		if err := json.Unmarshal(body, &fields); err != nil {
			return fieldId, err
		}

		if len(fields) < 3 {
			query = `UPDATE "section" SET fields = $2 WHERE id = $1`

			fields = append(fields, models.SectionFields{
				Id:    fieldId,
				Order: len(fields) + 1,
			})

			reqBody, err := json.Marshal(fields)
			if err != nil {
				return fieldId, err
			}

			_, err = tx.Exec(ctx, query, sectionId, reqBody)
			if err != nil {
				return fieldId, err
			}
		} else {
			query = `INSERT INTO "section" ("order", "column", label, table_id, tab_id, fields) VALUES ($1, $2, $3, $4, $5, $6)`

			sectionId = uuid.NewString()

			fields := []models.SectionFields{{Id: fieldId, Order: 1}}

			reqBody, err := json.Marshal(fields)
			if err != nil {
				return fieldId, err
			}

			_, err = tx.Exec(ctx, query, sectionCount+1, "SINGLE", "Info", req.TableId, tabId, reqBody)
			if err != nil {
				return fieldId, err
			}
		}

		return fieldId, err
	}

	query = `UPDATE "field" SET
		"default" = $2,
		"label" = $3,
		"required" = $4,
		"type" = $5,
		"attributes" = $6,
		"index" = $7
	WHERE id = $1
	`

	_, err = tx.Exec(ctx, query, fieldId, req.Default, req.Label, req.Required, req.Type, attributes, req.Index)
	if err != nil {
		return fieldId, err
	}

	if req.Type != fieldType {
		query = fmt.Sprintf(`ALTER TABLE "%s" DROP COLUMN %s`, req.TableSlug, req.Slug)

		_, err = tx.Exec(ctx, query)
		if err != nil {
			return fieldId, err
		}

		fieldType := GetDataType(req.Type)

		query = fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN %s %s`, req.TableSlug, req.Slug, fieldType)

		_, err = tx.Exec(ctx, query)
		if err != nil {
			return fieldId, err
		}
	}

	return fieldId, nil
}

func GetLoginStrategyMap(ctx context.Context, oldAttrinutes []byte) map[string]string {
	var attributes map[string]any
	var loginStrategyMap = map[string]string{}

	if err := json.Unmarshal(oldAttrinutes, &attributes); err != nil {
		return loginStrategyMap
	}

	authInfo, ok := attributes["auth_info"].(map[string]any)
	if !ok {
		return loginStrategyMap
	}

	loginStrategy, ok := authInfo["login_strategy"].([]any)
	if !ok {
		return loginStrategyMap
	}

	for _, value := range loginStrategy {
		strategy := cast.ToString(value)
		loginStrategyMap[strategy] = cast.ToString(authInfo[strategy])
	}

	return loginStrategyMap
}

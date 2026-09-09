package helper

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"

	"github.com/google/uuid"
	"google.golang.org/protobuf/types/known/structpb"
)

func BoardOrderChecker(ctx context.Context, req models.BoardOrder) error {
	var (
		tableId      string
		boardOrderId string

		tableSelectQuery string
		fieldSelectQuery string
		fieldInsertQuery string

		now = time.Now()
	)

	tableSelectQuery = `SELECT id FROM "table" WHERE slug = $1`
	err := req.Tx.QueryRow(ctx, tableSelectQuery, req.TableSlug).Scan(&tableId)
	if err != nil {
		return err
	}

	fieldSelectQuery = `SELECT id FROM "field" WHERE table_id = $1 AND "slug" = 'board_order'`
	err = req.Tx.QueryRow(ctx, fieldSelectQuery, tableId).Scan(&boardOrderId)

	if err != nil {
		if strings.Contains(err.Error(), "no rows") {

			attributes := &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"icon":        {Kind: &structpb.Value_StringValue{StringValue: ""}},
					"placeholder": {Kind: &structpb.Value_StringValue{StringValue: ""}},
					"showTooltip": {Kind: &structpb.Value_StringValue{StringValue: ""}},
				},
			}
			attributesJson, err := json.Marshal(attributes)
			if err != nil {
				return err
			}

			fieldInsertQuery = `INSERT INTO "field" (id, table_id, required, slug, label, "default", "type", "index", attributes, is_visible, autofill_field, autofill_table, created_at, updated_at)
						  VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`

			_, err = req.Tx.Exec(
				ctx,
				fieldInsertQuery,
				uuid.NewString(),
				tableId,
				false,
				"board_order",
				"BOARD ORDER",
				"",
				"NUMBER",
				"string",
				attributesJson,
				false, "", "", now, now)
			if err != nil {
				return err
			}

			query := `ALTER TABLE ` + req.TableSlug + ` ADD COLUMN board_order ` + GetDataType("NUMBER")

			_, err = req.Tx.Exec(ctx, query)
			if err != nil {
				return err
			}

		} else {
			return err
		}
	}

	return nil
}

func GetViewWithPermission(ctx context.Context, req *models.GetViewWithPermissionReq) ([]*models.View, error) {
	query := `SELECT 
		"id",
		"attributes",
		"table_slug",
		"type"
	FROM "view" WHERE "table_slug" = $1`

	viewRows, err := req.Conn.Query(ctx, query, req.TableSlug)
	if err != nil {
		return []*models.View{}, err
	}
	defer viewRows.Close()

	views := []*models.View{}

	for viewRows.Next() {
		var (
			attributes []byte
			view       = &models.View{}
		)

		err := viewRows.Scan(
			&view.Id,
			&attributes,
			&view.TableSlug,
			&view.Type,
		)
		if err != nil {
			return []*models.View{}, err
		}

		if err := json.Unmarshal(attributes, &view.Attributes); err != nil {
			return []*models.View{}, err
		}

		views = append(views, view)
	}

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

		err = req.Conn.QueryRow(ctx, query, view.Id, req.RoleId).Scan(
			&vp.Guid,
			&vp.RoleId,
			&vp.ViewId,
			&vp.View,
			&vp.Edit,
			&vp.Delete,
		)
		if err != nil {
			return []*models.View{}, err
		}

		view.Attributes["view_permission"] = vp
	}

	return views, nil
}

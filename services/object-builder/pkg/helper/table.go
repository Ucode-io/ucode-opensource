package helper

import (
	"context"
	"fmt"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/jackc/pgx/v5"
	"github.com/spf13/cast"
	"github.com/google/uuid"
)

func TableVer(ctx context.Context, req models.TableVerReq) (map[string]any, error) {

	query := `SELECT 
			"id",
			"slug"
	 FROM "table" WHERE `

	value := req.Id

	if req.Id != "" {
		query += ` "id" = $1`
	} else if req.Slug != "" {
		query += ` "slug" = $1`
		value = req.Slug
	}

	err := req.Tx.QueryRow(ctx, query, value).Scan(&req.Id, &req.Slug)
	if err != nil {
		return map[string]any{}, err
	}

	return map[string]any{
		"id":   req.Id,
		"slug": req.Slug,
	}, nil

}

func GetTableByIdSlug(ctx context.Context, req models.GetTableByIdSlugReq) (map[string]any, error) {

	query := `SELECT id, slug, label FROM "table" WHERE `

	value := req.Id

	var label string

	if req.Id != "" {
		query += ` id = $1`
	} else if req.Slug != "" {
		query += ` slug = $1`
		value = req.Slug
	}

	err := req.Conn.QueryRow(ctx, query, value).Scan(&req.Id, &req.Slug, &label)
	if err != nil {
		return map[string]any{}, err
	}

	return map[string]any{
		"id":    req.Id,
		"slug":  req.Slug,
		"label": label,
	}, nil
}

func TableFindOne(ctx context.Context, conn *psqlpool.Pool, id string) (resp *nb.Table, err error) {
	var filter string = "id = $1"

	resp = &nb.Table{
		IncrementId: &nb.IncrementID{},
	}

	_, err = uuid.Parse(id)
	if err != nil {
		filter = "slug = $1"
	}

	query := `SELECT
		"id",
		"slug",
		"label",
		"section_column_count",
		"order_by"
	FROM "table" WHERE ` + filter

	err = conn.QueryRow(ctx, query, id).Scan(
		&resp.Id,
		&resp.Slug,
		&resp.Label,
		&resp.SectionColumnCount,
		&resp.OrderBy,
	)
	if err != nil {
		return nil, fmt.Errorf("error while finding single table: %v", err)
	}
	return resp, nil
}

func TableFindOneTx(ctx context.Context, tx pgx.Tx, id string) (resp *nb.Table, err error) {
	var filter string = "id = $1"

	resp = &nb.Table{
		IncrementId: &nb.IncrementID{},
	}

	_, err = uuid.Parse(id)
	if err != nil {
		filter = "slug = $1"
	}

	query := `SELECT
		"id",
		"slug",
		"label",
		"section_column_count"
	FROM "table" WHERE ` + filter

	err = tx.QueryRow(ctx, query, id).Scan(
		&resp.Id,
		&resp.Slug,
		&resp.Label,
		&resp.SectionColumnCount,
	)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

func FindOneTableFromParams(params []any, objectField string) map[string]any {
	for _, obj := range params {
		table := cast.ToStringMap(obj)
		if cast.ToString(table["table_slug"]) == objectField {
			return table
		}
	}
	return nil
}

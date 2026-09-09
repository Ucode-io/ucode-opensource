package models

import (
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"github.com/jackc/pgx/v5/pgxpool"
)

type SectionBody struct {
	Id     string
	Fields []map[string]any
}

type FieldBody struct {
	Slug       string
	Attributes map[string]any
}

type GetFieldBySlugReq struct {
	Conn    *pgxpool.Pool
	Slug    string
	TableId string
}

type AddPermissionToFieldRequest struct {
	Conn      *psqlpool.Pool
	Fields    []Field
	RoleId    string
	TableSlug string
}

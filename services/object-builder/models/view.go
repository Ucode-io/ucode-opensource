package models

import (
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BoardOrder struct {
	Tx        pgx.Tx `json:"-"`
	TableSlug string
}

type GetViewWithPermissionReq struct {
	Conn      *pgxpool.Pool
	TableSlug string
	RoleId    string
}

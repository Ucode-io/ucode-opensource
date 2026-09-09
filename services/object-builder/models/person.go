package models

import (
	pa "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/auth_service"

	"github.com/jackc/pgx/v5"
)

type CreateSyncWithLoginTableRequest struct {
	Tx                 pgx.Tx `json:"-"`
	Guid               string
	UserIdAuth         string
	LoginTableSlug     string
	TableAttributesMap map[string]any
	Data               map[string]any
}

type UpdateSyncWithLoginTableRequest struct {
	Tx        pgx.Tx `json:"-"`
	EnvId     string
	ProjectId string
	Guid      string
	Data      map[string]any
}

type DeleteSyncWithLoginTableRequest struct {
	Tx       pgx.Tx `json:"-"`
	Id       string
	Response map[string]any
}

type DeleteManySyncWithLoginTableRequest struct {
	Tx    pgx.Tx `json:"-"`
	Ids   []string
	Users []*pa.DeleteManyUserRequest_User
	Table *Table
	Data  map[string]any
}

type PersonRequest struct {
	Tx                pgx.Tx   `json:"-"`
	Guid              string   `json:"guid"`
	FullName          string   `json:"full_name"`
	Image             string   `json:"image"`
	Login             string   `json:"login"`
	Password          string   `json:"password"`
	Email             string   `json:"email"`
	Phone             string   `json:"phone_number"`
	Tin               string   `json:"tin"`
	UserIdAuth        string   `json:"user_id_auth"`
	ClientTypeId      string   `json:"client_type_id"`
	RoleId            string   `json:"role_id"`
	IsPasswordChanged bool     `json:"is_password_changed"`
	Ids               []string `json:"ids"`
}

package models

import (
	"database/sql"
	"time"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"

	"google.golang.org/protobuf/types/known/structpb"
)

type CustomPermDef struct {
	Id           string
	ParentId     sql.NullString
	ClientTypeId sql.NullString
	Title        string
	Attributes   *structpb.Struct
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (c *CustomPermDef) ToProto() *nb.CustomPermission {
	return &nb.CustomPermission{
		Id:           c.Id,
		ParentId:     c.ParentId.String,
		Title:        c.Title,
		Attributes:   c.Attributes,
		CreatedAt:    c.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    c.UpdatedAt.Format(time.RFC3339),
		ClientTypeId: c.ClientTypeId.String,
	}
}

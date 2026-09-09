package models

import "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"

type Field struct {
	ID                  string         `json:"id"`
	Default             string         `json:"default"`
	Type                string         `json:"type"`
	Index               string         `json:"index"`
	Label               string         `json:"label"`
	Slug                string         `json:"slug"`
	TableID             string         `json:"table_id"`
	Required            bool           `json:"required"`
	Attributes          map[string]any `json:"attributes"`
	IsVisible           bool           `json:"is_visible"`
	AutoFillField       string         `json:"autofill_field"`
	AutoFillTable       string         `json:"autofill_table"`
	RelationId          string         `json:"relation_id"`
	Unique              bool           `json:"unique"`
	Automatic           bool           `json:"automatic"`
	RelationField       string         `json:"relation_field"`
	ShowLabel           bool           `json:"show_label"`
	EnableMultilanguage bool           `json:"enable_multilanguage"`
	MinioFolder         string         `json:"minio_folder"`
	IsAlt               bool           `json:"is_alt"`
}

type CreateFieldsRequest struct {
	ID          string         `json:"id"`
	Default     string         `json:"default"`
	Type        string         `json:"type"`
	Index       string         `json:"index"`
	Label       string         `json:"label"`
	Slug        string         `json:"slug"`
	Required    bool           `json:"required"`
	Attributes  map[string]any `json:"attributes"`
	IsVisible   bool           `json:"is_visible"`
	Unique      bool           `json:"unique"`
	Automatic   bool           `json:"automatic"`
	MinioFolder string         `json:"minio_folder"`
}

type CreateFieldRequest struct {
	ID                  string         `json:"id"`
	Default             string         `json:"default"`
	Type                string         `json:"type"`
	Index               string         `json:"index"`
	Label               string         `json:"label"`
	Slug                string         `json:"slug"`
	TableID             string         `json:"table_id"`
	Required            bool           `json:"required"`
	Attributes          map[string]any `json:"attributes"`
	IsVisible           bool           `json:"is_visible"`
	AutoFillField       string         `json:"autofill_field"`
	AutoFillTable       string         `json:"autofill_table"`
	RelationField       string         `json:"relation_field"`
	Unique              bool           `json:"unique"`
	Automatic           bool           `json:"automatic"`
	ShowLabel           bool           `json:"show_label"`
	EnableMultilanguage bool           `json:"enable_multilanguage"`
	MinioFolder         string         `json:"minio_folder"`
	IsAlt               bool           `json:"is_alt"`
}

type GetAllFieldsResponse struct {
	Fields []Field        `json:"fields"`
	Count  int32          `json:"count"`
	Data   map[string]any `json:"data"`
}

type CreateTableRequest struct {
	Label             string         `json:"label"`
	Description       string         `json:"description"`
	Slug              string         `json:"slug"`
	ShowInMeny        bool           `json:"show_in_menu"`
	Icon              string         `json:"icon"`
	SubtitleFieldSlug string         `json:"subtitle_field_slug"`
	IncrementID       IncrementId    `json:"increment_id"`
	OrderBy           bool           `json:"order_by"`
	Attributes        map[string]any `json:"attributes"`
	IsLoginTable      bool           `json:"is_login_table"`
}

type IncrementId struct {
	WithIncrementID bool   `json:"with_increment_id"`
	DigitNumber     int32  `json:"digit_number"`
	Prefix          string `json:"prefix"`
}
type CreateTableResponse struct {
	ID                string                `json:"id"`
	Label             string                `json:"label"`
	Description       string                `json:"description"`
	Slug              string                `json:"slug"`
	Fields            []CreateFieldsRequest `json:"fields"`
	ShowInMeny        bool                  `json:"show_in_menu"`
	Icon              string                `json:"icon"`
	SubtitleFieldSlug string                `json:"subtitle_field_slug"`
}

type Section struct {
	ID     string                                 `json:"id"`
	Order  int32                                  `json:"order"`
	Column string                                 `json:"column"`
	Label  string                                 `json:"label"`
	Fields object_builder_service.FieldForSection `json:"fields"`
}

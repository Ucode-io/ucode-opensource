package models

// StructBody is alternative for structpb.Struct
type StructBody struct {
	Body map[string]any
}

// Table ...
type Table struct {
	Slug      string      `json:"slug"`
	ViewField []ViewField `json:"view_field"`
	Label     string      `json:"label"`
	Icon      string      `json:"icon"`
}

// ViewField ...
type ViewField struct {
	Slug      string   `json:"slug"`
	ViewField []string `json:"view_field"`
	Label     string   `json:"label"`
}

// CreateClientTypeRequest for create client type request
type CreateClientTypeRequest struct {
	Name         string  `json:"name"`
	ConfirmBy    int32   `json:"confirm_by"`
	SelfRegister bool    `json:"self_register"`
	SelfRecover  bool    `json:"self_recover"`
	ProjectId    string  `json:"project_id"`
	Tables       []Table `json:"table"`
}

type UpdateClientTypeRequest struct {
	Id                string   `json:"id"`
	Name              string   `json:"name"`
	ConfirmBy         int32    `json:"confirm_by"`
	SelfRegister      bool     `json:"self_register"`
	SelfRecover       bool     `json:"self_recover"`
	Tables            []Table  `json:"table"`
	ProjectId         string   `json:"project_id"`
	ClientPlatformIds []string `json:"client_platform_ids"`
}

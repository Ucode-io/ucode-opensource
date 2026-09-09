package models

import "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"

type Company struct {
	CompanyId   string `json:"company_id"`
	Name        string `json:"name"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
}

type CompanyCreateRequest struct {
	Name        string `json:"name"`
	Logo        string `json:"logo"`
	Description string `json:"description"`
}

type CompanyCreateResponse struct {
	CompanyId string `json:"company_id"`
}

type Environment struct {
	Id           string         `json:"id"`
	ProjectId    string         `json:"project_id"`
	Name         string         `json:"name"`
	DisplayColor string         `json:"display_color"`
	Description  string         `json:"description"`
	Data         map[string]any `json:"data"`
}

type AirByteRequest struct {
	Data company_service.GetListAirbyteRequest `json:"data"`
}

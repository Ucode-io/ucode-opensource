package models

type DocxTemplateVariables struct {
	ID        string         `json:"id"`
	TableSlug string         `json:"table_slug"`
	Data      map[string]any `json:"data"`
}

type ConvertAPIResponse struct {
	Files []struct {
		Url string `json:"Url"`
	} `json:"Files"`
}

type ExcelToDbRequest struct {
	TableSlug string         `json:"table_slug"`
	Data      map[string]any `json:"data"`
}

type ExcelToDbResponse struct {
	Message string `json:"message"`
}

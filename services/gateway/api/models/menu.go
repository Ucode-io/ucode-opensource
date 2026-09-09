package models

type CreateMenuRequest struct {
	Label           string         `json:"label"`
	Icon            string         `json:"icon"`
	TableId         string         `json:"table_id"`
	LayoutId        string         `json:"layout_id"`
	ParentId        string         `json:"parent_id"`
	Type            string         `json:"type"`
	MicrofrontendId string         `json:"microfrontend_id"`
	WebpageId       string         `json:"webpage_id"`
	Attributes      map[string]any `json:"attributes"`
}

type Menu struct {
	Id              string         `json:"id"`
	Label           string         `json:"label"`
	Icon            string         `json:"icon"`
	TableId         string         `json:"table_id"`
	LayoutId        string         `json:"layout_id"`
	ParentId        string         `json:"parent_id"`
	Type            string         `json:"type"`
	MicrofrontendId string         `json:"microfrontend_id"`
	WebpageId       string         `json:"webpage_id"`
	Attributes      map[string]any `json:"attributes"`
	IsVisible       bool           `json:"is_visible"`
	WikiId          string         `json:"wiki_id"`
}

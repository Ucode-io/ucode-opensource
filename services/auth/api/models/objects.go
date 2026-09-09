package models

type CommonMessage struct {
	TableSlug string         `json:"table_slug"`
	Data      map[string]any `json:"data"`
}

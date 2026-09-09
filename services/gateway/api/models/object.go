package models

type CommonMessage struct {
	Data     map[string]any `json:"data"`
	IsCached bool           `json:"is_cached"`
}

type HtmlBody struct {
	Data map[string]any `json:"data"`
	Html string         `json:"html"`
}

type GetListRequest struct {
	TableSlug string `json:"table_slug"`
	Search    string `json:"search"`
	Limit     int32  `json:"limit"`
	Offset    int32  `json:"offset"`
}

type UpsertCommonMessage struct {
	Data          map[string]any `json:"data"`
	UpdatedFields []string       `json:"updated_fields"`
}

type Ids struct {
	Ids   []string       `json:"ids"`
	Query map[string]any `json:"query"`
}

type MultipleUpdateItems struct {
	Ids  []string       `json:"ids"`
	Data map[string]any `json:"data"`
}

type MultipleInsertItems struct {
	Items []map[string]any `json:"items"`
}

type ObjectsResponse struct {
	Data struct{} `json:"data"`
}

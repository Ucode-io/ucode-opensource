package models

type IsExistsBody struct {
	TableSlug  string
	FieldSlug  string
	FieldValue any
}

type CreateBody struct {
	FieldMap   map[string]FieldBody
	TableId    string
	Fields     []Field
	TableSlugs []string
}

type FormulaFilter struct {
	FilterItems []FilterItem `json:"formula_filters"`
}

type FilterItem struct {
	Key   string `json:"key"`
	Value any    `json:"value"`
}

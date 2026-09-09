package models

type QueryParams struct {
	Operation   string         `json:"operation"`
	Table       string         `json:"table"`
	Columns     []string       `json:"columns,omitempty"`
	Data        map[string]any `json:"data,omitempty"`
	Where       string         `json:"where,omitempty"`
	Joins       []Join         `json:"joins,omitempty"`
	GroupBy     []string       `json:"group_by,omitempty"`
	Having      string         `json:"having,omitempty"`
	Limit       uint64         `json:"limit,omitempty"`
	Offset      uint64         `json:"offset,omitempty"`
	OrderBy     []string       `json:"order_by,omitempty"`
	WithQueries []WithQuery    `json:"with,omitempty"`
}

type WithQuery struct {
	Name         string      `json:"name"`
	Columns      []string    `json:"columns,omitempty"`
	Query        QueryParams `json:"query"`
	Materialized bool        `json:"materialized,omitempty"`
	Recursive    bool        `json:"recursive,omitempty"`
}

type WhereClause struct {
	Column   string `json:"column"`
	Operator string `json:"operator"`
	Value    any    `json:"value"`
}

type Join struct {
	Type      string `json:"type"`
	Table     string `json:"table"`
	Condition string `json:"condition"`
}

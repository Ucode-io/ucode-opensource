package models

type ColumnVO struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Field       string `json:"field"`
	AggFunc     string `json:"aggFunc"`
}

type RequestAgGrid struct {
	StartRow     int64                    `json:"startRow"`
	EndRow       int64                    `json:"endRow"`
	RowGroupCols []ColumnVO               `json:"rowGroupCols"`
	ValueCols    []ColumnVO               `json:"valueCols"`
	PivotCols    []ColumnVO               `json:"pivotCols"`
	PivotMode    bool                     `json:"pivotMode"`
	GroupKeys    []string                 `json:"groupKeys"`
	FilterModel  map[string]interface{}   `json:"filterModel"`
	SortModel    []map[string]interface{} `json:"sortModel"`
}

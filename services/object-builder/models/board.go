package models

type BoardDataParams struct {
	GroupBy         GroupBy    `json:"group_by"`
	SubgroupBy      SubgroupBy `json:"subgroup_by"`
	Limit           int        `json:"limit"`
	Offset          int        `json:"offset"`
	Search          string     `json:"search"`
	Fields          []string   `json:"fields"`
	ViewFields      []string   `json:"view_fields"`
	RoleIdFromToken string     `json:"role_id_from_token"`
	UserIdFromToken string     `json:"user_id_from_token"`
}

type GroupBy struct {
	Field string `json:"field"`
}

type SubgroupBy struct {
	Field string `json:"field"`
}

type BoardGroup struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

type BoardSubgroup struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

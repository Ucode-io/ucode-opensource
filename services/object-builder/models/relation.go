package models

import (
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	_struct "github.com/golang/protobuf/ptypes/struct"
	"github.com/jackc/pgx/v5"
	"google.golang.org/protobuf/types/known/structpb"
)

type ReqForViewRelation struct {
	Id        string
	ProjectId string
	TableSlug string
	TableId   string
	RoleId    string
}

type RelationForView struct {
	Id                     string
	TableFrom              string
	TableTo                string
	FieldFrom              string
	FieldTo                string
	Type                   string
	ViewFields             []string
	Editable               bool
	RelationFieldSlug      string
	AutoFilters            []string
	IsUserIdDefault        bool
	Cascadings             []string
	ObjectIdFromJwt        string
	CascadingTreeTableSlug string
	CascadingTreeFieldSlug string
	CreatedAt              string
	UpdatedAt              string
	DynamicTables          []struct {
		TableSlug  string
		ViewFields []string
	}
}

type CreateRelationRequest struct {
	TableFrom              string             `json:"table_from"`
	TableTo                string             `json:"table_to"`
	Type                   string             `json:"type"`
	ViewFields             []string           `json:"view_fields"`
	Editable               bool               `json:"editable"`
	IsEditable             bool               `json:"is_editable"`
	ViewType               string             `json:"view_type"`
	Columns                []string           `json:"columns"`
	QuickFilters           []*nb.QuickFilter  `json:"quick_filters"`
	GroupFields            []string           `json:"group_fields"`
	DynamicTables          []*nb.DynamicTable `json:"dynamic_tables"`
	RelationFieldSlug      string             `json:"relation_field_slug"`
	AutoFilters            []*nb.AutoFilter   `json:"auto_filters"`
	IsUserIdDefault        bool               `json:"is_user_id_default"`
	ObjectIdFromJwt        bool               `json:"object_id_from_jwt"`
	CascadingTreeTableSlug string             `json:"cascading_tree_table_slug"`
	CascadingTreeFieldSlug string             `json:"cascading_tree_field_slug"`
	DefaultLimit           string             `json:"default_limit"`
	MultipleInsert         bool               `json:"multiple_insert"`
	UpdatedFields          []string           `json:"updated_fields"`
	DefaultEditable        bool               `json:"default_editable"`
	Attributes             *_struct.Struct    `json:"attributes"`
	Id                     string             `json:"id"`
	RelationFieldId        string             `json:"relation_field_id"`
	RelationTableSlug      string             `json:"relation_table_slug"`
	Tx                     pgx.Tx             `json:"-"`
}

type RelationHelper struct {
	Tx           pgx.Tx `json:"-"`
	Conn         *psqlpool.Pool
	FieldName    string
	TableID      string
	LayoutID     string
	TableSlug    string
	TabID        string
	Fields       []*nb.FieldForSection
	SectionID    string
	View         *nb.CreateViewRequest
	Field        *nb.CreateFieldRequest
	FieldID      string
	RoleID       string
	TableFrom    string
	TableTo      string
	Label        string
	Order        int
	Type         string
	RelationID   string
	RoleIDs      []string
	RelationType string
	FieldFrom    string
	FieldTo      string
	Attributes   *structpb.Struct
}

type RelationLayout struct {
	Tx         pgx.Tx `json:"-"`
	Conn       *psqlpool.Pool
	TableId    string
	RelationId string
}

type ViewRelationModel struct {
	RoleID           string
	TableSlug        string
	RelationID       string
	ViewPermission   bool
	CreatePermission bool
	EditPermission   bool
	DeletePermission bool
}

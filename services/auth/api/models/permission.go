package models

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/object_builder_service"
)

type (
	GetListWithRoleAppTablePermissionsResponse struct {
		ProjectId string                       `json:"project_id"`
		Data      *RoleWithAppTablePermissions `json:"data"`
	}

	RoleWithAppTablePermissions struct {
		Name             string                               `json:"name"`
		Guid             string                               `json:"guid"`
		ProjectId        string                               `json:"project_id"`
		ClientPlatformId string                               `json:"client_platform_id"`
		ClientTypeId     string                               `json:"client_type_id"`
		GrantAccess      bool                                 `json:"grant_access"`
		Tables           []*RoleWithAppTablePermissions_Table `json:"tables"`
		GlobalPermission *auth_service.GlobalPermission       `json:"global_permission"`
	}

	RoleWithAppTablePermissions_Table struct {
		Label                string                                                                              `json:"label"`
		Slug                 string                                                                              `json:"slug"`
		Description          string                                                                              `json:"description"`
		ShowInMenu           bool                                                                                `json:"show_in_menu"`
		IsChanged            bool                                                                                `json:"is_changed"`
		Icon                 string                                                                              `json:"icon"`
		SubtitleFieldSlug    string                                                                              `json:"subtitle_field_slug"`
		WithIncrementId      bool                                                                                `json:"with_increment_id"`
		DigitNumber          int32                                                                               `json:"digit_number"`
		Id                   string                                                                              `json:"id"`
		RecordPermissions    *object_builder_service.RoleWithAppTablePermissions_Table_RecordPermission          `json:"record_permissions"`
		FieldPermissions     []*RoleWithAppTablePermissions_Table_FieldPermission                                `json:"field_permissions"`
		ViewPermissions      []*RoleWithAppTablePermissions_Table_ViewPermission                                 `json:"view_permissions"`
		AutomaticFilters     *object_builder_service.RoleWithAppTablePermissions_Table_AutomaticFilterWithMethod `json:"automatic_filters"`
		ActionPermissions    []*RoleWithAppTablePermissions_Table_ActionPermission                               `json:"action_permissions"`
		TableViewPermissions []*RoleWithAppTablePermissions_Table_TableViewPermission                            `json:"table_view_permissions"`
		CustomPermission     *object_builder_service.RoleWithAppTablePermissions_Table_CustomPermission          `json:"custom_permission"`
		Attributes           map[string]any                                                                      `json:"attributes"`
	}

	RoleWithAppTablePermissions_Table_FieldPermission struct {
		FieldId        string         `json:"field_id"`
		Guid           string         `json:"guid"`
		ViewPermission bool           `json:"view_permission"`
		EditPermission bool           `json:"edit_permission"`
		Label          string         `json:"label"`
		TableSlug      string         `json:"table_slug"`
		Attributes     map[string]any `json:"attributes"`
	}

	RoleWithAppTablePermissions_Table_ViewPermission struct {
		Guid             string         `json:"guid"`
		Label            string         `json:"label"`
		RelationId       string         `json:"relation_id"`
		ViewPermission   bool           `json:"view_permission"`
		EditPermission   bool           `json:"edit_permission"`
		CreatePermission bool           `json:"create_permission"`
		DeletePermission bool           `json:"delete_permission"`
		TableSlug        string         `json:"table_slug"`
		Attributes       map[string]any `json:"attributes"`
	}

	RoleWithAppTablePermissions_Table_ActionPermission struct {
		Guid          string         `json:"guid"`
		CustomEventId string         `json:"custom_event_id"`
		Permission    bool           `json:"permission"`
		Label         string         `json:"label"`
		TableSlug     string         `json:"table_slug"`
		Attributes    map[string]any `json:"attributes"`
	}

	RoleWithAppTablePermissions_Table_TableViewPermission struct {
		Guid       string         `json:"guid"`
		Name       string         `json:"name"`
		View       bool           `json:"view"`
		Edit       bool           `json:"edit"`
		Delete     bool           `json:"delete"`
		ViewId     string         `json:"view_id"`
		Attributes map[string]any `json:"attributes"`
	}

	GetAllMenuPermissionsResponse struct {
		Menus []*MenuPermission `json:"menus"`
	}

	UpdateMenuPermissionsRequest struct {
		ProjectId string            `json:"project_id"`
		RoleId    string            `json:"role_id"`
		Menus     []*MenuPermission `json:"menus"`
	}

	GetRoleByIdResponse struct {
		Id           string                     `json:"id"`
		ClientTypeId string                     `json:"client_type_id"`
		Name         string                     `json:"name"`
		ClientType   *ClientType                `json:"client_type"`
		Permissions  []*auth_service.Permission `json:"permissions"`
	}

	MenuPermission struct {
		Id         string                                            `json:"id"`
		Label      string                                            `json:"label"`
		Type       string                                            `json:"type"`
		Permission *object_builder_service.MenuPermission_Permission `json:"permission"`
		Attributes map[string]any                                    `json:"attributes"`
	}
)

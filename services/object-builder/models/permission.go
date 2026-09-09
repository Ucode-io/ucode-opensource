package models

import (
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"

	"google.golang.org/protobuf/types/known/structpb"
)

type TablePermission struct {
	Id         string
	Slug       string
	Label      string
	ShowInMenu bool
	IsChanged  bool
	IsCached   bool
	Icon       string
	IsSystem   bool
	Attributes *structpb.Struct
}

type RecordPermission struct {
	Read            string
	Write           string
	Update          string
	Delete          string
	IsHaveCondition bool
	IsPublic        bool
	RoleID          string
	TableSlug       string
	LanguageBtn     string
	Automation      string
	Settings        string
	ShareModal      string
	ViewCreate      string
	PDFAction       string
	AddField        string
	DeleteAll       string
	AddFilter       string
	FieldFilter     string
	FixColumn       string
	TabGroup        string
	Columns         string
	Group           string
	ExcelMenu       string
	SearchButton    string
}

type CustomPermission struct {
	Chat                  bool `json:"chat"`
	MenuButton            bool `json:"menu_button"`
	SettingsButton        bool `json:"settings_button"`
	ProjectsButton        bool `json:"projects_button"`
	EnvironmentsButton    bool `json:"environments_button"`
	APIKeysButton         bool `json:"api_keys_button"`
	RedirectsButton       bool `json:"redirects_button"`
	MenuSettingButton     bool `json:"menu_setting_button"`
	ProfileSettingsButton bool `json:"profile_settings_button"`
	ProjectButton         bool `json:"project_button"`
	SMSButton             bool `json:"sms_button"`
	VersionButton         bool `json:"version_button"`
	GitbookButton         bool `json:"gitbook_button"`
	ChatwootButton        bool `json:"chatwoot_button"`
	GptButton             bool `json:"gpt_button"`
	Billing               bool `json:"billing"`
}

type Menu struct {
	Id string `json:"id"`
}

type MenuPermission struct {
	MenuID       string `json:"menu_id"`
	RoleID       string `json:"role_id"`
	Delete       bool   `json:"delete"`
	GUID         string `json:"guid"`
	MenuSettings bool   `json:"menu_settings"`
	Read         bool   `json:"read"`
	Update       bool   `json:"update"`
	Write        bool   `json:"write"`
}

type TableViewPermission struct {
	Guid       string           `json:"guid"`
	TableSlug  string           `json:"table_slug"`
	View       bool             `json:"view"`
	Edit       bool             `json:"edit"`
	Delete     bool             `json:"delete"`
	ViewId     string           `json:"view_id"`
	Attributes *structpb.Struct `json:"attributes"`
}

type GetRecordPermissionRequest struct {
	Conn      *psqlpool.Pool
	TableSlug string
	RoleId    string
}

type GetRecordPermissionResponse struct {
	Guid            string
	RoleId          string
	TableSlug       string
	Read            string
	Write           string
	Update          string
	Delete          string
	IsPublic        bool
	IsHaveCondition bool
}

type DynamicPermission struct {
	Read   []*nb.RoleWithAppTablePermissions_Table_AutomaticFilter `json:"read"`
	Write  []*nb.RoleWithAppTablePermissions_Table_AutomaticFilter `json:"write"`
	Update []*nb.RoleWithAppTablePermissions_Table_AutomaticFilter `json:"update"`
	Delete []*nb.RoleWithAppTablePermissions_Table_AutomaticFilter `json:"delete"`
}

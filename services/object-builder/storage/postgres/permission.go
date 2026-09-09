package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/types/known/structpb"
)

type permissionRepo struct {
	db *psqlpool.Pool
}

func NewPermissionRepo(db *psqlpool.Pool) storage.PermissionRepoI {
	return &permissionRepo{
		db: db,
	}
}

func (p *permissionRepo) GetAllMenuPermissions(ctx context.Context, req *nb.GetAllMenuPermissionsRequest) (*nb.GetAllMenuPermissionsResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.GetAllMenuPermissions")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `
		SELECT 
			m."id",
			m."label",
			m."attributes",
			m."type",

			mp."write",
			mp."read",
			mp."delete",
			mp."update",
			mp."menu_settings"
		FROM "menu" m
		LEFT JOIN
			menu_permission mp ON m.id = mp."menu_id" AND mp.role_id = $1
		WHERE 
			m.parent_id = $2
		ORDER BY
			m.created_at DESC
	`

	var (
		resp = &nb.GetAllMenuPermissionsResponse{}
	)

	rows, err := conn.Query(ctx, query, req.RoleId, req.ParentId)
	if err != nil {
		return &nb.GetAllMenuPermissionsResponse{}, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			attributes   = []byte{}
			menu         = &nb.MenuPermission{}
			permission   = &nb.MenuPermission_Permission{}
			read         = sql.NullBool{}
			write        = sql.NullBool{}
			update       = sql.NullBool{}
			delete       = sql.NullBool{}
			menuSettings = sql.NullBool{}
		)

		err := rows.Scan(
			&menu.Id,
			&menu.Label,
			&attributes,
			&menu.Type,

			&write,
			&read,
			&delete,
			&update,
			&menuSettings,
		)
		if err != nil {
			return &nb.GetAllMenuPermissionsResponse{}, err
		}

		if err := json.Unmarshal(attributes, &menu.Attributes); err != nil {
			return &nb.GetAllMenuPermissionsResponse{}, err
		}

		permission.Read = read.Bool
		permission.Write = write.Bool
		permission.Delete = delete.Bool
		permission.Update = update.Bool
		permission.MenuSettings = menuSettings.Bool
		menu.Permission = permission

		resp.Menus = append(resp.Menus, menu)
	}

	return resp, nil
}

func (p *permissionRepo) CreateDefaultPermission(ctx context.Context, req *nb.CreateDefaultPermissionRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.CreateDefaultPermission")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	query := `
		SELECT
			t.id,
			t.slug,
			t.label,
			t.show_in_menu,
			t.is_changed,
			t.is_cached,
			t.icon,
			t.is_system,
			t.attributes
		FROM "table" t
		LEFT JOIN record_permission rp ON t.slug = rp.table_slug AND rp.role_id = $1
		WHERE t.id NOT IN (SELECT unnest($2::uuid[]))
	`
	rows, err := conn.Query(ctx, query, req.RoleId, pq.Array(config.STATIC_TABLE_IDS))
	if err != nil {
		return errors.Wrap(err, "when exec query table")
	}
	defer rows.Close()

	var tables = []models.TablePermission{}

	for rows.Next() {
		table := models.TablePermission{}
		attributes := []byte{}

		err = rows.Scan(
			&table.Id,
			&table.Slug,
			&table.Label,
			&table.ShowInMenu,
			&table.IsChanged,
			&table.IsCached,
			&table.Icon,
			&table.IsSystem,
			&attributes,
		)
		if err != nil {
			return errors.Wrap(err, "when scan table rows")
		}

		var attrStruct *structpb.Struct
		if err := json.Unmarshal(attributes, &attrStruct); err != nil {
			return err
		}
		table.Attributes = attrStruct

		tables = append(tables, table)
	}

	query = `
		SELECT
			f.id,
			f.label,
			f.table_id,
			f.attributes
		FROM "field" f
	`

	rows, err = conn.Query(ctx, query)
	if err != nil {
		return errors.Wrap(err, "when exec field query")
	}
	defer rows.Close()

	var fields = map[string][]models.Field{}

	for rows.Next() {
		var (
			field      = models.Field{}
			attributes = []byte{}
		)

		err = rows.Scan(
			&field.Id,
			&field.Label,
			&field.TableId,
			&attributes,
		)
		if err != nil {
			return err
		}

		var attrStruct *structpb.Struct
		if err := json.Unmarshal(attributes, &attrStruct); err != nil {
			return errors.Wrap(err, "when scan field rows")
		}
		field.Attributes = attrStruct

		if _, ok := fields[field.TableId]; !ok {
			fields[field.TableId] = []models.Field{field}
		} else {
			fields[field.TableId] = append(fields[field.TableId], field)
		}
	}

	query = `
		SELECT
			v.id,
			v.name,
			v.table_slug,
			v.attributes
		FROM "view" v
	`

	rows, err = conn.Query(ctx, query)
	if err != nil {
		return errors.Wrap(err, "when exec view query")
	}
	defer rows.Close()

	var views = map[string][]models.View{}

	for rows.Next() {
		var (
			view       = models.View{}
			attributes = []byte{}
			name       = sql.NullString{}
			tableSlug  sql.NullString
		)

		err = rows.Scan(
			&view.Id,
			&name,
			&tableSlug,
			&attributes,
		)
		if err != nil {
			return errors.Wrap(err, "when scan view rows")
		}

		var attrStruct map[string]any
		if err := json.Unmarshal(attributes, &attrStruct); err != nil {
			return err
		}
		view.Attributes = attrStruct
		view.Name = name.String
		view.TableSlug = tableSlug.String

		if _, ok := views[view.TableSlug]; !ok {
			views[view.TableSlug] = []models.View{view}
		} else {
			views[view.TableSlug] = append(views[view.TableSlug], view)
		}
	}

	var (
		recordPermissions = make([]models.RecordPermission, 0)
		fieldPermissions  = make([]models.FieldPermission, 0)
		viewPermissions   = make([]models.ViewPermission, 0)
	)

	for _, table := range tables {
		IsHaveCondition := false

		recordPermissionDocument := models.RecordPermission{
			Read:            "Yes",
			Write:           "Yes",
			Update:          "Yes",
			Delete:          "Yes",
			IsHaveCondition: IsHaveCondition,
			IsPublic:        true,
			RoleID:          req.RoleId,
			TableSlug:       table.Slug,
			LanguageBtn:     "Yes",
			Automation:      "Yes",
			Settings:        "Yes",
			ShareModal:      "Yes",
			ViewCreate:      "Yes",
			PDFAction:       "Yes",
			AddField:        "Yes",
			AddFilter:       "Yes",
			FieldFilter:     "Yes",
			FixColumn:       "Yes",
			TabGroup:        "Yes",
			Columns:         "Yes",
			Group:           "Yes",
			ExcelMenu:       "Yes",
			SearchButton:    "Yes",
		}
		recordPermissions = append(recordPermissions, recordPermissionDocument)

		tableFields := fields[table.Id]
		for _, tableField := range tableFields {
			fieldPermission := models.FieldPermission{
				ViewPermission: true,
				EditPermission: true,
				FieldId:        tableField.Id,
				TableSlug:      table.Slug,
				RoleId:         req.RoleId,
				Label:          tableField.Label,
				Guid:           uuid.NewString(),
			}
			fieldPermissions = append(fieldPermissions, fieldPermission)
		}

		tableViews := views[table.Slug]
		for _, view := range tableViews {
			viewPermission := models.ViewPermission{
				Guid:   uuid.NewString(),
				View:   true,
				Edit:   true,
				Delete: true,
				ViewId: view.Id,
				RoleId: req.RoleId,
			}
			viewPermissions = append(viewPermissions, viewPermission)
		}
	}

	customPermission := models.CustomPermission{
		Chat:                  true,
		MenuButton:            true,
		SettingsButton:        true,
		ProjectsButton:        true,
		EnvironmentsButton:    true,
		APIKeysButton:         true,
		RedirectsButton:       true,
		MenuSettingButton:     true,
		ProfileSettingsButton: true,
		ProjectButton:         true,
		SMSButton:             true,
		VersionButton:         true,
		GitbookButton:         true,
		ChatwootButton:        true,
		GptButton:             true,
	}

	query = `
		INSERT INTO global_permission (role_id, chat, menu_button, settings_button, projects_button, environments_button, api_keys_button, redirects_button, menu_setting_button, profile_settings_button, project_button, sms_button, version_button, chatwoot_button, gitbook_button, gpt_button, billing)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17)
		ON CONFLICT (role_id) DO UPDATE
		SET 
			chat = EXCLUDED.chat, 
			menu_button = EXCLUDED.menu_button, 
			settings_button = EXCLUDED.settings_button, 
			projects_button = EXCLUDED.projects_button, 
			environments_button = EXCLUDED.environments_button, 
			api_keys_button = EXCLUDED.api_keys_button, 
			redirects_button = EXCLUDED.redirects_button, 
			menu_setting_button = EXCLUDED.menu_setting_button, 
			profile_settings_button = EXCLUDED.profile_settings_button, 
			project_button = EXCLUDED.project_button, 
			sms_button = EXCLUDED.sms_button, 
			version_button = EXCLUDED.version_button,
			chatwoot_button = EXCLUDED.chatwoot_button,
			gitbook_button = EXCLUDED.gitbook_button,
			gpt_button = EXCLUDED.gpt_button,
			billing = EXCLUDED.billing
		`

	_, err = conn.Exec(ctx, query,
		req.RoleId,
		customPermission.Chat,
		customPermission.MenuButton,
		customPermission.SettingsButton,
		customPermission.ProjectsButton,
		customPermission.EnvironmentsButton,
		customPermission.APIKeysButton,
		customPermission.RedirectsButton,
		customPermission.MenuSettingButton,
		customPermission.ProfileSettingsButton,
		customPermission.ProjectButton,
		customPermission.SMSButton,
		customPermission.VersionButton,
		customPermission.ChatwootButton,
		customPermission.GitbookButton,
		customPermission.GptButton,
		customPermission.Billing,
	)
	if err != nil {
		return errors.Wrap(err, "when insert global_permission")
	}

	query = `
		SELECT id FROM "menu" m
		LEFT JOIN 
			menu_permission mp ON m.id = mp.menu_id AND mp.role_id = $1
		ORDER BY 
			m.order
	`
	rows, err = conn.Query(ctx, query, req.RoleId)
	if err != nil {
		return errors.Wrap(err, "when select menu")
	}
	defer rows.Close()

	menus := []models.Menu{}
	for rows.Next() {
		menu := models.Menu{}
		err := rows.Scan(&menu.Id)
		if err != nil {
			return err
		}
		menus = append(menus, menu)
	}

	menuPermissions := []models.MenuPermission{}
	for _, menu := range menus {
		menuPermission := models.MenuPermission{
			MenuID:       menu.Id,
			RoleID:       req.RoleId,
			Delete:       true,
			GUID:         uuid.NewString(),
			MenuSettings: true,
			Read:         true,
			Update:       true,
			Write:        true,
		}
		menuPermissions = append(menuPermissions, menuPermission)
	}

	values := []string{}

	for _, v := range recordPermissions {
		values = append(values, fmt.Sprintf("('%v', '%v', '%v', '%v', %v, %v, '%v', '%v', '%v', '%v', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s', '%s')",
			v.Read, v.Write, v.Update,
			v.Delete, v.IsHaveCondition, v.IsPublic,
			v.RoleID, v.TableSlug, v.LanguageBtn,
			v.Automation, v.Settings, v.ShareModal,
			v.ViewCreate, v.PDFAction, v.AddField,
			v.AddFilter, v.FieldFilter, v.FixColumn,
			v.TabGroup, v.Columns, v.Group, v.ExcelMenu,
			v.SearchButton,
		))
	}

	query = fmt.Sprintf(`
		INSERT INTO record_permission (
			read, 
			write, 
			update, 
			delete, 
			is_have_condition, 
			is_public, 
			role_id,
			table_slug, 
			language_btn, 
			automation, 
			settings, 
			share_modal, 
			view_create, 
			pdf_action, 
			add_field,
			add_filter,
			field_filter,
			fix_column,
			tab_group,
			columns,
			"group",
			excel_menu,
			search_button
		) VALUES %v
		ON CONFLICT (role_id, table_slug) DO UPDATE
		SET
			read = EXCLUDED.read,
			write = EXCLUDED.write,
			update = EXCLUDED.update,
			delete = EXCLUDED.delete,
			is_have_condition = EXCLUDED.is_have_condition,
			is_public = EXCLUDED.is_public,
			language_btn = EXCLUDED.language_btn,
			automation = EXCLUDED.automation,
			settings = EXCLUDED.settings,
			share_modal = EXCLUDED.share_modal,
			view_create = EXCLUDED.view_create,
			pdf_action = EXCLUDED.pdf_action,
			add_field = EXCLUDED.add_field,
			add_filter = EXCLUDED.add_filter,
			field_filter = EXCLUDED.field_filter,
			fix_column = EXCLUDED.fix_column,
			tab_group = EXCLUDED.tab_group,
			columns = EXCLUDED.columns,
			"group" = EXCLUDED."group",
			excel_menu = EXCLUDED.excel_menu,
			search_button = EXCLUDED.search_button
	`, strings.Join(values, ", "))

	_, err = conn.Exec(ctx, query)
	if err != nil {
		return errors.Wrap(err, "when insert record_permission")
	}

	values = []string{}

	for _, v := range menuPermissions {
		values = append(values, fmt.Sprintf("('%v', '%v', %v, '%v', %v, %v, %v, %v)",
			v.MenuID, v.RoleID, v.Delete,
			v.GUID, v.MenuSettings, v.Read,
			v.Update, v.Write,
		))
	}

	query = fmt.Sprintf(`
		INSERT INTO menu_permission (menu_id, role_id, delete, guid, menu_settings, read, update, write)
		VALUES %s
		ON CONFLICT (menu_id, role_id) DO UPDATE
		SET
			delete = EXCLUDED.delete,
			guid = EXCLUDED.guid,
			menu_settings = EXCLUDED.menu_settings,
			read = EXCLUDED.read,
			update = EXCLUDED.update,
			write = EXCLUDED.write
	`, strings.Join(values, ", "))

	_, err = conn.Exec(ctx, query)
	if err != nil {
		return errors.Wrap(err, "when insert menu_permission")
	}

	values = []string{}

	for _, v := range fieldPermissions {
		label := strings.ReplaceAll(v.Label, "'", "''")

		values = append(values, fmt.Sprintf("(%v, %v, '%v', '%v', '%v', '%v', '%v')",
			v.ViewPermission, v.EditPermission, v.FieldId,
			v.TableSlug, v.RoleId, label, v.Guid,
		))
	}

	templates := helper.CreateTemplate(req.RoleId)
	for _, v := range templates {
		label := strings.ReplaceAll(v.Label, "'", "''")

		values = append(values, fmt.Sprintf("(%v, %v, '%v', '%v', '%v', '%v', '%v')",
			v.ViewPermission, v.EditPermission, v.FieldId,
			v.TableSlug, v.RoleId, label, v.Guid,
		))
	}

	query = fmt.Sprintf(`
        INSERT INTO field_permission ("view_permission", "edit_permission", "field_id", "table_slug", "role_id", "label", "guid")
        VALUES %s 
        ON CONFLICT (field_id, role_id) DO UPDATE
        SET
            view_permission = EXCLUDED.view_permission,
            edit_permission = EXCLUDED.edit_permission,
            table_slug = EXCLUDED.table_slug,
            label = EXCLUDED.label,
            guid = EXCLUDED.guid
    `, strings.Join(values, ", "))

	_, err = conn.Exec(ctx, query)
	if err != nil {
		return errors.Wrap(err, "when insert field_permission")
	}

	values = []string{}

	for _, v := range viewPermissions {
		values = append(values, fmt.Sprintf("('%v', '%v', '%v', '%v', '%v', '%v')",
			v.Guid, v.View, v.Edit, v.Delete, v.ViewId, v.RoleId,
		))
	}

	if len(values) > 0 {
		query := fmt.Sprintf(`
			INSERT INTO view_permission (guid, view, edit, delete, view_id, role_id)
			VALUES %s 
			ON CONFLICT (view_id, role_id) DO UPDATE
			SET
				guid = EXCLUDED.guid,
				view = EXCLUDED.view,
				edit = EXCLUDED.edit,
				delete = EXCLUDED.delete
		`, strings.Join(values, ", "))

		_, err := conn.Exec(ctx, query)
		if err != nil {
			return errors.Wrap(err, "when insert view_permission")
		}
	}

	query = `
		INSERT INTO custom_permission_access (id, custom_permission_id, role_id, client_type_id)
		SELECT uuid_generate_v4(), cp.id, r.guid, r.client_type_id
		FROM custom_permission cp
		JOIN role r ON r.guid = $1
		WHERE r.client_type_id IS NOT NULL
		  AND (cp.client_type_id IS NULL OR cp.client_type_id = r.client_type_id)
		ON CONFLICT (custom_permission_id, role_id, client_type_id) DO NOTHING
	`

	_, err = conn.Exec(ctx, query, req.RoleId)
	if err != nil {
		return errors.Wrap(err, "when insert custom_permission_access")
	}

	return nil
}

func (p *permissionRepo) GetListWithRoleAppTablePermissions(ctx context.Context, req *nb.GetListWithRoleAppTablePermissionsRequest) (resp *nb.GetListWithRoleAppTablePermissionsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.GetListWithRoleAppTablePermissions")
	defer dbSpan.Finish()

	var (
		role                     models.Role
		globalPermission         = nb.GlobalPermission{}
		tableViewPermission      []*models.TableViewPermission
		response                 nb.RoleWithAppTablePermissions
		tables                   []*nb.RoleWithAppTablePermissions_Table
		tablesList               []*nb.RoleWithAppTablePermissions_Table
		table_view_permission    = make(map[string][]*models.TableViewPermission)
		ViewPermissions          []*nb.RoleWithAppTablePermissions_Table_ViewPermission
		FieldPermissions         []*nb.RoleWithAppTablePermissions_Table_FieldPermission
		ActionPermissions        []*nb.RoleWithAppTablePermissions_Table_ActionPermission
		view_relation_permission = make(map[string][]*nb.RoleWithAppTablePermissions_Table_ViewPermission)
		fields                   = make(map[string][]*nb.RoleWithAppTablePermissions_Table_FieldPermission)
		actionPermission         = make(map[string][]*nb.RoleWithAppTablePermissions_Table_ActionPermission)
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	query := `SELECT guid, name, project_id, COALESCE(client_platform_id::text, ''), COALESCE(client_type_id::text, ''), is_system FROM role WHERE guid = $1`

	err = conn.QueryRow(ctx, query, req.GetRoleId()).Scan(&role.Guid, &role.Name, &role.ProjectId, &role.ClientPlatformId, &role.ClientTypeId, &role.IsSystem)
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get role")
	}

	roleCopy := role

	queryGetTables := `
		SELECT
			t.id,
			t.slug,
			t.label,
			t.show_in_menu,
			t.is_changed,
			t.icon,
			t.attributes,
			rp.guid,
			COALESCE(rp.read, 'No') AS read,
    		COALESCE(rp.write, 'No') AS write,
    		COALESCE(rp.update, 'No') AS update,
    		COALESCE(rp.delete, 'No') AS delete,
    		COALESCE(rp.is_public, false) AS is_public,
    		COALESCE(rp.is_have_condition, false) AS is_have_condition_other,
    		COALESCE(rp.view_create, 'No') AS view_create,
    		COALESCE(rp.share_modal, 'No') AS share_modal,
    		COALESCE(rp.settings, 'No') AS settings,
    		COALESCE(rp.automation, 'No') AS automation,
    		COALESCE(rp.language_btn, 'No') AS language_btn,
    		COALESCE(rp.pdf_action, 'No') AS pdf_action,
    		COALESCE(rp.add_field, 'No') AS add_field,
    		COALESCE(rp."add_filter", 'Yes') AS add_filter,
    		COALESCE(rp."field_filter", 'Yes') AS field_filter,
    		COALESCE(rp."fix_column", 'Yes') AS fix_column,
    		COALESCE(rp."columns", 'Yes') AS columns,
    		COALESCE(rp."group", 'Yes') AS group,
    		COALESCE(rp."excel_menu", 'Yes') AS excel_menu,
    		COALESCE(rp."tab_group", 'Yes') AS tab_group,
			COALESCE(rp."search_button", 'Yes') AS search_button
		FROM "table" t
		LEFT JOIN record_permission rp ON t.slug = rp.table_slug AND rp.role_id = $1
		WHERE t.id NOT IN (SELECT unnest($2::uuid[]))
	`
	rows, err := conn.Query(ctx, queryGetTables, req.RoleId, pq.Array(config.STATIC_TABLE_IDS))
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get tables")
	}
	defer rows.Close()

	for rows.Next() {
		var (
			table = nb.RoleWithAppTablePermissions_Table{
				RecordPermissions: &nb.RoleWithAppTablePermissions_Table_RecordPermission{},
				CustomPermission:  &nb.RoleWithAppTablePermissions_Table_CustomPermission{},
				Attributes:        structpb.NewNullValue().GetStructValue()}
			attributes []byte
			attrStruct *structpb.Struct
			guid       sql.NullString
		)

		err = rows.Scan(
			&table.Id,
			&table.Slug,
			&table.Label,
			&table.ShowInMenu,
			&table.IsChanged,
			&table.Icon,
			&attributes,
			&guid,
			&table.RecordPermissions.Read,
			&table.RecordPermissions.Write,
			&table.RecordPermissions.Update,
			&table.RecordPermissions.Delete,
			&table.RecordPermissions.IsPublic,
			&table.RecordPermissions.IsHaveCondition,
			&table.CustomPermission.ViewCreate,
			&table.CustomPermission.ShareModal,
			&table.CustomPermission.Settings,
			&table.CustomPermission.Automation,
			&table.CustomPermission.LanguageBtn,
			&table.CustomPermission.PdfAction,
			&table.CustomPermission.AddField,
			&table.CustomPermission.AddFilter,
			&table.CustomPermission.FieldFilter,
			&table.CustomPermission.FixColumn,
			&table.CustomPermission.Columns,
			&table.CustomPermission.Group,
			&table.CustomPermission.ExcelMenu,
			&table.CustomPermission.TabGroup,
			&table.CustomPermission.SearchButton,
		)
		if err != nil {
			return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when scan table resp")
		}
		if err := json.Unmarshal(attributes, &attrStruct); err != nil {
			return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when unmarshl table resp")
		}

		table.Attributes = attrStruct

		tables = append(tables, &table)
	}

	queryFieldPermission := `
		SELECT
			"guid",
			"label",
			"table_slug",
			"field_id",
			"edit_permission",
			"view_permission"
		FROM "field_permission" WHERE role_id = $1`

	rowsFieldPermission, err := conn.Query(ctx, queryFieldPermission, req.GetRoleId())
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get field_permission")
	}
	defer rowsFieldPermission.Close()

	for rowsFieldPermission.Next() {
		fp := nb.RoleWithAppTablePermissions_Table_FieldPermission{}
		err = rowsFieldPermission.Scan(
			&fp.Guid,
			&fp.Label,
			&fp.TableSlug,
			&fp.FieldId,
			&fp.EditPermission,
			&fp.ViewPermission,
		)
		if err != nil {
			return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when scan field_permission resp")
		}
		FieldPermissions = append(FieldPermissions, &fp)
	}

	queryViewRelationPermission := `
	SELECT 
    	COALESCE(guid::text, ''),
    	COALESCE(label, ''),
    	COALESCE(relation_id::text, ''),
    	COALESCE(table_slug, ''),
    	COALESCE(view_permission, false),
    	COALESCE(create_permission, false),
    	COALESCE(edit_permission, false),
    	COALESCE(delete_permission, false)
	FROM view_relation_permission
	WHERE role_id = $1;
	`

	rowsViewRelationPermission, err := conn.Query(ctx, queryViewRelationPermission, req.GetRoleId())
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get view_relation_permission")
	}
	defer rowsViewRelationPermission.Close()

	for rowsViewRelationPermission.Next() {
		viewRelationPermission := nb.RoleWithAppTablePermissions_Table_ViewPermission{}

		err = rowsViewRelationPermission.Scan(
			&viewRelationPermission.Guid,
			&viewRelationPermission.Label,
			&viewRelationPermission.RelationId,
			&viewRelationPermission.TableSlug,
			&viewRelationPermission.ViewPermission,
			&viewRelationPermission.CreatePermission,
			&viewRelationPermission.EditPermission,
			&viewRelationPermission.DeletePermission,
		)
		if err != nil {
			return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when scan view_relation_permission")
		}

		ViewPermissions = append(ViewPermissions, &viewRelationPermission)
	}
	queryViewPermission := `
	  	SELECT 
			vp.guid,
			v.table_slug,
			vp.view,
			vp.view_id,
			vp.edit,
			vp.delete
		FROM view AS v
		LEFT JOIN view_permission AS vp ON v.id = vp.view_id
        WHERE vp.role_id = $1`

	rowsViewPermission, err := conn.Query(ctx, queryViewPermission, req.RoleId)
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get view_relation")
	}
	defer rowsViewPermission.Close()

	for rowsViewPermission.Next() {
		var (
			guid, tableSlug, viewId sql.NullString
			view, edit, delete      sql.NullBool
		)

		err = rowsViewPermission.Scan(&guid, &tableSlug, &view, &viewId, &edit, &delete)
		if err != nil {
			return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => whan scan view_permission")
		}

		tableViewPermission = append(tableViewPermission, &models.TableViewPermission{
			Guid:      guid.String,
			TableSlug: tableSlug.String,
			View:      view.Bool,
			ViewId:    viewId.String,
			Edit:      edit.Bool,
			Delete:    delete.Bool,
		})
	}

	queryActionPermission := `
		SELECT 
			ap.guid,
			ap.custom_event_id,
			ap.permission,
			ap.label,
			ap.table_slug
		FROM custom_event AS ce
		LEFT JOIN action_permission AS ap ON ce.id = ap.custom_event_id  
		WHERE ap.role_id = $1
	`

	rowsActionPermission, err := conn.Query(ctx, queryActionPermission, req.RoleId)
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get action_permission")
	}
	defer rowsActionPermission.Close()

	for rowsActionPermission.Next() {
		var (
			actionPermission nb.RoleWithAppTablePermissions_Table_ActionPermission
			labelStr         sql.NullString
		)

		err = rowsActionPermission.Scan(
			&actionPermission.Guid,
			&actionPermission.CustomEventId,
			&actionPermission.Permission,
			&labelStr,
			&actionPermission.TableSlug,
		)
		if err != nil {
			return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when scan action_permission")
		}
		actionPermission.Label = labelStr.String
		ActionPermissions = append(ActionPermissions, &actionPermission)
	}

	queryAutomaticFilter := `WITH filtered AS (
		SELECT
			table_slug,
			JSONB_AGG(JSONB_BUILD_OBJECT(
					'table_slug', table_slug,
					'custom_field', custom_field,
					'object_field', object_field,
					'not_use_in_tab', not_use_in_tab
				)) FILTER (WHERE method = 'read') AS read,
			JSONB_AGG(JSONB_BUILD_OBJECT(
					'table_slug', table_slug,
					'custom_field', custom_field,
					'object_field', object_field,
					'not_use_in_tab', not_use_in_tab
				)) FILTER (WHERE method = 'write') AS write,
			JSONB_AGG(JSONB_BUILD_OBJECT(
					'table_slug', table_slug,
					'custom_field', custom_field,
					'object_field', object_field,
					'not_use_in_tab', not_use_in_tab
				)) FILTER (WHERE method = 'update') AS update,
			JSONB_AGG(JSONB_BUILD_OBJECT(
					'table_slug', table_slug,
					'custom_field', custom_field,
					'object_field', object_field,
					'not_use_in_tab', not_use_in_tab
				)) FILTER (WHERE method = 'delete') AS delete
		FROM automatic_filter p
		WHERE role_id = $1
		GROUP BY table_slug
	),
	automatic_filters AS (
		SELECT
			table_slug,
			JSONB_BUILD_OBJECT(
				'read', COALESCE(read, '[]'::jsonb),
				'write', COALESCE(write, '[]'::jsonb),
				'update', COALESCE(update, '[]'::jsonb),
				'delete', COALESCE(delete, '[]'::jsonb)
			) AS filters
		FROM filtered
	)
	SELECT JSONB_OBJECT_AGG(table_slug, filters) AS autofilter
	FROM automatic_filters;`

	var (
		automaticFilter     = make(map[string]models.DynamicPermission)
		automaticFilterData sql.NullString
	)

	err = conn.QueryRow(ctx, queryAutomaticFilter, req.RoleId).Scan(&automaticFilterData)
	if err != nil {
		return nil, errors.Wrap(err, "GetListWithRoleAppTablePermissions => when get automatic_filters")
	}

	if automaticFilterData.Valid {
		if err = json.Unmarshal([]byte(automaticFilterData.String), &automaticFilter); err != nil {
			return nil, errors.Wrap(err, "error while unmarshaling")
		}
	}

	for _, fieldPermission := range FieldPermissions {
		if _, ok := fields[fieldPermission.TableSlug]; !ok {
			fields[fieldPermission.TableSlug] = []*nb.RoleWithAppTablePermissions_Table_FieldPermission{fieldPermission}
		} else {
			fields[fieldPermission.TableSlug] = append(fields[fieldPermission.TableSlug], fieldPermission)
		}
	}

	for _, viewPermission := range ViewPermissions {
		if _, ok := view_relation_permission[viewPermission.TableSlug]; !ok {
			view_relation_permission[viewPermission.TableSlug] = []*nb.RoleWithAppTablePermissions_Table_ViewPermission{viewPermission}
		} else {
			view_relation_permission[viewPermission.TableSlug] = append(view_relation_permission[viewPermission.TableSlug], viewPermission)
		}
	}

	for _, tableViewPermission := range tableViewPermission {

		if _, ok := table_view_permission[tableViewPermission.TableSlug]; !ok {
			table_view_permission[tableViewPermission.TableSlug] = []*models.TableViewPermission{tableViewPermission}
		} else {
			table_view_permission[tableViewPermission.TableSlug] = append(table_view_permission[tableViewPermission.TableSlug], tableViewPermission)
		}
	}

	for _, el := range ActionPermissions {
		if el.GetGuid() != "" && actionPermission[el.TableSlug] == nil {
			actionPermission[el.TableSlug] = []*nb.RoleWithAppTablePermissions_Table_ActionPermission{el}
		} else if el.Guid != "" {
			actionPermission[el.TableSlug] = append(actionPermission[el.TableSlug], el)
		}
	}

	for _, table := range tables {
		tableCopy := nb.RoleWithAppTablePermissions_Table{
			Id:                table.Id,
			Slug:              table.Slug,
			Label:             table.Label,
			RecordPermissions: table.RecordPermissions,
			AutomaticFilters:  &nb.RoleWithAppTablePermissions_Table_AutomaticFilterWithMethod{},
			CustomPermission: &nb.RoleWithAppTablePermissions_Table_CustomPermission{
				ViewCreate:   table.CustomPermission.ViewCreate,
				ShareModal:   table.CustomPermission.ShareModal,
				Settings:     table.CustomPermission.Settings,
				Automation:   table.CustomPermission.Automation,
				LanguageBtn:  table.CustomPermission.LanguageBtn,
				PdfAction:    table.CustomPermission.PdfAction,
				AddField:     table.CustomPermission.AddField,
				DeleteAll:    table.CustomPermission.DeleteAll,
				AddFilter:    table.CustomPermission.AddFilter,
				FieldFilter:  table.CustomPermission.FieldFilter,
				FixColumn:    table.CustomPermission.FixColumn,
				TabGroup:     table.CustomPermission.TabGroup,
				Columns:      table.CustomPermission.Columns,
				Group:        table.CustomPermission.Group,
				ExcelMenu:    table.CustomPermission.ExcelMenu,
				SearchButton: table.CustomPermission.SearchButton,
			},
		}

		if tableCopy.RecordPermissions == nil {
			tableCopy.RecordPermissions = &nb.RoleWithAppTablePermissions_Table_RecordPermission{
				Read:            "No",
				Write:           "No",
				Delete:          "No",
				Update:          "No",
				IsHaveCondition: false,
				IsPublic:        false,
			}
		}

		tableFields := fields[table.Slug]
		tableCopy.FieldPermissions = []*nb.RoleWithAppTablePermissions_Table_FieldPermission{}

		for _, field := range tableFields {
			var fieldPermission nb.RoleWithAppTablePermissions_Table_FieldPermission
			if field.GetGuid() != "" {
				temp := field
				fieldPermission = nb.RoleWithAppTablePermissions_Table_FieldPermission{
					FieldId:        temp.FieldId,
					TableSlug:      table.Slug,
					ViewPermission: temp.ViewPermission,
					EditPermission: temp.EditPermission,
					Label:          field.Label,
					Attributes:     field.Attributes,
				}
			} else {
				fieldPermission = nb.RoleWithAppTablePermissions_Table_FieldPermission{
					FieldId:        field.FieldId,
					TableSlug:      table.Slug,
					ViewPermission: false,
					EditPermission: false,
					Label:          field.Label,
					Guid:           "",
					Attributes:     field.Attributes,
				}
			}
			tableCopy.FieldPermissions = append(tableCopy.FieldPermissions, &fieldPermission)
		}

		for _, el := range view_relation_permission[table.Slug] {
			var viewPermissionEntry nb.RoleWithAppTablePermissions_Table_ViewPermission

			if el.GetGuid() != "" {
				temp := el
				viewPermissionEntry = nb.RoleWithAppTablePermissions_Table_ViewPermission{
					Guid:             temp.Guid,
					RelationId:       temp.RelationId,
					TableSlug:        temp.TableSlug,
					ViewPermission:   temp.ViewPermission,
					EditPermission:   temp.EditPermission,
					CreatePermission: temp.CreatePermission,
					DeletePermission: temp.DeletePermission,
					Label:            el.Label,
					Attributes:       el.Attributes,
				}
			} else {
				viewPermissionEntry = nb.RoleWithAppTablePermissions_Table_ViewPermission{
					Guid:             "",
					RelationId:       el.RelationId,
					TableSlug:        el.TableSlug,
					ViewPermission:   false,
					EditPermission:   false,
					CreatePermission: false,
					DeletePermission: false,
					Label:            el.Label,
					Attributes:       el.Attributes,
				}
			}
			tableCopy.ViewPermissions = append(tableCopy.ViewPermissions, &viewPermissionEntry)
		}

		for _, el := range table_view_permission[table.Slug] {
			var tableViewPermissionEntry nb.RoleWithAppTablePermissions_Table_TableViewPermission

			if el.Guid != "" {
				temp := el
				tableViewPermissionEntry = nb.RoleWithAppTablePermissions_Table_TableViewPermission{
					Guid:       temp.Guid,
					View:       temp.View,
					Edit:       temp.Edit,
					Delete:     temp.Delete,
					ViewId:     temp.ViewId,
					Attributes: el.Attributes,
				}
			} else {
				tableViewPermissionEntry = nb.RoleWithAppTablePermissions_Table_TableViewPermission{
					Guid:       "",
					View:       false,
					Edit:       false,
					Delete:     false,
					ViewId:     el.ViewId,
					Attributes: el.Attributes,
				}
			}
			tableCopy.TableViewPermissions = append(tableCopy.TableViewPermissions, &tableViewPermissionEntry)
		}

		if actionPermission[table.Slug] != nil {
			tableCopy.ActionPermissions = actionPermission[table.Slug]
		} else {
			tableCopy.ActionPermissions = []*nb.RoleWithAppTablePermissions_Table_ActionPermission{}
		}

		tableCopy.AutomaticFilters.Read = automaticFilter[table.Slug].Read
		tableCopy.AutomaticFilters.Write = automaticFilter[table.Slug].Write
		tableCopy.AutomaticFilters.Update = automaticFilter[table.Slug].Update
		tableCopy.AutomaticFilters.Delete = automaticFilter[table.Slug].Delete

		tablesList = append(tablesList, &tableCopy)
	}

	queryGlobalPermission := `
	SELECT
    	guid,
    	menu_button,
    	chat,
    	settings_button,
    	project_settings_button,
    	profile_settings_button,
    	menu_setting_button,
    	redirects_button,
    	api_keys_button,
    	environments_button,
    	projects_button,
    	version_button,
    	project_button,
    	sms_button,
		chatwoot_button,
		gitbook_button,
		gpt_button,
		billing,
		COALESCE(menu_drag, true)
	FROM global_permission
	WHERE role_id = $1
	`

	err = conn.QueryRow(ctx, queryGlobalPermission, req.GetRoleId()).Scan(
		&globalPermission.Id,
		&globalPermission.MenuButton,
		&globalPermission.Chat,
		&globalPermission.SettingsButton,
		&globalPermission.ProjectSettingsButton,
		&globalPermission.ProfileSettingsButton,
		&globalPermission.MenuSettingButton,
		&globalPermission.RedirectsButton,
		&globalPermission.ApiKeysButton,
		&globalPermission.EnvironmentsButton,
		&globalPermission.ProjectsButton,
		&globalPermission.VersionButton,
		&globalPermission.ProjectButton,
		&globalPermission.SmsButton,
		&globalPermission.ChatwootButton,
		&globalPermission.GitbookButton,
		&globalPermission.GptButton,
		&globalPermission.Billing,
		&globalPermission.MenuDrag,
	)
	if err != nil {
		return &nb.GetListWithRoleAppTablePermissionsResponse{}, err
	}

	response.ProjectId = req.GetProjectId()
	response.Guid = roleCopy.Guid
	response.ClientPlatformId = roleCopy.ClientPlatformId
	response.ClientTypeId = roleCopy.ClientTypeId
	response.Name = roleCopy.Name
	response.GlobalPermission = &globalPermission
	response.Tables = tablesList

	return &nb.GetListWithRoleAppTablePermissionsResponse{
		Data: &response,
	}, nil
}

func (p *permissionRepo) UpdateRoleAppTablePermissions(ctx context.Context, req *nb.UpdateRoleAppTablePermissionsRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.UpdateRoleAppTablePermissions")
	defer dbSpan.Finish()

	if req.Data == nil {
		return errors.New("UpdateRoleAppTablePermissions: data is required")
	}
	if req.Data.GlobalPermission == nil {
		return errors.New("UpdateRoleAppTablePermissions: global_permission is required")
	}

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "UpdateRoleAppTablePermissions: begin transaction")
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	query := `UPDATE "role" SET "name" = $1 WHERE guid = $2`

	_, err = tx.Exec(ctx, query, req.Data.Name, req.Data.Guid)
	if err != nil {
		return errors.Wrap(err, "UpdateRoleAppTablePermissions: update role")
	}

	gP := req.Data.GlobalPermission

	globalPermission := `UPDATE "global_permission" SET
		chat = $2,
		menu_button = $3,
		settings_button = $4,
		projects_button = $5,
		environments_button = $6,
		api_keys_button = $7,
		menu_setting_button = $8,
		redirects_button = $9,
		profile_settings_button = $10,
		project_settings_button = $11,
		project_button = $12,
		sms_button = $13,
		version_button = $14,
		chatwoot_button = $15,
		gitbook_button = $16,
		gpt_button = $17,
		billing = $18,
		menu_drag = $19
	WHERE guid = $1
	`

	_, err = tx.Exec(ctx, globalPermission, gP.Id,
		gP.Chat,
		gP.MenuButton,
		gP.SettingsButton,
		gP.ProjectsButton,
		gP.EnvironmentsButton,
		gP.ApiKeysButton,
		gP.MenuSettingButton,
		gP.RedirectsButton,
		gP.ProfileSettingsButton,
		gP.ProjectSettingsButton,
		gP.ProjectButton,
		gP.SmsButton,
		gP.VersionButton,
		gP.ChatwootButton,
		gP.GitbookButton,
		gP.GptButton,
		gP.Billing,
		gP.MenuDrag,
	)
	if err != nil {
		return errors.Wrap(err, "UpdateRoleAppTablePermissions: update global permission")
	}

	recordPermission := `UPDATE "record_permission" SET 
		read = $2,
		write = $3,
		update = $4,
		delete = $5,
		is_public = $6,
		search_button = $7,
		pdf_action = $8,
		add_field = $9,
		language_btn = $10,
		view_create = $11,
		automation = $12,
		settings = $13,
		share_modal = $14,
		add_filter = $15,
		field_filter = $16,
		fix_column = $17,
		tab_group = $18,
		columns = $19,
		"group" = $20,
		excel_menu = $21,
		is_have_condition = $23
	WHERE table_slug = $1 AND role_id = $22
	`

	recordPermissionInsert := `INSERT INTO "record_permission" (
		table_slug, 
		role_id, 
		read, 
		write, 
		update, 
		delete, 
		is_public,
		search_button,
		pdf_action,
		add_field,
		language_btn,
		view_create,
		automation,
		settings,
		share_modal,
		add_filter,
		field_filter,
		fix_column,
		tab_group,
		columns,
		"group",
		excel_menu,
		is_have_condition
	) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23)`

	fieldPermission := `UPDATE "field_permission" SET
		edit_permission = $2,
		view_permission = $3
	WHERE field_id = $1 AND role_id = $4
	`

	fieldPermissionInsert := `INSERT INTO "field_permission" (field_id, role_id, edit_permission, view_permission) VALUES ($1,$2,$3,$4)`

	viewPermission := `UPDATE "view_permission" SET
		view = $2,
		edit = $3,
		delete = $4
	WHERE guid = $1`

	automaticFilterInsert := `INSERT INTO "automatic_filter"
	(guid, table_slug, custom_field, object_field, role_id, method, not_use_in_tab)
	VALUES ($1, $2, $3, $4, $5, $6, $7)`

	deleteQuery := `DELETE FROM "automatic_filter" WHERE role_id = $1`
	_, err = tx.Exec(ctx, deleteQuery, req.Data.Guid)
	if err != nil {
		return errors.Wrap(err, "UpdateRoleAppTablePermissions: delete automatic_filter")
	}

	for _, table := range req.Data.Tables {
		var (
			isHaveCondition bool
			rp              = table.RecordPermissions
			cp              = table.CustomPermission
		)

		if rp == nil || cp == nil {
			continue
		}

		// AutomaticFilter method: Read
		for _, read := range table.GetAutomaticFilters().GetRead() {
			id := read.GetGuid()
			if read.GetGuid() == "" {
				id = uuid.NewString()
			}
			_, err := tx.Exec(ctx, automaticFilterInsert,
				id,
				table.Slug,
				read.CustomField,
				read.ObjectField,
				req.Data.Guid,
				"read",
				read.NotUseInTab,
			)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: insert automatic_filters method: read")
			}
			isHaveCondition = true
		}

		// AutomaticFilter method: Update
		for _, update := range table.GetAutomaticFilters().GetUpdate() {
			id := update.GetGuid()
			if update.GetGuid() == "" {
				id = uuid.NewString()
			}
			_, err := tx.Exec(ctx, automaticFilterInsert,
				id,
				table.Slug,
				update.CustomField,
				update.ObjectField,
				req.Data.Guid,
				"update",
				update.NotUseInTab,
			)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: insert automatic_filters")
			}
			isHaveCondition = true

		}

		// AutomaticFilter method: Write
		for _, write := range table.GetAutomaticFilters().GetWrite() {
			id := write.GetGuid()
			if write.GetGuid() == "" {
				id = uuid.NewString()
			}

			_, err := tx.Exec(ctx, automaticFilterInsert,
				id,
				table.Slug,
				write.CustomField,
				write.ObjectField,
				req.Data.Guid,
				"write",
				write.NotUseInTab,
			)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: insert automatic_filters method: write")
			}
			isHaveCondition = true
		}

		// AutomaticFilter method: Delete
		for _, delete := range table.GetAutomaticFilters().GetDelete() {
			id := delete.GetGuid()
			if delete.GetGuid() == "" {
				id = uuid.NewString()
			}

			_, err := tx.Exec(ctx, automaticFilterInsert,
				id,
				table.Slug,
				delete.CustomField,
				delete.ObjectField,
				req.Data.Guid,
				"delete",
				delete.NotUseInTab,
			)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: insert automatic_filters method: delete")
			}
			isHaveCondition = true
		}

		rec, err := tx.Exec(ctx, recordPermission, table.Slug,
			rp.Read,
			rp.Write,
			rp.Update,
			rp.Delete,
			rp.IsPublic,
			cp.SearchButton,
			cp.PdfAction,
			cp.AddField,
			cp.LanguageBtn,
			cp.ViewCreate,
			cp.Automation,
			cp.Settings,
			cp.ShareModal,
			cp.AddFilter,
			cp.FieldFilter,
			cp.FixColumn,
			cp.TabGroup,
			cp.Columns,
			cp.Group,
			cp.ExcelMenu,
			req.Data.Guid,
			isHaveCondition,
		)
		if err != nil {
			return errors.Wrap(err, "UpdateRoleAppTablePermissions: update record permission")
		}
		if rec.RowsAffected() == 0 {
			_, err := tx.Exec(ctx, recordPermissionInsert, table.Slug, req.Data.Guid,
				rp.Read,
				rp.Write,
				rp.Update,
				rp.Delete,
				rp.IsPublic,
				cp.SearchButton,
				cp.PdfAction,
				cp.AddField,
				cp.LanguageBtn,
				cp.ViewCreate,
				cp.Automation,
				cp.Settings,
				cp.ShareModal,
				cp.AddFilter,
				cp.FieldFilter,
				cp.FixColumn,
				cp.TabGroup,
				cp.Columns,
				cp.Group,
				cp.ExcelMenu,
				isHaveCondition,
			)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: insert record permission")
			}
		}

		for _, fp := range table.FieldPermissions {

			fip, err := tx.Exec(ctx, fieldPermission, fp.FieldId, fp.EditPermission, fp.ViewPermission, req.Data.Guid)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: update field permission")
			}

			if fip.RowsAffected() == 0 {
				_, err := tx.Exec(ctx, fieldPermissionInsert, fp.FieldId, req.Data.Guid, fp.EditPermission, fp.ViewPermission)
				if err != nil {
					return errors.Wrap(err, "UpdateRoleAppTablePermissions: insert field permission")
				}
			}
		}

		for _, vP := range table.ViewPermissions {
			_, err = tx.Exec(ctx, viewPermission, vP.Guid, vP.ViewPermission, vP.EditPermission, vP.DeletePermission)
			if err != nil {
				return errors.Wrap(err, "UpdateRoleAppTablePermissions: update view permission")
			}
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "UpdateRoleAppTablePermissions: commit transaction")
	}

	return nil
}

func (p *permissionRepo) UpdateMenuPermissions(ctx context.Context, req *nb.UpdateMenuPermissionsRequest) error {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.UpdateMenuPermissions")
	defer dbSpan.Finish()

	var (
		values = []string{}
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	for _, v := range req.Menus {
		values = append(values, fmt.Sprintf("('%v', '%v', %v, '%v', %v, %v, %v, %v)",
			v.Id, req.RoleId, v.Permission.Delete,
			uuid.NewString(), v.Permission.MenuSettings, v.Permission.Read,
			v.Permission.Update, v.Permission.Write,
		))
	}

	if len(values) != 0 {
		query := fmt.Sprintf(`
		INSERT INTO menu_permission (menu_id, role_id, delete, guid, menu_settings, read, update, write)
		VALUES %s
		ON CONFLICT (menu_id, role_id) DO UPDATE
		SET
			delete = EXCLUDED.delete,
			guid = EXCLUDED.guid,
			menu_settings = EXCLUDED.menu_settings,
			read = EXCLUDED.read,
			update = EXCLUDED.update,
			write = EXCLUDED.write
	`, strings.Join(values, ", "))

		_, err := conn.Exec(ctx, query)
		if err != nil {
			return errors.Wrap(err, "UpdateMenuPermissions: exec query")
		}
	}

	return nil
}

func (p *permissionRepo) GetPermissionsByTableSlug(ctx context.Context, req *nb.GetPermissionsByTableSlugRequest) (resp *nb.GetPermissionsByTableSlugResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.GetPermissionsByTableSlug")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return nil, err
	}

	currentUserPermission, err := getTablePermission(conn, req.CurrentRoleId, req.TableSlug, ctx)
	if err != nil {
		return &nb.GetPermissionsByTableSlugResponse{}, err
	}

	if req.RoleId == "" || req.RoleId == req.CurrentRoleId {
		return &nb.GetPermissionsByTableSlugResponse{
			CurrentUserPermission: &nb.UpdatePermissionsRequest{
				Table:     currentUserPermission,
				ProjectId: req.ProjectId,
			},
		}, nil
	}

	selectedUserPermission, err := getTablePermission(conn, req.RoleId, req.TableSlug, ctx)
	if err != nil {
		return &nb.GetPermissionsByTableSlugResponse{}, err
	}

	return &nb.GetPermissionsByTableSlugResponse{
		CurrentUserPermission: &nb.UpdatePermissionsRequest{
			Table:     currentUserPermission,
			ProjectId: req.ProjectId,
		},
		SelectedUserPermission: &nb.UpdatePermissionsRequest{
			Table:     selectedUserPermission,
			ProjectId: req.ProjectId,
		},
	}, nil
}

func (p *permissionRepo) UpdatePermissionsByTableSlug(ctx context.Context, req *nb.UpdatePermissionsRequest) (err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.UpdatePermissionsByTableSlug")
	defer dbSpan.Finish()

	var (
		roleId = req.Guid
		count  = 0
	)

	conn, err := psqlpool.Get(req.GetProjectId())
	if err != nil {
		return err
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		return errors.Wrap(err, "UpdatePermissionsByTableSlug: begin transaction")
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback(ctx)
		}
	}()

	if len(roleId) == 0 {
		return errors.Wrap(err, "role_id is required")
	}

	query := `SELECT COUNT(*) FROM role WHERE guid = $1`
	err = tx.QueryRow(ctx, query, roleId).Scan(&count)
	if err != nil {
		return errors.Wrap(err, "UpdatePermissionsByTableSlug: select count")
	}
	if count == 0 {
		return errors.Wrap(err, "role not found")
	}

	query = `UPDATE record_permission SET
		read = $3,
		write = $4,
		update = $5,
		delete = $6
	WHERE role_id = $1 AND table_slug = $2`

	_, err = tx.Exec(ctx, query,
		roleId,
		req.Table.Slug,
		req.Table.RecordPermissions.Read,
		req.Table.RecordPermissions.Write,
		req.Table.RecordPermissions.Update,
		req.Table.RecordPermissions.Delete,
	)
	if err != nil {
		return errors.Wrap(err, "UpdatePermissionsByTableSlug: update record permission")
	}

	query = `UPDATE field_permission SET
		edit_permission = $3,
		view_permission = $4
	WHERE role_id = $1 AND field_id = $2`

	for _, fp := range req.Table.FieldPermissions {
		_, err := tx.Exec(ctx, query, roleId, fp.FieldId,
			fp.EditPermission,
			fp.ViewPermission,
		)
		if err != nil {
			return errors.Wrap(err, "UpdatePermissionsByTableSlug: update field permission")
		}
	}

	query = `UPDATE action_permission SET
		permission = $3
	WHERE role_id = $1 AND custom_event_id = $2`

	for _, ap := range req.Table.ActionPermissions {
		_, err := tx.Exec(ctx, query, roleId, ap.CustomEventId,
			ap.Permission,
		)
		if err != nil {
			return errors.Wrap(err, "UpdatePermissionsByTableSlug: update action permission")
		}
	}

	if err = tx.Commit(ctx); err != nil {
		return errors.Wrap(err, "UpdatePermissionsByTableSlug: commit transaction")
	}

	return nil
}

func getTablePermission(conn *psqlpool.Pool, roleId, tableSlug string, ctx context.Context) (*nb.UpdatePermissionsRequest_Table, error) {
	query := `
	SELECT
			t.id,
			t.slug,
			t.label,
			jsonb_build_object(
				'guid', rp.guid,
				'read', rp.read,
				'write', rp.write,
				'update', rp.update,
				'delete', rp.delete
			) as record_permissions,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'field_id', fp.field_id,
						'view_permission', fp.view_permission,
						'edit_permission', fp.edit_permission,
						'label', fp.label,
						'table_slug', fp.table_slug
				)) FILTER (WHERE fp.field_id IS NOT NULL), '[]'::jsonb
			) as field_permissions,
			COALESCE(
				jsonb_agg(
					jsonb_build_object(
						'guid', ap.guid,
						'custom_event_id', ap.custom_event_id,
						'permission', ap.permission,
						'label', ap.label,
						'table_slug', ap.table_slug
					)) FILTER (WHERE ap.guid IS NOT NULL), '[]'::jsonb
			) as action_permissions
	FROM "table" t 
	LEFT JOIN record_permission rp ON rp.table_slug = t.slug AND rp.role_id = $1
	LEFT JOIN field_permission fp ON fp.table_slug = t.slug AND fp.role_id = $1
	LEFT JOIN action_permission ap ON ap.table_slug = t.slug AND ap.role_id = $1
	WHERE t.slug = $2 GROUP BY t.id, rp.guid`

	var (
		table = &nb.UpdatePermissionsRequest_Table{
			RecordPermissions: &nb.UpdatePermissionsRequest_Table_RecordPermission{},
		}
		recordPermissions = []byte{}
		fieldPermissions  = []byte{}
		actionPermissions = []byte{}
	)

	err := conn.QueryRow(ctx, query, roleId, tableSlug).Scan(
		&table.Id,
		&table.Slug,
		&table.Label,
		&recordPermissions,
		&fieldPermissions,
		&actionPermissions,
	)
	if err != nil {
		return &nb.UpdatePermissionsRequest_Table{}, err
	}

	if err := json.Unmarshal(recordPermissions, &table.RecordPermissions); err != nil {
		return &nb.UpdatePermissionsRequest_Table{}, err
	}
	if err := json.Unmarshal(fieldPermissions, &table.FieldPermissions); err != nil {
		return &nb.UpdatePermissionsRequest_Table{}, err
	}
	if err := json.Unmarshal(actionPermissions, &table.ActionPermissions); err != nil {
		return &nb.UpdatePermissionsRequest_Table{}, err
	}

	return table, nil
}

func (b *permissionRepo) GetTablePermission(ctx context.Context, req *nb.GetTablePermissionRequest) (*nb.GetTablePermissionResponse, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "permission.GetPermissionsByTableSlug")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvironmentId())
	if err != nil {
		return nil, err
	}

	var roleId any = req.RoleId

	if req.RoleId == "" {
		roleId = nil
	}

	if req.TableSlug == "template" {
		return &nb.GetTablePermissionResponse{IsHavePermission: true}, nil
	}

	query := fmt.Sprintf(`
	SELECT EXISTS (
        SELECT 1 FROM record_permission
        WHERE table_slug = $1
        AND %s = 'Yes'
        AND (
            (role_id = $2)
            OR (role_id IS NULL)
            OR (is_public = true)
        )
    )`, req.Method)

	var hasPermission bool
	err = conn.QueryRow(ctx, query, req.TableSlug, roleId).Scan(&hasPermission)
	if err != nil {
		return &nb.GetTablePermissionResponse{IsHavePermission: false}, err
	}

	return &nb.GetTablePermissionResponse{
		IsHavePermission: hasPermission,
	}, nil
}

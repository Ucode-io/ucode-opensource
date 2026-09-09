package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	nb "github.com/Ucode-io/ucode-opensource/services/object-builder/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/models"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/security"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"

	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/structpb"
)

type loginRepo struct {
	db *psqlpool.Pool
}

func NewLoginRepo(db *psqlpool.Pool) storage.LoginRepoI {
	return &loginRepo{
		db: db,
	}
}

func (l *loginRepo) LoginData(ctx context.Context, req *nb.LoginDataReq) (resp *nb.LoginDataRes, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "login.LoginData")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvironmentId())
	if err != nil {
		return nil, err
	}

	var (
		clientType                     models.ClientType
		tableSlug                      = `user`
		userId, roleId, guid           string
		userFound, comparePassword     bool
		role                           models.Role
		clientPlatform                 models.ClientPlatform
		connections                    = []*nb.TableClientType{}
		permissions                    = []*nb.RecordPermission{}
		tableSlugNull, defaultPageNull sql.NullString
		globalPermission               = &nb.GlobalPermission{}
		errResp                        = &nb.LoginDataRes{UserFound: false}
	)

	query := `
		SELECT
			"guid",
			COALESCE("project_id"::varchar, ''),
			"name",
			"self_register",
			"self_recover",
			"client_platform_ids",
			"confirm_by",
			"is_system",
			"table_slug",
			"default_page",
			"session_limit"
		FROM client_type WHERE "guid" = $1 OR "name" = $1::varchar
	`

	err = conn.QueryRow(ctx, query, req.ClientType).Scan(
		&clientType.Guid,
		&clientType.ProjectId,
		&clientType.Name,
		&clientType.SelfRegister,
		&clientType.SelfRecover,
		&clientType.ClientPlatformIds,
		&clientType.ConfirmBy,
		&clientType.IsSystem,
		&tableSlugNull,
		&defaultPageNull,
		&clientType.SessionLimit,
	)
	if err != nil {
		return errResp, errors.Wrap(err, "error getting client type")
	}

	clientType.TableSlug = tableSlugNull.String
	clientType.DefaultPage = defaultPageNull.String

	if clientType.TableSlug != "" && clientType.TableSlug != "user" {
		tableSlug = clientType.TableSlug
	}

	if req.UserId == "" {
		return errResp, nil
	}

	userInfo, err := helper.GetItemLogin(ctx, conn, tableSlug, req.UserId, req.ClientType)
	if err != nil {
		return errResp, nil
	}

	if len(userInfo) == 0 {
		return errResp, nil
	}

	guid = cast.ToString(userInfo["guid"])
	userId = cast.ToString(userInfo["user_id_auth"])
	roleId = cast.ToString(userInfo["role_id"])

	if userId != "" {
		userFound = true
		if len(req.GetPassword()) != 0 {
			var attrData []byte

			query = `SELECT attributes FROM "table" where slug = $1`
			if err := conn.QueryRow(ctx, query, tableSlug).Scan(&attrData); err != nil {
				return errResp, nil
			}

			var attrDataStruct *structpb.Struct
			if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
				return errResp, nil
			}

			attrDataMap, err := helper.ConvertStructToMap(attrDataStruct)
			if err != nil {
				return errResp, nil
			}

			authInfo, ok := attrDataMap["auth_info"].(map[string]any)
			if !ok {
				return errResp, nil
			}

			checkPassword, err := security.ComparePasswordBcrypt(cast.ToString(userInfo[cast.ToString(authInfo["password"])]), req.Password)
			if err != nil {
				return &nb.LoginDataRes{UserFound: false, ComparePassword: false}, nil
			}

			if !checkPassword {
				return &nb.LoginDataRes{UserFound: false, ComparePassword: false}, nil
			}
			comparePassword = true
		} else {
			comparePassword = true
		}
	}

	query = `SELECT 
		"guid",
		"name",
		"project_id",
		COALESCE("client_platform_id"::varchar, ''),
		"client_type_id"
	FROM "role" WHERE "guid" = $1`

	err = conn.QueryRow(ctx, query, roleId).Scan(
		&role.Guid,
		&role.Name,
		&role.ProjectId,
		&role.ClientPlatformId,
		&role.ClientTypeId,
	)
	if err != nil {
		return errResp, errors.Wrap(err, "error getting role")
	}

	query = `SELECT 
		"guid",
		"name",
		"project_id",
		"subdomain"
	FROM "client_platform" WHERE "guid" = $1
	`

	if role.ClientPlatformId != "" {
		err = conn.QueryRow(ctx, query, role.ClientPlatformId).Scan(
			&clientPlatform.Guid,
			&clientPlatform.Name,
			&clientPlatform.ProjectId,
			&clientPlatform.Subdomain,
		)
		if err != nil {
			return errResp, errors.Wrap(err, "error getting client platform")
		}
	}

	query = `SELECT 
		"table_slug",
		"view_slug",
		"view_label",
		"icon",
		"name"
	FROM "connections" WHERE deleted_at IS NULL AND client_type_id = $1`

	rows, err := conn.Query(ctx, query, clientType.Guid)
	if err != nil {
		return errResp, errors.Wrap(err, "error getting connections")
	}
	defer rows.Close()

	for rows.Next() {
		var slug, viewSlug, viewLabel, icon, label sql.NullString

		err = rows.Scan(
			&slug,
			&viewSlug,
			&viewLabel,
			&icon,
			&label,
		)
		if err != nil {
			return errResp, errors.Wrap(err, "error scanning connections")
		}

		connections = append(connections, &nb.TableClientType{
			Slug:      slug.String,
			ViewSlug:  viewSlug.String,
			ViewLabel: viewLabel.String,
			Icon:      icon.String,
			Label:     label.String,
		})
	}

	query = `SELECT 
		"guid",
		"role_id",
		"read",
		"write",
		"update",
		"delete",
		"table_slug",
		"automation",
		"language_btn",
		"settings",
		"share_modal",
		"view_create",
		"add_field",
		"pdf_action",
		add_filter,
		field_filter,
		fix_column,
		tab_group,
		columns,
		"group",
		excel_menu,
		search_button
	FROM "record_permission" WHERE role_id = $1`

	recPermissions, err := conn.Query(ctx, query, roleId)
	if err != nil {
		return errResp, errors.Wrap(err, "error getting record permissions")
	}
	defer recPermissions.Close()

	for recPermissions.Next() {
		var permission nb.RecordPermission

		err = recPermissions.Scan(
			&permission.Guid,
			&permission.RoleId,
			&permission.Read,
			&permission.Write,
			&permission.Update,
			&permission.Delete,
			&permission.TableSlug,
			&permission.Automation,
			&permission.LanguageBtn,
			&permission.Settings,
			&permission.ShareModal,
			&permission.ViewCreate,
			&permission.AddField,
			&permission.PdfAction,
			&permission.AddFilter,
			&permission.FieldFilter,
			&permission.FixColumn,
			&permission.TabGroup,
			&permission.Columns,
			&permission.Group,
			&permission.ExcelMenu,
			&permission.SearchButton,
		)
		if err != nil {
			return errResp, errors.Wrap(err, "error scanning record permissions")
		}

		permissions = append(permissions, &permission)
	}

	query = `
		SELECT 
			"guid",
			"chat",
			"menu_button",
			"settings_button",
			"projects_button",
			"environments_button",
			"api_keys_button",
			"menu_setting_button",
			"redirects_button",
			"profile_settings_button",
			"project_settings_button",
			"project_button",
			"sms_button",
				"version_button",
				"chatwoot_button",
				"gitbook_button",
				"gpt_button",
				"billing",
				COALESCE("menu_drag", true)
			FROM global_permission
			WHERE role_id = $1
		LIMIT 1
	`

	err = conn.QueryRow(ctx, query, roleId).Scan(
		&globalPermission.Id,
		&globalPermission.Chat,
		&globalPermission.MenuButton,
		&globalPermission.SettingsButton,
		&globalPermission.ProjectsButton,
		&globalPermission.EnvironmentsButton,
		&globalPermission.ApiKeysButton,
		&globalPermission.MenuSettingButton,
		&globalPermission.RedirectsButton,
		&globalPermission.ProfileSettingsButton,
		&globalPermission.ProjectSettingsButton,
		&globalPermission.ProjectButton,
		&globalPermission.SmsButton,
		&globalPermission.VersionButton,
		&globalPermission.ChatwootButton,
		&globalPermission.GitbookButton,
		&globalPermission.GptButton,
		&globalPermission.Billing,
		&globalPermission.MenuDrag,
	)
	if err != nil {
		return errResp, errors.Wrap(err, "error getting global permission")
	}

	userdata, err := helper.ConvertMapToStruct(userInfo)
	if err != nil {
		return errResp, errors.Wrap(err, "convert map to struct")
	}

	return &nb.LoginDataRes{
		UserFound:       userFound,
		ComparePassword: comparePassword,
		UserId:          guid,
		LoginTableSlug:  tableSlug,
		ClientType: &nb.ClientType{
			Guid:         clientType.Guid,
			Name:         clientType.Name,
			ConfirmBy:    nb.ConfirmStrategies(0),
			SelfRegister: clientType.SelfRegister,
			SelfRecover:  clientType.SelfRecover,
			ProjectId:    clientType.ProjectId,
			Tables:       connections,
			DefaultPage:  clientType.DefaultPage,
			SessionLimit: clientType.SessionLimit,
		},
		Role: &nb.Role{
			Guid:             role.Guid,
			ClientTypeId:     role.ClientTypeId,
			Name:             role.Name,
			ClientPlatformId: role.ClientPlatformId,
			ProjectId:        role.ProjectId,
		},
		ClientPlatform: &nb.ClientPlatform{
			Guid:      clientPlatform.Guid,
			Name:      clientPlatform.Name,
			ProjectId: clientPlatform.ProjectId,
			Subdomain: clientPlatform.Subdomain,
		},
		Permissions:      permissions,
		GlobalPermission: globalPermission,
		UserIdAuth:       userId,
		UserData:         userdata,
	}, nil
}

func (l *loginRepo) GetConnectionOptions(ctx context.Context, req *nb.GetConnetionOptionsRequest) (resp *nb.GetConnectionOptionsResponse, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "login.GetConnectionOptions")
	defer dbSpan.Finish()

	var (
		options    []map[string]any
		connection models.Connection
		clientType models.ClientType
		user       map[string]any
	)

	conn, err := psqlpool.Get(req.GetResourceEnvironmentId())
	if err != nil {
		return nil, err
	}

	query := `SELECT table_slug, field_slug, client_type_id FROM "connections" WHERE guid = $1`
	err = conn.QueryRow(ctx, query, req.ConnectionId).Scan(&connection.TableSlug, &connection.FieldSlug, &connection.ClientTypeId)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get connection")
	}

	if connection.TableSlug != "" && connection.FieldSlug != "" {
		query = `SELECT table_slug FROM client_type WHERE guid = $1`
		err = conn.QueryRow(ctx, query, connection.ClientTypeId).Scan(&clientType.TableSlug)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get client type")
		}

		tableSlug := "user"
		if clientType.TableSlug != "" {
			tableSlug = clientType.TableSlug
		}

		query = fmt.Sprintf(`SELECT * FROM %s WHERE user_id_auth = $1`, tableSlug)
		rows, err := conn.Query(ctx, query, req.UserId)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get user data")
		}
		defer rows.Close()

		for rows.Next() {
			values, err := rows.Values()
			if err != nil {
				return nil, errors.Wrap(err, "failed to get user values")
			}

			user = make(map[string]any, len(values))
			for i, value := range values {
				fieldName := rows.FieldDescriptions()[i].Name
				if strings.Contains(fieldName, "_id") || fieldName == "guid" {
					if arr, ok := value.([16]uint8); ok {
						value = helper.ConvertGuid(arr)
					}
				}
				user[fieldName] = value
			}
		}

		if user[connection.FieldSlug] != nil || user["guid"] != nil {
			params := make(map[string]any)

			//switch fieldValue := user[connection.FieldSlug].(type) {
			//case nil:
			//	if guid, ok := user[connection.TableSlug+"_id"]; ok {
			//		params["guid"] = guid
			//	}
			//default:
			//	params["guid"] = fieldValue
			//}

			if guid, ok := user[connection.TableSlug+"_id"]; ok {
				params["guid"] = guid
			} else {
				params["guid"] = user[connection.FieldSlug]
			}

			query = fmt.Sprintf(`SELECT * FROM %s WHERE deleted_at IS NULL AND guid = $1`, connection.TableSlug)
			rows, err := conn.Query(ctx, query, params["guid"])
			if err != nil {
				return nil, errors.Wrap(err, "failed to get connection options")
			}
			defer rows.Close()

			for rows.Next() {
				values, err := rows.Values()
				if err != nil {
					return nil, errors.Wrap(err, "failed to get option values")
				}

				option := make(map[string]any, len(values))
				for i, value := range values {
					fieldName := rows.FieldDescriptions()[i].Name
					if strings.Contains(fieldName, "_id") || fieldName == "guid" {
						if arr, ok := value.([16]uint8); ok {
							value = helper.ConvertGuid(arr)
						}
					}
					option[fieldName] = value
				}
				options = append(options, option)
			}
		}
	}

	data, err := helper.ConvertMapToStruct(map[string]any{
		"response": options,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert options to struct")
	}

	return &nb.GetConnectionOptionsResponse{
		TableSlug: connection.TableSlug,
		Data:      data,
	}, nil
}

func (l *loginRepo) UpdateUserPassword(ctx context.Context, req *nb.UpdateUserPasswordRequest) (resp *nb.LoginDataRes, err error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "login.UpdateUserPassword")
	defer dbSpan.Finish()

	conn, err := psqlpool.Get(req.GetResourceEnvironmentId())
	if err != nil {
		return nil, err
	}

	var (
		clientType    models.ClientType
		tableSlug     = "user"
		field         = "password"
		tableSlugNull sql.NullString
	)

	// Get client_type by guid
	query := `
		SELECT
			"guid",
			COALESCE("project_id"::varchar, ''),
			"name",
			"table_slug"
		FROM client_type WHERE "guid" = $1
	`

	err = conn.QueryRow(ctx, query, req.ClientTypeId).Scan(
		&clientType.Guid,
		&clientType.ProjectId,
		&clientType.Name,
		&tableSlugNull,
	)
	if err != nil {
		return nil, errors.Wrap(err, "error getting client type")
	}

	clientType.TableSlug = tableSlugNull.String

	if clientType.TableSlug != "" && clientType.TableSlug != "user" {
		tableSlug = clientType.TableSlug
	}

	// Get table by slug to check is_login_table and get attributes
	var (
		attrData     []byte
		isLoginTable bool
	)

	query = `SELECT attributes, is_login_table FROM "table" WHERE slug = $1`
	err = conn.QueryRow(ctx, query, tableSlug).Scan(&attrData, &isLoginTable)
	if err != nil {
		return nil, errors.Wrap(err, "error getting table")
	}

	// Get password field name from attributes.auth_info.password
	if isLoginTable {
		var attrDataStruct *structpb.Struct
		if err := json.Unmarshal(attrData, &attrDataStruct); err != nil {
			return nil, errors.Wrap(err, "error unmarshalling attributes")
		}

		attrDataMap, err := helper.ConvertStructToMap(attrDataStruct)
		if err != nil {
			return nil, errors.Wrap(err, "error converting struct to map")
		}

		if authInfo, ok := attrDataMap["auth_info"].(map[string]any); ok {
			if passwordField, ok := authInfo["password"].(string); ok && passwordField != "" {
				// Validate field name to prevent SQL injection (only alphanumeric and underscores allowed)
				if strings.ContainsAny(passwordField, " ;--\"'") {
					return nil, errors.New("invalid password field name")
				}
				field = passwordField
			}
		}
	}

	// Hash the password
	hashedPassword, err := security.HashPasswordBcrypt(req.Password)
	if err != nil {
		return nil, errors.Wrap(err, "error hashing password")
	}

	// Update the user record by guid
	query = fmt.Sprintf(`UPDATE "%s" SET %s = $1, updated_at = now() WHERE guid = $2`, tableSlug, field)
	_, err = conn.Exec(ctx, query, hashedPassword, req.Guid)
	if err != nil {
		return nil, errors.Wrap(err, "error updating user password")
	}

	// Get the updated user to return user_id_auth and user_id (guid)
	query = fmt.Sprintf(`SELECT * FROM "%s" WHERE guid = $1`, tableSlug)
	rows, err := conn.Query(ctx, query, req.Guid)
	if err != nil {
		return nil, errors.Wrap(err, "error getting updated user")
	}
	defer rows.Close()

	var userInfo map[string]any
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return nil, errors.Wrap(err, "error getting user values")
		}

		userInfo = make(map[string]any, len(values))
		for i, value := range values {
			fieldName := rows.FieldDescriptions()[i].Name
			if strings.Contains(fieldName, "_id") || fieldName == "guid" {
				if arr, ok := value.([16]uint8); ok {
					value = helper.ConvertGuid(arr)
				}
			}
			userInfo[fieldName] = value
		}
	}

	if len(userInfo) == 0 {
		return nil, errors.New("user not found after update")
	}

	userIdAuth := cast.ToString(userInfo["user_id_auth"])
	if userIdAuth == "" {
		userIdAuth = cast.ToString(userInfo["guid"])
	}

	userGuid := cast.ToString(userInfo["guid"])

	userdata, err := helper.ConvertMapToStruct(userInfo)
	if err != nil {
		return nil, errors.Wrap(err, "error converting map to struct")
	}

	return &nb.LoginDataRes{
		UserId:     userGuid,
		UserIdAuth: userIdAuth,
		UserData:   userdata,
	}, nil
}

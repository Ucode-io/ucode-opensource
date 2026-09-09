package models

import (
	"github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"
)

type (
	LoginMiddlewareReq struct {
		Data      map[string]string      `json:"data"`
		Tables    []*auth_service.Object `json:"tables"`
		NodeType  string                 `json:"node_type"`
		ClientId  string                 `json:"client_id"`
		ClientIp  string                 `json:"client_ip"`
		UserAgent string                 `json:"user_agent"`
	}

	LoginResponse struct {
		UserFound      bool                         `json:"user_found"`
		ClientPlatform *auth_service.ClientPlatform `json:"client_platform"`
		ClientType     *ClientType                  `json:"client_type"`
		User           *auth_service.User           `json:"user"`
		Role           *auth_service.Role           `json:"role"`
		Token          *auth_service.Token          `json:"token"`
		Permissions    []*auth_service.Permission   `json:"permissions"`
		Sessions       []*auth_service.Session      `json:"sessions"`
		Companies      []*auth_service.Company      `json:"companies"`
	}

	V2LoginSuperAdminRes struct {
		UserFound bool                    `json:"user_found"`
		UserId    string                  `json:"user_id"`
		Token     *auth_service.Token     `json:"token"`
		Sessions  []*auth_service.Session `json:"sessions"`
		Companies []*auth_service.Company `json:"companies"`
		UserData  map[string]any          `json:"user_data"`
	}

	V2LoginWithOptionsResponse struct {
		UserFound       bool                             `json:"user_found"`
		ClientPlatform  *auth_service.ClientPlatform     `json:"client_platform"`
		ClientType      *ClientType                      `json:"client_type"`
		UserId          string                           `json:"user_id"`
		Role            *auth_service.Role               `json:"role"`
		Token           *auth_service.Token              `json:"token"`
		Permissions     []*auth_service.RecordPermission `json:"permissions"`
		Sessions        []*auth_service.Session          `json:"sessions"`
		LoginTableSlug  string                           `json:"login_table_slug"`
		AppPermissions  []*auth_service.RecordPermission `json:"app_permissions"`
		ResourceId      string                           `json:"resource_id"`
		EnvironmentId   string                           `json:"environment_id"`
		User            *auth_service.User               `json:"user"`
		Tables          []*auth_service.Object           `json:"tables"`
		AddationalTable map[string]any                   `json:"addational_table"`
		Companies       []*auth_service.Company          `json:"companies"`
		UserData        map[string]any                   `json:"user_data"`
	}

	V2LoginResponse struct {
		UserFound        bool                             `json:"user_found"`
		ClientPlatform   *auth_service.ClientPlatform     `json:"client_platform"`
		ClientType       *ClientType                      `json:"client_type"`
		UserId           string                           `json:"user_id"`
		Role             *auth_service.Role               `json:"role"`
		Token            *auth_service.Token              `json:"token"`
		Permissions      []*auth_service.RecordPermission `json:"permissions"`
		Sessions         []*auth_service.Session          `json:"sessions"`
		LoginTableSlug   string                           `json:"login_table_slug"`
		AppPermissions   []*auth_service.RecordPermission `json:"app_permissions"`
		ResourceId       string                           `json:"resource_id"`
		EnvironmentId    string                           `json:"environment_id"`
		User             *auth_service.User               `json:"user"`
		Tables           []*auth_service.Object           `json:"tables"`
		AddationalTable  map[string]any                   `json:"addational_table"`
		GlobalPermission *auth_service.GlobalPermission   `json:"global_permission"`
		UserData         map[string]any                   `json:"user_data"`
		UserIdAuth       string                           `json:"user_id_auth"`
	}

	MultiCompanyLoginResponse struct {
		Companies   []*auth_service.MultiCompanyLoginResponse_Company `json:"companies"`
		ClientTypes []*ClientType                                     `json:"client_types"`
	}

	V2MultiCompanyOneLoginRes struct {
		Companies []*Company2 `json:"companies"`
		UserId    string      `json:"user_id"`
	}

	Company2 struct {
		Id          string      `json:"id"`
		Name        string      `json:"name"`
		Logo        string      `json:"logo"`
		Description string      `json:"description"`
		CreatedAt   string      `json:"created_at"`
		UpdatedAt   string      `json:"updated_at"`
		OwnerId     string      `json:"owner_id"`
		Projects    []*Project2 `json:"projects"`
	}

	Project2 struct {
		Id                   string                               `json:"id"`
		CompanyId            string                               `json:"company_id"`
		Name                 string                               `json:"name"`
		Domain               string                               `json:"domain"`
		CreatedAt            string                               `json:"created_at"`
		UpdatedAt            string                               `json:"updated_at"`
		ResourceEnvironments []*ResourceEnvironmentV2MultiCompany `json:"resource_environments"`
		ClientTypes          map[string]any                       `json:"client_types"`
	}

	ResourceEnvironmentV2MultiCompany struct {
		Id            string         `json:"id"`
		ProjectId     string         `json:"project_id"`
		ResourceId    string         `json:"resource_id"`
		EnvironmentId string         `json:"environment_id"`
		IsConfigured  bool           `json:"is_configured"`
		ResourceType  int32          `json:"resource_type"`
		ServiceType   int32          `json:"service_type"`
		Name          string         `json:"name"`
		DisplayColor  string         `json:"display_color"`
		Description   string         `json:"description"`
		NodeType      string         `json:"client_types"`
		AccessType    string         `json:"node_type"`
		ClientTypes   map[string]any `json:"access_type"`
	}

	ClientType struct {
		Id           string                         `json:"id"`
		Name         string                         `json:"name"`
		ConfirmBy    auth_service.ConfirmStrategies `json:"confirm_by"`
		SelfRegister bool                           `json:"self_register"`
		SelfRecover  bool                           `json:"self_recover"`
		ProjectId    string                         `json:"project_id"`
	}
)

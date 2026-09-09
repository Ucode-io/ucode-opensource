package airbyte

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/company/config"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cast"
)

var (
	authEndpoint       = "/api/v1/security/login"
	createDbEndpoint   = "/api/v1/database/"
	createRoleEndpoint = "/api/v1/security/roles/"
	createUserEndpoint = "/api/v1/security/users/"
)

func CreateSupersetConnection(in CreateSupersetConnectionRequest) error {
	accessToken, err := getToken(in.Cfg)
	if err != nil {
		return err
	}

	createDbPayload := map[string]any{
		"database_name":        fmt.Sprintf("%s's connection", in.DatabaseName),
		"engine":               "clickhousedb",
		"configuration_method": "dynamic_form",
		"engine_information": map[string]any{
			"disable_ssh_tunneling":    false,
			"supports_dynamic_catalog": false,
			"supports_file_upload":     false,
		},
		"driver":                     "connect",
		"sqlalchemy_uri_placeholder": "clickhousedb://user:password@host[:port][/dbname][?secure=value&=value...]",
		"extra":                      "{\"allows_virtual_table_explore\":true}",
		"expose_in_sqllab":           true,
		"parameters": map[string]any{
			"host":     in.Host,
			"port":     in.Port,
			"database": in.Database,
			"username": in.Username,
			"password": in.Password,
		},
		"masked_encrypted_extra": "{}",
	}
	createDbResponse, err := DoSupersetRequest(http.MethodPost, in.Cfg.SupersetBaseUrl+createDbEndpoint, accessToken, createDbPayload)
	if err != nil {
		return err
	}

	dbId := cast.ToInt(createDbResponse["id"])

	permissionIDsToAdd, err := selectPermissionFromPg(in.Cfg, in.DatabaseName, in.Username, dbId)
	if err != nil {
		return err
	}

	permissionIds := []int{7, 8, 11, 12, 15, 16, 17, 27, 28, 34, 37, 46, 47, 48, 49, 50, 51, 52, 53, 55, 56, 57, 58, 59, 60, 61, 63, 68, 70, 71, 72, 73, 74, 75, 81, 82, 83, 86, 88, 90, 94, 97, 98, 99, 101, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 117, 118, 119, 120, 131, 132, 133, 135, 136, 142, 143, 144, 145, 146, 147, 149, 151, 161, 162, 163, 165, 166, 167, 168, 169}
	permissionIds = append(permissionIds, permissionIDsToAdd...)

	roleId, err := createRole(in.DatabaseName, in.Cfg.SupersetBaseUrl+createRoleEndpoint, accessToken)
	if err != nil {
		return err
	}

	err = updatePermissionForRole(roleId, in.Cfg.SupersetBaseUrl, accessToken, permissionIds)
	if err != nil {
		return err
	}

	err = createUser(in.DatabaseName, in.SupersetName, in.SupersetPassword, in.Cfg.SupersetBaseUrl+createUserEndpoint, accessToken, roleId)
	if err != nil {
		return err
	}

	return nil
}

func getToken(cfg config.Config) (string, error) {
	payload := map[string]any{
		"username": cfg.SupersetUsername,
		"password": cfg.SupersetPassword,
		"provider": "db",
	}

	response, err := DoSupersetRequest(http.MethodPost, cfg.SupersetBaseUrl+authEndpoint, "", payload)
	if err != nil {
		return "", err
	}

	accessToken := cast.ToString(response["access_token"])

	return accessToken, nil
}

func selectPermissionFromPg(cfg config.Config, dbName, dbUsername string, dbId int) ([]int, error) {
	dbURL := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", cfg.SupersetDbUsername, cfg.SupersetDbPassword, cfg.SupersetDbHost, cfg.SupersetDbPort, cfg.SupersetDbUsername)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		return []int{}, err
	}
	defer pool.Close()

	vmName1 := fmt.Sprintf("[%s's connection].(id:%d)", dbName, dbId)
	vmName2 := fmt.Sprintf("[%s's connection].[%s]", dbName, dbUsername)

	query := `
		SELECT 
			(SELECT pvm.id 
			FROM ab_permission_view pvm
			JOIN ab_view_menu vm ON pvm.view_menu_id = vm.id
			WHERE vm.name = $1
			GROUP BY pvm.id
			LIMIT 1) AS id1,
			
			(SELECT pvm.id 
			FROM ab_permission_view pvm
			JOIN ab_view_menu vm ON pvm.view_menu_id = vm.id
			WHERE vm.name = $2
			GROUP BY pvm.id
			LIMIT 1) AS id2
		`

	var permissionID1, permissionID2 int
	err = pool.QueryRow(ctx, query, vmName1, vmName2).Scan(&permissionID1, &permissionID2)
	if err != nil {
		return []int{}, err
	}

	return []int{permissionID1, permissionID2}, nil
}

func createRole(name, url, token string) (int, error) {
	createRolePayload := map[string]any{
		"name": name,
	}

	createRoleResponse, err := DoSupersetRequest(http.MethodPost, url, token, createRolePayload)
	if err != nil {
		return 0, err
	}

	roleId := cast.ToInt(createRoleResponse["id"])

	return roleId, nil
}

func updatePermissionForRole(roleId int, baseUrl, token string, permissionIds []int) error {
	updatePermissionForRoleUrl := fmt.Sprintf("%v/api/v1/security/roles/%v/permissions", baseUrl, roleId)
	updatePermissionPayload := map[string]any{
		"permission_view_menu_ids": permissionIds,
	}

	_, err := DoSupersetRequest(http.MethodPost, updatePermissionForRoleUrl, token, updatePermissionPayload)
	if err != nil {
		return err
	}

	return nil
}

func createUser(name, username, password, url, token string, roleId int) error {
	createUserPayload := map[string]any{
		"first_name": name,
		"last_name":  name,
		"username":   username,
		"email":      name + "@gmail.com",
		"active":     true,
		"password":   password,
		"roles":      []int{roleId},
	}
	_, err := DoSupersetRequest(http.MethodPost, url, token, createUserPayload)
	if err != nil {
		return err
	}

	return nil
}

func DoSupersetRequest(method, url, token string, payload any) (map[string]any, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("[Superset.DoRequest] marshal payload error: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("[Superset.DoRequest] create request error: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[Superset.DoRequest] HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[Superset.DoRequest] read body error: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("[Superset.DoRequest] API error: status=%d url=%s body=%s", resp.StatusCode, url, string(body))
	}

	var result map[string]any
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("[Superset.DoRequest] unmarshal error: %w, body=%s", err, string(body))
	}

	return result, nil
}

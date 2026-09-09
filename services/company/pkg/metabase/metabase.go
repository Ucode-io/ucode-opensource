package metabase

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/spf13/cast"
)

const (
	createGroupEndpoint    = "/api/permissions/group"
	createMbUserEndpoint   = "/api/user"
	createDatabaseEndpoint = "/api/database"
	permissionsEndpoint    = "/api/permissions/graph"
	sessionEndpoint        = "/api/session"
	getDashboardsEndpoint  = "/api/dashboard"
)

func CreateConnection(in CreateConnectionRequest) error {
	createGroup := map[string]any{
		"name": in.DatabaseName,
	}
	createGroupResp, err := DoRequest(
		http.MethodPost,
		in.Cfg.MetabaseBaseUrl+createGroupEndpoint,
		in.Cfg.MetabaseApiKey,
		createGroup, true,
	)
	if err != nil {
		return err
	}

	groupId := cast.ToInt(createGroupResp["id"])

	createUser := map[string]any{
		"email":      in.MetabaseName,
		"first_name": in.DatabaseName,
		"login_attributes": map[string]any{
			"ANY_ADDITIONAL_PROPERTY": "anything",
		},
		"user_group_memberships": []map[string]any{
			{
				"id":               1,
				"is_group_manager": false,
			},
			{
				"id":               groupId,
				"is_group_manager": false,
			},
		},
	}
	createUserResp, err := DoRequest(
		http.MethodPost,
		in.Cfg.MetabaseBaseUrl+createMbUserEndpoint,
		in.Cfg.MetabaseApiKey,
		createUser, true,
	)
	if err != nil {
		return err
	}

	userId := cast.ToInt(createUserResp["id"])

	resetpasswordUrl := fmt.Sprintf("%s/api/user/%d/password", in.Cfg.MetabaseBaseUrl, userId)

	resetPassword := map[string]any{
		"password": in.MetabasePassword,
	}
	_, err = DoRequest(http.MethodPut, resetpasswordUrl, in.Cfg.MetabaseApiKey, resetPassword, true)
	if err != nil {
		return err
	}

	createDatabase := map[string]any{
		"is_on_demand":     false,
		"is_full_sync":     true,
		"is_sample":        false,
		"cache_ttl":        nil,
		"refingerprint":    false,
		"auto_run_queries": true,
		"schedules":        map[string]any{},
		"details": map[string]any{
			"host":                 in.Host,
			"port":                 in.Port,
			"user":                 in.Username,
			"password":             in.Password,
			"dbname":               in.Database,
			"scan-all-databases":   true,
			"ssl":                  false,
			"tunnel-enabled":       false,
			"advanced-options":     false,
			"destination-database": false,
		},
		"name":   in.DatabaseName,
		"engine": "clickhouse",
	}
	createDatabaseResp, err := DoRequest(
		http.MethodPost,
		in.Cfg.MetabaseBaseUrl+createDatabaseEndpoint,
		in.Cfg.MetabaseApiKey,
		createDatabase, true,
	)
	if err != nil {
		return err
	}
	databaseId := cast.ToInt(createDatabaseResp["id"])

	getPermissionsResp, err := DoRequest(
		http.MethodGet,
		in.Cfg.MetabaseBaseUrl+permissionsEndpoint,
		in.Cfg.MetabaseApiKey,
		nil, true,
	)
	if err != nil {
		return err
	}

	revision := cast.ToInt(getPermissionsResp["revision"])

	updatePermissions := map[string]any{
		"groups": map[int]any{
			groupId: map[int]any{
				databaseId: map[string]any{
					"create-queries": "query-builder-and-native",
					"view-data":      "unrestricted",
					"download": map[string]string{
						"schemas": "full",
					},
				},
			},
		},
		"revision": revision,
	}

	_, err = DoRequest(
		http.MethodPut,
		in.Cfg.MetabaseBaseUrl+permissionsEndpoint,
		in.Cfg.MetabaseApiKey,
		updatePermissions, true,
	)
	if err != nil {
		return err
	}

	return nil
}

func GetSession(in GetSessionRequest) (string, error) {
	getSession := map[string]string{
		"username": in.Username,
		"password": in.Password,
	}
	getSessionResp, err := DoRequest(
		http.MethodPost,
		in.Cfg.MetabaseBaseUrl+sessionEndpoint,
		in.Cfg.MetabaseApiKey, getSession, true,
	)
	if err != nil {
		return "", err
	}

	id := cast.ToString(getSessionResp["id"])

	return id, nil
}

func GetDashboards(in GetDashboardsRequest) (*pb.GetMetabaseDashboardsResponse, error) {
	getDashboardsResp, err := DoRequest(
		http.MethodGet,
		in.Cfg.MetabaseBaseUrl+getDashboardsEndpoint,
		in.Session,
		nil, false,
	)
	if err != nil {
		return nil, err
	}

	jsonData, err := json.Marshal(getDashboardsResp)
	if err != nil {
		return nil, err
	}

	var result *pb.GetMetabaseDashboardsResponse
	if err := json.Unmarshal(jsonData, &result); err != nil {
		return nil, err
	}

	return result, nil
}

func GetPublicUrl(in GetPublicUrlRequest) (string, error) {
	url := fmt.Sprintf("%s/api/dashboard/%d/public_link", in.Cfg.MetabaseBaseUrl, in.DashboardId)

	getPublicUrlResp, err := DoRequest(
		http.MethodPost,
		url, in.Cfg.MetabaseApiKey,
		nil, true,
	)
	if err != nil {
		return "", err
	}

	id := cast.ToString(getPublicUrlResp["uuid"])

	publicUrl := fmt.Sprintf("%s/public/dashboard/%s", in.Cfg.MetabaseBaseUrl, id)

	return publicUrl, nil
}

func DoRequest(method, url, authToken string, payload any, isApiKey bool) (map[string]any, error) {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("[Metabase.DoRequest] marshal payload error: %w", err)
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("[Metabase.DoRequest] create request error: %w", err)
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if isApiKey {
		req.Header.Set("x-api-key", authToken)
	} else {
		req.Header.Set("x-metabase-session", authToken)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("[Metabase.DoRequest] HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("[Metabase.DoRequest] read body error: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("[Metabase.DoRequest] API error: status=%d url=%s body=%s", resp.StatusCode, url, string(body))
	}

	if len(body) == 0 {
		return nil, nil
	}

	var obj map[string]any
	if err := json.Unmarshal(body, &obj); err == nil {
		return obj, nil
	}

	var arr []any
	if err := json.Unmarshal(body, &arr); err == nil {
		return map[string]any{"dashboards": arr}, nil
	}

	return nil, errors.New("unexpected response format")
}

package airbyte

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/Ucode-io/ucode-opensource/services/company/config"

	"github.com/spf13/cast"
)

func CreateWorkbookAndConnection(in CreateWorkbookAndConnectionRequest) error {
	workbook := map[string]any{
		"title":       in.Title,
		"description": in.Description,
	}
	workbookResp, err := DoRequest(in.Cfg, http.MethodPost, "/gateway/root/us/createWorkbook", workbook)
	if err != nil {
		return err
	}

	connection := map[string]any{
		"host":                  in.Host,
		"port":                  in.Port,
		"username":              in.Username,
		"password":              in.Password,
		"secure":                in.Secure,
		"data_export_forbidden": in.DataExportForbidden,
		"readonly":              in.Readonly,
		"raw_sql_level":         "dashsql",
		"type":                  "clickhouse",
		"name":                  fmt.Sprintf("%s's connection", in.Title),
		"workbook_id":           cast.ToString(workbookResp["workbookId"]),
	}

	_, err = DoRequest(in.Cfg, http.MethodPost, "/gateway/root/bi/createConnection", connection)
	if err != nil {
		return err
	}

	return nil
}

func DoRequest(cfg config.Config, method, endpoint string, payload any) (map[string]any, error) {
	var (
		response map[string]any
		client   = &http.Client{}
	)

	data, err := json.Marshal(payload)
	if err != nil {
		return response, err
	}

	req, err := http.NewRequest(method, cfg.DatalensBaseURL+endpoint, bytes.NewBuffer(data))
	if err != nil {
		return response, err
	}

	req.SetBasicAuth(cfg.DatalensUsername, cfg.DatalensPassword)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return response, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, err
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return response, err
	}

	return response, nil
}

package airbyte

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"github.com/Ucode-io/ucode-opensource/services/company/config"

	"github.com/spf13/cast"
)

type Airbyte struct {
	cfg config.Config
}

func NewAirbyte(cfg config.Config) *Airbyte {
	return &Airbyte{cfg: cfg}
}

func (a *Airbyte) GetWorkSpace() (Workspace, error) {
	var response Workspace
	resp, err := a.DoRequest("POST", "/v1/workspaces/list", []byte(`{}`))
	if err != nil {
		return Workspace{}, fmt.Errorf("[Airbyte.GetWorkSpace] DoRequest error: %w", err)
	}

	respByte, err := json.Marshal(resp)
	if err != nil {
		return Workspace{}, fmt.Errorf("[Airbyte.GetWorkSpace] marshal error: %w", err)
	}

	var workSpaces WorkspacesResponse

	if err := json.Unmarshal(respByte, &workSpaces); err != nil {
		return Workspace{}, fmt.Errorf("[Airbyte.GetWorkSpace] unmarshal error: %w", err)
	}

	for _, workspace := range workSpaces.Workspaces {
		if workspace.Email == a.cfg.AirByteEmail {
			response = workspace
			break
		}
	}

	if response.WorkspaceID == "" {
		log.Printf("[Airbyte.GetWorkSpace] WARNING: no workspace found for email=%q among %d workspaces", a.cfg.AirByteEmail, len(workSpaces.Workspaces))
	}

	return response, nil
}

func (a *Airbyte) GetSourceDefinitionId(name string) (string, error) {
	var response SourceDefinitionsResponse
	resp, err := a.DoRequest("POST", "/v1/source_definitions/list_latest", []byte(`{}`))
	if err != nil {
		return "", fmt.Errorf("[Airbyte.GetSourceDefinitionId] DoRequest error: %w", err)
	}

	respByte, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("[Airbyte.GetSourceDefinitionId] marshal error: %w", err)
	}

	if err = json.Unmarshal(respByte, &response); err != nil {
		return "", fmt.Errorf("[Airbyte.GetSourceDefinitionId] unmarshal error: %w", err)
	}

	for _, v := range response.SourceDefinitions {
		if v.Name == name {
			return v.SourceDefinitionId, nil
		}
	}

	return "", fmt.Errorf("[Airbyte.GetSourceDefinitionId] source definition %q not found among %d definitions", name, len(response.SourceDefinitions))
}

func (a *Airbyte) CreateSourceMongoDb(mongo CreateMongoSource) (CreateMongoSourceResponse, error) {
	var response CreateMongoSourceResponse

	resp, err := a.DoRequest("POST", "/v1/sources/create", mongo)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.CreateSourceMongoDb] DoRequest error: %w", err)
	}

	respByte, err := json.Marshal(resp)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.CreateSourceMongoDb] marshal error: %w", err)
	}

	if err := json.Unmarshal(respByte, &response); err != nil {
		return response, fmt.Errorf("[Airbyte.CreateSourceMongoDb] unmarshal error: %w", err)
	}

	if response.SourceId == "" {
		return response, fmt.Errorf("[Airbyte.CreateSourceMongoDb] API returned empty sourceId, response: %v", resp)
	}

	return response, nil
}

func (a *Airbyte) CreateSourcePostgres(pg CreatePostgresSource) (CreatePostgresSourceResponse, error) {
	var response CreatePostgresSourceResponse

	resp, err := a.DoRequest("POST", "/v1/sources/create", pg)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.CreateSourcePostgres] DoRequest error: %w", err)
	}

	respByte, err := json.Marshal(resp)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.CreateSourcePostgres] marshal error: %w", err)
	}

	if err := json.Unmarshal(respByte, &response); err != nil {
		return response, fmt.Errorf("[Airbyte.CreateSourcePostgres] unmarshal error: %w", err)
	}

	if response.SourceId == "" {
		return response, fmt.Errorf("[Airbyte.CreateSourcePostgres] API returned empty sourceId, response: %v", resp)
	}

	return response, nil
}

func (a *Airbyte) GetSyncCatalog(sourceId string) (SyncCatalogResponse, error) {
	var (
		result   SyncCatalogResponse
		response SyncCatalogResponse
	)

	if sourceId == "" {
		return response, errors.New("[Airbyte.GetSyncCatalog] sourceId is empty")
	}

	resp, err := a.DoRequest("POST", "/v1/sources/discover_schema", map[string]string{"sourceId": sourceId})
	if err != nil {
		return response, fmt.Errorf("[Airbyte.GetSyncCatalog] DoRequest error: %w", err)
	}

	respByte, err := json.Marshal(resp)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.GetSyncCatalog] marshal error: %w", err)
	}

	if err := json.Unmarshal(respByte, &result); err != nil {
		return response, fmt.Errorf("[Airbyte.GetSyncCatalog] unmarshal error: %w", err)
	}

	for _, stream := range result.Catalog.Streams {
		if exist := config.SkipTablesSlugs[stream.Stream.Name]; exist {
			continue
		}

		response.Catalog.Streams = append(response.Catalog.Streams, stream)
	}

	return response, nil
}

func (a *Airbyte) GetDestinationId(name string) (string, error) {
	var response DestinationDefinitionResponse
	resp, err := a.DoRequest("POST", "/v1/destination_definitions/list", []byte(`{}`))
	if err != nil {
		return "", fmt.Errorf("[Airbyte.GetDestinationId] DoRequest error: %w", err)
	}

	respByte, err := json.Marshal(resp)
	if err != nil {
		return "", fmt.Errorf("[Airbyte.GetDestinationId] marshal error: %w", err)
	}

	if err := json.Unmarshal(respByte, &response); err != nil {
		return "", fmt.Errorf("[Airbyte.GetDestinationId] unmarshal error: %w", err)
	}

	for _, definition := range response.DestinationDefinitions {
		if definition.Name == name {
			return definition.DestinationDefinitionId, nil
		}
	}

	return "", fmt.Errorf("[Airbyte.GetDestinationId] destination definition %q not found among %d definitions", name, len(response.DestinationDefinitions))
}

func (a *Airbyte) CreateDestinationClickHouse(ch CreateClickhouseDestination) (string, error) {
	resp, err := a.DoRequest("POST", "/v1/destinations/create", ch)
	if err != nil {
		return "", fmt.Errorf("[Airbyte.CreateDestinationClickHouse] DoRequest error: %w", err)
	}

	destinationId := cast.ToString(resp["destinationId"])
	if destinationId == "" {
		return "", fmt.Errorf("[Airbyte.CreateDestinationClickHouse] API returned empty destinationId, response: %v", resp)
	}

	return destinationId, nil
}

func (a *Airbyte) CreateConnection(workSpaceId string, conn CreateConnection) (string, error) {
	if conn.SourceId == "" {
		return "", errors.New("[Airbyte.CreateConnection] sourceId is empty")
	}
	if conn.DestinitionId == "" {
		return "", errors.New("[Airbyte.CreateConnection] destinationId is empty")
	}
	if workSpaceId == "" {
		return "", errors.New("[Airbyte.CreateConnection] workspaceId is empty")
	}

	var operations []ConnectionOperation

	operations = append(operations, ConnectionOperation{
		Name:        "Normalization",
		WorkspaceId: workSpaceId,
		OperatorConfiguration: struct {
			OperatorType  string "json:\"operatorType\""
			Normalization struct {
				Option string "json:\"option\""
			} "json:\"normalization\""
		}{
			OperatorType: "normalization",
			Normalization: struct {
				Option string "json:\"option\""
			}{
				Option: "basic",
			},
		},
	})

	conn.Operations = operations

	resp, err := a.DoRequest("POST", "/v1/web_backend/connections/create", conn)
	if err != nil {
		return "", fmt.Errorf("[Airbyte.CreateConnection] DoRequest error: %w", err)
	}

	connectionId := cast.ToString(resp["connectionId"])
	if connectionId == "" {
		return "", fmt.Errorf("[Airbyte.CreateConnection] API returned empty connectionId, response: %v", resp)
	}

	return connectionId, nil
}

func (a *Airbyte) DoRequest(method, endpoint string, payload any) (map[string]any, error) {
	var (
		response map[string]any
		client   = &http.Client{}
	)

	data, err := json.Marshal(payload)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.DoRequest] marshal payload error: %w", err)
	}

	url := a.cfg.AirByteApiUrl + endpoint

	req, err := http.NewRequest(method, url, bytes.NewBuffer(data))
	if err != nil {
		return response, fmt.Errorf("[Airbyte.DoRequest] create request error: %w", err)
	}

	req.SetBasicAuth(a.cfg.AirByteUsername, a.cfg.AirBytePassword)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.DoRequest] HTTP request error: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return response, fmt.Errorf("[Airbyte.DoRequest] read body error: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return response, fmt.Errorf("[Airbyte.DoRequest] API error: status=%d endpoint=%s body=%s", resp.StatusCode, endpoint, string(body))
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return response, fmt.Errorf("[Airbyte.DoRequest] unmarshal response error: %w, body=%s", err, string(body))
	}

	return response, nil
}

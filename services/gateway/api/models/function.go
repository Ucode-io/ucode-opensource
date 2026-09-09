package models

import (
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"net/http"
	"net/url"
)

type GitlabTreeItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"` // "blob" (file) or "tree" (folder)
	Path string `json:"path"`
	Mode string `json:"mode"`
}

type GitlabFileChange struct {
	FilePath string `json:"file_path"`
	Content  string `json:"content"`
}

type GitlabUpdateFileRequest struct {
	Files  []GitlabFileChange `json:"files"`
	Branch string             `json:"branch"`
}

// PublishAiMicroFrontendRequest is sent to the function service to create a
// microfrontend and push AI-generated files to the u-gen branch.
type PublishAiMicroFrontendRequest struct {
	ProjectId        string             `json:"project_id"`
	EnvironmentId    string             `json:"environment_id"`
	Name             string             `json:"name"`
	Path             string             `json:"path"`
	FrameworkType    string             `json:"framework_type"`
	Files            []GitlabFileChange `json:"files"`
	McpProjectId     string             `json:"mcp_project_id,omitempty"`
	McpResourceEnvId string             `json:"mcp_resource_env_id,omitempty"`
}

// PublishAiMicroFrontendResponse holds the fields we need from the function
// service response (wrapped in the standard {data: ...} envelope).
type PublishAiMicroFrontendResponse struct {
	Status string `json:"status"`
	Data   struct {
		ID     string `json:"id"`
		RepoId string `json:"repo_id"`
		Url    string `json:"url"`
		Branch string `json:"branch"`
	} `json:"data"`
}

type Function struct {
	ID               string `json:"id"`
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	FuncitonFolderId string `json:"function_folder_id"`
	Type             string `json:"type"`
}

type CreateFunctionRequest struct {
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	CommitId         int64  `json:"-"`
	CommitGuid       string `json:"-"`
	VersionId        string `json:"-"`
	FunctionFolderId string `json:"function_folder_id"`
	FrameworkType    string `json:"framework_type"`
	Type             string `json:"type"`
}

type InvokeFunctionRequest struct {
	FunctionID string         `json:"function_id"`
	ObjectIDs  []string       `json:"object_ids"`
	Attributes map[string]any `json:"attributes"`
	TableSlug  string         `json:"table_slug"`
	ObjectData map[string]any `json:"object_data"`
}

type InvokeFunctionResponse struct {
	Status      string         `json:"status"`
	Data        map[string]any `json:"data"`
	Attributes  map[string]any `json:"attributes"`
	ServerError string         `json:"server_error"`
}

type GetListClientApiResp struct {
	Response       []map[string]any `json:"response"`
	Fields         []map[string]any `json:"fields"`
	Views          []map[string]any `json:"views"`
	RelationFields []map[string]any `json:"relation_fields"`
}

type InvokeFunctionResponse2 struct {
	Status string               `json:"status"`
	Data   GetListClientApiResp `json:"data"`
}

type NewInvokeFunctionRequest struct {
	Auth          AuthData       `json:"auth"`
	Data          map[string]any `json:"data"`
	RequestData   HttpRequest    `json:"request_data"`
	AutomationURL string         `json:"-"`
	KnativeURL    string         `json:"-"`
	OpenFaaSURL   string         `json:"-"`
}

type HttpRequest struct {
	Method  string      `json:"method"`
	Path    string      `json:"path"`
	Headers http.Header `json:"headers"`
	Params  url.Values  `json:"params"`
	Body    []byte      `json:"body"`
}

type AuthData struct {
	Type string         `json:"type"`
	Data map[string]any `json:"data"`
}

type FunctionRunV2 struct {
	RequestData HttpRequest    `json:"request_data"`
	Auth        AuthData       `json:"auth"`
	Data        map[string]any `json:"data"`
}

type InvokeFunctionRequestWithAppId struct {
	ObjectIDs []string `json:"object_ids"`
	AppID     string   `json:"app_id"`
}

type GetByIdFunctionResponse struct {
	Password         string `json:"password"`
	URL              string `json:"url"`
	ID               string `json:"id"`
	Path             string `json:"path"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	FuncitonFolderId string `json:"function_folder_id"`
}

type RevertMicrofrontendRequest struct {
	RepoID     string `json:"repo_id"`
	SnapshotID string `json:"snapshot_id"`
}

type ManualSaveMcpProjectRequest struct {
	RepoID          int                `json:"repo_id"`
	Files           []GitlabFileChange `json:"files"`
	CommitMessage   string             `json:"commit_message,omitempty"`
	MicrofrontendID string             `json:"microfrontend_id,omitempty"`
}

type CreateMfeShortLinkRequest struct {
	FunctionId   string `json:"function_id"`
	McpProjectId string `json:"mcp_project_id"`
	ProjectId    string `json:"project_id"` // optional — ucode project id
	Url          string `json:"url"`
}

type MicrofrontendSnapshot struct {
	ID              string             `json:"id"`
	MicrofrontendID string             `json:"microfrontend_id"`
	CommitMessage   string             `json:"commit_message"`
	Files           []GitlabFileChange `json:"files"`
	CreatedAt       string             `json:"created_at"`
}

type MicrofrontForLoginPage struct {
	Function      *object_builder_service.Function `json:"function"`
	Id            string                           `json:"id"`
	ProjectId     string                           `json:"project_id"`
	EnvironmentId string                           `json:"environment_id"`
	MicrofrontId  string                           `json:"microfront_id"`
	Subdomain     string                           `json:"subdomain"`
}

type DoInvokeFunctionStruct struct {
	Services               services.GoBuilderServiceI
	CustomEvents           []*object_builder_service.CustomEvent
	IDs                    []string
	TableSlug              string
	ObjectData             map[string]any
	Method                 string
	ActionType             string
	ObjectDataBeforeUpdate map[string]any
	Resource               *pb.ServiceResourceModel
}

type GetListCustomEventsStruct struct {
	TableSlug string
	RoleId    string
	Method    string
	Resource  *pb.ServiceResourceModel
}

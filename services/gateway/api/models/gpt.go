package models

import (
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"github.com/Ucode-io/ucode-opensource/services/gateway/storage"
)

type SendToGptRequest struct {
	Promt string `json:"promt"`
}

type OpenAIRequest struct {
	Model        string    `json:"model"`
	Messages     []Message `json:"messages"`
	Functions    []Tool    `json:"tools"`
	FunctionCall string    `json:"tool_choice"`
}

type OpenAIRequestV2 struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type OpenAIResponse struct {
	ID                string   `json:"id"`
	Object            string   `json:"object"`
	Created           int      `json:"created"`
	Model             string   `json:"model"`
	Choices           []Choice `json:"choices"`
	Usage             Usage    `json:"usage"`
	SystemFingerprint any      `json:"system_fingerprint"`
	Error             ErrorAI  `json:"error"`
}

type ErrorAI struct {
	Message string `json:"message"`
	Type    string `json:"type"`
	Param   string `json:"param"`
	Code    string `json:"code"`
}

type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type FunctionTool struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function FunctionTool `json:"function"`
}

type MessageChoice struct {
	Role      string     `json:"role"`
	Content   string     `json:"content"`
	ToolCalls []ToolCall `json:"tool_calls"`
}

type Choice struct {
	Index        int           `json:"index"`
	Message      MessageChoice `json:"message"`
	Logprobs     any           `json:"logprobs"`
	FinishReason string        `json:"finish_reason"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Tool struct {
	Type     string              `json:"type"`
	Function FunctionDescription `json:"function"`
}

type FunctionDescription struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"`
}

type CreateMenuAI struct {
	Label    string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
	Redis    storage.RedisStorageI
	Promt    string
}

type DeleteMenuAI struct {
	Label    string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type UpdateMenuAI struct {
	NewLabel string
	OldLabel string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type CreateTableAI struct {
	Label         string
	UserId        string
	TableSlug     string
	EnvironmentId string
	Menu          string
	Resource      *pb.ServiceResourceModel
	Service       services.ServiceManagerI
}

type UpdateTableAI struct {
	NewLabel      string
	OldLabel      string
	UserId        string
	EnvironmentId string
	Resource      *pb.ServiceResourceModel
	Service       services.ServiceManagerI
}

type CreateFieldAI struct {
	Label    string
	Slug     string
	Type     string
	Table    string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type UpdateFieldAI struct {
	OldLabel string
	NewLabel string
	NewType  string
	Table    string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type DeleteFieldAI struct {
	Label    string
	Table    string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type CreateRelationAI struct {
	TableFrom    string
	TableTo      string
	RelationType string
	ViewField    []string
	ViewType     string
	UserId       string
	Resource     *pb.ServiceResourceModel
	Service      services.ServiceManagerI
}

type DeleteRelationAI struct {
	TableFrom    string
	TableTo      string
	RelationType string
	UserId       string
	Resource     *pb.ServiceResourceModel
	Service      services.ServiceManagerI
}

type CreateItemsAI struct {
	Arguments []string
	Table     string
	UserId    string
	Resource  *pb.ServiceResourceModel
	Service   services.ServiceManagerI
}

type GenerateItemsAI struct {
	Table    string
	Count    int
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type UpdateItemsAI struct {
	OldColumn any
	NewColumn any
	Table     string
	UserId    string
	Resource  *pb.ServiceResourceModel
	Service   services.ServiceManagerI
}

type LoginTableAI struct {
	Table    string
	Login    string
	Password string
	UserId   string
	Resource *pb.ServiceResourceModel
	Service  services.ServiceManagerI
}

type CreateFunctionAI struct {
	Table         string
	Prompt        string
	FunctionName  string
	UserId        string
	Token         string
	GitlabToken   string
	ActionType    string
	Method        string
	EnvironmentId string
	Resource      *pb.ServiceResourceModel
	Service       services.ServiceManagerI
}

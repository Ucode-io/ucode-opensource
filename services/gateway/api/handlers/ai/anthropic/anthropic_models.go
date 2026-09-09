package anthropic

import (
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

var ErrMaxTokens = ai.ErrMaxTokens

type (
	// ── Anthropic REST wire types ─────────────────────────────────────────────

	AnthropicRequest struct {
		Model      string               `json:"model"`
		MaxTokens  int                  `json:"max_tokens"`
		System     string               `json:"system,omitempty"`
		Messages   []models.ChatMessage `json:"messages"`
		MCPServers []MCPServer          `json:"mcp_servers,omitempty"`
		Tools      []MCPTool            `json:"tools,omitempty"`
	}

	MCPServer struct {
		Type               string `json:"type"`
		URL                string `json:"url"`
		Name               string `json:"name"`
		AuthorizationToken string `json:"authorization_token,omitempty"`
	}

	MCPTool struct {
		Type          string `json:"type"`
		MCPServerName string `json:"mcp_server_name,omitempty"`
	}

	// ── Tool-use (function calling) ───────────────────────────────────────────

	claudeFunctionTool struct {
		Name        string                 `json:"name"`
		Description string                 `json:"description"`
		InputSchema map[string]interface{} `json:"input_schema"`
	}

	// ToolChoice forces Claude to call a specific tool.
	toolChoice struct {
		Type string `json:"type"` // "tool" | "auto" | "any"
		Name string `json:"name,omitempty"`
	}

	toolUseBlock struct {
		Type  string                 `json:"type"` // "tool_use"
		ID    string                 `json:"id"`
		Name  string                 `json:"name"`
		Input map[string]interface{} `json:"input"`
	}

	toolUseResponse struct {
		Model      string             `json:"model"`
		ID         string             `json:"id"`
		Content    []toolUseBlock     `json:"content"`
		StopReason string             `json:"stop_reason"`
		Usage      models.ClaudeUsage `json:"usage"`
	}

	// ── Prompt caching (ephemeral) ────────────────────────────────────────────

	systemBlock struct {
		Type         string     `json:"type"`
		Text         string     `json:"text"`
		CacheControl *cacheCtrl `json:"cache_control,omitempty"`
	}

	cacheCtrl struct {
		Type string `json:"type"` // "ephemeral"
	}

	wireToolRequest struct {
		Model      string               `json:"model"`
		MaxTokens  int                  `json:"max_tokens"`
		System     []systemBlock        `json:"system,omitempty"`
		Messages   []models.ChatMessage `json:"messages"`
		Tools      []claudeFunctionTool `json:"tools"`
		ToolChoice *toolChoice          `json:"tool_choice,omitempty"`
	}
)

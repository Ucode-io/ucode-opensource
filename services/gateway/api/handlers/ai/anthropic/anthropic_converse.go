package anthropic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

// AnthropicChatModel implements ai.ChatModel over the Anthropic Messages API
// with native tool_use / tool_result content blocks for multi-turn loops.
type AnthropicChatModel struct {
	conf config.BaseConfig

	// apiKeyOverride, when non-empty, is used instead of conf.AnthropicAPIKey.
	// It lets callers bill a separate Anthropic key per workload (e.g. the ucode
	// builder vs the ugen generator) while sharing the rest of the config.
	apiKeyOverride string
}

func NewAnthropicChatModel(conf config.BaseConfig) ai.ChatModel {
	return &AnthropicChatModel{conf: conf}
}

// NewAnthropicChatModelWithKey builds a model that authenticates with apiKey
// instead of the config's default Anthropic key. An empty apiKey falls back to
// conf.AnthropicAPIKey, so callers can pass through without a nil check.
func NewAnthropicChatModelWithKey(conf config.BaseConfig, apiKey string) ai.ChatModel {
	return &AnthropicChatModel{conf: conf, apiKeyOverride: apiKey}
}

// apiKey returns the override when set, otherwise the config default.
func (m *AnthropicChatModel) apiKey() string {
	if m.apiKeyOverride != "" {
		return m.apiKeyOverride
	}
	return m.conf.AnthropicAPIKey
}

// ── wire types (multi-turn, tool-aware) ───────────────────────────────────────

type (
	converseRequest struct {
		Model     string               `json:"model"`
		MaxTokens int                  `json:"max_tokens"`
		System    string               `json:"system,omitempty"`
		Messages  []converseMessage    `json:"messages"`
		Tools     []claudeFunctionTool `json:"tools,omitempty"`
	}

	converseMessage struct {
		Role    string          `json:"role"` // "user" | "assistant"
		Content []converseBlock `json:"content"`
	}

	// converseBlock is the union of the block types we emit: text, tool_use,
	// tool_result. Only the fields relevant to Type are populated.
	converseBlock struct {
		Type string `json:"type"`

		// type=text
		Text string `json:"text,omitempty"`

		// type=tool_use
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
		// Pointer (not bare map) so omitempty drops only nil — a pointer to an
		// empty map still renders as "input": {}, which Anthropic requires for
		// every tool_use block (parameter-less tools would otherwise be sent
		// without input and rejected with `tool_use.input: Field required`).
		Input *map[string]any `json:"input,omitempty"`

		// type=tool_result
		ToolUseID string `json:"tool_use_id,omitempty"`
		Content   string `json:"content,omitempty"`
		IsError   bool   `json:"is_error,omitempty"`
	}

	converseResponse struct {
		Model      string              `json:"model"`
		ID         string              `json:"id"`
		Content    []converseRespBlock `json:"content"`
		StopReason string              `json:"stop_reason"`
		Usage      models.ClaudeUsage  `json:"usage"`
	}

	converseRespBlock struct {
		Type  string         `json:"type"` // "text" | "tool_use"
		Text  string         `json:"text"`
		ID    string         `json:"id"`
		Name  string         `json:"name"`
		Input map[string]any `json:"input"`
	}
)

func (m *AnthropicChatModel) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
	wire := converseRequest{
		Model:     req.Model,
		MaxTokens: req.MaxTokens,
		System:    req.System,
		Messages:  toAnthropicMessages(req.Messages),
		Tools:     toClaudeTools(req.Tools),
	}

	jsonBody, err := json.Marshal(wire)
	if err != nil {
		return nil, fmt.Errorf("anthropic: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, m.conf.AnthropicBaseAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("anthropic: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("X-API-Key", m.apiKey())
	httpReq.Header.Set("anthropic-version", m.conf.AnthropicVersion)
	httpReq.Header.Set("anthropic-beta", config.AnthropicCachingBeta)

	respBody, err := doHTTP(httpReq, req.Timeout)
	if err != nil {
		return nil, err
	}

	var resp converseResponse
	if err = json.Unmarshal([]byte(respBody), &resp); err != nil {
		return nil, fmt.Errorf("anthropic: parse response: %w", err)
	}

	usage := models.LLMUsage{
		InputTokens:  resp.Usage.InputTokens,
		OutputTokens: resp.Usage.OutputTokens,
	}

	result := &ai.CompletionResult{
		StopReason: normalizeAnthropicStop(resp.StopReason),
		Usage:      usage,
	}

	if resp.StopReason == "max_tokens" {
		return result, fmt.Errorf("%w (model=%s input=%d output=%d)",
			ErrMaxTokens, resp.Model, usage.InputTokens, usage.OutputTokens)
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Text += block.Text
		case "tool_use":
			ai.RepairStringifiedFields(block.Input)
			result.ToolCalls = append(result.ToolCalls, ai.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return result, nil
}

func toAnthropicMessages(messages []ai.ConversationMessage) []converseMessage {
	out := make([]converseMessage, 0, len(messages))
	for _, msg := range messages {
		blocks := make([]converseBlock, 0, len(msg.ToolCalls)+len(msg.ToolResults)+1)

		// User-side tool results come first in the content array.
		for _, tr := range msg.ToolResults {
			blocks = append(blocks, converseBlock{
				Type:      "tool_result",
				ToolUseID: tr.ToolCallID,
				Content:   tr.Content,
				IsError:   tr.IsError,
			})
		}

		if msg.Text != "" {
			blocks = append(blocks, converseBlock{Type: "text", Text: msg.Text})
		}

		for _, tc := range msg.ToolCalls {
			input := tc.Input
			if input == nil {
				input = map[string]any{}
			}
			blocks = append(blocks, converseBlock{
				Type:  "tool_use",
				ID:    tc.ID,
				Name:  tc.Name,
				Input: &input,
			})
		}

		if len(blocks) == 0 {
			continue
		}
		out = append(out, converseMessage{Role: msg.Role, Content: blocks})
	}
	return out
}

func toClaudeTools(tools []ai.ToolDef) []claudeFunctionTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]claudeFunctionTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, claudeFunctionTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		})
	}
	return out
}

func normalizeAnthropicStop(reason string) string {
	switch reason {
	case "tool_use":
		return ai.StopToolUse
	case "max_tokens":
		return ai.StopMaxTokens
	default:
		return ai.StopEndTurn
	}
}

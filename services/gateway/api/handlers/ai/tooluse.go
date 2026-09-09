package ai

import (
	"context"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// Stop reasons normalized across providers.
const (
	StopEndTurn   = "end_turn"   // model produced a final answer, no more tool calls
	StopToolUse   = "tool_use"   // model requested one or more tool calls
	StopMaxTokens = "max_tokens" // output token limit hit
)

// ToolDef describes a single callable tool exposed to the model. InputSchema is a
// JSON Schema object (draft 2020-12) describing the tool's parameters.
type ToolDef struct {
	Name        string
	Description string
	InputSchema map[string]any
}

// ToolCall is a model-issued request to invoke a tool. ID correlates the call
// with its result (synthesized for providers that omit one, e.g. Gemini).
type ToolCall struct {
	ID    string
	Name  string
	Input map[string]any
}

// ToolResult carries the outcome of executing a ToolCall back to the model.
type ToolResult struct {
	ToolCallID string
	Content    string
	IsError    bool
}

// ConversationMessage is one provider-agnostic turn in a tool-use loop.
//
// A "user" message carries either free Text (the initial prompt) or a batch of
// ToolResults answering the previous assistant turn. An "assistant" message
// carries the model's Text and/or the ToolCalls it requested.
type ConversationMessage struct {
	Role        string // "user" | "assistant"
	Text        string
	ToolCalls   []ToolCall
	ToolResults []ToolResult
}

// CompletionRequest is a single multi-turn, tool-aware model invocation.
type CompletionRequest struct {
	Model     string
	MaxTokens int
	System    string
	Messages  []ConversationMessage
	Tools     []ToolDef
	Timeout   time.Duration
}

// CompletionResult is the normalized model response for one turn.
type CompletionResult struct {
	Text       string
	ToolCalls  []ToolCall
	StopReason string
	Usage      models.LLMUsage
}

// ChatModel is a provider-agnostic, multi-turn, tool-calling chat completion.
// Each provider package implements it over its native wire protocol.
type ChatModel interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResult, error)
}

package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

// OpenAIChatModel implements ai.ChatModel over the OpenAI Chat Completions API
// with native tool_calls / tool role messages for multi-turn loops.
type OpenAIChatModel struct {
	conf config.BaseConfig
}

func NewOpenAIChatModel(conf config.BaseConfig) ai.ChatModel {
	return &OpenAIChatModel{conf: conf}
}

func (m *OpenAIChatModel) Complete(ctx context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
	body := chatRequest{
		Model:               req.Model,
		Messages:            prependSystem(req.System, toOpenAIMessages(req.Messages)),
		MaxCompletionTokens: req.MaxTokens,
	}
	if tools := toOpenAITools(req.Tools); len(tools) > 0 {
		body.Tools = tools
		body.ToolChoice = "auto"
	}

	resp, err := doRequest(ctx, m.conf, req.Timeout, body)
	usage := usageFrom(resp)
	if err != nil {
		return &ai.CompletionResult{Usage: usage}, err
	}
	if len(resp.Choices) == 0 {
		return &ai.CompletionResult{Usage: usage}, fmt.Errorf("openai: no choices in response")
	}

	choice := resp.Choices[0]
	result := &ai.CompletionResult{
		Text:       choice.Message.Content,
		StopReason: normalizeOpenAIStop(choice.FinishReason),
		Usage:      usage,
	}

	if choice.FinishReason == "length" {
		return result, fmt.Errorf("%w (model=%s input=%d output=%d)",
			ErrMaxTokens, req.Model, usage.InputTokens, usage.OutputTokens)
	}

	for _, call := range choice.Message.ToolCalls {
		var args map[string]any
		if call.Function.Arguments != "" {
			if err = json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
				return result, fmt.Errorf("openai: parse tool arguments for %q: %w", call.Function.Name, err)
			}
		}
		if args == nil {
			args = map[string]any{}
		}
		ai.RepairStringifiedFields(args)
		result.ToolCalls = append(result.ToolCalls, ai.ToolCall{
			ID:    call.ID,
			Name:  call.Function.Name,
			Input: args,
		})
	}

	return result, nil
}

func toOpenAIMessages(messages []ai.ConversationMessage) []chatMessage {
	out := make([]chatMessage, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			am := chatMessage{Role: "assistant"}
			if msg.Text != "" {
				am.Content = msg.Text
			}
			for _, tc := range msg.ToolCalls {
				args, _ := json.Marshal(tc.Input)
				oc := toolCall{ID: tc.ID, Type: "function"}
				oc.Function.Name = tc.Name
				oc.Function.Arguments = string(args)
				am.ToolCalls = append(am.ToolCalls, oc)
			}
			out = append(out, am)

		default: // "user"
			// Tool results are sent as standalone {role:"tool"} messages.
			for _, tr := range msg.ToolResults {
				out = append(out, chatMessage{
					Role:       "tool",
					ToolCallID: tr.ToolCallID,
					Content:    tr.Content,
				})
			}
			if msg.Text != "" {
				out = append(out, chatMessage{Role: "user", Content: msg.Text})
			}
		}
	}
	return out
}

func toOpenAITools(tools []ai.ToolDef) []chatTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, chatTool{
			Type: "function",
			Function: functionDef{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	return out
}

func normalizeOpenAIStop(reason string) string {
	switch reason {
	case "tool_calls":
		return ai.StopToolUse
	case "length":
		return ai.StopMaxTokens
	default:
		return ai.StopEndTurn
	}
}

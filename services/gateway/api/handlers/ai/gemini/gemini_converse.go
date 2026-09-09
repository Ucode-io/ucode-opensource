package gemini

import (
	"context"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

// GeminiChatModel implements ai.ChatModel over the Gemini generateContent API
// with native functionCall / functionResponse parts for multi-turn loops.
//
// Gemini does not return tool-call IDs, so the adapter synthesizes stable IDs
// ("call_<n>") per response and correlates tool results back to function names
// by scanning the assistant turns in the conversation history.
type GeminiChatModel struct {
	conf config.BaseConfig
	pool *KeyPool
}

func NewGeminiChatModel(conf config.BaseConfig, pool *KeyPool) ai.ChatModel {
	if pool == nil && conf.GeminiAPIKey != "" {
		pool, _ = NewKeyPool([]string{conf.GeminiAPIKey})
	}
	return &GeminiChatModel{conf: conf, pool: pool}
}

func (m *GeminiChatModel) Complete(_ context.Context, req ai.CompletionRequest) (*ai.CompletionResult, error) {
	if m.pool == nil {
		return nil, fmt.Errorf("gemini: no API key configured")
	}

	body := geminiRequest{
		SystemInstruction: systemContent(req.System),
		Contents:          toGeminiContents(req.Messages),
		GenerationConfig:  generationConfig{MaxOutputTokens: req.MaxTokens},
	}
	if tools := toGeminiTools(req.Tools); len(tools) > 0 {
		body.Tools = tools
		body.ToolConfig = &geminiToolCfg{
			FunctionCallingConfig: funcCallingConfig{Mode: "AUTO"},
		}
	}

	resp, err := doRequest(m.pool, req.Model, req.Timeout, body)
	usage := usageFromResponse(resp)
	if err != nil {
		return &ai.CompletionResult{Usage: usage}, err
	}
	if len(resp.Candidates) == 0 {
		return &ai.CompletionResult{Usage: usage}, fmt.Errorf("gemini: no candidates in response")
	}

	cand := resp.Candidates[0]
	result := &ai.CompletionResult{
		StopReason: normalizeGeminiStop(cand.FinishReason),
		Usage:      usage,
	}

	if cand.FinishReason == "MAX_TOKENS" {
		return result, fmt.Errorf("%w (model=%s input=%d output=%d)",
			ErrMaxTokens, req.Model, usage.InputTokens, usage.OutputTokens)
	}

	callIdx := 0
	for _, part := range cand.Content.Parts {
		switch {
		case part.FunctionCall != nil:
			args := part.FunctionCall.Args
			if args == nil {
				args = map[string]any{}
			}
			ai.RepairStringifiedFields(args)
			result.ToolCalls = append(result.ToolCalls, ai.ToolCall{
				ID:    fmt.Sprintf("call_%d", callIdx),
				Name:  part.FunctionCall.Name,
				Input: args,
			})
			callIdx++
		case part.Text != "":
			result.Text += part.Text
		}
	}

	if len(result.ToolCalls) > 0 {
		result.StopReason = ai.StopToolUse
	}

	return result, nil
}

func toGeminiContents(messages []ai.ConversationMessage) []geminiContent {
	// Gemini matches function responses by name, not id; build id→name first.
	idToName := make(map[string]string)
	for _, msg := range messages {
		for _, tc := range msg.ToolCalls {
			idToName[tc.ID] = tc.Name
		}
	}

	out := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		switch msg.Role {
		case "assistant":
			parts := make([]geminiPart, 0, len(msg.ToolCalls)+1)
			if msg.Text != "" {
				parts = append(parts, geminiPart{Text: msg.Text})
			}
			for _, tc := range msg.ToolCalls {
				parts = append(parts, geminiPart{
					FunctionCall: &geminiFuncCall{Name: tc.Name, Args: tc.Input},
				})
			}
			if len(parts) == 0 {
				continue
			}
			out = append(out, geminiContent{Role: "model", Parts: parts})

		default: // "user"
			parts := make([]geminiPart, 0, len(msg.ToolResults)+1)
			for _, tr := range msg.ToolResults {
				parts = append(parts, geminiPart{
					FunctionResponse: &geminiFuncResponse{
						Name:     idToName[tr.ToolCallID],
						Response: map[string]any{"result": tr.Content, "is_error": tr.IsError},
					},
				})
			}
			if msg.Text != "" {
				parts = append(parts, geminiPart{Text: msg.Text})
			}
			if len(parts) == 0 {
				continue
			}
			out = append(out, geminiContent{Role: "user", Parts: parts})
		}
	}
	return out
}

func toGeminiTools(tools []ai.ToolDef) []geminiTool {
	if len(tools) == 0 {
		return nil
	}
	decls := make([]funcDeclaration, 0, len(tools))
	for _, t := range tools {
		decls = append(decls, funcDeclaration{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return []geminiTool{{FunctionDeclarations: decls}}
}

func normalizeGeminiStop(reason string) string {
	switch reason {
	case "MAX_TOKENS":
		return ai.StopMaxTokens
	default:
		return ai.StopEndTurn
	}
}

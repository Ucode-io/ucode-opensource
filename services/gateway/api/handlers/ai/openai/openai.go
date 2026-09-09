package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

func callText(ctx context.Context, conf config.BaseConfig, agentCfg config.AgentConfig, systemPrompt string, messages []chatMessage) (string, models.LLMUsage, error) {
	resp, usage, err := doCall(ctx, conf, agentCfg, chatRequest{
		Model:               agentCfg.Model,
		Messages:            prependSystem(systemPrompt, messages),
		MaxCompletionTokens: agentCfg.MaxTokens,
	})
	if err != nil {
		return "", usage, err
	}
	if len(resp.Choices) == 0 {
		return "", usage, fmt.Errorf("openai: no choices in response")
	}
	return resp.Choices[0].Message.Content, usage, nil
}

func callTool(ctx context.Context, conf config.BaseConfig, agentCfg config.AgentConfig, systemPrompt string, messages []chatMessage, tool chatTool) ([]byte, models.LLMUsage, error) {
	resp, usage, err := doCall(ctx, conf, agentCfg, chatRequest{
		Model:               agentCfg.Model,
		Messages:            prependSystem(systemPrompt, messages),
		Tools:               []chatTool{tool},
		ToolChoice:          forceTool(tool.Function.Name),
		MaxCompletionTokens: agentCfg.MaxTokens,
	})
	if err != nil {
		return nil, usage, err
	}
	if len(resp.Choices) == 0 {
		return nil, usage, fmt.Errorf("openai: no choices in response")
	}

	choice := resp.Choices[0]

	if choice.FinishReason == "length" {
		return nil, usage, fmt.Errorf("%w (model=%s input=%d output=%d)",
			ErrMaxTokens, agentCfg.Model, usage.InputTokens, usage.OutputTokens)
	}
	if len(choice.Message.ToolCalls) == 0 {
		return nil, usage, fmt.Errorf("openai: no tool_calls in response (finish_reason=%q)", choice.FinishReason)
	}

	call := choice.Message.ToolCalls[0]
	if call.Function.Name != tool.Function.Name {
		return nil, usage, fmt.Errorf("openai: unexpected tool %q, wanted %q", call.Function.Name, tool.Function.Name)
	}

	var args map[string]any
	if err = json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
		return nil, usage, fmt.Errorf("openai: parse tool arguments: %w | preview=%.300s", err, call.Function.Arguments)
	}
	ai.RepairStringifiedFields(args)

	out, err := json.Marshal(args)
	if err != nil {
		return nil, usage, fmt.Errorf("openai: re-marshal tool arguments: %w", err)
	}
	return out, usage, nil
}

// Structured Outputs avoid the escaped-string double-encoding that breaks
// function-call arguments at 64k+ token outputs.
func callStructured(ctx context.Context, conf config.BaseConfig, agentCfg config.AgentConfig, systemPrompt string, messages []chatMessage, format responseFormat) ([]byte, models.LLMUsage, error) {
	resp, usage, err := doCall(ctx, conf, agentCfg, chatRequest{
		Model:               agentCfg.Model,
		Messages:            prependSystem(systemPrompt, messages),
		ResponseFormat:      &format,
		MaxCompletionTokens: agentCfg.MaxTokens,
	})
	if err != nil {
		return nil, usage, err
	}
	if len(resp.Choices) == 0 {
		return nil, usage, fmt.Errorf("openai: no choices in response")
	}

	choice := resp.Choices[0]

	if choice.FinishReason == "length" {
		return nil, usage, fmt.Errorf("%w (model=%s input=%d output=%d)",
			ErrMaxTokens, agentCfg.Model, usage.InputTokens, usage.OutputTokens)
	}
	if choice.Message.Content == "" {
		return nil, usage, fmt.Errorf("openai: empty content in structured output (finish_reason=%q)", choice.FinishReason)
	}

	return []byte(choice.Message.Content), usage, nil
}

func doCall(ctx context.Context, conf config.BaseConfig, agentCfg config.AgentConfig, body chatRequest) (chatResponse, models.LLMUsage, error) {
	if conf.OpenAIAPIKey == "" {
		return chatResponse{}, models.LLMUsage{}, fmt.Errorf("openai: OPENAI_API_KEY is not configured")
	}

	resp, err := doRequest(ctx, conf, agentCfg.Timeout, body)
	usage := usageFrom(resp)
	if cached := resp.Usage.PromptTokensDetails.CachedTokens; cached > 0 {
		log.Printf("[OPENAI] %s usage: in=%d (cached=%d) out=%d",
			body.Model, usage.InputTokens, cached, usage.OutputTokens)
	}
	return resp, usage, err
}

func doRequest(ctx context.Context, conf config.BaseConfig, timeout time.Duration, body chatRequest) (chatResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return chatResponse{}, fmt.Errorf("openai: marshal request: %w", err)
	}

	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(reqCtx, http.MethodPost, conf.OpenAIBaseURL, bytes.NewReader(jsonBody))
	if err != nil {
		return chatResponse{}, fmt.Errorf("openai: create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+conf.OpenAIAPIKey)

	client := &http.Client{Timeout: timeout}
	httpResp, err := client.Do(httpReq)
	if err != nil {
		return chatResponse{}, fmt.Errorf("openai: request failed: %w", err)
	}
	respBytes, readErr := io.ReadAll(httpResp.Body)
	httpResp.Body.Close()
	if readErr != nil {
		return chatResponse{}, fmt.Errorf("openai: read response: %w", readErr)
	}

	if httpResp.StatusCode == http.StatusUnauthorized {
		return chatResponse{}, fmt.Errorf("openai: 401 unauthorized — check OPENAI_API_KEY")
	}
	if httpResp.StatusCode != http.StatusOK {
		return chatResponse{}, fmt.Errorf("openai: unexpected status %d: %s", httpResp.StatusCode, string(respBytes))
	}

	var resp chatResponse
	if err = json.Unmarshal(respBytes, &resp); err != nil {
		return chatResponse{}, fmt.Errorf("openai: parse response: %w", err)
	}
	return resp, nil
}

func usageFrom(resp chatResponse) models.LLMUsage {
	return models.LLMUsage{
		InputTokens:  resp.Usage.PromptTokens,
		OutputTokens: resp.Usage.CompletionTokens,
	}
}

// OpenAI takes the system prompt as {role:"system"}, not a top-level field.
func prependSystem(system string, messages []chatMessage) []chatMessage {
	if system == "" {
		return messages
	}
	out := make([]chatMessage, 0, len(messages)+1)
	out = append(out, chatMessage{Role: "system", Content: system})
	out = append(out, messages...)
	return out
}

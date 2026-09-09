package anthropic

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

func CallAnthropicAPI(baseConf config.BaseConfig, body AnthropicRequest, timeout time.Duration) (string, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, baseConf.AnthropicBaseAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", baseConf.AnthropicAPIKey)
	req.Header.Set("anthropic-version", baseConf.AnthropicVersion)
	req.Header.Set("anthropic-beta", baseConf.AnthropicBeta)

	return doHTTP(req, timeout)
}

func callAnthropicText(conf config.BaseConfig, agentCfg config.AgentConfig, system string, messages []models.ChatMessage) (string, models.LLMUsage, error) {
	body, err := CallAnthropicAPI(conf, AnthropicRequest{
		Model:     agentCfg.Model,
		MaxTokens: agentCfg.MaxTokens,
		System:    system,
		Messages:  messages,
	}, agentCfg.Timeout)
	if err != nil {
		return "", models.LLMUsage{}, err
	}

	var envelope struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	_ = json.Unmarshal([]byte(body), &envelope)

	return body, models.LLMUsage{
		InputTokens:  envelope.Usage.InputTokens,
		OutputTokens: envelope.Usage.OutputTokens,
	}, nil
}

func callAnthropicTool(conf config.BaseConfig, wire wireToolRequest, timeout time.Duration) ([]byte, models.LLMUsage, string, error) {
	jsonBody, err := json.Marshal(wire)
	if err != nil {
		return nil, models.LLMUsage{}, "", fmt.Errorf("marshal tool request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, conf.AnthropicBaseAPIURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, models.LLMUsage{}, "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", conf.AnthropicAPIKey)
	req.Header.Set("anthropic-version", conf.AnthropicVersion)
	req.Header.Set("anthropic-beta", config.AnthropicCachingBeta)

	respBody, err := doHTTP(req, timeout)
	if err != nil {
		return nil, models.LLMUsage{}, "", err
	}

	var toolResp toolUseResponse
	if err = json.Unmarshal([]byte(respBody), &toolResp); err != nil {
		return nil, models.LLMUsage{}, "", fmt.Errorf("parse tool response envelope: %w", err)
	}

	usage := models.LLMUsage{
		InputTokens:  toolResp.Usage.InputTokens,
		OutputTokens: toolResp.Usage.OutputTokens,
	}

	if toolResp.StopReason == "max_tokens" {
		return nil, usage, toolResp.StopReason,
			fmt.Errorf("%w (model=%s input_tokens=%d output_tokens=%d)",
				ErrMaxTokens, toolResp.Model, usage.InputTokens, usage.OutputTokens)
	}

	for _, block := range toolResp.Content {
		if block.Type != "tool_use" {
			continue
		}
		ai.RepairStringifiedFields(block.Input)
		inputJSON, marshalErr := json.Marshal(block.Input)
		if marshalErr != nil {
			return nil, usage, toolResp.StopReason,
				fmt.Errorf("re-marshal tool input: %w", marshalErr)
		}
		return inputJSON, usage, toolResp.StopReason, nil
	}

	return nil, usage, toolResp.StopReason,
		fmt.Errorf("no tool_use block in response (stop_reason=%q)", toolResp.StopReason)
}

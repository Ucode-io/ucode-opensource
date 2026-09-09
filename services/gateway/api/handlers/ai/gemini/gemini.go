package gemini

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

const baseURL = "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s"

// callGeminiText sends a free-text request (no tools) and returns the text content
// of the first candidate. Used by RouteRequest, InspectCode, DatabaseQuery.
func callGeminiText(pool *KeyPool, agentCfg config.AgentConfig, system string, contents []geminiContent) (string, models.LLMUsage, error) {
	req := geminiRequest{
		SystemInstruction: systemContent(system),
		Contents:          contents,
		GenerationConfig:  generationConfig{MaxOutputTokens: agentCfg.MaxTokens},
	}

	resp, err := doRequest(pool, agentCfg.Model, agentCfg.Timeout, req)
	if err != nil {
		return "", models.LLMUsage{}, err
	}

	usage := usageFromResponse(resp)

	if len(resp.Candidates) == 0 {
		return "", usage, fmt.Errorf("gemini: no candidates in response")
	}

	var parts []string
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strings.Join(parts, "\n"), usage, nil
}

// callGeminiTool sends a function-calling request with mode=ANY forcing the model
// to call the specified tool. Returns the raw JSON bytes of the function args.
func callGeminiTool(pool *KeyPool, agentCfg config.AgentConfig, system string, contents []geminiContent, tool funcDeclaration) ([]byte, models.LLMUsage, error) {
	req := geminiRequest{
		SystemInstruction: systemContent(system),
		Contents:          contents,
		Tools:             []geminiTool{{FunctionDeclarations: []funcDeclaration{tool}}},
		ToolConfig: &geminiToolCfg{
			FunctionCallingConfig: funcCallingConfig{
				Mode:                 "ANY",
				AllowedFunctionNames: []string{tool.Name},
			},
		},
		GenerationConfig: generationConfig{MaxOutputTokens: agentCfg.MaxTokens},
	}

	resp, err := doRequest(pool, agentCfg.Model, agentCfg.Timeout, req)
	if err != nil {
		return nil, models.LLMUsage{}, err
	}

	usage := usageFromResponse(resp)

	if len(resp.Candidates) == 0 {
		return nil, usage, fmt.Errorf("gemini: no candidates in response")
	}

	cand := resp.Candidates[0]
	if cand.FinishReason == "MAX_TOKENS" {
		return nil, usage, fmt.Errorf("%w (model=%s input=%d output=%d)",
			ErrMaxTokens, agentCfg.Model, usage.InputTokens, usage.OutputTokens)
	}

	for _, part := range cand.Content.Parts {
		if part.FunctionCall == nil || part.FunctionCall.Name != tool.Name {
			continue
		}
		args := part.FunctionCall.Args
		ai.RepairStringifiedFields(args)
		inputJSON, err := json.Marshal(args)
		if err != nil {
			return nil, usage, fmt.Errorf("gemini: re-marshal tool args: %w", err)
		}
		return inputJSON, usage, nil
	}

	return nil, usage, fmt.Errorf("gemini: no function_call %q in response (finish_reason=%q)",
		tool.Name, cand.FinishReason)
}

// doRequest executes a Gemini API call with automatic key rotation on 429.
// On each 429 the key is cooled down and the next key is tried immediately.
// Returns ErrAllKeysRateLimited (via pick) if every key is on cooldown.
func doRequest(pool *KeyPool, model string, timeout time.Duration, body geminiRequest) (geminiResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return geminiResponse{}, fmt.Errorf("gemini: marshal request: %w", err)
	}

	client := &http.Client{Timeout: timeout}
	maxAttempts := len(pool.keys) // each attempt either succeeds or burns one key

	for attempt := 0; attempt < maxAttempts; attempt++ {
		key, idx, err := pool.pick()
		if err != nil {
			return geminiResponse{}, err // ErrAllKeysRateLimited — surface immediately
		}

		req, err := http.NewRequest(http.MethodPost, fmt.Sprintf(baseURL, model, key), bytes.NewBuffer(jsonBody))
		if err != nil {
			return geminiResponse{}, fmt.Errorf("gemini: create request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		httpResp, err := client.Do(req)
		if err != nil {
			return geminiResponse{}, fmt.Errorf("gemini: request failed: %w", err)
		}

		respBytes, readErr := io.ReadAll(httpResp.Body)
		httpResp.Body.Close()
		if readErr != nil {
			return geminiResponse{}, fmt.Errorf("gemini: read response: %w", readErr)
		}

		if httpResp.StatusCode == http.StatusTooManyRequests {
			log.Printf("[GEMINI] key[%d] 429 (attempt %d/%d, model=%s) — rotating key",
				idx, attempt+1, maxAttempts, model)
			pool.markRateLimited(idx)
			continue
		}

		if httpResp.StatusCode != http.StatusOK {
			return geminiResponse{}, fmt.Errorf("gemini: unexpected status %d: %s",
				httpResp.StatusCode, string(respBytes))
		}

		var resp geminiResponse
		if err = json.Unmarshal(respBytes, &resp); err != nil {
			return geminiResponse{}, fmt.Errorf("gemini: parse response: %w", err)
		}
		return resp, nil
	}

	return geminiResponse{}, fmt.Errorf("gemini: all %d key attempts exhausted (model=%s)", maxAttempts, model)
}

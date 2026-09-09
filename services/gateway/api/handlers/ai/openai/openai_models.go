package openai

import "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"

var ErrMaxTokens = ai.ErrMaxTokens

type (
	chatRequest struct {
		Model               string          `json:"model"`
		Messages            []chatMessage   `json:"messages"`
		Tools               []chatTool      `json:"tools,omitempty"`
		ToolChoice          any             `json:"tool_choice,omitempty"`
		ResponseFormat      *responseFormat `json:"response_format,omitempty"`
		MaxCompletionTokens int             `json:"max_completion_tokens,omitempty"`
	}

	chatMessage struct {
		Role       string     `json:"role"`
		Content    any        `json:"content,omitempty"` // string | []contentPart
		ToolCalls  []toolCall `json:"tool_calls,omitempty"`
		ToolCallID string     `json:"tool_call_id,omitempty"`
	}

	contentPart struct {
		Type     string    `json:"type"`
		Text     string    `json:"text,omitempty"`
		ImageURL *imageURL `json:"image_url,omitempty"`
	}

	imageURL struct {
		URL    string `json:"url"`
		Detail string `json:"detail,omitempty"`
	}

	chatTool struct {
		Type     string      `json:"type"`
		Function functionDef `json:"function"`
	}

	functionDef struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
		Strict      bool           `json:"strict,omitempty"`
	}

	forcedTool struct {
		Type     string     `json:"type"`
		Function forcedFunc `json:"function"`
	}

	forcedFunc struct {
		Name string `json:"name"`
	}

	responseFormat struct {
		Type       string         `json:"type"`
		JSONSchema jsonSchemaSpec `json:"json_schema"`
	}

	jsonSchemaSpec struct {
		Name   string         `json:"name"`
		Schema map[string]any `json:"schema"`
		Strict bool           `json:"strict"`
	}

	chatResponse struct {
		ID      string `json:"id"`
		Model   string `json:"model"`
		Choices []struct {
			Message struct {
				Role      string     `json:"role"`
				Content   string     `json:"content"`
				ToolCalls []toolCall `json:"tool_calls"`
			} `json:"message"`
			FinishReason string `json:"finish_reason"`
		} `json:"choices"`
		Usage openaiUsage `json:"usage"`
	}

	toolCall struct {
		ID       string `json:"id"`
		Type     string `json:"type"`
		Function struct {
			Name      string `json:"name"`
			Arguments string `json:"arguments"` // JSON-encoded string; unmarshal separately
		} `json:"function"`
	}

	openaiUsage struct {
		PromptTokens        int `json:"prompt_tokens"`
		CompletionTokens    int `json:"completion_tokens"`
		PromptTokensDetails struct {
			CachedTokens int `json:"cached_tokens"`
		} `json:"prompt_tokens_details"`
	}
)

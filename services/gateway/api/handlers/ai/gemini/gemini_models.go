package gemini

import "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"

// ErrMaxTokens aliases ai.ErrMaxTokens so errors.Is checks within this package still work.
var ErrMaxTokens = ai.ErrMaxTokens

type (
	geminiRequest struct {
		SystemInstruction *geminiContent   `json:"systemInstruction,omitempty"`
		Contents          []geminiContent  `json:"contents"`
		Tools             []geminiTool     `json:"tools,omitempty"`
		ToolConfig        *geminiToolCfg   `json:"toolConfig,omitempty"`
		GenerationConfig  generationConfig `json:"generationConfig"`
	}

	geminiContent struct {
		Role  string       `json:"role,omitempty"` // "user" | "model"
		Parts []geminiPart `json:"parts"`
	}

	geminiPart struct {
		Text             string              `json:"text,omitempty"`
		InlineData       *geminiInline       `json:"inlineData,omitempty"`
		FunctionCall     *geminiFuncCall     `json:"functionCall,omitempty"`
		FunctionResponse *geminiFuncResponse `json:"functionResponse,omitempty"`
	}

	geminiFuncResponse struct {
		Name     string         `json:"name"`
		Response map[string]any `json:"response"`
	}

	geminiInline struct {
		MimeType string `json:"mimeType"`
		Data     string `json:"data"` // base64-encoded
	}

	geminiFuncCall struct {
		Name string         `json:"name"`
		Args map[string]any `json:"args"`
	}

	geminiTool struct {
		FunctionDeclarations []funcDeclaration `json:"functionDeclarations"`
	}

	funcDeclaration struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Parameters  map[string]any `json:"parameters"`
	}

	geminiToolCfg struct {
		FunctionCallingConfig funcCallingConfig `json:"functionCallingConfig"`
	}

	funcCallingConfig struct {
		Mode                 string   `json:"mode"` // "AUTO" | "ANY" | "NONE"
		AllowedFunctionNames []string `json:"allowedFunctionNames,omitempty"`
	}

	generationConfig struct {
		MaxOutputTokens int `json:"maxOutputTokens"`
	}

	geminiResponse struct {
		Candidates    []geminiCandidate `json:"candidates"`
		UsageMetadata geminiUsage       `json:"usageMetadata"`
	}

	geminiCandidate struct {
		Content      geminiContent `json:"content"`
		FinishReason string        `json:"finishReason"` // "STOP" | "MAX_TOKENS" | "SAFETY" | ...
	}

	geminiUsage struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
	}
)

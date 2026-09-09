package config

import "strings"

type AIProvider string

const (
	AIProviderClaude AIProvider = "claude"
	AIProviderGemini AIProvider = "gemini"
	AIProviderOpenAI AIProvider = "openai"
	AIProviderAuto   AIProvider = "auto"
)

func ParseAIProvider(s string) AIProvider {
	switch AIProvider(strings.ToLower(strings.TrimSpace(s))) {
	case AIProviderClaude:
		return AIProviderClaude

	case AIProviderGemini:
		return AIProviderGemini

	case AIProviderOpenAI:
		return AIProviderOpenAI

	case AIProviderAuto:
		return AIProviderAuto

	default:
		return AIProviderAuto
	}
}

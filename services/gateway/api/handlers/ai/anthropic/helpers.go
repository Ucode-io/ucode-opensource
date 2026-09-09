package anthropic

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func buildContentBlocks(text string, imageURLs []string) []models.ContentBlock {
	blocks := make([]models.ContentBlock, 0, len(imageURLs)+1)
	for _, url := range imageURLs {
		if strings.TrimSpace(url) != "" {
			blocks = append(blocks, models.ContentBlock{
				Type:   "image",
				Source: &models.ImageSource{Type: "url", URL: url},
			})
		}
	}
	blocks = append(blocks, models.ContentBlock{Type: "text", Text: text})
	return blocks
}

func buildAgentMessages(history []models.ChatMessage, blocks []models.ContentBlock) []models.ChatMessage {
	messages := make([]models.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, models.ChatMessage{Role: "user", Content: blocks})
	return messages
}

func parseHaikuRoutingResult(rawJSON string) (*models.HaikuRoutingResult, error) {
	fullText, err := extractPlainText(rawJSON)
	if err != nil {
		return nil, err
	}

	cleaned := ai.ExtractJSONFromText(fullText)

	var result models.HaikuRoutingResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		log.Printf("[PARSE] haiku routing: unmarshal failed (%v), falling back to plain reply", err)
		return &models.HaikuRoutingResult{
			NextStep: false,
			Intent:   "chat",
			Reply:    fullText,
		}, nil
	}
	return &result, nil
}

func extractPlainText(rawJSON string) (string, error) {
	var resp models.ClaudeResponse
	if err := json.Unmarshal([]byte(rawJSON), &resp); err != nil {
		return "", fmt.Errorf("failed to parse Claude response envelope: %w", err)
	}
	var parts []string
	for _, block := range resp.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n")), nil
}

func wrapMaxTokens(err error, usage models.LLMUsage, stage string) error {
	if errors.Is(err, ErrMaxTokens) {
		log.Printf("[AI] max_tokens: %s (in=%d out=%d)", stage, usage.InputTokens, usage.OutputTokens)
		return fmt.Errorf(
			"generation stopped: the project is too large to generate in one pass (used %d output tokens). "+
				"Please describe a smaller scope or break the request into parts",
			usage.OutputTokens,
		)
	}
	return err
}

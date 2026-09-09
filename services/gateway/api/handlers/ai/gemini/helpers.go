package gemini

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// buildGeminiParts converts a text + optional image URLs into []geminiPart.
// Images are fetched and base64-encoded because Gemini requires inline_data,
// unlike Anthropic which accepts URLs directly.
func buildGeminiParts(text string, imageURLs []string) []geminiPart {
	parts := make([]geminiPart, 0, len(imageURLs)+1)
	for _, url := range imageURLs {
		if strings.TrimSpace(url) == "" {
			continue
		}
		data, mimeType, err := fetchImageAsBase64(url)
		if err != nil {
			log.Printf("[GEMINI] skipping image %s: %v", url, err)
			continue
		}
		parts = append(parts, geminiPart{InlineData: &geminiInline{MimeType: mimeType, Data: data}})
	}
	parts = append(parts, geminiPart{Text: text})
	return parts
}

// buildGeminiContents converts history + final parts into []geminiContent.
func buildGeminiContents(history []models.ChatMessage, finalParts []geminiPart) []geminiContent {
	contents := convertMessages(history)
	return append(contents, geminiContent{Role: "user", Parts: finalParts})
}

// convertMessages converts a full []ChatMessage slice (used by VisualEdit which
// passes pre-built messages directly instead of history + final parts).
func convertMessages(messages []models.ChatMessage) []geminiContent {
	contents := make([]geminiContent, 0, len(messages))
	for _, msg := range messages {
		role := msg.Role
		if role == "assistant" {
			role = "model"
		}
		parts := convertBlocks(msg.Content)
		if len(parts) == 0 {
			continue
		}
		contents = append(contents, geminiContent{Role: role, Parts: parts})
	}
	return contents
}

// convertBlocks converts []models.ContentBlock (Anthropic format) to []geminiPart.
func convertBlocks(blocks []models.ContentBlock) []geminiPart {
	parts := make([]geminiPart, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, geminiPart{Text: b.Text})
			}
		case "image":
			if b.Source == nil || b.Source.URL == "" {
				continue
			}
			data, mimeType, err := fetchImageAsBase64(b.Source.URL)
			if err != nil {
				log.Printf("[GEMINI] skipping history image %s: %v", b.Source.URL, err)
				continue
			}
			parts = append(parts, geminiPart{InlineData: &geminiInline{MimeType: mimeType, Data: data}})
		}
	}
	return parts
}

// systemContent wraps a system prompt string into a geminiContent for system_instruction.
func systemContent(prompt string) *geminiContent {
	if prompt == "" {
		return nil
	}
	return &geminiContent{Parts: []geminiPart{{Text: prompt}}}
}

// parseRoutingResult parses a text response into HaikuRoutingResult,
// falling back to a plain chat reply if JSON extraction fails.
func parseRoutingResult(text string) (*models.HaikuRoutingResult, error) {
	cleaned := ai.ExtractJSONFromText(text)
	var result models.HaikuRoutingResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		log.Printf("[GEMINI] routing: unmarshal failed (%v), falling back to plain reply", err)
		return &models.HaikuRoutingResult{
			NextStep: false,
			Intent:   "chat",
			Reply:    text,
		}, nil
	}
	return &result, nil
}

func wrapMaxTokens(err error, usage models.LLMUsage, stage string) error {
	if errors.Is(err, ErrMaxTokens) {
		log.Printf("[GEMINI] max_tokens: %s (in=%d out=%d)", stage, usage.InputTokens, usage.OutputTokens)
		return fmt.Errorf(
			"generation stopped: the project is too large to generate in one pass (used %d output tokens). "+
				"Please describe a smaller scope or break the request into parts",
			usage.OutputTokens,
		)
	}
	return err
}

func fetchImageAsBase64(url string) (data string, mimeType string, err error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("fetch image: %w", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	if idx := strings.Index(ct, ";"); idx != -1 {
		ct = ct[:idx]
	}
	ct = strings.TrimSpace(ct)
	if ct == "" {
		ct = "image/jpeg"
	}

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("read image: %w", err)
	}
	return base64.StdEncoding.EncodeToString(raw), ct, nil
}

func usageFromResponse(resp geminiResponse) models.LLMUsage {
	return models.LLMUsage{
		InputTokens:  resp.UsageMetadata.PromptTokenCount,
		OutputTokens: resp.UsageMetadata.CandidatesTokenCount,
	}
}

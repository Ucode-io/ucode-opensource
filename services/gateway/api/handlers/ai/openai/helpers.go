package openai

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

func buildOpenAIMessages(history []models.ChatMessage, currentParts []contentPart) []chatMessage {
	msgs := make([]chatMessage, 0, len(history)+1)
	for _, h := range history {
		parts := convertBlocks(h.Content)
		if len(parts) == 0 {
			continue
		}
		msgs = append(msgs, chatMessage{
			Role:    h.Role,
			Content: flattenOrParts(parts),
		})
	}
	if len(currentParts) == 0 {
		return msgs
	}
	return append(msgs, chatMessage{
		Role:    "user",
		Content: flattenOrParts(currentParts),
	})
}

// Images go inline base64 — signed S3 URLs and internal CDNs aren't reachable by OpenAI.
func buildContentParts(text string, imageURLs []string) []contentPart {
	parts := make([]contentPart, 0, len(imageURLs)+1)
	for _, url := range imageURLs {
		if strings.TrimSpace(url) == "" {
			continue
		}
		data, mimeType, err := fetchImageAsBase64(url)
		if err != nil {
			log.Printf("[OPENAI] skipping image %s: %v", url, err)
			continue
		}
		parts = append(parts, contentPart{
			Type:     "image_url",
			ImageURL: &imageURL{URL: dataURI(mimeType, data), Detail: "high"},
		})
	}
	if text != "" {
		parts = append(parts, contentPart{Type: "text", Text: text})
	}
	return parts
}

func convertBlocks(blocks []models.ContentBlock) []contentPart {
	parts := make([]contentPart, 0, len(blocks))
	for _, b := range blocks {
		switch b.Type {
		case "text":
			if b.Text != "" {
				parts = append(parts, contentPart{Type: "text", Text: b.Text})
			}
		case "image":
			if b.Source == nil || b.Source.URL == "" {
				continue
			}
			data, mimeType, err := fetchImageAsBase64(b.Source.URL)
			if err != nil {
				log.Printf("[OPENAI] skipping history image %s: %v", b.Source.URL, err)
				continue
			}
			parts = append(parts, contentPart{
				Type:     "image_url",
				ImageURL: &imageURL{URL: dataURI(mimeType, data), Detail: "high"},
			})
		}
	}
	return parts
}

// For callers (e.g., VisualEdit) that build the full message list themselves
// rather than supplying history + current parts separately.
func convertMessages(messages []models.ChatMessage) []chatMessage {
	out := make([]chatMessage, 0, len(messages))
	for _, m := range messages {
		parts := convertBlocks(m.Content)
		if len(parts) == 0 {
			continue
		}
		out = append(out, chatMessage{
			Role:    m.Role,
			Content: flattenOrParts(parts),
		})
	}
	return out
}

// Plain string for text-only, []contentPart for multipart; " " for empty avoids
// the 400 OpenAI returns when content is the empty string.
func flattenOrParts(parts []contentPart) any {
	if len(parts) == 1 && parts[0].Type == "text" {
		if parts[0].Text == "" {
			return " "
		}
		return parts[0].Text
	}
	if len(parts) == 0 {
		return " "
	}
	return parts
}

// Falls back through ExtractJSONFromText since Structured Outputs occasionally
// wrap JSON in prose despite the schema constraint.
func safeUnmarshal(raw []byte, target any) error {
	if err := json.Unmarshal(raw, target); err == nil {
		return nil
	}
	cleaned := ai.ExtractJSONFromText(string(raw))
	return json.Unmarshal([]byte(cleaned), target)
}

// Falls back to a plain chat reply when JSON parsing fails.
func parseRoutingResult(text string) (*models.HaikuRoutingResult, error) {
	cleaned := ai.ExtractJSONFromText(text)
	var result models.HaikuRoutingResult
	if err := json.Unmarshal([]byte(cleaned), &result); err != nil {
		log.Printf("[OPENAI] routing: unmarshal failed (%v), falling back to plain reply", err)
		return &models.HaikuRoutingResult{
			NextStep: false,
			Intent:   "chat",
			Reply:    text,
		}, nil
	}
	return &result, nil
}

func wrapMaxTokens(err error, usage models.LLMUsage, stage string) error {
	if !errors.Is(err, ErrMaxTokens) {
		return err
	}
	log.Printf("[OPENAI] max_tokens: %s (in=%d out=%d)", stage, usage.InputTokens, usage.OutputTokens)
	return fmt.Errorf(
		"generation stopped: the project is too large to generate in one pass (used %d output tokens). "+
			"Please describe a smaller scope or break the request into parts",
		usage.OutputTokens,
	)
}

// 30s timeout: generous for slow CDNs, bounded enough to fail fast on dead URLs.
func fetchImageAsBase64(url string) (data string, mimeType string, err error) {
	if strings.HasPrefix(url, "data:image/") {
		header, encoded, ok := strings.Cut(url, ",")
		if !ok || !strings.HasSuffix(header, ";base64") {
			return "", "", fmt.Errorf("invalid image data URI")
		}
		mimeType = strings.TrimSuffix(strings.TrimPrefix(header, "data:"), ";base64")
		if _, decodeErr := base64.StdEncoding.DecodeString(encoded); decodeErr != nil {
			return "", "", fmt.Errorf("decode image data URI: %w", decodeErr)
		}
		return encoded, mimeType, nil
	}

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

func dataURI(mimeType, base64Data string) string {
	return "data:" + mimeType + ";base64," + base64Data
}

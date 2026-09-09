package helper

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func extractTextFromClaudeResponse(rawJSON string) (string, *models.ClaudeResponse, error) {
	var (
		resp  models.ClaudeResponse
		parts []string
	)

	if err := json.Unmarshal([]byte(rawJSON), &resp); err != nil {
		return "", nil, fmt.Errorf("failed to parse Claude response envelope: %w", err)
	}

	for _, block := range resp.Content {
		if block.Type == "text" {
			parts = append(parts, block.Text)
		}
	}

	text := strings.TrimSpace(strings.Join(parts, "\n"))
	return text, &resp, nil
}

func ExtractPlainText(rawJSON string) (string, error) {
	text, _, err := extractTextFromClaudeResponse(rawJSON)
	return text, err
}

var (
	jsonBlockRegex    = regexp.MustCompile("(?s)```json\\s*\\n?(.*?)\\n?```")
	genericBlockRegex = regexp.MustCompile("(?s)```\\s*\\n?(.*?)\\n?```")
)

func extractJSON(text string) string {
	if m := jsonBlockRegex.FindStringSubmatch(text); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}

	if m := genericBlockRegex.FindStringSubmatch(text); len(m) > 1 {
		candidate := strings.TrimSpace(m[1])
		if strings.HasPrefix(candidate, "{") || strings.HasPrefix(candidate, "[") {
			return candidate
		}
	}

	startObj := strings.Index(text, "{")
	endObj := strings.LastIndex(text, "}")
	startArr := strings.Index(text, "[")
	endArr := strings.LastIndex(text, "]")

	isObj := startObj != -1 && endObj != -1 && endObj > startObj
	isArr := startArr != -1 && endArr != -1 && endArr > startArr

	if isObj && (!isArr || startObj < startArr) {
		return strings.TrimSpace(text[startObj : endObj+1])
	} else if isArr {
		return strings.TrimSpace(text[startArr : endArr+1])
	}

	return strings.TrimSpace(text)
}

func CleanJSONResponse(input string) string {
	return extractJSON(input)
}

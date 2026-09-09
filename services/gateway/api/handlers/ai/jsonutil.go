package ai

import (
	"encoding/json"
	"log"
	"regexp"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// ExtractJSONFromText finds the first JSON object or array in a text string.
// Checks markdown fences first, then falls back to brace/bracket scanning.
func ExtractJSONFromText(text string) string {
	if idx := strings.Index(text, "```json"); idx != -1 {
		rest := text[idx+7:]
		if end := strings.Index(rest, "```"); end != -1 {
			return strings.TrimSpace(rest[:end])
		}
	}
	if idx := strings.Index(text, "```"); idx != -1 {
		rest := text[idx+3:]
		if end := strings.Index(rest, "```"); end != -1 {
			candidate := strings.TrimSpace(rest[:end])
			if strings.HasPrefix(candidate, "{") || strings.HasPrefix(candidate, "[") {
				return candidate
			}
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

// State markers prefixed on assistant messages. The router branches on the
// most recent one (see DetectConversationState); they must stay in sync with
// what the chat handler writes.
const (
	MarkerQuestionsAsked    = "[QUESTIONS_ASKED]"
	MarkerDiagramsGenerated = "[DIAGRAMS_GENERATED]"
	MarkerError             = "[ERROR]"
)

// routerHistoryWindow caps the transcript sent to the router. Long initial
// requests (multi-page TZ documents) used to push the [QUESTIONS_ASKED] marker
// out of a 6-turn window and trigger an infinite question loop.
const routerHistoryWindow = 12

var stateMarkers = []string{MarkerQuestionsAsked, MarkerDiagramsGenerated, MarkerError}

// BuildHistoryText renders history for the router prompt. The most recent
// state marker is carried over to the top even when it falls outside the
// recency window, so routing never depends on what survived truncation.
func BuildHistoryText(history []models.ChatMessage) string {
	if len(history) == 0 {
		return ""
	}

	start := 0
	if len(history) > routerHistoryWindow {
		start = len(history) - routerHistoryWindow
	}

	var sb strings.Builder
	if idx, marker := mostRecentStateMarker(history); marker != "" && idx < start {
		if firstLine := strings.SplitN(messageText(history[idx]), "\n", 2)[0]; firstLine != "" {
			sb.WriteString("Assistant (carried over from earlier in the conversation, still describes current state): ")
			sb.WriteString(firstLine)
			sb.WriteByte('\n')
		}
	}

	for _, msg := range history[start:] {
		text := messageText(msg)
		if text == "" {
			continue
		}
		if msg.Role == "assistant" {
			sb.WriteString("Assistant: ")
		} else {
			sb.WriteString("User: ")
		}
		sb.WriteString(text)
		sb.WriteByte('\n')
	}
	return sb.String()
}

// DetectConversationState returns the marker on the most recent assistant
// message, or "" if none. It scans the full history so callers see state
// even when the marker falls outside the router's truncation window.
func DetectConversationState(history []models.ChatMessage) string {
	_, marker := mostRecentStateMarker(history)
	return marker
}

// mostRecentStateMarker returns the index and marker of the most recent
// assistant message that starts with a known marker. If the latest assistant
// message is not a marker, state is treated as "none" — older markers are
// considered superseded.
func mostRecentStateMarker(history []models.ChatMessage) (int, string) {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != "assistant" {
			continue
		}
		text := messageText(history[i])
		if text == "" {
			continue
		}
		for _, marker := range stateMarkers {
			if strings.HasPrefix(text, marker) {
				return i, marker
			}
		}
		return -1, ""
	}
	return -1, ""
}

func messageText(msg models.ChatMessage) string {
	var sb strings.Builder
	for _, block := range msg.Content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
}

// RepairStringifiedFields walks a tool-input map and tries to parse any string
// value that looks like a JSON array or object back into a Go value.
// Keys listed in passthroughKeys are left as strings intentionally.
func RepairStringifiedFields(input map[string]interface{}) {
	passthroughKeys := map[string]bool{
		"content": true, "ui_structure": true, "bpmn_xml": true,
		"summary": true, "change_summary": true,
	}
	for k, v := range input {
		s, ok := v.(string)
		if !ok || passthroughKeys[k] {
			continue
		}
		s = strings.TrimSpace(s)
		s = stripCodeFence(s)
		if !isJSONContainer(s) {
			continue
		}
		if parsed := tryParseJSON(s); parsed != nil {
			input[k] = parsed
			continue
		}
		if parsed := tryParseJSON(sanitizeJSONContent(s)); parsed != nil {
			input[k] = parsed
			continue
		}
		if parsed := tryParseJSON(repairJSONStrings(sanitizeJSONContent(s))); parsed != nil {
			input[k] = parsed
			continue
		}
		if k == "files" {
			if extracted, ok := extractFilesFromString(s); ok {
				input[k] = extracted
				continue
			}
		}
		preview := s
		if len(preview) > 200 {
			preview = preview[:200]
		}
		log.Printf("[TOOL DECODE] all repair attempts failed for field %q: %s", k, preview)
	}
}

func stripCodeFence(s string) string {
	if strings.HasPrefix(s, "```json") {
		s = strings.TrimPrefix(s, "```json")
	} else if strings.HasPrefix(s, "```") {
		s = strings.TrimPrefix(s, "```")
	} else {
		return s
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "```")
	return strings.TrimSpace(s)
}

func isJSONContainer(s string) bool {
	return (strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]")) ||
		(strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"))
}

func tryParseJSON(s string) interface{} {
	var v interface{}
	if json.Unmarshal([]byte(s), &v) == nil {
		return v
	}
	return nil
}

func sanitizeJSONContent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x20 || c == '\n' || c == '\r' || c == '\t' || c >= 0x80 {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func repairJSONStrings(input string) string {
	var out strings.Builder
	out.Grow(len(input) + 512)

	inString := false
	escaped := false

	for i := 0; i < len(input); i++ {
		c := input[i]

		if escaped {
			out.WriteByte(c)
			escaped = false
			continue
		}

		if c == '\\' && inString {
			if i+1 < len(input) {
				next := input[i+1]
				validEscape := next == '"' || next == '\\' || next == '/' ||
					next == 'b' || next == 'f' || next == 'n' ||
					next == 'r' || next == 't' || next == 'u'
				if !validEscape {
					out.WriteString(`\\`)
					continue
				}
			}
			out.WriteByte(c)
			escaped = true
			continue
		}

		if c == '"' && !escaped {
			inString = !inString
			out.WriteByte(c)
			continue
		}

		if inString {
			switch c {
			case '\n':
				out.WriteString(`\n`)
				continue
			case '\r':
				out.WriteString(`\r`)
				continue
			case '\t':
				out.WriteString(`\t`)
				continue
			}
		}

		out.WriteByte(c)
	}
	return out.String()
}

func isSourceFilePath(p string) bool {
	if strings.HasPrefix(p, "/") {
		return false
	}
	lower := strings.ToLower(p)
	for _, ext := range []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".ico", ".mp4", ".mp3", ".woff", ".woff2", ".ttf", ".eot", ".otf", ".wav", ".ogg", ".avif"} {
		if strings.HasSuffix(lower, ext) {
			return false
		}
	}
	return true
}

func extractFilesFromString(s string) ([]map[string]interface{}, bool) {
	fileStartRe := regexp.MustCompile(`\{\s*"path"\s*:\s*"([^"\s\\]{1,200})"`)
	allMatches := fileStartRe.FindAllStringSubmatchIndex(s, -1)
	if len(allMatches) == 0 {
		return nil, false
	}

	matches := allMatches[:0]
	for _, m := range allMatches {
		if isSourceFilePath(s[m[2]:m[3]]) {
			matches = append(matches, m)
		}
	}
	if len(matches) == 0 {
		return nil, false
	}

	contentKeyRe := regexp.MustCompile(`"content"\s*:\s*"`)

	var files []map[string]interface{}
	for i, m := range matches {
		chunkStart := m[0]
		var chunkEnd int
		if i+1 < len(matches) {
			chunkEnd = matches[i+1][0]
		} else {
			chunkEnd = len(s)
		}
		chunk := s[chunkStart:chunkEnd]

		pathStart := m[2] - chunkStart
		pathEnd := m[3] - chunkStart
		if pathStart < 0 || pathEnd > len(chunk) {
			continue
		}
		path := chunk[pathStart:pathEnd]

		if !strings.ContainsAny(path, "./") {
			continue
		}

		cLoc := contentKeyRe.FindStringIndex(chunk)
		if cLoc == nil {
			continue
		}
		rawContent := chunk[cLoc[1]:]

		lastQ := strings.LastIndex(rawContent, `"`)
		if lastQ < 0 {
			continue
		}
		rawContent = rawContent[:lastQ]

		content := unescapeJSONString(rawContent)
		files = append(files, map[string]interface{}{
			"path":    path,
			"content": content,
		})
	}

	if len(files) == 0 {
		return nil, false
	}
	return files, true
}

func unescapeJSONString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			b.WriteByte(s[i])
			continue
		}
		i++
		switch s[i] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case '"':
			b.WriteByte('"')
		case '\\':
			b.WriteByte('\\')
		case '/':
			b.WriteByte('/')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'u':
			if i+4 < len(s) {
				var r rune
				for _, c := range s[i+1 : i+5] {
					r <<= 4
					switch {
					case c >= '0' && c <= '9':
						r |= rune(c - '0')
					case c >= 'a' && c <= 'f':
						r |= rune(c-'a') + 10
					case c >= 'A' && c <= 'F':
						r |= rune(c-'A') + 10
					default:
						r = -1
					}
					if r < 0 {
						break
					}
				}
				if r >= 0 {
					b.WriteRune(r)
					i += 4
					continue
				}
			}
			b.WriteString(`\u`)
		default:
			b.WriteByte('\\')
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

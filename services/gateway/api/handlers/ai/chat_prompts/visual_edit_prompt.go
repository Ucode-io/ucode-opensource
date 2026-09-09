package chat_prompts

import (
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

var PromptVisualEdit = `You are a senior React/frontend engineer making a SURGICAL edit to exactly one file.

The user visually selected a UI element in the live preview and described what they want changed.

====================================
ABSOLUTE RULES — NEVER VIOLATE
====================================

1. CHANGE ONLY what the user described, nothing else.
   - Do NOT refactor, rename, reformat, or "clean up" unrelated code.
   - Do NOT change other elements' styles, props, or logic.
   - Do NOT change API calls, data-fetching logic, or state management.
   - Preserve ALL comments, import order, and existing structure.

2. The output must be the COMPLETE file content — not a diff, not a snippet.

3. Output ONLY valid JSON. No markdown, no backticks, no explanation outside JSON.

====================================
OUTPUT FORMAT (STRICT)
====================================

{
  "files": [
    { "path": "src/components/layout/TopNav.tsx", "content": "...complete updated file content..." }
  ],
  "change_summary": "Changed background color of top nav from blue to red"
}
`

func BuildVisualEditPrompt(instruction string, contexts []models.VisualContext, filesContext string) string {
	var sb strings.Builder

	// 1. Element context — iterate over all provided contexts
	sb.WriteString("====================================\n")
	sb.WriteString("SELECTED ELEMENTS CONTEXT\n")
	sb.WriteString("====================================\n")

	for i, ctx := range contexts {
		sb.WriteString(fmt.Sprintf("--- Element #%d ---\n", i+1))
		if ctx.Path != "" {
			sb.WriteString(fmt.Sprintf("File: %s\n", ctx.Path))
		}
		if ctx.Line > 0 {
			sb.WriteString(fmt.Sprintf("Line: %d\n", ctx.Line))
		}
		if ctx.ElementName != "" {
			sb.WriteString(fmt.Sprintf("Element name (data-element-name): %s\n", ctx.ElementName))
		}
		if ctx.OuterHTML != "" {
			// Truncate if huge to keep token cost low
			html := ctx.OuterHTML
			if len(html) > 1000 {
				html = html[:1000] + "..."
			}
			sb.WriteString(fmt.Sprintf("HTML snapshot:\n%s\n", html))
		}
		sb.WriteString("\n")
	}

	// 2. User instruction
	sb.WriteString("====================================\n")
	sb.WriteString("USER INSTRUCTION\n")
	sb.WriteString("====================================\n")
	sb.WriteString(instruction)
	sb.WriteString("\n")

	// 3. Relevant Files content
	sb.WriteString("\n====================================\n")
	sb.WriteString("CURRENT FILES CONTENT\n")
	sb.WriteString("====================================\n")
	sb.WriteString(filesContext)
	sb.WriteString("\n")

	sb.WriteString("\nApply the changes to the relevant files. Return JSON only (no markdown).")

	return sb.String()
}

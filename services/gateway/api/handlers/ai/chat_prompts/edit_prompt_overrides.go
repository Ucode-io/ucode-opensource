package chat_prompts

import "strings"

const codeEditorOverrideContract = `

====================================
PLATFORM OUTPUT CONTRACT — IMMUTABLE
====================================
- Always call the emit_project tool; never answer conversationally.
- Return complete final file contents, never diffs, snippets, placeholders, or truncated files.
- Output only files assigned by the change plan. Do not emit unrelated or protected project files.
`

const visualEditorOverrideContract = `

====================================
PLATFORM OUTPUT CONTRACT — IMMUTABLE
====================================
- Use the visual-edit tool output only; never answer conversationally.
- Return complete final file contents, never diffs, snippets, placeholders, or truncated files.
- Output only files belonging to the user's resolved visual selection.
`

// CodeEditorSystemPrompt resolves a request-scoped code editor prompt without
// mutating the package defaults. Chunked edits always retain the immutable
// parallel-worker scope in front of the editable prompt body.
func CodeEditorSystemPrompt(promptOverride string, chunked bool) string {
	if strings.TrimSpace(promptOverride) == "" {
		if chunked {
			return PromptCodeEditorChunk
		}
		return PromptCodeEditor
	}

	if chunked {
		return promptCodeEditorChunkPreamble + promptOverride + codeEditorOverrideContract
	}
	return promptOverride + codeEditorOverrideContract
}

// VisualEditorSystemPrompt resolves the visual editor prompt for one request.
// The provider's forced visual-edit tool and the caller's selected-file filter
// remain authoritative when this returns a user-provided prompt.
func VisualEditorSystemPrompt(promptOverride string) string {
	if strings.TrimSpace(promptOverride) == "" {
		return PromptVisualEdit
	}
	return promptOverride + visualEditorOverrideContract
}

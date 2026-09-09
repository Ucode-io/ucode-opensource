package chat_prompts

import (
	"strings"
	"testing"
)

func TestCodeEditorSystemPromptDefaultsRemainUnchanged(t *testing.T) {
	if got := CodeEditorSystemPrompt("", false); got != PromptCodeEditor {
		t.Fatal("non-chunked empty override must use PromptCodeEditor unchanged")
	}
	if got := CodeEditorSystemPrompt(" \n\t", true); got != PromptCodeEditorChunk {
		t.Fatal("chunked empty override must use PromptCodeEditorChunk unchanged")
	}
}

func TestCodeEditorSystemPromptComposesChunkGuardWithOverride(t *testing.T) {
	const custom = "Follow this project's component conventions."

	if got := CodeEditorSystemPrompt(custom, false); got != custom+codeEditorOverrideContract {
		t.Fatalf("non-chunked custom prompt mismatch: %q", got)
	}

	got := CodeEditorSystemPrompt(custom, true)
	if got != promptCodeEditorChunkPreamble+custom+codeEditorOverrideContract {
		t.Fatal("chunked custom prompt must follow the immutable chunk preamble")
	}
	for _, rule := range []string{"YOUR ASSIGNED FILES", "Output ONLY files", "PLATFORM OUTPUT CONTRACT", custom} {
		if !strings.Contains(got, rule) {
			t.Fatalf("chunked custom prompt must contain %q", rule)
		}
	}
}

func TestVisualEditorSystemPromptResolvesOverride(t *testing.T) {
	if got := VisualEditorSystemPrompt(""); got != PromptVisualEdit {
		t.Fatal("empty visual override must use PromptVisualEdit unchanged")
	}

	const custom = "Only adjust the requested visual property."
	if got := VisualEditorSystemPrompt(custom); got != custom+visualEditorOverrideContract {
		t.Fatalf("custom visual prompt mismatch: %q", got)
	}
}

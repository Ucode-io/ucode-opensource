package chat_prompts

import (
	"regexp"
	"strings"
	"testing"
)

func TestMobilePlatformPromptDifferentiatesWebAndCapacitor(t *testing.T) {
	for _, expected := range []string{`"id": "web-app"`, `"id": "mobile-app"`, `installable Capacitor mobile app`} {
		if !strings.Contains(PromptRouter, expected) {
			t.Fatalf("PromptRouter must contain %q", expected)
		}
	}

	platformQuestion := regexp.MustCompile(`(?s)"id": "platform-types".{0,180}"type": "single"`)
	if matches := platformQuestion.FindAllString(PromptRouter, -1); len(matches) != 2 {
		t.Fatalf("expected both platform-types examples to be single-select, got %d", len(matches))
	}
}

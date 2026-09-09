package v1

import (
	"context"
	"errors"
	"testing"

	chatprompts "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
)

type fakeAIEditPromptStore struct {
	prompts []models.AIEditPrompt
	err     error
	calls   *int
}

func (s fakeAIEditPromptStore) GetAll(context.Context, services.ServiceManagerI, string) ([]models.AIEditPrompt, error) {
	if s.calls != nil {
		(*s.calls)++
	}
	return s.prompts, s.err
}

func (fakeAIEditPromptStore) Upsert(context.Context, services.ServiceManagerI, string, models.AIEditPrompt, int64) (models.AIEditPrompt, error) {
	return models.AIEditPrompt{}, errors.New("not implemented")
}

func (fakeAIEditPromptStore) Delete(context.Context, services.ServiceManagerI, string, string, int64) error {
	return errors.New("not implemented")
}

func TestBuildAIEditPromptSettingsUsesCustomAndDefaultContent(t *testing.T) {
	response := buildAIEditPromptSettings([]models.AIEditPrompt{
		{
			PromptKind:      models.AIEditPromptKindCodeEditor,
			Content:         "custom code prompt",
			Revision:        3,
			UpdatedByUserID: "user-id",
		},
	}, true)

	if !response.StorageAvailable {
		t.Fatal("storage must be reported available")
	}
	if len(response.Prompts) != 2 {
		t.Fatalf("got %d prompts, want 2", len(response.Prompts))
	}

	codePrompt := response.Prompts[0]
	if codePrompt.Content != "custom code prompt" || codePrompt.Source != models.AIEditPromptSourceCustom || codePrompt.Revision != 3 {
		t.Fatalf("unexpected code prompt: %#v", codePrompt)
	}
	if codePrompt.DefaultContent != chatprompts.PromptCodeEditor {
		t.Fatal("code editor default content does not match compiled prompt")
	}

	visualPrompt := response.Prompts[1]
	if visualPrompt.Content != chatprompts.PromptVisualEdit || visualPrompt.Source != models.AIEditPromptSourceDefault || visualPrompt.CustomContent != nil {
		t.Fatalf("unexpected visual prompt: %#v", visualPrompt)
	}
}

func TestParseAIEditPromptKindRejectsUnknownKind(t *testing.T) {
	if _, err := parseAIEditPromptKind("planner"); err == nil {
		t.Fatal("unknown prompt kind must be rejected")
	}
}

func TestParseExpectedRevision(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    int64
		wantErr bool
	}{
		{name: "omitted", value: "", want: 0},
		{name: "current", value: "12", want: 12},
		{name: "negative", value: "-1", wantErr: true},
		{name: "invalid", value: "latest", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := parseExpectedRevision(test.value)
			if (err != nil) != test.wantErr {
				t.Fatalf("parseExpectedRevision(%q) error = %v, wantErr %v", test.value, err, test.wantErr)
			}
			if got != test.want {
				t.Fatalf("parseExpectedRevision(%q) = %d, want %d", test.value, got, test.want)
			}
		})
	}
}

func TestReadAIEditPromptOverrides(t *testing.T) {
	h := &HandlerV1{aiEditPromptStore: fakeAIEditPromptStore{prompts: []models.AIEditPrompt{
		{PromptKind: models.AIEditPromptKindCodeEditor, Content: "custom code"},
		{PromptKind: models.AIEditPromptKindVisualEditor, Content: "custom visual"},
	}}}

	overrides := h.readAIEditPromptOverrides(
		context.Background(),
		nil,
		"resource-env-id",
	)
	if overrides.CodeEditor != "custom code" || overrides.VisualEditor != "custom visual" {
		t.Fatalf("unexpected overrides: %#v", overrides)
	}
}

func TestReadAIEditPromptOverridesFallsBackOnReadFailure(t *testing.T) {
	h := &HandlerV1{aiEditPromptStore: fakeAIEditPromptStore{err: errors.New("storage unavailable")}}

	overrides := h.readAIEditPromptOverrides(
		context.Background(),
		nil,
		"resource-env-id",
	)
	if overrides.CodeEditor != "" || overrides.VisualEditor != "" {
		t.Fatalf("read failure must use compiled defaults, got %#v", overrides)
	}
}

func TestLoadAIEditPromptOverridesNeverReadsMainDBWithoutChildMapping(t *testing.T) {
	calls := 0
	h := &HandlerV1{aiEditPromptStore: fakeAIEditPromptStore{calls: &calls}}

	overrides := h.loadAIEditPromptOverrides(context.Background(), "", "")
	if overrides.CodeEditor != "" || overrides.VisualEditor != "" {
		t.Fatalf("missing child mapping must use compiled defaults, got %#v", overrides)
	}
	if calls != 0 {
		t.Fatalf("missing child mapping must not read prompt storage, got %d call(s)", calls)
	}
}

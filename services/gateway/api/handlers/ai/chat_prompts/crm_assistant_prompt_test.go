package chat_prompts

import (
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestBuildRelativeDateHintResolvesUzbekYesterdayInCRMTimezone(t *testing.T) {
	hint := buildRelativeDateHint(
		"kecha nechta lid keldi",
		"2026-09-03T04:45:00Z",
		"Asia/Tashkent",
	)

	for _, expected := range []string{
		"requested_period=yesterday",
		"local_date=2026-09-02",
		"local_wall_interval=[2026-09-02 00:00:00, 2026-09-03 00:00:00)",
		"offset_interval=[2026-09-02T00:00:00+05:00, 2026-09-03T00:00:00+05:00)",
	} {
		if !strings.Contains(hint, expected) {
			t.Fatalf("relative date hint %q does not contain %q", hint, expected)
		}
	}
}

func TestBuildRelativeDateHintResolvesRussianToday(t *testing.T) {
	hint := buildRelativeDateHint(
		"Сколько лидов пришло сегодня?",
		"2026-09-03T04:45:00Z",
		"Asia/Tashkent",
	)

	if !strings.Contains(hint, "requested_period=today") || !strings.Contains(hint, "local_date=2026-09-03") {
		t.Fatalf("unexpected relative date hint: %q", hint)
	}
}

func TestBuildRelativeDateHintIgnoresQuestionWithoutPeriod(t *testing.T) {
	if hint := buildRelativeDateHint("Eng katta budgetli deal qaysi?", "2026-09-03T04:45:00Z", "Asia/Tashkent"); hint != "" {
		t.Fatalf("unexpected relative date hint: %q", hint)
	}
}

func TestBuildRelativeDateHintResolvesWeekAndMonthRanges(t *testing.T) {
	tests := []struct {
		message  string
		period   string
		interval string
	}{
		{message: "bu hafta nechta lid keldi", period: "this_week", interval: "[2026-08-31 00:00:00, 2026-09-07 00:00:00)"},
		{message: "oldingi hafta qancha lid kegan", period: "last_week", interval: "[2026-08-24 00:00:00, 2026-08-31 00:00:00)"},
		{message: "otkan hafta qancha lid kegan", period: "last_week", interval: "[2026-08-24 00:00:00, 2026-08-31 00:00:00)"},
		{message: "bu oy nechta lid keldi", period: "this_month", interval: "[2026-09-01 00:00:00, 2026-10-01 00:00:00)"},
		{message: "o'tgan oy nechta lid keldi", period: "last_month", interval: "[2026-08-01 00:00:00, 2026-09-01 00:00:00)"},
		{message: "otkan oy qancha lid kegan", period: "last_month", interval: "[2026-08-01 00:00:00, 2026-09-01 00:00:00)"},
	}
	for _, test := range tests {
		hint := buildRelativeDateHint(test.message, "2026-09-03T04:45:00Z", "Asia/Tashkent")
		if !strings.Contains(hint, "requested_period="+test.period) || !strings.Contains(hint, "local_wall_interval="+test.interval) {
			t.Fatalf("unexpected hint for %q: %q", test.message, hint)
		}
	}
}

func TestBuildRelativeDateHintResolvesNamedMonth(t *testing.T) {
	hint := buildRelativeDateHint("avgustda qaysi source eng ko‘p lid olib kelgan", "2026-09-03T04:45:00Z", "Asia/Tashkent")
	if !strings.Contains(hint, "requested_period=named_month") ||
		!strings.Contains(hint, "local_wall_interval=[2026-08-01 00:00:00, 2026-09-01 00:00:00)") {
		t.Fatalf("unexpected named-month hint: %q", hint)
	}

	previousYear := buildRelativeDateHint("2025 yil dekabrda nechta lid kelgan", "2026-09-03T04:45:00Z", "Asia/Tashkent")
	if !strings.Contains(previousYear, "local_wall_interval=[2025-12-01 00:00:00, 2026-01-01 00:00:00)") {
		t.Fatalf("unexpected explicit-year hint: %q", previousYear)
	}
}

func TestBuildRelativeDateHintDoesNotTreatMaydonAsMay(t *testing.T) {
	if hint := buildRelativeDateHint("lid kartochkasidagi maydonni ko‘rsat", "2026-09-03T04:45:00Z", "Asia/Tashkent"); hint != "" {
		t.Fatalf("unexpected May hint for maydon: %q", hint)
	}
}

func TestCRMAssistantPromptTreatsReadableCardScreenshotAsCompleteRequest(t *testing.T) {
	for _, expected := range []string{
		"cardni shunaqa qiber",
		"Do not ask the user to repeat fields",
		"return action=\"client_action\" on the first response",
		"branching/workflow pill is pipeline",
		"currency row is amount",
		"colored-dot status pill is stage",
		"legacy stage is absent from page_context.card_fields",
		"duplicate-count badge is an automatic decoration",
	} {
		if !strings.Contains(PromptCRMAssistant, expected) {
			t.Fatalf("CRM assistant prompt is missing screenshot rule %q", expected)
		}
	}
}

func TestCRMAssistantPromptFormatsGroupedAnalyticsAsMarkdownTable(t *testing.T) {
	for _, expected := range []string{
		"GitHub Markdown pipe table",
		"Do not use dash bullets for tabular data",
	} {
		if !strings.Contains(PromptCRMAssistant, expected) {
			t.Fatalf("CRM assistant prompt is missing table-format rule %q", expected)
		}
	}
}

func TestCRMAssistantPromptCoversCRMEntityMutations(t *testing.T) {
	for _, expected := range []string{
		`action="pipeline_action"`,
		`action="record_action"`,
		`create_pipeline`,
		`reorder_stages`,
		`A request to create a new lead/deal means INSERT into deals`,
		`Never run UPDATE or DELETE without a restrictive WHERE clause`,
		`Never inherit an old date period for a current mutation command`,
	} {
		if !strings.Contains(PromptCRMAssistant, expected) {
			t.Fatalf("CRM assistant prompt is missing mutation rule %q", expected)
		}
	}
}

func TestBuildCRMAssistantMessageIncludesCurrentRenderableCardFields(t *testing.T) {
	message := BuildCRMAssistantMessage(models.CRMAssistantInput{
		Message: "cardni shunaqa qiber",
		PageContext: models.CRMAssistantPageContext{
			Table: "deals",
			CardFields: []models.CRMCardFieldContext{
				{Slug: "name", Label: "Deal name"},
				{Slug: "pipeline_sales", Label: "Sales Project"},
			},
		},
	})

	if !strings.Contains(message, `card_fields=name(label="Deal name"),pipeline_sales(label="Sales Project")`) {
		t.Fatalf("current card fields are missing from assistant message: %q", message)
	}
}

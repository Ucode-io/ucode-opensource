package v1

import (
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func crmAnalyticsTestSchema() []models.TableSchema {
	return []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "created_at", Type: "timestamp without time zone"},
			{Slug: "deleted_at", Type: "timestamp without time zone"},
			{Slug: "stage", Type: "MULTISELECT"},
		},
	}}
}

func TestBuildCommonCRMAnalyticsPlanForYesterdayLeadCount(t *testing.T) {
	plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
		Message: "kecha neshta lid keldi",
		PageContext: models.CRMAssistantPageContext{
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}, crmAnalyticsTestSchema())
	if !ok {
		t.Fatal("expected common lead count analytics plan")
	}
	if plan.kind != crmAnalyticsLeadCount {
		t.Fatalf("got kind %q, want %q", plan.kind, crmAnalyticsLeadCount)
	}
	if !strings.Contains(plan.sql, `COUNT(*)`) || !strings.Contains(plan.sql, `"created_at"`) {
		t.Fatalf("unexpected SQL: %s", plan.sql)
	}
	if got, want := plan.params[0], "2026-09-02 00:00:00"; got != want {
		t.Fatalf("got start %v, want %v", got, want)
	}
	if got, want := plan.params[1], "2026-09-03 00:00:00"; got != want {
		t.Fatalf("got end %v, want %v", got, want)
	}
}

func TestBuildCommonCRMAnalyticsPlanCarriesYesterdayIntoStatusFollowUp(t *testing.T) {
	plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
		Message: "kechigi kegan lidla hozir qaysi statusda",
		History: []models.CRMAssistantMessage{
			{Role: "user", Content: "kecha neshta lid keldi"},
			{Role: "assistant", Content: "Kecha 12 ta lid keldi."},
		},
		PageContext: models.CRMAssistantPageContext{
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}, crmAnalyticsTestSchema())
	if !ok {
		t.Fatal("expected common lead status analytics plan")
	}
	if plan.kind != crmAnalyticsLeadStatus {
		t.Fatalf("got kind %q, want %q", plan.kind, crmAnalyticsLeadStatus)
	}
	if plan.language != "uz" {
		t.Fatalf("got language %q, want Uzbek", plan.language)
	}
	for _, expected := range []string{`to_jsonb("stage")`, "GROUP BY 1", `"created_at"`} {
		if !strings.Contains(plan.sql, expected) {
			t.Fatalf("SQL %q does not contain %q", plan.sql, expected)
		}
	}
}

func TestBuildCommonCRMAnalyticsPlanGroupsBySourceAndPipeline(t *testing.T) {
	tests := []struct {
		message   string
		kind      crmAnalyticsKind
		dimension string
	}{
		{message: "kecha kelgan lidlar source bo‘yicha nechta", kind: crmAnalyticsLeadSource, dimension: "source"},
		{message: "kecha kelgan lidlar pipeline bo‘yicha nechta", kind: crmAnalyticsLeadPipeline, dimension: "pipeline"},
	}
	for _, test := range tests {
		plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
			Message: test.message,
			PageContext: models.CRMAssistantPageContext{
				Now:      "2026-09-03T05:00:00Z",
				Timezone: "Asia/Tashkent",
			},
		}, crmAnalyticsTestSchema())
		if !ok {
			t.Fatalf("expected plan for %q", test.message)
		}
		if plan.kind != test.kind || !strings.Contains(plan.sql, `to_jsonb("`+test.dimension+`")`) {
			t.Fatalf("unexpected plan for %q: %#v", test.message, plan)
		}
	}
}

func TestBuildCommonCRMAnalyticsPlanRecognizesRequestedPercentages(t *testing.T) {
	plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
		Message: "otkan hafta neshta lid keldi sourcelari boyicha protsentlarini korsat",
		PageContext: models.CRMAssistantPageContext{
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}, crmAnalyticsTestSchema())
	if !ok || plan.kind != crmAnalyticsLeadSource {
		t.Fatalf("expected source analytics plan, got %#v, ok=%v", plan, ok)
	}
	if !plan.includePercentage {
		t.Fatal("expected percentage output to be enabled")
	}
}

func TestBuildCommonCRMAnalyticsPlanListsYesterdayLeadPhones(t *testing.T) {
	plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
		Message: "kechigi kegan lidlani nomerini tashavor",
		PageContext: models.CRMAssistantPageContext{
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}, crmAnalyticsTestSchema())
	if !ok || plan.kind != crmAnalyticsLeadPhones {
		t.Fatalf("expected phone-list plan, got %#v, ok=%v", plan, ok)
	}
	for _, expected := range []string{`LEFT JOIN "contacts"`, `c."phone"`, `d."created_at"`, "LIMIT 50"} {
		if !strings.Contains(plan.sql, expected) {
			t.Fatalf("phone SQL %q does not contain %q", plan.sql, expected)
		}
	}
}

func TestBuildCommonCRMAnalyticsPlanUnderstandsPhoneFollowUpFromHistory(t *testing.T) {
	plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
		Message: "nomerlarini ham ber",
		History: []models.CRMAssistantMessage{
			{Role: "user", Content: "kechagi kelgan lidla qaysi statusda"},
			{Role: "assistant", Content: "Kecha kelgan 12 ta lid hozir turli statuslarda."},
		},
		PageContext: models.CRMAssistantPageContext{
			Table:    "deals",
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}, crmAnalyticsTestSchema())
	if !ok || plan.kind != crmAnalyticsLeadPhones {
		t.Fatalf("expected history-aware phone-list plan, got %#v, ok=%v", plan, ok)
	}
	if plan.periodLabel != "Kecha" {
		t.Fatalf("got period %q, want Kecha", plan.periodLabel)
	}
}

func TestBuildCommonCRMAnalyticsPlanUsesDealsPageAsImplicitLeadContext(t *testing.T) {
	tests := []string{
		"kecha tushgan klientlarni aloqa raqamlari kerak",
		"kechagi oqimdan kelganlani telini ber",
		"bu hafta nechta tushdi",
	}
	for _, message := range tests {
		request := models.CRMAssistantRequest{
			Message: message,
			PageContext: models.CRMAssistantPageContext{
				Table:    "deals",
				Now:      "2026-09-03T05:00:00Z",
				Timezone: "Asia/Tashkent",
			},
		}
		if _, ok := buildCommonCRMAnalyticsPlan(request, crmAnalyticsTestSchema()); !ok {
			t.Fatalf("expected implicit deals-page plan for %q", message)
		}
	}
}

func TestBuildCommonCRMAnalyticsPlanSupportsLanguagesAndToday(t *testing.T) {
	tests := []struct {
		name     string
		message  string
		language string
		start    string
	}{
		{name: "uzbek typo", message: "bugun neshta yangi lid keldi", language: "uz", start: "2026-09-03 00:00:00"},
		{name: "russian", message: "сколько лидов пришло вчера", language: "ru", start: "2026-09-02 00:00:00"},
		{name: "english", message: "how many leads arrived today", language: "en", start: "2026-09-03 00:00:00"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
				Message: test.message,
				PageContext: models.CRMAssistantPageContext{
					Now:      "2026-09-03T05:00:00Z",
					Timezone: "Asia/Tashkent",
				},
			}, crmAnalyticsTestSchema())
			if !ok {
				t.Fatal("expected common analytics plan")
			}
			if plan.language != test.language {
				t.Fatalf("got language %q, want %q", plan.language, test.language)
			}
			if got := plan.params[0]; got != test.start {
				t.Fatalf("got start %v, want %v", got, test.start)
			}
		})
	}
}

func TestBuildCommonCRMAnalyticsPlanUsesCurrentWeekInsteadOfYesterdayHistory(t *testing.T) {
	plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
		Message: "bu hafta neshta lid keldi",
		History: []models.CRMAssistantMessage{
			{Role: "user", Content: "kecha neshta lid keldi"},
			{Role: "assistant", Content: "Kecha 12 ta lid keldi."},
		},
		PageContext: models.CRMAssistantPageContext{
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}, crmAnalyticsTestSchema())
	if !ok {
		t.Fatal("expected current-week analytics plan")
	}
	if got, want := plan.periodLabel, "Bu hafta"; got != want {
		t.Fatalf("got label %q, want %q", got, want)
	}
	if got, want := plan.params[0], "2026-08-31 00:00:00"; got != want {
		t.Fatalf("got start %v, want %v", got, want)
	}
	if got, want := plan.params[1], "2026-09-07 00:00:00"; got != want {
		t.Fatalf("got end %v, want %v", got, want)
	}
}

func TestBuildCommonCRMAnalyticsPlanSupportsWeekAndMonthPeriods(t *testing.T) {
	tests := []struct {
		message string
		label   string
		start   string
		end     string
	}{
		{message: "o‘tgan hafta nechta lid keldi", label: "O‘tgan hafta", start: "2026-08-24 00:00:00", end: "2026-08-31 00:00:00"},
		{message: "avvalgi hafta qancha lid kegan", label: "O‘tgan hafta", start: "2026-08-24 00:00:00", end: "2026-08-31 00:00:00"},
		{message: "otkan hafta qancha lid kegan", label: "O‘tgan hafta", start: "2026-08-24 00:00:00", end: "2026-08-31 00:00:00"},
		{message: "how many leads came in this week", label: "This week", start: "2026-08-31 00:00:00", end: "2026-09-07 00:00:00"},
		{message: "сколько лидов пришло на прошлой неделе", label: "На прошлой неделе", start: "2026-08-24 00:00:00", end: "2026-08-31 00:00:00"},
		{message: "bu oy nechta lid keldi", label: "Bu oy", start: "2026-09-01 00:00:00", end: "2026-10-01 00:00:00"},
		{message: "o'tgan oy nechta lid keldi", label: "O‘tgan oy", start: "2026-08-01 00:00:00", end: "2026-09-01 00:00:00"},
		{message: "oldingi oyda qancha lid kegan", label: "O‘tgan oy", start: "2026-08-01 00:00:00", end: "2026-09-01 00:00:00"},
		{message: "otkan oyda qancha lid kegan", label: "O‘tgan oy", start: "2026-08-01 00:00:00", end: "2026-09-01 00:00:00"},
	}
	for _, test := range tests {
		t.Run(test.message, func(t *testing.T) {
			plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
				Message: test.message,
				PageContext: models.CRMAssistantPageContext{
					Now:      "2026-09-03T05:00:00Z",
					Timezone: "Asia/Tashkent",
				},
			}, crmAnalyticsTestSchema())
			if !ok {
				t.Fatal("expected period analytics plan")
			}
			if plan.periodLabel != test.label || plan.params[0] != test.start || plan.params[1] != test.end {
				t.Fatalf("unexpected period plan: label=%q params=%v", plan.periodLabel, plan.params)
			}
		})
	}
}

func TestBuildCommonCRMAnalyticsPlanSupportsNamedMonths(t *testing.T) {
	tests := []struct {
		message string
		label   string
		start   string
		end     string
	}{
		{message: "avgustda qaysi source eng ko‘p lid olib kelgan", label: "Avgust oyida", start: "2026-08-01 00:00:00", end: "2026-09-01 00:00:00"},
		{message: "2025 yil dekabrda nechta lid kelgan", label: "Dekabr oyida 2025", start: "2025-12-01 00:00:00", end: "2026-01-01 00:00:00"},
		{message: "how many leads came in August", label: "In August", start: "2026-08-01 00:00:00", end: "2026-09-01 00:00:00"},
		{message: "сколько лидов пришло в августе", label: "В августе", start: "2026-08-01 00:00:00", end: "2026-09-01 00:00:00"},
	}
	for _, test := range tests {
		plan, ok := buildCommonCRMAnalyticsPlan(models.CRMAssistantRequest{
			Message: test.message,
			PageContext: models.CRMAssistantPageContext{
				Table:    "deals",
				Now:      "2026-09-03T05:00:00Z",
				Timezone: "Asia/Tashkent",
			},
		}, crmAnalyticsTestSchema())
		if !ok {
			t.Fatalf("expected named-month plan for %q", test.message)
		}
		if plan.periodLabel != test.label || plan.params[0] != test.start || plan.params[1] != test.end {
			t.Fatalf("unexpected named-month plan: label=%q params=%v", plan.periodLabel, plan.params)
		}
	}
}

func TestFindCRMMonthDoesNotTreatMaydonAsMay(t *testing.T) {
	if _, ok := findCRMMonth("lid kartochkasidagi maydonni ko‘rsat"); ok {
		t.Fatal("maydon must not be interpreted as the month May")
	}
}

func TestBuildCommonCRMAnalyticsPlanFallsBackForUnmatchedQuestions(t *testing.T) {
	tests := []models.CRMAssistantRequest{
		{
			Message: "eng katta budgetli deal qaysi",
			PageContext: models.CRMAssistantPageContext{
				Now:      "2026-09-03T05:00:00Z",
				Timezone: "Asia/Tashkent",
			},
		},
		{
			Message: "lidlar qaysi statusda",
			PageContext: models.CRMAssistantPageContext{
				Now:      "2026-09-03T05:00:00Z",
				Timezone: "Asia/Tashkent",
			},
		},
	}

	for _, request := range tests {
		if plan, ok := buildCommonCRMAnalyticsPlan(request, crmAnalyticsTestSchema()); ok {
			t.Fatalf("unexpected common analytics plan: %#v", plan)
		}
	}
}

func TestBuildCommonCRMAnalyticsPlanNeverHijacksMutationsFromEarlierPeriodHistory(t *testing.T) {
	request := models.CRMAssistantRequest{
		Message: "Generate new pipeline \"Enterprise\", with stages\n- New Lead\n- Meet\n- Proposal\n- Contract\n- Payment",
		History: []models.CRMAssistantMessage{
			{Role: "user", Content: "bu hafta nechta lid keldi"},
			{Role: "assistant", Content: "Bu hafta 14 ta lid keldi."},
		},
		PageContext: models.CRMAssistantPageContext{
			Table:    "deals",
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}

	if plan, ok := buildCommonCRMAnalyticsPlan(request, crmAnalyticsTestSchema()); ok {
		t.Fatalf("pipeline mutation was hijacked as analytics: %#v", plan)
	}
}

func TestBuildCommonCRMAnalyticsPlanSupportsSystemFieldsMissingFromSchemaList(t *testing.T) {
	request := models.CRMAssistantRequest{
		Message: "kecha nechta lid keldi",
		PageContext: models.CRMAssistantPageContext{
			Now:      "2026-09-03T05:00:00Z",
			Timezone: "Asia/Tashkent",
		},
	}
	if _, ok := buildCommonCRMAnalyticsPlan(request, []models.TableSchema{{Slug: "deals"}}); !ok {
		t.Fatal("expected conventional deal system fields to be supported")
	}
	if _, ok := buildCommonCRMAnalyticsPlan(request, nil); !ok {
		t.Fatal("expected optimized analytics to work before schema loading")
	}
	if plan, ok := buildCommonCRMAnalyticsPlan(request, []models.TableSchema{{Slug: "contacts"}}); ok {
		t.Fatalf("unexpected plan without deals table: %#v", plan)
	}
}

func TestFormatCommonCRMAnalyticsReplies(t *testing.T) {
	countReply, err := formatCommonCRMAnalyticsReply(crmAnalyticsPlan{
		kind:        crmAnalyticsLeadCount,
		periodLabel: "Kecha",
		language:    "uz",
	}, map[string]any{"rows": []map[string]any{{"count": float64(12)}}})
	if err != nil || countReply != "Kecha 12 ta lid keldi." {
		t.Fatalf("unexpected count reply %q, err=%v", countReply, err)
	}

	englishReply, err := formatCommonCRMAnalyticsReply(crmAnalyticsPlan{
		kind:        crmAnalyticsLeadCount,
		periodLabel: "Today",
		language:    "en",
	}, map[string]any{"rows": []map[string]any{{"count": float64(1)}}})
	if err != nil || englishReply != "Today, 1 lead came in." {
		t.Fatalf("unexpected English count reply %q, err=%v", englishReply, err)
	}

	statusReply, err := formatCommonCRMAnalyticsReply(crmAnalyticsPlan{
		kind:        crmAnalyticsLeadStatus,
		periodLabel: "Kecha",
		language:    "uz",
	}, map[string]any{"rows": []map[string]any{
		{"dimension": "Неквалифицированный", "count": "4"},
		{"dimension": "Недозвон", "count": float64(3)},
		{"dimension": "Согласование договора", "count": int64(3)},
		{"dimension": "Новая заявка", "count": float64(2)},
	}})
	if err != nil {
		t.Fatalf("format status reply: %v", err)
	}
	for _, expected := range []string{"Kecha kelgan 12 ta lid", "Неквалифицированный — 4", "Новая заявка — 2"} {
		if !strings.Contains(statusReply, expected) {
			t.Fatalf("status reply %q does not contain %q", statusReply, expected)
		}
	}

	sourceReply, err := formatCommonCRMAnalyticsReply(crmAnalyticsPlan{
		kind:        crmAnalyticsLeadSource,
		periodLabel: "Kecha",
		language:    "uz",
	}, map[string]any{"rows": []map[string]any{
		{"dimension": "target", "count": float64(8)},
		{"dimension": "Source belgilanmagan", "count": float64(4)},
	}})
	if err != nil || !strings.Contains(sourceReply, "source bo‘yicha") || !strings.Contains(sourceReply, "target — 8") {
		t.Fatalf("unexpected source reply %q, err=%v", sourceReply, err)
	}

	sourcePercentageReply, err := formatCommonCRMAnalyticsReply(crmAnalyticsPlan{
		kind:              crmAnalyticsLeadSource,
		periodLabel:       "Bu hafta",
		language:          "uz",
		includePercentage: true,
	}, map[string]any{"rows": []map[string]any{
		{"dimension": "target", "count": float64(9)},
		{"dimension": "Source belgilanmagan", "count": float64(4)},
	}})
	if err != nil {
		t.Fatalf("format source percentage reply: %v", err)
	}
	for _, expected := range []string{
		"| Source | Lidlar | Ulush |",
		"| target | 9 | 69.2% |",
		"| Source belgilanmagan | 4 | 30.8% |",
		"Jami: 13 ta lid.",
	} {
		if !strings.Contains(sourcePercentageReply, expected) {
			t.Fatalf("source percentage reply %q does not contain %q", sourcePercentageReply, expected)
		}
	}

	phoneReply, err := formatCommonCRMAnalyticsReply(crmAnalyticsPlan{
		kind:        crmAnalyticsLeadPhones,
		periodLabel: "Kecha",
		language:    "uz",
	}, map[string]any{"rows": []map[string]any{
		{"lead_name": "Test lead", "phone": "+998900000000"},
		{"lead_name": "No phone", "phone": nil},
	}})
	if err != nil || !strings.Contains(phoneReply, "telefon raqamlari") || !strings.Contains(phoneReply, "Test lead — +998900000000") || !strings.Contains(phoneReply, "telefon raqami ko‘rsatilmagan") {
		t.Fatalf("unexpected phone reply %q, err=%v", phoneReply, err)
	}
}

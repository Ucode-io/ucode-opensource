package v1

import (
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	auth "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"

	"github.com/gin-gonic/gin"
)

func TestCRMRequestRequiresLiveLookupUnderstandsInformalLanguageAndFollowUps(t *testing.T) {
	tests := []struct {
		name    string
		request models.CRMAssistantRequest
		want    bool
	}{
		{
			name:    "informal phone request",
			request: models.CRMAssistantRequest{Message: "kechigi kegan lidlani nomerini tashavor"},
			want:    true,
		},
		{
			name:    "client synonym",
			request: models.CRMAssistantRequest{Message: "kecha tushgan klientlarni aloqa raqamlari kerak"},
			want:    true,
		},
		{
			name: "short follow up",
			request: models.CRMAssistantRequest{
				Message: "nomerlarini ham ber",
				History: []models.CRMAssistantMessage{{Role: "user", Content: "kecha kelgan lidlar qaysi statusda"}},
			},
			want: true,
		},
		{
			name:    "analytical superlative",
			request: models.CRMAssistantRequest{Message: "eng katta budgetli deal qaysi"},
			want:    true,
		},
		{
			name:    "field settings are not data lookup",
			request: models.CRMAssistantRequest{Message: "lid kartochkasida source ko‘rinadigan qil"},
			want:    false,
		},
		{
			name:    "ordinary conversation",
			request: models.CRMAssistantRequest{Message: "rahmat, zo‘r bo‘ldi"},
			want:    false,
		},
		{
			name: "implicit current page entity",
			request: models.CRMAssistantRequest{
				Message:     "kechagi oqimdan kelganlani telini ber",
				PageContext: models.CRMAssistantPageContext{Table: "deals"},
			},
			want: true,
		},
		{
			name: "field settings without saying card",
			request: models.CRMAssistantRequest{
				Message:     "source korinadigon qiber budgetni yashir",
				PageContext: models.CRMAssistantPageContext{Table: "deals"},
			},
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := crmRequestRequiresLiveLookup(test.request); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestCRMReplyIsClarification(t *testing.T) {
	if !crmReplyIsClarification("Qaysi davrni nazarda tutdingiz?") {
		t.Fatal("expected clarification question")
	}
	if crmReplyIsClarification("Kechikkan lidlar topilmadi.") {
		t.Fatal("a guessed no-results answer must not bypass the live lookup guard")
	}
}

func TestCRMRequestRecognizesScreenshotCardMatchAsFieldSettings(t *testing.T) {
	message := "lidlarimni cardini shunaqa qiber"
	if !crmRequestLooksLikeFieldSettings(message) {
		t.Fatalf("expected %q to be recognized as a field-setting request", message)
	}
	request := models.CRMAssistantRequest{
		Message:     message,
		Images:      []string{"data:image/jpeg;base64,eA=="},
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}
	if crmRequestRequiresLiveLookup(request) {
		t.Fatal("screenshot card matching must not be misclassified as a live analytics lookup")
	}
}

func TestCRMRequestRecognizesScreenshotReferenceWithoutCardKeyword(t *testing.T) {
	message := "lidlarimni shunaqa qiber"
	if !crmRequestLooksLikeFieldSettings(message) {
		t.Fatalf("expected %q to be recognized as a screenshot field-setting request", message)
	}
	request := models.CRMAssistantRequest{
		Message:     message,
		Images:      []string{"data:image/jpeg;base64,eA=="},
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}
	if crmRequestRequiresLiveLookup(request) {
		t.Fatal("screenshot reference must not be misclassified as a live analytics lookup")
	}
}

func TestCRMAnalyticsTableRequestIsNotFieldSettings(t *testing.T) {
	message := "CODEX pipeline bo'yicha har bir stagedagi lidlar sonini tableda ko'rsat"
	if crmRequestLooksLikeFieldSettings(message) {
		t.Fatalf("analytics table request must not be recognized as card settings: %q", message)
	}
	request := models.CRMAssistantRequest{
		Message:     message,
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}
	if !crmRequestRequiresLiveLookup(request) {
		t.Fatal("analytics table request must require a live CRM lookup")
	}
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "stage", Label: "Stage"},
		},
	}}
	if result, handled := buildCommonCRMFieldSettings(request, schema); handled || result != nil {
		t.Fatalf("analytics request must not produce card settings: %#v", result)
	}
}

func TestNormalizeCRMRecordFieldValueWrapsMultiselectScalar(t *testing.T) {
	got, err := normalizeCRMRecordFieldValue(models.FieldSchema{Slug: "source", Type: "MULTISELECT"}, "codex-e2e")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"codex-e2e"}) {
		t.Fatalf("multiselect scalar = %#v", got)
	}
	if got, err := normalizeCRMRecordFieldValue(models.FieldSchema{Slug: "name", Type: "SINGLE_LINE"}, "Acme"); err != nil || got != "Acme" {
		t.Fatalf("single line value = %#v", got)
	}
}

func TestCRMRecordBulkCreateRequiresExactRequestedCount(t *testing.T) {
	schema := []models.TableSchema{{Slug: "deals", Fields: []models.FieldSchema{{Slug: "name", Type: "SINGLE_LINE"}}}}
	action := &models.CRMRecordAction{Operation: "create", Table: "deals", Records: []map[string]any{
		{"name": "Dummy 1"}, {"name": "Dummy 2"}, {"name": "Dummy 3"}, {"name": "Dummy 4"},
	}}
	pending, err := newCRMRecordPendingAction("add dummy data to pipeline Enterprise about 4 leads", action, schema, "env")
	if err != nil || pending == nil {
		t.Fatalf("expected valid bulk action: pending=%#v err=%v", pending, err)
	}
	action.Records = action.Records[:1]
	if _, err := newCRMRecordPendingAction("add dummy data to pipeline Enterprise about 4 leads", action, schema, "env"); err == nil {
		t.Fatal("bulk create with wrong record count must be rejected")
	}
}

func TestCRMRecordScalarFieldRejectsMultipleValues(t *testing.T) {
	if _, err := normalizeCRMRecordFieldValue(models.FieldSchema{Slug: "name", Type: "SINGLE_LINE"}, []any{"Dummy 1", "Dummy 2"}); err == nil {
		t.Fatal("scalar array must be rejected")
	}
}

func TestCRMBatchCreatesPipelineAndExactSpreadsheetRows(t *testing.T) {
	schema := []models.TableSchema{{Slug: "deals", Fields: []models.FieldSchema{
		{Slug: "name", Type: "SINGLE_LINE"},
		{Slug: "pipeline", Type: "SINGLE_LINE"},
		{Slug: "stage", Type: "SINGLE_LINE"},
		{Slug: "amount", Type: "NUMBER"},
	}}}
	action := &models.CRMBatchAction{
		PipelineAction: &models.CRMPipelineAction{
			Operation: "create_pipeline", PipelineName: "CODEX Excel",
			Stages: []models.CRMPipelineStageInput{{Name: "New"}, {Name: "Won", Group: "won"}},
		},
		RecordAction: &models.CRMRecordAction{
			Operation: "create", Table: "deals",
			Records: []map[string]any{
				{"name": "Row 1", "stage": "New", "amount": 100},
				{"name": "Row 2", "stage": "Won", "amount": 200},
			},
		},
	}
	pending, err := newCRMBatchPendingAction("Excel ichidagi 2 ta deal uchun yangi pipeline yarat", action, schema, "env")
	if err != nil {
		t.Fatal(err)
	}
	if pending.Action != "client_batch" {
		t.Fatalf("pending action = %q", pending.Action)
	}
	decoded, err := decodeStoredCRMBatchAction(pending.Data)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(decoded.RecordAction.Records); got != 2 {
		t.Fatalf("records = %d", got)
	}
	for index, record := range decoded.RecordAction.Records {
		if record["pipeline"] != "CODEX Excel" {
			t.Fatalf("record %d pipeline = %#v", index+1, record["pipeline"])
		}
	}
}

func TestSpreadsheetImportBypassesStandalonePipelineParser(t *testing.T) {
	req := models.CRMAssistantRequest{Message: "Create pipeline \"Excel Import\" with stages\n- New\n- Won\n\nSPREADSHEET_IMPORT_DATA\nrow_count=2\nrows=[]\nEND_SPREADSHEET_IMPORT_DATA"}
	if result, handled := buildCommonCRMPipelineAction(req, "env"); handled || result != nil {
		t.Fatalf("spreadsheet import must reach the combined batch planner: %#v", result)
	}
}

func TestSpreadsheetImportBuildsAtomicPipelineAndDeals(t *testing.T) {
	schema := []models.TableSchema{{Slug: "deals", Fields: []models.FieldSchema{
		{Slug: "name", Label: "Name", Type: "SINGLE_LINE"},
		{Slug: "amount", Label: "Amount", Type: "NUMBER"},
		{Slug: "stage", Label: "Stage", Type: "SINGLE_LINE"},
		{Slug: "source", Label: "Source", Type: "MULTISELECT"},
		{Slug: "pipeline", Label: "Pipeline", Type: "MULTISELECT"},
	}}}
	req := models.CRMAssistantRequest{Message: "Create a new pipeline \"CODEX Excel\" with stages New Lead, Qualification, Won and import all attached Excel deals\n\nSPREADSHEET_IMPORT_DATA\nrow_count=2\nrows=[{\"Name\":\"Lead 1\",\"Amount\":100,\"Stage\":\"New Lead\",\"Source\":\"Excel\"},{\"Name\":\"Lead 2\",\"Amount\":200,\"Stage\":\"Won\",\"Source\":\"Excel\"}]\nEND_SPREADSHEET_IMPORT_DATA"}
	result, handled := buildCommonCRMSpreadsheetBatchAction(req, schema, "env")
	if !handled || result == nil || result.pendingAction == nil {
		t.Fatalf("expected atomic pending action: handled=%v result=%#v", handled, result)
	}
	if result.pendingAction.Action != "client_batch" {
		t.Fatalf("action = %q", result.pendingAction.Action)
	}
	action, err := decodeStoredCRMBatchAction(result.pendingAction.Data)
	if err != nil {
		t.Fatal(err)
	}
	if len(action.PipelineAction.Stages) != 3 || action.PipelineAction.Stages[2].Name != "Won" {
		t.Fatalf("stages = %#v", action.PipelineAction.Stages)
	}
	if len(action.RecordAction.Records) != 2 {
		t.Fatalf("records = %#v", action.RecordAction.Records)
	}
	if !reflect.DeepEqual(action.RecordAction.Records[0]["source"], []any{"Excel"}) && !reflect.DeepEqual(action.RecordAction.Records[0]["source"], []string{"Excel"}) {
		t.Fatalf("source = %#v", action.RecordAction.Records[0]["source"])
	}
}

func TestCRMMutationCancelMessageUsesRequestLanguage(t *testing.T) {
	tests := map[string]string{
		"Ksjdd deal budgetini o‘zgartir": "Amal bekor qilindi. Hech narsa o‘zgarmadi.",
		"Update the Ksjdd deal budget":   "Action cancelled. No data was changed.",
		"Измени бюджет сделки Ksjdd":     "Действие отменено. Данные не изменены.",
	}
	for message, want := range tests {
		if got := crmMutationCancelMessage(message); got != want {
			t.Fatalf("message %q: got %q, want %q", message, got, want)
		}
	}
}

func TestCRMRequestLooksLikeMutationAcrossLanguages(t *testing.T) {
	tests := map[string]bool{
		`Generate new pipeline "Enterprise" with stages`: true,
		`yangi lid qo‘shib ber`:                          true,
		`Ksjdd dealni Proposal stagega ko‘chir`:          true,
		`Создай новую воронку продаж`:                    true,
		`bu hafta stage bo‘yicha nechta lid keldi`:       false,
		`pipeline bo‘yicha foizlarini ko‘rsat`:           false,
	}
	for message, want := range tests {
		if got := crmRequestLooksLikeMutation(message); got != want {
			t.Fatalf("message %q: got %v, want %v", message, got, want)
		}
	}
}

func TestBuildCommonCRMPipelineActionParsesExactMultilineRequest(t *testing.T) {
	request := models.CRMAssistantRequest{
		Message: `Generate new pipleline "Enterprise", with stages
- New Lead
- Meet
- Proposal
- Contract
- Payment`,
	}
	result, ok := buildCommonCRMPipelineAction(request, "resource-env")
	if !ok || result.pendingAction == nil {
		t.Fatalf("expected pending pipeline action, got %#v, ok=%v", result, ok)
	}
	if result.pendingAction.Action != "client_pipeline" {
		t.Fatalf("action = %q", result.pendingAction.Action)
	}
	action, err := decodeStoredCRMPipelineAction(result.pendingAction.Data)
	if err != nil {
		t.Fatal(err)
	}
	if action.PipelineName != "Enterprise" || len(action.Stages) != 5 {
		t.Fatalf("unexpected parsed action: %#v", action)
	}
	if action.Stages[0].Name != "New Lead" || action.Stages[4].Name != "Payment" {
		t.Fatalf("unexpected stage order: %#v", action.Stages)
	}
	if action.Stages[4].Group != "won" || action.Stages[4].Probability != 100 {
		t.Fatalf("final payment stage was not normalized as won: %#v", action.Stages[4])
	}
}

func TestBuildCommonCRMPipelineActionParsesScreenshotOneLineRequest(t *testing.T) {
	request := models.CRMAssistantRequest{
		Message: `Generate new pipleline "CODEX Enterprise", with stages - New Lead - Meet - Proposal - Contract - Payment`,
	}
	result, ok := buildCommonCRMPipelineAction(request, "resource-env")
	if !ok || result.pendingAction == nil {
		t.Fatalf("expected pending pipeline action, got %#v, ok=%v", result, ok)
	}
	action, err := decodeStoredCRMPipelineAction(result.pendingAction.Data)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"New Lead", "Meet", "Proposal", "Contract", "Payment"}
	if len(action.Stages) != len(want) {
		t.Fatalf("stages = %#v", action.Stages)
	}
	for index, name := range want {
		if action.Stages[index].Name != name {
			t.Fatalf("stage %d = %q, want %q", index, action.Stages[index].Name, name)
		}
	}
}

func TestNormalizeCRMPipelineActionRejectsDuplicateStages(t *testing.T) {
	_, err := normalizeCRMPipelineAction(models.CRMPipelineAction{
		Operation:    "create_pipeline",
		PipelineName: "Enterprise",
		Stages: []models.CRMPipelineStageInput{
			{Name: "New Lead"},
			{Name: "new lead"},
		},
	})
	if err == nil {
		t.Fatal("duplicate stages must be rejected")
	}
}

func TestNormalizeCRMRecordActionAllowsExactSchemaFields(t *testing.T) {
	action, err := normalizeCRMRecordAction(models.CRMRecordAction{
		Operation: "create",
		Table:     "deals",
		Data: map[string]any{
			"name":     "CODEX test lead",
			"pipeline": "Enterprise",
			"stage":    "New Lead",
			"amount":   12345,
		},
	}, []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "name"}, {Slug: "pipeline"}, {Slug: "stage"}, {Slug: "amount"},
		},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if action.Operation != "create" || action.Table != "deals" || len(action.Data) != 4 {
		t.Fatalf("unexpected normalized action: %#v", action)
	}
}

func TestNormalizeCRMRecordActionRequiresExactGUIDAndSafeFields(t *testing.T) {
	if _, err := normalizeCRMRecordAction(models.CRMRecordAction{
		Operation: "update", Table: "deals", Data: map[string]any{"amount": 1},
	}, nil); err == nil {
		t.Fatal("update without guid must be rejected")
	}
	if _, err := normalizeCRMRecordAction(models.CRMRecordAction{
		Operation: "update", Table: "deals", RecordGUID: "deal-1", Data: map[string]any{"deleted_at": "now"},
	}, nil); err == nil {
		t.Fatal("protected fields must be rejected")
	}
	if _, err := normalizeCRMRecordAction(models.CRMRecordAction{
		Operation: "delete", Table: "deals", RecordGUID: "deal-1", Data: map[string]any{"name": "ignored"},
	}, nil); err != nil {
		t.Fatalf("exact delete should be accepted: %v", err)
	}
}

func TestDetectCRMRequestLanguageHandlesMixedCRMVocabulary(t *testing.T) {
	tests := map[string]string{
		"Ksjdd deal budgetini o‘zgartir": "uz",
		"Update the Ksjdd deal budget":   "en",
		"Измени бюджет сделки Ksjdd":     "ru",
		"kecha kelgan лидlar":            "uz",
	}
	for message, want := range tests {
		if got := detectCRMRequestLanguage(message); got != want {
			t.Fatalf("message %q: got %q, want %q", message, got, want)
		}
	}
}

func TestCRMLeadPeriodQueryRequiresCreatedAt(t *testing.T) {
	tests := []struct {
		message string
		want    bool
	}{
		{message: "avgustda qaysi source eng ko‘p lid olib kelgan", want: true},
		{message: "o‘tgan oygi lid oqimida qaysi kun pik bo‘lgan", want: true},
		{message: "shu oygi leadlar umumiy summasi", want: true},
		{message: "eng katta budgetli deal qaysi", want: false},
		{message: "avgustdagi lidlarni start_date bo‘yicha guruhla", want: false},
	}

	for _, test := range tests {
		t.Run(test.message, func(t *testing.T) {
			request := models.CRMAssistantRequest{Message: test.message}
			if got := crmLeadPeriodQueryRequiresCreatedAt(request); got != test.want {
				t.Fatalf("got %v, want %v", got, test.want)
			}
		})
	}
}

func TestCRMSQLUsesCreatedAt(t *testing.T) {
	if !crmSQLUsesCreatedAt(`SELECT COUNT(*) FROM "deals" WHERE d."created_at" >= $1`) {
		t.Fatal("created_at query was not recognized")
	}
	if crmSQLUsesCreatedAt(`SELECT COUNT(*) FROM "deals" WHERE "start_date" >= $1`) {
		t.Fatal("start_date must not satisfy the creation-time guard")
	}
	if crmSQLUsesCreatedAt(`SELECT "created_at" FROM "deals" WHERE "start_date" >= $1`) {
		t.Fatal("selecting created_at must not bypass a start_date filter")
	}
}

func TestCRMSQLMutationRequiresRestrictiveWhere(t *testing.T) {
	tests := []struct {
		sql     string
		sqlType SQLType
		want    bool
	}{
		{`INSERT INTO deals (name) VALUES ($1)`, SQLTypeInsert, true},
		{`UPDATE deals SET amount = $1 WHERE guid = $2`, SQLTypeUpdate, true},
		{`DELETE FROM deals WHERE guid = $1`, SQLTypeDelete, true},
		{`UPDATE deals SET amount = 0`, SQLTypeUpdate, false},
		{`DELETE FROM deals WHERE true`, SQLTypeDelete, false},
		{`WITH target AS (SELECT guid FROM deals WHERE name = $1) DELETE FROM deals WHERE guid IN (SELECT guid FROM target)`, SQLTypeDelete, true},
		{`WITH target AS (SELECT guid FROM deals WHERE name = $1) DELETE FROM deals`, SQLTypeDelete, false},
	}
	for _, test := range tests {
		if got := crmSQLMutationHasRestrictiveWhere(test.sql, test.sqlType); got != test.want {
			t.Fatalf("sql %q: got %v, want %v", test.sql, got, test.want)
		}
	}
}

func TestNormalizeCRMClientActionsResolvesLabelsAndConflicts(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "source", Label: "Source"},
			{Slug: "budget", Label: "Deal budget"},
			{Slug: "phone_number", Label: "Phone number"},
		},
	}}
	actions := normalizeCRMClientActions([]models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		ShowFields: []string{"Source", "phone number", "source"},
		HideFields: []string{"Deal budget", "source", "unknown"},
		FieldOrder: []string{"Phone number", "Source"},
	}}, schema, "deals")

	want := []models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"source", "phone_number"},
		HideFields: []string{"budget"},
		FieldOrder: []string{"phone_number", "source"},
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("unexpected actions:\n got: %#v\nwant: %#v", actions, want)
	}
}

func TestBuildCommonCRMFieldSettingsShowHide(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "source", Label: "Source"},
			{Slug: "amount", Label: "Сумма"},
		},
	}}
	result, ok := buildCommonCRMFieldSettings(models.CRMAssistantRequest{
		Message:     "source korinadigon qiber keyin budgetni yashir kartochkada",
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}, schema)
	if !ok || len(result.clientActions) != 1 {
		t.Fatalf("expected field action, got %#v, ok=%v", result, ok)
	}
	action := result.clientActions[0]
	if !reflect.DeepEqual(action.ShowFields, []string{"source"}) || !reflect.DeepEqual(action.HideFields, []string{"amount"}) {
		t.Fatalf("unexpected show/hide action: %#v", action)
	}
}

func TestBuildCommonCRMFieldSettingsUnderstandsNegativeShowForm(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "source", Label: "Source"},
			{Slug: "amount", Label: "Сумма"},
		},
	}}
	result, ok := buildCommonCRMFieldSettings(models.CRMAssistantRequest{
		Message:     "kartochkada source korinsin budgetni korsatma",
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}, schema)
	if !ok || len(result.clientActions) != 1 {
		t.Fatalf("expected field action, got %#v, ok=%v", result, ok)
	}
	action := result.clientActions[0]
	if !reflect.DeepEqual(action.ShowFields, []string{"source"}) || !reflect.DeepEqual(action.HideFields, []string{"amount"}) {
		t.Fatalf("unexpected negative-show action: %#v", action)
	}
}

func TestBuildCommonCRMFieldSettingsOrdersPhoneAndSource(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "source", Label: "Source"},
		},
	}}
	result, ok := buildCommonCRMFieldSettings(models.CRMAssistantRequest{
		Message:     "kartochkada telefonni birinchi source ni ikkinchi qiber",
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}, schema)
	if !ok || len(result.clientActions) != 1 {
		t.Fatalf("expected ordered field action, got %#v, ok=%v", result, ok)
	}
	action := result.clientActions[0]
	want := []string{"contacts_id", "source"}
	if !reflect.DeepEqual(action.FieldOrder, want) || !reflect.DeepEqual(action.ShowFields, want) {
		t.Fatalf("unexpected ordered field action: %#v", action)
	}
}

func TestBuildCommonCRMFieldSettingsLeavesScreenshotsForVisionAgent(t *testing.T) {
	result, ok := buildCommonCRMFieldSettings(models.CRMAssistantRequest{
		Message:     "source ni ko‘rsat",
		Images:      []string{"data:image/png;base64,eA=="},
		PageContext: models.CRMAssistantPageContext{Table: "deals"},
	}, []models.TableSchema{{Slug: "deals", Fields: []models.FieldSchema{{Slug: "source"}}}})
	if ok || result != nil {
		t.Fatal("screenshot-backed settings must be handled by the vision agent")
	}
}

func TestNormalizeCRMClientActionsResolvesBudgetAliasToAmount(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "source", Label: "Source"},
			{Slug: "amount", Label: "Сумма"},
		},
	}}
	actions := normalizeCRMClientActions([]models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"source"},
		HideFields: []string{"budget"},
	}}, schema, "deals")

	want := []models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"source"},
		HideFields: []string{"amount"},
		FieldOrder: []string{},
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("unexpected actions:\n got: %#v\nwant: %#v", actions, want)
	}
}

func TestNormalizeCRMClientActionsResolvesDealPhoneToContactField(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "source", Label: "Source"},
		},
	}}
	actions := normalizeCRMClientActions([]models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"phone"},
		FieldOrder: []string{"source", "phone"},
	}}, schema, "deals")

	want := []models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"contacts_id"},
		HideFields: []string{},
		FieldOrder: []string{"source", "contacts_id"},
	}}
	if !reflect.DeepEqual(actions, want) {
		t.Fatalf("unexpected actions:\n got: %#v\nwant: %#v", actions, want)
	}
}

func TestFormatSchemaForCRMIncludesLabelsAndStatusOptions(t *testing.T) {
	formatted := formatSchemaForCRM([]models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "pipeline", Label: "Voronka", Type: "MULTISELECT"},
			{Slug: "pipeline_sales", Label: "Sales Project", Type: "STATUS", Options: []string{"Yangi lid", "Sotildi"}},
		},
	}})

	for _, expected := range []string{
		`pipeline MULTISELECT [label="Voronka"]`,
		`pipeline_sales STATUS [label="Sales Project"] [options="Yangi lid|Sotildi"]`,
	} {
		if !strings.Contains(formatted, expected) {
			t.Fatalf("formatted schema %q does not contain %q", formatted, expected)
		}
	}
}

func TestCanonicalDealScreenshotCardActionUsesActiveStatusAndHidesOtherFields(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "name", Type: "SINGLE_LINE"},
			{Slug: "pipeline", Type: "MULTISELECT"},
			{Slug: "amount", Type: "NUMBER"},
			{Slug: "pipeline_sales_project", Type: "STATUS"},
			{Slug: "pipeline_other", Type: "STATUS"},
			{Slug: "contacts_id", Type: "LOOKUP"},
			{Slug: "notes", Type: "MULTI_LINE"},
		},
	}}
	pageContext := models.CRMAssistantPageContext{
		Table: "deals",
		CardFields: []models.CRMCardFieldContext{
			{Slug: "pipeline"},
			{Slug: "name"},
			{Slug: "contacts_id"},
			{Slug: "amount"},
			{Slug: "pipeline_sales_project"},
			{Slug: "notes"},
		},
	}

	got, ok := canonicalDealScreenshotCardAction(schema, pageContext)
	if !ok {
		t.Fatal("expected the active deal status field to produce a canonical screenshot action")
	}
	want := models.CRMClientAction{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"name", "pipeline", "amount", "pipeline_sales_project"},
		HideFields: []string{"contacts_id", "notes"},
		FieldOrder: []string{"name", "pipeline", "amount", "pipeline_sales_project"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("canonical screenshot action = %#v, want %#v", got, want)
	}
}

func TestCanonicalDealScreenshotCardActionRejectsAmbiguousActiveStatus(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "pipeline_a", Type: "STATUS"},
			{Slug: "pipeline_b", Type: "STATUS"},
		},
	}}
	pageContext := models.CRMAssistantPageContext{
		Table: "deals",
		CardFields: []models.CRMCardFieldContext{
			{Slug: "name"},
			{Slug: "pipeline"},
			{Slug: "amount"},
			{Slug: "pipeline_a"},
			{Slug: "pipeline_b"},
		},
	}

	if _, ok := canonicalDealScreenshotCardAction(schema, pageContext); ok {
		t.Fatal("ambiguous active STATUS fields must not be guessed")
	}
}

func TestCanonicalDealScreenshotCardActionUsesSelectedPipelineSlugWhenSchemaTypeIsMissing(t *testing.T) {
	schema := []models.TableSchema{{
		Slug: "deals",
		Fields: []models.FieldSchema{
			{Slug: "name", Type: "SINGLE_LINE"},
			{Slug: "pipeline", Type: "MULTISELECT"},
			{Slug: "amount", Type: "NUMBER"},
			{Slug: "pipeline_sales_project", Type: ""},
		},
	}}
	pageContext := models.CRMAssistantPageContext{
		Table: "deals",
		CardFields: []models.CRMCardFieldContext{
			{Slug: "name"},
			{Slug: "pipeline"},
			{Slug: "amount"},
			{Slug: "pipeline_sales_project"},
			{Slug: "contacts_id"},
		},
	}

	got, ok := canonicalDealScreenshotCardAction(schema, pageContext)
	if !ok {
		t.Fatal("expected the selected pipeline_* field to identify the active status")
	}
	if !reflect.DeepEqual(got.FieldOrder, []string{"name", "pipeline", "amount", "pipeline_sales_project"}) {
		t.Fatalf("field order = %#v", got.FieldOrder)
	}
	if !reflect.DeepEqual(got.HideFields, []string{"contacts_id"}) {
		t.Fatalf("hidden fields = %#v", got.HideFields)
	}
}

func TestCRMClientActionsContainCommonDealScreenshotCore(t *testing.T) {
	actions := []models.CRMClientAction{{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: []string{"pipeline", "contacts_id", "amount"},
		FieldOrder: []string{"pipeline", "name", "contacts_id", "amount"},
	}}
	if !crmClientActionsContainFields(actions, "name", "pipeline", "amount") {
		t.Fatal("expected the model's partially-correct screenshot mapping to be recognized")
	}
	if crmClientActionsContainFields(actions, "name", "pipeline", "amount", "pipeline_sales_project") {
		t.Fatal("missing active status must not be reported as present")
	}
}

func TestExtractSchemaFieldOptionsFindsNestedStatusGroups(t *testing.T) {
	attributes := map[string]any{
		"todo": map[string]any{
			"options": []any{
				map[string]any{"label_en": "Yangi lid", "value": "new"},
				map[string]any{"label_en": "Primary contact", "value": "primary"},
			},
		},
		"complete": map[string]any{
			"options": []any{
				map[string]any{"label_en": "Sotildi", "value": "won"},
			},
		},
	}

	want := map[string]bool{"Yangi lid": true, "Primary contact": true, "Sotildi": true}
	got := extractSchemaFieldOptions(attributes)
	if len(got) != len(want) {
		t.Fatalf("got %#v, want %d unique options", got, len(want))
	}
	for _, option := range got {
		if !want[option] {
			t.Fatalf("unexpected extracted option %q from %#v", option, got)
		}
	}
}

func TestValidateCRMImagesAcceptsInlineScreenshot(t *testing.T) {
	if err := validateCRMImages([]string{"data:image/png;base64,eA=="}); err != nil {
		t.Fatalf("valid screenshot rejected: %v", err)
	}
}

func TestValidateCRMImagesRejectsRemoteURL(t *testing.T) {
	if err := validateCRMImages([]string{"https://example.com/screenshot.png"}); err == nil {
		t.Fatal("remote screenshot URL must be rejected")
	}
}

func TestNormalizePreferenceFields(t *testing.T) {
	got := normalizePreferenceFields([]string{"source", "source", "bad field", "budget"})
	want := []string{"source", "budget"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestGetAiChatUserIDUsesCRMContextWithoutWritingError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("user_id", "crm-user-id")

	userID, err := (&HandlerV1{}).getAiChatUserID(context)
	if err != nil {
		t.Fatalf("getAiChatUserID returned an error: %v", err)
	}
	if userID != "crm-user-id" {
		t.Fatalf("got user ID %q, want %q", userID, "crm-user-id")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("identity lookup unexpectedly wrote an HTTP response: %s", recorder.Body.String())
	}
}

func TestGetAiChatUserIDUsesClientAuthFallback(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Set("Auth", &auth.V2HasAccessUserRes{UserIdAuth: "auth-user-id"})

	userID, err := (&HandlerV1{}).getAiChatUserID(context)
	if err != nil {
		t.Fatalf("getAiChatUserID returned an error: %v", err)
	}
	if userID != "auth-user-id" {
		t.Fatalf("got user ID %q, want %q", userID, "auth-user-id")
	}
	if recorder.Body.Len() != 0 {
		t.Fatalf("identity lookup unexpectedly wrote an HTTP response: %s", recorder.Body.String())
	}
}

package v1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

var crmSpreadsheetRowsPattern = regexp.MustCompile(`(?m)^rows=(\[[^\r\n]*\])\s*$`)

func buildCommonCRMSpreadsheetBatchAction(
	req models.CRMAssistantRequest,
	schema []models.TableSchema,
	resourceEnvID string,
) (*crmAssistantResult, bool) {
	message := strings.TrimSpace(req.Message)
	marker := strings.Index(strings.ToLower(message), "spreadsheet_import_data")
	if marker < 0 {
		return nil, false
	}
	nameMatch := quotedCRMPipelineName.FindStringSubmatch(message[:marker])
	rowsMatch := crmSpreadsheetRowsPattern.FindStringSubmatch(message)
	if len(nameMatch) < 2 || len(rowsMatch) < 2 {
		return &crmAssistantResult{reply: "Excel import uchun yangi pipeline nomi va jadval qatorlari kerak."}, true
	}

	var rows []map[string]any
	if err := json.Unmarshal([]byte(rowsMatch[1]), &rows); err != nil || len(rows) == 0 {
		return &crmAssistantResult{reply: "Excel qatorlarini o‘qib bo‘lmadi. Faylni qayta biriktiring."}, true
	}
	if len(rows) > 50 {
		return &crmAssistantResult{reply: "Bir importda 50 tagacha qator qo‘shish mumkin."}, true
	}
	if requested := requestedCRMRecordCreateCount(message); requested > 0 && requested != len(rows) {
		return &crmAssistantResult{reply: fmt.Sprintf("Excel row_count=%d, lekin %d ta qator o‘qildi. Faylni qayta biriktiring.", requested, len(rows))}, true
	}

	dealTable, ok := findCRMSchemaTable(schema, "deals")
	if !ok {
		return &crmAssistantResult{reply: "Deals jadvali schema ichida topilmadi."}, true
	}
	fieldByName := make(map[string]string, len(dealTable.Fields)*2)
	for _, field := range dealTable.Fields {
		fieldByName[normalizeCRMSpreadsheetHeader(field.Slug)] = field.Slug
		fieldByName[normalizeCRMSpreadsheetHeader(field.Label)] = field.Slug
	}
	aliases := map[string][]string{
		"name":     {"name", "dealname", "leadname", "title", "nomi", "название"},
		"amount":   {"amount", "budget", "sum", "summ", "summa", "сумма", "qiymat"},
		"stage":    {"stage", "status", "bosqich", "этап", "статус"},
		"source":   {"source", "leadsource", "manba", "источник"},
		"pipeline": {"pipeline", "voronka", "воронка"},
	}
	for slug, names := range aliases {
		if _, exists := fieldByName[normalizeCRMSpreadsheetHeader(slug)]; !exists {
			continue
		}
		for _, name := range names {
			fieldByName[normalizeCRMSpreadsheetHeader(name)] = slug
		}
	}

	pipelineName := strings.TrimSpace(nameMatch[1])
	mapped := make([]map[string]any, 0, len(rows))
	for index, row := range rows {
		record := make(map[string]any, len(row)+1)
		for header, value := range row {
			if value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
				continue
			}
			slug := fieldByName[normalizeCRMSpreadsheetHeader(header)]
			if slug == "" {
				return &crmAssistantResult{reply: fmt.Sprintf("Excel ustuni %q deals schema maydoniga mos kelmadi.", header)}, true
			}
			record[slug] = value
		}
		if _, exists := record["name"]; !exists {
			return &crmAssistantResult{reply: fmt.Sprintf("Excelning %d-qatorida deal nomi yo‘q.", index+1)}, true
		}
		record["pipeline"] = pipelineName
		mapped = append(mapped, record)
	}

	stages := parseCRMSpreadsheetStages(message[:marker])
	pending, err := newCRMBatchPendingAction(message, &models.CRMBatchAction{
		PipelineAction: &models.CRMPipelineAction{
			Operation: "create_pipeline", PipelineName: pipelineName, Stages: stages,
		},
		RecordAction: &models.CRMRecordAction{
			Operation: "create", Table: "deals", Records: mapped,
		},
	}, schema, resourceEnvID)
	if err != nil {
		return &crmAssistantResult{reply: fmt.Sprintf("Excel import tayyorlanmadi: %v", err)}, true
	}
	return &crmAssistantResult{reply: pending.Description, pendingAction: pending}, true
}

func normalizeCRMSpreadsheetHeader(value string) string {
	return strings.Map(func(char rune) rune {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			return unicode.ToLower(char)
		}
		return -1
	}, strings.TrimSpace(value))
}

func parseCRMSpreadsheetStages(prompt string) []models.CRMPipelineStageInput {
	lower := strings.ToLower(prompt)
	start := strings.Index(lower, "stages")
	if start < 0 {
		start = strings.Index(lower, "bosqich")
	}
	if start < 0 {
		return nil
	}
	text := strings.SplitN(prompt[start:], "\n", 2)[0]
	if colon := strings.Index(text, ":"); colon >= 0 {
		text = text[colon+1:]
	} else if space := strings.Index(text, " "); space >= 0 {
		text = text[space+1:]
	}
	parts := strings.FieldsFunc(text, func(char rune) bool {
		return char == ',' || char == '\n' || char == ';' || char == '•'
	})
	stages := make([]models.CRMPipelineStageInput, 0, len(parts))
	for _, part := range parts {
		name := strings.Trim(strings.TrimSpace(part), "-*–—")
		if strings.EqualFold(name, "and") || name == "" {
			continue
		}
		if strings.HasPrefix(strings.ToLower(name), "and ") {
			name = strings.TrimSpace(name[4:])
		}
		if strings.Contains(strings.ToLower(name), "import") {
			name = strings.TrimSpace(strings.SplitN(name, " and import", 2)[0])
		}
		if name != "" {
			stages = append(stages, models.CRMPipelineStageInput{Name: name})
		}
	}
	return stages
}

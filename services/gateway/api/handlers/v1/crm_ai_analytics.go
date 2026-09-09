package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

type crmAnalyticsKind string

const (
	crmAnalyticsLeadCount    crmAnalyticsKind = "lead_count"
	crmAnalyticsLeadStatus   crmAnalyticsKind = "lead_status"
	crmAnalyticsLeadSource   crmAnalyticsKind = "lead_source"
	crmAnalyticsLeadPipeline crmAnalyticsKind = "lead_pipeline"
	crmAnalyticsLeadPhones   crmAnalyticsKind = "lead_phones"
)

type crmAnalyticsPlan struct {
	kind              crmAnalyticsKind
	sql               string
	params            []any
	periodLabel       string
	language          string
	includePercentage bool
}

func (p *ChatProcessor) runCommonCRMAnalytics(
	ctx context.Context,
	req models.CRMAssistantRequest,
	schema []models.TableSchema,
) (*crmAssistantResult, bool, error) {
	plan, ok := buildCommonCRMAnalyticsPlan(req, schema)
	if !ok {
		return nil, false, nil
	}

	data, err := p.executeSQLQuery(ctx, plan.sql, plan.params, p.resourceEnvId)
	if err != nil {
		// Older/custom deal tables may not contain the conventional system
		// columns. Fall back to the schema-aware agent instead of failing the
		// whole request when the optimized query is not applicable.
		return nil, false, nil
	}
	reply, err := formatCommonCRMAnalyticsReply(plan, data)
	if err != nil {
		return nil, true, fmt.Errorf("format common CRM analytics: %w", err)
	}
	return &crmAssistantResult{reply: reply}, true, nil
}

func buildCommonCRMAnalyticsPlan(
	req models.CRMAssistantRequest,
	schema []models.TableSchema,
) (crmAnalyticsPlan, bool) {
	current := strings.ToLower(strings.TrimSpace(req.Message))
	if crmRequestLooksLikeFieldSettings(current) || crmRequestLooksLikeMutation(current) || !requestHasDealContext(req) {
		return crmAnalyticsPlan{}, false
	}

	if len(schema) > 0 {
		if _, ok := findCRMSchemaTable(schema, "deals"); !ok {
			return crmAnalyticsPlan{}, false
		}
	}

	start, end, periodLabel, language, ok := resolveCRMRelativePeriod(req)
	if !ok {
		return crmAnalyticsPlan{}, false
	}
	includePercentage := containsAnyFold(current,
		"protsent", "prosent", "foiz", "ulush", "percent", "percentage", "%", "процент", "доля",
	)

	kind := crmAnalyticsKind("")
	switch {
	case containsAnyFold(current, "nomer", "raqam", "telefon", "tel", "aloqa", "kontakt", "contact", "phone", "mobile", "number", "номер", "телефон", "контакт"):
		kind = crmAnalyticsLeadPhones
	case containsAnyFold(current, "source", "manba", "источник"):
		kind = crmAnalyticsLeadSource
	case containsAnyFold(current, "pipeline", "voronka", "воронк"):
		kind = crmAnalyticsLeadPipeline
	case containsAnyFold(current, "status", "stage", "bosqich", "статус", "этап"):
		kind = crmAnalyticsLeadStatus
	case containsAnyFold(current, "nechta", "neshta", "qancha", "how many", "count", "сколько", "soni") &&
		containsAnyFold(current, "kel", "kegan", "tush", "kir", "qo‘shil", "qo'shil", "qoshil", "yangi", "came", "arriv", "entered", "added", "new", "приш", "поступ", "добав", "нов"):
		kind = crmAnalyticsLeadCount
	default:
		return crmAnalyticsPlan{}, false
	}

	where := `"created_at" >= $1::timestamp AND "created_at" < $2::timestamp`
	where += ` AND "deleted_at" IS NULL`
	params := []any{
		start.Format("2006-01-02 15:04:05"),
		end.Format("2006-01-02 15:04:05"),
	}

	if kind == crmAnalyticsLeadCount {
		return crmAnalyticsPlan{
			kind:              kind,
			sql:               `SELECT COUNT(*) AS count FROM "deals" WHERE ` + where,
			params:            params,
			periodLabel:       periodLabel,
			language:          language,
			includePercentage: includePercentage,
		}, true
	}
	if kind == crmAnalyticsLeadPhones {
		contactIDExpression := `CASE WHEN jsonb_typeof(to_jsonb(d."contacts_id")) = 'array' THEN to_jsonb(d."contacts_id")->>0 ELSE TRIM(BOTH '"' FROM to_jsonb(d."contacts_id")::text) END`
		return crmAnalyticsPlan{
			kind: kind,
			sql: `SELECT COALESCE(NULLIF(d."name", ''), 'Nomsiz lid') AS lead_name,
NULLIF(TRIM(BOTH '"' FROM to_jsonb(c."phone")::text), '') AS phone
FROM "deals" d
LEFT JOIN "contacts" c ON c."guid"::text = ` + contactIDExpression + `
WHERE d."created_at" >= $1::timestamp AND d."created_at" < $2::timestamp
  AND d."deleted_at" IS NULL
ORDER BY d."created_at" DESC
LIMIT 50`,
			params:            params,
			periodLabel:       periodLabel,
			language:          language,
			includePercentage: includePercentage,
		}, true
	}

	dimension := "stage"
	missingLabel := "Statussiz"
	if kind == crmAnalyticsLeadSource {
		dimension = "source"
		missingLabel = "Source belgilanmagan"
	} else if kind == crmAnalyticsLeadPipeline {
		dimension = "pipeline"
		missingLabel = "Pipeline belgilanmagan"
	}
	// CRM select fields are commonly Postgres text[] but can be scalar in older
	// projects. Converting to jsonb lets one expression support both shapes.
	dimensionExpression := fmt.Sprintf(
		`COALESCE(NULLIF(CASE WHEN jsonb_typeof(to_jsonb("%s")) = 'array' THEN to_jsonb("%s")->>0 ELSE TRIM(BOTH '"' FROM to_jsonb("%s")::text) END, ''), '%s')`,
		dimension,
		dimension,
		dimension,
		missingLabel,
	)
	return crmAnalyticsPlan{
		kind: kind,
		sql: `SELECT ` + dimensionExpression + ` AS dimension, COUNT(*) AS count
FROM "deals"
WHERE ` + where + `
GROUP BY 1
ORDER BY 2 DESC`,
		params:            params,
		periodLabel:       periodLabel,
		language:          language,
		includePercentage: includePercentage,
	}, true
}

func requestHasDealContext(req models.CRMAssistantRequest) bool {
	if containsAnyFold(strings.ToLower(req.Message), "lid", "lead", "лид", "deal", "сделк", "mijoz", "client", "klient", "клиент") {
		return true
	}
	if strings.EqualFold(strings.TrimSpace(req.PageContext.Table), "deals") {
		return true
	}
	for index := len(req.History) - 1; index >= 0; index-- {
		if strings.EqualFold(strings.TrimSpace(req.History[index].Role), "user") &&
			containsAnyFold(strings.ToLower(req.History[index].Content), "lid", "lead", "лид", "deal", "сделк", "mijoz", "client", "klient", "клиент") {
			return true
		}
	}
	return false
}

func resolveCRMRelativePeriod(req models.CRMAssistantRequest) (time.Time, time.Time, string, string, bool) {
	texts := []string{strings.ToLower(req.Message)}
	for index := len(req.History) - 1; index >= 0; index-- {
		if req.History[index].Role == "user" {
			texts = append(texts, strings.ToLower(req.History[index].Content))
		}
	}

	language := detectCRMRequestLanguage(strings.ToLower(req.Message))
	now, err := time.Parse(time.RFC3339, strings.TrimSpace(req.PageContext.Now))
	if err != nil {
		return time.Time{}, time.Time{}, "", "", false
	}
	location := now.Location()
	if requestedLocation, locationErr := time.LoadLocation(strings.TrimSpace(req.PageContext.Timezone)); locationErr == nil {
		location = requestedLocation
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)

	labels := map[string]map[string]string{
		"uz": {
			"today": "Bugun", "yesterday": "Kecha",
			"this_week": "Bu hafta", "last_week": "O‘tgan hafta",
			"this_month": "Bu oy", "last_month": "O‘tgan oy",
		},
		"ru": {
			"today": "Сегодня", "yesterday": "Вчера",
			"this_week": "На этой неделе", "last_week": "На прошлой неделе",
			"this_month": "В этом месяце", "last_month": "В прошлом месяце",
		},
		"en": {
			"today": "Today", "yesterday": "Yesterday",
			"this_week": "This week", "last_week": "Last week",
			"this_month": "This month", "last_month": "Last month",
		},
	}

	// The current message is checked first. History is consulted only for real
	// follow-ups that omit a period, so "bu hafta" can never inherit "kecha".
	for _, text := range texts {
		period := ""
		var start, end time.Time
		switch {
		case containsAnyFold(text, "o‘tgan hafta", "o'tgan hafta", "o’tgan hafta", "otgan hafta", "o‘tkan hafta", "o'tkan hafta", "o’tkan hafta", "otkan hafta", "utgan hafta", "avvalgi hafta", "oldingi hafta", "last week", "previous week", "прошлой недел", "прошлую недел", "предыдущей недел", "неделю назад"):
			period = "last_week"
			start = startOfCRMWeek(today).AddDate(0, 0, -7)
			end = start.AddDate(0, 0, 7)
		case containsAnyFold(text, "bu hafta", "shu hafta", "ushbu hafta", "haftada", "this week", "current week", "этой недел", "эту недел", "текущей недел"):
			period = "this_week"
			start = startOfCRMWeek(today)
			end = start.AddDate(0, 0, 7)
		case containsAnyFold(text, "o‘tgan oy", "o'tgan oy", "o’tgan oy", "otgan oy", "o‘tkan oy", "o'tkan oy", "o’tkan oy", "otkan oy", "utgan oy", "avvalgi oy", "oldingi oy", "last month", "previous month", "прошлом месяц", "прошлый месяц", "предыдущем месяц"):
			period = "last_month"
			end = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, location)
			start = end.AddDate(0, -1, 0)
		case containsAnyFold(text, "bu oy", "shu oy", "ushbu oy", "oyda", "this month", "current month", "этом месяц", "этот месяц", "текущем месяц"):
			period = "this_month"
			start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, location)
			end = start.AddDate(0, 1, 0)
		case containsAnyFold(text, "kecha", "kechagi", "kechigi", "yesterday", "вчера"):
			period = "yesterday"
			start = today.AddDate(0, 0, -1)
			end = today
		case containsAnyFold(text, "bugun", "today", "сегодня"):
			period = "today"
			start = today
			end = today.AddDate(0, 0, 1)
		}
		if period != "" {
			return start, end, labels[language][period], language, true
		}
		if start, end, label, found := resolveCRMNamedMonth(text, localNow, language); found {
			return start, end, label, language, true
		}
	}
	return time.Time{}, time.Time{}, "", "", false
}

func startOfCRMWeek(day time.Time) time.Time {
	daysSinceMonday := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -daysSinceMonday)
}

func resolveCRMNamedMonth(text string, localNow time.Time, language string) (time.Time, time.Time, string, bool) {
	month, ok := findCRMMonth(text)
	if !ok {
		return time.Time{}, time.Time{}, "", false
	}
	year := findCRMYear(text)
	if year == 0 {
		year = localNow.Year()
		if month > localNow.Month() {
			year--
		}
	}
	start := time.Date(year, month, 1, 0, 0, 0, 0, localNow.Location())
	end := start.AddDate(0, 1, 0)

	uzbekMonths := [...]string{"", "Yanvar", "Fevral", "Mart", "Aprel", "May", "Iyun", "Iyul", "Avgust", "Sentabr", "Oktabr", "Noyabr", "Dekabr"}
	englishMonths := [...]string{"", "January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"}
	russianMonths := [...]string{"", "январе", "феврале", "марте", "апреле", "мае", "июне", "июле", "августе", "сентябре", "октябре", "ноябре", "декабре"}
	label := uzbekMonths[month] + " oyida"
	if language == "en" {
		label = "In " + englishMonths[month]
	} else if language == "ru" {
		label = "В " + russianMonths[month]
	}
	if findCRMYear(text) != 0 {
		label += fmt.Sprintf(" %d", year)
	}
	return start, end, label, true
}

func findCRMMonth(text string) (time.Month, bool) {
	aliases := []struct {
		month    time.Month
		prefixes []string
	}{
		{time.January, []string{"yanvar", "january", "январ"}},
		{time.February, []string{"fevral", "february", "феврал"}},
		{time.March, []string{"mart", "march", "март"}},
		{time.April, []string{"aprel", "april", "апрел"}},
		{time.May, []string{"may", "май"}},
		{time.June, []string{"iyun", "june", "июн"}},
		{time.July, []string{"iyul", "july", "июл"}},
		{time.August, []string{"avgust", "august", "август"}},
		{time.September, []string{"sentabr", "september", "сентябр"}},
		{time.October, []string{"oktabr", "october", "октябр"}},
		{time.November, []string{"noyabr", "november", "ноябр"}},
		{time.December, []string{"dekabr", "december", "декабр"}},
	}
	words := strings.Fields(strings.NewReplacer(
		".", " ", ",", " ", "?", " ", "!", " ", ":", " ", ";", " ",
		"(", " ", ")", " ", "[", " ", "]", " ", "{", " ", "}", " ",
	).Replace(strings.ToLower(text)))
	for _, word := range words {
		for _, candidate := range aliases {
			for _, prefix := range candidate.prefixes {
				if strings.HasPrefix(word, prefix) {
					// Avoid treating Uzbek words such as "maydon" as the month May.
					if prefix == "may" && word != "may" && !strings.HasPrefix(word, "mayda") && !strings.HasPrefix(word, "mayning") {
						continue
					}
					return candidate.month, true
				}
			}
		}
	}
	return 0, false
}

func findCRMYear(text string) int {
	for index := 0; index+4 <= len(text); index++ {
		candidate := text[index : index+4]
		year, err := strconv.Atoi(candidate)
		if err == nil && year >= 2000 && year <= 2100 {
			return year
		}
	}
	return 0
}

func formatCommonCRMAnalyticsReply(plan crmAnalyticsPlan, data any) (string, error) {
	rows, err := crmAnalyticsRows(data)
	if err != nil {
		return "", err
	}

	if plan.kind == crmAnalyticsLeadCount {
		if len(rows) == 0 {
			return "", fmt.Errorf("count query returned no rows")
		}
		count, ok := crmAnalyticsInteger(rows[0]["count"])
		if !ok {
			return "", fmt.Errorf("count query returned an invalid count")
		}
		switch plan.language {
		case "ru":
			return fmt.Sprintf("%s пришло %d лидов.", plan.periodLabel, count), nil
		case "en":
			noun := "leads"
			if count == 1 {
				noun = "lead"
			}
			return fmt.Sprintf("%s, %d %s came in.", plan.periodLabel, count, noun), nil
		default:
			return fmt.Sprintf("%s %d ta lid keldi.", plan.periodLabel, count), nil
		}
	}
	if plan.kind == crmAnalyticsLeadPhones {
		lines := make([]string, 0, len(rows))
		for _, row := range rows {
			name := strings.TrimSpace(fmt.Sprintf("%v", row["lead_name"]))
			phone := strings.TrimSpace(fmt.Sprintf("%v", row["phone"]))
			if name == "" || name == "<nil>" {
				name = "Nomsiz lid"
			}
			if phone == "" || phone == "<nil>" {
				switch plan.language {
				case "ru":
					phone = "номер не указан"
				case "en":
					phone = "phone not provided"
				default:
					phone = "telefon raqami ko‘rsatilmagan"
				}
			}
			lines = append(lines, fmt.Sprintf("- %s — %s", name, phone))
		}

		header := ""
		switch plan.language {
		case "ru":
			header = plan.periodLabel + " пришли лиды со следующими номерами:"
		case "en":
			header = plan.periodLabel + ", the incoming leads have these phone numbers:"
		default:
			header = plan.periodLabel + " kelgan lidlarning telefon raqamlari:"
		}
		if len(lines) == 0 {
			switch plan.language {
			case "ru":
				return plan.periodLabel + " лидов не было.", nil
			case "en":
				return plan.periodLabel + ", no leads came in.", nil
			default:
				return plan.periodLabel + " lid kelmagan.", nil
			}
		}
		return header + "\n" + strings.Join(lines, "\n"), nil
	}

	type dimensionCount struct {
		dimension string
		count     int64
	}
	dimensionCounts := make([]dimensionCount, 0, len(rows))
	total := int64(0)
	for _, row := range rows {
		count, ok := crmAnalyticsInteger(row["count"])
		if !ok {
			return "", fmt.Errorf("status query returned an invalid count")
		}
		dimension := strings.TrimSpace(fmt.Sprintf("%v", row["dimension"]))
		if dimension == "" || dimension == "<nil>" {
			dimension = "Belgilanmagan"
		}
		total += count
		dimensionCounts = append(dimensionCounts, dimensionCount{dimension: dimension, count: count})
	}

	header := ""
	dimensionName := "status"
	if plan.kind == crmAnalyticsLeadSource {
		dimensionName = "source"
	} else if plan.kind == crmAnalyticsLeadPipeline {
		dimensionName = "pipeline"
	}
	switch plan.language {
	case "ru":
		russianDimension := map[string]string{"status": "по текущим статусам", "source": "по источникам", "pipeline": "по воронкам"}[dimensionName]
		header = fmt.Sprintf("%s пришло %d лидов, %s:", plan.periodLabel, total, russianDimension)
	case "en":
		noun := "leads"
		if total == 1 {
			noun = "lead"
		}
		englishDimension := map[string]string{"status": "current status", "source": "source", "pipeline": "pipeline"}[dimensionName]
		header = fmt.Sprintf("%s, %d %s came in, grouped by %s:", plan.periodLabel, total, noun, englishDimension)
	default:
		if dimensionName == "status" {
			header = fmt.Sprintf("%s kelgan %d ta lid hozir quyidagi statuslarda:", plan.periodLabel, total)
		} else {
			header = fmt.Sprintf("%s kelgan %d ta lid %s bo‘yicha:", plan.periodLabel, total, dimensionName)
		}
	}
	if len(dimensionCounts) == 0 {
		return header + "\n- Ma’lumot yo‘q", nil
	}
	if plan.includePercentage {
		dimensionHeader := map[string]string{"status": "Status", "source": "Source", "pipeline": "Pipeline"}[dimensionName]
		valueHeader := "Lidlar"
		shareHeader := "Ulush"
		totalLine := fmt.Sprintf("Jami: %d ta lid.", total)
		if plan.language == "ru" {
			dimensionHeader = map[string]string{"status": "Статус", "source": "Источник", "pipeline": "Воронка"}[dimensionName]
			valueHeader = "Лиды"
			shareHeader = "Доля"
			totalLine = fmt.Sprintf("Итого: %d лидов.", total)
		} else if plan.language == "en" {
			dimensionHeader = map[string]string{"status": "Status", "source": "Source", "pipeline": "Pipeline"}[dimensionName]
			valueHeader = "Leads"
			shareHeader = "Share"
			totalLine = fmt.Sprintf("Total: %d leads.", total)
		}
		tableLines := []string{
			fmt.Sprintf("| %s | %s | %s |", dimensionHeader, valueHeader, shareHeader),
			"| --- | ---: | ---: |",
		}
		for _, item := range dimensionCounts {
			share := float64(0)
			if total > 0 {
				share = float64(item.count) * 100 / float64(total)
			}
			safeDimension := strings.ReplaceAll(item.dimension, "|", "¦")
			tableLines = append(tableLines, fmt.Sprintf("| %s | %d | %.1f%% |", safeDimension, item.count, share))
		}
		return header + "\n\n" + strings.Join(tableLines, "\n") + "\n\n" + totalLine, nil
	}
	lines := make([]string, 0, len(dimensionCounts))
	for _, item := range dimensionCounts {
		lines = append(lines, fmt.Sprintf("- %s — %d", item.dimension, item.count))
	}
	return header + "\n" + strings.Join(lines, "\n"), nil
}

func crmAnalyticsRows(data any) ([]map[string]any, error) {
	result, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("unexpected analytics result type")
	}
	rows, ok := result["rows"].([]map[string]any)
	if !ok {
		return nil, fmt.Errorf("analytics result has no rows")
	}
	return rows, nil
}

func crmAnalyticsInteger(value any) (int64, bool) {
	switch typed := value.(type) {
	case int:
		return int64(typed), true
	case int32:
		return int64(typed), true
	case int64:
		return typed, true
	case float32:
		return int64(typed), true
	case float64:
		return int64(typed), true
	case json.Number:
		parsed, err := typed.Int64()
		return parsed, err == nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		return parsed, err == nil
	default:
		return 0, false
	}
}

func findCRMSchemaTable(schema []models.TableSchema, slug string) (models.TableSchema, bool) {
	for _, table := range schema {
		if table.Slug == slug {
			return table, true
		}
	}
	return models.TableSchema{}, false
}

func containsAnyFold(value string, candidates ...string) bool {
	value = strings.ToLower(value)
	for _, candidate := range candidates {
		if strings.Contains(value, strings.ToLower(candidate)) {
			return true
		}
	}
	return false
}

func detectCRMRequestLanguage(value string) string {
	if containsAnyFold(value,
		"bugun", "kecha", "nechta", "neshta", "qancha", "qaysi", "hafta", " oy", "oyda",
		"qiber", "ber", "o‘zgartir", "o'zgartir", "ozgartir", "ko‘rsat", "ko'rsat", "korsat", "yashir", "kel", "tush",
	) {
		return "uz"
	}
	if containsAnyFold(value, "вчера", "сегодня", "лид", "статус", "этап", "сколько", "приш", "измени", "сделк", "покажи", "скрой") {
		return "ru"
	}
	if containsAnyFold(value,
		"yesterday", "today", "lead", "how many", "came", "arrived", "update", "change", "delete", "create",
		"show", "hide", "first", "second", " the ", "please", "cancel",
	) {
		return "en"
	}
	return "uz"
}

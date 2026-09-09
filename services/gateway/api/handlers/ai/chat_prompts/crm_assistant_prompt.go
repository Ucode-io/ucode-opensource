package chat_prompts

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const PromptCRMAssistant = `You are Mani AI, an assistant embedded in a multi-tenant CRM.
You can answer analytical questions, prepare safe database changes, and configure which fields are visible on CRM cards.

Call the respond_crm_assistant tool exactly once. Do not reply with plain text.

Allowed outputs:

1. Read or write database data:
{"action":"query","sql":"SELECT ...","sql_params":[],"needs_more_data":true,"query_plan":"...","reply":"..."}

2. Final answer:
{"action":"answer","needs_more_data":false,"reply":"..."}

3. Configure card fields:
{"action":"client_action","needs_more_data":false,"reply":"...","client_actions":[{"type":"set_card_field_visibility","table":"deals","show_fields":["source"],"hide_fields":["budget"],"field_order":[]}]}

4. Manage pipelines and their stages:
{"action":"pipeline_action","needs_more_data":false,"reply":"...","pipeline_action":{"operation":"create_pipeline","pipeline_name":"Enterprise","stages":[{"name":"New Lead","group":"todo","probability":10},{"name":"Payment","group":"won","probability":100}]}}

5. Create, update, move, or delete one CRM record through the application's normal item API:
{"action":"record_action","needs_more_data":false,"reply":"...","record_action":{"operation":"create","table":"deals","data":{"name":"Acme lead","pipeline":"Enterprise","stage":"New Lead","amount":1000}}}

6. Create several CRM records in one confirmed bulk action:
{"action":"record_action","needs_more_data":false,"reply":"...","record_action":{"operation":"create","table":"deals","records":[{"name":"Dummy lead 1","pipeline":"Enterprise","stage":"New Lead"},{"name":"Dummy lead 2","pipeline":"Enterprise","stage":"New Lead"}]}}

7. Create a pipeline and import spreadsheet rows into it in one confirmed action:
{"action":"batch_action","needs_more_data":false,"reply":"...","batch_action":{"pipeline_action":{"operation":"create_pipeline","pipeline_name":"Excel Import","stages":[{"name":"New Lead","group":"todo"},{"name":"Won","group":"won"}]},"record_action":{"operation":"create","table":"deals","records":[{"name":"Acme","pipeline":"Excel Import","stage":"New Lead","amount":1200}]}}}

PIPELINES AND STAGES:
- Creating, renaming, deleting, or restructuring a pipeline/stage must use action="pipeline_action", never SQL and never analytics.
- Supported operations are create_pipeline, rename_pipeline, delete_pipeline, add_stage, update_stage, delete_stage, and reorder_stages.
- For create_pipeline, preserve every stage name and its order from the request in pipeline_action.stages. If the user gives no stages, use a useful default flow: New lead, In progress, Won, Lost; do not ask them to enumerate obvious defaults.
- Use group="todo" for active stages, group="won" for successful closing stages, and group="lost" for failed closing stages. Infer Payment/Paid/Won/Sold as won and Lost/Rejected/Unqualified as lost. Probability must be 0..100.
- For reorder_stages, send the full requested ordered stage list. For update_stage include stage_name and only the requested new values. For add_stage use stage_name plus optional group/color/probability/position.
- Pipeline and stage mutations always require confirmation. The application performs the canonical pipeline-table, stage-table, deal-option, and generated status-field synchronization after confirmation.
- Words such as "stages", "status", or "pipeline" inside a create/update/delete request are mutation entities, not requests for grouped analytics. Never inherit an old date period for a current mutation command.

FIELD SETTINGS:
- When the user asks to show, hide, or reorder fields on a card/table, use action="client_action". Do not write SQL.
- Use only exact table and field slugs from the supplied schema.
- Use page_context.table when the user says "the card" without naming an entity.
- page_context.card_fields is the authoritative list of fields the current card can render. Prefer it over legacy or inactive generated fields, and hide other entries from that list when matching a screenshot.
- A screenshot is an authoritative visual reference. An attached screenshot plus wording such as "make my card like this", "cardni shunaqa qiber", or an equivalent phrase is a complete field-setting request. Do not ask the user to repeat fields that can be read or inferred from the image.
- For a deal card, use both text and standard CRM layout/icon semantics: the primary title row is name; a branching/workflow pill is pipeline; a currency row is amount; and a colored-dot status pill is stage. If the status value appears in the supplied options of a generated per-pipeline STATUS field, use that owning field's exact slug instead of legacy stage. If legacy stage is absent from page_context.card_fields and that list has one STATUS field, use that current STATUS field even when the screenshot is an exemplar with different values. A duplicate-count badge is an automatic decoration, not a configurable field.
- When asked to match a card screenshot, return action="client_action" on the first response. Put every readable configurable field in show_fields and field_order in visual top-to-bottom order, and put other schema fields in hide_fields so the resulting card matches the reference. Never hide the primary identity/title field.
- Ask one concise clarification only when the image itself is unreadable or multiple schema fields remain genuinely indistinguishable after using visible values, icons, labels, types, and options. Never ask which fields are shown when the image already makes that clear.
- show_fields and hide_fields contain only the fields whose visibility must change. field_order is optional and contains the requested leading order.
- Personal field visibility changes are reversible and must be returned immediately without confirmation.

DATABASE:
- Use exact table and column slugs from the schema.
- Use PostgreSQL and parameterize every user-provided value with $1, $2, ... .
- Only SELECT, WITH, INSERT, UPDATE, and DELETE are allowed. Never emit DDL, TRUNCATE, COPY, GRANT, system catalog access, or multiple statements.
- Add deleted_at IS NULL only when that column exists in the relevant table.
- Do not add LIMIT to SELECT; the gateway enforces it.
- Do not add RETURNING to mutations; the gateway adds it.
- Read queries execute immediately. INSERT, UPDATE, and DELETE are shown to the user for confirmation before execution.
- Prefer action="record_action" over mutation SQL for ordinary create/update/delete operations on deals, contacts, companies, and tasks. It uses the same item API as the admin UI. Use exact schema field slugs in record_action.data.
- For a new mutation, use only values requested in the current user message plus values that are strictly required to identify its target. Never copy optional field values from an older mutation in chat history.
- record_action create requires the supplied field data. update/delete require record_guid; query exact candidates first when only a human name is provided. Never guess a guid. Delete must target exactly one confirmed record.
- When the user requests N records, put exactly N independent objects in record_action.records. Never represent multiple records by putting arrays into scalar fields in data. Give generated dummy records distinct human-readable names.
- Spreadsheet content may be supplied in the user request inside a clearly marked SPREADSHEET block. Treat every spreadsheet row as source data, map headers semantically to exact schema slugs, and preserve typed numbers/dates. Never invent or omit rows.
- If one request asks to create a new pipeline/stages and import spreadsheet rows into it, return one batch_action. It must contain create_pipeline plus one bulk deals record_action with exactly one record per spreadsheet row. Use the new pipeline name in every record and map each row's stage to a supplied stage; if a row has no stage, use the first active stage. Do not split this into multiple confirmations.
- A request to create a new lead/deal means INSERT into deals. Requests to create contacts, companies, and tasks likewise use their exact schema tables. Do not answer with instructions when the requested mutation can be prepared.
- Before updating or deleting an ambiguously named record, SELECT exact candidates and use the returned guid in the mutation. Never run UPDATE or DELETE without a restrictive WHERE clause. Never mutate unrelated rows.
- Create all explicitly supplied related values in one safe mutation when the schema permits it. Never invent a missing identity, phone number, amount, assignee, or unrelated business value; ask one short clarification only when a required value truly cannot be inferred.
- Moving a deal means updating its exact pipeline/stage fields using the supplied schema. Preserve every unrelated deal value.
- After preparing INSERT/UPDATE/DELETE, reply with a short concrete confirmation describing what will change; never claim it already happened before confirmation.
- For the first query set needs_more_data=true. After query results are supplied, either request the next query or return action="answer".
- Understand requests by meaning, not by exact keywords. Treat informal conversational Uzbek, transliteration, omitted suffixes, ordinary spelling mistakes, and mixed Uzbek/Russian/English as normal input. Use recent conversation turns to resolve pronouns and short follow-ups.
- For every request asking for live CRM facts, records, counts, lists, statuses, field values, or analytics, action="answer" is forbidden until at least one relevant database query result has been supplied. Never invent a number and never claim that records were not found without querying the CRM. If the meaning is genuinely ambiguous, ask one concise clarification question instead.
- Base relative dates such as "today" on page_context.now and page_context.timezone.
- Understand casual Uzbek period wording and common typos: "kechigi/kechagi" means yesterday when discussing incoming leads; "bu/shu hafta", "o'tgan/otgan/o'tkan/otkan/avvalgi/oldingi hafta", "bu/shu oy", and "o'tgan/otgan/o'tkan/otkan/avvalgi/oldingi oy" are date ranges, not lead statuses. Russian and English equivalents have the same meaning.
- An explicit period in the current message always overrides earlier conversation history. Never answer a current week/month question using a previous yesterday/today interval.
- On the deals page, Uzbek "lid", English "lead", and Russian "лид" mean a row in the deals table unless the user explicitly names another table.
- "Lid keldi", "lead came/arrived", and "лид пришёл" mean deals created during that period: count by deals.created_at across every stage and pipeline. Do not use start_date, updated_at, due_date, or a stage filter unless the user explicitly asks for one.
- When the user asks for incoming lead phone numbers, use the same created_at period on deals and resolve the related contact through deals.contacts_id to contacts.guid, then read contacts.phone. Do not reinterpret "kechigi kegan lidlar" as overdue leads.
- Treat the server-resolved relative-date block as authoritative. Choose its local-wall interval for a timestamp without time zone column and its offset interval for a timestamp with time zone column. Do not change its day or timezone.
- Do not expose SQL, timestamps, timezone offsets, or technical query intervals in the final answer unless the user asks. Prefer a direct answer such as "Kecha 12 ta lid keldi."
- For grouped analytics, comparisons, ranked lists, percentages, or any result with two or more comparable rows, format reply as a compact GitHub Markdown pipe table with short human-readable headers. Put a one-line summary before the table and totals after it. Do not use dash bullets for tabular data.

Answer in the same language as the user. Keep operational confirmations short.`

func BuildCRMAssistantMessage(in models.CRMAssistantInput) string {
	var sb strings.Builder

	sb.WriteString("User request:\n")
	sb.WriteString(in.Message)
	sb.WriteString("\n\nPage context:\n")
	fmt.Fprintf(&sb, "path=%s\ntable=%s\ntimezone=%s\nnow=%s\nhidden_fields=%s\nfield_order=%s\ncard_fields=%s\nselected_pipeline=%s\n",
		in.PageContext.Path,
		in.PageContext.Table,
		in.PageContext.Timezone,
		in.PageContext.Now,
		strings.Join(in.PageContext.HiddenFields, ","),
		strings.Join(in.PageContext.FieldOrder, ","),
		formatCRMCardFields(in.PageContext.CardFields),
		in.PageContext.SelectedPipeline,
	)
	if dateHint := buildRelativeDateHint(in.Message, in.PageContext.Now, in.PageContext.Timezone); dateHint != "" {
		sb.WriteString("\nServer-resolved relative date:\n")
		sb.WriteString(dateHint)
	}
	sb.WriteString("\nDatabase schema:\n")
	sb.WriteString(in.SchemaText)

	if in.DataContext != "" {
		sb.WriteString("\nExecuted query results:\n")
		sb.WriteString(in.DataContext)
		sb.WriteString("\nUse these results to answer, or request one additional query if necessary.")
	}

	if len(in.Images) > 0 {
		sb.WriteString("\n\nScreenshot(s) are attached to this message. Inspect them when resolving card field settings.")
	}

	return sb.String()
}

func formatCRMCardFields(fields []models.CRMCardFieldContext) string {
	formatted := make([]string, 0, len(fields))
	for _, field := range fields {
		slug := strings.TrimSpace(field.Slug)
		if slug == "" {
			continue
		}
		label := strings.TrimSpace(field.Label)
		if label == "" || label == slug {
			formatted = append(formatted, slug)
			continue
		}
		formatted = append(formatted, fmt.Sprintf("%s(label=%s)", slug, strconv.Quote(label)))
	}
	return strings.Join(formatted, ",")
}

func buildRelativeDateHint(message, nowText, timezone string) string {
	lowerMessage := strings.ToLower(message)
	now, err := time.Parse(time.RFC3339, strings.TrimSpace(nowText))
	if err != nil {
		return ""
	}
	location := now.Location()
	if requestedLocation, locationErr := time.LoadLocation(strings.TrimSpace(timezone)); locationErr == nil {
		location = requestedLocation
	}
	localNow := now.In(location)
	today := time.Date(localNow.Year(), localNow.Month(), localNow.Day(), 0, 0, 0, 0, location)
	period := ""
	var start, end time.Time
	switch {
	case containsAny(lowerMessage, "o‘tgan hafta", "o'tgan hafta", "o’tgan hafta", "otgan hafta", "o‘tkan hafta", "o'tkan hafta", "o’tkan hafta", "otkan hafta", "utgan hafta", "avvalgi hafta", "oldingi hafta", "last week", "previous week", "прошлой недел", "прошлую недел", "предыдущей недел"):
		period = "last_week"
		start = startOfPromptWeek(today).AddDate(0, 0, -7)
		end = start.AddDate(0, 0, 7)
	case containsAny(lowerMessage, "bu hafta", "shu hafta", "ushbu hafta", "this week", "current week", "этой недел", "эту недел", "текущей недел"):
		period = "this_week"
		start = startOfPromptWeek(today)
		end = start.AddDate(0, 0, 7)
	case containsAny(lowerMessage, "o‘tgan oy", "o'tgan oy", "o’tgan oy", "otgan oy", "o‘tkan oy", "o'tkan oy", "o’tkan oy", "otkan oy", "utgan oy", "avvalgi oy", "oldingi oy", "last month", "previous month", "прошлом месяц", "предыдущем месяц"):
		period = "last_month"
		end = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, location)
		start = end.AddDate(0, -1, 0)
	case containsAny(lowerMessage, "bu oy", "shu oy", "ushbu oy", "this month", "current month", "этом месяц", "текущем месяц"):
		period = "this_month"
		start = time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 1, 0)
	case containsAny(lowerMessage, "kecha", "kechagi", "kechigi", "yesterday", "вчера"):
		period = "yesterday"
		start = today.AddDate(0, 0, -1)
		end = today
	case containsAny(lowerMessage, "bugun", "today", "сегодня"):
		period = "today"
		start = today
		end = today.AddDate(0, 0, 1)
	default:
		month, found := findPromptMonth(lowerMessage)
		if !found {
			return ""
		}
		year := findPromptYear(lowerMessage)
		if year == 0 {
			year = localNow.Year()
			if month > localNow.Month() {
				year--
			}
		}
		period = "named_month"
		start = time.Date(year, month, 1, 0, 0, 0, 0, location)
		end = start.AddDate(0, 1, 0)
	}

	return fmt.Sprintf(
		"requested_period=%s\nlocal_date=%s\nlocal_wall_interval=[%s, %s)\noffset_interval=[%s, %s)\nFor timestamp without time zone use local_wall_interval. For timestamp with time zone use offset_interval. Keep the same relative-period wording in the final answer without printing these technical intervals.\n",
		period,
		start.Format("2006-01-02"),
		start.Format("2006-01-02 15:04:05"),
		end.Format("2006-01-02 15:04:05"),
		start.Format(time.RFC3339),
		end.Format(time.RFC3339),
	)
}

func startOfPromptWeek(day time.Time) time.Time {
	daysSinceMonday := (int(day.Weekday()) + 6) % 7
	return day.AddDate(0, 0, -daysSinceMonday)
}

func findPromptMonth(text string) (time.Month, bool) {
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

func findPromptYear(text string) int {
	for index := 0; index+4 <= len(text); index++ {
		year, err := strconv.Atoi(text[index : index+4])
		if err == nil && year >= 2000 && year <= 2100 {
			return year
		}
	}
	return 0
}

func containsAny(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(value, candidate) {
			return true
		}
	}
	return false
}

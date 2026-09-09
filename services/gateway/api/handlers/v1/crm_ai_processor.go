package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/openai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const (
	maxCRMAssistantIterations    = 6
	canonicalDealScreenshotReply = "Screenshotga mosladim: kartochkada nomi, pipeline, summa va active status qoldirildi."
)

type crmAssistantResult struct {
	reply         string
	clientActions []models.CRMClientAction
	pendingAction *models.PendingAction
}

func (p *ChatProcessor) runCRMAssistantFlow(
	ctx context.Context,
	agent *openai.OpenAIAgent,
	req models.CRMAssistantRequest,
) (*crmAssistantResult, error) {
	if result, handled := buildCommonCRMPipelineAction(req, p.resourceEnvId); handled {
		return result, nil
	}

	// Common lead analytics only use conventional deal system fields. Run them
	// before the much heavier project-schema fetch so routine dashboard questions
	// stay fast even if a metadata service is temporarily slow.
	if result, handled, analyticsErr := p.runCommonCRMAnalytics(ctx, req, nil); handled {
		return result, analyticsErr
	}

	schema, err := p.getProjectSchemaCached(ctx, p.resourceEnvId)
	if err != nil {
		return nil, fmt.Errorf("load CRM schema: %w", err)
	}
	if result, handled := buildCommonCRMSpreadsheetBatchAction(req, schema, p.resourceEnvId); handled {
		return result, nil
	}
	if result, handled := buildCommonCRMFieldSettings(req, schema); handled {
		return result, nil
	}

	message := strings.TrimSpace(req.Message)
	if message == "" && len(req.Images) > 0 {
		message = "Bu screenshot asosida kartochkadagi maydonlar ko‘rinishini sozla."
	}

	history := crmAssistantHistory(req.History)
	schemaText := formatSchemaForCRM(schema)
	dataContext := ""
	hasSuccessfulRead := false
	screenshotFieldSettings := len(req.Images) > 0 && crmRequestLooksLikeFieldSettings(message)

	for iteration := 0; iteration < maxCRMAssistantIterations; iteration++ {
		if err := p.Check(); err != nil {
			return nil, err
		}
		if iteration == maxCRMAssistantIterations-1 && hasSuccessfulRead {
			if crmRequestLooksLikeMutation(message) {
				dataContext += "\n\n[SYSTEM: Final step. Use the fetched guid/data to return action=record_action, action=pipeline_action, or a safe mutation query now. Do not answer with instructions and do not request another read.]"
			} else {
				dataContext += "\n\n[SYSTEM: Final step. Return action=answer using the fetched data. Do not request another query.]"
			}
		}

		images := req.Images
		if iteration > 0 && !screenshotFieldSettings {
			images = nil
		}
		plan, err := agent.CRMQuery(ctx, models.CRMAssistantInput{
			Message:     message,
			SchemaText:  schemaText,
			DataContext: dataContext,
			Images:      images,
			History:     history,
			PageContext: req.PageContext,
		})
		if err != nil {
			return nil, err
		}

		switch plan.Action {
		case "answer", "schema":
			if screenshotFieldSettings {
				if canonical, ok := canonicalDealScreenshotCardAction(schema, req.PageContext); ok && iteration >= 2 {
					return &crmAssistantResult{
						reply:         canonicalDealScreenshotReply,
						clientActions: []models.CRMClientAction{canonical},
					}, nil
				}
				if iteration < 2 {
					dataContext = appendDataContext(
						dataContext,
						"Mandatory screenshot field action",
						"[SYSTEM: The attached screenshot and the user's first message are sufficient. Inspect the image again, map its visible configurable card rows to the supplied schema, and return action=client_action now. Do not ask the user to enumerate fields that are already visible.]",
					)
					continue
				}
			}
			if crmRequestLooksLikeMutation(message) && !crmReplyIsClarification(plan.Reply) {
				dataContext = appendDataContext(
					dataContext,
					"Mandatory CRM mutation",
					"[SYSTEM: The user asked to change CRM state. Do not answer with instructions or claim completion. Prepare the requested record_action, pipeline_action, or a safe INSERT/UPDATE/DELETE now; ask one concise clarification only if a required business value is genuinely missing.]",
				)
				continue
			}
			// The language model may understand a very informal request correctly but
			// still try to answer from conversation context. Live CRM facts must never
			// be guessed: require an actual successful database read first. A genuine
			// clarification question is allowed when the request is ambiguous.
			if crmRequestRequiresLiveLookup(req) && !hasSuccessfulRead && !crmReplyIsClarification(plan.Reply) {
				dataContext = appendDataContext(
					dataContext,
					"Mandatory live CRM lookup",
					"[SYSTEM: Do not answer from memory or infer that records are absent. Interpret the user's informal, misspelled, or mixed-language wording semantically and execute a safe query against the supplied CRM schema now.]",
				)
				continue
			}
			return &crmAssistantResult{reply: plan.Reply}, nil

		case "client_action":
			// Card visibility is the only supported client-side configuration
			// action. Never let the model reinterpret an analytics request such as
			// "show the stage counts in a table" as a card-layout change.
			if !crmRequestLooksLikeFieldSettings(message) {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Rejected unrelated client action %d", iteration+1),
					"[SYSTEM: The user did not ask to configure card fields. Do not return client_action. Answer the requested CRM question using a live database query, or prepare the requested CRM mutation.]",
				)
				continue
			}
			actions := normalizeCRMClientActions(plan.ClientActions, schema, req.PageContext.Table)
			if screenshotFieldSettings {
				if canonical, ok := canonicalDealScreenshotCardAction(schema, req.PageContext); ok {
					if crmClientActionsContainFields(actions, "name", "pipeline", "amount") || iteration >= 2 {
						return &crmAssistantResult{
							reply:         canonicalDealScreenshotReply,
							clientActions: []models.CRMClientAction{canonical},
						}, nil
					}
					dataContext = appendDataContext(
						dataContext,
						"Rejected screenshot card mapping",
						fmt.Sprintf("[SYSTEM: Reinspect the screenshot. This deal-card reference contains the standard title, pipeline, amount, and colored-dot status rows. Return action=client_action using the exact order name, pipeline, amount, %s. Do not map the branching pipeline pill to contacts_id, and do not include phone/contact unless a phone or contact row is actually visible.]", canonical.FieldOrder[len(canonical.FieldOrder)-1]),
					)
					continue
				}
			}
			if len(actions) == 0 {
				reply := strings.TrimSpace(plan.Reply)
				if reply == "" {
					reply = "Maydon nomlarini aniqlab bo‘lmadi. Qaysi maydonlarni ko‘rsatish yoki yashirish kerakligini aniqlashtiring."
				}
				return &crmAssistantResult{reply: reply}, nil
			}
			return &crmAssistantResult{reply: plan.Reply, clientActions: actions}, nil

		case "pipeline_action":
			pending, normalizeErr := newCRMPipelinePendingAction(req.Message, plan.PipelineAction, p.resourceEnvId)
			if normalizeErr != nil {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Rejected pipeline action %d", iteration+1),
					fmt.Sprintf("[SYSTEM: The pipeline action was incomplete or invalid: %v. Correct it from the user's request. Ask one concise clarification only if a required name is genuinely missing.]", normalizeErr),
				)
				continue
			}
			return &crmAssistantResult{reply: pending.Description, pendingAction: pending}, nil

		case "record_action":
			pending, normalizeErr := newCRMRecordPendingAction(req.Message, plan.RecordAction, schema, p.resourceEnvId)
			if normalizeErr != nil {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Rejected record action %d", iteration+1),
					fmt.Sprintf("[SYSTEM: The CRM record action was incomplete or invalid: %v. Correct it using the exact schema. Query an exact record guid before update/delete; ask one concise clarification only if required user data is missing.]", normalizeErr),
				)
				continue
			}
			return &crmAssistantResult{reply: pending.Description, pendingAction: pending}, nil

		case "batch_action":
			pending, normalizeErr := newCRMBatchPendingAction(req.Message, plan.BatchAction, schema, p.resourceEnvId)
			if normalizeErr != nil {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Rejected batch action %d", iteration+1),
					fmt.Sprintf("[SYSTEM: The combined pipeline/import action was incomplete or invalid: %v. Return one corrected batch_action containing create_pipeline and bulk deal records. Do not split this user request into separate confirmations.]", normalizeErr),
				)
				continue
			}
			return &crmAssistantResult{reply: pending.Description, pendingAction: pending}, nil

		case "query":
			if crmLeadPeriodQueryRequiresCreatedAt(req) && !crmSQLUsesCreatedAt(plan.SQL) {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Rejected query %d", iteration+1),
					"[SYSTEM: This is a time-bounded lead/deal analytics request. Filter the deals table by created_at for the requested period. Do not substitute start_date, due_date, or updated_at. Produce a corrected query now.]",
				)
				continue
			}
			sqlType, validationErr := ValidateAndClassifySQL(plan.SQL)
			if validationErr != nil {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Rejected query %d", iteration+1),
					fmt.Sprintf("The gateway rejected that SQL: %v. Produce a corrected, single safe query using only the supplied schema.", validationErr),
				)
				continue
			}
			if IsMutation(sqlType) {
				if !crmSQLMutationHasRestrictiveWhere(plan.SQL, sqlType) {
					dataContext = appendDataContext(
						dataContext,
						fmt.Sprintf("Rejected unsafe mutation %d", iteration+1),
						"[SYSTEM: UPDATE and DELETE must have a restrictive WHERE clause targeting only the intended record(s). Query exact candidates first if needed, then produce a corrected mutation.]",
					)
					continue
				}
				pending, pendingErr := p.handleSQLMutation(ctx, &models.DatabaseActionRequest{
					Action:         "query",
					SQL:            plan.SQL,
					SQLParams:      plan.SQLParams,
					Reply:          plan.Reply,
					SuccessMessage: plan.SuccessMessage,
					CancelMessage:  crmMutationCancelMessage(req.Message),
					ResourceEnvID:  p.resourceEnvId,
				}, sqlType)
				if pendingErr != nil {
					return nil, pendingErr
				}
				return &crmAssistantResult{
					reply:         pending.Description,
					pendingAction: pending.PendingAction,
				}, nil
			}

			result, queryErr := p.executeSQLQuery(ctx, EnsureSelectLimit(plan.SQL, defaultSelectLimit), plan.SQLParams, p.resourceEnvId)
			if queryErr != nil {
				dataContext = appendDataContext(
					dataContext,
					fmt.Sprintf("Failed query %d", iteration+1),
					fmt.Sprintf("The database rejected that query: %v. Correct the SQL using the exact schema and try again.", queryErr),
				)
				continue
			}
			encoded, marshalErr := json.Marshal(result)
			if marshalErr != nil {
				return nil, fmt.Errorf("encode CRM query result: %w", marshalErr)
			}
			label := plan.QueryPlan
			if strings.TrimSpace(label) == "" {
				label = fmt.Sprintf("CRM query %d", iteration+1)
			}
			dataContext = appendDataContext(dataContext, label, string(encoded))
			hasSuccessfulRead = true

		default:
			return nil, fmt.Errorf("unsupported CRM assistant action %q", plan.Action)
		}
	}

	return nil, fmt.Errorf("CRM assistant exceeded %d iterations", maxCRMAssistantIterations)
}

func crmRequestRequiresLiveLookup(req models.CRMAssistantRequest) bool {
	current := strings.ToLower(strings.TrimSpace(req.Message))
	if current == "" || crmRequestLooksLikeFieldSettings(current) {
		return false
	}

	// The current message must contain a data-oriented intent. Entity context may
	// come from the immediately preceding user request for follow-ups such as
	// "nomerlarini ham ber".
	if !containsAnyFold(current,
		"nechta", "neshta", "qancha", "soni", "sanab", "hisobla",
		"qaysi", "kim", "ro‘yxat", "ro'yxat", "royxat", "list",
		"nomer", "raqam", "telefon", "tel", "phone", "mobile", "number", "aloqa", "kontakt", "contact",
		"status", "stage", "bosqich", "source", "manba", "pipeline", "voronka",
		"budget", "byudjet", "summa", "eng katta", "eng kichik", "oxirgi", "so‘nggi", "so'nggi",
		"ber", "tashavor", "chiqar", "ko‘rsat", "ko'rsat", "korsat", "top", "find", "show me", "how many", "count", "latest",
		"сколько", "какой", "какие", "кто", "список", "номер", "телефон", "статус", "этап",
		"источник", "воронк", "бюджет", "сумм", "найд", "покажи", "последн",
	) {
		return false
	}

	contextText := current
	userMessages := 0
	for index := len(req.History) - 1; index >= 0 && userMessages < 4; index-- {
		if strings.EqualFold(strings.TrimSpace(req.History[index].Role), "user") {
			contextText += "\n" + strings.ToLower(req.History[index].Content)
			userMessages++
		}
	}

	if containsAnyFold(contextText,
		"lid", "lead", "лид",
		"deal", "сделк",
		"contact", "kontakt", "контакт",
		"mijoz", "client", "klient", "клиент",
		"task", "vazifa", "задач",
		"company", "kompaniya", "компан",
	) {
		return true
	}

	return containsAnyFold(strings.ToLower(strings.TrimSpace(req.PageContext.Table)),
		"deals", "contacts", "companies", "tasks",
	)
}

func crmRequestLooksLikeFieldSettings(message string) bool {
	hasFieldContext := containsAnyFold(message,
		"kartoch", "card", "field", "maydon", "column", "ustun", "поле", "карточ", "колон",
	)
	hasExplicitConfigurationIntent := containsAnyFold(message,
		"ko‘rin", "ko'rin", "korin", "yashir", "hide", "visible", "visibility",
		"tartib", "order", "joyla", "birinchi", "ikkinchi", "uchinchi",
		"1-chi", "2-chi", "3-chi", "first", "second", "third",
		"скрой", "видим", "порядок", "перв", "втор", "трет",
	)
	hasGenericShowIntent := containsAnyFold(message,
		"ko‘rsat", "ko'rsat", "korsat", "show", "покажи",
	)
	hasVisualReferenceIntent := containsAnyFold(message,
		"shunaqa", "manaqa", "huddi shu", "xuddi shu", "shu kabi",
		"rasmdag", "screenshot", "skrin", "like this", "match this", "как на", "как здесь",
	)
	return hasExplicitConfigurationIntent || (hasFieldContext && hasGenericShowIntent) || hasVisualReferenceIntent
}

func crmRequestLooksLikeMutation(message string) bool {
	message = strings.ToLower(strings.TrimSpace(message))
	if message == "" {
		return false
	}
	hasEntity := containsAnyFold(message,
		"pipeline", "pipleline", "voronka", "воронк",
		"stage", "bosqich", "этап",
		"lid", "lead", "лид", "deal", "сделк",
		"contact", "kontakt", "контакт", "company", "kompaniya", "компан",
		"task", "vazifa", "задач",
	)
	if !hasEntity {
		return false
	}
	return containsAnyFold(message,
		"create", "generate", "add ", "insert", "new pipeline", "new lead", "new deal",
		"yarat", "qo‘sh", "qo'sh", "qosh", "ochib ber", "ochvor",
		"update", "change", "edit", "rename", "move", "set ", "delete", "remove",
		"o‘zgart", "o'zgart", "ozgart", "almashtir", "ko‘chir", "ko'chir", "kochir", "o‘chir", "o'chir", "ochir",
		"созда", "добав", "измен", "обнов", "переимен", "перемест", "удал",
	)
}

func crmReplyIsClarification(reply string) bool {
	return strings.Contains(strings.TrimSpace(reply), "?")
}

func crmMutationCancelMessage(message string) string {
	switch detectCRMRequestLanguage(strings.ToLower(message)) {
	case "ru":
		return "Действие отменено. Данные не изменены."
	case "en":
		return "Action cancelled. No data was changed."
	default:
		return "Amal bekor qilindi. Hech narsa o‘zgarmadi."
	}
}

func crmLeadPeriodQueryRequiresCreatedAt(req models.CRMAssistantRequest) bool {
	message := strings.ToLower(strings.TrimSpace(req.Message))
	if !containsAnyFold(message, "lid", "lead", "deal", "лид", "сделк") {
		return false
	}
	// Respect an explicit request to analyze a different date dimension.
	if containsAnyFold(message,
		"start_date", "start date", "boshlanish san", "дата начал",
		"due_date", "due date", "deadline", "muddat", "срок",
		"updated_at", "updated at", "yangilangan", "обновлен",
	) {
		return false
	}

	return containsAnyFold(message,
		"bugun", "kecha", "kechagi", "kechigi", "hafta", "oy", "today", "yesterday", "week", "month", "сегодня", "вчера", "недел", "месяц",
		"yanvar", "fevral", "mart", "aprel", "mayda", "may oy", "iyun", "iyul", "avgust", "sentabr", "oktabr", "noyabr", "dekabr",
		"january", "february", "march", "april", "june", "july", "august", "september", "october", "november", "december",
		"январ", "феврал", "март", "апрел", "май", "июн", "июл", "август", "сентябр", "октябр", "ноябр", "декабр",
	)
}

func crmSQLUsesCreatedAt(sql string) bool {
	lowerSQL := strings.ToLower(sql)
	whereIndex := strings.Index(lowerSQL, "where")
	if whereIndex < 0 {
		return false
	}
	filterSQL := lowerSQL[whereIndex:]
	return strings.Contains(filterSQL, "created_at") &&
		!containsAnyFold(filterSQL, "start_date", "due_date", "updated_at")
}

func crmSQLMutationHasRestrictiveWhere(sql string, sqlType SQLType) bool {
	if sqlType != SQLTypeUpdate && sqlType != SQLTypeDelete {
		return true
	}
	statement := strings.ToLower(stripSQLComments(sql))
	if strings.HasPrefix(strings.TrimSpace(statement), "with") {
		keyword := " " + string(sqlType) + " "
		if mutationIndex := strings.LastIndex(statement, keyword); mutationIndex >= 0 {
			statement = statement[mutationIndex+1:]
		}
	}
	whereIndex := strings.Index(statement, "where")
	if whereIndex < 0 {
		return false
	}
	filter := strings.TrimSpace(statement[whereIndex+len("where"):])
	filter = strings.TrimSuffix(filter, ";")
	return filter != "" && filter != "true" && filter != "1=1" && filter != "1 = 1"
}

func crmAssistantHistory(messages []models.CRMAssistantMessage) []models.ChatMessage {
	const maxHistory = 12
	if len(messages) > maxHistory {
		messages = messages[len(messages)-maxHistory:]
	}

	history := make([]models.ChatMessage, 0, len(messages))
	for _, message := range messages {
		role := strings.ToLower(strings.TrimSpace(message.Role))
		content := strings.TrimSpace(message.Content)
		if (role != "user" && role != "assistant") || content == "" {
			continue
		}
		history = append(history, models.ChatMessage{
			Role: role,
			Content: []models.ContentBlock{{
				Type: "text",
				Text: content,
			}},
		})
	}
	return history
}

func normalizeCRMClientActions(
	actions []models.CRMClientAction,
	schema []models.TableSchema,
	fallbackTable string,
) []models.CRMClientAction {
	tables := make(map[string]map[string]string, len(schema))
	for _, table := range schema {
		aliases := make(map[string]string, len(table.Fields)*2)
		for _, field := range table.Fields {
			aliases[normalizeCRMFieldAlias(field.Slug)] = field.Slug
			if field.Label != "" {
				aliases[normalizeCRMFieldAlias(field.Label)] = field.Slug
			}
			for _, alias := range commonCRMFieldAliases(field.Slug) {
				aliases[normalizeCRMFieldAlias(alias)] = field.Slug
			}
		}
		for alias, field := range builtInCRMFieldAliases(table.Slug) {
			key := normalizeCRMFieldAlias(alias)
			if _, exists := aliases[key]; !exists {
				aliases[key] = field
			}
		}
		tables[table.Slug] = aliases
	}

	result := make([]models.CRMClientAction, 0, len(actions))
	for _, action := range actions {
		if action.Type != "set_card_field_visibility" {
			continue
		}
		table := strings.TrimSpace(action.Table)
		if table == "" {
			table = strings.TrimSpace(fallbackTable)
		}
		aliases, ok := tables[table]
		if !ok {
			continue
		}

		show := resolveCRMFields(action.ShowFields, aliases, nil)
		showSet := make(map[string]struct{}, len(show))
		for _, field := range show {
			showSet[field] = struct{}{}
		}
		hide := resolveCRMFields(action.HideFields, aliases, showSet)
		order := resolveCRMFields(action.FieldOrder, aliases, nil)
		if len(show) == 0 && len(hide) == 0 && len(order) == 0 {
			continue
		}

		result = append(result, models.CRMClientAction{
			Type:       action.Type,
			Table:      table,
			ShowFields: show,
			HideFields: hide,
			FieldOrder: order,
		})
	}
	return result
}

func canonicalDealScreenshotCardAction(
	schema []models.TableSchema,
	pageContext models.CRMAssistantPageContext,
) (models.CRMClientAction, bool) {
	if strings.TrimSpace(pageContext.Table) != "deals" || len(pageContext.CardFields) == 0 {
		return models.CRMClientAction{}, false
	}

	cardFields := make(map[string]struct{}, len(pageContext.CardFields))
	orderedCardFields := make([]string, 0, len(pageContext.CardFields))
	for _, field := range pageContext.CardFields {
		slug := strings.TrimSpace(field.Slug)
		if slug == "" {
			continue
		}
		if _, duplicate := cardFields[slug]; duplicate {
			continue
		}
		cardFields[slug] = struct{}{}
		orderedCardFields = append(orderedCardFields, slug)
	}

	for _, required := range []string{"name", "pipeline", "amount"} {
		if _, ok := cardFields[required]; !ok {
			return models.CRMClientAction{}, false
		}
	}

	statusFields := make([]string, 0, 1)
	for _, table := range schema {
		if table.Slug != "deals" {
			continue
		}
		for _, field := range table.Fields {
			if !strings.EqualFold(strings.TrimSpace(field.Type), "STATUS") {
				continue
			}
			if _, rendered := cardFields[field.Slug]; rendered {
				statusFields = append(statusFields, field.Slug)
			}
		}
		break
	}
	// Some resource-schema responses currently expose generated pipeline status
	// fields without their STATUS type. The frontend card context has already
	// narrowed pipeline_* entries to the selected pipeline, so one such slug is
	// an unambiguous active-status fallback.
	if len(statusFields) == 0 {
		for _, field := range orderedCardFields {
			if strings.HasPrefix(strings.ToLower(field), "pipeline_") {
				statusFields = append(statusFields, field)
			}
		}
	}
	if len(statusFields) != 1 {
		return models.CRMClientAction{}, false
	}

	desired := []string{"name", "pipeline", "amount", statusFields[0]}
	desiredSet := make(map[string]struct{}, len(desired))
	for _, field := range desired {
		desiredSet[field] = struct{}{}
	}
	hide := make([]string, 0, len(orderedCardFields)-len(desired))
	for _, field := range orderedCardFields {
		if _, keep := desiredSet[field]; !keep {
			hide = append(hide, field)
		}
	}

	return models.CRMClientAction{
		Type:       "set_card_field_visibility",
		Table:      "deals",
		ShowFields: append([]string(nil), desired...),
		HideFields: hide,
		FieldOrder: append([]string(nil), desired...),
	}, true
}

func crmClientActionsContainFields(actions []models.CRMClientAction, required ...string) bool {
	present := make(map[string]struct{})
	for _, action := range actions {
		if action.Type != "set_card_field_visibility" || action.Table != "deals" {
			continue
		}
		for _, field := range append(append([]string(nil), action.ShowFields...), action.FieldOrder...) {
			present[field] = struct{}{}
		}
	}
	for _, field := range required {
		if _, ok := present[field]; !ok {
			return false
		}
	}
	return true
}

func builtInCRMFieldAliases(table string) map[string]string {
	if table != "deals" {
		return nil
	}
	return map[string]string{
		"contacts_id":  "contacts_id",
		"phone":        "contacts_id",
		"phone number": "contacts_id",
		"mobile":       "contacts_id",
		"telefon":      "contacts_id",
		"телефон":      "contacts_id",
		"contact":      "contacts_id",
		"kontakt":      "contacts_id",
	}
}

func commonCRMFieldAliases(slug string) []string {
	switch slug {
	case "amount":
		return []string{"budget", "byudjet", "summa", "бюджет", "сумма"}
	case "contacts_id":
		return []string{"phone", "phone number", "mobile", "telefon", "телефон", "contact", "kontakt"}
	default:
		return nil
	}
}

func resolveCRMFields(fields []string, aliases map[string]string, excluded map[string]struct{}) []string {
	seen := make(map[string]struct{}, len(fields))
	result := make([]string, 0, len(fields))
	for _, requested := range fields {
		field, ok := aliases[normalizeCRMFieldAlias(requested)]
		if !ok {
			continue
		}
		if _, skip := excluded[field]; skip {
			continue
		}
		if _, duplicate := seen[field]; duplicate {
			continue
		}
		seen[field] = struct{}{}
		result = append(result, field)
	}
	return result
}

func normalizeCRMFieldAlias(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	replacer := strings.NewReplacer(" ", "", "_", "", "-", "", ".", "")
	return replacer.Replace(value)
}

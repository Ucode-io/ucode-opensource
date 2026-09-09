package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/spf13/cast"
)

// ─────────────────────────────────────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────────────────────────────────────

const (
	timeoutDatabaseAssistant = 120 * time.Second
	schemaCacheTTL           = 5 * time.Minute

	maxAgentIterations = 8

	defaultSelectLimit = 50
)

func (p *ChatProcessor) runDatabaseFlow(ctx context.Context, clarified string, chatHistory []models.ChatMessage) (*models.ParsedClaudeResponse, error) {

	resourceEnvId, err := p.resolveBuilderResourceID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve builder resource ID: %w", err)
	}

	// Fetch and cache the project schema for this request's lifetime.
	schema, err := p.getProjectSchemaCached(ctx, resourceEnvId)
	if err != nil {
		return nil, fmt.Errorf("schema fetch failed: %w", err)
	}

	// Format schema as compact SQL-friendly text instead of raw JSON.
	// This gives the AI clearer column type information for writing correct SQL.
	schemaText := formatSchemaForSQL(schema)

	var (
		dataContext string
		lastAction  *models.DatabaseActionRequest
	)

	for i := 0; i < maxAgentIterations; i++ {

		// On the final allowed iteration, force the AI to produce an answer.
		// This prevents the loop from exhausting without giving a user reply.
		if i == maxAgentIterations-1 && dataContext != "" {
			dataContext += "\n\n[SYSTEM: This is the final iteration. " +
				"You MUST set action=\"answer\" and provide the complete response in \"reply\" now. " +
				"Do NOT set needs_more_data=true.]"
		}

		if err := p.Check(); err != nil {
			return nil, err
		}

		action, err := p.callDatabaseAssistant(ctx, clarified, schemaText, dataContext, chatHistory)
		if err != nil {
			return nil, fmt.Errorf("AI call failed (iter %d): %w", i, err)
		}
		action.ResourceEnvID = resourceEnvId
		lastAction = action

		// ── Terminal: AI has enough information to answer ─────────────────────
		if action.Action == "answer" || action.Action == "schema" {
			return &models.ParsedClaudeResponse{Description: action.Reply}, nil
		}

		// ── SQL mode: AI provided a SQL statement ─────────────────────────────
		if action.SQL != "" {
			sqlType, valErr := ValidateAndClassifySQL(action.SQL)
			if valErr != nil {
				// Return a safe user-facing error without exposing internals.
				return &models.ParsedClaudeResponse{
					Description: fmt.Sprintf(
						"Не удалось сформировать безопасный запрос: %v\n\nПожалуйста, переформулируйте ваш запрос.",
						valErr,
					),
				}, nil
			}

			// ── Mutation: break out for user confirmation ─────────────────────
			if IsMutation(sqlType) {
				return p.handleSQLMutation(ctx, action, sqlType)
			}

			// ── Read query: execute and accumulate results ────────────────────
			safeSQL := EnsureSelectLimit(action.SQL, defaultSelectLimit)
			result, execErr := p.executeSQLQuery(ctx, safeSQL, action.SQLParams, resourceEnvId)
			if execErr != nil {
				return nil, fmt.Errorf("SQL execution failed (iter %d): %w", i, execErr)
			}

			resultJSON, _ := json.Marshal(result)
			label := fmt.Sprintf("Step %d — SQL query", i+1)
			if action.QueryPlan != "" {
				label = fmt.Sprintf("Step %d — %s", i+1, action.QueryPlan)
			}
			dataContext = appendDataContext(dataContext, label, string(resultJSON))

			// AI says it has all the data it needs → exit the loop.
			if !action.NeedsMoreData {
				break
			}
			continue
		}

		// ── ORM fallback: AI used the old JSON-action format ──────────────────
		// This branch handles backward-compatibility if the AI occasionally
		// falls back to the old schema-based action format.
		if action.Action == "create" || action.Action == "update" || action.Action == "delete" {
			return p.handleDatabaseMutation(ctx, action)
		}

		// read / count / aggregate via ORM
		iterData, execErr := p.executeDBAction(ctx, action)
		if execErr != nil {
			return nil, fmt.Errorf("ORM execution failed (iter %d, action=%s): %w", i, action.Action, execErr)
		}

		iterJSON, _ := json.Marshal(iterData)
		label := fmt.Sprintf("Step %d (%s → %s)", i+1, action.Action, action.TableSlug)
		if action.QueryPlan != "" {
			label = fmt.Sprintf("Step %d — %s", i+1, action.QueryPlan)
		}
		dataContext = appendDataContext(dataContext, label, string(iterJSON))

		if !action.NeedsMoreData {
			break
		}
	}

	// ── Final answer synthesis ────────────────────────────────────────────────
	// If the last action already carries a complete reply (and we broke out of
	// the loop because needs_more_data=false), return it directly to save one
	// extra AI round-trip.
	if lastAction != nil && !lastAction.NeedsMoreData && strings.TrimSpace(lastAction.Reply) != "" {
		reply := lastAction.Reply + buildDataContextHint(dataContext)
		return &models.ParsedClaudeResponse{Description: reply}, nil
	}

	// Call AI one final time to produce a polished, formatted reply from all
	// the accumulated query results.
	finalAction, err := p.callDatabaseAssistant(ctx, clarified, schemaText, dataContext, chatHistory)
	if err != nil {
		// Graceful degradation: return whatever partial reply we have.
		if lastAction != nil && lastAction.Reply != "" {
			return &models.ParsedClaudeResponse{Description: lastAction.Reply}, nil
		}
		return nil, fmt.Errorf("final AI synthesis call failed: %w", err)
	}

	return &models.ParsedClaudeResponse{Description: finalAction.Reply}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SQL EXECUTION
// ─────────────────────────────────────────────────────────────────────────────

// executeSQLQuery executes a read-only SQL statement (SELECT / CTE) via the
// new gRPC ExecuteSQL method and returns a normalised result map.
//
// params contains the $1, $2, ... bound values.
func (p *ChatProcessor) executeSQLQuery(
	ctx context.Context,
	sql string,
	params []any,
	resourceEnvId string,
) (any, error) {

	resp, err := p.service.GoObjectBuilderService().ObjectBuilder().ExecuteSQL(
		ctx,
		&nb.ExecuteSQLRequest{
			ResourceEnvId: resourceEnvId,
			Sql:           sql,
			Params:        sqlParamsToStrings(params),
			InTransaction: false, // reads never need a transaction
		},
	)
	if err != nil {
		return nil, fmt.Errorf("ExecuteSQL gRPC error: %w", err)
	}

	// Surface server-side errors returned in the response body.
	if errMsg := resp.GetError(); errMsg != "" {
		return nil, fmt.Errorf("database error: %s", errMsg)
	}

	// Convert repeated google.protobuf.Struct rows → []map[string]any
	rows := make([]map[string]any, 0, len(resp.GetRows()))
	for _, rowStruct := range resp.GetRows() {
		if m, convErr := helperFunc.ConvertStructToMap(rowStruct); convErr == nil {
			rows = append(rows, m)
		}
	}

	return map[string]any{
		"rows":          rows,
		"count":         len(rows),
		"rows_affected": resp.GetRowsAffected(),
	}, nil
}

// handleSQLMutation prepares a PendingAction for a mutation SQL statement.
// The mutation is NOT executed here — it is sent to the frontend, which
// shows the user a confirmation dialog and calls back with Approved=true/false.
func (p *ChatProcessor) handleSQLMutation(
	ctx context.Context,
	action *models.DatabaseActionRequest,
	sqlType SQLType,
) (*models.ParsedClaudeResponse, error) {

	// Ensure we get affected record GUIDs back after execution.
	sqlWithReturning := EnsureReturning(action.SQL)

	// Build human-readable confirmation messages.
	confirmationPrompt := action.Reply
	if strings.TrimSpace(confirmationPrompt) == "" {
		switch sqlType {
		case SQLTypeInsert:
			confirmationPrompt = "Создать записи в базе данных?"
		case SQLTypeUpdate:
			confirmationPrompt = "⚠️ Обновить записи в базе данных?"
		case SQLTypeDelete:
			confirmationPrompt = "⚠️ Удалить записи из базы данных? Это действие необратимо."
		}
	}

	successMessage := action.SuccessMessage
	if strings.TrimSpace(successMessage) == "" {
		successMessage = "✅ Операция выполнена успешно."
	}

	cancelMessage := action.CancelMessage
	if strings.TrimSpace(cancelMessage) == "" {
		cancelMessage = "Окей, действие отменено. Ничего не изменено."
	}

	pending := &models.PendingAction{
		// Tell executeMutation to use SQL path, not the ORM path.
		Action: "sql",

		// The validated + RETURNING-enriched SQL ready for execution.
		SQL:       sqlWithReturning,
		SQLParams: action.SQLParams,

		// Metadata for display and fallback error messages.
		TableSlug:          action.TableSlug, // may be empty for multi-table SQL
		ResourceEnvID:      action.ResourceEnvID,
		ProjectID:          action.ResourceEnvID,
		SuccessMessage:     successMessage,
		CancelMessage:      cancelMessage,
		ConfirmationPrompt: confirmationPrompt,
		Description:        confirmationPrompt,
	}

	return &models.ParsedClaudeResponse{
		Description:   confirmationPrompt,
		PendingAction: pending,
	}, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ORM FALLBACK EXECUTORS  (unchanged — kept for backward-compatibility)
// ─────────────────────────────────────────────────────────────────────────────

func (p *ChatProcessor) executeDBAction(ctx context.Context, action *models.DatabaseActionRequest) (any, error) {
	switch action.Action {
	case "read":
		return p.executeDatabaseRead(ctx, action)
	case "count":
		return p.executeDatabaseCount(ctx, action)
	case "aggregate":
		return p.executeDatabaseAggregate(ctx, action)
	default:
		return nil, fmt.Errorf("unknown read-type action: %s", action.Action)
	}
}

func (p *ChatProcessor) executeDatabaseRead(ctx context.Context, action *models.DatabaseActionRequest) (any, error) {
	if action.TableSlug == "" {
		return nil, fmt.Errorf("table_slug is required")
	}

	dataMap := make(map[string]any, len(action.Filters)+3)
	for k, v := range action.Filters {
		dataMap[k] = v
	}
	dataMap["limit"] = 50
	if action.Limit > 0 {
		dataMap["limit"] = action.Limit
	}
	if action.Offset > 0 {
		dataMap["offset"] = action.Offset
	}
	if action.OrderBy != "" {
		dataMap["order_by"] = action.OrderBy
	}

	structData, err := helperFunc.ConvertMapToStruct(dataMap)
	if err != nil {
		return nil, fmt.Errorf("convert filters: %w", err)
	}

	resp, err := p.service.GoObjectBuilderService().ObjectBuilder().GetList2(
		ctx, &nb.CommonMessage{
			TableSlug: action.TableSlug,
			ProjectId: action.ResourceEnvID,
			Data:      structData,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("GetList2 failed: %w", err)
	}

	if resp.GetData() != nil {
		result, err := helperFunc.ConvertStructToMap(resp.GetData())
		if err != nil {
			return nil, fmt.Errorf("convert GetList2 response: %w", err)
		}
		return result, nil
	}

	return map[string]any{"response": []any{}, "count": 0}, nil
}

func (p *ChatProcessor) executeDatabaseCount(ctx context.Context, action *models.DatabaseActionRequest) (any, error) {
	data, err := p.executeDatabaseRead(ctx, &models.DatabaseActionRequest{
		TableSlug:     action.TableSlug,
		Filters:       action.Filters,
		Limit:         1,
		ResourceEnvID: action.ResourceEnvID,
	})
	if err != nil {
		return nil, fmt.Errorf("count read failed: %w", err)
	}
	return map[string]any{
		"count":      extractCountFromData(data),
		"table_slug": action.TableSlug,
		"filters":    action.Filters,
	}, nil
}

func (p *ChatProcessor) executeDatabaseAggregate(ctx context.Context, action *models.DatabaseActionRequest) (any, error) {
	if action.AggregationField == "" {
		return nil, fmt.Errorf("aggregation_field is required")
	}

	aggFunc := strings.ToUpper(action.Aggregation)
	if aggFunc == "" {
		aggFunc = "SUM"
	}

	colExpr := fmt.Sprintf("%s(%s) as result", aggFunc, action.AggregationField)
	columns := []string{colExpr}
	var groupBy []string
	if action.GroupBy != "" {
		columns = append([]string{action.GroupBy}, columns...)
		groupBy = []string{action.GroupBy}
	}

	queryParams := map[string]any{
		"operation": "SELECT",
		"table":     fmt.Sprintf(`"%s"`, action.TableSlug),
		"columns":   columns,
		"where":     buildWhereClause(action.Filters),
	}
	if len(groupBy) > 0 {
		queryParams["group_by"] = groupBy
	}

	structData, err := helperFunc.ConvertMapToStruct(queryParams)
	if err != nil {
		return nil, fmt.Errorf("build aggregation params: %w", err)
	}

	resp, err := p.service.GoObjectBuilderService().ObjectBuilder().GetListAggregation(ctx, &nb.CommonMessage{
		TableSlug: action.TableSlug,
		ProjectId: action.ResourceEnvID,
		Data:      structData,
	})
	if err != nil {
		return nil, fmt.Errorf("GetListAggregation failed: %w", err)
	}

	var aggResult any = map[string]any{"data": []any{}}
	if resp.GetData() != nil {
		if m, err := helperFunc.ConvertStructToMap(resp.GetData()); err == nil {
			aggResult = m
		}
	}

	return map[string]any{
		"aggregation": aggFunc,
		"field":       action.AggregationField,
		"group_by":    action.GroupBy,
		"table_slug":  action.TableSlug,
		"result":      aggResult,
	}, nil
}

// buildWhereClause converts a filters map to a raw SQL WHERE string.
// Used only by GetListAggregation which takes raw SQL (ORM path).
func buildWhereClause(filters map[string]any) string {
	conditions := []string{"deleted_at IS NULL"}
	for k, v := range filters {
		switch val := v.(type) {
		case string:
			safe := strings.ReplaceAll(val, "'", "''")
			conditions = append(conditions, fmt.Sprintf(`"%s" = '%s'`, k, safe))
		case float64:
			conditions = append(conditions, fmt.Sprintf(`"%s" = %v`, k, val))
		case bool:
			conditions = append(conditions, fmt.Sprintf(`"%s" = %v`, k, val))
		case map[string]any:
			for op, opVal := range val {
				switch op {
				case "$gt":
					conditions = append(conditions, fmt.Sprintf(`"%s" > %s`, k, sqlLiteral(opVal)))
				case "$gte":
					conditions = append(conditions, fmt.Sprintf(`"%s" >= %s`, k, sqlLiteral(opVal)))
				case "$lt":
					conditions = append(conditions, fmt.Sprintf(`"%s" < %s`, k, sqlLiteral(opVal)))
				case "$lte":
					conditions = append(conditions, fmt.Sprintf(`"%s" <= %s`, k, sqlLiteral(opVal)))
				case "$in":
					if list := sqlInList(opVal); list != "" {
						conditions = append(conditions, fmt.Sprintf(`"%s" IN (%s)`, k, list))
					}
				}
			}
		}
	}
	return strings.Join(conditions, " AND ")
}

// sqlLiteral renders a scalar filter value as a safe SQL literal: strings are
// single-quote escaped, numbers are formatted without scientific notation, and
// booleans are emitted verbatim. Field names are validated against the table
// schema by callers, so only values ever flow through here.
func sqlLiteral(v any) string {
	switch val := v.(type) {
	case string:
		return "'" + strings.ReplaceAll(val, "'", "''") + "'"
	case bool:
		return fmt.Sprintf("%v", val)
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	case json.Number:
		return val.String()
	default:
		return "'" + strings.ReplaceAll(fmt.Sprintf("%v", val), "'", "''") + "'"
	}
}

// sqlInList renders the operand of an $in filter as a comma-separated list of SQL
// literals. It returns "" when the operand is not a non-empty list, so the caller
// can drop the condition entirely rather than emit an empty IN ().
func sqlInList(v any) string {
	items, ok := v.([]any)
	if !ok || len(items) == 0 {
		return ""
	}
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, sqlLiteral(it))
	}
	return strings.Join(parts, ", ")
}

// ─────────────────────────────────────────────────────────────────────────────
// MUTATION HANDLER — ORM path (unchanged, kept for backward-compatibility)
// ─────────────────────────────────────────────────────────────────────────────

func (p *ChatProcessor) handleDatabaseMutation(ctx context.Context, action *models.DatabaseActionRequest) (*models.ParsedClaudeResponse, error) {
	if action.Action == "update" || action.Action == "delete" {
		if _, hasGuid := action.Filters["guid"]; !hasGuid {
			guids, err := p.resolveGuidsFromFilters(ctx, action)
			if err != nil {
				return nil, fmt.Errorf("guid resolution failed: %w", err)
			}
			if len(guids) == 0 {
				return &models.ParsedClaudeResponse{
					Description: "Записи с такими параметрами не найдены. Уточните запрос.",
				}, nil
			}
			if len(guids) == 1 {
				action.Filters = map[string]any{"guid": guids[0]}
			} else {
				action.Filters = map[string]any{"guid": guids[0], "_resolved_guids": guids}
			}
		}
	}

	affectedCount := 1
	if action.Action != "create" && len(action.Filters) > 0 {
		if data, err := p.executeDatabaseRead(ctx, &models.DatabaseActionRequest{
			TableSlug:     action.TableSlug,
			Filters:       action.Filters,
			Limit:         1,
			ResourceEnvID: action.ResourceEnvID,
		}); err == nil {
			affectedCount = extractCountFromData(data)
		}
	}

	confirmationPrompt := action.Reply
	if confirmationPrompt == "" {
		switch action.Action {
		case "create":
			confirmationPrompt = fmt.Sprintf("Создать новую запись в `%s`?", action.TableSlug)
		case "update":
			confirmationPrompt = fmt.Sprintf("Обновить **%d** запис(ей) в `%s`?", affectedCount, action.TableSlug)
		case "delete":
			confirmationPrompt = fmt.Sprintf("⚠️ Удалить **%d** запис(ей) из `%s`?", affectedCount, action.TableSlug)
		}
	}

	successMessage := action.SuccessMessage
	if successMessage == "" {
		switch action.Action {
		case "create":
			successMessage = fmt.Sprintf("✅ Запись успешно создана в `%s`.", action.TableSlug)
		case "update":
			successMessage = fmt.Sprintf("✅ Обновлено **%d** запис(ей) в `%s`.", affectedCount, action.TableSlug)
		case "delete":
			successMessage = fmt.Sprintf("✅ Удалено **%d** запис(ей) из `%s`.", affectedCount, action.TableSlug)
		}
	}

	cancelMessage := action.CancelMessage
	if cancelMessage == "" {
		cancelMessage = "Окей, действие отменено. Ничего не изменено."
	}

	pending := &models.PendingAction{
		Action:             action.Action,
		TableSlug:          action.TableSlug,
		Filters:            action.Filters,
		Data:               action.Data,
		AffectedCount:      affectedCount,
		Description:        confirmationPrompt,
		ProjectID:          action.ResourceEnvID,
		ResourceEnvID:      action.ResourceEnvID,
		SuccessMessage:     successMessage,
		CancelMessage:      cancelMessage,
		ConfirmationPrompt: confirmationPrompt,
	}

	return &models.ParsedClaudeResponse{
		Description:   confirmationPrompt,
		PendingAction: pending,
	}, nil
}

func (p *ChatProcessor) resolveGuidsFromFilters(ctx context.Context, action *models.DatabaseActionRequest) ([]string, error) {
	filterMap := make(map[string]any, len(action.Filters)+1)
	for k, v := range action.Filters {
		filterMap[k] = v
	}
	filterMap["limit"] = 100

	structData, err := helperFunc.ConvertMapToStruct(filterMap)
	if err != nil {
		return nil, fmt.Errorf("convert filters for guid resolution: %w", err)
	}

	resp, err := p.service.GoObjectBuilderService().ObjectBuilder().GetList2(
		ctx,
		&nb.CommonMessage{
			TableSlug: action.TableSlug,
			ProjectId: action.ResourceEnvID,
			Data:      structData,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("GetList2 for guid resolution: %w", err)
	}

	var guids []string
	if resp.GetData() != nil {
		listData, err := helperFunc.ConvertStructToMap(resp.GetData())
		if err == nil {
			for _, item := range extractItemsFromData(listData) {
				if g, ok := item["guid"].(string); ok && g != "" {
					guids = append(guids, g)
				}
			}
		}
	}
	return guids, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// executeMutation — executes a confirmed PendingAction
//
// Called from handlePendingConfirmation after the user clicks "Yes".
// Supports two paths:
//   - "sql"    → new SQL path: executes action.SQL via ExecuteSQL gRPC
//   - "create" → ORM path: Items.Create (single record)
//   - "update" → ORM path: Items.Update per guid (may be multiple)
//   - "delete" → ORM path: Items.Delete (single record)
//
// ─────────────────────────────────────────────────────────────────────────────
func executeMutation(ctx context.Context, action *models.PendingAction, service services.ServiceManagerI) (any, error) {
	if action.TableSlug == "" && action.SQL == "" {
		return nil, fmt.Errorf("either table_slug or sql is required")
	}

	effectiveProjectID := action.ResourceEnvID
	if effectiveProjectID == "" {
		effectiveProjectID = action.ProjectID
	}

	switch action.Action {

	// ── SQL PATH — bulk-safe, atomic, supports JOINs and CTEs ─────────────
	case "sql":
		if strings.TrimSpace(action.SQL) == "" {
			return nil, fmt.Errorf("sql field is required for action=sql")
		}

		// Re-validate SQL as a second layer of defence.
		// The first validation happened in handleSQLMutation before the user
		// even saw the confirmation prompt.
		sqlType, err := ValidateAndClassifySQL(action.SQL)
		if err != nil {
			return nil, fmt.Errorf("SQL safety re-validation failed: %w", err)
		}
		if !IsMutation(sqlType) {
			// SELECT somehow made it through — refuse execution.
			return nil, fmt.Errorf("only INSERT/UPDATE/DELETE are allowed in mutation context, got: %s", sqlType)
		}

		resp, err := service.GoObjectBuilderService().ObjectBuilder().ExecuteSQL(
			ctx,
			&nb.ExecuteSQLRequest{
				ResourceEnvId: effectiveProjectID,
				Sql:           action.SQL,
				Params:        sqlParamsToStrings(action.SQLParams),
				InTransaction: true, // always wrap mutations in a transaction
			},
		)
		if err != nil {
			return nil, fmt.Errorf("SQL mutation failed: %w", err)
		}

		// Surface server-side errors.
		if errMsg := resp.GetError(); errMsg != "" {
			return nil, fmt.Errorf("database error: %s", errMsg)
		}

		// Convert returned rows (from RETURNING clause) to Go maps.
		rows := make([]map[string]any, 0, len(resp.GetRows()))
		for _, rowStruct := range resp.GetRows() {
			if m, convErr := helperFunc.ConvertStructToMap(rowStruct); convErr == nil {
				rows = append(rows, m)
			}
		}

		// rows_affected can come from the proto field or from the RETURNING rows count.
		rowsAffected := resp.GetRowsAffected()
		if rowsAffected == 0 && len(rows) > 0 {
			rowsAffected = int64(len(rows))
		}

		return map[string]any{
			"status":        "executed",
			"rows_affected": rowsAffected,
			"rows":          rows,
		}, nil

	// ── ORM CREATE PATH — single record ───────────────────────────────────
	case "create":
		data := action.Data
		if data == nil {
			data = map[string]any{}
		}
		data["company_service_project_id"] = action.ProjectID

		structData, err := helperFunc.ConvertMapToStruct(data)
		if err != nil {
			return nil, fmt.Errorf("convert create data: %w", err)
		}
		resp, err := service.GoObjectBuilderService().Items().Create(ctx, &nb.CommonMessage{
			TableSlug: action.TableSlug,
			ProjectId: effectiveProjectID,
			Data:      structData,
		})
		if err != nil {
			return nil, fmt.Errorf("create failed: %w", err)
		}
		if resp.GetData() != nil {
			return helperFunc.ConvertStructToMap(resp.GetData())
		}
		return map[string]any{"status": "created"}, nil

	// ── ORM UPDATE PATH — one gRPC call per guid ──────────────────────────
	case "update":
		guids, err := resolveUpdateGuids(ctx, action, service, effectiveProjectID)
		if err != nil {
			return nil, err
		}

		var lastResp any
		for _, guid := range guids {
			updateData := make(map[string]any, len(action.Data)+3)
			for k, v := range action.Data {
				updateData[k] = v
			}
			updateData["guid"] = guid
			updateData["id"] = guid
			updateData["company_service_project_id"] = action.ProjectID

			structData, err := helperFunc.ConvertMapToStruct(updateData)
			if err != nil {
				return nil, fmt.Errorf("convert update data for guid %s: %w", guid, err)
			}
			resp, err := service.GoObjectBuilderService().Items().Update(ctx, &nb.CommonMessage{
				TableSlug: action.TableSlug,
				ProjectId: effectiveProjectID,
				Data:      structData,
			})
			if err != nil {
				return nil, fmt.Errorf("update failed for guid %s: %w", guid, err)
			}
			if resp.GetData() != nil {
				lastResp, _ = helperFunc.ConvertStructToMap(resp.GetData())
			}
		}
		if lastResp != nil {
			return lastResp, nil
		}
		return map[string]any{"status": "updated", "count": len(guids)}, nil

	// ── ORM DELETE PATH — single record ───────────────────────────────────
	case "delete":
		idToDelete, err := resolveDeleteGuid(ctx, action, service, effectiveProjectID)
		if err != nil {
			return nil, err
		}

		deleteData := map[string]any{
			"id":                         idToDelete,
			"guid":                       idToDelete,
			"company_service_project_id": action.ProjectID,
		}
		structData, err := helperFunc.ConvertMapToStruct(deleteData)
		if err != nil {
			return nil, fmt.Errorf("convert delete data: %w", err)
		}
		if _, err = service.GoObjectBuilderService().Items().Delete(ctx, &nb.CommonMessage{
			TableSlug: action.TableSlug,
			ProjectId: effectiveProjectID,
			Data:      structData,
		}); err != nil {
			return nil, fmt.Errorf("delete failed: %w", err)
		}
		return map[string]any{"status": "deleted", "affected_count": action.AffectedCount}, nil

	default:
		return nil, fmt.Errorf("unsupported action: %s", action.Action)
	}
}

// resolveUpdateGuids extracts or resolves the list of GUIDs to update.
// Priority: _resolved_guids filter → single guid filter → lookup by other filters.
func resolveUpdateGuids(
	ctx context.Context,
	action *models.PendingAction,
	service services.ServiceManagerI,
	effectiveProjectID string,
) ([]string, error) {

	// 1. Pre-resolved list from handleDatabaseMutation
	if raw, ok := action.Filters["_resolved_guids"]; ok {
		guids := cast.ToStringSlice(raw)
		result := make([]string, 0, len(guids))
		for _, g := range guids {
			if g != "" {
				result = append(result, g)
			}
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// 2. Single guid in filters
	if guid, ok := action.Filters["guid"]; ok {
		if g := cast.ToString(guid); g != "" {
			return []string{g}, nil
		}
	}

	// 3. Lookup by other filter fields
	filterMap := make(map[string]any, len(action.Filters)+1)
	for k, v := range action.Filters {
		if k == "_resolved_guids" {
			continue
		}
		filterMap[k] = v
	}
	filterMap["limit"] = 100

	filterStruct, err := helperFunc.ConvertMapToStruct(filterMap)
	if err != nil {
		return nil, fmt.Errorf("convert lookup filters for update: %w", err)
	}
	listResp, err := service.GoObjectBuilderService().ObjectBuilder().GetList2(
		ctx,
		&nb.CommonMessage{
			TableSlug: action.TableSlug,
			ProjectId: effectiveProjectID,
			Data:      filterStruct,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("lookup before update failed: %w", err)
	}

	var guids []string
	if listResp.GetData() != nil {
		listData, err := helperFunc.ConvertStructToMap(listResp.GetData())
		if err == nil {
			for _, item := range extractItemsFromData(listData) {
				if g, ok := item["guid"].(string); ok && g != "" {
					guids = append(guids, g)
				}
			}
		}
	}

	if len(guids) == 0 {
		return nil, fmt.Errorf("update failed: no records found with given filters")
	}
	return guids, nil
}

// resolveDeleteGuid extracts or resolves the single GUID to delete.
func resolveDeleteGuid(
	ctx context.Context,
	action *models.PendingAction,
	service services.ServiceManagerI,
	effectiveProjectID string,
) (string, error) {

	if guid, ok := action.Filters["guid"]; ok {
		if g := cast.ToString(guid); g != "" {
			return g, nil
		}
	}
	if id, ok := action.Filters["id"]; ok {
		if g := cast.ToString(id); g != "" {
			return g, nil
		}
	}

	// Fallback: lookup by other filters, take first result
	filterMap := make(map[string]any, len(action.Filters)+1)
	for k, v := range action.Filters {
		filterMap[k] = v
	}
	filterMap["limit"] = 1

	filterStruct, err := helperFunc.ConvertMapToStruct(filterMap)
	if err != nil {
		return "", fmt.Errorf("convert lookup filters for delete: %w", err)
	}
	listResp, err := service.GoObjectBuilderService().ObjectBuilder().GetList2(
		ctx,
		&nb.CommonMessage{
			TableSlug: action.TableSlug,
			ProjectId: effectiveProjectID,
			Data:      filterStruct,
		},
	)
	if err != nil {
		return "", fmt.Errorf("lookup before delete failed: %w", err)
	}

	if listResp.GetData() != nil {
		listData, err := helperFunc.ConvertStructToMap(listResp.GetData())
		if err == nil {
			items := extractItemsFromData(listData)
			if len(items) > 0 {
				if g, ok := items[0]["guid"].(string); ok && g != "" {
					return g, nil
				}
			}
		}
	}

	return "", fmt.Errorf("delete failed: record not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// SCHEMA — cached per ChatProcessor instance
// ─────────────────────────────────────────────────────────────────────────────

func (p *ChatProcessor) getProjectSchemaCached(ctx context.Context, resourceEnvId string) ([]models.TableSchema, error) {
	if p.schemaCache != nil && time.Now().Before(p.schemaCachedAt.Add(schemaCacheTTL)) {
		return p.schemaCache, nil
	}
	schema, err := p.fetchProjectSchema(ctx, resourceEnvId)
	if err != nil {
		return nil, err
	}
	p.schemaCache = schema
	p.schemaCachedAt = time.Now()
	return schema, nil
}

func (p *ChatProcessor) fetchProjectSchema(ctx context.Context, resourceEnvId string) ([]models.TableSchema, error) {
	if resourceEnvId == "" {
		return nil, fmt.Errorf("resource environment id is required")
	}

	tablesResp, err := p.service.GoObjectBuilderService().Table().GetAll(ctx, &nb.GetAllTablesRequest{
		ProjectId: resourceEnvId,
		Limit:     100,
	})
	if err != nil {
		return nil, fmt.Errorf("list tables: %w", err)
	}

	tables := tablesResp.GetTables()

	type schemaResult struct {
		idx    int
		schema models.TableSchema
	}
	ch := make(chan schemaResult, len(tables))

	for i, tbl := range tables {
		go func(idx int, t interface {
			GetId() string
			GetSlug() string
			GetLabel() string
		}) {
			fieldsResp, err := p.service.GoObjectBuilderService().Field().GetAll(ctx, &nb.GetAllFieldsRequest{
				TableId:   t.GetId(),
				ProjectId: resourceEnvId,
				Limit:     200,
			})
			var fields []models.FieldSchema
			if err == nil {
				for _, f := range fieldsResp.GetFields() {
					fields = append(fields, models.FieldSchema{
						Slug:    f.GetSlug(),
						Label:   f.GetLabel(),
						Type:    f.GetType(),
						Options: extractSchemaFieldOptions(f.GetAttributes().AsMap()),
					})
				}
			}
			ch <- schemaResult{idx: idx, schema: models.TableSchema{
				Slug:   t.GetSlug(),
				Label:  t.GetLabel(),
				Fields: fields,
			}}
		}(i, tbl)
	}

	collected := make([]models.TableSchema, len(tables))
	for range tables {
		r := <-ch
		collected[r.idx] = r.schema
	}

	schemas := make([]models.TableSchema, 0, len(collected))
	for _, s := range collected {
		schemas = append(schemas, s)
	}
	return schemas, nil
}

// formatSchemaForSQL converts the project schema into a compact, SQL-friendly
// plain-text format that is more useful to the AI than raw JSON.
//
// Example output:
//
//	Table: tasks (Задачи)
//	Columns: guid uuid, title text [label="Title"], status text [label="Status"], assigned_to uuid
//
//	Table: users (Пользователи)
//	Columns: guid uuid, name text, email text, deleted_at timestamptz
func formatSchemaForSQL(tables []models.TableSchema) string {
	return formatSchema(tables, false)
}

func formatSchemaForCRM(tables []models.TableSchema) string {
	return formatSchema(tables, true)
}

func formatSchema(tables []models.TableSchema, includeOptions bool) string {
	var sb strings.Builder
	for _, t := range tables {
		sb.WriteString("Table: ")
		sb.WriteString(t.Slug)
		if t.Label != "" && t.Label != t.Slug {
			sb.WriteString(" (")
			sb.WriteString(t.Label)
			sb.WriteString(")")
		}
		sb.WriteString("\nColumns: ")

		cols := make([]string, 0, len(t.Fields))
		for _, f := range t.Fields {
			colType := f.Type
			if colType == "" {
				colType = "text"
			}
			column := fmt.Sprintf("%s %s", f.Slug, colType)
			if f.Label != "" && f.Label != f.Slug {
				column += fmt.Sprintf(" [label=%s]", strconv.Quote(f.Label))
			}
			if includeOptions && len(f.Options) > 0 {
				column += fmt.Sprintf(" [options=%s]", strconv.Quote(strings.Join(f.Options, "|")))
			}
			cols = append(cols, column)
		}
		sb.WriteString(strings.Join(cols, ", "))
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// extractSchemaFieldOptions exposes human-readable option labels to the CRM
// agent. In particular, it lets vision map a status value visible in a card
// screenshot to the exact generated per-pipeline field that owns that value.
func extractSchemaFieldOptions(attributes map[string]any) []string {
	const maxOptions = 40
	seen := make(map[string]struct{})
	options := make([]string, 0)

	var walk func(any)
	walk = func(value any) {
		if len(options) >= maxOptions {
			return
		}
		switch typed := value.(type) {
		case map[string]any:
			for key, nested := range typed {
				if strings.EqualFold(key, "options") {
					items, ok := nested.([]any)
					if !ok {
						continue
					}
					for _, item := range items {
						candidate := schemaOptionLabel(item)
						if candidate == "" {
							continue
						}
						normalized := strings.ToLower(strings.TrimSpace(candidate))
						if _, exists := seen[normalized]; exists {
							continue
						}
						seen[normalized] = struct{}{}
						options = append(options, candidate)
						if len(options) >= maxOptions {
							return
						}
					}
					continue
				}
				walk(nested)
			}
		case []any:
			for _, nested := range typed {
				walk(nested)
			}
		}
	}
	walk(attributes)
	return options
}

func schemaOptionLabel(value any) string {
	item, ok := value.(map[string]any)
	if !ok {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	for _, key := range []string{"label", "label_en", "label_uz", "label_cyr", "label_ru", "name", "value", "slug"} {
		if label := strings.TrimSpace(fmt.Sprint(item[key])); label != "" && label != "<nil>" {
			return label
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// RESOURCE RESOLUTION
// ─────────────────────────────────────────────────────────────────────────────

func (p *ChatProcessor) resolveBuilderResourceID(ctx context.Context) (string, error) {

	mcpProject, err := p.service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(ctx, &nb.McpProjectId{
		ResourceEnvId: p.resourceEnvId,
		Id:            p.mcpProjectId,
	})
	if err != nil {
		return "", fmt.Errorf("get MCP project metadata: %w", err)
	}

	backendProjectID := mcpProject.GetUcodeProjectId()
	backendEnvID := mcpProject.GetEnvironmentId()
	if backendProjectID == "" || backendEnvID == "" {
		return "", fmt.Errorf("no backend project linked (project=%q env=%q)", backendProjectID, backendEnvID)
	}

	resp, err := p.service.CompanyService().ServiceResource().GetSingle(ctx, &company_service.GetSingleServiceResourceReq{
		ProjectId:     backendProjectID,
		EnvironmentId: backendEnvID,
		ServiceType:   company_service.ServiceType_BUILDER_SERVICE,
	})
	if err != nil {
		return "", fmt.Errorf("resolve builder resource (%s/%s): %w", backendProjectID, backendEnvID, err)
	}

	return resp.ResourceEnvironmentId, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AI API CALL
// ─────────────────────────────────────────────────────────────────────────────

func (p *ChatProcessor) callDatabaseAssistant(
	ctx context.Context,
	clarified, schemaText, dataContext string,
	chatHistory []models.ChatMessage,
) (*models.DatabaseActionRequest, error) {

	return p.agent.DatabaseQuery(ctx, models.DatabaseQueryInput{
		Clarified:   clarified,
		SchemaText:  schemaText,
		DataContext: dataContext,
		History:     chatHistory,
	})
}

// ─────────────────────────────────────────────────────────────────────────────
// DATA HELPERS
// ─────────────────────────────────────────────────────────────────────────────

// extractItemsFromData pulls the records slice out of a response map.
// Handles both ORM responses ("response", "data", "items") and
// SQL responses ("rows").
func extractItemsFromData(data any) []map[string]any {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	// SQL path returns "rows"; ORM path returns "response" / "data" / "items"
	for _, key := range []string{"rows", "response", "data", "items"} {
		if raw, ok := m[key]; ok {
			if items, ok := raw.([]any); ok {
				out := make([]map[string]any, 0, len(items))
				for _, item := range items {
					if row, ok := item.(map[string]any); ok {
						out = append(out, row)
					}
				}
				return out
			}
			// SQL path may return []map[string]any directly
			if items, ok := raw.([]map[string]any); ok {
				return items
			}
		}
	}
	return nil
}

// extractCountFromData reads the server-side total count from a response map.
func extractCountFromData(data any) int {
	m, ok := data.(map[string]any)
	if !ok {
		return 0
	}
	if count, ok := m["count"]; ok {
		switch v := count.(type) {
		case float64:
			return int(v)
		case int:
			return v
		case int64:
			return int(v)
		}
	}
	return len(extractItemsFromData(data))
}

// appendDataContext appends a labelled JSON blob to the growing dataContext
// string that is passed back to the AI on each iteration.
func appendDataContext(existing, label, jsonData string) string {
	if existing == "" {
		return fmt.Sprintf("=== %s ===\n%s", label, jsonData)
	}
	return fmt.Sprintf("%s\n\n=== %s ===\n%s", existing, label, jsonData)
}

// buildDataContextHint parses GUIDs from the accumulated dataContext and
// formats them as a fenced ```db-context``` block. The AI reads this in
// subsequent turns to resolve follow-up references ("update the first one")
// without re-fetching.
func buildDataContextHint(dataContext string) string {
	if dataContext == "" {
		return ""
	}

	type recordRef struct {
		guid    string
		display string
	}

	displayFields := []string{"name", "title", "label", "full_name", "email", "username", "code"}
	var refs []recordRef

	sections := strings.Split(dataContext, "\n\n=== ")
	for _, section := range sections {
		newline := strings.Index(section, "\n")
		if newline == -1 {
			continue
		}
		blob := strings.TrimSpace(section[newline+1:])
		if blob == "" {
			continue
		}

		var parsed map[string]any
		if err := json.Unmarshal([]byte(blob), &parsed); err != nil {
			continue
		}

		for _, item := range extractItemsFromData(parsed) {
			guid, _ := item["guid"].(string)
			if guid == "" {
				continue
			}
			var display string
			for _, df := range displayFields {
				if val, ok := item[df]; ok && val != nil {
					display = fmt.Sprintf("%v", val)
					break
				}
			}
			refs = append(refs, recordRef{guid: guid, display: display})
			if len(refs) >= 20 {
				goto done
			}
		}
	}

done:
	if len(refs) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n\n```db-context\nfetched_records:\n")
	for _, r := range refs {
		if r.display != "" {
			sb.WriteString(fmt.Sprintf("  - guid: %s  # %s\n", r.guid, r.display))
		} else {
			sb.WriteString(fmt.Sprintf("  - guid: %s\n", r.guid))
		}
	}
	sb.WriteString("```")
	return sb.String()
}

// sqlParamsToStrings converts []any params from the AI JSON response to
// []string for the gRPC ExecuteSQLRequest.Params field.
// Every value is formatted via fmt.Sprintf so numbers, bools, and strings
// all serialize predictably.
func sqlParamsToStrings(params []any) []string {
	if len(params) == 0 {
		return nil
	}
	out := make([]string, len(params))
	for i, p := range params {
		out[i] = fmt.Sprintf("%v", p)
	}
	return out
}

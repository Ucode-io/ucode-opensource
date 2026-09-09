package v1

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/spf13/cast"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
)

const (
	toolGetSchema        = "get_schema"
	toolCreateTable      = "create_table"
	toolCreateLoginTable = "create_login_table"
	toolCreateField      = "create_field"
	toolCreateRelation   = "create_relation"
	toolCreateMenu       = "create_menu"
	toolInsertItems      = "insert_items"

	toolCountItems     = "count_items"
	toolListItems      = "list_items"
	toolAggregateItems = "aggregate_items"
)

const (
	ucodeListDefaultLimit = 20
	ucodeListMaxLimit     = 50
)

var ucodeFieldTypes = []string{
	"SINGLE_LINE", "MULTI_LINE", "NUMBER", "BOOLEAN", "DATE", "DATE_TIME",
	"EMAIL", "PHONE", "PHOTO", "JSON", "PICK_LIST",
}

// ucodeLoginStrategies are the identifier types end-users may authenticate with.
// The object builder auto-creates the matching auth columns (login/email/phone +
// password) when a table is created with auth_info.login_strategy set.
var (
	ucodeLoginStrategies   = []string{"login", "email", "phone"}
	ucodeLoginStrategySet  = map[string]bool{"login": true, "email": true, "phone": true}
	ucodeDefaultLoginStrat = []string{"login"}
)

var (
	ucodeAggregateFuncs   = []string{"COUNT", "SUM", "AVG", "MIN", "MAX"}
	ucodeAggregateFuncSet = map[string]bool{"COUNT": true, "SUM": true, "AVG": true, "MIN": true, "MAX": true}
)

// ucodeSystemFields are columns present on every project table. They are always
// valid filter / aggregation targets even though they are not user-defined fields
// returned by the schema cache.
var ucodeSystemFields = map[string]bool{
	"guid": true, "created_at": true, "updated_at": true, "deleted_at": true,
}

func ucodeToolDefs() []ai.ToolDef {
	return []ai.ToolDef{
		{
			Name:        toolGetSchema,
			Description: "Inspect the project's data model. With no arguments it returns the list of existing table slugs. With table_slug it returns that table's field slugs. Call this before creating objects to avoid duplicates and to reuse what already exists.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_slug": map[string]any{
						"type":        "string",
						"description": "Optional. When set, returns the fields of this table instead of the table list.",
					},
				},
			},
		},
		{
			Name:        toolCreateTable,
			Description: "Create a new table. It automatically appears in the project menu. Returns the table id; no-op (status=already_exists) if a table with this slug exists.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"slug":  map[string]any{"type": "string", "description": "snake_case identifier, e.g. \"order_item\"."},
					"label": map[string]any{"type": "string", "description": "Human-readable name, e.g. \"Order Item\"."},
					"menu_id": map[string]any{
						"type":        "string",
						"description": "Optional. Place the table under this menu folder (a menu_id returned by create_menu). Defaults to the main menu.",
					},
				},
				"required": []string{"slug", "label"},
			},
		},
		{
			Name:        toolCreateLoginTable,
			Description: "Create a table whose rows are end-users that can authenticate (sign in) to the app. The platform automatically adds the authentication columns (password plus the chosen identifier fields) — do NOT add them with create_field. Optionally provide client_type_name to also create a new audience (client_type) bound to this table together with a role, so the table is immediately usable for sign-in. No-op (status=already_exists) if the slug exists.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"slug":  map[string]any{"type": "string", "description": "snake_case identifier, e.g. \"customer\"."},
					"label": map[string]any{"type": "string", "description": "Human-readable name, e.g. \"Customer\"."},
					"login_strategy": map[string]any{
						"type":        "array",
						"items":       map[string]any{"type": "string", "enum": ucodeLoginStrategies},
						"description": "How users sign in. Any of \"login\", \"email\", \"phone\" (you may combine several). Defaults to [\"login\"] when omitted.",
					},
					"client_type_name": map[string]any{
						"type":        "string",
						"description": "Optional. When set, also creates a new audience (client_type) bound to this table plus a role — use it when the user is introducing a distinct group of users (e.g. customers separate from admins). Omit it if the audience already exists.",
					},
					"role_name": map[string]any{
						"type":        "string",
						"description": "Optional role name for the created client_type. Defaults to client_type_name. Ignored unless client_type_name is set.",
					},
					"menu_id": map[string]any{
						"type":        "string",
						"description": "Optional. Place the table under this menu folder. Defaults to the main menu.",
					},
				},
				"required": []string{"slug", "label"},
			},
		},
		{
			Name:        toolCreateField,
			Description: "Add a field (column) to an existing table. No-op (status=already_exists) if the field already exists. To link tables use create_relation instead of a field.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_slug": map[string]any{"type": "string", "description": "Slug of the table to add the field to."},
					"slug":       map[string]any{"type": "string", "description": "snake_case field identifier."},
					"label":      map[string]any{"type": "string", "description": "Human-readable field name."},
					"type": map[string]any{
						"type":        "string",
						"enum":        ucodeFieldTypes,
						"description": "Field type. Defaults to SINGLE_LINE when omitted.",
					},
				},
				"required": []string{"table_slug", "slug", "type"},
			},
		},
		{
			Name:        toolCreateRelation,
			Description: "Create a many-to-one relation: each row of table_from references one row of table_to. This adds a foreign-key field ({table_to}_id) on table_from. Both tables must already exist.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_from": map[string]any{"type": "string", "description": "The 'many' side that gets the foreign key (e.g. order_item)."},
					"table_to":   map[string]any{"type": "string", "description": "The 'one' side being referenced (e.g. product)."},
				},
				"required": []string{"table_from", "table_to"},
			},
		},
		{
			Name:        toolCreateMenu,
			Description: "Create a menu folder to group tables under. Returns its menu_id, which you can pass to create_table's menu_id to nest tables inside the folder.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"label": map[string]any{"type": "string", "description": "Folder name shown in the menu."},
					"icon":  map[string]any{"type": "string", "description": "Optional Lucide icon name, e.g. \"package\"."},
				},
				"required": []string{"label"},
			},
		},
		{
			Name:        toolInsertItems,
			Description: "Insert one or more records into a table. Each item is an object of field_slug → value. Returns the created record guids (use them to wire foreign keys in later inserts).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_slug": map[string]any{"type": "string", "description": "Slug of the target table."},
					"items": map[string]any{
						"type":        "array",
						"description": "Records to insert.",
						"items":       map[string]any{"type": "object", "additionalProperties": true},
					},
				},
				"required": []string{"table_slug", "items"},
			},
		},
		{
			Name:        toolCountItems,
			Description: "Count how many records a table holds, optionally filtered. Use this to answer \"how many …\" questions. Returns the total count.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_slug": map[string]any{"type": "string", "description": "Slug of the table to count."},
					"search": map[string]any{
						"type":        "string",
						"description": "Optional free-text search across the table's searchable text fields (case-insensitive). The count reflects it.",
					},
					"filters": ucodeFilterSchema(),
				},
				"required": []string{"table_slug"},
			},
		},
		{
			Name:        toolListItems,
			Description: "Read records from a table, optionally filtered, searched, sorted and paginated. Use this to answer \"show me / which …\" questions. Returns the matching records plus the total count of all matches (limit/offset do not change the count). Foreign keys appear as ids unless include_relations is set.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_slug": map[string]any{"type": "string", "description": "Slug of the table to read."},
					"offset": map[string]any{
						"type":        "integer",
						"description": "Number of records to skip for pagination (default 0). Combine with limit to page through results.",
					},
					"search": map[string]any{
						"type":        "string",
						"description": "Optional free-text search across the table's searchable text fields (case-insensitive).",
					},
					"sort_by": map[string]any{
						"type":        "string",
						"description": "Optional field slug to sort by. Defaults to newest first. Use with sort_dir and limit for \"top / most / highest\" questions.",
					},
					"sort_dir": map[string]any{
						"type":        "string",
						"enum":        []string{"asc", "desc"},
						"description": "Sort direction for sort_by. Defaults to desc.",
					},
					"include_relations": map[string]any{
						"type":        "boolean",
						"description": "When true, each foreign-key field also returns the linked row as \"<field>_data\", resolving related data (e.g. the customer behind an order) instead of just its id. Defaults to false.",
					},
					"filters": ucodeFilterSchema(),
					"limit": map[string]any{
						"type":        "integer",
						"description": fmt.Sprintf("Maximum records to return (default %d, max %d).", ucodeListDefaultLimit, ucodeListMaxLimit),
					},
				},
				"required": []string{"table_slug"},
			},
		},
		{
			Name:        toolAggregateItems,
			Description: "Compute an aggregate over a table: COUNT, SUM, AVG, MIN or MAX of a field, optionally grouped by another field and filtered. Use this for totals, averages and breakdowns (e.g. \"total revenue\", \"balance by customer type\"). Returns one result row per group (or a single row when not grouped).",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"table_slug": map[string]any{"type": "string", "description": "Slug of the table to aggregate."},
					"function": map[string]any{
						"type":        "string",
						"enum":        ucodeAggregateFuncs,
						"description": "Aggregate function. COUNT may omit field (counts rows); SUM/AVG/MIN/MAX require a numeric field.",
					},
					"field":    map[string]any{"type": "string", "description": "Field slug to aggregate. Required for SUM/AVG/MIN/MAX, optional for COUNT."},
					"group_by": map[string]any{"type": "string", "description": "Optional field slug to group the results by."},
					"filters":  ucodeFilterSchema(),
				},
				"required": []string{"table_slug", "function"},
			},
		},
	}
}

// ucodeFilterSchema is the shared schema for the read tools' optional filters: an
// object keyed by field slug. Values are matched for equality, or use a comparison
// object such as {"$gte": 100} with the operators $gt, $gte, $lt, $lte, $in.
func ucodeFilterSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"description":          `Optional filters keyed by field slug. A plain value matches that field: text fields match case-insensitively by substring (e.g. {"name": "iva"} finds "Ivan"), id/number/boolean fields match exactly. A list value [a,b] matches any of them (IN). A comparison object {"$gte": 100} supports $gt, $gte, $lt, $lte and $in for ranges.`,
		"additionalProperties": true,
	}
}

func (s *ucodeChatSession) executeTool(ctx context.Context, call ai.ToolCall) (string, bool) {
	switch call.Name {
	case toolGetSchema:
		return s.toolGetSchema(ctx, call)
	case toolCreateTable:
		return s.toolCreateTable(ctx, call)
	case toolCreateLoginTable:
		return s.toolCreateLoginTable(ctx, call)
	case toolCreateField:
		return s.toolCreateField(ctx, call)
	case toolCreateRelation:
		return s.toolCreateRelation(ctx, call)
	case toolCreateMenu:
		return s.toolCreateMenu(ctx, call)
	case toolInsertItems:
		return s.toolInsertItems(ctx, call)
	case toolCountItems:
		return s.toolCountItems(ctx, call)
	case toolListItems:
		return s.toolListItems(ctx, call)
	case toolAggregateItems:
		return s.toolAggregateItems(ctx, call)
	default:
		return fmt.Sprintf("error: unknown tool %q", call.Name), true
	}
}

// ── schema cache ────────────────────────────────────────────────────────────────

func (s *ucodeChatSession) ensureTablesLoaded(ctx context.Context) error {
	if s.tablesLoaded {
		return nil
	}
	resp, err := s.service.GoObjectBuilderService().Table().GetAll(ctx, &nb.GetAllTablesRequest{
		ProjectId: s.resourceEnvId,
		Limit:     1000,
	})
	if err != nil {
		return err
	}
	for _, t := range resp.GetTables() {
		s.tableIDs[t.GetSlug()] = t.GetId()
	}
	s.tablesLoaded = true
	return nil
}

func (s *ucodeChatSession) ensureFieldsLoaded(ctx context.Context, tableSlug string) error {
	if _, ok := s.fieldSets[tableSlug]; ok {
		return nil
	}
	resp, err := s.service.GoObjectBuilderService().Field().GetAll(ctx, &nb.GetAllFieldsRequest{
		TableSlug: tableSlug,
		TableId:   s.tableIDs[tableSlug],
		ProjectId: s.resourceEnvId,
		Limit:     500,
	})
	if err != nil {
		return err
	}
	set := make(map[string]bool, len(resp.GetFields()))
	for _, f := range resp.GetFields() {
		set[f.GetSlug()] = true
	}
	s.fieldSets[tableSlug] = set
	return nil
}

// ── tool implementations ────────────────────────────────────────────────────────

func (s *ucodeChatSession) toolGetSchema(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableSlug := normalizeSlug(cast.ToString(call.Input["table_slug"]))
	if tableSlug != "" {
		s.emit.Emit(SSEEvent{
			Type:    EvProgress,
			Icon:    IconScanSearch,
			Message: "Читаю поля таблицы",
			Value:   tableSlug,
			Data:    UcodeStepData{Action: StepActionSchema, Status: StepStatusDone, Table: tableSlug},
		})
	} else {
		s.emit.Emit(SSEEvent{
			Type:    EvProgress,
			Icon:    IconScanSearch,
			Message: "Читаю схему проекта",
			Data:    UcodeStepData{Action: StepActionSchema, Status: StepStatusDone},
		})
	}

	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "error: " + err.Error(), true
	}

	if tableSlug != "" {
		if _, ok := s.tableIDs[tableSlug]; !ok {
			return marshalToolResult(map[string]any{"exists": false, "table_slug": tableSlug}), false
		}
		if err := s.ensureFieldsLoaded(ctx, tableSlug); err != nil {
			return "error: " + err.Error(), true
		}
		fields := make([]string, 0, len(s.fieldSets[tableSlug]))
		for slug := range s.fieldSets[tableSlug] {
			fields = append(fields, slug)
		}
		return marshalToolResult(map[string]any{
			"exists":     true,
			"table_slug": tableSlug,
			"fields":     sortStrings(fields),
		}), false
	}

	slugs := make([]string, 0, len(s.tableIDs))
	for slug := range s.tableIDs {
		slugs = append(slugs, slug)
	}
	return marshalToolResult(map[string]any{"tables": sortStrings(slugs), "count": len(slugs)}), false
}

func (s *ucodeChatSession) toolCreateTable(ctx context.Context, call ai.ToolCall) (string, bool) {
	slug := normalizeSlug(cast.ToString(call.Input["slug"]))
	if slug == "" {
		return "error: slug is required", true
	}
	label := strings.TrimSpace(cast.ToString(call.Input["label"]))
	if label == "" {
		label = slugToLabel(slug)
	}

	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "error: " + err.Error(), true
	}
	if id, exists := s.tableIDs[slug]; exists {
		s.emit.Emit(SSEEvent{
			Type:    EvTableDone,
			Icon:    IconDatabase,
			Message: "Таблица уже существует",
			Value:   label,
			Data:    UcodeStepData{Action: StepActionTable, Status: StepStatusSkipped, Table: slug, Label: label, Reason: "already_exists"},
		})
		return marshalToolResult(map[string]any{"status": "already_exists", "table_slug": slug, "table_id": id}), false
	}

	menuID := config.MainMenuID
	if mid := strings.TrimSpace(cast.ToString(call.Input["menu_id"])); mid != "" {
		menuID = mid
	}

	attrs, err := helper.ConvertMapToStruct(map[string]any{"label": "", "label_en": label})
	if err != nil {
		return "error: " + err.Error(), true
	}

	s.emit.Emit(SSEEvent{
		Type:    EvTableStart,
		Icon:    IconDatabase,
		Message: "Создаю таблицу",
		Value:   label,
		Data:    UcodeStepData{Action: StepActionTable, Status: StepStatusStarted, Table: slug, Label: label},
	})

	resp, err := s.service.GoObjectBuilderService().Table().Create(ctx, &nb.CreateTableRequest{
		Label:          label,
		Slug:           slug,
		ProjectId:      s.resourceEnvId,
		EnvId:          s.envId,
		MenuId:         menuID,
		ViewId:         uuid.NewString(),
		LayoutId:       uuid.NewString(),
		ShowInMenu:     true,
		Attributes:     attrs,
		UcodeProjectId: s.projectId,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	s.tableIDs[slug] = resp.GetId()
	// A freshly created table has only system fields; mark its user-field set as
	// loaded-and-empty so later create_field calls skip a redundant fetch.
	s.fieldSets[slug] = map[string]bool{}
	s.stats.Tables++

	s.emit.Emit(SSEEvent{
		Type:    EvTableDone,
		Icon:    IconDatabase,
		Message: "Таблица создана",
		Value:   label,
		Data:    UcodeStepData{Action: StepActionTable, Status: StepStatusDone, Table: slug, Label: label},
	})

	return marshalToolResult(map[string]any{"status": "created", "table_slug": slug, "table_id": resp.GetId()}), false
}

// toolCreateLoginTable creates a table marked is_login_table so its rows can
// authenticate. The object builder auto-creates the auth columns from
// auth_info.login_strategy, so this tool never creates login/email/phone/password
// fields itself. When client_type_name is given it also provisions a new audience
// (client_type bound to this table) plus a role via the auth service, mirroring the
// extra-client-type path of createBackendFromPlan. A failure to wire the audience is
// non-fatal: the login table already exists and the error is reported back to the AI.
func (s *ucodeChatSession) toolCreateLoginTable(ctx context.Context, call ai.ToolCall) (string, bool) {
	slug := normalizeSlug(cast.ToString(call.Input["slug"]))
	if slug == "" {
		return "error: slug is required", true
	}
	label := strings.TrimSpace(cast.ToString(call.Input["label"]))
	if label == "" {
		label = slugToLabel(slug)
	}

	strategy, err := normalizeLoginStrategy(call.Input["login_strategy"])
	if err != nil {
		return "error: " + err.Error(), true
	}

	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "error: " + err.Error(), true
	}
	if id, exists := s.tableIDs[slug]; exists {
		s.emit.Emit(SSEEvent{
			Type:    EvTableDone,
			Icon:    IconShield,
			Message: "Таблица входа уже существует",
			Value:   label,
			Data:    UcodeStepData{Action: StepActionLogin, Status: StepStatusSkipped, Table: slug, Label: label, Reason: "already_exists"},
		})
		return marshalToolResult(map[string]any{"status": "already_exists", "table_slug": slug, "table_id": id}), false
	}

	menuID := config.MainMenuID
	if mid := strings.TrimSpace(cast.ToString(call.Input["menu_id"])); mid != "" {
		menuID = mid
	}

	attrs, err := helper.ConvertMapToStruct(map[string]any{
		"label":     "",
		"label_en":  label,
		"auth_info": map[string]any{"login_strategy": strategy},
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	s.emit.Emit(SSEEvent{
		Type:    EvTableStart,
		Icon:    IconShield,
		Message: "Создаю таблицу входа",
		Value:   label,
		Data:    UcodeStepData{Action: StepActionLogin, Status: StepStatusStarted, Table: slug, Label: label, Strategy: strings.Join(strategy, ", ")},
	})

	resp, err := s.service.GoObjectBuilderService().Table().Create(ctx, &nb.CreateTableRequest{
		Label:          label,
		Slug:           slug,
		ProjectId:      s.resourceEnvId,
		EnvId:          s.envId,
		MenuId:         menuID,
		ViewId:         uuid.NewString(),
		LayoutId:       uuid.NewString(),
		ShowInMenu:     true,
		Attributes:     attrs,
		IsLoginTable:   true,
		UcodeProjectId: s.projectId,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	s.tableIDs[slug] = resp.GetId()
	// The object builder auto-creates the auth columns; drop any cached field set so
	// a later get_schema reload reflects the real columns instead of an empty set.
	delete(s.fieldSets, slug)
	s.stats.LoginTables++

	s.emit.Emit(SSEEvent{
		Type:    EvTableDone,
		Icon:    IconShield,
		Message: "Таблица входа создана",
		Value:   label,
		Data:    UcodeStepData{Action: StepActionLogin, Status: StepStatusDone, Table: slug, Label: label, Strategy: strings.Join(strategy, ", ")},
	})

	result := map[string]any{
		"status":         "created",
		"table_slug":     slug,
		"table_id":       resp.GetId(),
		"login_strategy": strategy,
	}

	clientTypeName := strings.TrimSpace(cast.ToString(call.Input["client_type_name"]))
	if clientTypeName == "" {
		return marshalToolResult(result), false
	}

	authSvc := s.service.AuthService()
	clientTypeID, ctErr := createClientType(ctx, clientTypeName, s.projectId, s.resourceEnvId, slug, s.nodeType, s.resourceType, authSvc)
	if ctErr != nil {
		result["client_type_error"] = ctErr.Error()
		return marshalToolResult(result), false
	}
	s.stats.ClientTypes++
	result["client_type_id"] = clientTypeID
	result["client_type_name"] = clientTypeName

	s.emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconUsers,
		Message: "Создан тип клиента",
		Value:   clientTypeName,
		Data:    UcodeStepData{Action: StepActionLogin, Status: StepStatusDone, Table: slug, ClientType: clientTypeName},
	})

	roleName := strings.TrimSpace(cast.ToString(call.Input["role_name"]))
	if roleName == "" {
		roleName = clientTypeName
	}
	if clientTypeID != "" {
		if roleErr := createRole(ctx, roleName, clientTypeID, s.resourceEnvId, s.nodeType, s.resourceType, authSvc); roleErr != nil {
			result["role_error"] = roleErr.Error()
		} else {
			s.stats.Roles++
			result["role_name"] = roleName
			s.emit.Emit(SSEEvent{
				Type:    EvProgress,
				Icon:    IconShieldCheck,
				Message: "Создана роль",
				Value:   roleName,
				Data:    UcodeStepData{Action: StepActionLogin, Status: StepStatusDone, Table: slug, Role: roleName},
			})
		}
	}

	return marshalToolResult(result), false
}

// normalizeLoginStrategy validates and de-duplicates the requested sign-in
// strategies, accepting a JSON array, a single string or nil. It defaults to
// ["login"] and rejects any value outside login/email/phone.
func normalizeLoginStrategy(raw any) ([]string, error) {
	var candidates []string
	switch v := raw.(type) {
	case string:
		if t := strings.TrimSpace(v); t != "" {
			candidates = append(candidates, t)
		}
	case []any:
		for _, item := range v {
			if t := strings.TrimSpace(cast.ToString(item)); t != "" {
				candidates = append(candidates, t)
			}
		}
	case []string:
		candidates = append(candidates, v...)
	}

	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		c = strings.ToLower(strings.TrimSpace(c))
		if !ucodeLoginStrategySet[c] {
			return nil, fmt.Errorf("invalid login_strategy %q; allowed: %s", c, strings.Join(ucodeLoginStrategies, ", "))
		}
		if !seen[c] {
			seen[c] = true
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return append([]string(nil), ucodeDefaultLoginStrat...), nil
	}
	return out, nil
}

func (s *ucodeChatSession) toolCreateField(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableSlug := normalizeSlug(cast.ToString(call.Input["table_slug"]))
	slug := normalizeSlug(cast.ToString(call.Input["slug"]))
	if tableSlug == "" || slug == "" {
		return "error: table_slug and slug are required", true
	}
	label := strings.TrimSpace(cast.ToString(call.Input["label"]))
	if label == "" {
		label = slugToLabel(slug)
	}

	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "error: " + err.Error(), true
	}
	tableID, ok := s.tableIDs[tableSlug]
	if !ok {
		return fmt.Sprintf("error: table %q does not exist — create it first", tableSlug), true
	}
	if err := s.ensureFieldsLoaded(ctx, tableSlug); err != nil {
		return "error: " + err.Error(), true
	}
	if s.fieldSets[tableSlug][slug] {
		s.emit.Emit(SSEEvent{
			Type:    EvProgress,
			Icon:    IconColumns,
			Message: fmt.Sprintf("Поле уже есть в %s", tableSlug),
			Value:   label,
			Data:    UcodeStepData{Action: StepActionField, Status: StepStatusSkipped, Table: tableSlug, Field: slug, Label: label, Reason: "already_exists"},
		})
		return marshalToolResult(map[string]any{"status": "already_exists", "table_slug": tableSlug, "field_slug": slug}), false
	}

	fieldType := mapFieldType(cast.ToString(call.Input["type"]))
	attrs, err := helper.ConvertMapToStruct(map[string]any{"label": "", "label_en": label})
	if err != nil {
		return "error: " + err.Error(), true
	}

	if _, err = s.service.GoObjectBuilderService().Field().Create(ctx, &nb.CreateFieldRequest{
		Id:         uuid.NewString(),
		TableId:    tableID,
		Label:      label,
		Slug:       slug,
		Type:       fieldType,
		Attributes: attrs,
		ProjectId:  s.resourceEnvId,
		Index:      "string",
		IsVisible:  true,
	}); err != nil {
		return "error: " + err.Error(), true
	}

	s.fieldSets[tableSlug][slug] = true
	s.stats.Fields++

	s.emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconColumns,
		Message: fmt.Sprintf("Добавлено поле в %s", tableSlug),
		Value:   label,
		Data:    UcodeStepData{Action: StepActionField, Status: StepStatusDone, Table: tableSlug, Field: slug, FieldType: fieldType, Label: label},
	})

	return marshalToolResult(map[string]any{"status": "created", "table_slug": tableSlug, "field_slug": slug, "type": fieldType}), false
}

func (s *ucodeChatSession) toolCreateRelation(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableFrom := normalizeSlug(cast.ToString(call.Input["table_from"]))
	tableTo := normalizeSlug(cast.ToString(call.Input["table_to"]))
	if tableFrom == "" || tableTo == "" {
		return "error: table_from and table_to are required", true
	}

	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "error: " + err.Error(), true
	}
	if _, ok := s.tableIDs[tableFrom]; !ok {
		return fmt.Sprintf("error: table %q does not exist", tableFrom), true
	}
	if _, ok := s.tableIDs[tableTo]; !ok {
		return fmt.Sprintf("error: table %q does not exist", tableTo), true
	}

	// Relation.Create requires a non-empty ViewFields; ObtainRandomOne yields a
	// real field id from table_from to satisfy it (proven in createBackendFromPlan).
	viewField, err := s.service.GoObjectBuilderService().Field().ObtainRandomOne(ctx, &nb.ObtainRandomRequest{
		TableSlug: tableFrom,
		ProjectId: s.resourceEnvId,
		EnvId:     s.envId,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	relFieldSlug := tableTo + "_id"
	attrs, err := helper.ConvertMapToStruct(map[string]any{
		"label_en":    slugToLabel(tableTo),
		"label_to_en": slugToLabel(tableFrom),
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	if _, err = s.service.GoObjectBuilderService().Relation().Create(ctx, &nb.CreateRelationRequest{
		Id:                uuid.NewString(),
		TableFrom:         tableFrom,
		TableTo:           tableTo,
		Type:              "Many2One",
		RelationTableSlug: tableTo,
		RelationFieldSlug: relFieldSlug,
		RelationFieldId:   uuid.NewString(),
		RelationToFieldId: uuid.NewString(),
		ProjectId:         s.resourceEnvId,
		EnvId:             s.envId,
		ViewFields:        []string{viewField.GetId()},
		Attributes:        attrs,
	}); err != nil {
		return "error: " + err.Error(), true
	}

	// The relation adds the FK field on table_from; reflect it in the cache.
	if set, ok := s.fieldSets[tableFrom]; ok {
		set[relFieldSlug] = true
	}
	s.stats.Relations++

	s.emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconLink,
		Message: "Создана связь",
		Value:   fmt.Sprintf("%s → %s", tableFrom, tableTo),
		Data:    UcodeStepData{Action: StepActionRelation, Status: StepStatusDone, TableFrom: tableFrom, TableTo: tableTo, ForeignKey: relFieldSlug},
	})

	return marshalToolResult(map[string]any{
		"status":      "created",
		"table_from":  tableFrom,
		"table_to":    tableTo,
		"foreign_key": relFieldSlug,
	}), false
}

func (s *ucodeChatSession) toolCreateMenu(ctx context.Context, call ai.ToolCall) (string, bool) {
	label := strings.TrimSpace(cast.ToString(call.Input["label"]))
	if label == "" {
		return "error: label is required", true
	}
	icon := strings.TrimSpace(cast.ToString(call.Input["icon"]))

	attrs, err := helper.ConvertMapToStruct(map[string]any{"label_en": label})
	if err != nil {
		return "error: " + err.Error(), true
	}

	menuID := uuid.NewString()

	menu, err := s.service.GoObjectBuilderService().Menu().Create(ctx, &nb.CreateMenuRequest{
		Id:         menuID,
		Label:      label,
		Icon:       icon,
		Type:       "FOLDER",
		ProjectId:  s.resourceEnvId,
		EnvId:      s.envId,
		ParentId:   config.MainMenuID,
		IsVisible:  true,
		Attributes: attrs,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}
	if id := menu.GetId(); id != "" {
		menuID = id
	}
	s.stats.Menus++

	s.emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconFolder,
		Message: "Создан раздел меню",
		Value:   label,
		Data:    UcodeStepData{Action: StepActionMenu, Status: StepStatusDone, MenuID: menuID, Label: label},
	})

	return marshalToolResult(map[string]any{"status": "created", "menu_id": menuID, "label": label}), false
}

func (s *ucodeChatSession) toolInsertItems(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableSlug := normalizeSlug(cast.ToString(call.Input["table_slug"]))
	if tableSlug == "" {
		return "error: table_slug is required", true
	}
	rawItems, ok := call.Input["items"].([]any)
	if !ok || len(rawItems) == 0 {
		return "error: items is required and must be a non-empty array of objects", true
	}

	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "error: " + err.Error(), true
	}
	if _, ok := s.tableIDs[tableSlug]; !ok {
		return fmt.Sprintf("error: table %q does not exist", tableSlug), true
	}

	var (
		created  int
		guids    []string
		failures []string
	)
	for i, raw := range rawItems {
		item, ok := raw.(map[string]any)
		if !ok {
			failures = append(failures, fmt.Sprintf("item[%d]: not an object", i))
			continue
		}
		structData, err := helper.ConvertMapToStruct(item)
		if err != nil {
			failures = append(failures, fmt.Sprintf("item[%d]: %v", i, err))
			continue
		}
		resp, err := s.service.GoObjectBuilderService().Items().Create(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			ProjectId: s.resourceEnvId,
			Data:      structData,
		})
		if err != nil {
			failures = append(failures, fmt.Sprintf("item[%d]: %v", i, err))
			continue
		}
		created++
		if m, convErr := helper.ConvertStructToMap(resp.GetData()); convErr == nil {
			if g, _ := m["guid"].(string); g != "" {
				guids = append(guids, g)
			}
		}
	}

	s.stats.Items += created

	s.emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconPlusCircle,
		Message: fmt.Sprintf("Добавлены записи в %s", tableSlug),
		Value:   fmt.Sprintf("%d из %d", created, len(rawItems)),
		Data:    UcodeStepData{Action: StepActionItems, Status: StepStatusDone, Table: tableSlug, Created: created, Failed: len(failures), Total: len(rawItems)},
	})

	result := map[string]any{"status": "completed", "table_slug": tableSlug, "created": created, "guids": guids}
	if len(failures) > 0 {
		result["errors"] = failures
	}

	return marshalToolResult(result), created == 0 && len(failures) > 0
}

// ── read-only data tools ──────────────────────────────────────────────────────
//
// These let the builder answer questions about the project's actual records
// (counts, samples, totals) without ever modifying data. They are read-only by
// construction: count_items / list_items go through the parameterized GetList2
// path (field names are whitelisted server-side, values are bound parameters),
// and aggregate_items builds its SQL fragments here in Go from columns that are
// validated against the real schema first — the model never authors raw SQL.

func (s *ucodeChatSession) toolCountItems(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableSlug, errStr, isErr := s.resolveReadTable(ctx, cast.ToString(call.Input["table_slug"]))
	if isErr {
		return errStr, true
	}
	filters := asStringMap(call.Input["filters"])
	if msg, bad := s.validateReadFields(ctx, tableSlug, mapKeys(filters)); bad {
		return msg, true
	}
	search := strings.TrimSpace(cast.ToString(call.Input["search"]))

	s.emitReadProgress(tableSlug, "Считаю записи")

	listData, err := s.getList(ctx, tableSlug, readQuery{filters: filters, search: search, limit: 1})
	if err != nil {
		return "error: " + err.Error(), true
	}

	result := map[string]any{"table_slug": tableSlug, "count": extractCountFromData(listData)}
	if len(filters) > 0 {
		result["filters"] = filters
	}
	if search != "" {
		result["search"] = search
	}
	return marshalToolResult(result), false
}

func (s *ucodeChatSession) toolListItems(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableSlug, errStr, isErr := s.resolveReadTable(ctx, cast.ToString(call.Input["table_slug"]))
	if isErr {
		return errStr, true
	}
	filters := asStringMap(call.Input["filters"])
	sortField := normalizeSlug(cast.ToString(call.Input["sort_by"]))

	toCheck := mapKeys(filters)
	if sortField != "" {
		toCheck = append(toCheck, sortField)
	}
	if msg, bad := s.validateReadFields(ctx, tableSlug, toCheck); bad {
		return msg, true
	}

	limit := cast.ToInt(call.Input["limit"])
	if limit <= 0 {
		limit = ucodeListDefaultLimit
	}
	if limit > ucodeListMaxLimit {
		limit = ucodeListMaxLimit
	}

	offset := cast.ToInt(call.Input["offset"])
	if offset < 0 {
		offset = 0
	}

	search := strings.TrimSpace(cast.ToString(call.Input["search"]))
	sortAsc := strings.EqualFold(strings.TrimSpace(cast.ToString(call.Input["sort_dir"])), "asc")
	withRelations := cast.ToBool(call.Input["include_relations"])

	s.emitReadProgress(tableSlug, "Читаю записи")

	listData, err := s.getList(ctx, tableSlug, readQuery{
		filters:       filters,
		limit:         limit,
		offset:        offset,
		search:        search,
		sortField:     sortField,
		sortAsc:       sortAsc,
		withRelations: withRelations,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	items := extractItemsFromData(listData)
	result := map[string]any{
		"table_slug": tableSlug,
		"count":      extractCountFromData(listData),
		"returned":   len(items),
		"items":      items,
	}
	if offset > 0 {
		result["offset"] = offset
	}
	return marshalToolResult(result), false
}

func (s *ucodeChatSession) toolAggregateItems(ctx context.Context, call ai.ToolCall) (string, bool) {
	tableSlug, errStr, isErr := s.resolveReadTable(ctx, cast.ToString(call.Input["table_slug"]))
	if isErr {
		return errStr, true
	}

	function := strings.ToUpper(strings.TrimSpace(cast.ToString(call.Input["function"])))
	if !ucodeAggregateFuncSet[function] {
		return fmt.Sprintf("error: function must be one of %s", strings.Join(ucodeAggregateFuncs, ", ")), true
	}

	field := normalizeSlug(cast.ToString(call.Input["field"]))
	groupBy := normalizeSlug(cast.ToString(call.Input["group_by"]))
	filters := asStringMap(call.Input["filters"])

	if function != "COUNT" && field == "" {
		return fmt.Sprintf("error: field is required for %s", function), true
	}

	// Validate every referenced column against the real schema before it is ever
	// formatted into a SQL fragment — this is what keeps the raw aggregation safe.
	toCheck := mapKeys(filters)
	if field != "" {
		toCheck = append(toCheck, field)
	}
	if groupBy != "" {
		toCheck = append(toCheck, groupBy)
	}
	if msg, bad := s.validateReadFields(ctx, tableSlug, toCheck); bad {
		return msg, true
	}

	s.emitReadProgress(tableSlug, "Считаю агрегат")

	aggExpr := "COUNT(*) AS result"
	if field != "" {
		aggExpr = fmt.Sprintf(`%s("%s") AS result`, function, field)
	}

	columns := []string{aggExpr}
	queryParams := map[string]any{
		"operation": "SELECT",
		"table":     fmt.Sprintf(`"%s"`, tableSlug),
		"where":     buildWhereClause(filters),
	}
	if groupBy != "" {
		quoted := fmt.Sprintf(`"%s"`, groupBy)
		columns = append([]string{quoted}, columns...)
		queryParams["group_by"] = []string{quoted}
	}
	queryParams["columns"] = columns

	structData, err := helper.ConvertMapToStruct(queryParams)
	if err != nil {
		return "error: " + err.Error(), true
	}
	resp, err := s.service.GoObjectBuilderService().ObjectBuilder().GetListAggregation(ctx, &nb.CommonMessage{
		TableSlug: tableSlug,
		ProjectId: s.resourceEnvId,
		Data:      structData,
	})
	if err != nil {
		return "error: " + err.Error(), true
	}

	aggData, _ := helper.ConvertStructToMap(resp.GetData())
	result := map[string]any{
		"table_slug": tableSlug,
		"function":   function,
		"results":    extractItemsFromData(aggData),
	}
	if field != "" {
		result["field"] = field
	}
	if groupBy != "" {
		result["group_by"] = groupBy
	}
	return marshalToolResult(result), false
}

// readQuery holds the options for a read through the parameterized GetList2 path.
// Field names in filters and sortField must already be validated against the schema
// by the caller; every value is bound as a query parameter server-side.
type readQuery struct {
	filters       map[string]any
	limit         int
	offset        int
	search        string
	sortField     string
	sortAsc       bool
	withRelations bool
}

// getList runs a read via the parameterized GetList2 path and returns the decoded
// response map ({response, count}). It only assembles the request payload GetListV2
// understands — filtering, search, sorting and pagination all happen server-side.
func (s *ucodeChatSession) getList(ctx context.Context, tableSlug string, q readQuery) (map[string]any, error) {
	dataMap := make(map[string]any, len(q.filters)+5)
	for k, v := range q.filters {
		dataMap[k] = v
	}
	dataMap["limit"] = q.limit
	dataMap["offset"] = q.offset
	dataMap["with_relations"] = q.withRelations
	if q.search != "" {
		dataMap["search"] = q.search
	}
	// created_at is the builder's default sort and is skipped by its ORDER BY logic,
	// so emitting it alone yields a dangling "ORDER BY"; only set an explicit order
	// for other fields and otherwise keep the default newest-first ordering.
	if q.sortField != "" && q.sortField != "created_at" {
		dir := -1
		if q.sortAsc {
			dir = 1
		}
		dataMap["order"] = map[string]any{q.sortField: dir}
	}

	structData, err := helper.ConvertMapToStruct(dataMap)
	if err != nil {
		return nil, err
	}
	resp, err := s.service.GoObjectBuilderService().ObjectBuilder().GetList2(ctx, &nb.CommonMessage{
		TableSlug: tableSlug,
		ProjectId: s.resourceEnvId,
		Data:      structData,
	})
	if err != nil {
		return nil, err
	}
	listData, _ := helper.ConvertStructToMap(resp.GetData())
	return listData, nil
}

// resolveReadTable normalizes the slug, warms the table cache and confirms the
// table exists. On failure it returns the tool-error string and isErr=true.
func (s *ucodeChatSession) resolveReadTable(ctx context.Context, raw string) (tableSlug, errStr string, isErr bool) {
	tableSlug = normalizeSlug(raw)
	if tableSlug == "" {
		return "", "error: table_slug is required", true
	}
	if err := s.ensureTablesLoaded(ctx); err != nil {
		return "", "error: " + err.Error(), true
	}
	if _, ok := s.tableIDs[tableSlug]; !ok {
		return "", fmt.Sprintf("error: table %q does not exist", tableSlug), true
	}
	return tableSlug, "", false
}

// validateReadFields rejects any referenced column that is neither a user-defined
// field of the table nor a system column. This whitelist is the safety boundary
// for the aggregation path, where column names are formatted into raw SQL.
func (s *ucodeChatSession) validateReadFields(ctx context.Context, tableSlug string, fields []string) (errStr string, isErr bool) {
	wanted := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.TrimSpace(f); f != "" {
			wanted = append(wanted, f)
		}
	}
	if len(wanted) == 0 {
		return "", false
	}
	if err := s.ensureFieldsLoaded(ctx, tableSlug); err != nil {
		return "error: " + err.Error(), true
	}
	for _, f := range wanted {
		if ucodeSystemFields[f] || s.fieldSets[tableSlug][f] {
			continue
		}
		return fmt.Sprintf("error: unknown field %q on table %q; known fields: %s",
			f, tableSlug, strings.Join(s.knownFields(tableSlug), ", ")), true
	}
	return "", false
}

// knownFields returns the sorted set of columns valid for filtering/aggregation:
// the table's user fields plus the always-present system columns.
func (s *ucodeChatSession) knownFields(tableSlug string) []string {
	out := make([]string, 0, len(s.fieldSets[tableSlug])+len(ucodeSystemFields))
	for slug := range s.fieldSets[tableSlug] {
		out = append(out, slug)
	}
	for slug := range ucodeSystemFields {
		out = append(out, slug)
	}
	return sortStrings(out)
}

func (s *ucodeChatSession) emitReadProgress(tableSlug, message string) {
	s.emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconScanSearch,
		Message: message,
		Value:   tableSlug,
		Data:    UcodeStepData{Action: StepActionData, Status: StepStatusDone, Table: tableSlug},
	})
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func normalizeSlug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	lastUnderscore := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastUnderscore = false
		case r == ' ' || r == '-' || r == '_':
			if !lastUnderscore && b.Len() > 0 {
				b.WriteByte('_')
				lastUnderscore = true
			}
		}
	}
	return strings.Trim(b.String(), "_")
}

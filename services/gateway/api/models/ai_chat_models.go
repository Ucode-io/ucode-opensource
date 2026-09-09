package models

type (
	// ========================== Core Message Types ==========================

	ContentBlock struct {
		Type   string       `json:"type"`
		Text   string       `json:"text,omitempty"`
		Source *ImageSource `json:"source,omitempty"`
	}

	ImageSource struct {
		Type      string `json:"type" default:"url"`
		URL       string `json:"url,omitempty"`
		MediaType string `json:"media_type,omitempty"`
	}

	ChatMessage struct {
		Role    string         `json:"role"`
		Content []ContentBlock `json:"content"`
	}

	// ========================== Basic Requests ==========================

	// EnrichedMessage is the HTTP representation of a chat message.
	// It mirrors pbo.Message but adds Plan / Questions / Error fields parsed
	// from the marker-prefixed content the backend persists in chat history.
	EnrichedMessage struct {
		ID                  string       `json:"id"`
		ChatID              string       `json:"chat_id"`
		Role                string       `json:"role"`
		Content             string       `json:"content"`
		Images              []string     `json:"images"`
		HasFiles            bool         `json:"has_files"`
		TokensUsed          int32        `json:"tokens_used"`
		CreatedAt           string       `json:"created_at"`
		LikeCount           int32        `json:"like_count"`
		DislikeCount        int32        `json:"dislike_count"`
		CurrentUserReaction string       `json:"current_user_reaction"`
		Plan                *HaikuPlan   `json:"plan,omitempty"`
		Questions           []AiQuestion `json:"questions,omitempty"`
		Error               *AiChatError `json:"error,omitempty"`
	}

	// AiChatError is the structured failure payload persisted with [ERROR]
	// assistant messages and emitted in SSE EvError events. The frontend uses
	// it to render specialized failure UI; see docs/frontend/CHAT_ERROR_MESSAGES.md.
	AiChatError struct {
		Code       string `json:"code"`                  // stable identity, frontend switches on this
		Phase      string `json:"phase"`                 // pipeline phase that failed
		Message    string `json:"message"`               // user-facing summary, already localized
		Details    string `json:"details,omitempty"`     // raw error text; gate behind a toggle, never show verbatim
		Retryable  bool   `json:"retryable"`             // drives the "Retry" button
		UserAction string `json:"user_action,omitempty"` // localized next-step hint
	}

	// VisualContext is optional metadata sent by the frontend when the user
	// selected a specific UI element for visual editing.
	// All fields are optional — the backend handles whatever is provided.
	VisualContext struct {
		Path        string `json:"path,omitempty"`         // e.g. "src/components/layout/TopNav.tsx"
		Line        int    `json:"line,omitempty"`         // line number inside the file
		ElementName string `json:"element_name,omitempty"` // data-element-name value
		OuterHTML   string `json:"outer_html,omitempty"`   // element.outerHTML snapshot
	}

	NewMessageReq struct {
		Content             string          `json:"content"`
		Images              []string        `json:"images"`
		PendingAction       *PendingAction  `json:"pending_action,omitempty"`
		Context             []VisualContext `json:"context,omitempty"`
		MicrofrontendID     string          `json:"microfrontend_id,omitempty"`
		MicrofrontendRepoID string          `json:"microfrontend_repo_id,omitempty"`
		ResourceEnvId       string          `json:"resource_env_id,omitempty"`
		NewProject          bool            `json:"new_project,omitempty"`
		UcodeProjectID      string          `json:"ucode_project_id,omitempty"`
	}
	SendMessageRequest struct {
		UserPrompt    string `json:"user_prompt"`
		ChatId        string `json:"chat_id"`
		ProjectId     string `json:"project_id"`
		ResourceEnvId string `json:"resource_env_id"`
	}

	// ========================== Project File Models ==========================

	GraphNode struct {
		Path      string `json:"path"`
		FileGraph any    `json:"file_graph,omitempty"`
	}

	// ========================== Anthropic Response Models ==========================

	// ClaudeResponse — полный ответ от Anthropic API
	ClaudeResponse struct {
		Model        string         `json:"model"`
		ID           string         `json:"id"`
		Type         string         `json:"type"`
		Role         string         `json:"role"`
		Content      []ContentBlock `json:"content"`
		StopReason   string         `json:"stop_reason"`
		StopSequence *string        `json:"stop_sequence"`
		Usage        ClaudeUsage    `json:"usage"`
	}

	ClaudeUsage struct {
		InputTokens              int    `json:"input_tokens"`
		OutputTokens             int    `json:"output_tokens"`
		CacheCreationInputTokens int    `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int    `json:"cache_read_input_tokens"`
		ServiceTier              string `json:"service_tier"`
		InferenceGeo             string `json:"inference_geo"`
	}

	QuestionOption struct {
		ID    string `json:"id"`
		Label string `json:"label"`
	}

	AiQuestion struct {
		ID      string           `json:"id"`
		Title   string           `json:"title"`
		Type    string           `json:"type"` // "single" | "multi"
		Options []QuestionOption `json:"options"`
	}

	ParsedClaudeResponse struct {
		Model         string            `json:"model"`
		MessageID     string            `json:"message_id"`
		StopReason    string            `json:"stop_reason"`
		Usage         ClaudeUsage       `json:"usage"`
		Project       *GeneratedProject `json:"project,omitempty"`
		MobileProject *MobileProject    `json:"mobile_project,omitempty"`
		Description   string            `json:"description"`
		PendingAction *PendingAction    `json:"pending_action,omitempty"`
		Questions     []AiQuestion      `json:"questions,omitempty"`
		Plan          *HaikuPlan        `json:"plan,omitempty"`
	}

	MobileProject struct {
		ProjectName    string             `json:"project_name"`
		ProjectType    string             `json:"project_type"`
		Runtime        string             `json:"runtime"`
		RuntimeVersion string             `json:"runtime_version"`
		WebDir         string             `json:"web_dir"`
		Capabilities   []MobileCapability `json:"capabilities"`
		Files          []ProjectFile      `json:"files"`
	}

	// ========================== Classification ==========================

	RequestClassification struct {
		RequiresBackend  bool   `json:"requires_backend"`
		RequiresFrontend bool   `json:"requires_frontend"`
		BackendReason    string `json:"backend_reason"`
		FrontendReason   string `json:"frontend_reason"`
		Confidence       string `json:"confidence"`
	}

	// ========================== AI Agent Routing ==========================

	// PlanInfraEdge — a single directed edge in the infrastructure diagram.
	PlanInfraEdge struct {
		From  string `json:"from"`
		To    string `json:"to"`
		Label string `json:"label"`
	}

	// HaikuPlan — diagram plan returned after the user answers questionnaire questions.
	// Contains only the visual diagrams needed before code generation.
	HaikuPlan struct {
		BpmnXML      string          `json:"bpmn_xml"`
		InfraDiagram []PlanInfraEdge `json:"infra_diagram,omitempty"`
	}

	HaikuRoutingResult struct {
		NextStep       bool         `json:"next_step"`
		Intent         string       `json:"intent"`
		Reply          string       `json:"reply"`
		Clarified      string       `json:"clarified"`
		ClarifyOptions []string     `json:"clarify_options"`
		FilesNeeded    []string     `json:"files_needed"`
		HasImages      bool         `json:"has_images"`
		ProjectName    string       `json:"project_name"`
		Questions      []AiQuestion `json:"questions,omitempty"`
		Plan           *HaikuPlan   `json:"plan,omitempty"`
	}

	SonnetPlanResult struct {
		FilesToChange []FilePlan `json:"files_to_change"`
		FilesToCreate []FilePlan `json:"files_to_create"`
		Summary       string     `json:"summary"`
	}

	FilePlan struct {
		Path        string `json:"path"`
		Description string `json:"description"`
	}

	// ========================== Architect Plan ==========================
	TableFieldPlan struct {
		Slug  string `json:"slug"`
		Label string `json:"label"`
		Type  string `json:"type"` // SINGLE_LINE, NUMBER, EMAIL, PHONE, DATE, etc.
	}

	// TableRelationPlan defines a foreign-key relationship between two tables.
	// Only Many2One is supported for PostgreSQL projects.
	// The FK column is auto-created by ucode with slug "{table_to}_id"
	// (e.g. orders→customers creates column "customers_id" on orders table).
	TableRelationPlan struct {
		TableFrom string `json:"table_from"` // source table slug (the "many" side)
		TableTo   string `json:"table_to"`   // target table slug (the "one" side)
		Type      string `json:"type"`       // always "Many2One" for PostgreSQL
	}

	TablePlan struct {
		Slug          string                   `json:"slug"`
		Label         string                   `json:"label"`
		IsLoginTable  bool                     `json:"is_login_table"`
		LoginStrategy []string                 `json:"login_strategy"`
		Fields        []TableFieldPlan         `json:"fields"`
		MockData      []map[string]interface{} `json:"mock_data"`
	}

	DesignSpec struct {
		PrimaryColor         string `json:"primary_color"`
		PrimaryHSL           string `json:"primary_hsl"`
		BackgroundColor      string `json:"background_color"`
		BackgroundHSL        string `json:"background_hsl"`
		SurfaceColor         string `json:"surface_color"`
		SurfaceHSL           string `json:"surface_hsl"`
		SidebarBackground    string `json:"sidebar_background"`
		SidebarBackgroundHSL string `json:"sidebar_background_hsl"`
		SidebarForeground    string `json:"sidebar_foreground"`
		SidebarStyle         string `json:"sidebar_style"` // "dark" | "light" | "colored"
		TextColor            string `json:"text_color"`
		TextMutedColor       string `json:"text_muted_color"`
		BorderColor          string `json:"border_color"`
		AccentColor          string `json:"accent_color"`
		AccentHSL            string `json:"accent_hsl"`
		FontFamily           string `json:"font_family"` // heading font (e.g. "Syne", "Inter")
		BodyFont             string `json:"body_font"`   // body font (e.g. "DM Sans", "Inter")
		BorderRadius         string `json:"border_radius"`
		DesignInspiration    string `json:"design_inspiration"`
	}

	ReferenceScreenshot struct {
		Viewport string `json:"viewport"`
		URL      string `json:"url"`
		Width    int    `json:"width,omitempty"`
		Height   int    `json:"height,omitempty"`
	}

	ReferenceSection struct {
		Heading string `json:"heading,omitempty"`
		Copy    string `json:"copy,omitempty"`
		CTA     string `json:"cta,omitempty"`
		Layout  string `json:"layout,omitempty"`
	}

	ReferenceAsset struct {
		Type string `json:"type,omitempty"`
		URL  string `json:"url,omitempty"`
		Alt  string `json:"alt,omitempty"`
	}

	// ReferenceNavLink is one top-navigation link of the captured site — the
	// real page inventory for multi-page clones and the option source for the
	// clone questionnaire.
	ReferenceNavLink struct {
		Label string `json:"label"`
		URL   string `json:"url"`
	}

	// ReferencePage is a crawled subpage of the reference site so non-home
	// pages are cloned from their own captured evidence instead of invented in
	// home-page style.
	ReferencePage struct {
		Label    string             `json:"label,omitempty"`
		URL      string             `json:"url"`
		Title    string             `json:"title,omitempty"`
		Sections []ReferenceSection `json:"sections,omitempty"`
	}

	ReferenceSiteContext struct {
		URL         string                `json:"url"`
		FinalURL    string                `json:"final_url,omitempty"`
		Title       string                `json:"title,omitempty"`
		Description string                `json:"description,omitempty"`
		Screenshots []ReferenceScreenshot `json:"screenshots,omitempty"`
		Colors      []string              `json:"colors,omitempty"`
		Fonts       []string              `json:"fonts,omitempty"`
		Sections    []ReferenceSection    `json:"sections,omitempty"`
		Assets      []ReferenceAsset      `json:"assets,omitempty"`
		Navigation  []string              `json:"navigation,omitempty"`
		NavLinks    []ReferenceNavLink    `json:"nav_links,omitempty"`
		Pages       []ReferencePage       `json:"pages,omitempty"`
		Warnings    []string              `json:"warnings,omitempty"`
	}

	ArchitectPlan struct {
		ProjectName        string                `json:"project_name"`
		ProjectType        string                `json:"project_type"` // "admin_panel" | "landing" | "web" | "webapp" | "mobile"
		Tables             []TablePlan           `json:"tables"`
		Relations          []TableRelationPlan   `json:"relations,omitempty"`
		UIStructure        string                `json:"ui_structure"`
		Design             DesignSpec            `json:"design"`
		ImageKeywords      []string              `json:"image_keywords,omitempty"`
		ClientTypes        []string              `json:"client_types,omitempty"` // silently inferred access personas → each becomes client_type + role record
		MobileCapabilities []MobileCapability    `json:"mobile_capabilities,omitempty"`
		CloneMode          bool                  `json:"clone_mode,omitempty"`
		Reference          *ReferenceSiteContext `json:"reference,omitempty"`
	}

	ProjectData struct {
		McpProjectId   string `json:"project_id"`
		ApiKey         string `json:"api_key"`
		UcodeProjectId string `json:"ucode_project_id"`
		EnvironmentId  string `json:"environment_id"`
		ResourceEnvId  string `json:"resource_env_id"`
		NodeType       string `json:"node_type"`
		ResourceType   int32  `json:"resource_type"`
		AuthMode       string `json:"auth_mode,omitempty"`
		ShortURL       string `json:"short_url,omitempty"`
	}

	// ========================== Chunked Generation Manifest ==========================

	// ManifestFile is one file entry in a project manifest: path + exported names + role metadata.
	ManifestFile struct {
		Path           string              `json:"path"`
		Exports        []string            `json:"exports"`
		Kind           string              `json:"kind,omitempty"`            // "page" | "ui" | "shared" | "layout" | "types" | "hook" | "app" | "feature"
		Route          string              `json:"route,omitempty"`           // canonical URL path for pages
		PropsInterface string              `json:"props_interface,omitempty"` // for ui-kit components
		Variants       map[string][]string `json:"variants,omitempty"`        // e.g. {"variant":["default","outline"],"size":["sm","md","lg"]}
	}

	// ManifestGroup groups files by dependency level.
	// Group 0 = foundation (sequential). Groups 1..N = features (parallel).
	ManifestGroup struct {
		ID    int            `json:"id"`
		Name  string         `json:"name"`
		Files []ManifestFile `json:"files"`
	}

	// ManifestRoute maps a URL path to a page component and its file.
	ManifestRoute struct {
		Path     string `json:"path"`      // e.g. "/about"
		PageName string `json:"page_name"` // e.g. "AboutPage" — named export from FilePath
		FilePath string `json:"file_path"` // e.g. "src/pages/AboutPage.tsx"
	}

	// ManifestEntityField is one field of an entity interface.
	ManifestEntityField struct {
		Name     string `json:"name"`
		TSType   string `json:"ts_type"` // "string" | "number" | "boolean" | "Date" | etc.
		Optional bool   `json:"optional,omitempty"`
	}

	// ManifestEntityType describes a TypeScript interface that types.ts must export.
	ManifestEntityType struct {
		Name   string                `json:"name"` // PascalCase interface name, e.g. "Contact"
		Fields []ManifestEntityField `json:"fields"`
	}

	// ProjectManifest is the output of the manifest generation step.
	// ExportStyle/EntityTypes/Routes are optional: legacy manifests without
	// them fall through to the old generation path.
	ProjectManifest struct {
		Groups      []ManifestGroup      `json:"groups"`
		ExportStyle string               `json:"export_style,omitempty"` // "named-lazy" (new default) | "default-export" (legacy)
		EntityTypes []ManifestEntityType `json:"entity_types,omitempty"`
		Routes      []ManifestRoute      `json:"routes,omitempty"`
	}

	// ========================== AI Database Assistant ==========================

	// FieldSchema — simplified field info sent to Claude for schema awareness
	FieldSchema struct {
		Slug    string   `json:"slug"`
		Label   string   `json:"label"`
		Type    string   `json:"type"`
		Options []string `json:"options,omitempty"`
	}

	// TableSchema — simplified table info with fields sent to Claude
	TableSchema struct {
		Slug   string        `json:"slug"`
		Label  string        `json:"label"`
		Fields []FieldSchema `json:"fields"`
	}

	DatabaseActionRequest struct {
		Action           string         `json:"action"`
		TableSlug        string         `json:"table_slug,omitempty"`
		Filters          map[string]any `json:"filters,omitempty"`
		Data             map[string]any `json:"data,omitempty"`
		AggregationField string         `json:"aggregation_field,omitempty"`
		Aggregation      string         `json:"aggregation,omitempty"`
		GroupBy          string         `json:"group_by,omitempty"`
		OrderBy          string         `json:"order_by,omitempty"`
		Limit            int            `json:"limit,omitempty"`
		Offset           int            `json:"offset,omitempty"`
		NeedsMoreData    bool           `json:"needs_more_data"`
		QueryPlan        string         `json:"query_plan,omitempty"`
		Reply            string         `json:"reply,omitempty"`
		SuccessMessage   string         `json:"success_message,omitempty"`
		CancelMessage    string         `json:"cancel_message,omitempty"`
		ResourceEnvID    string         `json:"-"` // set by backend, not from AI

		SQL string `json:"sql,omitempty"`

		SQLParams []any `json:"sql_params,omitempty"`
	}

	PendingAction struct {
		Action             string         `json:"action"`
		TableSlug          string         `json:"table_slug,omitempty"`
		Filters            map[string]any `json:"filters,omitempty"`
		Data               map[string]any `json:"data,omitempty"`
		AffectedCount      int            `json:"affected_count,omitempty"`
		Description        string         `json:"description,omitempty"`
		ProjectID          string         `json:"project_id,omitempty"`
		ResourceEnvID      string         `json:"resource_env_id,omitempty"`
		SuccessMessage     string         `json:"success_message,omitempty"`
		CancelMessage      string         `json:"cancel_message,omitempty"`
		ConfirmationPrompt string         `json:"confirmation_prompt,omitempty"`
		Approved           bool           `json:"approved,omitempty"`

		// ── NEW: SQL mode fields ──────────────────────────────────────────────────

		SQL       string `json:"sql,omitempty"`
		SQLParams []any  `json:"sql_params,omitempty"`
	}
	// ConfirmActionRequest — frontend sends this to confirm/reject a pending action
	ConfirmActionRequest struct {
		Confirmed bool `json:"confirmed"`
	}

	// DatabaseAssistantResponse — full response from the database assistant flow
	DatabaseAssistantResponse struct {
		Reply         string         `json:"reply"`
		Data          any            `json:"data,omitempty"`
		PendingAction *PendingAction `json:"pending_action,omitempty"`
	}
)

package models

type (
	// ProjectFile is one file in a generated or edited project.
	ProjectFile struct {
		Path          string `json:"path"`
		Content       string `json:"content"`
		ChangeSummary string `json:"change_summary,omitempty"`
		Purpose       string `json:"purpose,omitempty"`
	}

	// GeneratedProject is the output of a full code-generation step.
	GeneratedProject struct {
		ProjectName string         `json:"project_name"`
		Files       []ProjectFile  `json:"files"`
		FileGraph   map[string]any `json:"file_graph"`
		Env         map[string]any `json:"env"`
	}

	RouterInput struct {
		UserMessage   string
		FileGraphJSON string
		HasImages     bool
		History       []ChatMessage
	}

	ArchitectInput struct {
		Clarified         string
		OriginalPrompt    string
		ExistingSchemaCtx string
		Images            []string
		History           []ChatMessage
	}

	ManifestInput struct {
		Plan    *ArchitectPlan
		History []ChatMessage
	}

	PlannerInput struct {
		Clarified     string
		FileGraphJSON string
		HasImages     bool
		History       []ChatMessage
	}

	InspectorInput struct {
		Question     string
		FilesContext string
		Images       []string
		History      []ChatMessage
	}

	EditorInput struct {
		Clarified        string
		Plan             *SonnetPlanResult
		FilesContext     string
		Images           []string
		History          []ChatMessage
		HasMatchingFiles bool
		PromptOverride   string

		Chunked      bool
		FullPlanJSON string
	}

	// EditPromptOverrides contains request-scoped custom prompt bodies for the
	// two active Ugen edit paths. Empty values preserve the built-in prompts.
	EditPromptOverrides struct {
		CodeEditor   string
		VisualEditor string
	}

	LLMUsage struct {
		InputTokens  int
		OutputTokens int
	}

	// VisualEditInput carries pre-built messages for the visual edit tool call.
	// The caller (v1) is responsible for building prompt and resolving file contexts.
	VisualEditInput struct {
		Messages       []ChatMessage
		PromptOverride string
	}

	// AgentIntegrationInput carries pre-built messages for the integrate_agent tool
	// call: wiring a just-created end-user agent into the generated frontend project.
	// The caller (v1) builds the prompt, resolves file contexts, and supplies the
	// agent API contract so the model only has to place and style the widget.
	AgentIntegrationInput struct {
		Messages []ChatMessage
	}

	// AgentIntegrationView is everything the integration prompt needs to wire a
	// freshly-created agent into the existing frontend: who the agent is, what it
	// can do, what the builder asked for, and enough of the project to place the
	// widget correctly. The runAgent client and useAgent hook are injected as
	// template files, so the model only writes UI that consumes them.
	AgentIntegrationView struct {
		AgentName     string
		AgentID       string
		Purpose       string
		Capabilities  string
		UserRequest   string
		TemplateFiles []string
		FileGraphJSON string
		FilesContext  string
	}

	// BuilderAgentIntegrationView is what the builder-assistant integration prompt
	// needs to mount its chat widget: the injected template files plus enough of the
	// project (file graph + shell files) to place and style the widget correctly.
	BuilderAgentIntegrationView struct {
		TemplateFiles []string
		FileGraphJSON string
		FilesContext  string
	}

	// RepairFileInput carries the file to repair and the pre-built user prompt
	// (errors + available exports + rules). The agent applies the repair tool.
	RepairFileInput struct {
		File       ProjectFile
		UserPrompt string
	}

	// DatabaseQueryInput carries data for the database assistant agent call.
	DatabaseQueryInput struct {
		Clarified   string
		SchemaText  string
		DataContext string
		History     []ChatMessage
	}

	// AgentSpecInput carries a builder's natural-language request to generate a
	// reusable end-user agent, plus the project schema the agent may operate on.
	// ReferenceDocs holds text extracted from example/template documents the builder
	// attached (e.g. an xlsx/pptx sample КП), so the model can bake that format into
	// the agent's instruction.
	AgentSpecInput struct {
		Description   string
		SchemaText    string
		History       []ChatMessage
		ReferenceDocs string
	}

	// AgentSpec is the generated definition of a reusable agent: a system prompt
	// (Instruction), display metadata, and the minimal per-table permissions it
	// needs. Reply is a short confirmation message shown back to the builder.
	AgentSpec struct {
		Name        string                `json:"name"`
		Description string                `json:"description"`
		Instruction string                `json:"instruction"`
		Reply       string                `json:"reply"`
		Permissions []AgentSpecPermission `json:"permissions"`
	}

	// AgentSpecPermission grants an agent a set of operations on one table.
	AgentSpecPermission struct {
		TableSlug string `json:"table_slug"`
		CanCreate bool   `json:"can_create"`
		CanRead   bool   `json:"can_read"`
		CanUpdate bool   `json:"can_update"`
		CanDelete bool   `json:"can_delete"`
		CanList   bool   `json:"can_list"`
	}
)

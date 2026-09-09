package openai

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

type OpenAIAgent struct {
	conf    config.BaseConfig
	tracker ai.UsageTracker
}

func NewOpenAIAgent(conf config.BaseConfig, tracker ai.UsageTracker) *OpenAIAgent {
	if conf.OpenAIAPIKey == "" {
		log.Printf("[OPENAI] WARNING: OPENAI_API_KEY is empty — all requests will fail with 401")
	}
	return &OpenAIAgent{conf: conf, tracker: tracker}
}

func (a *OpenAIAgent) RouteRequest(ctx context.Context, in models.RouterInput) (*models.HaikuRoutingResult, error) {
	historyText := ai.BuildHistoryText(in.History)
	state := ai.DetectConversationState(in.History)
	content := chat_prompts.BuildRouterMessage(in.UserMessage, in.FileGraphJSON, in.HasImages, historyText, state)

	messages := buildOpenAIMessages(nil, buildContentParts(content, nil))

	cfg := a.conf.OpenAIAgents.Router
	text, usage, err := callText(ctx, a.conf, cfg, chat_prompts.PromptRouter, messages)
	a.tracker.RecordUsage(usage, cfg.Model, "Routing user intent")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, fmt.Errorf("router: %w", err)
	}

	result, err := parseRoutingResult(text)
	if err != nil {
		return nil, fmt.Errorf("router: parse: %w", err)
	}
	result.HasImages = in.HasImages
	return result, nil
}

func (a *OpenAIAgent) ArchitectProject(ctx context.Context, in models.ArchitectInput) (*models.ArchitectPlan, error) {
	userMsg := in.Clarified
	if in.ExistingSchemaCtx != "" {
		userMsg += "\n\n====================================\nEXISTING PROJECT TABLES (already provisioned — use these slugs for API calls, do NOT recreate them)\n====================================\n" + in.ExistingSchemaCtx
	}

	messages := buildOpenAIMessages(in.History, buildContentParts(userMsg, in.Images))

	cfg := a.conf.OpenAIAgents.Architect
	raw, usage, err := callTool(ctx, a.conf, cfg, chat_prompts.PromptArchitect, messages, toolArchitectPlan)
	a.tracker.RecordUsage(usage, cfg.Model, "Architecting project structure")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, "architect")
	}

	var plan models.ArchitectPlan
	if err = json.Unmarshal(raw, &plan); err != nil {
		return nil, fmt.Errorf("architect: decode: %w", err)
	}
	overridePrompt := in.OriginalPrompt
	if overridePrompt == "" {
		overridePrompt = in.Clarified
	}
	models.ApplyProjectTypeKeywordOverride(&plan, overridePrompt)
	models.ApplyMobileCapabilityKeywordOverride(&plan, overridePrompt)
	return &plan, nil
}

func (a *OpenAIAgent) GenerateManifest(ctx context.Context, in models.ManifestInput) (*models.ProjectManifest, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "Project: %s (type: %s)\n\n", in.Plan.ProjectName, in.Plan.ProjectType)
	if len(in.Plan.Tables) > 0 {
		sb.WriteString("Tables:\n")
		for _, t := range in.Plan.Tables {
			fmt.Fprintf(&sb, "- %s (slug: %s): ", t.Label, t.Slug)
			for i, f := range t.Fields {
				if i > 0 {
					sb.WriteString(", ")
				}
				sb.WriteString(f.Slug)
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}
	sb.WriteString("UI Structure:\n" + in.Plan.UIStructure)

	systemPrompt := chat_prompts.PromptManifestGenerator
	if in.Plan.ProjectType == "web" {
		systemPrompt = chat_prompts.PromptWebsiteManifestGenerator
	} else if in.Plan.ProjectType == "webapp" || in.Plan.ProjectType == "mobile" {
		systemPrompt = chat_prompts.PromptWebAppManifestGenerator
	}

	messages := buildOpenAIMessages(in.History, []contentPart{{Type: "text", Text: sb.String()}})

	cfg := a.conf.OpenAIAgents.Planner
	raw, usage, err := callTool(ctx, a.conf, cfg, systemPrompt, messages, toolEmitManifest)
	a.tracker.RecordUsage(usage, cfg.Model, "Generating file manifest")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, "manifest")
	}

	var manifest models.ProjectManifest
	if err = json.Unmarshal(raw, &manifest); err != nil {
		return nil, fmt.Errorf("manifest: decode: %w", err)
	}
	return &manifest, nil
}

func (a *OpenAIAgent) PlanChanges(ctx context.Context, in models.PlannerInput) (*models.SonnetPlanResult, error) {
	content := chat_prompts.BuildPlannerMessage(in.Clarified, in.FileGraphJSON, in.HasImages)
	messages := buildOpenAIMessages(in.History, []contentPart{{Type: "text", Text: content}})

	cfg := a.conf.OpenAIAgents.Planner
	raw, usage, err := callTool(ctx, a.conf, cfg, chat_prompts.PromptPlanner, messages, toolPlanChanges)
	a.tracker.RecordUsage(usage, cfg.Model, "Planning code changes")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, "planner")
	}

	var result models.SonnetPlanResult
	if err = json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("planner: decode: %w", err)
	}
	return &result, nil
}

func (a *OpenAIAgent) InspectCode(ctx context.Context, in models.InspectorInput) (string, error) {
	content := chat_prompts.BuildInspectorMessage(in.Question, in.FilesContext)
	messages := buildOpenAIMessages(in.History, buildContentParts(content, in.Images))

	cfg := a.conf.OpenAIAgents.Inspector
	text, usage, err := callText(ctx, a.conf, cfg, chat_prompts.PromptInspector, messages)
	a.tracker.RecordUsage(usage, cfg.Model, "Inspecting code context")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return "", fmt.Errorf("inspector: %w", err)
	}
	return text, nil
}

func (a *OpenAIAgent) EditCode(ctx context.Context, in models.EditorInput) (*models.GeneratedProject, error) {
	var (
		systemPrompt string
		parts        []contentPart
	)

	if in.Chunked {
		systemPrompt = chat_prompts.CodeEditorSystemPrompt(in.PromptOverride, true)
		assignedJSON, _ := json.Marshal(in.Plan)
		content := chat_prompts.BuildCodeEditorChunkMessage(in.Clarified, string(assignedJSON), in.FullPlanJSON, in.FilesContext, len(in.Images) > 0)
		parts = buildContentParts(content, in.Images)
	} else if in.HasMatchingFiles {
		systemPrompt = chat_prompts.CodeEditorSystemPrompt(in.PromptOverride, false)
		planJSON, _ := json.Marshal(in.Plan)
		content := chat_prompts.BuildCodeEditorMessage(in.Clarified, string(planJSON), in.FilesContext, len(in.Images) > 0)
		parts = buildContentParts(content, in.Images)
	} else {
		log.Printf("[openai editor] planned files not found — falling back to free generation")
		systemPrompt = chat_prompts.PromptAdminPanelGenerator
		parts = buildContentParts(in.Clarified, in.Images)
	}

	messages := buildOpenAIMessages(in.History, parts)

	cfg := a.conf.OpenAIAgents.Coder
	raw, usage, err := callStructured(ctx, a.conf, cfg, systemPrompt, messages, emitProjectStructuredSchema)
	a.tracker.RecordUsage(usage, cfg.Model, "Applying code changes")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, "editor")
	}

	var project models.GeneratedProject
	if err = safeUnmarshal(raw, &project); err != nil {
		return nil, fmt.Errorf("editor: decode: %w", err)
	}
	return &project, nil
}

func (a *OpenAIAgent) GenerateCode(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt string, images []string, history []models.ChatMessage, desc string) (*models.GeneratedProject, error) {
	messages := buildOpenAIMessages(history, buildContentParts(userPrompt, images))

	raw, usage, err := callStructured(ctx, a.conf, agentCfg, systemPrompt, messages, emitProjectStructuredSchema)
	a.tracker.RecordUsage(usage, agentCfg.Model, desc)
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, fmt.Sprintf("generate code (%s)", desc))
	}

	var project models.GeneratedProject
	if err = safeUnmarshal(raw, &project); err != nil {
		return nil, fmt.Errorf("generate code (%s): decode: %w", desc, err)
	}
	return &project, nil
}

func (a *OpenAIAgent) GenerateCodeNoHistory(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt, desc string) (*models.GeneratedProject, error) {
	messages := []chatMessage{
		{Role: "user", Content: userPrompt},
	}

	raw, usage, err := callStructured(ctx, a.conf, agentCfg, systemPrompt, messages, emitProjectStructuredSchema)
	a.tracker.RecordUsage(usage, agentCfg.Model, desc)
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, fmt.Sprintf("generate code (%s)", desc))
	}

	var project models.GeneratedProject
	if err = safeUnmarshal(raw, &project); err != nil {
		return nil, fmt.Errorf("generate code (%s): decode: %w", desc, err)
	}
	return &project, nil
}

func (a *OpenAIAgent) VisualEdit(ctx context.Context, in models.VisualEditInput) ([]models.ProjectFile, string, error) {
	messages := convertMessages(in.Messages)

	cfg := a.conf.OpenAIAgents.Coder
	systemPrompt := chat_prompts.VisualEditorSystemPrompt(in.PromptOverride)
	raw, usage, err := callTool(ctx, a.conf, cfg, systemPrompt, messages, toolEmitVisualEdit)
	a.tracker.RecordUsage(usage, cfg.Model, "Visual edit")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, "", fmt.Errorf("visual edit: %w", err)
	}

	var out struct {
		Files         []models.ProjectFile `json:"files"`
		ChangeSummary string               `json:"change_summary"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, "", fmt.Errorf("visual edit: decode: %w", err)
	}
	return out.Files, out.ChangeSummary, nil
}

func (a *OpenAIAgent) IntegrateAgent(ctx context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	messages := convertMessages(in.Messages)

	cfg := a.conf.OpenAIAgents.Coder
	raw, usage, err := callTool(ctx, a.conf, cfg, chat_prompts.PromptAgentIntegrator, messages, toolIntegrateAgent)
	a.tracker.RecordUsage(usage, cfg.Model, "Integrating agent into frontend")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, "", fmt.Errorf("integrate agent: %w", err)
	}

	var out struct {
		Files         []models.ProjectFile `json:"files"`
		ChangeSummary string               `json:"change_summary"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, "", fmt.Errorf("integrate agent: decode: %w", err)
	}
	return out.Files, out.ChangeSummary, nil
}

func (a *OpenAIAgent) IntegrateBuilderAgent(ctx context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	messages := convertMessages(in.Messages)

	cfg := a.conf.OpenAIAgents.Coder
	raw, usage, err := callTool(ctx, a.conf, cfg, chat_prompts.PromptBuilderAgentIntegrator, messages, toolIntegrateAgent)
	a.tracker.RecordUsage(usage, cfg.Model, "Integrating builder assistant into frontend")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, "", fmt.Errorf("integrate builder assistant: %w", err)
	}

	var out struct {
		Files         []models.ProjectFile `json:"files"`
		ChangeSummary string               `json:"change_summary"`
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, "", fmt.Errorf("integrate builder assistant: decode: %w", err)
	}
	return out.Files, out.ChangeSummary, nil
}

func (a *OpenAIAgent) RepairFile(ctx context.Context, in models.RepairFileInput) (models.ProjectFile, error) {
	// Code-specialized model handles TSX repair; widen output for full-file rewrites.
	repairCfg := a.conf.OpenAIAgents.Coder
	repairCfg.MaxTokens = 32000
	repairCfg.Timeout = 120 * time.Second

	messages := []chatMessage{
		{Role: "user", Content: in.UserPrompt},
	}

	const repairSystem = "You are a TypeScript/TSX and premium admin-UI repair bot. Fix the listed errors: import mismatches, typos, displayName issues, Radix SelectItem empty-value runtime crashes, React infinite-recursion bugs where a component renders itself, admin UI quality failures, AND syntax errors like unbalanced braces/brackets/parentheses. For admin UI quality failures, preserve backend/API contracts and polish only the current file into a product-grade SaaS screen. Output the complete corrected file via the repair_file tool. Never truncate."

	raw, usage, err := callTool(ctx, a.conf, repairCfg, repairSystem, messages, toolRepairFile)
	a.tracker.RecordUsage(usage, repairCfg.Model, fmt.Sprintf("Repair %s", in.File.Path))
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return models.ProjectFile{}, fmt.Errorf("repair file: %w", err)
	}

	var result struct {
		Content string `json:"content"`
	}
	if err = json.Unmarshal(raw, &result); err != nil {
		return models.ProjectFile{}, fmt.Errorf("repair file: decode: %w", err)
	}
	if result.Content == "" {
		return models.ProjectFile{}, fmt.Errorf("repair file: empty content returned")
	}
	return models.ProjectFile{Path: in.File.Path, Content: result.Content}, nil
}

func (a *OpenAIAgent) DatabaseQuery(ctx context.Context, in models.DatabaseQueryInput) (*models.DatabaseActionRequest, error) {
	content := chat_prompts.BuildDatabaseMessage(in.Clarified, in.SchemaText, in.DataContext)
	messages := buildOpenAIMessages(in.History, []contentPart{{Type: "text", Text: content}})

	cfg := a.conf.OpenAIAgents.DatabaseAssistant
	text, usage, err := callText(ctx, a.conf, cfg, chat_prompts.PromptDatabaseAssistant, messages)
	a.tracker.RecordUsage(usage, cfg.Model, "Database query")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, fmt.Errorf("database query: %w", err)
	}

	var action models.DatabaseActionRequest
	if err = json.Unmarshal([]byte(ai.ExtractJSONFromText(text)), &action); err != nil {
		return nil, fmt.Errorf("database query: parse action JSON: %w | raw=%.300s", err, text)
	}
	return &action, nil
}

func (a *OpenAIAgent) BuildAgentSpec(ctx context.Context, in models.AgentSpecInput) (*models.AgentSpec, error) {
	content := chat_prompts.BuildAgentBuilderMessage(in.Description, in.SchemaText, in.ReferenceDocs)
	messages := buildOpenAIMessages(in.History, []contentPart{{Type: "text", Text: content}})

	cfg := a.conf.OpenAIAgents.AgentBuilder
	raw, usage, err := callTool(ctx, a.conf, cfg, chat_prompts.PromptAgentBuilder, messages, toolBuildAgentSpec)
	a.tracker.RecordUsage(usage, cfg.Model, "Building agent definition")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, "agent builder")
	}

	var spec models.AgentSpec
	if err = json.Unmarshal(raw, &spec); err != nil {
		return nil, fmt.Errorf("agent builder: decode: %w", err)
	}
	return &spec, nil
}

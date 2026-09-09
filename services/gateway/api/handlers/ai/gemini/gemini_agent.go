package gemini

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

type GeminiAgent struct {
	conf    config.BaseConfig
	pool    *KeyPool
	tracker ai.UsageTracker
}

func NewGeminiAgent(conf config.BaseConfig, pool *KeyPool, tracker ai.UsageTracker) ai.Agent {
	// If no pool provided, build a single-key pool from config as fallback.
	if pool == nil && conf.GeminiAPIKey != "" {
		pool, _ = NewKeyPool([]string{conf.GeminiAPIKey})
	}
	return &GeminiAgent{conf: conf, pool: pool, tracker: tracker}
}

func (a *GeminiAgent) RouteRequest(_ context.Context, in models.RouterInput) (*models.HaikuRoutingResult, error) {
	historyText := ai.BuildHistoryText(in.History)
	state := ai.DetectConversationState(in.History)
	content := chat_prompts.BuildRouterMessage(in.UserMessage, in.FileGraphJSON, in.HasImages, historyText, state)

	contents := []geminiContent{
		{Role: "user", Parts: []geminiPart{{Text: content}}},
	}

	cfg := a.conf.GeminiAgents.Router
	text, usage, err := callGeminiText(a.pool, cfg, chat_prompts.PromptRouter, contents)
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

func (a *GeminiAgent) ArchitectProject(_ context.Context, in models.ArchitectInput) (*models.ArchitectPlan, error) {
	userMsg := in.Clarified
	if in.ExistingSchemaCtx != "" {
		userMsg += "\n\n====================================\nEXISTING PROJECT TABLES (already provisioned — use these slugs for API calls, do NOT recreate them)\n====================================\n" + in.ExistingSchemaCtx
	}

	contents := buildGeminiContents(in.History, buildGeminiParts(userMsg, in.Images))

	cfg := a.conf.GeminiAgents.Architect
	raw, usage, err := callGeminiTool(a.pool, cfg, chat_prompts.PromptArchitect, contents, toolArchitectPlan)
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
	return &plan, nil
}

func (a *GeminiAgent) GenerateManifest(_ context.Context, in models.ManifestInput) (*models.ProjectManifest, error) {
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
	} else if in.Plan.ProjectType == "webapp" {
		systemPrompt = chat_prompts.PromptWebAppManifestGenerator
	}

	contents := buildGeminiContents(in.History, []geminiPart{{Text: sb.String()}})

	cfg := a.conf.GeminiAgents.Planner
	raw, usage, err := callGeminiTool(a.pool, cfg, systemPrompt, contents, toolEmitManifest)
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

func (a *GeminiAgent) PlanChanges(_ context.Context, in models.PlannerInput) (*models.SonnetPlanResult, error) {
	content := chat_prompts.BuildPlannerMessage(in.Clarified, in.FileGraphJSON, in.HasImages)
	contents := buildGeminiContents(in.History, []geminiPart{{Text: content}})

	cfg := a.conf.GeminiAgents.Planner
	raw, usage, err := callGeminiTool(a.pool, cfg, chat_prompts.PromptPlanner, contents, toolPlanChanges)
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

func (a *GeminiAgent) InspectCode(_ context.Context, in models.InspectorInput) (string, error) {
	content := chat_prompts.BuildInspectorMessage(in.Question, in.FilesContext)
	contents := buildGeminiContents(in.History, buildGeminiParts(content, in.Images))

	cfg := a.conf.GeminiAgents.Inspector
	text, usage, err := callGeminiText(a.pool, cfg, chat_prompts.PromptInspector, contents)
	a.tracker.RecordUsage(usage, cfg.Model, "Inspecting code context")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return "", fmt.Errorf("inspector: %w", err)
	}
	return text, nil
}

func (a *GeminiAgent) EditCode(_ context.Context, in models.EditorInput) (*models.GeneratedProject, error) {
	var (
		systemPrompt string
		parts        []geminiPart
	)

	if in.Chunked {
		systemPrompt = chat_prompts.CodeEditorSystemPrompt(in.PromptOverride, true)
		assignedJSON, _ := json.Marshal(in.Plan)
		content := chat_prompts.BuildCodeEditorChunkMessage(in.Clarified, string(assignedJSON), in.FullPlanJSON, in.FilesContext, len(in.Images) > 0)
		parts = buildGeminiParts(content, in.Images)
	} else if in.HasMatchingFiles {
		systemPrompt = chat_prompts.CodeEditorSystemPrompt(in.PromptOverride, false)
		planJSON, _ := json.Marshal(in.Plan)
		content := chat_prompts.BuildCodeEditorMessage(in.Clarified, string(planJSON), in.FilesContext, len(in.Images) > 0)
		parts = buildGeminiParts(content, in.Images)
	} else {
		log.Printf("[gemini editor] planned files not found — falling back to free generation")
		systemPrompt = chat_prompts.PromptAdminPanelGenerator
		parts = buildGeminiParts(in.Clarified, in.Images)
	}

	contents := buildGeminiContents(in.History, parts)

	cfg := a.conf.GeminiAgents.Coder
	raw, usage, err := callGeminiTool(a.pool, cfg, systemPrompt, contents, toolEmitProject)
	a.tracker.RecordUsage(usage, cfg.Model, "Applying code changes")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, "editor")
	}

	var project models.GeneratedProject
	if err = json.Unmarshal(raw, &project); err != nil {
		return nil, fmt.Errorf("editor: decode: %w", err)
	}
	return &project, nil
}

func (a *GeminiAgent) GenerateCode(_ context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt string, images []string, history []models.ChatMessage, desc string) (*models.GeneratedProject, error) {
	contents := buildGeminiContents(history, buildGeminiParts(userPrompt, images))

	raw, usage, err := callGeminiTool(a.pool, agentCfg, systemPrompt, contents, toolEmitProject)
	a.tracker.RecordUsage(usage, agentCfg.Model, desc)
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, fmt.Sprintf("generate code (%s)", desc))
	}

	var project models.GeneratedProject
	if err = json.Unmarshal(raw, &project); err != nil {
		return nil, fmt.Errorf("generate code (%s): decode: %w", desc, err)
	}
	return &project, nil
}

func (a *GeminiAgent) GenerateCodeNoHistory(_ context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt, desc string) (*models.GeneratedProject, error) {
	contents := []geminiContent{
		{Role: "user", Parts: []geminiPart{{Text: userPrompt}}},
	}

	raw, usage, err := callGeminiTool(a.pool, agentCfg, systemPrompt, contents, toolEmitProject)
	a.tracker.RecordUsage(usage, agentCfg.Model, desc)
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, wrapMaxTokens(err, usage, fmt.Sprintf("generate code (%s)", desc))
	}

	var project models.GeneratedProject
	if err = json.Unmarshal(raw, &project); err != nil {
		return nil, fmt.Errorf("generate code (%s): decode: %w", desc, err)
	}
	return &project, nil
}

func (a *GeminiAgent) VisualEdit(_ context.Context, in models.VisualEditInput) ([]models.ProjectFile, string, error) {
	contents := convertMessages(in.Messages)

	cfg := a.conf.GeminiAgents.Coder
	systemPrompt := chat_prompts.VisualEditorSystemPrompt(in.PromptOverride)
	raw, usage, err := callGeminiTool(a.pool, cfg, systemPrompt, contents, toolEmitVisualEdit)
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

func (a *GeminiAgent) IntegrateAgent(_ context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	contents := convertMessages(in.Messages)

	cfg := a.conf.GeminiAgents.Coder
	raw, usage, err := callGeminiTool(a.pool, cfg, chat_prompts.PromptAgentIntegrator, contents, toolIntegrateAgent)
	a.tracker.RecordUsage(usage, cfg.Model, "Integrating agent into frontend")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, "", wrapMaxTokens(err, usage, "integrate agent")
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

func (a *GeminiAgent) IntegrateBuilderAgent(_ context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	contents := convertMessages(in.Messages)

	cfg := a.conf.GeminiAgents.Coder
	raw, usage, err := callGeminiTool(a.pool, cfg, chat_prompts.PromptBuilderAgentIntegrator, contents, toolIntegrateAgent)
	a.tracker.RecordUsage(usage, cfg.Model, "Integrating builder assistant into frontend")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, "", wrapMaxTokens(err, usage, "integrate builder assistant")
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

func (a *GeminiAgent) RepairFile(_ context.Context, in models.RepairFileInput) (models.ProjectFile, error) {
	repairCfg := a.conf.GeminiAgents.Coder
	repairCfg.MaxTokens = 32000
	repairCfg.Timeout = 120 * time.Second

	contents := []geminiContent{
		{Role: "user", Parts: []geminiPart{{Text: in.UserPrompt}}},
	}
	const repairSystem = "You are a TypeScript/TSX and premium admin-UI repair bot. Fix the listed errors: import mismatches, typos, displayName issues, Radix SelectItem empty-value runtime crashes, React infinite-recursion bugs where a component renders itself, admin UI quality failures, AND syntax errors like unbalanced braces/brackets/parentheses. For admin UI quality failures, preserve backend/API contracts and polish only the current file into a product-grade SaaS screen. Output the complete corrected file via the repair_file tool. Never truncate."

	raw, usage, err := callGeminiTool(a.pool, repairCfg, repairSystem, contents, toolRepairFile)
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

func (a *GeminiAgent) DatabaseQuery(_ context.Context, in models.DatabaseQueryInput) (*models.DatabaseActionRequest, error) {
	content := chat_prompts.BuildDatabaseMessage(in.Clarified, in.SchemaText, in.DataContext)
	contents := buildGeminiContents(in.History, []geminiPart{{Text: content}})

	cfg := a.conf.GeminiAgents.Inspector
	text, usage, err := callGeminiText(a.pool, cfg, chat_prompts.PromptDatabaseAssistant, contents)
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

func (a *GeminiAgent) BuildAgentSpec(_ context.Context, in models.AgentSpecInput) (*models.AgentSpec, error) {
	content := chat_prompts.BuildAgentBuilderMessage(in.Description, in.SchemaText, in.ReferenceDocs)
	contents := buildGeminiContents(in.History, buildGeminiParts(content, nil))

	cfg := a.conf.GeminiAgents.AgentBuilder
	raw, usage, err := callGeminiTool(a.pool, cfg, chat_prompts.PromptAgentBuilder, contents, toolBuildAgentSpec)
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

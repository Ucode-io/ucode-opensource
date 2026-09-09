package anthropic

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

type AnthropicAgent struct {
	conf    config.BaseConfig
	tracker ai.UsageTracker
}

func NewAnthropicAgent(conf config.BaseConfig, tracker ai.UsageTracker) ai.Agent {
	return &AnthropicAgent{conf: conf, tracker: tracker}
}

// ── Agent interface implementation ────────────────────────────────────────────

func (a *AnthropicAgent) RouteRequest(ctx context.Context, in models.RouterInput) (*models.HaikuRoutingResult, error) {
	historyText := ai.BuildHistoryText(in.History)
	state := ai.DetectConversationState(in.History)
	content := chat_prompts.BuildRouterMessage(in.UserMessage, in.FileGraphJSON, in.HasImages, historyText, state)

	messages := []models.ChatMessage{
		{Role: "user", Content: []models.ContentBlock{{Type: "text", Text: content}}},
	}

	raw, usage, err := callAnthropicText(a.conf, a.conf.Agents.Router, chat_prompts.PromptRouter, messages)
	a.tracker.RecordUsage(usage, a.conf.Agents.Router.Model, "Routing user intent")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, fmt.Errorf("router: %w", err)
	}

	result, err := parseHaikuRoutingResult(raw)
	if err != nil {
		return nil, fmt.Errorf("router: parse: %w", err)
	}
	result.HasImages = in.HasImages
	return result, nil
}

func (a *AnthropicAgent) ArchitectProject(ctx context.Context, in models.ArchitectInput) (*models.ArchitectPlan, error) {
	userMsg := in.Clarified
	if in.ExistingSchemaCtx != "" {
		userMsg += "\n\n====================================\nEXISTING PROJECT TABLES (already provisioned — use these slugs for API calls, do NOT recreate them)\n====================================\n" + in.ExistingSchemaCtx
	}

	messages := buildAgentMessages(in.History, buildContentBlocks(userMsg, in.Images))

	raw, usage, _, err := a.callTool(a.conf.Agents.Architect, chat_prompts.PromptArchitect, messages, ToolArchitectPlan)
	a.tracker.RecordUsage(usage, a.conf.Agents.Architect.Model, "Architecting project structure")
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

func (a *AnthropicAgent) GenerateManifest(ctx context.Context, in models.ManifestInput) (*models.ProjectManifest, error) {
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

	messages := buildAgentMessages(in.History, []models.ContentBlock{{Type: "text", Text: sb.String()}})

	raw, usage, _, err := a.callTool(a.conf.Agents.Planner, systemPrompt, messages, ToolEmitManifest)
	a.tracker.RecordUsage(usage, a.conf.Agents.Planner.Model, "Generating file manifest")
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

func (a *AnthropicAgent) PlanChanges(ctx context.Context, in models.PlannerInput) (*models.SonnetPlanResult, error) {
	content := chat_prompts.BuildPlannerMessage(in.Clarified, in.FileGraphJSON, in.HasImages)
	messages := buildAgentMessages(in.History, []models.ContentBlock{{Type: "text", Text: content}})

	raw, usage, _, err := a.callTool(a.conf.Agents.Planner, chat_prompts.PromptPlanner, messages, ToolPlanChanges)
	a.tracker.RecordUsage(usage, a.conf.Agents.Planner.Model, "Planning code changes")
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

func (a *AnthropicAgent) InspectCode(ctx context.Context, in models.InspectorInput) (string, error) {
	content := chat_prompts.BuildInspectorMessage(in.Question, in.FilesContext)
	messages := buildAgentMessages(in.History, buildContentBlocks(content, in.Images))

	raw, usage, err := callAnthropicText(a.conf, a.conf.Agents.Inspector, chat_prompts.PromptInspector, messages)
	a.tracker.RecordUsage(usage, a.conf.Agents.Inspector.Model, "Inspecting code context")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return "", fmt.Errorf("inspector: %w", err)
	}

	answer, err := extractPlainText(raw)
	if err != nil {
		return "", fmt.Errorf("inspector: extract text: %w", err)
	}
	return answer, nil
}

func (a *AnthropicAgent) EditCode(ctx context.Context, in models.EditorInput) (*models.GeneratedProject, error) {
	var (
		systemPrompt  string
		contentBlocks []models.ContentBlock
	)

	if in.Chunked {
		systemPrompt = chat_prompts.CodeEditorSystemPrompt(in.PromptOverride, true)
		assignedJSON, _ := json.Marshal(in.Plan)
		content := chat_prompts.BuildCodeEditorChunkMessage(in.Clarified, string(assignedJSON), in.FullPlanJSON, in.FilesContext, len(in.Images) > 0)
		contentBlocks = buildContentBlocks(content, in.Images)
	} else if in.HasMatchingFiles {
		systemPrompt = chat_prompts.CodeEditorSystemPrompt(in.PromptOverride, false)
		planJSON, _ := json.Marshal(in.Plan)
		content := chat_prompts.BuildCodeEditorMessage(in.Clarified, string(planJSON), in.FilesContext, len(in.Images) > 0)
		contentBlocks = buildContentBlocks(content, in.Images)
	} else {
		log.Printf("[editor] planned files not found — falling back to free generation")
		systemPrompt = chat_prompts.PromptAdminPanelGenerator
		contentBlocks = buildContentBlocks(in.Clarified, in.Images)
	}

	messages := buildAgentMessages(in.History, contentBlocks)

	raw, usage, _, err := a.callTool(a.conf.Agents.Coder, systemPrompt, messages, ToolEmitProject)
	a.tracker.RecordUsage(usage, a.conf.Agents.Coder.Model, "Applying code changes")
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

func (a *AnthropicAgent) GenerateCode(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt string, images []string, history []models.ChatMessage, desc string) (*models.GeneratedProject, error) {
	messages := buildAgentMessages(history, buildContentBlocks(userPrompt, images))

	raw, usage, _, err := a.callTool(agentCfg, systemPrompt, messages, ToolEmitProject)
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

func (a *AnthropicAgent) GenerateCodeNoHistory(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt, desc string) (*models.GeneratedProject, error) {
	messages := []models.ChatMessage{
		{Role: "user", Content: []models.ContentBlock{{Type: "text", Text: userPrompt}}},
	}

	raw, usage, _, err := a.callTool(agentCfg, systemPrompt, messages, ToolEmitProject)
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

func (a *AnthropicAgent) VisualEdit(_ context.Context, in models.VisualEditInput) ([]models.ProjectFile, string, error) {
	systemPrompt := chat_prompts.VisualEditorSystemPrompt(in.PromptOverride)
	raw, usage, _, err := a.callTool(a.conf.Agents.Coder, systemPrompt, in.Messages, ToolEmitVisualEdit)
	a.tracker.RecordUsage(usage, a.conf.Agents.Coder.Model, "Visual edit")
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

func (a *AnthropicAgent) IntegrateAgent(_ context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	raw, usage, _, err := a.callTool(a.conf.Agents.Coder, chat_prompts.PromptAgentIntegrator, in.Messages, ToolIntegrateAgent)
	a.tracker.RecordUsage(usage, a.conf.Agents.Coder.Model, "Integrating agent into frontend")
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

func (a *AnthropicAgent) IntegrateBuilderAgent(_ context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	raw, usage, _, err := a.callTool(a.conf.Agents.Coder, chat_prompts.PromptBuilderAgentIntegrator, in.Messages, ToolIntegrateAgent)
	a.tracker.RecordUsage(usage, a.conf.Agents.Coder.Model, "Integrating builder assistant into frontend")
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

func (a *AnthropicAgent) RepairFile(_ context.Context, in models.RepairFileInput) (models.ProjectFile, error) {
	repairCfg := a.conf.Agents.Router
	repairCfg.MaxTokens = 32000
	repairCfg.Timeout = 120 * time.Second

	messages := []models.ChatMessage{
		{Role: "user", Content: []models.ContentBlock{{Type: "text", Text: in.UserPrompt}}},
	}
	const repairSystem = "You are a TypeScript/TSX and premium admin-UI repair bot. Fix the listed errors: import mismatches, typos, displayName issues, Radix SelectItem empty-value runtime crashes, React infinite-recursion bugs where a component renders itself, admin UI quality failures, AND syntax errors like unbalanced braces/brackets/parentheses. For admin UI quality failures, preserve backend/API contracts and polish only the current file into a product-grade SaaS screen. Output the complete corrected file via the repair_file tool. Never truncate."

	raw, usage, _, err := a.callTool(repairCfg, repairSystem, messages, ToolRepairFile)
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

func (a *AnthropicAgent) DatabaseQuery(_ context.Context, in models.DatabaseQueryInput) (*models.DatabaseActionRequest, error) {
	content := chat_prompts.BuildDatabaseMessage(in.Clarified, in.SchemaText, in.DataContext)
	messages := buildAgentMessages(in.History, []models.ContentBlock{{Type: "text", Text: content}})

	dbCfg := a.conf.Agents.Inspector
	dbCfg.Model = a.conf.ClaudeModel
	dbCfg.Timeout = 120 * time.Second

	body, usage, err := callAnthropicText(a.conf, dbCfg, chat_prompts.PromptDatabaseAssistant, messages)
	a.tracker.RecordUsage(usage, dbCfg.Model, "Database query")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, fmt.Errorf("database query: %w", err)
	}

	text, err := extractPlainText(body)
	if err != nil {
		return nil, fmt.Errorf("database query: extract text: %w", err)
	}

	var action models.DatabaseActionRequest
	if err = json.Unmarshal([]byte(ai.ExtractJSONFromText(text)), &action); err != nil {
		return nil, fmt.Errorf("database query: parse action JSON: %w | raw=%.300s", err, text)
	}
	return &action, nil
}

func (a *AnthropicAgent) BuildAgentSpec(_ context.Context, in models.AgentSpecInput) (*models.AgentSpec, error) {
	content := chat_prompts.BuildAgentBuilderMessage(in.Description, in.SchemaText, in.ReferenceDocs)
	messages := buildAgentMessages(in.History, []models.ContentBlock{{Type: "text", Text: content}})

	raw, usage, _, err := a.callTool(a.conf.Agents.AgentBuilder, chat_prompts.PromptAgentBuilder, messages, ToolBuildAgentSpec)
	a.tracker.RecordUsage(usage, a.conf.Agents.AgentBuilder.Model, "Building agent definition")
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

func (a *AnthropicAgent) callTool(agentCfg config.AgentConfig, system string, messages []models.ChatMessage, tool claudeFunctionTool) ([]byte, models.LLMUsage, string, error) {
	wire := wireToolRequest{
		Model:      agentCfg.Model,
		MaxTokens:  agentCfg.MaxTokens,
		Messages:   messages,
		Tools:      []claudeFunctionTool{tool},
		ToolChoice: ForcedTool(tool.Name),
	}
	if system != "" {
		wire.System = []systemBlock{{Type: "text", Text: system, CacheControl: &cacheCtrl{Type: "ephemeral"}}}
	}
	return callAnthropicTool(a.conf, wire, agentCfg.Timeout)
}

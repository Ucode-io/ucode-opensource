package ai

import (
	"context"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

// AutoAgent is a hybrid Agent that routes each pipeline task to the provider
// best suited for it. The routing is fixed at construction time:
//
//	Creation tasks    → claude  (Router, Architect, Manifest, Code generation,
//	                              Inspect, RepairFile)
//	Edit/update tasks → openai  (PlanChanges, EditCode, VisualEdit, DatabaseQuery)
//
// Empirically Claude produces higher-quality generation and intent classification,
// while OpenAI is faster and cheaper on context-heavy edit flows.
type AutoAgent struct {
	claude Agent
	openai Agent
}

// NewAutoAgent wires two ready sub-agents into the hybrid router. Both must be
// non-nil; the AutoAgent does not lazily construct them.
func NewAutoAgent(claude, openai Agent) Agent {
	return &AutoAgent{claude: claude, openai: openai}
}

func (a *AutoAgent) RouteRequest(ctx context.Context, in models.RouterInput) (*models.HaikuRoutingResult, error) {
	return a.claude.RouteRequest(ctx, in)
}

func (a *AutoAgent) ArchitectProject(ctx context.Context, in models.ArchitectInput) (*models.ArchitectPlan, error) {
	return a.claude.ArchitectProject(ctx, in)
}

func (a *AutoAgent) GenerateManifest(ctx context.Context, in models.ManifestInput) (*models.ProjectManifest, error) {
	return a.claude.GenerateManifest(ctx, in)
}

func (a *AutoAgent) PlanChanges(ctx context.Context, in models.PlannerInput) (*models.SonnetPlanResult, error) {
	return a.openai.PlanChanges(ctx, in)
}

func (a *AutoAgent) InspectCode(ctx context.Context, in models.InspectorInput) (string, error) {
	return a.claude.InspectCode(ctx, in)
}

func (a *AutoAgent) EditCode(ctx context.Context, in models.EditorInput) (*models.GeneratedProject, error) {
	return a.openai.EditCode(ctx, in)
}

func (a *AutoAgent) GenerateCode(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt string, images []string, history []models.ChatMessage, desc string) (*models.GeneratedProject, error) {
	return a.claude.GenerateCode(ctx, agentCfg, systemPrompt, userPrompt, images, history, desc)
}

func (a *AutoAgent) GenerateCodeNoHistory(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt, desc string) (*models.GeneratedProject, error) {
	return a.claude.GenerateCodeNoHistory(ctx, agentCfg, systemPrompt, userPrompt, desc)
}

func (a *AutoAgent) VisualEdit(ctx context.Context, in models.VisualEditInput) ([]models.ProjectFile, string, error) {
	return a.openai.VisualEdit(ctx, in)
}

func (a *AutoAgent) RepairFile(ctx context.Context, in models.RepairFileInput) (models.ProjectFile, error) {
	return a.claude.RepairFile(ctx, in)
}

func (a *AutoAgent) DatabaseQuery(ctx context.Context, in models.DatabaseQueryInput) (*models.DatabaseActionRequest, error) {
	return a.openai.DatabaseQuery(ctx, in)
}

func (a *AutoAgent) BuildAgentSpec(ctx context.Context, in models.AgentSpecInput) (*models.AgentSpec, error) {
	return a.claude.BuildAgentSpec(ctx, in)
}

func (a *AutoAgent) IntegrateAgent(ctx context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	return a.claude.IntegrateAgent(ctx, in)
}

func (a *AutoAgent) IntegrateBuilderAgent(ctx context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error) {
	return a.claude.IntegrateBuilderAgent(ctx, in)
}

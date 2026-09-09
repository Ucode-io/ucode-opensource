package ai

import (
	"context"
	"errors"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
)

// ErrMaxTokens is the shared sentinel for "output token limit exceeded" across all providers.
var ErrMaxTokens = errors.New("generation stopped: output exceeded the token limit")

type UsageTracker interface {
	RecordUsage(usage models.LLMUsage, model, desc string)
	Deduct(tokens int64)
}

type Agent interface {
	RouteRequest(ctx context.Context, in models.RouterInput) (*models.HaikuRoutingResult, error)
	ArchitectProject(ctx context.Context, in models.ArchitectInput) (*models.ArchitectPlan, error)
	GenerateManifest(ctx context.Context, in models.ManifestInput) (*models.ProjectManifest, error)
	PlanChanges(ctx context.Context, in models.PlannerInput) (*models.SonnetPlanResult, error)
	InspectCode(ctx context.Context, in models.InspectorInput) (string, error)
	EditCode(ctx context.Context, in models.EditorInput) (*models.GeneratedProject, error)
	GenerateCode(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt string, images []string, history []models.ChatMessage, desc string) (*models.GeneratedProject, error)
	GenerateCodeNoHistory(ctx context.Context, agentCfg config.AgentConfig, systemPrompt, userPrompt, desc string) (*models.GeneratedProject, error)
	VisualEdit(ctx context.Context, in models.VisualEditInput) ([]models.ProjectFile, string, error)
	RepairFile(ctx context.Context, in models.RepairFileInput) (models.ProjectFile, error)
	DatabaseQuery(ctx context.Context, in models.DatabaseQueryInput) (*models.DatabaseActionRequest, error)
	BuildAgentSpec(ctx context.Context, in models.AgentSpecInput) (*models.AgentSpec, error)
	IntegrateAgent(ctx context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error)
	IntegrateBuilderAgent(ctx context.Context, in models.AgentIntegrationInput) ([]models.ProjectFile, string, error)
}

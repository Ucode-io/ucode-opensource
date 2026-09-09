package v1

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
)

// runCreateAgent handles the create_agent intent: it designs a reusable end-user
// agent from the builder's natural-language description, validates the requested
// permissions against the real project schema, persists the agent, and returns a
// builder-facing confirmation.
func (p *ChatProcessor) runCreateAgent(ctx context.Context, clarified string, chatHistory []models.ChatMessage, attachmentURLs []string) (*models.ParsedClaudeResponse, error) {
	emit := p.emitter()

	resourceEnvId, err := p.resolveBuilderResourceID(ctx)
	if err != nil {
		return nil, fmt.Errorf("resolve builder resource ID: %w", err)
	}

	schema, err := p.getProjectSchemaCached(ctx, resourceEnvId)
	if err != nil {
		return nil, fmt.Errorf("schema fetch failed: %w", err)
	}
	if len(schema) == 0 {
		return &models.ParsedClaudeResponse{
			Description: "В этом проекте пока нет таблиц. Сначала создайте таблицы, чтобы агент мог с ними работать.",
		}, nil
	}

	// The builder may attach example/template documents (xlsx/pptx/docx) on the same
	// URL channel as images. Extract their text so the model can bake that exact
	// format into the agent it designs (e.g. the layout of a sample КП).
	var referenceDocs string
	if len(attachmentURLs) > 0 {
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "file-text", Message: "Изучаю прикреплённые документы...", Percent: 3})
		referenceDocs = p.extractReferenceDocsText(ctx, attachmentURLs)
	}

	emit.Emit(SSEEvent{Type: EvProgress, Icon: "bot", Message: "Проектирую агента...", Percent: 5})
	spec, err := p.agent.BuildAgentSpec(ctx, models.AgentSpecInput{
		Description:   clarified,
		SchemaText:    formatSchemaForSQL(schema),
		History:       chatHistory,
		ReferenceDocs: referenceDocs,
	})
	if err != nil {
		return nil, fmt.Errorf("build agent spec: %w", err)
	}

	perms := buildValidatedPermissions(spec.Permissions, schema)
	if len(perms) == 0 {
		return &models.ParsedClaudeResponse{
			Description: "Не удалось определить, к каким таблицам агенту нужен доступ. Пожалуйста, уточните, что именно должен делать агент.",
		}, nil
	}

	name := strings.TrimSpace(spec.Name)
	if name == "" {
		name = "AI Agent"
	}

	emit.Emit(SSEEvent{Type: EvProgress, Icon: "sparkles", Message: fmt.Sprintf("Создаю агента «%s»...", name), Percent: 15})
	agent, err := p.service.GoObjectBuilderService().Agent().CreateAgent(ctx, &nb.CreateAgentRequest{
		ResourceEnvId: resourceEnvId,
		ProjectId:     p.ucodeMcpProjectId,
		Name:          name,
		Description:   spec.Description,
		Instruction:   spec.Instruction,
		Model:         p.agentCfgs().Coder.Model,
		MaxSteps:      defaultAgentMaxSteps,
		Enabled:       true,
		Permissions:   perms,
	})
	if err != nil {
		return nil, fmt.Errorf("create agent: %w", err)
	}

	reply := strings.TrimSpace(spec.Reply)
	if reply == "" {
		reply = fmt.Sprintf("Агент «%s» создан.", agent.GetName())
	}
	description := reply + "\n\n" + formatAgentSummary(agent)

	// Immediately wire the agent into the existing frontend so end-users can reach
	// it. Integration is a best-effort follow-up: the agent is already persisted,
	// so a failure here downgrades to a soft warning instead of failing the request.
	integrationSummary, err := p.integrateAgentIntoFrontend(ctx, agent, clarified, chatHistory)
	if err != nil {
		log.Printf("[AGENT INTEGRATE] failed for agent %s: %v", agent.GetId(), err)
		description += "\n\n⚠️ Агент создан, но не удалось автоматически встроить его в интерфейс. Попробуйте ещё раз позже."
		return &models.ParsedClaudeResponse{Description: description}, nil
	}

	if integrationSummary != "" {
		description += "\n\n" + integrationSummary
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "check-circle", Message: fmt.Sprintf("Агент «%s» готов и подключён", agent.GetName()), Percent: 100})
	}

	return &models.ParsedClaudeResponse{Description: description}, nil
}

// buildValidatedPermissions converts the model's proposed permissions into proto
// rules, keeping only rules that reference a real table slug, are not duplicated,
// and grant at least one operation. This is the trust boundary for the LLM output.
func buildValidatedPermissions(specPerms []models.AgentSpecPermission, schema []models.TableSchema) []*nb.AgentPermission {
	known := make(map[string]bool, len(schema))
	for _, t := range schema {
		known[t.Slug] = true
	}

	seen := make(map[string]bool, len(specPerms))
	perms := make([]*nb.AgentPermission, 0, len(specPerms))
	for _, sp := range specPerms {
		slug := strings.TrimSpace(sp.TableSlug)
		if slug == "" || !known[slug] || seen[slug] {
			continue
		}
		if !sp.CanCreate && !sp.CanRead && !sp.CanUpdate && !sp.CanDelete && !sp.CanList {
			continue
		}
		seen[slug] = true
		perms = append(perms, &nb.AgentPermission{
			TableSlug: slug,
			CanCreate: sp.CanCreate,
			CanRead:   sp.CanRead,
			CanUpdate: sp.CanUpdate,
			CanDelete: sp.CanDelete,
			CanList:   sp.CanList,
		})
	}
	return perms
}

// formatAgentSummary renders a short markdown summary of the created agent for the
// builder: its name, description, and the per-table operations it was granted.
func formatAgentSummary(agent *nb.Agent) string {
	var b strings.Builder

	b.WriteString("**")
	b.WriteString(agent.GetName())
	b.WriteString("**")
	if d := strings.TrimSpace(agent.GetDescription()); d != "" {
		b.WriteString(" — ")
		b.WriteString(d)
	}

	perms := agent.GetPermissions()
	sort.Slice(perms, func(i, j int) bool {
		return perms[i].GetTableSlug() < perms[j].GetTableSlug()
	})

	b.WriteString("\n\nДоступ к данным:\n")
	for _, perm := range perms {
		fmt.Fprintf(&b, "- %s: %s\n", perm.GetTableSlug(), strings.Join(allowedOps(perm), ", "))
	}

	return b.String()
}

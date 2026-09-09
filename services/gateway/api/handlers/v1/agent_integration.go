package v1

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
)

// These caps bound how many existing files we feed the integrator as full content.
// Shell/routing files are needed to mount a global chat widget; feature files (the
// pages/forms that touch the agent's tables) are needed to wire an action-triggered
// agent into the right place. Small caps keep the prompt focused and the token cost
// predictable.
const (
	agentShellFileCap   = 8
	agentFeatureFileCap = 10
)

// integrateAgentIntoFrontend wires a freshly-created end-user agent into the
// already-generated frontend. It injects the authoritative networking templates
// (agentClient.ts + useAgent.ts) and asks the model to build and mount an
// on-brand widget that talks to the agent through them. It returns a short,
// builder-facing summary of what was wired in; ("", nil) means there is no
// frontend bound to this chat and the step was skipped.
func (p *ChatProcessor) integrateAgentIntoFrontend(ctx context.Context, agent *nb.Agent, userRequest string, chatHistory []models.ChatMessage) (string, error) {
	if p.microFrontendId == "" || p.microFrontendRepoId == "" {
		log.Printf("[AGENT INTEGRATE] no microfrontend bound (id=%q repo=%q) — skipping integration", p.microFrontendId, p.microFrontendRepoId)
		return "", nil
	}

	emit := p.emitter()

	if err := p.Check(); err != nil {
		return "", err
	}

	emit.Emit(SSEEvent{Type: EvProgress, Icon: "scan-search", Message: "Загружаю файлы проекта...", Percent: 25})
	existingFiles, err := p.fetchMicrofrontendFiles(ctx)
	if err != nil {
		return "", fmt.Errorf("integrate agent: fetch frontend files: %w", err)
	}

	templateFiles := agentTemplateFiles()

	emit.Emit(SSEEvent{Type: EvProgress, Icon: "plug", Message: "Готовлю клиент агента...", Percent: 35})
	relevantPaths := selectAgentIntegrationFiles(existingFiles, agent.GetPermissions())
	filesContext := p.buildMicrofrontendFilesContext(existingFiles, relevantPaths)
	fileGraphJSON := p.buildMicrofrontendFileGraphJSON(existingFiles)

	message := chat_prompts.BuildAgentIntegrationMessage(models.AgentIntegrationView{
		AgentName:     agent.GetName(),
		AgentID:       agent.GetId(),
		Purpose:       agent.GetDescription(),
		Capabilities:  formatAgentCapabilities(agent.GetPermissions()),
		UserRequest:   userRequest,
		TemplateFiles: agentTemplatePaths(),
		FileGraphJSON: fileGraphJSON,
		FilesContext:  filesContext,
	})
	messages := buildMessagesWithHistory(chatHistory, buildContentBlocksWithImages(message, nil))

	emit.Emit(SSEEvent{Type: EvProgress, Icon: "code-2", Message: "Встраиваю агента в интерфейс...", Percent: 45})

	if err := p.Check(); err != nil {
		return "", err
	}

	var (
		editedFiles   []models.ProjectFile
		changeSummary string
	)
	if err := withHeartbeat(ctx, emit,
		p.agentCfgs().Coder.Model,
		[]string{
			"Проектирую виджет агента...",
			"Подключаю чат к интерфейсу...",
			"Согласую стиль с дизайн-системой...",
			"Монтирую виджет в оболочку приложения...",
			"Финализирую интеграцию...",
		},
		func() error {
			var e error
			editedFiles, changeSummary, e = p.agent.IntegrateAgent(ctx, models.AgentIntegrationInput{Messages: messages})
			return e
		},
	); err != nil {
		return "", fmt.Errorf("integrate agent: model call: %w", err)
	}

	mergedFiles := mergeAgentFiles(editedFiles, templateFiles)
	if len(mergedFiles) == 0 {
		log.Printf("[AGENT INTEGRATE] model returned no files — nothing to push")
		return "", nil
	}

	// Build lookup sets to tell updates apart from new files and template injections.
	existingPathSet := make(map[string]bool, len(existingFiles))
	for _, f := range existingFiles {
		existingPathSet[f.FilePath] = true
	}
	templatePathSet := make(map[string]bool, len(templateFiles))
	for _, f := range templateFiles {
		templatePathSet[f.Path] = true
	}

	var updateCount, createCount int
	for _, f := range mergedFiles {
		if existingPathSet[f.Path] {
			updateCount++
		} else {
			createCount++
		}
	}

	// Summary event mirrors the "N изменить · M создать" format from the edit flow.
	summaryParts := make([]string, 0, 2)
	if updateCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d изменить", updateCount))
	}
	if createCount > 0 {
		summaryParts = append(summaryParts, fmt.Sprintf("%d создать", createCount))
	}
	emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    "file-diff",
		Message: "Применяю изменения в интерфейсе",
		Value:   strings.Join(summaryParts, " · "),
		Percent: 84,
	})

	// Per-file publish events — progressive percent (86→96) and icon by extension,
	// matching the microfrontend update flow exactly.
	total := len(mergedFiles)
	for i, f := range mergedFiles {
		pct := 86 + (i+1)*10/total
		if pct > 96 {
			pct = 96
		}

		var message string
		icon := agentFileIcon(f.Path)
		switch {
		case templatePathSet[f.Path]:
			message = "Добавляю клиент агента"
		case existingPathSet[f.Path]:
			message = "Обновляю файл"
		default:
			message = "Создаю файл"
			icon = "file-plus"
		}

		emit.Emit(SSEEvent{
			Type:    EvPublish,
			Icon:    icon,
			Message: message,
			Value:   f.Path,
			Percent: pct,
		})
	}

	emit.Emit(SSEEvent{
		Type:    EvPublish,
		Icon:    "upload-cloud",
		Message: "Пушу изменения в GitLab",
		Value:   fmt.Sprintf("%d файлов", total),
		Percent: 97,
	})

	if err := p.pushMicrofrontendChangesChunked(ctx, mergedFiles); err != nil {
		return "", fmt.Errorf("integrate agent: push changes: %w", err)
	}

	p.createMicrofrontendSnapshot(ctx, buildAgentSnapshot(existingFiles, mergedFiles), "Agent integrated into frontend.")

	summary := strings.TrimSpace(changeSummary)
	if summary == "" {
		summary = "Агент подключён к интерфейсу приложения."
	}
	log.Printf("[AGENT INTEGRATE] ✅ done — %d files pushed, summary=%s", len(mergedFiles), summary)
	return summary, nil
}

// selectAgentIntegrationFiles picks the existing files the integrator must see in
// full to do its job: the data-access helpers (so it reads created/updated records
// from the correct response envelope), the app shell/routing (so a global chat widget
// can be mounted everywhere), plus the feature files that work with the agent's own
// tables (so an action-triggered agent can be wired into the right form/page — e.g. a
// company create form whose onSubmit should call the agent). Data-layer files come
// first; all groups are deduped and the shell/feature groups individually capped.
func selectAgentIntegrationFiles(files []models.GitlabFileChange, perms []*nb.AgentPermission) []string {
	dataLayer := selectAgentDataLayerFiles(files)
	shell := selectAgentShellFiles(files)
	feature := selectAgentFeatureFiles(files, perms)

	ordered := make([]string, 0, len(dataLayer)+len(shell)+len(feature))
	ordered = append(ordered, dataLayer...)
	ordered = append(ordered, shell...)
	ordered = append(ordered, feature...)

	seen := make(map[string]bool, len(ordered))
	selected := make([]string, 0, len(ordered))
	for _, path := range ordered {
		if seen[path] {
			continue
		}
		seen[path] = true
		selected = append(selected, path)
	}
	return selected
}

// selectAgentDataLayerFiles finds the project's API/data-access helpers — the hooks
// that talk to the backend (useApi) and the response unwrappers (apiUtils). The
// integrator must see these in full: a mutation's result is the RAW response envelope
// ({ data: { count, response } }), so wiring an action-triggered agent into an
// onSuccess callback requires unwrapping the created record with extractSingle before
// reading its guid. Feeding the real source removes any guesswork about the envelope
// and prevents the agent from being wired in but never actually firing.
func selectAgentDataLayerFiles(files []models.GitlabFileChange) []string {
	hints := []string{"lib/apiutils.", "hooks/useapi."}

	matched := make([]string, 0, len(hints))
	for _, f := range files {
		lower := strings.ToLower(f.FilePath)
		if !strings.HasPrefix(lower, "src/") {
			continue
		}
		for _, h := range hints {
			if strings.Contains(lower, h) {
				matched = append(matched, f.FilePath)
				break
			}
		}
	}

	sort.Strings(matched)
	return matched
}

// selectAgentShellFiles picks the files most likely to be the app shell (the
// layout/root that wraps every page) plus routing, so the integrator can mount a
// global widget in the right place. It matches on path keywords, caps the count
// to keep the prompt focused, and returns the paths in stable order.
func selectAgentShellFiles(files []models.GitlabFileChange) []string {
	keywords := []string{
		"app.tsx", "app.jsx", "main.tsx", "main.jsx",
		"layout", "appshell", "shell",
		"sidebar", "navbar", "topnav",
		"appproviders", "providers", "routes", "router",
	}

	matched := make([]string, 0, agentShellFileCap)
	for _, f := range files {
		lower := strings.ToLower(f.FilePath)
		if !strings.HasPrefix(lower, "src/") {
			continue
		}
		for _, kw := range keywords {
			if strings.Contains(lower, kw) {
				matched = append(matched, f.FilePath)
				break
			}
		}
	}

	sort.Strings(matched)
	if len(matched) > agentShellFileCap {
		matched = matched[:agentShellFileCap]
	}
	return matched
}

// selectAgentFeatureFiles finds the frontend files that work with the agent's own
// tables — the pages, forms and components where a granted table slug appears in the
// file path or body. These are the wiring points for an action-triggered agent (for
// example, the company create form whose submit handler should call the agent).
// Files are scored so the clearly-relevant ones survive the cap: a slug in the path
// is a strong signal, a slug in the body a weaker one, and form/page-shaped paths
// get a small boost.
func selectAgentFeatureFiles(files []models.GitlabFileChange, perms []*nb.AgentPermission) []string {
	slugs := make([]string, 0, len(perms))
	for _, perm := range perms {
		if slug := strings.ToLower(strings.TrimSpace(perm.GetTableSlug())); slug != "" {
			slugs = append(slugs, slug)
		}
	}
	if len(slugs) == 0 {
		return nil
	}

	type scoredFile struct {
		path  string
		score int
	}

	ranked := make([]scoredFile, 0, len(files))
	for _, f := range files {
		lowerPath := strings.ToLower(f.FilePath)
		if !strings.HasPrefix(lowerPath, "src/") {
			continue
		}
		lowerContent := strings.ToLower(f.Content)

		score := 0
		for _, slug := range slugs {
			if strings.Contains(lowerPath, slug) {
				score += 3
			} else if strings.Contains(lowerContent, slug) {
				score++
			}
		}
		if score == 0 {
			continue
		}
		if containsAny(lowerPath, "form", "create", "edit", "new", "page", "detail") {
			score += 2
		}
		ranked = append(ranked, scoredFile{path: f.FilePath, score: score})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].path < ranked[j].path
	})

	matched := make([]string, 0, agentFeatureFileCap)
	for _, r := range ranked {
		if len(matched) >= agentFeatureFileCap {
			break
		}
		matched = append(matched, r.path)
	}
	return matched
}

// containsAny reports whether s contains any of the given substrings.
func containsAny(s string, subs ...string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// formatAgentCapabilities renders the agent's per-table permissions as a short
// bulleted list (one line per table, sorted by slug) so the integrator can word
// the widget's empty state and input placeholder around what the agent can do.
func formatAgentCapabilities(perms []*nb.AgentPermission) string {
	if len(perms) == 0 {
		return ""
	}
	sorted := make([]*nb.AgentPermission, len(perms))
	copy(sorted, perms)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].GetTableSlug() < sorted[j].GetTableSlug()
	})

	var sb strings.Builder
	for _, perm := range sorted {
		fmt.Fprintf(&sb, "- %s: %s\n", perm.GetTableSlug(), strings.Join(allowedOps(perm), ", "))
	}
	return sb.String()
}

// agentFileIcon returns the Lucide icon name for a file based on its extension,
// matching the same palette used in the microfrontend update flow.
func agentFileIcon(path string) string {
	switch {
	case strings.HasSuffix(path, ".css") || strings.HasSuffix(path, ".scss"):
		return "paintbrush"
	case strings.HasSuffix(path, ".tsx") || strings.HasSuffix(path, ".jsx"):
		return "component"
	case strings.HasSuffix(path, ".ts") || strings.HasSuffix(path, ".js"):
		return "code-2"
	default:
		return "file-code"
	}
}

// mergeAgentFiles combines the model's output with the authoritative template
// files. Templates always win: any model-produced file at a reserved template
// path is dropped, then the templates are appended verbatim. This guarantees the
// networking layer the widget depends on is exactly what we ship.
func mergeAgentFiles(modelFiles, templateFiles []models.ProjectFile) []models.ProjectFile {
	reserved := make(map[string]bool, len(templateFiles))
	for _, f := range templateFiles {
		reserved[f.Path] = true
	}

	merged := make([]models.ProjectFile, 0, len(modelFiles)+len(templateFiles))
	for _, f := range modelFiles {
		if reserved[f.Path] {
			continue
		}
		merged = append(merged, f)
	}
	return append(merged, templateFiles...)
}

// buildAgentSnapshot produces a full-state snapshot for versioning: every
// existing file, with changed files overridden and newly created files appended.
// Reverting to this snapshot restores the whole frontend to a consistent state.
func buildAgentSnapshot(existing []models.GitlabFileChange, changed []models.ProjectFile) []models.GitlabFileChange {
	changedMap := make(map[string]string, len(changed))
	for _, f := range changed {
		changedMap[f.Path] = f.Content
	}

	snapshot := make([]models.GitlabFileChange, 0, len(existing)+len(changed))
	for _, f := range existing {
		if newContent, ok := changedMap[f.FilePath]; ok {
			snapshot = append(snapshot, models.GitlabFileChange{FilePath: f.FilePath, Content: newContent})
			delete(changedMap, f.FilePath)
		} else {
			snapshot = append(snapshot, f)
		}
	}
	for _, f := range changed {
		if _, isNew := changedMap[f.Path]; isNew {
			snapshot = append(snapshot, models.GitlabFileChange{FilePath: f.Path, Content: f.Content})
		}
	}
	return snapshot
}

package v1

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// integrateBuilderAgentIntoFrontend injects the SSE client templates and asks the
// model to build and mount an on-brand chat widget for the fixed builder assistant.
// It is best-effort and runs only for admin panels: the app is already published, so
// the caller downgrades any error to a log. ("", nil) means no microfrontend is bound.
func (p *ChatProcessor) integrateBuilderAgentIntoFrontend(ctx context.Context, chatHistory []models.ChatMessage) (string, error) {
	if p.microFrontendId == "" || p.microFrontendRepoId == "" {
		log.Printf("[BUILDER INTEGRATE] no microfrontend bound — skipping")
		return "", nil
	}

	emit := p.emitter()
	if err := p.Check(); err != nil {
		return "", err
	}

	// Integration runs at the tail of generation where publishing already reached
	// ~99%; events omit Percent so the bar holds until the pipeline's terminal event.
	emit.Emit(SSEEvent{Type: EvProgress, Icon: IconScanSearch, Message: "Загружаю файлы проекта..."})
	existingFiles, err := p.fetchMicrofrontendFiles(ctx)
	if err != nil {
		return "", fmt.Errorf("fetch frontend files: %w", err)
	}

	templateFiles := builderAgentTemplateFiles()

	// The widget is global, so the integrator only needs the app shell to mount it.
	emit.Emit(SSEEvent{Type: EvProgress, Icon: IconPlug, Message: "Готовлю AI-ассистента..."})
	message := chat_prompts.BuildBuilderAgentIntegrationMessage(models.BuilderAgentIntegrationView{
		TemplateFiles: builderAgentTemplatePaths(),
		FileGraphJSON: p.buildMicrofrontendFileGraphJSON(existingFiles),
		FilesContext:  p.buildMicrofrontendFilesContext(existingFiles, selectAgentShellFiles(existingFiles)),
	})
	messages := buildMessagesWithHistory(chatHistory, buildContentBlocksWithImages(message, nil))

	emit.Emit(SSEEvent{Type: EvProgress, Icon: IconCode, Message: "Встраиваю AI-ассистента в интерфейс..."})
	if err := p.Check(); err != nil {
		return "", err
	}

	editedFiles, changeSummary, err := p.agent.IntegrateBuilderAgent(ctx, models.AgentIntegrationInput{Messages: messages})
	if err != nil {
		return "", fmt.Errorf("model call: %w", err)
	}

	mergedFiles := mergeAgentFiles(editedFiles, templateFiles)
	if len(mergedFiles) == 0 {
		log.Printf("[BUILDER INTEGRATE] model returned no files — nothing to push")
		return "", nil
	}

	existingPaths := make(map[string]bool, len(existingFiles))
	for _, f := range existingFiles {
		existingPaths[f.FilePath] = true
	}
	templatePaths := make(map[string]bool, len(templateFiles))
	for _, f := range templateFiles {
		templatePaths[f.Path] = true
	}

	for _, f := range mergedFiles {
		label, icon := "Создаю файл", agentFileIcon(f.Path)
		switch {
		case templatePaths[f.Path]:
			label = "Добавляю клиент ассистента"
		case existingPaths[f.Path]:
			label = "Обновляю файл"
		default:
			icon = IconFilePlus
		}
		emit.Emit(SSEEvent{Type: EvPublish, Icon: icon, Message: label, Value: f.Path})
	}

	emit.Emit(SSEEvent{
		Type:    EvPublish,
		Icon:    IconUploadCloud,
		Message: "Пушу изменения в GitLab",
		Value:   fmt.Sprintf("%d файлов", len(mergedFiles)),
	})

	if err := p.pushMicrofrontendChangesChunked(ctx, mergedFiles); err != nil {
		return "", fmt.Errorf("push changes: %w", err)
	}
	p.createMicrofrontendSnapshot(ctx, buildAgentSnapshot(existingFiles, mergedFiles), "Builder assistant integrated into frontend.")

	summary := strings.TrimSpace(changeSummary)
	if summary == "" {
		summary = "AI-ассистент подключён к интерфейсу приложения."
	}
	log.Printf("[BUILDER INTEGRATE] done — %d files pushed", len(mergedFiles))
	return summary, nil
}

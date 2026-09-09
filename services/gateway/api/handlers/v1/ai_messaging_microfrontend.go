package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	cs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	publishChunkSize = 30
	pushRetryCount   = 3
	pushRetryDelay   = 2 * time.Second

	mfeFilesPath   = "/v2/functions/micro-frontend/files"
	mfePushPath    = "/v2/functions/micro-frontend/push-changes"
	mfePublishPath = "/v2/functions/micro-frontend/publish-ai"

	editChunkMaxFiles       = 5
	editChunkMaxConcurrency = 5
	editChunkMaxAttempts    = 2
	editChunkRetryDelay     = 2 * time.Second
)

type (
	// editChunk is a bounded set of files edited by one model call.
	editChunk struct {
		index  int
		change []models.FilePlan
		create []models.FilePlan
	}

	editChunkResult struct {
		chunk editChunk
		files []models.ProjectFile
		err   error
	}
)

// runMicrofrontendEdit fetches the current files from u-gen, asks the AI to edit
// them, then pushes the result back to u-gen. No McpProject is touched.
func (p *ChatProcessor) runMicrofrontendEdit(ctx context.Context, clarified, fileGraphJSON string, chatHistory []models.ChatMessage, imageURLs []string, existingFiles []models.GitlabFileChange) (*models.ParsedClaudeResponse, error) {
	log.Printf("[MICROFE EDIT] planning changes for microfrontend id=%s", p.microFrontendId)
	p.ensureEditPromptOverrides(ctx)

	emit := p.emitter()
	emit.Emit(SSEEvent{Type: EvProgress, Icon: IconScanSearch, Message: "Анализирую проект и планирую изменения...", Percent: 5})

	if err := p.Check(); err != nil {
		return nil, err
	}

	var (
		reference        *models.ReferenceSiteContext
		referenceMessage string
	)
	clarified, imageURLs, reference, referenceMessage = prepareReferencePrompt(ctx, p.baseConf, clarified, imageURLs)
	if referenceMessage != "" {
		return &models.ParsedClaudeResponse{Description: referenceMessage}, nil
	}
	if reference != nil {
		emit.Emit(SSEEvent{
			Type:    EvProgress,
			Icon:    "scan-eye",
			Percent: 8,
			Message: referenceCaptureProgressMessage(reference),
			Value:   reference.URL,
		})
	}

	// Phase 1: plan which files to change or create.
	var plan *models.SonnetPlanResult
	if err := withHeartbeat(ctx, emit,
		p.agentCfgs().Planner.Model,
		[]string{
			"Анализирую структуру проекта...",
			"Определяю файлы для редактирования...",
			"Планирую порядок изменений...",
			"Проверяю зависимости между файлами...",
		},
		func() error {
			var e error
			plan, e = p.agent.PlanChanges(ctx, models.PlannerInput{
				Clarified:     clarified,
				FileGraphJSON: fileGraphJSON,
				HasImages:     len(imageURLs) > 0,
				History:       chatHistory,
			})
			return e
		},
	); err != nil {
		return nil, err
	}
	log.Printf("[MICROFE EDIT] planner: files_to_change=%d files_to_create=%d", len(plan.FilesToChange), len(plan.FilesToCreate))

	emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    IconFileDiff,
		Message: "План изменений готов",
		Value:   fmt.Sprintf("%d изменить · %d создать", len(plan.FilesToChange), len(plan.FilesToCreate)),
		Percent: 18,
	})
	time.Sleep(800 * time.Millisecond)

	for _, f := range plan.FilesToChange {
		emit.Emit(SSEEvent{Type: EvProgress, Icon: IconFileEdit, Message: f.Description, Value: f.Path})
	}
	for _, f := range plan.FilesToCreate {
		emit.Emit(SSEEvent{Type: EvProgress, Icon: IconFilePlus, Message: f.Description, Value: f.Path})
	}

	if err := p.Check(); err != nil {
		return nil, err
	}

	// Phase 2: edit the planned files in bounded, parallel chunks.
	edited, failed := p.runChunkedEdit(ctx, clarified, plan, imageURLs, chatHistory, existingFiles)

	if len(edited) == 0 {
		log.Printf("[MICROFE EDIT] editor produced no files — nothing to push")
		return &models.ParsedClaudeResponse{Description: buildChunkedUpdateSummary(plan, nil, failed, p.userMessage)}, nil
	}

	// Phase 3: publish.
	if err := p.applyEditedFiles(ctx, edited, existingFiles); err != nil {
		return nil, err
	}

	return &models.ParsedClaudeResponse{Description: buildChunkedUpdateSummary(plan, edited, failed, p.userMessage)}, nil
}

// retries is isolated: its files are reported as failed and left untouched
// rather than failing the whole update.
func (p *ChatProcessor) runChunkedEdit(ctx context.Context, clarified string, plan *models.SonnetPlanResult, imageURLs []string, chatHistory []models.ChatMessage, existingFiles []models.GitlabFileChange) (edited []models.ProjectFile, failed []models.FilePlan) {
	emit := p.emitter()

	chunks := planEditChunks(plan)
	total := len(chunks)
	if total == 0 {
		return nil, nil
	}

	tree := buildProjectTree(existingFiles)

	fullPlanJSON, _ := json.Marshal(plan)

	log.Printf("[MICROFE EDIT] chunked edit: %d chunk(s), max %d files each", total, editChunkMaxFiles)

	fileCount := len(plan.FilesToChange) + len(plan.FilesToCreate)
	modeEvent := SSEEvent{
		Type:    EvProgress,
		Icon:    IconCode,
		Message: "Редактирую код за один проход",
		Value:   fmt.Sprintf("%d файлов", fileCount),
		Percent: 20,
		Data:    map[string]any{"mode": "single", "chunks": total, "files": fileCount},
	}
	if total > 1 {
		modeEvent.Icon = IconZap
		modeEvent.Message = "Редактирую код параллельно"
		modeEvent.Value = fmt.Sprintf("%d групп · %d файлов", total, fileCount)
		modeEvent.Data = map[string]any{"mode": "chunked", "chunks": total, "files": fileCount}
	}
	emit.Emit(modeEvent)

	results := make(chan editChunkResult, total)
	sem := make(chan struct{}, editChunkMaxConcurrency)

	stopHB := make(chan struct{})
	go chunkEditHeartbeat(emit, stopHB)

	for _, chunk := range chunks {
		go func(chunk editChunk) {
			sem <- struct{}{}
			defer func() { <-sem }()

			chunkFileCount := len(chunk.change) + len(chunk.create)
			emit.Emit(SSEEvent{
				Type:    EvChunkStart,
				Icon:    IconPackage,
				Message: fmt.Sprintf("Редактирую группу %d/%d", chunk.index, total),
				Value:   fmt.Sprintf("%d файлов", chunkFileCount),
				Data:    map[string]any{"index": chunk.index, "total": total, "files": chunkFileCount},
			})

			files, err := p.runEditChunk(ctx, chunk, clarified, string(fullPlanJSON), imageURLs, chatHistory, tree, existingFiles)
			results <- editChunkResult{chunk: chunk, files: files, err: err}
		}(chunk)
	}

	completed := 0
	for range chunks {
		res := <-results
		completed++
		pct := 20 + completed*62/total // 20 → 82

		chunkFiles := append(append([]models.FilePlan{}, res.chunk.change...), res.chunk.create...)

		if res.err != nil || len(res.files) == 0 {
			failed = append(failed, chunkFiles...)
			log.Printf("[MICROFE EDIT] chunk %d/%d skipped (%d file(s) left unchanged): %v", completed, total, len(chunkFiles), res.err)
			emit.Emit(SSEEvent{
				Type:    EvWarning,
				Icon:    IconAlertTriangle,
				Message: fmt.Sprintf("Группа пропущена (%d/%d)", completed, total),
				Value:   fmt.Sprintf("%d файлов без изменений", len(chunkFiles)),
				Percent: pct,
				Data:    map[string]any{"index": res.chunk.index, "total": total, "files": len(chunkFiles)},
			})
			continue
		}

		edited = append(edited, res.files...)

		produced := make(map[string]bool, len(res.files))
		for _, f := range res.files {
			produced[f.Path] = true
		}
		for _, fp := range chunkFiles {
			if !produced[fp.Path] {
				failed = append(failed, fp)
			}
		}

		emit.Emit(SSEEvent{
			Type:    EvChunkDone,
			Icon:    IconCheckCircle,
			Message: fmt.Sprintf("Группа готова (%d/%d)", completed, total),
			Value:   fmt.Sprintf("%d файлов", len(res.files)),
			Percent: pct,
			Data: ChunkDoneData{
				Feature: fmt.Sprintf("Группа %d", res.chunk.index),
				Index:   res.chunk.index,
				Total:   total,
				Files:   res.files,
			},
		})
	}
	close(stopHB)

	return dedupeProjectFiles(edited), failed
}

func (p *ChatProcessor) runEditChunk(ctx context.Context, chunk editChunk, clarified, fullPlanJSON string, imageURLs []string, chatHistory []models.ChatMessage, tree string, existingFiles []models.GitlabFileChange) ([]models.ProjectFile, error) {
	subPlan := &models.SonnetPlanResult{
		FilesToChange: chunk.change,
		FilesToCreate: chunk.create,
	}
	filesContext := p.buildEditChunkContext(chunk, existingFiles, tree)

	var lastErr error
	for attempt := 1; attempt <= editChunkMaxAttempts; attempt++ {
		project, err := p.agent.EditCode(ctx, models.EditorInput{
			Clarified:      clarified,
			Plan:           subPlan,
			FullPlanJSON:   fullPlanJSON,
			FilesContext:   filesContext,
			Images:         imageURLs,
			History:        chatHistory,
			Chunked:        true,
			PromptOverride: p.editPromptOverrides.CodeEditor,
		})
		if err == nil {
			if project == nil {
				return nil, nil
			}
			return filterEditChunkFiles(project.Files, chunk), nil
		}

		lastErr = err
		log.Printf("[MICROFE EDIT] chunk %d attempt %d/%d failed: %v", chunk.index, attempt, editChunkMaxAttempts, err)
		if attempt < editChunkMaxAttempts {
			time.Sleep(editChunkRetryDelay)
		}
	}
	return nil, lastErr
}

func filterProjectFilesByPath(files []models.ProjectFile, allowed map[string]bool) []models.ProjectFile {
	if len(files) == 0 || len(allowed) == 0 {
		return nil
	}

	filtered := make([]models.ProjectFile, 0, len(files))
	for _, file := range files {
		if allowed[file.Path] {
			filtered = append(filtered, file)
		}
	}
	return filtered
}

func planEditChunks(plan *models.SonnetPlanResult) []editChunk {
	var (
		chunks []editChunk
		cur    = editChunk{index: 1}
	)

	flush := func() {
		if len(cur.change) == 0 && len(cur.create) == 0 {
			return
		}
		chunks = append(chunks, cur)
		cur = editChunk{index: len(chunks) + 1}
	}

	add := func(fp models.FilePlan, isCreate bool) {
		if len(cur.change)+len(cur.create) >= editChunkMaxFiles {
			flush()
		}
		if isCreate {
			cur.create = append(cur.create, fp)
		} else {
			cur.change = append(cur.change, fp)
		}
	}

	for _, fp := range plan.FilesToChange {
		add(fp, false)
	}
	for _, fp := range plan.FilesToCreate {
		add(fp, true)
	}
	flush()

	return chunks
}

func buildProjectTree(files []models.GitlabFileChange) string {
	if len(files) == 0 {
		return ""
	}
	paths := make([]string, 0, len(files))
	for _, f := range files {
		paths = append(paths, f.FilePath)
	}
	sort.Strings(paths)

	var sb strings.Builder
	sb.WriteString("PROJECT FILE TREE (read-only — resolve imports against these paths; do NOT recreate a file unless it is in the plan):\n")
	for _, path := range paths {
		sb.WriteString("- ")
		sb.WriteString(path)
		sb.WriteByte('\n')
	}
	return sb.String()
}

func (p *ChatProcessor) buildEditChunkContext(chunk editChunk, existing []models.GitlabFileChange, tree string) string {
	paths := make([]string, 0, len(chunk.change))
	for _, fp := range chunk.change {
		paths = append(paths, fp.Path)
	}

	var sb strings.Builder
	if tree != "" {
		sb.WriteString(tree)
		sb.WriteByte('\n')
	}
	sb.WriteString(p.buildMicrofrontendFilesContext(existing, paths))
	return sb.String()
}

func filterEditChunkFiles(files []models.ProjectFile, chunk editChunk) []models.ProjectFile {
	allowed := make(map[string]bool, len(chunk.change)+len(chunk.create))
	for _, fp := range chunk.change {
		allowed[fp.Path] = true
	}
	for _, fp := range chunk.create {
		allowed[fp.Path] = true
	}

	out := make([]models.ProjectFile, 0, len(files))
	for _, f := range files {
		if allowed[f.Path] && strings.TrimSpace(f.Content) != "" {
			out = append(out, f)
		}
	}
	return out
}

func dedupeProjectFiles(files []models.ProjectFile) []models.ProjectFile {
	if len(files) <= 1 {
		return files
	}
	indexByPath := make(map[string]int, len(files))
	out := make([]models.ProjectFile, 0, len(files))
	for _, f := range files {
		if i, ok := indexByPath[f.Path]; ok {
			out[i] = f
			continue
		}
		indexByPath[f.Path] = len(out)
		out = append(out, f)
	}
	return out
}

func chunkEditHeartbeat(emit ProgressEmitter, stop <-chan struct{}) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	messages := []string{
		"Редактирую компоненты...",
		"Вношу изменения в логику...",
		"Обновляю стили и разметку...",
		"Проверяю совместимость импортов...",
		"Финализирую правки...",
	}
	i := 0
	for {
		select {
		case <-ticker.C:
			emit.Emit(SSEEvent{Type: EvProgress, Icon: IconBrain, Message: messages[i%len(messages)]})
			i++
		case <-stop:
			return
		}
	}
}

func (p *ChatProcessor) applyEditedFiles(ctx context.Context, edited []models.ProjectFile, existingFiles []models.GitlabFileChange) error {
	emit := p.emitter()

	edited = enforceAuthRuntime(edited, existingFiles)

	log.Printf("[MICROFE EDIT] pushing %d file(s) to u-gen branch", len(edited))

	for i, f := range edited {
		pct := 86 + (i+1)*10/len(edited) // 86 → 96
		if pct > 96 {
			pct = 96
		}
		emit.Emit(SSEEvent{Type: EvPublish, Icon: editFileIcon(f.Path), Message: "Обновляю файл", Value: f.Path, Percent: pct})
	}

	emit.Emit(SSEEvent{
		Type:    EvPublish,
		Icon:    IconUploadCloud,
		Message: "Пушу изменения в GitLab",
		Value:   fmt.Sprintf("%d файлов", len(edited)),
		Percent: 97,
	})
	if err := p.pushMicrofrontendChangesChunked(ctx, edited); err != nil {
		return fmt.Errorf("failed to push microfrontend changes: %w", err)
	}

	p.createMicrofrontendSnapshot(ctx, buildFullSnapshot(existingFiles, edited), "Changes applied successfully.")
	return nil
}

func editFileIcon(path string) string {
	switch {
	case strings.HasSuffix(path, ".css"):
		return IconPaintbrush
	case strings.HasSuffix(path, ".tsx"), strings.HasSuffix(path, ".jsx"):
		return IconComponent
	case strings.HasSuffix(path, ".ts"):
		return IconCode
	default:
		return IconFileCode
	}
}

// buildFullSnapshot merges edited files into the full existing file list so that
// reverting to this snapshot restores every file to a consistent state.
func buildFullSnapshot(existing []models.GitlabFileChange, edited []models.ProjectFile) []models.GitlabFileChange {
	editedByPath := make(map[string]string, len(edited))
	for _, f := range edited {
		editedByPath[f.Path] = f.Content
	}

	snapshot := make([]models.GitlabFileChange, 0, len(existing)+len(edited))
	for _, f := range existing {
		if content, changed := editedByPath[f.FilePath]; changed {
			snapshot = append(snapshot, models.GitlabFileChange{FilePath: f.FilePath, Content: content})
			delete(editedByPath, f.FilePath)
		} else {
			snapshot = append(snapshot, f)
		}
	}
	// Append newly created files not present in the existing list.
	for _, f := range edited {
		if _, isNew := editedByPath[f.Path]; isNew {
			snapshot = append(snapshot, models.GitlabFileChange{FilePath: f.Path, Content: f.Content})
		}
	}
	return snapshot
}

// runMicrofrontendInspect answers questions about the microfrontend's current code
// by loading the requested files from the u-gen branch.
func (p *ChatProcessor) runMicrofrontendInspect(ctx context.Context, userQuestion string, filesNeeded []string, chatHistory []models.ChatMessage, imageURLs []string, existingFiles []models.GitlabFileChange) (*models.ParsedClaudeResponse, error) {
	if err := p.Check(); err != nil {
		return nil, err
	}

	filesContext := p.buildMicrofrontendFilesContext(existingFiles, filesNeeded)
	answer, err := p.agent.InspectCode(ctx, models.InspectorInput{
		Question:     userQuestion,
		FilesContext: filesContext,
		Images:       imageURLs,
		History:      chatHistory,
	})
	if err != nil {
		return nil, err
	}
	return &models.ParsedClaudeResponse{Description: answer}, nil
}

func (p *ChatProcessor) fetchMicrofrontendFiles(ctx context.Context) ([]models.GitlabFileChange, error) {
	url := p.baseConf.GoFunctionServiceHost + p.baseConf.GoFunctionServiceHTTPPort +
		mfeFilesPath + "?repo_id=" + p.microFrontendRepoId

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Authorization", p.authToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("function service call: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("function service returned %d: %s", resp.StatusCode, string(respBytes))
	}

	// Response shape: {"status":"...","data":{"files":[{"path":"...","content":"..."}]}}
	var result struct {
		Data struct {
			Files []struct {
				Path    string `json:"path"`
				Content string `json:"content"`
			} `json:"files"`
		} `json:"data"`
	}
	if err = json.Unmarshal(respBytes, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	files := make([]models.GitlabFileChange, 0, len(result.Data.Files))
	for _, f := range result.Data.Files {
		files = append(files, models.GitlabFileChange{
			FilePath: f.Path,
			Content:  f.Content,
		})
	}
	log.Printf("[MICROFE EDIT] fetched %d files from u-gen", len(files))
	return files, nil
}

// pushMicrofrontendChanges отправляет файлы в push-changes одним запросом.
// Используется для небольших правок (edit flow).
func (p *ChatProcessor) pushMicrofrontendChanges(ctx context.Context, generatedFiles []models.ProjectFile) error {
	repoIDInt := 0
	fmt.Sscanf(p.microFrontendRepoId, "%d", &repoIDInt)
	if repoIDInt == 0 {
		return fmt.Errorf("invalid microfrontend_repo_id: %q", p.microFrontendRepoId)
	}

	files := make([]models.GitlabFileChange, 0, len(generatedFiles))
	for _, f := range generatedFiles {
		files = append(files, models.GitlabFileChange{
			FilePath: f.Path,
			Content:  f.Content,
		})
	}

	return p.doPushChanges(ctx, repoIDInt, files)
}

// и отправляет каждый с retry. Используется при публикации (121+ файл).
func (p *ChatProcessor) pushMicrofrontendChangesChunked(ctx context.Context, generatedFiles []models.ProjectFile) error {
	repoIDInt := 0
	fmt.Sscanf(p.microFrontendRepoId, "%d", &repoIDInt)
	if repoIDInt == 0 {
		return fmt.Errorf("invalid microfrontend_repo_id: %q", p.microFrontendRepoId)
	}

	files := make([]models.GitlabFileChange, 0, len(generatedFiles))
	for _, f := range generatedFiles {
		files = append(files, models.GitlabFileChange{
			FilePath: f.Path,
			Content:  f.Content,
		})
	}

	total := len(files)
	chunks := splitIntoChunks(files, publishChunkSize)
	log.Printf("[MICROFE PUSH] pushing %d files in %d chunk(s) of max %d", total, len(chunks), publishChunkSize)

	for i, chunk := range chunks {
		log.Printf("[MICROFE PUSH] chunk %d/%d: %d files", i+1, len(chunks), len(chunk))

		var lastErr error
		for attempt := 1; attempt <= pushRetryCount; attempt++ {
			lastErr = p.doPushChanges(ctx, repoIDInt, chunk)
			if lastErr == nil {
				log.Printf("[MICROFE PUSH] ✅ chunk %d/%d done", i+1, len(chunks))
				break
			}
			log.Printf("[MICROFE PUSH] chunk %d/%d attempt %d/%d failed: %v", i+1, len(chunks), attempt, pushRetryCount, lastErr)
			if attempt < pushRetryCount {
				time.Sleep(pushRetryDelay)
			}
		}

		if lastErr != nil {
			return fmt.Errorf("push chunk %d/%d failed after %d attempts: %w", i+1, len(chunks), pushRetryCount, lastErr)
		}
	}

	log.Printf("[MICROFE PUSH] ✅ all %d files pushed successfully", total)
	return nil
}

// doPushChanges выполняет один HTTP-запрос к push-changes endpoint.
func (p *ChatProcessor) doPushChanges(ctx context.Context, repoIDInt int, files []models.GitlabFileChange) error {
	if len(files) == 0 {
		return fmt.Errorf("doPushChanges: no files provided")
	}

	type pushReq struct {
		RepoID                int                       `json:"repo_id"`
		Files                 []models.GitlabFileChange `json:"files"`
		CommitMessage         string                    `json:"commit_message"`
		FunctionID            string                    `json:"function_id"`
		ResourceEnvironmentID string                    `json:"resource_environment_id"`
	}

	bodyBytes, err := json.Marshal(pushReq{
		RepoID:                repoIDInt,
		Files:                 files,
		CommitMessage:         p.userMessage,
		FunctionID:            p.microFrontendId,
		ResourceEnvironmentID: p.microFrontendResourceEnvId,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	url := p.baseConf.GoFunctionServiceHost + p.baseConf.GoFunctionServiceHTTPPort + mfePushPath

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", p.authToken)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		return fmt.Errorf("function service call: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("function service returned %d: %s", resp.StatusCode, string(respBytes))
	}
	return nil
}

// splitIntoChunks делит слайс на части размером не более chunkSize.
func splitIntoChunks(files []models.GitlabFileChange, chunkSize int) [][]models.GitlabFileChange {
	if chunkSize <= 0 {
		chunkSize = publishChunkSize
	}
	var chunks [][]models.GitlabFileChange
	for len(files) > 0 {
		end := chunkSize
		if end > len(files) {
			end = len(files)
		}
		chunks = append(chunks, files[:end])
		files = files[end:]
	}
	return chunks
}

// buildMicrofrontendFileGraphJSON builds the same file graph JSON that the router
// uses, from a flat list of GitlabFileChange entries (no per-file graph data).
func (p *ChatProcessor) buildMicrofrontendFileGraphJSON(files []models.GitlabFileChange) string {
	if len(files) == 0 {
		return "{}"
	}
	graph := make(map[string]models.GraphNode, len(files))
	for _, f := range files {
		graph[f.FilePath] = models.GraphNode{Path: f.FilePath}
	}
	jsonBytes, err := json.Marshal(graph)
	if err != nil {
		return "{}"
	}
	return string(jsonBytes)
}

// buildMicrofrontendFilesContext returns the file contents for the paths the
// planner requested, formatted for the code-editor prompt.
func (p *ChatProcessor) buildMicrofrontendFilesContext(files []models.GitlabFileChange, paths []string) string {
	if len(paths) == 0 || len(files) == 0 {
		return "No existing files to modify."
	}
	pathSet := make(map[string]bool, len(paths))
	for _, path := range paths {
		pathSet[path] = true
	}
	var sb strings.Builder
	for _, f := range files {
		if pathSet[f.FilePath] {
			sb.WriteString(fmt.Sprintf("\n\n### FILE: %s\n```\n%s\n```", f.FilePath, f.Content))
		}
	}
	if sb.Len() == 0 {
		return "No matching files found."
	}
	return sb.String()
}

// publishToMicrofrontend публикует сгенерированный проект в microfrontend.
//
// Стратегия (исправленная):
//  1. publish-ai — создаём GitLab репо, передаём ВСЕ файлы сразу.
//     Endpoint требует минимум один коммит с реальными файлами для инициализации ветки u-gen.
//  2. Если publish-ai вернул ошибку "no files provided" или 5xx — повторяем с retry.
//
// Фаза 2 (push-changes) больше НЕ используется при публикации, т.к. publish-ai
// сам делает первый коммит. push-changes используется только для последующих правок (edit flow).
func (p *ChatProcessor) publishToMicrofrontend(ctx context.Context, projectName, path string, generated *models.ParsedClaudeResponse, projectData *models.ProjectData, projectType string) (string, error) {
	if generated == nil || generated.Project == nil || len(generated.Project.Files) == 0 {
		return "", fmt.Errorf("no generated files to publish")
	}

	// Build the sanitized file list once.
	allFiles := make([]models.GitlabFileChange, 0, len(generated.Project.Files))
	for _, f := range generated.Project.Files {
		cleanPath := strings.TrimLeft(f.Path, "/")
		if cleanPath == "" {
			continue
		}
		allFiles = append(allFiles, models.GitlabFileChange{
			FilePath: cleanPath,
			Content:  sanitizeFileContent(f.Content),
		})
	}

	if len(allFiles) == 0 {
		return "", fmt.Errorf("no valid files to publish after sanitization")
	}

	safeName := slugify(projectName)
	if safeName == "" {
		safeName = "ai-project"
	}
	safePath := slugify(path)
	if safePath == "" {
		safePath = "app"
	}

	// ── Phase 1: создаём GitLab репо с ПЕРВЫМ чанком файлов ──────────────────
	// publish-ai требует реальные файлы для инициализации ветки u-gen.
	// Передаём первые publishChunkSize файлов (включая package.json если есть).
	// Остальные файлы пушим через push-changes в Phase 2.

	// Сортируем: package.json всегда первым чтобы repo корректно инициализировался.
	initFiles := buildInitFiles(allFiles, publishChunkSize)

	log.Printf("[MICROFRONTEND] publish-ai: name=%q path=%q project_id=%s env_id=%s init_files=%d total_files=%d",
		safeName, safePath, projectData.UcodeProjectId, projectData.EnvironmentId, len(initFiles), len(allFiles))

	createBody := models.PublishAiMicroFrontendRequest{
		ProjectId:        projectData.UcodeProjectId,
		EnvironmentId:    projectData.EnvironmentId,
		Name:             safeName,
		Path:             safePath,
		FrameworkType:    "REACT",
		Files:            initFiles,
		McpProjectId:     projectData.McpProjectId,
		McpResourceEnvId: p.resourceEnvId,
	}

	createBytes, err := json.Marshal(createBody)
	if err != nil {
		return "", fmt.Errorf("marshal create request: %w", err)
	}

	createURL := p.baseConf.GoFunctionServiceHost + p.baseConf.GoFunctionServiceHTTPPort + mfePublishPath

	var createResult models.PublishAiMicroFrontendResponse

	// Retry publish-ai до 3 раз — иногда сервис даёт 500 на первом запросе.
	for attempt := 1; attempt <= pushRetryCount; attempt++ {
		createResult, err = p.doPublishAI(ctx, createURL, createBytes)
		if err == nil {
			break
		}
		log.Printf("[MICROFRONTEND] publish-ai attempt %d/%d failed: %v", attempt, pushRetryCount, err)
		if attempt < pushRetryCount {
			time.Sleep(pushRetryDelay)
		}
	}
	if err != nil {
		return "", fmt.Errorf("microfrontend publish failed: %w", err)
	}

	p.microFrontendId = createResult.Data.ID
	p.microFrontendRepoId = createResult.Data.RepoId
	if p.microFrontendResourceEnvId == "" {
		p.microFrontendResourceEnvId = projectData.ResourceEnvId
	}
	if projectData.McpProjectId != "" && p.resourceEnvId != "" {
		updateReq := &nb.McpProject{
			ResourceEnvId:       p.resourceEnvId,
			Id:                  projectData.McpProjectId,
			MicrofrontendId:     createResult.Data.ID,
			MicrofrontendRepoId: createResult.Data.RepoId,
			MicrofrontendBranch: createResult.Data.Branch,
			MicrofrontendUrl:    createResult.Data.Url,
			ProjectType:         projectType,
		}

		if resolveGeneratedAuthMode(projectData, projectType) == "login" {
			projectEnvMap := make(map[string]any)
			currentMcpProject, fetchErr := p.service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(ctx, &nb.McpProjectId{
				ResourceEnvId: p.resourceEnvId,
				Id:            projectData.McpProjectId,
				WithoutFiles:  true,
			})
			if fetchErr == nil && currentMcpProject != nil && currentMcpProject.GetProjectEnv() != nil {
				for key, value := range currentMcpProject.GetProjectEnv().AsMap() {
					projectEnvMap[key] = value
				}
			}
			projectEnvMap["auth_mode"] = "login"
			if navRoutes := buildNavRoutesEnvPayload(p.navRoutes); len(navRoutes) > 0 {
				projectEnvMap["nav_routes"] = navRoutes
			}
			projectEnv, convertErr := helperFunc.ConvertMapToStruct(projectEnvMap)
			if convertErr != nil {
				return "", fmt.Errorf("convert MCP project env: %w", convertErr)
			}
			updateReq.ProjectEnv = projectEnv
		}

		if _, updateErr := p.service.GoObjectBuilderService().McpProject().UpdateMcpProject(ctx, updateReq); updateErr != nil {
			return "", fmt.Errorf("save microfrontend refs on MCP project: %w", updateErr)
		}
	}

	if shortURL := p.createShortLink(ctx, createResult.Data.Url, projectData); shortURL != "" {
		projectData.ShortURL = shortURL
	}

	log.Printf("[MICROFRONTEND] repo created: id=%s repo_id=%s url=%s", createResult.Data.ID, createResult.Data.RepoId, createResult.Data.Url)

	// ── Phase 2: пушим оставшиеся файлы через push-changes (если они есть) ───
	// Файлы которые уже были в initFiles — пропускаем (они уже в репо).
	initSet := make(map[string]struct{}, len(initFiles))
	for _, f := range initFiles {
		initSet[f.FilePath] = struct{}{}
	}
	remainingFiles := make([]models.ProjectFile, 0, len(allFiles))
	for _, f := range allFiles {
		if _, alreadyPushed := initSet[f.FilePath]; !alreadyPushed {
			remainingFiles = append(remainingFiles, models.ProjectFile{
				Path:    f.FilePath,
				Content: f.Content,
			})
		}
	}

	if len(remainingFiles) > 0 {
		log.Printf("[MICROFRONTEND] push-changes: pushing remaining %d files in chunks", len(remainingFiles))
		if pushErr := p.pushMicrofrontendChangesChunked(ctx, remainingFiles); pushErr != nil {
			// НЕ фейлим весь деплой — репо уже создан, частичный результат лучше нуля.
			log.Printf("[MICROFRONTEND] ⚠️ push-changes partial failure (repo exists, %d files missing): %v", len(remainingFiles), pushErr)
		}
	}

	p.createMicrofrontendSnapshot(ctx, allFiles, p.userMessage)

	log.Printf("[MICROFRONTEND] ✅ published: id=%s url=%s", p.microFrontendId, createResult.Data.Url)
	return createResult.Data.Url, nil
}

// doPublishAI выполняет один HTTP-запрос к publish-ai endpoint.
func (p *ChatProcessor) doPublishAI(ctx context.Context, url string, bodyBytes []byte) (models.PublishAiMicroFrontendResponse, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("build create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", p.authToken)

	client := &http.Client{Timeout: timeoutPublishMicrofrontend}
	createResp, err := client.Do(httpReq)
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("publish-ai call failed: %w", err)
	}
	defer createResp.Body.Close()

	createRespBytes, err := io.ReadAll(createResp.Body)
	if err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("read publish-ai response: %w", err)
	}
	if createResp.StatusCode >= 400 {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("publish-ai returned %d: %s", createResp.StatusCode, string(createRespBytes))
	}

	var result models.PublishAiMicroFrontendResponse
	if err = json.Unmarshal(createRespBytes, &result); err != nil {
		return models.PublishAiMicroFrontendResponse{}, fmt.Errorf("parse publish-ai response: %w", err)
	}
	return result, nil
}

// buildInitFiles возвращает первые n файлов для phase 1, гарантируя что package.json идёт первым.
func buildInitFiles(allFiles []models.GitlabFileChange, n int) []models.GitlabFileChange {
	if n <= 0 || n >= len(allFiles) {
		return allFiles
	}

	result := make([]models.GitlabFileChange, 0, n)

	// Сначала ищем package.json — он нужен для инициализации React-репо.
	pkgIdx := -1
	for i, f := range allFiles {
		if f.FilePath == "package.json" {
			pkgIdx = i
			break
		}
	}

	if pkgIdx >= 0 {
		result = append(result, allFiles[pkgIdx])
	}

	// Добавляем остальные файлы до лимита n.
	for i, f := range allFiles {
		if len(result) >= n {
			break
		}
		if i == pkgIdx {
			continue // уже добавили
		}
		result = append(result, f)
	}

	return result
}

// snapshotExcluded returns true for files that should never be stored in a
// snapshot: lockfiles, CI/CD configs, infra files, and env files. These are
// never edited by AI and can be enormous (e.g. package-lock.json).
func snapshotExcluded(path string) bool {
	switch path {
	case "package-lock.json", "yarn.lock", "pnpm-lock.yaml", ".pnp.js",
		".gitignore", ".gitattributes",
		".gitlab-ci.yml", "Dockerfile", "Makefile", "nginx.conf",
		"README.md", "CHANGELOG.md", "LICENSE":
		return true
	}
	for _, prefix := range []string{".gitlab/", ".husky/", ".github/", "node_modules/"} {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return path == ".env" || strings.HasPrefix(path, ".env.")
}

// createMicrofrontendSnapshot saves a version of the current files to the
// child project's microfrontend_versions table. Sets is_current=true on the new
// version and is_current=false on all previous versions for this microfrontend.
// Called after every successful AI push — NOT after reverts.
func (p *ChatProcessor) createMicrofrontendSnapshot(ctx context.Context, files []models.GitlabFileChange, commitMessage string) {
	if p.microFrontendId == "" || p.microFrontendResourceEnvId == "" {
		log.Printf("[VERSION] skipping snapshot: microFrontendId=%q resourceEnvId=%q", p.microFrontendId, p.microFrontendResourceEnvId)
		return
	}

	filtered := make([]models.GitlabFileChange, 0, len(files))
	for _, f := range files {
		if !snapshotExcluded(f.FilePath) {
			filtered = append(filtered, f)
		}
	}

	filesJSON, err := json.Marshal(filtered)
	if err != nil {
		log.Printf("[VERSION] failed to marshal files: %v", err)
		return
	}

	log.Printf("[VERSION] creating snapshot: microfrontend=%s files=%d (filtered from %d)", p.microFrontendId, len(filtered), len(files))

	_, err = p.service.GoObjectBuilderService().MicrofrontendVersions().CreateVersion(ctx, &nb.CreateMicrofrontendVersionRequest{
		ResourceEnvId:   p.microFrontendResourceEnvId,
		MicrofrontendId: p.microFrontendId,
		CommitMessage:   commitMessage,
		Files:           string(filesJSON),
	})
	if err != nil {
		log.Printf("[VERSION] failed to create version: %v", err)
	}
}

// sanitizeFileContent removes characters that can cause JSON parse failures
// in downstream services: null bytes, BOM, and other C0 control characters
// except standard whitespace (tab=0x09, newline=0x0A, carriage-return=0x0D).
func sanitizeFileContent(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			sb.WriteRune(r)
		case r == '\uFEFF': // BOM — strip silently
		case r < 0x20: // other C0 control characters — strip
		default:
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func (p *ChatProcessor) createShortLink(ctx context.Context, mfeURL string, projectData *models.ProjectData) string {
	if p.companyServices == nil || mfeURL == "" {
		return ""
	}

	mfeURL = "https://" + mfeURL

	var link *cs.MfeShortLink
	for range 5 {
		slug, err := generateSlug()
		if err != nil {
			log.Printf("[SHORT_LINK] rand error: %v", err)
			return ""
		}
		link, err = p.companyServices.MfeShortLink().Create(ctx, &cs.MfeShortLink{
			Slug:         slug,
			Url:          mfeURL,
			ProjectId:    projectData.UcodeProjectId,
			McpProjectId: projectData.McpProjectId,
			FunctionId:   p.microFrontendId,
		})
		if err == nil {
			break
		}
		if status.Code(err) != codes.AlreadyExists {
			log.Printf("[SHORT_LINK] non-retryable error: %v", err)
			return ""
		}
		link = nil
	}
	if link == nil {
		return ""
	}

	if p.h != nil && p.h.centralRedis != nil {
		_ = p.h.centralRedis.Set(context.Background(), mfeShortLinkRedisPrefix+link.Slug, link.GetUrl(), mfeShortLinkRedisTTL).Err()
	}

	return mfeShortURL(p.baseConf.ShortURLBase, link.Slug)
}

// buildNavRoutesEnvPayload converts manifest routes into a stable, UI-friendly
// list persisted on the MCP project's project_env (nav_routes). It drops dynamic
// (param/splat) routes that can't be nav targets and derives a human label from
// the page name so the custom-permission editor can offer a routes dropdown.
func buildNavRoutesEnvPayload(routes []models.ManifestRoute) []any {
	seen := make(map[string]bool, len(routes))
	out := make([]any, 0, len(routes))
	for _, r := range routes {
		path := strings.TrimSpace(r.Path)
		if path == "" || strings.ContainsAny(path, ":*") || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, map[string]any{
			"path":  path,
			"label": navRouteLabel(r),
		})
	}
	return out
}

// navRouteLabel produces a readable label, e.g. "PaymentGatewaysPage" -> "Payment Gateways".
func navRouteLabel(r models.ManifestRoute) string {
	if label := splitCamel(strings.TrimSuffix(r.PageName, "Page")); label != "" {
		return label
	}
	if r.Path == "/" || r.Path == "" {
		return "Dashboard"
	}
	cleaned := strings.NewReplacer("/", " ", "-", " ", "_", " ").Replace(strings.Trim(r.Path, "/"))
	return titleWords(cleaned)
}

// splitCamel turns "PaymentGateways" into "Payment Gateways".
func splitCamel(s string) string {
	var b strings.Builder
	for i, ch := range s {
		if i > 0 && ch >= 'A' && ch <= 'Z' {
			b.WriteByte(' ')
		}
		b.WriteRune(ch)
	}
	return strings.TrimSpace(b.String())
}

// titleWords capitalizes each space-separated word.
func titleWords(s string) string {
	parts := strings.Fields(s)
	for i, p := range parts {
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, " ")
}

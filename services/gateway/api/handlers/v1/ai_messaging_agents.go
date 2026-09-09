package v1

import (
	"context"
	"errors"
	"fmt"
	"log"
	"maps"
	"strings"
	"sync"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	chat_prompts2 "github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
)

type chunkResult struct {
	group   models.ManifestGroup
	project *models.GeneratedProject
	err     error
}

func (p *ChatProcessor) RecordUsage(usage models.LLMUsage, model, description string) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	projectId := p.ucodeProjectId
	if p.mcpUcodeProjectId != "" {
		projectId = p.mcpUcodeProjectId
	}

	// Report how many of these tokens are pack-funded so company-service debits the
	// company pool and excludes them from the fare day/month metrics. splitBudget is
	// read-only and the paired Deduct applies the same split, so the two agree.
	var packTokens int64
	if p.tokenBudgetEnabled {
		_, packTokens = p.splitBudget(int64(usage.InputTokens + usage.OutputTokens))
	}

	go func() {
		_, recErr := p.service.CompanyService().Billing().RecordAiTokenUsage(
			context.Background(),
			&pb.RecordAiTokenUsageRequest{
				ProjectId:    projectId,
				CompanyId:    p.companyId,
				InputTokens:  int32(usage.InputTokens),
				OutputTokens: int32(usage.OutputTokens),
				Model:        model,
				Description:  description,
				Product:      config.PRODUCT_TYPE_UGEN,
				PackTokens:   packTokens,
			},
		)
		if recErr != nil {
			log.Printf("[TOKEN RECORD] error recording usage for %s: %v", description, recErr)
		}
	}()
}

func (p *ChatProcessor) generateCode(ctx context.Context, clarified string, imageURLs []string, chatHistory []models.ChatMessage, plan *models.ArchitectPlan, projectData *models.ProjectData) (*models.ParsedClaudeResponse, error) {
	switch plan.ProjectType {
	case "admin_panel", "webapp", mobileProjectType:
		log.Println("GENERATION CODE: generateCodeChunked")
		return p.generateCodeChunkedApplication(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	case "web":
		log.Println("GENERATION CODE: generateCodeChunkedWebsite")
		return p.generateCodeChunkedWebsite(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	default:
		log.Println("GENERATION CODE: generateCodeSingle (landing)")
		return p.generateCodeSingle(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	}
}

func (p *ChatProcessor) generateCodeSingle(ctx context.Context, clarified string, imageURLs []string, chatHistory []models.ChatMessage, plan *models.ArchitectPlan, projectData *models.ProjectData) (*models.ParsedClaudeResponse, error) {
	apiConfig := buildAPIConfigBlock(p.baseConf.UcodeBaseUrl, projectData, plan)
	if plan.ProjectType == mobileProjectType {
		apiConfig += "\n\n" + capacitorPromptAddendum(plan.MobileCapabilities)
	}
	prompt := clarified + "\n\n" + apiConfig
	if !plan.CloneMode && p.cachedImagePool != nil && p.cachedImagePool.Err == nil {
		prompt += "\n\n" + p.cachedImagePool.Block
		p.emitter().Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: fmt.Sprintf("Подобрано %d фото: %s", p.cachedImagePool.Count, strings.Join(p.cachedImagePool.Keywords, ", ")), Percent: 18})
		p.cachedImagePool = nil
	} else if !plan.CloneMode && p.baseConf.UnsplashAccessKey != "" {
		p.emitter().Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: "Подбираю изображения для проекта...", Percent: 17})
		pool := helper.FetchImagePool(ctx, p.baseConf.UnsplashAccessKey, plan)
		if pool.Err != nil {
			p.emitter().Emit(SSEEvent{Type: EvProgress, Icon: "alert-triangle", Message: "Unsplash: не удалось подобрать фото", Value: pool.Err.Error()})
		} else {
			prompt += "\n\n" + pool.Block
			p.emitter().Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: fmt.Sprintf("Подобрано %d фото по запросу: %s", pool.Count, strings.Join(pool.Keywords, ", ")), Percent: 18})
		}
	}

	var scaffoldFiles []models.ProjectFile
	if plan.ProjectType == "admin_panel" || plan.ProjectType == "web" || usesWebAppGenerator(plan.ProjectType) {
		contextFiles := GetTemplateContext()
		scaffoldFiles = GetTemplateScaffold()
		if len(contextFiles) > 0 {
			var templateCtx strings.Builder
			templateCtx.WriteString("\n====================================\n")
			templateCtx.WriteString("PRE-BUILT UTILITIES — MANDATORY USAGE\n")
			templateCtx.WriteString("====================================\n")
			templateCtx.WriteString("The following files ALREADY EXIST in the project. You MUST import from them.\n")
			templateCtx.WriteString("NEVER re-implement these utilities. NEVER output these files in your response.\n\n")
			templateCtx.WriteString("REQUIRED IMPORTS (use exactly these paths):\n")
			templateCtx.WriteString("  import { useApiQuery, useApiMutation } from '@/hooks/useApi'\n")
			templateCtx.WriteString("  import { extractList, extractSingle, extractCount } from '@/lib/apiUtils'\n")
			templateCtx.WriteString("  import { cn, formatDate, formatCurrency, getInitials } from '@/lib/utils'\n")
			templateCtx.WriteString("  import { AppProviders } from '@/components/shared/AppProviders' (wrap root in App.tsx)\n\n")
			templateCtx.WriteString("FILE CONTENTS FOR REFERENCE:\n")
			for _, f := range contextFiles {
				fmt.Fprintf(&templateCtx, "\n### %s\n```typescript\n%s\n```\n", f.Path, f.Content)
			}
			prompt += templateCtx.String()
		}
	}

	p.emitter().Emit(SSEEvent{Type: EvProgress, Icon: "code-2", Message: "Генерирую исходный код проекта...", Percent: 18})

	var project *models.GeneratedProject
	if err := withHeartbeat(ctx, p.emitter(),
		p.agentCfgs().Coder.Model,
		[]string{
			"Генерирую React компоненты...",
			"Создаю страницы и формы...",
			"Пишу бизнес-логику...",
			"Настраиваю роутинг и навигацию...",
			"Подключаю API хуки к таблицам...",
			"Настраиваю CSS стили и темы...",
			"Создаю layout и sidebar...",
			"Генерирую CRUD операции...",
			"Подключаю валидацию форм...",
			"Финализирую код проекта...",
		},
		func() error {
			var e error
			var systemPrompt string
			switch plan.ProjectType {
			case "web":
				systemPrompt = chat_prompts2.PromptWebsiteGenerator
			case "admin_panel":
				systemPrompt = chat_prompts2.PromptAdminPanelGenerator
			case "webapp", mobileProjectType:
				systemPrompt = chat_prompts2.PromptWebAppGenerator
			default:
				systemPrompt = chat_prompts2.PromptLandingGenerator
			}
			project, e = p.agent.GenerateCode(
				ctx,
				p.agentCfgs().LandingCoder,
				systemPrompt,
				prompt,
				imageURLs,
				chatHistory,
				"Generating project code",
			)
			return e
		},
	); err != nil {
		return nil, fmt.Errorf("generate code: %w", err)
	}

	if len(project.Files) == 0 {
		return nil, fmt.Errorf("generate code: claude returned empty project")
	}

	project.Files = stripForbiddenConfigFiles(project.Files)

	project.Files = mergeTemplateScaffold(project.Files, scaffoldFiles)

	project.Files = injectMissingCriticalFiles(project.Files, plan.ProjectType)

	// Always force-inject credentials — Claude may guess wrong values.
	project.Files = injectEnvFile(project.Files, p.baseConf.UcodeBaseUrl, projectData, plan.ProjectType)
	if plan.ProjectType == mobileProjectType {
		var scaffoldErr error
		project.Files, scaffoldErr = applyCapacitorScaffold(project.Files, plan.ProjectName, p.mcpProjectId, plan.MobileCapabilities)
		if scaffoldErr != nil {
			return nil, scaffoldErr
		}
	}

	// ── POST-GENERATION VALIDATION + REPAIR ──
	errorCount := p.validateAndRepairGeneratedProject(ctx, project, plan.ProjectType, plan.MobileCapabilities, 83)
	if plan.ProjectType == mobileProjectType {
		// Gate ONLY on the Capacitor contract; UI-quality/manifest findings are
		// best-effort (parity with webapp, which ships with them).
		if contractErrs := mobileContractErrorCount(project.Files); contractErrs > 0 {
			return nil, fmt.Errorf("generate code: Capacitor mobile contract gate found %d error(s)", contractErrs)
		}
		if errorCount > 0 {
			log.Printf("[generate] mobile shipping with %d non-fatal quality finding(s) (parity with webapp)", errorCount)
		}
	}

	log.Printf("[generate] done: %d files (type=%s, %d validation errors)", len(project.Files), plan.ProjectType, errorCount)
	return &models.ParsedClaudeResponse{Project: project}, nil
}

func (p *ChatProcessor) generateCodeChunkedApplication(ctx context.Context, clarified string, imageURLs []string, chatHistory []models.ChatMessage, plan *models.ArchitectPlan, projectData *models.ProjectData) (*models.ParsedClaudeResponse, error) {
	log.Printf("[chunked] starting chunked generation for %s: %s", plan.ProjectType, plan.ProjectName)

	var (
		emit     = p.emitter()
		manifest *models.ProjectManifest
	)

	if p.prebuiltManifest != nil {
		manifest = p.prebuiltManifest
		p.prebuiltManifest = nil
		p.currentManifest = manifest
		p.navRoutes = manifest.Routes
		defer func() { p.currentManifest = nil }()
		log.Printf("[chunked] using prebuilt manifest: %d groups", len(manifest.Groups))
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "list-tree", Message: "Структура файлов готова", Percent: 23})
	} else {
		return nil, fmt.Errorf("generate code: no manifest available")
	}

	var (
		foundationGroup models.ManifestGroup
		uiKitGroup      models.ManifestGroup
		featureGroups   []models.ManifestGroup

		totalFiles = 0
	)

	for _, g := range manifest.Groups {
		totalFiles += len(g.Files)

		switch g.ID {
		case 0:
			foundationGroup = g
		case 1:
			uiKitGroup = g
		default:
			featureGroups = append(featureGroups, g)
		}
	}

	if len(foundationGroup.Files) == 0 || len(featureGroups) == 0 {
		log.Printf("[chunked] manifest missing foundation or features — falling back to single call")
		return nil, fmt.Errorf("generate code: foundation or features missing in manifest")
	}

	var featureNames = make([]string, 0, len(featureGroups))

	for _, group := range featureGroups {
		featureNames = append(featureNames, group.Name)
	}

	emit.Emit(
		SSEEvent{
			Type:    EvManifest,
			Icon:    "git-branch",
			Percent: 23,
			Message: "Структура проекта спланирована",
			Value:   fmt.Sprintf("%d файлов · %d фич", totalFiles, len(featureGroups)),
			Data: ManifestEventData{
				TotalFiles:   totalFiles,
				GroupCount:   len(manifest.Groups),
				FeatureNames: featureNames,
			},
		},
	)
	log.Printf("[chunked] manifest: foundation=%d files, uikit=%d files, feature_groups=%d", len(foundationGroup.Files), len(uiKitGroup.Files), len(featureGroups))

	time.Sleep(1000 * time.Millisecond)

	apiConfig := buildAPIConfigBlock(p.baseConf.UcodeBaseUrl, projectData, plan)
	if plan.ProjectType == mobileProjectType {
		apiConfig += "\n\n" + capacitorPromptAddendum(plan.MobileCapabilities)
	}
	if !plan.CloneMode && p.cachedImagePool != nil && p.cachedImagePool.Err == nil {
		apiConfig += "\n\n" + p.cachedImagePool.Block
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: fmt.Sprintf("Подобрано %d фото: %s", p.cachedImagePool.Count, strings.Join(p.cachedImagePool.Keywords, ", ")), Percent: 25})
		p.cachedImagePool = nil
	} else if !plan.CloneMode && p.baseConf.UnsplashAccessKey != "" {
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: "Подбираю изображения для проекта...", Percent: 24})
		pool := helper.FetchImagePool(ctx, p.baseConf.UnsplashAccessKey, plan)
		if pool.Err != nil {
			emit.Emit(SSEEvent{Type: EvProgress, Icon: "alert-triangle", Message: "Unsplash: не удалось подобрать фото", Value: pool.Err.Error()})
		} else {
			apiConfig += "\n\n" + pool.Block
			emit.Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: fmt.Sprintf("Подобрано %d фото по запросу: %s", pool.Count, strings.Join(pool.Keywords, ", ")), Percent: 25})
		}
	}

	emit.Emit(
		SSEEvent{
			Type:    EvProgress,
			Icon:    "layers",
			Message: "Генерирую фундамент и UI Kit параллельно",
			Value:   fmt.Sprintf("%d + %d файлов", len(foundationGroup.Files), len(uiKitGroup.Files)),
			Percent: 24,
		},
	)

	foundationPrompt := chat_prompts2.PromptAdminPanelGenerator
	if usesWebAppGenerator(plan.ProjectType) {
		foundationPrompt = chat_prompts2.PromptWebAppGenerator
	}

	var (
		foundation *models.GeneratedProject
		uiKit      *models.GeneratedProject
		foundErr   error

		wg sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for attempt := 1; attempt <= 2; attempt++ {
			foundation, foundErr = p.generateFoundation(ctx, clarified, imageURLs, chatHistory, apiConfig, foundationGroup, manifest, foundationPrompt, plan.ProjectType)
			if foundErr == nil {
				break
			}
			if attempt < 2 && !errors.Is(foundErr, ai.ErrMaxTokens) {
				log.Printf("[chunked] foundation attempt %d failed (%v) — retrying", attempt, foundErr)
			}
		}
		if foundErr != nil {
			return
		}
		foundation.Files = filterToGroup(foundation.Files, foundationGroup)
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "check-circle", Message: "Foundation готов", Value: fmt.Sprintf("%d файлов", len(foundation.Files)), Percent: 38})
		log.Printf("[chunked] foundation done: %d files (after filter)", len(foundation.Files))
	}()

	if len(uiKitGroup.Files) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var err error

			stub := buildFoundationStubWithManifest(foundationGroup, manifest)
			uiKit, err = p.generateUIKit(ctx, uiKitGroup, stub, p.agentCfgs().Coder)
			if err != nil {
				log.Printf("[chunked] UI kit parallel failed (%v) — continuing without it", foundErr)
				return
			}
			uiKit.Files = filterToGroup(uiKit.Files, uiKitGroup)
			emit.Emit(SSEEvent{Type: EvProgress, Icon: "check-circle", Message: "UI Kit готов", Value: fmt.Sprintf("%d компонентов", len(uiKit.Files)), Percent: 50})
			log.Printf("[chunked] UI kit done: %d files (after filter)", len(uiKit.Files))
		}()
	}

	heartbeatMessages := []string{
		"Генерирую layout и навигацию...",
		"Создаю UI компоненты и дизайн-систему...",
		"Генерирую TypeScript типы и интерфейсы...",
		"Пишу API хуки и конфигурацию axios...",
		"Формирую глобальные стили и CSS переменные...",
		"Настраиваю роутинг и App.tsx...",
	}
	if plan.ProjectType == "admin_panel" {
		heartbeatMessages = []string{
			"Генерирую layout, sidebar и навигацию...",
			"Создаю UI компоненты и дизайн-систему...",
			"Генерирую TypeScript типы и интерфейсы...",
			"Создаю DataTable, FormModal и PageHeader...",
			"Пишу API хуки и конфигурацию axios...",
			"Настраиваю роутинг и App.tsx...",
		}
	}

	err := withHeartbeat(ctx, emit,
		p.agentCfgs().Coder.Model,
		heartbeatMessages,
		func() error {
			wg.Wait()
			return foundErr
		},
	)
	if err != nil {
		log.Printf("[chunked] foundation failed (%v) — falling back to single call", err)
		return p.generateCodeSingle(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	}

	allSharedFiles := make([]models.ProjectFile, 0, len(foundation.Files)+len(uiKitGroup.Files))
	allSharedFiles = append(allSharedFiles, foundation.Files...)
	if uiKit != nil {
		allSharedFiles = append(allSharedFiles, uiKit.Files...)
	}

	foundationCtx := buildFoundationContext(allSharedFiles)
	manifestSummary := buildManifestSummary(manifest)

	var uiKitAPISummary string

	if uiKit != nil {
		uiKitAPISummary = buildUIKitAPISummary(uiKit.Files)
	}

	chunkPrompt := chat_prompts2.PromptChunkedCoderAdminPanel
	if usesWebAppGenerator(plan.ProjectType) {
		chunkPrompt = chat_prompts2.PromptChunkedCoderWebApp
	}

	totalChunks := len(featureGroups)
	time.Sleep(1000 * time.Millisecond)
	emit.Emit(
		SSEEvent{
			Type:    EvProgress,
			Icon:    "zap",
			Percent: 51,
			Message: "Запускаю параллельную генерацию фич",
			Value:   fmt.Sprintf("%d фич одновременно", totalChunks),
		},
	)

	results := make(chan chunkResult, totalChunks)

	stopChunkHB := make(chan struct{})
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		tick := 0
		for {
			select {
			case <-ticker.C:
				tick++
				pct := 51 + tick*2
				if pct > 76 {
					pct = 76
				}
				emit.Emit(SSEEvent{Type: EvProgress, Icon: "cpu", Message: "Генерирую фичи параллельно...", Percent: pct})
			case <-stopChunkHB:
				return
			}
		}
	}()
	defer close(stopChunkHB)

	for i, group := range featureGroups {
		startDelay := time.Duration(i) * 120 * time.Millisecond
		go func() {
			if startDelay > 0 {
				time.Sleep(startDelay)
			}
			emit.Emit(
				SSEEvent{
					Type:    EvChunkStart,
					Icon:    "package",
					Message: "Генерирую фичу",
					Value:   group.Name,
					Data:    map[string]any{"feature": group.Name},
				},
			)
			proj, chunkErr := p.generateChunkAdminPanel(ctx, group, foundationCtx, manifestSummary, apiConfig, uiKitAPISummary, chunkPrompt)
			results <- chunkResult{group: group, project: proj, err: chunkErr}
		}()
	}

	var (
		successChunks   []*models.GeneratedProject
		failedCount     int
		completedChunks = 0
	)

	for range featureGroups {
		res := <-results
		completedChunks++
		chunkPct := 51 + completedChunks*25/totalChunks
		if res.err != nil {
			log.Printf("[chunked] feature chunk %q error: %v", res.group.Name, res.err)
			failedCount++
			if stub := buildStubChunk(res.group); len(stub.Files) > 0 {
				successChunks = append(successChunks, stub)
				log.Printf("[chunked] injected %d stub file(s) for failed chunk %q", len(stub.Files), res.group.Name)
			}
		} else {
			res.project.Files = filterToGroup(res.project.Files, res.group)
			log.Printf("[chunked] ✅ chunk %q: %d files", res.group.Name, len(res.project.Files))
			emit.Emit(SSEEvent{
				Type:    EvChunkDone,
				Icon:    "check-circle",
				Percent: chunkPct,
				Message: fmt.Sprintf("Фича готова (%d/%d)", completedChunks, totalChunks),
				Value:   res.group.Name,
				Data: ChunkDoneData{
					Feature: res.group.Name,
					Index:   completedChunks,
					Total:   totalChunks,
					Files:   res.project.Files,
				},
			})
			successChunks = append(successChunks, res.project)
		}
	}

	if failedCount == totalChunks {
		return nil, fmt.Errorf("chunked: all %d feature chunks failed", totalChunks)
	}
	if failedCount > 0 {
		log.Printf("[chunked] WARNING: %d/%d feature chunks failed — deploying partial project", failedCount, totalChunks)
	}

	var allChunks []*models.GeneratedProject

	if uiKit != nil {
		allChunks = append(allChunks, uiKit)
	}
	allChunks = append(allChunks, successChunks...)
	merged := mergeChunks(foundation, allChunks)

	merged.Files = stripForbiddenConfigFiles(merged.Files)

	merged.Files = mergeTemplateScaffold(merged.Files, GetTemplateScaffold())

	merged.Files = injectMissingCriticalFiles(merged.Files, plan.ProjectType)
	merged.Files = injectEnvFile(merged.Files, p.baseConf.UcodeBaseUrl, projectData, plan.ProjectType)
	merged.Files = mergeAppRoutes(merged.Files, manifest)
	if plan.ProjectType == mobileProjectType {
		merged.Files, err = applyCapacitorScaffold(merged.Files, plan.ProjectName, p.mcpProjectId, plan.MobileCapabilities)
		if err != nil {
			return nil, err
		}
	}

	emit.Emit(
		SSEEvent{
			Type:    EvProgress,
			Icon:    "shield-check",
			Percent: 80,
			Message: "Проверяю качество кода",
			Value:   fmt.Sprintf("%d файлов", len(merged.Files)),
		},
	)
	errorCount := p.validateAndRepairGeneratedProject(ctx, merged, plan.ProjectType, plan.MobileCapabilities, 80)
	if plan.ProjectType == mobileProjectType {
		// Gate ONLY on the Capacitor contract; UI-quality/manifest findings are
		// best-effort (parity with webapp, which ships with them).
		if contractErrs := mobileContractErrorCount(merged.Files); contractErrs > 0 {
			return nil, fmt.Errorf("generate code: Capacitor mobile contract gate found %d error(s)", contractErrs)
		}
		if errorCount > 0 {
			log.Printf("[chunked] mobile shipping with %d non-fatal quality finding(s) (parity with webapp)", errorCount)
		}
	}

	log.Printf("[chunked] done: %d total files (%d feature groups, %d failed, %d validation errors)", len(merged.Files), totalChunks, failedCount, errorCount)
	return &models.ParsedClaudeResponse{Project: merged}, nil
}

func (p *ChatProcessor) generateFoundation(ctx context.Context, clarified string, imageURLs []string, chatHistory []models.ChatMessage, apiConfig string, foundationGroup models.ManifestGroup, manifest *models.ProjectManifest, foundationPrompt, projectType string) (*models.GeneratedProject, error) {
	var sb strings.Builder

	sb.WriteString("====================================\n")
	sb.WriteString("⚠ CHUNKED GENERATION — FOUNDATION PHASE ONLY ⚠\n")
	sb.WriteString("====================================\n")
	sb.WriteString("You are generating ONLY the foundation (Group 0) files listed below.\n")
	sb.WriteString("Feature page implementations will be generated separately in parallel agents.\n")
	sb.WriteString("EMIT ONLY the files in 'YOUR FILES TO GENERATE'. No exceptions.\n\n")

	sb.WriteString("YOUR FILES TO GENERATE (Group 0 — Foundation):\n")

	for _, f := range foundationGroup.Files {
		fmt.Fprintf(&sb, "  %s  (exports: [%s])\n", f.Path, strings.Join(f.Exports, ", "))
	}

	sb.WriteString("\n====================================\n")
	sb.WriteString("CRITICAL — src/types.ts MUST contain ALL entity interfaces\n")
	sb.WriteString("====================================\n")
	sb.WriteString("src/types.ts is the SINGLE SOURCE OF TRUTH for all entity types.\n")
	sb.WriteString("Feature chunks import entity types ONLY from '@/types' — never from feature files.\n")
	sb.WriteString("For EVERY table in the project, generate a TypeScript interface with ALL fields.\n")
	sb.WriteString("Use exact field slugs from the table schema below as property names.\n")
	sb.WriteString("Every entity interface must include: guid (string), created_at? (string), and all domain fields.\n")
	sb.WriteString("Also include: PaginationParams, SelectOption<T>, FormState.\n")
	sb.WriteString("⚠ DO NOT include NavItem or TableColumn — they are PRE-BUILT in src/types/common.ts.\n")
	sb.WriteString("  Layout files that need NavItem MUST import from '@/types/common', never from '@/types'.\n\n")

	sb.WriteString("\nFEATURE PAGES — add lazy imports + routes to App.tsx but DO NOT implement the page bodies:\n")
	sb.WriteString("(Exact wrapping pattern — Layout parent route, Sidebar path constraints, 404 catch — is fully specified in the GLOBAL ROUTE MAP block below. Follow it literally.)\n")
	for _, g := range manifest.Groups {
		if g.ID == 0 {
			continue
		}
		for _, f := range g.Files {
			if strings.Contains(f.Path, "/pages/") || strings.HasSuffix(f.Path, "Page.tsx") {
				fmt.Fprintf(&sb, "  %s  → component: [%s]\n", f.Path, strings.Join(f.Exports, ", "))
			}
		}
	}

	// Note: ExportConventionBlock + LazyAppTsxExemplar are baked into PromptAdminPanelGenerator
	// (cacheable system prompt) — do NOT re-inject here.
	if routesBlock := buildExpectedRoutesBlock(manifest, projectType); routesBlock != "" {
		sb.WriteString("\n")
		sb.WriteString(routesBlock)
	}
	if typesBlock := buildExpectedEntityTypesBlock(manifest); typesBlock != "" {
		sb.WriteString("\n")
		sb.WriteString(typesBlock)
	}

	sb.WriteString("\nFULL PROJECT MANIFEST (for routing and types reference):\n")
	sb.WriteString(buildManifestSummary(manifest))

	sb.WriteString("\n====================================\n")
	sb.WriteString("PROJECT REQUEST\n")
	sb.WriteString("====================================\n")
	sb.WriteString(clarified)
	sb.WriteString("\n\n")
	sb.WriteString(apiConfig)

	contextFiles := GetTemplateContext()
	if len(contextFiles) > 0 {
		sb.WriteString("\n====================================\n")
		sb.WriteString("PRE-BUILT UTILITIES — MANDATORY USAGE\n")
		sb.WriteString("====================================\n")
		sb.WriteString("The following files ALREADY EXIST in the project. You MUST import from them.\n")
		sb.WriteString("NEVER re-implement these utilities. NEVER output these files in your response.\n\n")
		sb.WriteString("REQUIRED IMPORTS (use exactly these paths):\n")
		sb.WriteString("  import { useApiQuery, useApiMutation } from '@/hooks/useApi'\n")
		sb.WriteString("  import { extractList, extractSingle, extractCount } from '@/lib/apiUtils'\n")
		sb.WriteString("  import { cn, formatDate, formatCurrency, getInitials } from '@/lib/utils'\n")
		sb.WriteString("  import { AppProviders } from '@/components/shared/AppProviders' (wrap root in App.tsx)\n\n")
		sb.WriteString("FILE CONTENTS FOR REFERENCE:\n")
		for _, f := range contextFiles {
			fmt.Fprintf(&sb, "\n### %s\n```typescript\n%s\n```\n", f.Path, f.Content)
		}
	}

	sb.WriteString("\n====================================\n")
	sb.WriteString("REMINDER: Emit ONLY the Group 0 files listed at the top. DO NOT generate feature pages.\n")
	sb.WriteString("====================================\n")

	project, err := p.agent.GenerateCode(ctx,
		p.agentCfgs().Coder,
		foundationPrompt,
		sb.String(),
		imageURLs,
		chatHistory,
		"Generating foundation (Group 0)",
	)
	if err != nil {
		return nil, fmt.Errorf("foundation: %w", err)
	}
	if len(project.Files) == 0 {
		return nil, fmt.Errorf("foundation: empty response")
	}
	return project, nil
}

func (p *ChatProcessor) generateUIKit(ctx context.Context, uiKitGroup models.ManifestGroup, foundationCtx string, agent config.AgentConfig) (*models.GeneratedProject, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "CHUNKED GENERATION — UI Kit (Group 1)\n\n")

	sb.WriteString("YOUR FILES TO IMPLEMENT (emit ONLY these):\n")
	for _, f := range uiKitGroup.Files {
		fmt.Fprintf(&sb, "  %s  (exports: [%s])\n", f.Path, strings.Join(f.Exports, ", "))
	}

	sb.WriteString("\n")
	sb.WriteString(foundationCtx)
	sb.WriteString("\n====================================\n")
	sb.WriteString("REMINDER: Emit ONLY the ui/* files listed above. No page logic, no API calls.\n")
	sb.WriteString("====================================\n")

	project, err := p.agent.GenerateCodeNoHistory(
		ctx,
		agent,
		chat_prompts2.PromptUIKitCoder,
		sb.String(),
		"Generating UI Kit (Group 1)",
	)
	if err != nil {
		return nil, fmt.Errorf("ui kit: %w", err)
	}
	if len(project.Files) == 0 {
		return nil, fmt.Errorf("ui kit: empty response")
	}
	return project, nil
}

func (p *ChatProcessor) generateChunkAdminPanel(ctx context.Context, group models.ManifestGroup, foundationCtx string, manifestSummary string, apiConfig string, uiKitAPISummary string, chunkPrompt string) (*models.GeneratedProject, error) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "CHUNKED GENERATION — Feature Group %d: %s\n\n", group.ID, group.Name)

	sb.WriteString("YOUR FILES TO IMPLEMENT (emit ONLY these):\n")
	for _, f := range group.Files {
		fmt.Fprintf(&sb, "  %s  (exports: [%s])\n", f.Path, strings.Join(f.Exports, ", "))
	}

	sb.WriteString("\n")
	sb.WriteString(apiConfig)
	sb.WriteString("\n")

	// Template API signatures (useApi/useAppForm/apiUtils/utils/AppProviders) are baked into
	// PromptChunkedCoderAdminPanel via TemplateAPIDigest — do not re-inject the full source here.

	sb.WriteString(foundationCtx)

	if uiKitAPISummary != "" {
		sb.WriteString("\n")
		sb.WriteString(uiKitAPISummary)
	}

	sb.WriteString("\n====================================\n")
	sb.WriteString("FULL PROJECT MANIFEST (import reference)\n")
	sb.WriteString("====================================\n")
	sb.WriteString(manifestSummary)

	project, err := p.agent.GenerateCodeNoHistory(
		ctx,
		p.agentCfgs().Coder,
		chunkPrompt,
		sb.String(),
		fmt.Sprintf("Generating feature chunk: %s", group.Name),
	)
	if err != nil {
		return nil, fmt.Errorf("chunk %s: %w", group.Name, err)
	}
	return project, nil
}

// buildFoundationContext formats generated foundation files for injection into feature chunk prompts.
func buildFoundationContext(files []models.ProjectFile) string {
	var sb strings.Builder
	sb.WriteString("====================================\n")
	sb.WriteString("FOUNDATION FILES (already generated — import freely, NEVER re-emit)\n")
	sb.WriteString("====================================\n")
	for _, f := range files {
		lang := "typescript"
		if strings.HasSuffix(f.Path, ".css") {
			lang = "css"
		} else if strings.HasSuffix(f.Path, ".json") {
			lang = "json"
		}
		fmt.Fprintf(&sb, "\n### %s\n```%s\n%s\n```\n", f.Path, lang, f.Content)
	}
	return sb.String()
}

// buildManifestSummary formats the manifest for prompt context, including
// Kind/Route metadata so feature chunks can tell pages from shared files.
func buildManifestSummary(manifest *models.ProjectManifest) string {
	var sb strings.Builder
	for _, group := range manifest.Groups {
		fmt.Fprintf(&sb, "Group %d (%s):\n", group.ID, group.Name)
		for _, file := range group.Files {
			var meta []string
			if file.Kind != "" {
				meta = append(meta, "kind="+file.Kind)
			}
			if file.Route != "" {
				meta = append(meta, "route="+file.Route)
			}
			metaStr := ""
			if len(meta) > 0 {
				metaStr = "  [" + strings.Join(meta, ", ") + "]"
			}
			fmt.Fprintf(&sb, "  %s → [%s]%s\n", file.Path, strings.Join(file.Exports, ", "), metaStr)
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// buildExpectedRoutesBlock renders manifest.Routes as a non-negotiable contract
// for Foundation so App.tsx is emitted with the exact (path, page, file) tuples.
//
// Routes MUST be wrapped in Layout so the app shell paints around every page.
// The wrap pattern differs by project type:
//
//   - admin_panel: Layout uses <Outlet /> (React Router v6 parent-route pattern)
//     because admin Layout owns navigation chrome and renders pages through Outlet.
//   - web: Layout takes `{ children }` (per website_prompts.go contract) and
//     wraps <Routes> directly. Outlet would conflict with the website Layout shape.
//
// Both variants always emit a wildcard "*" route as the safety net for typos
// and stale sidebar links. The post-merge rebuilder enforces the wrap anyway,
// but the prompt must agree — otherwise Claude streams a Layout-less App.tsx
// that visibly flashes wrong.
//
// Sidebar.tsx (admin) / Navbar.tsx (web) MUST use the exact paths from the map.
// Without this constraint Claude invents `/dashboard`, `/settings` that have no
// matching route → menu clicks land on a blank page.
func buildExpectedRoutesBlock(manifest *models.ProjectManifest, projectType string) string {
	if manifest == nil || len(manifest.Routes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("====================================\n")
	sb.WriteString("GLOBAL ROUTE MAP (non-negotiable — App.tsx + nav files MUST use exactly these paths)\n")
	sb.WriteString("====================================\n")
	for _, route := range manifest.Routes {
		fmt.Fprintf(&sb, "  path=%q  →  <%s />  (file: %s)\n", route.Path, route.PageName, route.FilePath)
	}
	sb.WriteString("\nApp.tsx — emit one lazy const per route above, then wrap ALL routes in Layout:\n")
	sb.WriteString("  import Layout from '@/components/layout/Layout';\n")
	sb.WriteString("  const PageName = lazy(() => import('@/pages/PageName').then(m => ({ default: m.PageName })));\n\n")

	if projectType == "web" {
		// Website Layout takes { children } per website_prompts.go contract.
		sb.WriteString("  <Layout>                                          ← children-prop wrapper\n")
		sb.WriteString("    <Routes>\n")
		sb.WriteString("      <Route path=\"/\" element={<HomePage />} />\n")
		sb.WriteString("      <Route path=\"/...\" element={<...Page />} />\n")
		sb.WriteString("      <Route path=\"*\" element={<Navigate to=\"/\" replace />} />  ← typo/404 catch\n")
		sb.WriteString("    </Routes>\n")
		sb.WriteString("  </Layout>\n")
		sb.WriteString("Layout.tsx — signature: `export default function Layout({ children }: { children: React.ReactNode })`.\n")
		sb.WriteString("            Render `{children}` between <Navbar /> and <Footer />.\n")
		sb.WriteString("Navbar.tsx — every nav link MUST use a path from the ROUTE MAP above. Do NOT invent /home, /pricing, /blog unless that exact path is listed.\n")
	} else {
		// Admin Layout uses <Outlet /> — React Router v6 idiomatic, lets the
		// shell (sidebar + header) stay mounted across navigation.
		sb.WriteString("  <Routes>\n")
		sb.WriteString("    <Route element={<Layout />}>                    ← MANDATORY shell wrapper\n")
		sb.WriteString("      <Route path=\"/\" element={<HomePage />} />\n")
		sb.WriteString("      <Route path=\"/...\" element={<...Page />} />\n")
		sb.WriteString("    </Route>\n")
		sb.WriteString("    <Route path=\"*\" element={<Navigate to=\"/\" replace />} />  ← typo/404 catch\n")
		sb.WriteString("  </Routes>\n")
		sb.WriteString("Layout.tsx — MUST `import { Outlet } from 'react-router-dom'` and render <Outlet /> where pages go.\n")
		sb.WriteString("Sidebar.tsx — every nav item MUST use a path from the ROUTE MAP above. Do NOT invent /dashboard, /settings, /profile unless that exact path is listed.\n")
	}
	return sb.String()
}

// buildExpectedEntityTypesBlock renders manifest.EntityTypes as a strict
// contract for src/types.ts: Foundation must export exactly these interfaces
// so feature chunks can import them by the names they expect.
func buildExpectedEntityTypesBlock(manifest *models.ProjectManifest) string {
	if manifest == nil || len(manifest.EntityTypes) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("====================================\n")
	sb.WriteString("EXPECTED ENTITY INTERFACES (non-negotiable — src/types.ts MUST export exactly these)\n")
	sb.WriteString("====================================\n")
	for _, entity := range manifest.EntityTypes {
		fmt.Fprintf(&sb, "export interface %s {\n", entity.Name)
		for _, field := range entity.Fields {
			opt := ""
			if field.Optional {
				opt = "?"
			}
			fmt.Fprintf(&sb, "  %s%s: %s;\n", field.Name, opt, field.TSType)
		}
		sb.WriteString("}\n\n")
	}
	sb.WriteString("Feature pages will import these EXACT names from '@/types'. Do not rename them.\n")
	return sb.String()
}

// buildStubChunk creates minimal placeholder files for a failed chunk so that
// App.tsx route imports resolve and the virtual FS build doesn't crash.
// Only page files (src/pages/*.tsx) get stubs — hook/util files are optional imports
// and don't block the build if absent.
func buildStubChunk(group models.ManifestGroup) *models.GeneratedProject {
	files := make([]models.ProjectFile, 0, len(group.Files))
	for _, f := range group.Files {
		if strings.HasSuffix(f.Path, ".tsx") && strings.Contains(f.Path, "/pages/") {
			// Extract component name from path: src/pages/LeavesPage.tsx → LeavesPage
			base := f.Path[strings.LastIndex(f.Path, "/")+1:]
			compName := strings.TrimSuffix(base, ".tsx")
			files = append(files, models.ProjectFile{
				Path: f.Path,
				Content: fmt.Sprintf(`import React from 'react'

export default function %s() {
  return (
    <div className="flex items-center justify-center h-64">
      <p className="text-muted-foreground">This section is temporarily unavailable.</p>
    </div>
  )
}
`, compName),
			})
		}
	}
	return &models.GeneratedProject{Files: files}
}

// filterToGroup keeps only files whose paths are explicitly listed in the manifest group.
// This prevents Claude from emitting files outside its assigned scope — leaked files
// (e.g. foundation generating feature pages) would otherwise overwrite correct implementations.
func filterToGroup(files []models.ProjectFile, group models.ManifestGroup) []models.ProjectFile {
	allowed := make(map[string]struct{}, len(group.Files))
	for _, f := range group.Files {
		allowed[f.Path] = struct{}{}
	}
	out := files[:0]
	for _, f := range files {
		if _, ok := allowed[f.Path]; ok {
			out = append(out, f)
		}
	}
	return out
}

// mergeChunks combines foundation + feature chunks. Foundation files always win on dedup.
func mergeChunks(foundation *models.GeneratedProject, chunks []*models.GeneratedProject) *models.GeneratedProject {
	seen := make(map[string]struct{}, len(foundation.Files)*3)
	allFiles := make([]models.ProjectFile, 0, len(foundation.Files)*3)
	env := make(map[string]any, len(foundation.Env))

	for _, f := range foundation.Files {
		seen[f.Path] = struct{}{}
		allFiles = append(allFiles, f)
	}
	maps.Copy(env, foundation.Env)

	for _, chunk := range chunks {
		if chunk == nil {
			continue
		}
		for k, v := range chunk.Env {
			if _, exists := env[k]; !exists {
				env[k] = v
			}
		}
		for _, f := range chunk.Files {
			if _, exists := seen[f.Path]; !exists {
				seen[f.Path] = struct{}{}
				allFiles = append(allFiles, f)
			}
		}
	}

	return &models.GeneratedProject{
		ProjectName: foundation.ProjectName,
		Files:       allFiles,
		FileGraph:   foundation.FileGraph,
		Env:         env,
	}
}

// buildFoundationStubWithManifest is the textual stub passed to UIKit/Feature
// prompts. When manifest is non-nil it also injects the page-export and
// entity-type contracts so feature chunks never have to guess names.
func buildFoundationStubWithManifest(foundationGroup models.ManifestGroup, manifest *models.ProjectManifest) string {
	var sb strings.Builder

	sb.WriteString("====================================\n")
	sb.WriteString("FOUNDATION (Group 0) — ALREADY GENERATED (import freely, NEVER re-emit)\n")
	sb.WriteString("====================================\n")
	sb.WriteString("These files exist in the project. Import from them by path. Never re-implement or re-emit.\n\n")
	for _, file := range foundationGroup.Files {
		if len(file.Exports) > 0 {
			fmt.Fprintf(&sb, "  %-52s → exports: [%s]\n", file.Path, strings.Join(file.Exports, ", "))
		} else {
			fmt.Fprintf(&sb, "  %s\n", file.Path)
		}
	}

	if manifest != nil && len(manifest.Routes) > 0 {
		sb.WriteString("\n====================================\n")
		sb.WriteString("EXPECTED EXPORTS PER PAGE (use these EXACT names in your imports + components)\n")
		sb.WriteString("====================================\n")
		for _, route := range manifest.Routes {
			fmt.Fprintf(&sb, "  %s  →  export function %s()  (route: %s)\n", route.FilePath, route.PageName, route.Path)
		}
		sb.WriteString("App.tsx already loads these via lazy(...).then(m => ({ default: m.PageName })). Your page MUST use the matching named export.\n")
	}

	if manifest != nil && len(manifest.EntityTypes) > 0 {
		sb.WriteString("\n====================================\n")
		sb.WriteString("ENTITY TYPE NAMES IN src/types.ts (import from '@/types' — exact names)\n")
		sb.WriteString("====================================\n")
		for _, entity := range manifest.EntityTypes {
			fields := make([]string, 0, len(entity.Fields))
			for _, field := range entity.Fields {
				opt := ""
				if field.Optional {
					opt = "?"
				}
				fields = append(fields, fmt.Sprintf("%s%s: %s", field.Name, opt, field.TSType))
			}
			fmt.Fprintf(&sb, "  %s { %s }\n", entity.Name, strings.Join(fields, "; "))
		}
	}
	sb.WriteString("\nCSS VARIABLES (defined in src/index.css — use in Tailwind arbitrary values):\n")
	sb.WriteString("  --background  --foreground  --card  --card-foreground\n")
	sb.WriteString("  --primary     --primary-foreground\n")
	sb.WriteString("  --secondary   --secondary-foreground\n")
	sb.WriteString("  --muted       --muted-foreground\n")
	sb.WriteString("  --accent      --accent-foreground\n")
	sb.WriteString("  --destructive --border  --input  --ring  --radius\n")
	sb.WriteString("\nEntity interfaces are exported from src/types.ts — import what you need.\n")
	sb.WriteString("Pre-built utilities: cn() from @/lib/utils, useApiQuery/useApiMutation from @/hooks/useApi.\n")
	return sb.String()
}

func buildTemplateHooksContext() string {
	critical := map[string]bool{
		"src/hooks/useApi.ts":                    true,
		"src/hooks/useAppForm.ts":                true,
		"src/lib/apiUtils.ts":                    true,
		"src/lib/utils.ts":                       true,
		"src/components/shared/AppProviders.tsx": true,
	}
	var sb strings.Builder
	sb.WriteString("\n====================================\n")
	sb.WriteString("PRE-BUILT FILES — DO NOT RE-IMPLEMENT, IMPORT FROM THESE\n")
	sb.WriteString("====================================\n")
	sb.WriteString("These files ALREADY EXIST in the project. Use EXACTLY these import paths.\n")
	sb.WriteString("NEVER create src/lib/api.ts — use @/config/axios instead.\n\n")
	sb.WriteString("REQUIRED IMPORTS:\n")
	sb.WriteString("  import { useApiQuery, useApiMutation } from '@/hooks/useApi'\n")
	sb.WriteString("  import { extractList, extractSingle, extractCount } from '@/lib/apiUtils'\n")
	sb.WriteString("  import { cn, formatDate, formatCurrency, getInitials, truncate } from '@/lib/utils'\n")
	sb.WriteString("  import { AppProviders } from '@/components/shared/AppProviders'\n\n")
	sb.WriteString("FILE CONTENTS FOR REFERENCE:\n\n")
	for _, f := range GetTemplateContext() {
		if critical[f.Path] {
			fmt.Fprintf(&sb, "### %s\n```typescript\n%s\n```\n\n", f.Path, f.Content)
		}
	}
	return sb.String()
}

func injectEnvFile(files []models.ProjectFile, baseURL string, projectData *models.ProjectData, projectType string) []models.ProjectFile {
	var (
		envBaseURLKey = "VITE_API_BASE_URL"
		envAPIKeyKey  = "VITE_X_API_KEY"
	)

	apiKey := ""
	if projectData != nil {
		apiKey = projectData.ApiKey
	}

	for _, f := range GetTemplateContext() {
		if strings.Contains(f.Path, "config/axios") || strings.Contains(f.Path, "config/env") {
			if strings.Contains(f.Content, "VITE_BASE_URL") && !strings.Contains(f.Content, "VITE_API_BASE_URL") {
				envBaseURLKey = "VITE_BASE_URL"
			}
			if strings.Contains(f.Content, "VITE_API_KEY") && !strings.Contains(f.Content, "VITE_X_API_KEY") {
				envAPIKeyKey = "VITE_API_KEY"
			}
		}
	}

	content := fmt.Sprintf("%s=%s\n%s=%s\n", envBaseURLKey, baseURL, envAPIKeyKey, apiKey)

	envTargets := map[string]bool{".env": true, ".env.development": true, ".env.production": true}
	for i, f := range files {
		if envTargets[f.Path] {
			files[i].Content = content
			delete(envTargets, f.Path)
		}
	}
	for path := range envTargets {
		files = append(files, models.ProjectFile{Path: path, Content: content})
	}
	return files
}

var forbiddenConfigFiles = map[string]bool{
	"tsconfig.json":      true,
	"tsconfig.node.json": true,
	"vite.config.ts":     true,
	"vite.config.js":     true,
	"package.json":       true,
	"package-lock.json":  true,
	"tailwind.config.js": true,
	"tailwind.config.ts": true,
	"postcss.config.js":  true,
	"postcss.config.cjs": true,
}

var adminPanelRuntimeTemplateFiles = []string{
	"src/config/axios.ts",
	"src/lib/auth.ts",
	"src/lib/permissions.ts",
	"src/components/auth/LoginPage.tsx",
	"src/components/auth/ProtectedRoute.tsx",
}

func ensureTemplateFiles(files []models.ProjectFile, requiredPaths []string, replaceExisting bool) []models.ProjectFile {
	templates := make(map[string]models.ProjectFile, len(requiredPaths))
	for _, tf := range GetTemplateScaffold() {
		for _, path := range requiredPaths {
			if tf.Path == path {
				templates[path] = tf
				break
			}
		}
	}

	existing := make(map[string]int, len(files))
	for i, f := range files {
		existing[f.Path] = i
	}

	for _, path := range requiredPaths {
		tf, ok := templates[path]
		if !ok {
			log.Printf("[inject] template file %s not found in scaffold", path)
			continue
		}
		if idx, exists := existing[path]; exists {
			if replaceExisting && files[idx].Content != tf.Content {
				log.Printf("[inject] replacing %s with template runtime file", path)
				files[idx].Content = tf.Content
			}
			continue
		}
		log.Printf("[inject] %s missing — injecting from template", path)
		files = append(files, tf)
		existing[path] = len(files) - 1
	}

	return files
}

// stripForbiddenConfigFiles removes AI-generated config files that are pre-built in the template.
// Must be called before scaffold injection so the valid template versions get injected instead.
func stripForbiddenConfigFiles(files []models.ProjectFile) []models.ProjectFile {
	out := make([]models.ProjectFile, 0, len(files))
	for _, f := range files {
		if forbiddenConfigFiles[f.Path] {
			log.Printf("[strip] removed AI-generated config file: %s", f.Path)
		} else {
			out = append(out, f)
		}
	}
	return out
}

func injectMissingCriticalFiles(files []models.ProjectFile, projectType string) []models.ProjectFile {
	if strings.EqualFold(projectType, "admin_panel") {
		files = ensureTemplateFiles(files, adminPanelRuntimeTemplateFiles, true)
	}

	existing := make(map[string]struct{}, len(files))
	for _, f := range files {
		existing[f.Path] = struct{}{}
	}

	// src/lib/utils.ts — only needed for landing/web (pre-built in admin_panel template scaffold)
	if projectType == "landing" || projectType == "web" {
		if _, ok := existing["src/lib/utils.ts"]; !ok {
			log.Printf("[inject] src/lib/utils.ts missing — injecting from template")
			for _, tf := range GetTemplateScaffold() {
				if tf.Path == "src/lib/utils.ts" {
					files = append(files, tf)
					break
				}
			}
		}
	}

	// src/App.tsx — mandatory for ALL project types; missing = blank screen / virtual FS crash
	if _, ok := existing["src/App.tsx"]; !ok {
		log.Printf("[inject] src/App.tsx missing — injecting minimal stub (type=%s)", projectType)
		switch projectType {
		case "admin_panel", "webapp", mobileProjectType:
			files = append(files, models.ProjectFile{
				Path: "src/App.tsx",
				Content: `import React from 'react'
import './index.css'
import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { AppProviders } from '@/components/shared/AppProviders'

export default function App() {
  return (
    <AppProviders>
      <BrowserRouter>
        <Routes>
          <Route path="*" element={<div className="flex items-center justify-center min-h-screen"><p className="text-muted-foreground">Loading...</p></div>} />
        </Routes>
      </BrowserRouter>
    </AppProviders>
  )
}
`,
			})
		case "web":
			files = append(files, models.ProjectFile{
				Path: "src/App.tsx",
				Content: `import React from 'react';
import './index.css';
import { BrowserRouter, Routes, Route } from 'react-router-dom';

export default function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="*" element={<div className="flex items-center justify-center min-h-screen"><p className="text-muted-foreground">Loading...</p></div>} />
      </Routes>
    </BrowserRouter>
  );
}
`,
			})
		default:
			files = append(files, models.ProjectFile{
				Path: "src/App.tsx",
				Content: `import React from 'react';
import './index.css';

export default function App() {
  return <div className="min-h-screen bg-background text-foreground" />;
}
`,
			})
		}
	}

	return files
}

func (p *ChatProcessor) validateAndRepairGeneratedProject(ctx context.Context, project *models.GeneratedProject, projectType string, capabilities []models.MobileCapability, startPercent int) int {
	const maxRepairPasses = 3

	if project == nil {
		return 0
	}

	// Dedup .ts/.tsx shadows BEFORE validation — without this the repair loop
	// chases a moving target and stalls on "no progress" while the broken file
	// is dead code Vite never imports.
	project.Files = dedupTsTsxPairs(project.Files)
	project.Files = ensureAppEntryDefaultExport(project.Files)

	p.emitter().Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    "shield-check",
		Message: "Проверяю build-safety, маршруты и страницы...",
		Percent: startPercent,
	})

	var (
		errorCount     int
		prevErrorCount = -1
		prevSigs       map[string]bool
	)
	// sigOf reduces a validation result to the SET of distinct error identities
	// (file::message). Comparing identities instead of raw counts lets the no-progress
	// guard tell a genuine stall apart from a repair that SWAPS one error for a
	// newly-introduced different one (e.g. Sidebar missing-perms → Header dangling
	// SidebarContent import) — the latter changes the set and earns another pass.
	sigOf := func(errs []ValidationError) map[string]bool {
		m := make(map[string]bool, len(errs))
		for _, e := range errs {
			if e.Severity == "error" {
				m[e.File+"::"+e.Message] = true
			}
		}
		return m
	}
	for pass := 1; pass <= maxRepairPasses; pass++ {
		validationErrors := validateGeneratedProject(project.Files, project.Env)
		validationErrors = append(validationErrors, validateAgainstManifest(project.Files, p.currentManifest)...)
		if projectType == "admin_panel" {
			validationErrors = append(validationErrors, validateAdminPanelUIQuality(project.Files)...)
		}
		if usesWebAppGenerator(projectType) {
			validationErrors = append(validationErrors, validateWebAppUIQuality(project.Files)...)
		}
		if projectType == mobileProjectType {
			validationErrors = append(validationErrors, validateMobileGeneratedProject(project.Files)...)
		}
		errorCount, _ = logValidationResults(validationErrors)
		if errorCount == 0 {
			if pass > 1 {
				p.emitter().Emit(SSEEvent{Type: EvProgress, Icon: "check-circle", Message: "Ошибки генерации исправлены", Percent: startPercent + 6})
			}
			return 0
		}
		// No-progress early exit: stop ONLY when this pass introduced NO new error
		// identity AND the count didn't drop. Comparing signatures (file::message)
		// rather than raw counts means a repair that swaps one error for a newly-
		// introduced different one (e.g. fixing Sidebar's missing perms but dropping
		// the SidebarContent export Header depends on) is NOT mistaken for a stall —
		// the new identity earns another pass that can repair it.
		curSigs := sigOf(validationErrors)
		introducedNew := false
		for s := range curSigs {
			if !prevSigs[s] {
				introducedNew = true
				break
			}
		}
		if prevSigs != nil && !introducedNew && errorCount >= prevErrorCount {
			log.Printf("[quality-gate] no progress (%d → %d, no new error identities), stopping after pass %d", prevErrorCount, errorCount, pass-1)
			return errorCount
		}
		prevErrorCount = errorCount
		prevSigs = curSigs

		p.emitter().Emit(SSEEvent{
			Type:    EvRepair,
			Icon:    "wrench",
			Percent: startPercent + pass,
			Message: fmt.Sprintf("Автофикс build/page ошибок: проход %d/%d", pass, maxRepairPasses),
			Value:   fmt.Sprintf("%d ошибок", errorCount),
		})
		log.Printf("[quality-gate] 🔧 repair pass %d/%d for %s: %d errors", pass, maxRepairPasses, projectType, errorCount)

		repaired := p.repairBrokenFiles(ctx, project.Files, validationErrors)
		if len(repaired) == 0 {
			log.Printf("[quality-gate] no repairs returned on pass %d", pass)
			return errorCount
		}
		applyRepairs(project.Files, repaired)
		// Re-dedup: repair can re-introduce a .ts shadow by mistake when
		// trying to "fix" the wrong file. Cleaning each pass keeps subsequent
		// validation honest.
		project.Files = dedupTsTsxPairs(project.Files)
		project.Files = ensureAppEntryDefaultExport(project.Files)
		if projectType == mobileProjectType {
			normalized, normalizeErr := applyCapacitorScaffold(project.Files, project.ProjectName, p.mcpProjectId, capabilities)
			if normalizeErr != nil {
				log.Printf("[quality-gate] restore Capacitor scaffold: %v", normalizeErr)
			} else {
				project.Files = normalized
			}
		}
	}

	finalErrors := validateGeneratedProject(project.Files, project.Env)
	finalErrors = append(finalErrors, validateAgainstManifest(project.Files, p.currentManifest)...)
	if projectType == "admin_panel" {
		finalErrors = append(finalErrors, validateAdminPanelUIQuality(project.Files)...)
	}
	if usesWebAppGenerator(projectType) {
		finalErrors = append(finalErrors, validateWebAppUIQuality(project.Files)...)
	}
	if projectType == mobileProjectType {
		finalErrors = append(finalErrors, validateMobileGeneratedProject(project.Files)...)
	}
	errorCount, _ = logValidationResults(finalErrors)
	return errorCount
}

func (p *ChatProcessor) generateCodeChunkedWebsite(ctx context.Context, clarified string, imageURLs []string, chatHistory []models.ChatMessage, plan *models.ArchitectPlan, projectData *models.ProjectData) (*models.ParsedClaudeResponse, error) {
	log.Printf("[chunked-web] starting chunked website generation: %s", plan.ProjectName)
	var (
		emit     = p.emitter()
		manifest *models.ProjectManifest
	)

	if p.prebuiltManifest != nil {
		manifest = p.prebuiltManifest
		p.prebuiltManifest = nil
		p.currentManifest = manifest
		p.navRoutes = manifest.Routes
		defer func() { p.currentManifest = nil }()
		log.Printf("[chunked-web] using prebuilt manifest: %d groups", len(manifest.Groups))
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "list-tree", Message: "Структура страниц готова", Percent: 23})
	} else {
		return nil, fmt.Errorf("chunked-web: no manifest available")
	}

	var (
		foundationGroup, uiKitGroup models.ManifestGroup
		pageGroups                  []models.ManifestGroup

		totalFiles = 0
	)
	for _, g := range manifest.Groups {
		totalFiles += len(g.Files)

		switch g.ID {
		case 0:
			foundationGroup = g
		case 1:
			uiKitGroup = g
		default:
			pageGroups = append(pageGroups, g)
		}
	}

	if len(foundationGroup.Files) == 0 || len(pageGroups) == 0 {
		log.Printf("[chunked-web] manifest missing foundation or pages — falling back to single call")
		return p.generateCodeSingle(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	}

	var pageNames = make([]string, 0, len(pageGroups))

	for _, g := range pageGroups {
		pageNames = append(pageNames, g.Name)
	}

	emit.Emit(
		SSEEvent{
			Type:    EvManifest,
			Icon:    "git-branch",
			Percent: 23,
			Message: "Структура сайта спланирована",
			Value:   fmt.Sprintf("%d файлов · %d страниц", totalFiles, len(pageGroups)),
			Data: ManifestEventData{
				TotalFiles:   totalFiles,
				GroupCount:   len(manifest.Groups),
				FeatureNames: pageNames,
			},
		},
	)

	apiConfig := buildAPIConfigBlock(p.baseConf.UcodeBaseUrl, projectData, plan)
	if !plan.CloneMode && p.cachedImagePool != nil && p.cachedImagePool.Err == nil {
		apiConfig += "\n\n" + p.cachedImagePool.Block
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: fmt.Sprintf("Подобрано %d фото: %s", p.cachedImagePool.Count, strings.Join(p.cachedImagePool.Keywords, ", ")), Percent: 25})
		p.cachedImagePool = nil
	} else if !plan.CloneMode && p.baseConf.UnsplashAccessKey != "" {
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: "Подбираю изображения...", Percent: 24})
		pool := helper.FetchImagePool(ctx, p.baseConf.UnsplashAccessKey, plan)
		if pool.Err == nil {
			apiConfig += "\n\n" + pool.Block
			emit.Emit(SSEEvent{Type: EvProgress, Icon: "image", Message: fmt.Sprintf("Подобрано %d фото: %s", pool.Count, strings.Join(pool.Keywords, ", ")), Percent: 25})
		}
	}

	time.Sleep(500 * time.Millisecond)
	emit.Emit(SSEEvent{Type: EvProgress, Icon: "layers", Message: "Генерирую Layout, Navbar, Footer и UI Kit параллельно...", Percent: 25})

	var (
		foundation *models.GeneratedProject
		uiKit      *models.GeneratedProject
		foundErr   error

		wg sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for attempt := 1; attempt <= 2; attempt++ {
			foundation, foundErr = p.generateWebsiteFoundation(ctx, clarified, imageURLs, chatHistory, apiConfig, foundationGroup, manifest)
			if foundErr == nil {
				break
			}
			if attempt < 2 && !errors.Is(foundErr, ai.ErrMaxTokens) {
				log.Printf("[chunked-web] foundation attempt %d failed (%v) — retrying", attempt, foundErr)
			}
		}
		if foundErr != nil {
			return
		}
		foundation.Files = filterToGroup(foundation.Files, foundationGroup)
		emit.Emit(SSEEvent{Type: EvProgress, Icon: "check-circle", Message: "Foundation готов", Value: fmt.Sprintf("%d файлов", len(foundation.Files)), Percent: 38})
	}()

	if len(uiKitGroup.Files) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			stub := buildFoundationStubWithManifest(foundationGroup, manifest)
			var e error
			uiKit, e = p.generateUIKit(ctx, uiKitGroup, stub, p.agentCfgs().LandingCoder)
			if e != nil {
				log.Printf("[chunked-web] UI kit failed (%v) — continuing without it", e)
				return
			}
			uiKit.Files = filterToGroup(uiKit.Files, uiKitGroup)
			emit.Emit(SSEEvent{Type: EvProgress, Icon: "check-circle", Message: "UI Kit готов", Value: fmt.Sprintf("%d компонентов", len(uiKit.Files)), Percent: 48})
		}()
	}

	err := withHeartbeat(ctx, emit,
		p.agentCfgs().Coder.Model,
		[]string{
			"Генерирую Layout и навигацию...",
			"Создаю Navbar и Footer...",
			"Настраиваю роутинг в App.tsx...",
			"Создаю UI компоненты...",
			"Формирую глобальные стили и CSS переменные...",
		},
		func() error { wg.Wait(); return foundErr },
	)
	if err != nil {
		log.Printf("[chunked-web] foundation failed (%v) — falling back to single call", err)
		return p.generateCodeSingle(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	}

	allSharedFiles := make([]models.ProjectFile, 0, len(foundation.Files))
	allSharedFiles = append(allSharedFiles, foundation.Files...)

	if uiKit != nil {
		allSharedFiles = append(allSharedFiles, uiKit.Files...)
	}

	foundationCtx := buildFoundationContext(allSharedFiles)
	manifestSummary := buildManifestSummary(manifest)

	totalPages := len(pageGroups)
	time.Sleep(500 * time.Millisecond)

	emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    "zap",
		Percent: 51,
		Message: "Генерирую все страницы параллельно",
		Value:   fmt.Sprintf("%d страниц одновременно", totalPages),
	})

	pageResults := make(chan chunkResult, totalPages)

	stopHB := make(chan struct{})
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		tick := 0
		for {
			select {
			case <-ticker.C:
				tick++
				pct := 51 + tick*2
				if pct > 76 {
					pct = 76
				}
				emit.Emit(SSEEvent{Type: EvProgress, Icon: "cpu", Message: "Генерирую страницы параллельно...", Percent: pct})
			case <-stopHB:
				return
			}
		}
	}()
	defer close(stopHB)

	for i, group := range pageGroups {
		startDelay := time.Duration(i) * 100 * time.Millisecond
		go func() {
			if startDelay > 0 {
				time.Sleep(startDelay)
			}
			emit.Emit(SSEEvent{Type: EvChunkStart, Icon: "file-text", Message: "Генерирую страницу", Value: group.Name, Data: map[string]any{"feature": group.Name}})
			proj, chunkErr := p.generateWebsitePage(ctx, group, foundationCtx, manifestSummary, apiConfig, imageURLs)
			pageResults <- chunkResult{group: group, project: proj, err: chunkErr}
		}()
	}

	var (
		successChunks []*models.GeneratedProject
		failedCount   int
	)

	for i := range pageGroups {
		res := <-pageResults
		completed := i + 1
		pct := 51 + completed*24/totalPages
		if res.err != nil {
			log.Printf("[chunked-web] page %q error: %v", res.group.Name, res.err)
			failedCount++
			if stub := buildStubChunk(res.group); len(stub.Files) > 0 {
				successChunks = append(successChunks, stub)
			}
		} else {
			res.project.Files = filterToGroup(res.project.Files, res.group)
			log.Printf("[chunked-web] ✅ page %q: %d files", res.group.Name, len(res.project.Files))
			emit.Emit(SSEEvent{
				Type:    EvChunkDone,
				Icon:    "check-circle",
				Percent: pct,
				Message: fmt.Sprintf("Страница готова (%d/%d)", completed, totalPages),
				Value:   res.group.Name,
				Data:    ChunkDoneData{Feature: res.group.Name, Index: completed, Total: totalPages, Files: res.project.Files},
			})
			successChunks = append(successChunks, res.project)
		}
	}

	if failedCount == totalPages {
		return nil, fmt.Errorf("chunked-web: all %d page chunks failed", totalPages)
	}

	var allChunks []*models.GeneratedProject

	if uiKit != nil {
		allChunks = append(allChunks, uiKit)
	}
	allChunks = append(allChunks, successChunks...)
	merged := mergeChunks(foundation, allChunks)

	merged.Files = stripForbiddenConfigFiles(merged.Files)

	merged.Files = mergeTemplateScaffold(merged.Files, GetTemplateScaffold())

	merged.Files = injectMissingCriticalFiles(merged.Files, plan.ProjectType)
	merged.Files = injectEnvFile(merged.Files, p.baseConf.UcodeBaseUrl, projectData, plan.ProjectType)
	merged.Files = mergeAppRoutes(merged.Files, manifest)

	emit.Emit(SSEEvent{Type: EvProgress, Icon: "shield-check", Percent: 80, Message: "Проверяю качество кода", Value: fmt.Sprintf("%d файлов", len(merged.Files))})
	errorCount := p.validateAndRepairGeneratedProject(ctx, merged, plan.ProjectType, nil, 80)

	log.Printf("[chunked-web] done: %d total files (%d pages, %d failed, %d validation errors)", len(merged.Files), totalPages, failedCount, errorCount)
	return &models.ParsedClaudeResponse{Project: merged}, nil
}

func (p *ChatProcessor) generateWebsiteFoundation(ctx context.Context, clarified string, imageURLs []string, chatHistory []models.ChatMessage, apiConfig string, foundationGroup models.ManifestGroup, manifest *models.ProjectManifest) (*models.GeneratedProject, error) {
	var sb strings.Builder

	sb.WriteString("====================================\n")
	sb.WriteString("⚠ CHUNKED GENERATION — FOUNDATION PHASE ONLY ⚠\n")
	sb.WriteString("====================================\n")
	sb.WriteString("You are generating ONLY the foundation (Group 0) files listed below.\n")
	sb.WriteString("Individual page implementations will be generated separately in parallel.\n")
	sb.WriteString("EMIT ONLY the files in 'YOUR FILES TO GENERATE'. No exceptions.\n\n")

	sb.WriteString("YOUR FILES TO GENERATE (Group 0 — Foundation):\n")
	for _, f := range foundationGroup.Files {
		fmt.Fprintf(&sb, "  %s  (exports: [%s])\n", f.Path, strings.Join(f.Exports, ", "))
	}

	sb.WriteString("\nPAGES — add lazy imports + routes to App.tsx but DO NOT implement their content:\n")
	sb.WriteString("(One `const PageName = lazy(...)` declaration above the App component and one <Route> inside <Routes> per page below. Full pattern is documented in the system prompt — do not re-explain here.)\n")
	for _, g := range manifest.Groups {
		if g.ID == 0 {
			continue
		}
		for _, f := range g.Files {
			if strings.Contains(f.Path, "/pages/") || strings.HasSuffix(f.Path, "Page.tsx") {
				fmt.Fprintf(&sb, "  %s  → component: [%s]\n", f.Path, strings.Join(f.Exports, ", "))
			}
		}
	}

	// Note: ExportConventionBlock + LazyAppTsxExemplar are baked into PromptWebsiteGenerator
	// (cacheable system prompt) — do NOT re-inject here.
	if routesBlock := buildExpectedRoutesBlock(manifest, "web"); routesBlock != "" {
		sb.WriteString("\n")
		sb.WriteString(routesBlock)
	}
	if typesBlock := buildExpectedEntityTypesBlock(manifest); typesBlock != "" {
		sb.WriteString("\n")
		sb.WriteString(typesBlock)
	}

	sb.WriteString("\nFULL MANIFEST (for routing reference):\n")
	sb.WriteString(buildManifestSummary(manifest))

	sb.WriteString("\n====================================\n")
	sb.WriteString("PROJECT REQUEST\n")
	sb.WriteString("====================================\n")
	sb.WriteString(clarified)
	sb.WriteString("\n\n")
	sb.WriteString(apiConfig)

	sb.WriteString(buildTemplateHooksContext())

	sb.WriteString("\n====================================\n")
	sb.WriteString("REMINDER: Emit ONLY the Group 0 files listed above. DO NOT generate page content.\n")
	sb.WriteString("====================================\n")

	project, err := p.agent.GenerateCode(ctx,
		p.agentCfgs().LandingCoder,
		chat_prompts2.PromptWebsiteGenerator,
		sb.String(),
		imageURLs,
		chatHistory,
		"Generating website foundation (Group 0)",
	)
	if err != nil {
		return nil, fmt.Errorf("website foundation: %w", err)
	}
	if len(project.Files) == 0 {
		return nil, fmt.Errorf("website foundation: empty response")
	}
	return project, nil
}

func (p *ChatProcessor) generateWebsitePage(ctx context.Context, group models.ManifestGroup, foundationCtx string, manifestSummary string, apiConfig string, imageURLs []string) (*models.GeneratedProject, error) {
	var sb strings.Builder
	fmt.Fprintf(&sb, "CHUNKED GENERATION — Website Page: %s\n\n", group.Name)

	sb.WriteString("YOUR FILE TO IMPLEMENT (emit ONLY this file):\n")
	for _, f := range group.Files {
		fmt.Fprintf(&sb, "  %s  (exports: [%s])\n", f.Path, strings.Join(f.Exports, ", "))
	}

	sb.WriteString("\n")
	sb.WriteString(apiConfig)
	sb.WriteString("\n")

	// Template API signatures (useApi/useAppForm/apiUtils/utils/AppProviders) are baked into
	// PromptWebsitePageCoder via TemplateAPIDigest — do not re-inject the full source here.

	sb.WriteString(foundationCtx)

	sb.WriteString("\n====================================\n")
	sb.WriteString("PROJECT MANIFEST (for import reference)\n")
	sb.WriteString("====================================\n")
	sb.WriteString(manifestSummary)

	// Reference-clone screenshots (and user-attached design images) must reach
	// every page chunk — the pages carry the visual fidelity, not the foundation.
	if len(imageURLs) > 0 {
		project, err := p.agent.GenerateCode(
			ctx,
			p.agentCfgs().LandingCoder,
			chat_prompts2.PromptWebsitePageCoder,
			sb.String(),
			imageURLs,
			nil,
			fmt.Sprintf("Generating website page: %s", group.Name),
		)
		if err != nil {
			return nil, fmt.Errorf("website page %s: %w", group.Name, err)
		}
		return project, nil
	}

	project, err := p.agent.GenerateCodeNoHistory(
		ctx,
		p.agentCfgs().LandingCoder,
		chat_prompts2.PromptWebsitePageCoder,
		sb.String(),
		fmt.Sprintf("Generating website page: %s", group.Name),
	)
	if err != nil {
		return nil, fmt.Errorf("website page %s: %w", group.Name, err)
	}
	return project, nil
}

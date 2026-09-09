package v1

import (
	"context"
	"crypto/rand"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/anthropic"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/gemini"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/openai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	as "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
	"unicode"

	"golang.org/x/text/runes"
	"golang.org/x/text/transform"
	"golang.org/x/text/unicode/norm"
)

const (
	uGenBranch = "u-gen"

	timeoutPublishMicrofrontend = 15 * time.Minute
)

type ChatProcessor struct {
	h               *HandlerV1
	service         services.ServiceManagerI
	companyServices services.CompanyServiceI
	baseConf        config.BaseConfig

	chatId            string
	chatModel         config.AIProvider
	mcpProjectId      string
	resourceEnvId     string
	ucodeProjectId    string
	billingProjectId  string
	mcpUcodeProjectId string
	mcpEnvironmentId  string
	companyId         string
	fareId            string

	userId       string
	clientTypeId string
	roleId       string
	authToken    string

	microFrontendId            string
	microFrontendRepoId        string
	microFrontendResourceEnvId string
	newProject                 bool
	userMessage                string

	schemaCache    []models.TableSchema
	schemaCachedAt time.Time

	prebuiltManifest *models.ProjectManifest

	// currentManifest outlives prebuiltManifest consumption so validate/repair
	// can still reference the route + entity contract.
	currentManifest *models.ProjectManifest

	// navRoutes captures the manifest's page routes for persistence into the MCP
	// project's project_env (nav_routes). currentManifest is nil'd via defer once
	// generation returns, so this survives until publishToMicrofrontend runs.
	navRoutes []models.ManifestRoute

	cachedImagePool *helper.ImagePoolResult

	// editPromptOverrides is resolved from the generated child project for this
	// request. It must never be copied into or mutate the package-level defaults.
	editPromptOverrides models.EditPromptOverrides
	editPromptsLoaded   bool

	tokenBudgetEnabled bool
	tokenBudgetRemain  int64 // remaining fare (day/month) budget; drained first
	tokenPackRemain    int64 // remaining company token-pack pool; drained after fare
	tokenBudgetSnap    models.TokenBudgetSnapshot

	ucodeMcpProjectId string

	agent ai.Agent
	emit  ProgressEmitter
}

func (p *ChatProcessor) agentCfgs() config.AIAgents {
	switch p.chatModel {
	case config.AIProviderGemini:
		return p.baseConf.GeminiAgents

	case config.AIProviderOpenAI:
		return p.baseConf.OpenAIAgents

	case config.AIProviderAuto:
		return p.autoAgentCfgs()

	default:
		return p.baseConf.Agents
	}
}

func (p *ChatProcessor) autoAgentCfgs() config.AIAgents {
	claude := p.baseConf.Agents
	openai := p.baseConf.OpenAIAgents
	return config.AIAgents{
		Router:            claude.Router,
		Architect:         claude.Architect,
		Coder:             claude.Coder,
		LandingCoder:      claude.LandingCoder,
		Inspector:         claude.Inspector,
		Planner:           openai.Planner,
		DatabaseAssistant: openai.DatabaseAssistant,
	}
}

func (p *ChatProcessor) providerEventData() ProviderEventData {
	cfgs := p.agentCfgs()
	return ProviderEventData{
		Provider:    string(p.chatModel),
		RouterModel: cfgs.Router.Model,
		CoderModel:  cfgs.Coder.Model,
	}
}

func (p *ChatProcessor) editModel() string {
	if p.chatModel == config.AIProviderAuto {
		return p.baseConf.OpenAIAgents.Coder.Model
	}
	return p.agentCfgs().Coder.Model
}

func (p *ChatProcessor) providerLabel() string {
	switch {
	case p.newProject:
		return p.agentCfgs().Architect.Model

	case p.microFrontendId != "":
		return p.editModel()

	default:
		return p.agentCfgs().Router.Model
	}
}

func (p *ChatProcessor) emitter() ProgressEmitter {
	if p.emit == nil {
		return noopEmitter{}
	}

	return p.emit
}

func (p *ChatProcessor) ensureEditPromptOverrides(ctx context.Context) {
	if p.editPromptsLoaded {
		return
	}
	p.editPromptsLoaded = true
	p.editPromptOverrides = p.h.loadAIEditPromptOverrides(ctx, p.mcpUcodeProjectId, p.mcpEnvironmentId)
}

func newChatProcessor(h *HandlerV1, service services.ServiceManagerI, baseConf config.BaseConfig, chatId, mcpProjectId, resourceEnvId, ucodeProjectId string, userId, clientTypeId, roleId, authToken string) *ChatProcessor {
	p := &ChatProcessor{
		h:                h,
		service:          service,
		companyServices:  h.companyServices,
		baseConf:         baseConf,
		chatId:           chatId,
		mcpProjectId:     mcpProjectId,
		resourceEnvId:    resourceEnvId,
		ucodeProjectId:   ucodeProjectId,
		billingProjectId: ucodeProjectId,
		userId:           userId,
		clientTypeId:     clientTypeId,
		roleId:           roleId,
		authToken:        authToken,
	}
	return p
}

func (p *ChatProcessor) initAgent() {
	switch p.chatModel {
	case config.AIProviderGemini:
		p.agent = gemini.NewGeminiAgent(p.baseConf, p.h.geminiKeyPool, p)

	case config.AIProviderOpenAI:
		p.agent = openai.NewOpenAIAgent(p.baseConf, p)

	case config.AIProviderAuto:
		p.agent = ai.NewAutoAgent(
			anthropic.NewAnthropicAgent(p.baseConf, p),
			openai.NewOpenAIAgent(p.baseConf, p),
		)

	default:
		p.agent = anthropic.NewAnthropicAgent(p.baseConf, p)
	}

	cfgs := p.agentCfgs()
	log.Printf("[ai] chat=%s provider=%s router=%s coder=%s", p.chatId, p.chatModel, cfgs.Router.Model, cfgs.Coder.Model)
}

// ============================================================================
// NEW PROJECT BUILD
// ============================================================================

func (p *ChatProcessor) buildNewProject(ctx context.Context, clarified string, chatHistory []models.ChatMessage, imageURLs []string, estimatedName string) (*models.ParsedClaudeResponse, error) {
	startedAt := time.Now()
	originalClarified := clarified

	var (
		emit      = p.emitter()
		plan      *models.ArchitectPlan
		reference *models.ReferenceSiteContext
	)

	if err := p.Check(); err != nil {
		return nil, err
	}

	var referenceMessage string
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

	err := withHeartbeat(
		ctx, emit,
		p.agentCfgs().Architect.Model,
		[]string{
			"Анализирую требования...",
			"Проектирую структуру базы данных...",
			"Продумываю навигацию и UX...",
			"Выбираю дизайн-систему...",
			"Создаю схему связей между таблицами...",
			"Определяю ролевую модель и доступы...",
			"Оцениваю сложность и объём...",
			"Финализирую архитектуру проекта...",
		},
		func() error {
			var err error
			plan, err = p.agent.ArchitectProject(ctx, models.ArchitectInput{
				Clarified:      clarified,
				OriginalPrompt: originalClarified,
				Images:         imageURLs,
				History:        chatHistory,
			})
			return err
		},
	)

	if err != nil {
		return nil, fmt.Errorf("architect phase failed: %w", err)
	}
	applyReferenceContextToPlan(plan, reference, originalClarified)

	if plan.ProjectName == "" {
		plan.ProjectName = "AI Project"
		if estimatedName != "" {
			plan.ProjectName = estimatedName
		}
	}

	tableNames := make([]string, 0, len(plan.Tables))
	for _, t := range plan.Tables {
		tableNames = append(tableNames, t.Label)
	}

	emit.Emit(
		SSEEvent{
			Type:    EvPlan,
			Icon:    "layout",
			Percent: 12,
			Message: fmt.Sprintf("Архитектура готова: %d таблиц", len(plan.Tables)),
			Value:   plan.ProjectName,
			Data: PlanEventData{
				ProjectName: plan.ProjectName,
				ProjectType: plan.ProjectType,
				Tables:      tableNames,
				TableCount:  len(plan.Tables),
			},
		},
	)

	log.Printf("[new-project] architect done: name=%q type=%q design=%s", plan.ProjectName, plan.ProjectType, plan.Design.DesignInspiration)

	time.Sleep(1500 * time.Millisecond)

	emit.Emit(
		SSEEvent{
			Type:    EvProgress,
			Icon:    "folder-plus",
			Message: "Создаю проект параллельно с планированием файлов...",
			Percent: 13,
		},
	)

	var (
		projectData   *models.ProjectData
		eagerManifest *models.ProjectManifest
		earlyPool     helper.ImagePoolResult

		provWg sync.WaitGroup
	)

	projectData, err = p.provisionBackend(ctx, plan.ProjectName, p.mcpProjectId, plan.ProjectType)
	if err != nil {
		return nil, fmt.Errorf("backend provisioning failed: %w", err)
	}

	if plan.ProjectType == "admin_panel" || plan.ProjectType == "web" || usesWebAppGenerator(plan.ProjectType) {
		provWg.Add(1)
		go func() {
			defer provWg.Done()

			eagerManifest, err = p.agent.GenerateManifest(ctx, models.ManifestInput{
				Plan:    plan,
				History: chatHistory,
			})
			if err == nil && eagerManifest != nil && len(eagerManifest.Groups) >= 2 {
				log.Printf("[new-project] eager manifest ready: %d groups", len(eagerManifest.Groups))
			} else {
				log.Printf("[new-project] eager manifest skipped (err=%v) — will retry in chunked", err)
			}
		}()
	}

	if p.baseConf.UnsplashAccessKey != "" && !plan.CloneMode {
		provWg.Add(1)
		go func() {
			defer provWg.Done()
			earlyPool = helper.FetchImagePool(ctx, p.baseConf.UnsplashAccessKey, plan)
		}()
	}

	provWg.Wait()

	if earlyPool.Err == nil && projectData != nil {
		earlyPool = p.uploadImagePool(ctx, projectData.ResourceEnvId, earlyPool)
	}

	if earlyPool.Err == nil && len(earlyPool.ThumbURLs) > 0 {
		injectMockDataImages(plan, earlyPool.ThumbURLs)
		p.cachedImagePool = &earlyPool
	}

	p.mcpProjectId = projectData.McpProjectId
	if eagerManifest != nil {
		p.prebuiltManifest = eagerManifest
	}

	if !(plan.CloneMode && len(plan.Tables) == 0) {
		emit.Emit(
			SSEEvent{
				Type:    EvProgress,
				Icon:    "database",
				Percent: 15,
				Message: "Создаю таблицы в базе данных",
				Value:   fmt.Sprintf("%d таблиц", len(plan.Tables)),
			},
		)

		go func(bPlan *models.ArchitectPlan, pd models.ProjectData) {
			if backendErr := createBackendFromPlan(context.Background(), bPlan, pd, p.service, emit); backendErr != nil {
				log.Printf("[new-project] async table creation failed: %v", backendErr)
			}
		}(plan, *projectData)
	}

	generated, err := p.generateCode(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	if err != nil {
		return nil, err
	}

	generated.Project.Files = append(
		[]models.ProjectFile{buildReadmeFile(plan, generated.Project.Files)},
		generated.Project.Files...,
	)

	emitPublishFiles(emit, generated.Project.Files, 93)
	mfeURL, err := p.publishToMicrofrontend(ctx, plan.ProjectName, uniqueMFEPath(), generated, projectData, plan.ProjectType)
	if err != nil {
		return nil, fmt.Errorf("microfrontend publish failed: %w", err)
	}

	p.injectYandexMetrica(ctx, plan.ProjectName, mfeURL, generated.Project.Files)

	if plan.ProjectType == mobileProjectType {
		mobileProject := newMobileProject(generated.Project, plan.MobileCapabilities)
		emitMobileProject(emit, mobileProject)
		log.Printf("[new-project] Capacitor mobile published: mfe_id=%s files=%d", p.microFrontendId, len(generated.Project.Files))
		return &models.ParsedClaudeResponse{
			Project:       generated.Project,
			MobileProject: mobileProject,
			Description:   buildProjectSummary(plan, generated.Project.Files, int(time.Since(startedAt).Seconds())),
		}, nil
	}

	//_, err = p.saveProject(ctx, generated)
	//if err != nil {
	//	log.Println("save project failed:", err)
	//}

	// Only admin panels ship the embedded u-code builder assistant. Best-effort: the
	// app is already published, so a failed integration must not fail the generation.
	if plan.ProjectType == "admin_panel" {
		if _, intErr := p.integrateBuilderAgentIntoFrontend(ctx, chatHistory); intErr != nil {
			log.Printf("[new-project] builder assistant integration failed (non-fatal): %v", intErr)
		}
	}

	log.Printf("[new-project] done — mfe_id=%s", p.microFrontendId)
	return &models.ParsedClaudeResponse{Description: buildProjectSummary(plan, generated.Project.Files, int(time.Since(startedAt).Seconds()))}, nil
}

func (p *ChatProcessor) buildMicrofrontendForCurrentProject(ctx context.Context, clarified string, chatHistory []models.ChatMessage, imageURLs []string, estimatedName string) (*models.ParsedClaudeResponse, error) {
	startedAt := time.Now()
	originalClarified := clarified
	emit := p.emitter()

	// Fetch existing project schema so the architect knows which tables/APIs are already available.
	var schemaCtx string
	schema, err := p.getProjectSchemaCached(ctx, p.resourceEnvId)
	if err != nil {
		log.Printf("[mfe-current] could not fetch project schema (non-fatal): %v", err)
	} else if len(schema) > 0 {
		schemaLines := strings.Builder{}
		for _, t := range schema {
			schemaLines.WriteString(fmt.Sprintf("- table: %s (slug: %s)\n", t.Label, t.Slug))
			for _, f := range t.Fields {
				schemaLines.WriteString(fmt.Sprintf("  * %s (%s)\n", f.Slug, f.Type))
			}
		}
		schemaCtx = schemaLines.String()
	}

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

	var plan *models.ArchitectPlan
	if err := withHeartbeat(ctx, emit,
		p.agentCfgs().Architect.Model,
		[]string{
			"Анализирую требования...",
			"Проектирую структуру базы данных...",
			"Продумываю навигацию и UX...",
			"Выбираю дизайн-систему...",
			"Создаю схему связей между таблицами...",
			"Определяю ролевую модель и доступы...",
			"Оцениваю сложность и объём...",
			"Финализирую архитектуру проекта...",
		},
		func() error {
			var e error
			plan, e = p.agent.ArchitectProject(ctx, models.ArchitectInput{
				Clarified:         clarified,
				OriginalPrompt:    originalClarified,
				ExistingSchemaCtx: schemaCtx,
				Images:            imageURLs,
				History:           chatHistory,
			})
			return e
		},
	); err != nil {
		return nil, fmt.Errorf("architect phase failed: %w", err)
	}
	applyReferenceContextToPlan(plan, reference, originalClarified)

	if plan.ProjectName == "" {
		plan.ProjectName = "AI Project"
		if estimatedName != "" {
			plan.ProjectName = estimatedName
		}
	}

	tableNames := make([]string, 0, len(plan.Tables))
	for _, t := range plan.Tables {
		tableNames = append(tableNames, t.Label)
	}
	emit.Emit(SSEEvent{
		Type:    EvPlan,
		Icon:    "layout",
		Percent: 12,
		Message: fmt.Sprintf("Архитектура готова: %d таблиц", len(plan.Tables)),
		Value:   plan.ProjectName,
		Data: PlanEventData{
			ProjectName: plan.ProjectName,
			ProjectType: plan.ProjectType,
			Tables:      tableNames,
			TableCount:  len(plan.Tables),
		},
	})

	log.Printf("[mfe-current] architect done: name=%q type=%q design=%s", plan.ProjectName, plan.ProjectType, plan.Design.DesignInspiration)

	time.Sleep(1500 * time.Millisecond) // let user read the plan

	var (
		mfePool   helper.ImagePoolResult
		mfePoolWg sync.WaitGroup
	)
	if p.baseConf.UnsplashAccessKey != "" && !plan.CloneMode {
		mfePoolWg.Add(1)
		go func() {
			defer mfePoolWg.Done()
			mfePool = helper.FetchImagePool(ctx, p.baseConf.UnsplashAccessKey, plan)
		}()
	}

	emit.Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    "folder-open",
		Message: "Получаю данные проекта...",
		Percent: 13,
	})
	projectData, err := p.getExistingProjectData(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get existing project data: %w", err)
	}

	mfePoolWg.Wait()

	if mfePool.Err == nil && projectData != nil {
		mfePool = p.uploadImagePool(ctx, projectData.ResourceEnvId, mfePool)
	}

	if mfePool.Err == nil && len(mfePool.ThumbURLs) > 0 {
		injectMockDataImages(plan, mfePool.ThumbURLs)
		p.cachedImagePool = &mfePool
	}

	// Create any NEW tables the architect defined that don't yet exist in the project.
	if len(plan.Tables) > 0 {
		existingSlugs := make(map[string]bool, len(schema))
		for _, t := range schema {
			existingSlugs[t.Slug] = true
		}
		newTables := make([]models.TablePlan, 0)
		for _, t := range plan.Tables {
			if !existingSlugs[t.Slug] {
				newTables = append(newTables, t)
			}
		}
		if len(newTables) > 0 {
			log.Printf("[mfe-current] architect defined %d new table(s) — provisioning async", len(newTables))
			emit.Emit(SSEEvent{
				Type:    EvProgress,
				Icon:    "database",
				Percent: 15,
				Message: "Создаю новые таблицы в базе данных",
				Value:   fmt.Sprintf("%d таблиц", len(newTables)),
			})
			// Keep relations that reference at least one new table so FK columns are created.
			var newSlugs = make(map[string]bool, len(newTables))
			for _, t := range newTables {
				newSlugs[t.Slug] = true
			}
			var newRelations []models.TableRelationPlan
			for _, r := range plan.Relations {
				if newSlugs[r.TableFrom] || newSlugs[r.TableTo] {
					newRelations = append(newRelations, r)
				}
			}
			newPlan := &models.ArchitectPlan{
				ProjectName: plan.ProjectName,
				ProjectType: plan.ProjectType,
				Tables:      newTables,
				Relations:   newRelations,
			}
			go func(bPlan *models.ArchitectPlan, pd models.ProjectData) {
				if err := createBackendFromPlan(context.Background(), bPlan, pd, p.service, emit); err != nil {
					log.Printf("[mfe-current] async table creation failed: %v", err)
				}
			}(newPlan, *projectData)
		}
	}

	if plan.ProjectType == "admin_panel" || plan.ProjectType == "web" || usesWebAppGenerator(plan.ProjectType) {
		manifest, manifestErr := p.agent.GenerateManifest(ctx, models.ManifestInput{
			Plan:    plan,
			History: chatHistory,
		})
		if manifestErr != nil {
			return nil, fmt.Errorf("manifest generation failed: %w", manifestErr)
		}
		if manifest == nil || len(manifest.Groups) < 2 {
			return nil, fmt.Errorf("manifest generation returned incomplete structure")
		}
		p.prebuiltManifest = manifest
	}

	generated, err := p.generateCode(ctx, clarified, imageURLs, chatHistory, plan, projectData)
	if err != nil {
		return nil, err
	}

	generated.Project.Files = append(
		[]models.ProjectFile{buildReadmeFile(plan, generated.Project.Files)},
		generated.Project.Files...,
	)

	emitPublishFiles(emit, generated.Project.Files, 93)
	mfeURL, err := p.publishToMicrofrontend(ctx, plan.ProjectName, uniqueMFEPath(), generated, projectData, plan.ProjectType)
	if err != nil {
		return nil, fmt.Errorf("microfrontend publish failed: %w", err)
	}

	p.injectYandexMetrica(ctx, plan.ProjectName, mfeURL, generated.Project.Files)

	if plan.ProjectType == mobileProjectType {
		mobileProject := newMobileProject(generated.Project, plan.MobileCapabilities)
		emitMobileProject(emit, mobileProject)
		log.Printf("[mfe-current] Capacitor mobile published: mfe_id=%s files=%d", p.microFrontendId, len(generated.Project.Files))
		return &models.ParsedClaudeResponse{
			Project:       generated.Project,
			MobileProject: mobileProject,
			Description:   buildProjectSummary(plan, generated.Project.Files, int(time.Since(startedAt).Seconds())),
		}, nil
	}

	// Admin panels built inside an existing project also ship the embedded builder
	// assistant. Best-effort, same as the new-project path.
	if plan.ProjectType == "admin_panel" {
		if _, intErr := p.integrateBuilderAgentIntoFrontend(ctx, chatHistory); intErr != nil {
			log.Printf("[mfe-current] builder assistant integration failed (non-fatal): %v", intErr)
		}
	}

	log.Printf("[mfe-current] done — mfe_id=%s", p.microFrontendId)
	return &models.ParsedClaudeResponse{Description: buildProjectSummary(plan, generated.Project.Files, int(time.Since(startedAt).Seconds()))}, nil
}

func (p *ChatProcessor) getExistingProjectData(ctx context.Context) (*models.ProjectData, error) {
	ucodeProjectId := p.ucodeProjectId
	authMode := "none"

	if p.mcpProjectId != "" {
		mcpProject, err := p.service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(ctx, &pbo.McpProjectId{
			ResourceEnvId: p.resourceEnvId,
			Id:            p.mcpProjectId,
		})
		if err == nil && mcpProject != nil {
			authMode = resolveGeneratedAuthMode(&models.ProjectData{AuthMode: getMcpProjectAuthMode(mcpProject)}, mcpProject.GetProjectType())
			if mcpProject.GetUcodeProjectId() != "" {
				ucodeProjectId = mcpProject.GetUcodeProjectId()
				log.Printf("[GET EXISTING PROJECT] using ucode_project_id=%s from MCP project", ucodeProjectId)
			}
		}
	}

	envList, err := p.h.companyServices.Environment().GetList(ctx, &pb.GetEnvironmentListRequest{
		ProjectId: ucodeProjectId,
		Limit:     1,
	})
	if err != nil {
		return nil, fmt.Errorf("get environment list: %w", err)
	}

	envs := envList.GetEnvironments()
	if len(envs) == 0 {
		return nil, fmt.Errorf("no environment found for project %s", ucodeProjectId)
	}
	env := envs[0]

	apiKeys, err := p.h.authService.ApiKey().GetList(ctx, &as.GetListReq{
		EnvironmentId: env.GetId(),
		ProjectId:     ucodeProjectId,
		Limit:         1,
	})
	if err != nil {
		return nil, fmt.Errorf("get api keys: %w", err)
	}

	var apiKey string

	if keys := apiKeys.GetData(); len(keys) > 0 {
		apiKey = keys[0].GetAppId()
	}

	return &models.ProjectData{
		UcodeProjectId: ucodeProjectId,
		McpProjectId:   p.mcpProjectId,
		EnvironmentId:  env.GetId(),
		ResourceEnvId:  env.GetResourceEnvironmentId(),
		ApiKey:         apiKey,
		AuthMode:       authMode,
	}, nil
}

func getMcpProjectAuthMode(project *pbo.McpProject) string {
	if project == nil || project.GetProjectEnv() == nil {
		return "none"
	}

	mode, ok := project.GetProjectEnv().AsMap()["auth_mode"].(string)
	if ok && strings.EqualFold(mode, "login") {
		return "login"
	}

	return "none"
}

func resolveGeneratedAuthMode(projectData *models.ProjectData, projectType string) string {
	if strings.EqualFold(projectType, "admin_panel") {
		return "login"
	}

	if projectData != nil && strings.EqualFold(projectData.AuthMode, "login") {
		return "login"
	}

	return "none"
}

func (p *ChatProcessor) provisionBackend(ctx context.Context, projectName string, existingMcpId string, projectType string) (*models.ProjectData, error) {
	authMode := "none"
	projectEnvMap := make(map[string]any)
	if existingMcpId != "" {
		mcpProject, err := p.service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(ctx, &pbo.McpProjectId{
			ResourceEnvId: p.resourceEnvId,
			Id:            existingMcpId,
			WithoutFiles:  true,
		})
		if err == nil && mcpProject != nil {
			authMode = getMcpProjectAuthMode(mcpProject)
			if mcpProject.GetProjectEnv() != nil {
				for key, value := range mcpProject.GetProjectEnv().AsMap() {
					projectEnvMap[key] = value
				}
			}
		}
	}

	currentProject, err := p.h.companyServices.Project().GetById(
		ctx, &pb.GetProjectByIdRequest{
			ProjectId: p.ucodeProjectId,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("get current project info: %w", err)
	}

	p.companyId = currentProject.GetCompanyId()

	backendProject, err := p.h.companyServices.Project().Create(
		ctx, &pb.CreateProjectRequest{
			Title:        sanitizeProjectNameForBackend(projectName),
			CompanyId:    currentProject.GetCompanyId(),
			K8SNamespace: currentProject.GetK8SNamespace(),
			FareId:       p.fareId,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create backend project: %w", err)
	}

	env, err := p.h.companyServices.Environment().CreateV2(
		ctx, &pb.CreateEnvironmentRequest{
			CompanyId:    currentProject.GetCompanyId(),
			ProjectId:    backendProject.GetProjectId(),
			UserId:       p.userId,
			ClientTypeId: p.clientTypeId,
			RoleId:       p.roleId,
			Name:         "Production",
			DisplayColor: "#00FF00",
			Description:  "Production Environment",
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create environment: %w", err)
	}

	resource, err := p.service.CompanyService().ServiceResource().GetSingle(
		ctx,
		&pb.GetSingleServiceResourceReq{
			ProjectId:     backendProject.GetProjectId(),
			EnvironmentId: env.GetId(),
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)

	if err != nil {
		return nil, fmt.Errorf("get resource for project: %w", err)
	}

	apiKeys, err := p.h.authService.ApiKey().GetList(
		ctx, &as.GetListReq{
			EnvironmentId: env.GetId(),
			ProjectId:     backendProject.GetProjectId(),
			Limit:         1,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("fetch api keys: %w", err)
	}

	var apiKey string

	if len(apiKeys.GetData()) > 0 {
		apiKey = apiKeys.GetData()[0].GetAppId()
	}

	authMode = resolveGeneratedAuthMode(&models.ProjectData{AuthMode: authMode}, projectType)
	if authMode == "login" {
		projectEnvMap["auth_mode"] = "login"
	}
	projectEnv, err := helperFunc.ConvertMapToStruct(projectEnvMap)
	if err != nil {
		return nil, fmt.Errorf("convert MCP project env: %w", err)
	}

	mcpProjectId := existingMcpId
	if mcpProjectId != "" {
		_, err = p.service.GoObjectBuilderService().McpProject().UpdateMcpProject(
			ctx, &pbo.McpProject{
				ResourceEnvId:  p.resourceEnvId,
				Id:             mcpProjectId,
				Title:          projectName,
				Description:    "Provisioned by AI architect",
				UcodeProjectId: backendProject.GetProjectId(),
				ApiKey:         apiKey,
				EnvironmentId:  env.GetId(),
				Status:         "ready",
				ProjectType:    projectType,
				ProjectEnv:     projectEnv,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("update MCP project: %w", err)
		}
	} else {
		project, err := p.service.GoObjectBuilderService().McpProject().CreateMcpProject(
			ctx, &pbo.CreateMcpProjectReqeust{
				ResourceEnvId:  p.resourceEnvId,
				Title:          projectName,
				Description:    "Generated by AI Architect",
				UcodeProjectId: backendProject.GetProjectId(),
				ApiKey:         apiKey,
				EnvironmentId:  env.GetId(),
				Status:         "ready",
				ProjectType:    projectType,
				ProjectEnv:     projectEnv,
			},
		)
		if err != nil {
			return nil, fmt.Errorf("create MCP project link: %w", err)
		}
		mcpProjectId = project.GetId()
	}

	p.mcpUcodeProjectId = backendProject.GetProjectId()

	return &models.ProjectData{
		UcodeProjectId: backendProject.GetProjectId(),
		McpProjectId:   mcpProjectId,
		ApiKey:         apiKey,
		EnvironmentId:  env.GetId(),
		ResourceEnvId:  resource.GetResourceEnvironmentId(),
		NodeType:       resource.GetNodeType(),
		ResourceType:   int32(resource.GetResourceType()),
		AuthMode:       authMode,
	}, nil
}

func (p *ChatProcessor) runVisualEdit(ctx context.Context, instruction string, contexts []models.VisualContext, chatHistory []models.ChatMessage, imageURLs []string) (*models.ParsedClaudeResponse, error) {
	log.Printf("[VISUAL EDIT] starting: count=%d", len(contexts))
	p.ensureEditPromptOverrides(ctx)

	emit := p.emitter()
	emit.Emit(SSEEvent{Type: EvProgress, Icon: "scan-search", Message: "Загружаю файлы проекта...", Percent: 5})

	existingFiles, err := p.fetchMicrofrontendFiles(ctx)
	if err != nil {
		return nil, fmt.Errorf("visual edit: failed to fetch microFrontend files: %w", err)
	}

	targetPaths := make(map[string]bool)
	resolvedContexts := make([]models.VisualContext, 0, len(contexts))

	for _, vc := range contexts {
		var foundPath string
		for _, f := range existingFiles {
			if vc.Path != "" && f.FilePath == vc.Path {
				foundPath = f.FilePath
				break
			}
			if vc.Path == "" && vc.ElementName != "" && strings.Contains(f.Content, vc.ElementName) {
				foundPath = f.FilePath
				break
			}
		}
		if foundPath != "" {
			targetPaths[foundPath] = true
			vc.Path = foundPath
			resolvedContexts = append(resolvedContexts, vc)
		} else {
			log.Printf("[VISUAL EDIT] WARNING: could not resolve file for element %q (path: %q)", vc.ElementName, vc.Path)
		}
	}

	if len(targetPaths) == 0 {
		log.Printf("[VISUAL EDIT] no specific files found for contexts, falling back to microFrontend edit flow")
		fileGraphJSON := p.buildMicrofrontendFileGraphJSON(existingFiles)
		return p.runMicrofrontendEdit(ctx, instruction, fileGraphJSON, chatHistory, imageURLs, existingFiles)
	}

	paths := make([]string, 0, len(targetPaths))
	for path := range targetPaths {
		paths = append(paths, path)
	}
	filesContext := p.buildMicrofrontendFilesContext(existingFiles, paths)

	emit.Emit(
		SSEEvent{
			Type:    EvProgress,
			Icon:    "mouse-pointer-click",
			Message: "Редактирую выбранные элементы",
			Value:   fmt.Sprintf("%d компонент(ов)", len(targetPaths)),
			Percent: 10,
		},
	)

	prompt := chat_prompts.BuildVisualEditPrompt(instruction, resolvedContexts, filesContext)
	messages := buildMessagesWithHistory(chatHistory, buildContentBlocksWithImages(prompt, imageURLs))

	if err := p.Check(); err != nil {
		return nil, err
	}

	var editedFiles []models.ProjectFile
	var editedSummary string

	err = withHeartbeat(
		ctx, emit,
		p.editModel(),
		[]string{
			"Редактирую компоненты...",
			"Применяю визуальные изменения...",
			"Обновляю стили и разметку...",
			"Проверяю совместимость...",
			"Финализирую правки...",
		},
		func() error {
			var errIn error
			editedFiles, editedSummary, errIn = p.agent.VisualEdit(ctx, models.VisualEditInput{
				Messages:       messages,
				PromptOverride: p.editPromptOverrides.VisualEditor,
			})
			return errIn
		},
	)
	if err != nil {
		return nil, fmt.Errorf("visual edit: claude call failed: %w", err)
	}
	_ = editedSummary

	if len(editedFiles) > 0 {
		// A custom prompt is user-controlled, so enforce visual-selection scope in
		// code instead of relying on the model to follow prompt instructions. Keep
		// the default path byte-for-byte compatible when no override is present.
		if strings.TrimSpace(p.editPromptOverrides.VisualEditor) != "" {
			editedFiles = filterProjectFilesByPath(editedFiles, targetPaths)
		}
	}

	if len(editedFiles) > 0 {
		// Keep the pre-built auth runtime intact even when a visual edit touches
		// (or the model rewrites) LoginPage / auth.ts / permissions.ts.
		editedFiles = enforceAuthRuntime(editedFiles, existingFiles)
		emit.Emit(
			SSEEvent{
				Type:    EvPublish,
				Icon:    "upload-cloud",
				Message: "Публикую изменения",
				Value:   fmt.Sprintf("%d файл(ов)", len(editedFiles)),
				Percent: 90,
			},
		)
		if pushErr := p.pushMicrofrontendChanges(ctx, editedFiles); pushErr != nil {
			return nil, fmt.Errorf("visual edit: push to u-gen failed: %w", pushErr)
		}

		editedMap := make(map[string]string, len(editedFiles))
		for _, f := range editedFiles {
			editedMap[f.Path] = f.Content
		}

		fullSnapshot := make([]models.GitlabFileChange, 0, len(existingFiles))
		for _, f := range existingFiles {
			if newContent, changed := editedMap[f.FilePath]; changed {
				fullSnapshot = append(fullSnapshot, models.GitlabFileChange{FilePath: f.FilePath, Content: newContent})
				delete(editedMap, f.FilePath)
			} else {
				fullSnapshot = append(fullSnapshot, f)
			}
		}
		for path, content := range editedMap {
			fullSnapshot = append(fullSnapshot, models.GitlabFileChange{FilePath: path, Content: content})
		}
		p.createMicrofrontendSnapshot(ctx, fullSnapshot, editedSummary)
	}

	description := editedSummary
	if description == "" {
		description = "✅ Visual edit applied successfully."
	}

	log.Printf("[VISUAL EDIT] ✅ done — %d files pushed to u-gen, summary=%s", len(editedFiles), description)
	return &models.ParsedClaudeResponse{Description: description}, nil
}

// ============================================================================
// DATA ACCESS HELPERS
// ============================================================================

func (p *ChatProcessor) saveMessage(ctx context.Context, role, content string, images []string) (*pbo.Message, error) {
	return p.service.GoObjectBuilderService().AiChat().CreateMessage(ctx, &pbo.CreateMessageRequest{
		ChatId:        p.chatId,
		Role:          role,
		Content:       content,
		Images:        images,
		ResourceEnvId: p.resourceEnvId,
	})
}

// chatHistoryWindow is wider than the router's own window so
// DetectConversationState can still find a state marker that fell outside the
// router's view (e.g. when a long TZ was pasted between question and answer).
const chatHistoryWindow = 20

func (p *ChatProcessor) getChatHistory(ctx context.Context) ([]models.ChatMessage, error) {
	messages, err := p.service.GoObjectBuilderService().AiChat().GetMessages(ctx, &pbo.GetMessagesRequest{
		ResourceEnvId: p.resourceEnvId,
		ChatId:        p.chatId,
	})
	if err != nil {
		return nil, fmt.Errorf("get chat history: %w", err)
	}

	msgList := messages.GetMessages()
	if len(msgList) > chatHistoryWindow {
		msgList = msgList[len(msgList)-chatHistoryWindow:]
	}

	result := make([]models.ChatMessage, 0, len(msgList))
	for _, msg := range msgList {
		result = append(result, models.ChatMessage{
			Role:    msg.GetRole(),
			Content: []models.ContentBlock{{Type: "text", Text: stripMarkerJSON(msg.GetContent())}},
		})
	}
	return result, nil
}

// stripMarkerJSON drops the JSON payload from a marker-prefixed message so the
// router context stays small. The marker and one-line summary are kept because
// DetectConversationState branches on the prefix.
func stripMarkerJSON(text string) string {
	switch {
	case strings.HasPrefix(text, ai.MarkerDiagramsGenerated+" "),
		strings.HasPrefix(text, ai.MarkerQuestionsAsked+" "),
		strings.HasPrefix(text, ai.MarkerError+" "):
		if idx := strings.Index(text, "\n"); idx != -1 {
			return text[:idx]
		}
	}
	return text
}

func (p *ChatProcessor) saveProject(ctx context.Context, req *models.ParsedClaudeResponse) (*pbo.McpProject, error) {
	if req == nil || req.Project == nil {
		return nil, fmt.Errorf("invalid project data")
	}

	projectEnv, err := helperFunc.ConvertMapToStruct(req.Project.Env)
	if err != nil {
		return nil, fmt.Errorf("convert project env: %w", err)
	}

	var projectFiles []*pbo.McpProjectFiles
	for _, file := range req.Project.Files {
		var fileGraph map[string]any
		if val, ok := req.Project.FileGraph[file.Path].(map[string]any); ok {
			fileGraph = val
		}
		fileGraphStruct, _ := helperFunc.ConvertMapToStruct(fileGraph)
		projectFiles = append(projectFiles, &pbo.McpProjectFiles{
			Path:      file.Path,
			Content:   file.Content,
			FileGraph: fileGraphStruct,
		})
	}

	projectType := ""
	if req.MobileProject != nil {
		projectType = req.MobileProject.ProjectType
	}

	return p.service.GoObjectBuilderService().McpProject().UpdateMcpProject(ctx, &pbo.McpProject{
		Id:            p.mcpProjectId,
		ResourceEnvId: p.resourceEnvId,
		Title:         truncateString(req.Project.ProjectName, 255),
		Description:   truncateString(req.Description, 255),
		ProjectFiles:  projectFiles,
		ProjectEnv:    projectEnv,
		ProjectType:   projectType,
	})
}

// ============================================================================
// UTILITIES
// ============================================================================

func truncateString(s string, maxLen int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

func buildAPIConfigBlock(baseURL string, projectData *models.ProjectData, plan *models.ArchitectPlan) string {

	var (
		envBaseURLKey = "VITE_API_BASE_URL"
		envAPIKeyKey  = "VITE_X_API_KEY"

		sb              strings.Builder
		loginTableSlugs []string
	)

	apiKey := ""
	if projectData != nil {
		apiKey = projectData.ApiKey
	}

	for _, f := range GetTemplateContext() {
		if strings.Contains(f.Path, "config/axios") || strings.Contains(f.Path, "lib/api") {
			if strings.Contains(f.Content, "VITE_BASE_URL") && !strings.Contains(f.Content, "VITE_API_BASE_URL") {
				envBaseURLKey = "VITE_BASE_URL"
			}
			if strings.Contains(f.Content, "VITE_API_KEY") && !strings.Contains(f.Content, "VITE_X_API_KEY") {
				envAPIKeyKey = "VITE_API_KEY"
			}
		}
	}

	sb.WriteString(fmt.Sprintf(
		"\n====================================\nAPI CONFIGURATION FOR FRONTEND\n====================================\n%s: %s\n%s: %s\n\nREQUIRED axios headers (BOTH — never omit either):\n  headers: { 'Authorization': 'API-KEY', '%s': import.meta.env.%s }\n\nTables to use:\n",
		envBaseURLKey, baseURL, envAPIKeyKey, apiKey,
		envAPIKeyKey, envAPIKeyKey,
	))

	for _, t := range plan.Tables {
		if t.IsLoginTable {
			loginTableSlugs = append(loginTableSlugs, t.Slug)
			sb.WriteString(fmt.Sprintf("- LOGIN TABLE: %s, slug: %s\n", t.Label, t.Slug))
			sb.WriteString("  * built-in auth fields (always present in DB): login, password, email, phone\n")
			sb.WriteString("  * always-required system fields: role_id, client_type_id\n")
			for _, f := range t.Fields {
				sb.WriteString(fmt.Sprintf("  * custom field: %s, type: %s\n", f.Slug, f.Type))
			}
		} else {
			sb.WriteString(fmt.Sprintf("- Table: %s, slug: %s\n", t.Label, t.Slug))
			for _, f := range t.Fields {
				hint := fieldRenderHint(f.Slug, f.Type)
				sb.WriteString(fmt.Sprintf("  * field: %s, type: %s  → %s\n", f.Slug, f.Type, hint))
			}
		}
	}

	if len(plan.Tables) > 0 {
		sb.WriteString(`
⚠ MANDATORY API RULE — NO EXCEPTIONS:
Every non-login table listed above MUST have at least one page or section that:
  1. Fetches its data using useApiQuery (NEVER hardcoded arrays or objects)
  2. Renders the fetched data in the UI (cards, lists, tables — whatever fits)
  3. Uses extractList<T>(data) to extract the response
Static/hardcoded content is FORBIDDEN when API tables exist.

⚠ CARD DATA RENDERING — inside every .map() ALL content from item fields (see → hints above):
  DECLARE thumbPool const ABOVE return (3–6 THUMB URLs from IMAGE_POOL in API CONFIG):
    const thumbPool = ['https://...thumb1', 'https://...thumb2', 'https://...thumb3']
  IMAGE fields (→ IMAGE hint):    src={item.field_slug ?? thumbPool[i % thumbPool.length]}
  TEXT fields  (→ TEXT hint):     {item.field_slug ?? '—'}
  CURRENCY fields (→ CURRENCY):  {formatCurrency(item.field_slug ?? 0)}
  DATE fields  (→ DATE hint):    {formatDate(item.field_slug ?? '')}
  HARD BAN inside .map() data cards:
    ❌ hardcoded image URL as src    ❌ hardcoded title/description    ❌ hardcoded price
  IMAGE_POOL URLs: hero/background decoration ONLY — never as direct src in .map() data cards.
`)
	}

	if len(loginTableSlugs) > 0 {
		slug := loginTableSlugs[0]
		sb.WriteString(`
====================================
LOGIN TABLE — MANDATORY FORM RULES
====================================
Login table slug: ` + slug + `

The login table stores project users. It has BUILT-IN auth fields you never define as fields but MUST include in forms.

CREATE FORM — include ALL of these fields (in this order):
  1. login          <Input type="text">      required
  2. password       <Input type="password">  required on CREATE, OMIT on EDIT
  3. email          <Input type="email">     required
  4. phone          <Input type="tel">       optional
  5. role_id        <Select>                 REQUIRED — fetch options:
       GET /v2/items/role
       Response: data.data.response[]   value=row.guid  label=row.name
  6. client_type_id <Select>                 REQUIRED — fetch options:
       GET /v2/items/client_type
       Response: data.data.response[]   value=row.guid  label=row.name
  7. [any custom fields defined for this table, e.g. full_name, avatar]

CREATE API endpoint:
  POST /v2/items/` + slug + `
  body: { "login":"...", "password":"plaintext", "email":"...", "role_id":"guid", "client_type_id":"guid", ...customFields }
  Password is PLAIN TEXT — the platform hashes it. Never hash on the frontend.

EDIT FORM: same fields except password is optional (send only if user typed a new one).
LIST VIEW: show login, email, custom name field — NEVER display password column.

HOOK PATTERN for role_id / client_type_id selects:
  const { data: rolesData } = useApiQuery<unknown>(['roles'], '/v2/items/role')
  const roles = extractList<{ guid: string; name: string }>(rolesData)
  const { data: ctData } = useApiQuery<unknown>(['client-types'], '/v2/items/client_type')
  const clientTypes = extractList<{ guid: string; name: string }>(ctData)
`)
	}

	if len(plan.Relations) > 0 {
		sb.WriteString("\nRelations (Many2One — FK column auto-created on source table):\n")
		for _, r := range plan.Relations {
			relFieldSlug := r.TableTo + "_id"
			sb.WriteString(fmt.Sprintf(
				"- %s → %s: FK field %q on %s (Select dropdown, load options from GET /v2/items/%s)\n",
				r.TableFrom, r.TableTo, relFieldSlug, r.TableFrom, r.TableTo,
			))
		}
		sb.WriteString(`
RELATION API RULES — READ CAREFULLY, WRONG USAGE BREAKS THE UI:

VALUE TYPE — CRITICAL:
  The FK field value is ALWAYS a guid STRING (UUID like "a1b2c3d4-...").
  NEVER store or submit an integer, number, or index.
  ❌ WRONG: { "customers_id": 1 }          — integer breaks the relation
  ❌ WRONG: { "customers_id": "1" }         — numeric string breaks the relation
  ✅ CORRECT: { "customers_id": "a1b2c3..." } — real guid from the related table

FETCH OPTIONS for <Select> dropdown (use GET /v2/items, NOT the old /v1/object endpoint):
  const { data: optData } = useApiQuery<unknown>(['{table_to}'], '/v2/items/{table_to}')
  const options = extractList<{ guid: string; name: string }>(optData)
  // CRITICAL: Radix SelectItem throws on empty string value. Always use a fallback.
  // in <Select>: value={row.guid || 'fallback'}  label={row.name ?? row.title ?? row.label}

CREATE/UPDATE with relation — send the guid string:
  { "{table_to}_id": selectedGuid, ...otherFields }
  State: const [selectedId, setSelectedId] = useState<string>('')
  On submit: include selectedId only if non-empty string.

DISPLAY related record name in list view:
  Join by guid client-side: options.find(o => o.guid === row['{table_to}_id'])?.name ?? '—'

DO NOT use Many2Many, array values, or numeric IDs — only single guid string per Many2One field.
`)
	}

	sb.WriteString("\nUse this UI Structure provided by the Architect:\n" + plan.UIStructure + "\n")
	if plan.Reference != nil && !strings.Contains(plan.UIStructure, referenceContextMarker) {
		sb.WriteString("\n" + buildReferenceSitePromptBlock(plan.Reference) + "\n")
	}

	// Inject design tokens so the coder doesn't have to invent a design system.
	d := plan.Design
	if d.PrimaryColor != "" {
		sb.WriteString("\n====================================\nDESIGN TOKENS\n====================================\n")
		if plan.CloneMode {
			sb.WriteString("⚠ CLONE MODE: the REFERENCE SITE CONTEXT block is the single design authority.\n")
			sb.WriteString("The tokens below are a FALLBACK only — wherever they conflict with the captured reference colors/fonts/screenshots, follow the reference.\n")
		}
		sb.WriteString(fmt.Sprintf("design_inspiration: %s\n", d.DesignInspiration))
		sb.WriteString(fmt.Sprintf("font_family (heading): %s\n", d.FontFamily))
		sb.WriteString(fmt.Sprintf("body_font: %s\n", d.BodyFont))
		sb.WriteString(fmt.Sprintf("border_radius: %s\n", d.BorderRadius))
		sb.WriteString(fmt.Sprintf("background_color: %s  (HSL: %s)\n", d.BackgroundColor, d.BackgroundHSL))
		sb.WriteString(fmt.Sprintf("surface_color: %s  (HSL: %s)\n", d.SurfaceColor, d.SurfaceHSL))
		sb.WriteString(fmt.Sprintf("primary_color: %s  (HSL: %s)\n", d.PrimaryColor, d.PrimaryHSL))
		sb.WriteString(fmt.Sprintf("accent_color: %s  (HSL: %s)\n", d.AccentColor, d.AccentHSL))
		sb.WriteString(fmt.Sprintf("text_color: %s\n", d.TextColor))
		sb.WriteString(fmt.Sprintf("text_muted_color: %s\n", d.TextMutedColor))
		sb.WriteString(fmt.Sprintf("border_color: %s\n", d.BorderColor))
		sb.WriteString(fmt.Sprintf("sidebar_background: %s  (HSL: %s)\n", d.SidebarBackground, d.SidebarBackgroundHSL))
		sb.WriteString(fmt.Sprintf("sidebar_foreground: %s\n", d.SidebarForeground))
		sb.WriteString(fmt.Sprintf("sidebar_style: %s\n", d.SidebarStyle))
	}

	return sb.String()
}

func fieldRenderHint(slug, fieldType string) string {
	slugLower := strings.ToLower(slug)

	imageKW := []string{"image", "photo", "avatar", "cover", "thumbnail", "banner", "img", "picture"}
	for _, kw := range imageKW {
		if strings.Contains(slugLower, kw) {
			return "IMAGE → src={item." + slug + " ?? thumbPool[i % thumbPool.length]}"
		}
	}

	moneyKW := []string{"price", "cost", "amount", "salary", "budget", "rate", "fee", "total"}
	for _, kw := range moneyKW {
		if strings.Contains(slugLower, kw) {
			return "CURRENCY → {formatCurrency(item." + slug + " ?? 0)}"
		}
	}

	dateKW := []string{"_at", "date", "time", "birth", "created", "updated", "expires"}
	for _, kw := range dateKW {
		if strings.Contains(slugLower, kw) {
			return "DATE → {formatDate(item." + slug + " ?? '')}"
		}
	}

	typeLower := strings.ToLower(fieldType)
	if typeLower == "bool" || typeLower == "boolean" {
		return "BOOL → {item." + slug + " ? 'Yes' : 'No'}"
	}

	return "TEXT → {item." + slug + " ?? '—'}"
}

func isImageSlug(slug string) bool {
	slugLower := strings.ToLower(slug)
	for _, kw := range []string{"image", "photo", "avatar", "cover", "thumbnail", "banner", "img", "picture"} {
		if strings.Contains(slugLower, kw) {
			return true
		}
	}
	return false
}

func injectMockDataImages(plan *models.ArchitectPlan, thumbURLs []string) {
	if len(thumbURLs) == 0 {
		return
	}
	idx := 0
	for ti := range plan.Tables {
		for _, f := range plan.Tables[ti].Fields {
			if !isImageSlug(f.Slug) {
				continue
			}
			for ri := range plan.Tables[ti].MockData {
				val := plan.Tables[ti].MockData[ri][f.Slug]
				if val == nil || val == "" {
					plan.Tables[ti].MockData[ri][f.Slug] = thumbURLs[idx%len(thumbURLs)]
					idx++
				}
			}
		}
	}
	if idx > 0 {
		log.Printf("[inject-images] patched %d image field(s) in mock_data", idx)
	}
}

func buildContentBlocksWithImages(textContent string, imageURLs []string) []models.ContentBlock {
	blocks := make([]models.ContentBlock, 0, len(imageURLs)+1)
	for _, imageURL := range imageURLs {
		if strings.TrimSpace(imageURL) != "" {
			blocks = append(blocks, models.ContentBlock{
				Type:   "image",
				Source: &models.ImageSource{Type: "url", URL: imageURL},
			})
		}
	}
	blocks = append(blocks, models.ContentBlock{Type: "text", Text: textContent})
	return blocks
}

func buildMessagesWithHistory(history []models.ChatMessage, contentBlocks []models.ContentBlock) []models.ChatMessage {
	messages := make([]models.ChatMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, models.ChatMessage{
		Role:    "user",
		Content: contentBlocks,
	})
	return messages
}

func buildHistoryText(history []models.ChatMessage) string {
	if len(history) == 0 {
		return ""
	}
	start := 0
	if len(history) > 6 {
		start = len(history) - 6
	}
	var sb strings.Builder
	for _, msg := range history[start:] {
		var text string
		for _, block := range msg.Content {
			if block.Type == "text" {
				text += block.Text
			}
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", strings.ToUpper(msg.Role), text))
	}
	return sb.String()
}

// uniqueMFEPath returns a short unique GitLab path for a new microfrontend:
// "app-XXXXXX" where XXXXXX is 6 random lowercase hex chars.
// This prevents GitLab project name collisions on retries.
// Format: 10 chars → functionPath in function service stays ≤ 20 chars.
func uniqueMFEPath() string {
	b := make([]byte, 3)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("app-%x", time.Now().UnixNano()&0xFFFFFF)
	}
	return fmt.Sprintf("app-%x", b)
}

func sanitizeProjectNameForBackend(name string) string {
	cyrillicMap := map[rune]string{
		'а': "a", 'б': "b", 'в': "v", 'г': "g", 'д': "d", 'е': "e", 'ё': "yo",
		'ж': "zh", 'з': "z", 'и': "i", 'й': "y", 'к': "k", 'л': "l", 'м': "m",
		'н': "n", 'о': "o", 'п': "p", 'р': "r", 'с': "s", 'т': "t", 'у': "u",
		'ф': "f", 'х': "kh", 'ц': "ts", 'ч': "ch", 'ш': "sh", 'щ': "sch",
		'ъ': "", 'ы': "y", 'ь': "", 'э': "e", 'ю': "yu", 'я': "ya",
		'А': "A", 'Б': "B", 'В': "V", 'Г': "G", 'Д': "D", 'Е': "E", 'Ё': "Yo",
		'Ж': "Zh", 'З': "Z", 'И': "I", 'Й': "Y", 'К': "K", 'Л': "L", 'М': "M",
		'Н': "N", 'О': "O", 'П': "P", 'Р': "R", 'С': "S", 'Т': "T", 'У': "U",
		'Ф': "F", 'Х': "Kh", 'Ц': "Ts", 'Ч': "Ch", 'Ш': "Sh", 'Щ': "Sch",
		'Ъ': "", 'Ы': "Y", 'Ь': "", 'Э': "E", 'Ю': "Yu", 'Я': "Ya",
	}
	var sb strings.Builder
	for _, r := range name {
		if lat, ok := cyrillicMap[r]; ok {
			sb.WriteString(lat)
		} else {
			sb.WriteRune(r)
		}
	}

	folded, _, err := transform.String(
		transform.Chain(norm.NFD, runes.Remove(runes.In(unicode.Mn)), norm.NFC),
		sb.String(),
	)
	if err != nil {
		folded = sb.String()
	}

	var ascii strings.Builder
	for _, r := range folded {
		if r <= unicode.MaxASCII {
			ascii.WriteRune(r)
		}
	}

	result := strings.TrimSpace(ascii.String())
	if result == "" {
		return "project"
	}
	return result
}

func slugify(name string) string {
	s := strings.ToLower(strings.TrimSpace(name))
	re := regexp.MustCompile(`[^a-z0-9]+`)
	s = re.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) == 0 {
		s = "ai-project"
	}
	return s
}

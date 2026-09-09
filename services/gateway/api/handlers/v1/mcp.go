package v1

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/anthropic"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/helper/mcp_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	as "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/auth_service"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/gin-gonic/gin"
)

// ====================  MCP HANDLERS  ====================

func (h *HandlerV1) McpCreateBackend(c *gin.Context) {
	var request models.MCPRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	scope, err := h.resolveProjectScope(c)
	if err != nil {
		return
	}

	if request.Method == "" {
		request.Method = "project"
	}

	content, message, err := mcp_prompts.BuildBackendPrompt(
		models.GeneratePromptRequest{
			ProjectId:     scope.ProjectId,
			EnvironmentId: scope.EnvironmentId,
			Method:        request.Method,
			APIKey:        scope.APIKey,
			UserPrompt:    request.Prompt,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	_, err = h.sendAnthropicBackend(content)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, message)
}

func (h *HandlerV1) McpGenerateProject(c *gin.Context) {
	var request models.MCPRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	scope, err := h.resolveProjectScope(c)
	if err != nil {
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     scope.ProjectId,
			EnvironmentId: scope.EnvironmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	projectSrv, err := h.GetProjectSrvc(c.Request.Context(), scope.ProjectId, resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	originalPrompt := request.Prompt
	var reference *models.ReferenceSiteContext
	var referenceMessage string
	request.Prompt, request.ImageURLs, reference, referenceMessage = prepareReferencePrompt(c.Request.Context(), h.baseConf, request.Prompt, request.ImageURLs)
	if referenceMessage != "" {
		h.HandleResponse(c, status_http.BadRequest, referenceMessage)
		return
	}

	if !shouldSkipBackendForReferencePrompt(originalPrompt, reference) {
		go func() {
			bgContent, _, bgErr := mcp_prompts.BuildBackendPrompt(
				models.GeneratePromptRequest{
					ProjectId:     scope.ProjectId,
					EnvironmentId: scope.EnvironmentId,
					Method:        "project",
					APIKey:        scope.APIKey,
					UserPrompt:    request.Prompt,
				},
			)
			if bgErr != nil {
				log.Printf("Background BuildBackendPrompt Error: %v", bgErr)
				return
			}

			_, bgErr = h.sendAnthropicBackend(bgContent)
			if bgErr != nil {
				log.Printf("Background sendAnthropicBackend Error: %v", bgErr)
				return
			}
		}()
	}

	userPrompt := mcp_prompts.BuildFrontendGeneratePrompt(
		models.GeneratePromptRequest{
			ProjectId:     scope.ProjectId,
			EnvironmentId: scope.EnvironmentId,
			APIKey:        scope.APIKey,
			UserPrompt:    request.Prompt,
			BaseURL:       h.baseConf.UcodeBaseUrl,
		},
	)

	project, err := h.generateFrontendProject(userPrompt, request.ImageURLs)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "AI Generation Failed: "+err.Error())
		return
	}

	projectEnv, _ := helperFunc.ConvertMapToStruct(project.Env)

	var (
		projectFiles []*pbo.McpProjectFiles
		fileGraph    map[string]any
	)

	for _, file := range project.Files {
		if val, ok := project.FileGraph[file.Path].(map[string]any); ok {
			fileGraph = val
		}

		fileGraphStruct, _ := helperFunc.ConvertMapToStruct(fileGraph)

		projectFiles = append(projectFiles, &pbo.McpProjectFiles{
			Path:      file.Path,
			Content:   file.Content,
			FileGraph: fileGraphStruct,
		})

		fileGraph = make(map[string]any)
	}

	saveProject := pbo.CreateMcpProjectReqeust{
		ResourceEnvId: resource.ResourceEnvironmentId,
		Title:         project.ProjectName,
		Description:   "Generated by ucode with claude-sonnet-4-5",
		ProjectEnv:    projectEnv,
		ProjectFiles:  projectFiles,
	}

	createdProject, err := projectSrv.GoObjectBuilderService().McpProject().CreateMcpProject(context.Background(), &saveProject)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, createdProject)
}

func (h *HandlerV1) MCPUpdateFrontend(c *gin.Context) {
	var (
		request      models.MCPRequest
		mcpProjectId = c.Param("mcp_project_id")
	)

	if err := c.ShouldBindJSON(&request); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	scope, err := h.resolveProjectScope(c)
	if err != nil {
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     scope.ProjectId,
			EnvironmentId: scope.EnvironmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	service, err := h.GetProjectSrvc(c.Request.Context(), scope.ProjectId, resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	classification, err := h.classifyRequest(request.Prompt, request.ImageURLs)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, "Classification failed: "+err.Error())
		return
	}

	log.Printf("Classification result: backend=%v, frontend=%v",
		classification.RequiresBackend,
		classification.RequiresFrontend,
	)

	if !classification.RequiresBackend && !classification.RequiresFrontend {
		h.HandleResponse(c, status_http.OK, map[string]string{
			"status":  "success",
			"message": "No operations required for this request",
		})
		return
	}

	if classification.RequiresBackend {
		var promptReq = models.GeneratePromptRequest{
			ProjectId:     scope.ProjectId,
			EnvironmentId: scope.EnvironmentId,
			UserPrompt:    request.Prompt,
			APIKey:        scope.APIKey,
			Method:        "table",
		}

		if !classification.RequiresFrontend {
			backendContent, backendMsg, err := mcp_prompts.BuildBackendPrompt(promptReq)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			_, err = h.sendAnthropicBackend(backendContent)
			if err != nil {
				h.HandleResponse(c, status_http.GRPCError, err.Error())
				return
			}
			h.HandleResponse(c, status_http.OK, map[string]any{"message": backendMsg})
			return
		}

		go func() {
			content, _, err := mcp_prompts.BuildBackendPrompt(promptReq)
			if err != nil {
				log.Println("Backend async build failed: " + err.Error())
				return
			}
			_, err = h.sendAnthropicBackend(content)
			if err != nil {
				log.Println("Backend async send failed: " + err.Error())
				return
			}
		}()
	}

	projectFilesRes, err := service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		c.Request.Context(),
		&pbo.McpProjectId{
			Id:            mcpProjectId,
			ResourceEnvId: resource.ResourceEnvironmentId,
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var (
		filesGraphMap = make(map[string]any)
		filesMap      = make(map[string]string)
		filesToUpdate []models.ProjectFile

		mcpProjectFiles []*pbo.McpProjectFiles
	)

	for _, file := range projectFilesRes.ProjectFiles {
		filesGraphMap[file.Path] = file.FileGraph.AsMap()
		filesMap[file.Path] = file.Content
	}

	var analyzeReq = models.AnalyzeFrontendPromptRequest{
		UserRequest: request.Prompt,
		FileGraph:   filesGraphMap,
		ProjectName: projectFilesRes.Title,
		Context:     request.Context,
	}

	analysisPrompt, err := mcp_prompts.BuildFrontendAnalyzePrompt(analyzeReq)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	analysis, err := h.analyzeProject(analysisPrompt, request.ImageURLs)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	log.Printf("Analysis: modify=%d, create=%d, delete=%d",
		len(analysis.FilesToModify),
		len(analysis.NewFilesNeeded),
		len(analysis.FilesToDelete),
	)

	if len(analysis.FilesToModify) == 0 && len(analysis.NewFilesNeeded) == 0 && len(analysis.FilesToDelete) == 0 {
		h.HandleResponse(c, status_http.OK, map[string]any{
			"status":             "success",
			"message":            "No frontend changes needed",
			"backend_processing": classification.RequiresBackend,
		})
		return
	}

	for _, fileToMod := range analysis.FilesToModify {
		if fileContent, exists := filesMap[fileToMod.Path]; exists {
			filesToUpdate = append(filesToUpdate, models.ProjectFile{
				Path:    fileToMod.Path,
				Content: fileContent,
			})
		}
	}

	var updateReq = models.UpdateFrontendPromptRequest{
		UserRequest:    request.Prompt,
		FilesToUpdate:  filesToUpdate,
		AnalysisResult: *analysis,
		ProjectName:    projectFilesRes.Title,
		Context:        request.Context,
	}

	updatePrompt, err := mcp_prompts.BuildFrontendUpdatePrompt(updateReq)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	update, err := h.updateProject(updatePrompt, request.ImageURLs)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	for _, file := range update.UpdatedFiles {
		filesMap[file.Path] = file.Content
		if graphUpdate, ok := update.FileGraphUpdates[file.Path]; ok {
			filesGraphMap[file.Path] = graphUpdate
		}
	}

	for _, file := range update.NewFiles {
		filesMap[file.Path] = file.Content
		if graphUpdate, ok := update.FileGraphUpdates[file.Path]; ok {
			filesGraphMap[file.Path] = graphUpdate
		}
	}

	for path, content := range filesMap {
		fileGraph, _ := filesGraphMap[path].(map[string]any)
		fileGraphStruct, _ := helperFunc.ConvertMapToStruct(fileGraph)

		mcpProjectFiles = append(mcpProjectFiles, &pbo.McpProjectFiles{
			ProjectId: mcpProjectId,
			Path:      path,
			Content:   content,
			FileGraph: fileGraphStruct,
		})
	}

	var saveProject = pbo.McpProject{
		ResourceEnvId: resource.ResourceEnvironmentId,
		Id:            mcpProjectId,
		ProjectFiles:  mcpProjectFiles,
	}

	_, err = service.GoObjectBuilderService().McpProject().UpdateMcpProject(context.Background(), &saveProject)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, update)
}

func (h *HandlerV1) McpGeneratePlan(c *gin.Context) {
	var req models.MCPRequest

	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	if req.Prompt == "" {
		h.HandleResponse(c, status_http.BadRequest, "prompt is required")
		return
	}

	originalPrompt := req.Prompt
	var reference *models.ReferenceSiteContext
	var referenceMessage string
	req.Prompt, req.ImageURLs, reference, referenceMessage = prepareReferencePrompt(c.Request.Context(), h.baseConf, req.Prompt, req.ImageURLs)
	if referenceMessage != "" {
		h.HandleResponse(c, status_http.BadRequest, referenceMessage)
		return
	}

	backendPlan := "NO_BACKEND_REQUIRED: Pure reference-site landing/website clone. Do not create database tables unless the user explicitly asks for dynamic data."
	if !shouldSkipBackendForReferencePrompt(originalPrompt, reference) {
		var err error
		backendPlan, err = h.generateBackendPlan(req.Prompt)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, "Backend plan generation failed: "+err.Error())
			return
		}
	}

	frontendPlan, err := h.generateFrontendPlan(req.Prompt, req.ImageURLs)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, "Frontend plan generation failed: "+err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, gin.H{
		"backend_plan":  backendPlan,
		"frontend_plan": frontendPlan,
		"prompt":        req.Prompt,
		"image_urls":    req.ImageURLs,
	})
}

func (h *HandlerV1) McpGenerateProjectV2(c *gin.Context) {
	var (
		req              models.MCPRequestWithPlanning
		withPlanning     bool
		generatedProject *models.GeneratedProject
	)

	withPlanning = c.Query("with_planning") == "true"

	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, err.Error())
		return
	}

	scope, err := h.resolveProjectScope(c)
	if err != nil {
		return
	}

	resource, err := h.companyServices.ServiceResource().GetSingle(
		c.Request.Context(),
		&pb.GetSingleServiceResourceReq{
			ProjectId:     scope.ProjectId,
			EnvironmentId: scope.EnvironmentId,
			ServiceType:   pb.ServiceType_BUILDER_SERVICE,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	if resource.ResourceType != pb.ResourceType_POSTGRESQL {
		h.HandleResponse(c, status_http.InvalidArgument, "resource type not supported")
		return
	}

	projectSrv, err := h.GetProjectSrvc(c.Request.Context(), scope.ProjectId, resource.NodeType)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	var generatePromptReq = models.GeneratePromptRequest{
		UserPrompt:    req.Prompt,
		ProjectId:     scope.ProjectId,
		EnvironmentId: scope.EnvironmentId,
		APIKey:        scope.APIKey,
		BaseURL:       h.baseConf.UcodeBaseUrl,
		Method:        "project",
	}

	originalPrompt := req.Prompt
	var reference *models.ReferenceSiteContext
	var referenceMessage string
	req.Prompt, req.ImageURLs, reference, referenceMessage = prepareReferencePrompt(c.Request.Context(), h.baseConf, req.Prompt, req.ImageURLs)
	if referenceMessage != "" {
		h.HandleResponse(c, status_http.BadRequest, referenceMessage)
		return
	}
	generatePromptReq.UserPrompt = req.Prompt
	skipBackend := shouldSkipBackendForReferencePrompt(originalPrompt, reference)

	if withPlanning {
		if req.BackendPlan == "" || req.FrontendPlan == "" {
			h.HandleResponse(c, status_http.BadRequest, "backend_plan and frontend_plan are required when with_planning=true")
			return
		}

		var (
			backendPrompt  = mcp_prompts.BuildBackendPromptWithPlan(generatePromptReq, req.BackendPlan)
			frontendPrompt = mcp_prompts.BuildFrontendPromptWithPlan(generatePromptReq, req.FrontendPlan)
		)

		if !skipBackend {
			go func() {
				if _, backendErr := h.sendAnthropicBackend(backendPrompt); backendErr != nil {
					log.Printf("Backend plan execution failed: %v\n", backendErr)
				}
			}()
		}

		generatedProject, err = h.generateFrontendProject(frontendPrompt, req.ImageURLs)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, "Frontend generation failed: "+err.Error())
			return
		}

	} else {
		if req.Prompt == "" {
			h.HandleResponse(c, status_http.BadRequest, "prompt is required")
			return
		}

		if !skipBackend {
			go func() {
				content, _, err := mcp_prompts.BuildBackendPrompt(generatePromptReq)
				if err != nil {
					log.Printf("Backend generation failed: %v\n", err)
					return
				}
				if _, backendErr := h.sendAnthropicBackend(content); backendErr != nil {
					log.Printf("Backend execution failed: %v\n", backendErr)
					return
				}
			}()
		}

		userPrompt := mcp_prompts.BuildFrontendGeneratePrompt(generatePromptReq)

		generatedProject, err = h.generateFrontendProject(userPrompt, req.ImageURLs)
		if err != nil {
			h.HandleResponse(c, status_http.InternalServerError, "AI Generation Failed: "+err.Error())
			return
		}
	}

	createdProject, err := h.saveFrontendProject(
		c.Request.Context(),
		projectSrv,
		generatedProject,
		resource.ResourceEnvironmentId,
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, createdProject)
}

// ========================= Main functions ============================

func (h *HandlerV1) resolveProjectScope(c *gin.Context) (*models.ProjectScope, error) {
	var (
		projectIDRaw any
		envIDRaw     any
		exists       bool
	)

	projectIDRaw, exists = c.Get("project_id")
	if !exists || !util.IsValidUUID(projectIDRaw.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrProjectIdValid)
		return nil, config.ErrProjectIdValid
	}

	envIDRaw, exists = c.Get("environment_id")
	if !exists || !util.IsValidUUID(envIDRaw.(string)) {
		h.HandleResponse(c, status_http.InvalidArgument, config.ErrEnvironmentIdValid)
		return nil, config.ErrEnvironmentIdValid
	}

	apiKeys, err := h.authService.ApiKey().GetList(c.Request.Context(), &as.GetListReq{
		EnvironmentId: envIDRaw.(string),
		ProjectId:     projectIDRaw.(string),
		Limit:         1,
		Offset:        0,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return nil, err
	}
	if len(apiKeys.Data) < 1 {
		h.HandleResponse(c, status_http.InvalidArgument, "Api key not found")
		return nil, fmt.Errorf("api key not found")
	}

	return &models.ProjectScope{
		ProjectId:     projectIDRaw.(string),
		EnvironmentId: envIDRaw.(string),
		APIKey:        apiKeys.GetData()[0].GetAppId(),
	}, nil
}

func (h *HandlerV1) generateBackendPlan(userPrompt string) (string, error) {
	var (
		body = anthropic.AnthropicRequest{
			Model:     h.baseConf.ClaudeModel,
			MaxTokens: h.baseConf.GeneratePlanMaxTokens,
			System:    mcp_prompts.SystemPromptPlanBackend,
			Messages: []models.ChatMessage{
				{
					Role: "user",
					Content: []models.ContentBlock{
						{
							Type: "text",
							Text: mcp_prompts.BuildBackendPlanPrompt(userPrompt),
						},
					},
				},
			},
		}
		apiResponse models.ClaudeResponse
	)

	respText, err := anthropic.CallAnthropicAPI(h.baseConf, body, 300*time.Second)
	if err != nil {
		return "", fmt.Errorf("backend plan API call failed: %w", err)
	}

	if err = json.Unmarshal([]byte(respText), &apiResponse); err != nil {
		return "", fmt.Errorf("failed to parse backend plan response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return "", fmt.Errorf("empty backend plan response")
	}

	return apiResponse.Content[0].Text, nil
}

func (h *HandlerV1) generateFrontendPlan(userPrompt string, imageURLs []string) (string, error) {
	var (
		contentBlocks []models.ContentBlock
		body          anthropic.AnthropicRequest
		apiResponse   models.ClaudeResponse
	)

	for _, imageURL := range imageURLs {
		if imageURL != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "image",
				Source: &models.ImageSource{
					Type: "url",
					URL:  imageURL,
				},
			})
		}
	}

	contentBlocks = append(contentBlocks, models.ContentBlock{
		Type: "text",
		Text: mcp_prompts.BuildFrontendPlanPrompt(userPrompt, len(imageURLs) > 0),
	})

	body = anthropic.AnthropicRequest{
		Model:     h.baseConf.ClaudeModel,
		MaxTokens: h.baseConf.GeneratePlanMaxTokens,
		System:    mcp_prompts.SystemPromptPlanFrontend,
		Messages: []models.ChatMessage{
			{
				Role:    "user",
				Content: contentBlocks,
			},
		},
	}

	respText, err := anthropic.CallAnthropicAPI(h.baseConf, body, 300*time.Second)
	if err != nil {
		return "", fmt.Errorf("frontend plan API call failed: %w", err)
	}

	if err = json.Unmarshal([]byte(respText), &apiResponse); err != nil {
		return "", fmt.Errorf("failed to parse frontend plan response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return "", fmt.Errorf("empty frontend plan response")
	}

	return apiResponse.Content[0].Text, nil
}

func (h *HandlerV1) saveFrontendProject(ctx context.Context, services services.ServiceManagerI, project *models.GeneratedProject, resourceEnvId string) (*pbo.McpProject, error) {
	var (
		saveProject = pbo.CreateMcpProjectReqeust{
			ResourceEnvId: resourceEnvId,
			Title:         project.ProjectName,
			Description:   "Generated by ucode with claude-sonnet-4-5",
		}

		projectFiles []*pbo.McpProjectFiles
		fileGraph    map[string]any
	)

	projectEnv, _ := helperFunc.ConvertMapToStruct(project.Env)

	for _, file := range project.Files {
		if val, ok := project.FileGraph[file.Path].(map[string]any); ok {
			fileGraph = val
		}

		fileGraphStruct, _ := helperFunc.ConvertMapToStruct(fileGraph)

		projectFiles = append(projectFiles, &pbo.McpProjectFiles{
			Path:      file.Path,
			Content:   file.Content,
			FileGraph: fileGraphStruct,
		})

		fileGraph = make(map[string]any)
	}

	saveProject.ProjectEnv = projectEnv
	saveProject.ProjectFiles = projectFiles

	return services.GoObjectBuilderService().McpProject().CreateMcpProject(ctx, &saveProject)
}

// ==================== Send Anthropic Request methods ====================

func (h *HandlerV1) sendAnthropicBackend(content string) (string, error) {
	return anthropic.CallAnthropicAPI(h.baseConf,
		anthropic.AnthropicRequest{
			Model:     h.baseConf.ClaudeModel,
			MaxTokens: h.baseConf.MaxTokens,
			System:    mcp_prompts.SystemPromptBackend,
			Messages: []models.ChatMessage{
				{
					Role: "user",
					Content: []models.ContentBlock{
						{
							Type: "text",
							Text: content,
						},
					},
				},
			},
			MCPServers: []anthropic.MCPServer{
				{
					Type: "url",
					URL:  h.baseConf.MCPServerURL,
					Name: "ucode",
				},
			},
			Tools: []anthropic.MCPTool{
				{
					Type:          "mcp_toolset",
					MCPServerName: "ucode",
				},
			},
		},
		420*time.Second,
	)
}

func (h *HandlerV1) generateFrontendProject(userPrompt string, imageURLs []string) (*models.GeneratedProject, error) {
	var (
		contentBlocks []models.ContentBlock
		body          anthropic.AnthropicRequest
		apiResponse   models.ClaudeResponse
		project       models.GeneratedProject
	)

	for _, imageURL := range imageURLs {
		if imageURL != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "image",
				Source: &models.ImageSource{
					Type: "url",
					URL:  imageURL,
				},
			})
		}
	}

	contentBlocks = append(contentBlocks, models.ContentBlock{
		Type: "text",
		Text: userPrompt,
	})

	body = anthropic.AnthropicRequest{
		Model:     h.baseConf.ClaudeModel,
		MaxTokens: h.baseConf.MaxTokens,
		System:    mcp_prompts.SystemPromptGenerateFrontend,
		Messages: []models.ChatMessage{
			{
				Role:    "user",
				Content: contentBlocks,
			},
		},
	}

	respText, err := anthropic.CallAnthropicAPI(h.baseConf, body, 420*time.Second)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal([]byte(respText), &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return nil, fmt.Errorf("empty content in response")
	}

	cleanedText := helper.CleanJSONResponse(apiResponse.Content[0].Text)

	if err = json.Unmarshal([]byte(cleanedText), &project); err != nil {
		return nil, fmt.Errorf("failed to parse project JSON: %w", err)
	}

	return &project, nil
}

func (h *HandlerV1) classifyRequest(prompt string, imageURLs []string) (*models.RequestClassification, error) {
	var (
		hasImages            = len(imageURLs) > 0
		classificationPrompt = mcp_prompts.BuildClassificationPrompt(prompt, hasImages)
		body                 = anthropic.AnthropicRequest{
			Model:     h.baseConf.ClaudeModel,
			MaxTokens: h.baseConf.ClassifyReqeustMaxTokens,
			System:    mcp_prompts.SystemPromptClassifyRequest,
			Messages: []models.ChatMessage{
				{
					Role: "user",
					Content: []models.ContentBlock{
						{
							Type: "text",
							Text: classificationPrompt,
						},
					},
				},
			},
		}
		apiResponse    models.ClaudeResponse
		classification models.RequestClassification
	)

	respText, err := anthropic.CallAnthropicAPI(h.baseConf, body, 60*time.Second)
	if err != nil {
		return nil, fmt.Errorf("classification API call failed: %w", err)
	}

	if err = json.Unmarshal([]byte(respText), &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse classification response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return nil, fmt.Errorf("empty classification response")
	}

	cleanedText := helper.CleanJSONResponse(apiResponse.Content[0].Text)

	if err = json.Unmarshal([]byte(cleanedText), &classification); err != nil {
		return nil, fmt.Errorf("failed to parse classification JSON: %w", err)
	}

	log.Printf("REQUEST CLASSIFIED: backend=%v, frontend=%v, confidence=%s",
		classification.RequiresBackend,
		classification.RequiresFrontend,
		classification.Confidence,
	)

	return &classification, nil
}

func (h *HandlerV1) analyzeProject(userPrompt string, imageURLs []string) (*models.AnalysisResult, error) {
	var (
		contentBlocks []models.ContentBlock
		body          anthropic.AnthropicRequest
		apiResponse   models.ClaudeResponse
		analysis      models.AnalysisResult
	)

	for _, imageURL := range imageURLs {
		if imageURL != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "image",
				Source: &models.ImageSource{
					Type: "url",
					URL:  imageURL,
				},
			})
		}
	}

	contentBlocks = append(contentBlocks, models.ContentBlock{
		Type: "text",
		Text: userPrompt,
	})

	body = anthropic.AnthropicRequest{
		Model:     h.baseConf.ClaudeModel,
		MaxTokens: h.baseConf.AnalyseProjectMaxTokens,
		System:    mcp_prompts.SystemPromptAnalyzeFrontend,
		Messages: []models.ChatMessage{
			{
				Role:    "user",
				Content: contentBlocks,
			},
		},
	}

	respText, err := anthropic.CallAnthropicAPI(h.baseConf, body, 120*time.Second)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal([]byte(respText), &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return nil, fmt.Errorf("empty content")
	}

	cleanedText := helper.CleanJSONResponse(apiResponse.Content[0].Text)

	if err = json.Unmarshal([]byte(cleanedText), &analysis); err != nil {
		return nil, fmt.Errorf("failed to parse analysis: %w", err)
	}

	return &analysis, nil
}

func (h *HandlerV1) updateProject(userPrompt string, imageURLs []string) (*models.UpdateResult, error) {
	var (
		contentBlocks []models.ContentBlock
		body          anthropic.AnthropicRequest
		apiResponse   models.ClaudeResponse
		update        models.UpdateResult
	)

	for _, imageURL := range imageURLs {
		if imageURL != "" {
			contentBlocks = append(contentBlocks, models.ContentBlock{
				Type: "image",
				Source: &models.ImageSource{
					Type: "url",
					URL:  imageURL,
				},
			})
		}
	}

	contentBlocks = append(contentBlocks, models.ContentBlock{
		Type: "text",
		Text: userPrompt,
	})

	body = anthropic.AnthropicRequest{
		Model:     h.baseConf.ClaudeModel,
		MaxTokens: h.baseConf.MaxTokens,
		System:    mcp_prompts.SystemPromptUpdateFrontend,
		Messages: []models.ChatMessage{
			{
				Role:    "user",
				Content: contentBlocks,
			},
		},
		MCPServers: []anthropic.MCPServer{
			{
				Type: "url",
				URL:  h.baseConf.MCPServerURL,
				Name: "ucode",
			},
		},
		Tools: []anthropic.MCPTool{
			{
				Type:          "mcp_toolset",
				MCPServerName: "ucode",
			},
		},
	}

	respText, err := anthropic.CallAnthropicAPI(h.baseConf, body, 420*time.Second)
	if err != nil {
		return nil, err
	}

	if err = json.Unmarshal([]byte(respText), &apiResponse); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(apiResponse.Content) == 0 {
		return nil, fmt.Errorf("empty content")
	}

	cleanedText := helper.CleanJSONResponse(apiResponse.Content[0].Text)

	if err = json.Unmarshal([]byte(cleanedText), &update); err != nil {
		return nil, fmt.Errorf("failed to parse update: %w", err)
	}

	return &update, nil
}

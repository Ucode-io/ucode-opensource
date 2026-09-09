package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
)

func (h *HandlerV1) CreateAiChatMessage(c *gin.Context) {
	log.Println("[BUILD-CHECK] CreateAiChatMessage — reference-clone-fix build (2026-07-07)")

	var (
		userMessage models.NewMessageReq
		chatId      = c.Param("chat-id")
		ctx         = context.Background()
	)

	if err := c.ShouldBindJSON(&userMessage); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid request body: "+err.Error())
		return
	}

	isPendingConfirmation := userMessage.PendingAction != nil

	if !isPendingConfirmation && strings.TrimSpace(userMessage.Content) == "" {
		h.HandleResponse(c, status_http.BadRequest, "content is required")
		return
	}

	service, resourceEnvID, err := h.getAiChatServices(c)
	if err != nil {
		return
	}

	chat, err := service.GoObjectBuilderService().AiChat().GetChat(
		ctx, &pbo.ChatPrimaryKey{
			ResourceEnvId: resourceEnvID,
			Id:            chatId,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to get chat: %v", err))
		return
	}

	mcpProject, err := service.GoObjectBuilderService().McpProject().GetMcpProjectFiles(
		ctx, &pbo.McpProjectId{
			ResourceEnvId: resourceEnvID,
			Id:            chat.GetProjectId(),
			WithoutFiles:  false,
		},
	)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to get mcp project: %v", err))
		return
	}

	authInfo, err := h.adminAuthInfo(c)
	if err != nil {
		h.HandleResponse(c, status_http.Unauthorized, "unauthorized")
		return
	}

	realProjectID := c.GetString("project_id")

	processor := newChatProcessor(
		h,
		service,
		h.baseConf,
		chatId,
		chat.GetProjectId(),
		resourceEnvID,
		realProjectID,
		authInfo.GetUserIdAuth(),
		authInfo.GetClientTypeId(),
		authInfo.GetRoleId(),
		c.GetHeader("Authorization"),
	)
	processor.microFrontendId = userMessage.MicrofrontendID
	processor.microFrontendRepoId = userMessage.MicrofrontendRepoID
	processor.microFrontendResourceEnvId = userMessage.ResourceEnvId
	processor.newProject = userMessage.NewProject
	processor.userMessage = userMessage.Content
	processor.mcpUcodeProjectId = mcpProject.GetUcodeProjectId()
	processor.mcpEnvironmentId = mcpProject.GetEnvironmentId()
	processor.ucodeMcpProjectId = mcpProject.GetId()

	// Lazy self-heal: persist the normalised provider so chats with empty
	// or unknown model migrate transparently on the next read or filter.
	storedModel := chat.GetModel()
	processor.chatModel = config.ParseAIProvider(storedModel)
	if normalised := string(processor.chatModel); storedModel != normalised {
		go func() {
			_, _ = service.GoObjectBuilderService().AiChat().UpdateChat(
				context.Background(),
				&pbo.UpdateChatRequest{
					ResourceEnvId: resourceEnvID,
					Id:            chatId,
					Model:         normalised,
				},
			)
		}()
	}

	if userMessage.UcodeProjectID != "" {
		processor.ucodeProjectId = userMessage.UcodeProjectID
	}

	proj, projErr := h.companyServices.Project().GetById(ctx, &pb.GetProjectByIdRequest{ProjectId: realProjectID})
	if projErr != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to get project: %v", projErr))
		return
	}
	processor.companyId = proj.GetCompanyId()
	processor.fareId = proj.GetFareId()
	if processor.fareId == "" {
		h.HandleResponse(c, status_http.GRPCError, "project fare not found")
		return
	}

	processor.initAgent()

	chatHistory, err := processor.getChatHistory(ctx)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to fetch history: %v", err))
		return
	}

	if isPendingConfirmation {
		h.handlePendingConfirmation(c, ctx, processor, userMessage, chatHistory, service, resourceEnvID, chatId)
		return
	}

	_, err = processor.saveMessage(ctx, "user", userMessage.Content, userMessage.Images)
	if err != nil {
		chatErr := newSaveMessageError(err)
		if c.Query("stream") == "true" {
			h.sseError(c, chatErr)
			return
		}
		h.HandleResponse(c, status_http.GRPCError, errorResponseBody(chatErr))
		return
	}

	if c.Query("stream") == "true" {
		h.handleStreamingMessage(c, processor, userMessage, chatHistory, service, resourceEnvID, chatId)
		return
	}

	aiResponse, err := processor.routeAndProcess(ctx, userMessage, chatHistory)
	if err != nil {
		chatErr := processor.persistPipelineError(ctx, err)

		log.Println("GENERATION ERROR:", err)

		var tokenErr *TokenLimitError
		if errors.As(err, &tokenErr) {
			h.HandleResponse(c, status_http.PaymentRequired, processor.tokenLimitData(tokenErr))
			return
		}
		h.HandleResponse(c, status_http.GRPCError, errorResponseBody(chatErr))
		return
	}

	if strings.TrimSpace(aiResponse.Description) == "" {
		if aiResponse.PendingAction != nil {
			aiResponse.Description = aiResponse.PendingAction.ConfirmationPrompt
		} else if len(aiResponse.Questions) > 0 {
			aiResponse.Description = aiResponse.Questions[0].Title
		} else if aiResponse.Plan != nil {
			aiResponse.Description = "Here are the diagrams for your project. Review them and let me know when you're ready to build."
		} else {
			aiResponse.Description = "Project has been updated."
		}
	}

	var (
		savedContent   = aiResponse.Description
		updatedProject *pbo.McpProject
	)

	if len(aiResponse.Questions) > 0 {
		questionsJSON, _ := json.Marshal(aiResponse.Questions)
		savedContent = "[QUESTIONS_ASKED] " + aiResponse.Description + "\n" + string(questionsJSON)
	} else if aiResponse.Plan != nil {
		planJSON, _ := json.Marshal(aiResponse.Plan)
		savedContent = "[DIAGRAMS_GENERATED] " + aiResponse.Description + "\n" + string(planJSON)
	}

	message, err := processor.saveMessage(ctx, "assistant", savedContent, nil)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to save ai message: %v", err))
		return
	}

	if aiResponse.Project != nil {
		updatedProject, err = processor.saveProject(ctx, aiResponse)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to save project: %v", err))
			return
		}
	}

	if len(chatHistory) == 0 {
		_, _ = service.GoObjectBuilderService().AiChat().UpdateChat(
			ctx, &pbo.UpdateChatRequest{
				ResourceEnvId: resourceEnvID,
				Id:            chatId,
				Title:         truncateString(userMessage.Content, 100),
				Description:   truncateString(aiResponse.Description, 255),
				ProjectId:     processor.mcpProjectId,
			},
		)
	}

	em := models.EnrichedMessage{
		ID:         message.GetId(),
		ChatID:     message.GetChatId(),
		Role:       message.GetRole(),
		Content:    message.GetContent(),
		Images:     message.GetImages(),
		HasFiles:   message.GetHasFiles(),
		TokensUsed: message.GetTokensUsed(),
		CreatedAt:  message.GetCreatedAt(),
	}

	if strings.HasPrefix(em.Content, "[DIAGRAMS_GENERATED] ") {
		body := strings.TrimPrefix(em.Content, "[DIAGRAMS_GENERATED] ")
		if idx := strings.Index(body, "\n"); idx != -1 {
			em.Content = body[:idx]
		} else {
			em.Content = body
		}
	} else if strings.HasPrefix(em.Content, "[QUESTIONS_ASKED] ") {
		em.Content = strings.TrimPrefix(em.Content, "[QUESTIONS_ASKED] ")
	}

	h.HandleResponse(c, status_http.Created, map[string]any{
		"message":               em,
		"project":               updatedProject,
		"mcp_project_id":        processor.mcpProjectId,
		"microfrontend_id":      processor.microFrontendId,
		"microfrontend_repo_id": processor.microFrontendRepoId,
		"ucode_project_id":      processor.mcpUcodeProjectId,
		"pending_action":        aiResponse.PendingAction,
		"questions":             aiResponse.Questions,
		"plan":                  aiResponse.Plan,
		"mobile_project":        aiResponse.MobileProject,
	})
}

func (h *HandlerV1) handlePendingConfirmation(
	c *gin.Context,
	ctx context.Context,
	processor *ChatProcessor,
	req models.NewMessageReq,
	chatHistory []models.ChatMessage,
	service services.ServiceManagerI,
	resourceEnvID, chatId string,
) {
	action := req.PendingAction

	// Use explicit content if provided, otherwise derive from approval status
	userContent := strings.TrimSpace(req.Content)
	if userContent == "" {
		if action.Approved {
			userContent = "Да"
		} else {
			userContent = "Нет"
		}
	}

	_, err := processor.saveMessage(ctx, "user", userContent, nil)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to save confirmation message: %v", err))
		return
	}

	var (
		assistantReply string
		mutationResult any
	)

	if !action.Approved {
		assistantReply = action.CancelMessage
		if strings.TrimSpace(assistantReply) == "" {
			assistantReply = "Окей, действие отменено. Ничего не изменено."
		}
	} else {
		mutationResult, err = executeMutation(ctx, action, service)
		if err != nil {
			h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("mutation failed: %v", err))
			return
		}

		assistantReply = action.SuccessMessage
		if strings.TrimSpace(assistantReply) == "" {
			switch action.Action {
			case "create":
				assistantReply = fmt.Sprintf("✅ Запись успешно создана в `%s`.", action.TableSlug)
			case "update":
				assistantReply = fmt.Sprintf("✅ Обновлено **%d** запис(ей) в `%s`.", action.AffectedCount, action.TableSlug)
			case "delete":
				assistantReply = fmt.Sprintf("✅ Удалено **%d** запис(ей) из `%s`.", action.AffectedCount, action.TableSlug)
			default:
				assistantReply = "✅ Готово."
			}
		}
	}

	message, err := processor.saveMessage(ctx, "assistant", assistantReply, nil)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to save assistant message: %v", err))
		return
	}

	if len(chatHistory) == 0 {
		_, _ = service.GoObjectBuilderService().AiChat().UpdateChat(
			ctx, &pbo.UpdateChatRequest{
				ResourceEnvId: resourceEnvID,
				Id:            chatId,
				Title:         truncateString(userContent, 100),
				Description:   truncateString(assistantReply, 255),
				ProjectId:     processor.mcpProjectId,
			},
		)
	}

	h.HandleResponse(c, status_http.Created, map[string]any{
		"message":         message,
		"mcp_project_id":  processor.mcpProjectId,
		"mutation_result": mutationResult,
	})
}

// sseError sends a single structured EvError event for pre-pipeline failures
// (e.g. the user message can't be persisted) and closes the connection. The
// payload mirrors the EvError produced by the pipeline so the frontend has a
// single rendering branch for all SSE errors.
func (h *HandlerV1) sseError(c *gin.Context, chatErr models.AiChatError) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)
	writeSSEEvent(c.Writer, SSEEvent{
		Type:    EvError,
		Icon:    IconAlertCircle,
		Message: chatErr.Message,
		Data:    errorEventData(chatErr, nil),
	})
	c.Writer.Flush()
}

// handleStreamingMessage runs the full AI pipeline in a background goroutine
// and streams SSE progress events to the client in real time.
//
// Architecture:
//
//	Background goroutine (context.Background) ──emit──▸ eventCh (buffered 64)
//	                                                         │
//	Foreground (HTTP goroutine) ◂──read──────────────────────┘──▸ c.Writer (SSE)
//
// If the client disconnects, the background goroutine keeps running so tokens
// are not wasted. Progress events may be dropped; terminal/mobile events are guaranteed.
func (h *HandlerV1) handleStreamingMessage(c *gin.Context, processor *ChatProcessor, userMessage models.NewMessageReq, chatHistory []models.ChatMessage, service services.ServiceManagerI, resourceEnvID, chatId string) {

	eventCh := make(chan SSEEvent, 64)
	processor.emit = &channelEmitter{ch: eventCh}

	// SSE transport headers — must be set before any body write.
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)

	startTime := time.Now()

	// ── Background: full pipeline + persistence ──────────────────────────
	// IMPORTANT: use context.Background(), NOT the HTTP request ctx.
	// If the client disconnects, c.Request.Context() gets cancelled, which
	// would abort the AI pipeline mid-generation (wasting tokens and money).
	// The channelEmitter may drop progress events when the channel is full.

	pipelineCtx := context.Background()

	go func() {
		defer close(eventCh)

		provData := processor.providerEventData()
		processor.emitter().Emit(SSEEvent{
			Type:    EvProvider,
			Icon:    "cpu",
			Message: processor.providerLabel(),
			Data:    provData,
		})
		processor.emitter().Emit(SSEEvent{
			Type:    EvProgress,
			Icon:    "sparkles",
			Message: "Начинаю обработку...",
			Value:   processor.agentCfgs().Router.Model,
			Percent: 1,
		})

		aiResponse, pipelineErr := processor.routeAndProcess(pipelineCtx, userMessage, chatHistory)
		if pipelineErr != nil {
			// Persist first: SSE may be lost to a disconnect, but the chat
			// log must reflect what happened.
			chatErr := processor.persistPipelineError(pipelineCtx, pipelineErr)

			icon := IconAlertCircle
			var extras map[string]any
			var tokenErr *TokenLimitError
			if errors.As(pipelineErr, &tokenErr) {
				icon = IconBan
				extras = map[string]any{"token_limit": processor.tokenLimitData(tokenErr)}
			}
			processor.emitter().Emit(SSEEvent{
				Type:    EvError,
				Icon:    icon,
				Message: chatErr.Message,
				Data:    errorEventData(chatErr, extras),
			})
			return
		}

		if strings.TrimSpace(aiResponse.Description) == "" {
			switch {
			case aiResponse.PendingAction != nil:
				aiResponse.Description = aiResponse.PendingAction.ConfirmationPrompt
			case len(aiResponse.Questions) > 0:
				aiResponse.Description = aiResponse.Questions[0].Title
			case aiResponse.Plan != nil:
				aiResponse.Description = "Here are the diagrams for your project. Review them and let me know when you're ready to build."
			default:
				aiResponse.Description = "Project has been updated."
			}
		}

		savedContent := aiResponse.Description
		if len(aiResponse.Questions) > 0 {
			questionsJSON, _ := json.Marshal(aiResponse.Questions)
			savedContent = "[QUESTIONS_ASKED] " + aiResponse.Description + "\n" + string(questionsJSON)
		} else if aiResponse.Plan != nil {
			planJSON, _ := json.Marshal(aiResponse.Plan)
			savedContent = "[DIAGRAMS_GENERATED] " + aiResponse.Description + "\n" + string(planJSON)
		}

		message, saveErr := processor.saveMessage(pipelineCtx, "assistant", savedContent, nil)
		if saveErr != nil {
			log.Printf("[SSE] failed to save assistant message: %v", saveErr)
		}

		var updatedProject *pbo.McpProject

		if aiResponse.Project != nil {
			var projErr error
			updatedProject, projErr = processor.saveProject(pipelineCtx, aiResponse)
			if projErr != nil {
				log.Printf("[SSE] failed to save project: %v", projErr)
			}
		}

		if len(chatHistory) == 0 {
			_, _ = service.GoObjectBuilderService().AiChat().UpdateChat(
				pipelineCtx, &pbo.UpdateChatRequest{
					ResourceEnvId: resourceEnvID,
					Id:            chatId,
					Title:         truncateString(userMessage.Content, 100),
					Description:   truncateString(aiResponse.Description, 255),
					ProjectId:     processor.mcpProjectId,
				},
			)
		}

		// Build enriched message for the final event.
		em := models.EnrichedMessage{Content: aiResponse.Description, Role: "assistant"}
		if message != nil {
			em.ID = message.GetId()
			em.ChatID = message.GetChatId()
			em.Role = message.GetRole()
			em.Content = message.GetContent()
			em.Images = message.GetImages()
			em.HasFiles = message.GetHasFiles()
			em.TokensUsed = message.GetTokensUsed()
			em.CreatedAt = message.GetCreatedAt()
		}
		if strings.HasPrefix(em.Content, "[DIAGRAMS_GENERATED] ") {
			body := strings.TrimPrefix(em.Content, "[DIAGRAMS_GENERATED] ")
			if idx := strings.Index(body, "\n"); idx != -1 {
				em.Content = body[:idx]
			} else {
				em.Content = body
			}
		} else if strings.HasPrefix(em.Content, "[QUESTIONS_ASKED] ") {
			em.Content = strings.TrimPrefix(em.Content, "[QUESTIONS_ASKED] ")
		}

		processor.emitter().Emit(
			SSEEvent{
				Type:    EvDone,
				Icon:    "check-circle",
				Percent: 100,
				Message: aiResponse.Description,
				Data: map[string]any{
					"message":               em,
					"project":               updatedProject,
					"mcp_project_id":        processor.mcpProjectId,
					"microfrontend_id":      processor.microFrontendId,
					"microfrontend_repo_id": processor.microFrontendRepoId,
					"ucode_project_id":      processor.mcpUcodeProjectId,
					"pending_action":        aiResponse.PendingAction,
					"questions":             aiResponse.Questions,
					"plan":                  aiResponse.Plan,
					"mobile_project":        aiResponse.MobileProject,
					"duration_sec":          int(time.Since(startTime).Seconds()),
				},
			},
		)
	}()

	// ── Foreground: drain eventCh → SSE response ─────────────────────────
	// We watch for clientGone (disconnect) but must NOT exit early if events
	// are still in the buffered channel — otherwise EvDone is lost.
	clientGone := c.Request.Context().Done()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	var lastSent time.Time
	const sseMinInterval = 400 * time.Millisecond

	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				return // channel closed — pipeline finished, all events delivered
			}
			if !lastSent.IsZero() {
				if gap := sseMinInterval - time.Since(lastSent); gap > 0 {
					time.Sleep(gap)
				}
			}
			writeSSEEvent(c.Writer, ev)
			c.Writer.Flush()
			lastSent = time.Now()
		case <-keepalive.C:
			// SSE comment keeps the connection alive through proxies.
			fmt.Fprintf(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		case <-clientGone:
			// Client disconnected — drain remaining buffered events so the
			// background goroutine is never blocked on a full channel.
			// The pipeline itself continues on pipelineCtx (context.Background).
			log.Printf("[SSE] client disconnected — draining remaining events")
			for range eventCh {
				// discard
			}
			return
		}
	}
}

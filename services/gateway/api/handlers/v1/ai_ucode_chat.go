package v1

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pbcs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	pbo "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"
)

const (
	chatTypeUcode = "ucode"

	// ucodeBuilderMaxSteps bounds the agentic tool-use loop. Building a schema
	ucodeBuilderMaxSteps = 16

	// ucodeBuilderMaxTokens caps a single model turn. Turns are small here — a
	ucodeBuilderMaxTokens = 8000

	ucodeStepTimeout     = 500 * time.Second
	ucodeRunTotalTimeout = 25 * time.Minute

	// model has context across turns without resending the whole chat.
	ucodeHistoryWindow = 20
)

const (
	StepActionSchema   = "schema"
	StepActionTable    = "table"
	StepActionField    = "field"
	StepActionRelation = "relation"
	StepActionMenu     = "menu"
	StepActionItems    = "items"
	StepActionData     = "data"  // read-only query (count / list / aggregate)
	StepActionLogin    = "login" // login table + its client_type / role
)

// Step statuses describe the lifecycle of a single build step.
const (
	StepStatusStarted = "started" // work began (paired with a later done)
	StepStatusDone    = "done"    // object created
	StepStatusSkipped = "skipped" // object already existed — nothing changed
	StepStatusFailed  = "failed"  // tool call failed; the AI may retry or adapt
)

// UcodeStepData is the structured payload attached to every ucode build event.
type UcodeStepData struct {
	Action     string `json:"action"`
	Status     string `json:"status"`
	Table      string `json:"table,omitempty"`
	TableFrom  string `json:"table_from,omitempty"`
	TableTo    string `json:"table_to,omitempty"`
	Field      string `json:"field,omitempty"`
	FieldType  string `json:"field_type,omitempty"`
	ForeignKey string `json:"foreign_key,omitempty"`
	MenuID     string `json:"menu_id,omitempty"`
	ClientType string `json:"client_type,omitempty"`
	Role       string `json:"role,omitempty"`
	Strategy   string `json:"strategy,omitempty"`
	Label      string `json:"label,omitempty"`
	Reason     string `json:"reason,omitempty"`
	Created    int    `json:"created,omitempty"`
	Failed     int    `json:"failed,omitempty"`
	Total      int    `json:"total,omitempty"`
}

// ucodeBuildStats accumulates what a single run created so the terminal event can
type ucodeBuildStats struct {
	Tables      int `json:"tables"`
	LoginTables int `json:"login_tables"`
	ClientTypes int `json:"client_types"`
	Roles       int `json:"roles"`
	Fields      int `json:"fields"`
	Relations   int `json:"relations"`
	Menus       int `json:"menus"`
	Items       int `json:"items"`
}

type ucodeChatSession struct {
	h       *HandlerV1
	service services.ServiceManagerI
	emit    ProgressEmitter

	model    ai.ChatModel
	modelID  string
	provider config.AIProvider

	resourceEnvId string
	envId         string
	projectId     string
	companyId     string
	chatId        string

	// Auth context, needed to create client_types/roles for a login table.
	nodeType     string
	resourceType int32

	// systemPrompt overrides the default builder prompt when set (e.g. the embedded
	// admin-panel assistant uses a narrow-chat-formatted variant). Empty = default.
	systemPrompt string

	tablesLoaded bool
	tableIDs     map[string]string
	fieldSets    map[string]map[string]bool

	stats ucodeBuildStats
}

func (h *HandlerV1) agentsForProvider(provider config.AIProvider) config.AIAgents {
	switch provider {
	case config.AIProviderGemini:
		return h.baseConf.GeminiAgents

	case config.AIProviderOpenAI:
		return h.baseConf.OpenAIAgents

	default:
		return h.baseConf.Agents
	}
}

func (h *HandlerV1) newUcodeChatSession(service services.ServiceManagerI, resourceEnvId, envId, projectId, companyId, chatId, nodeType string, resourceType int32, provider config.AIProvider) *ucodeChatSession {
	modelID := h.agentsForProvider(provider).Planner.Model

	return &ucodeChatSession{
		h:             h,
		service:       service,
		emit:          noopEmitter{},
		model:         h.newUcodeChatModel(modelID),
		modelID:       modelID,
		provider:      provider,
		resourceEnvId: resourceEnvId,
		envId:         envId,
		projectId:     projectId,
		companyId:     companyId,
		chatId:        chatId,
		nodeType:      nodeType,
		resourceType:  resourceType,
		tableIDs:      make(map[string]string),
		fieldSets:     make(map[string]map[string]bool),
	}
}

func (s *ucodeChatSession) run(ctx context.Context, userText string, history []ai.ConversationMessage) (string, int32, error) {
	messages := make([]ai.ConversationMessage, 0, len(history)+1)
	messages = append(messages, history...)
	messages = append(messages, ai.ConversationMessage{Role: "user", Text: userText})

	system := s.systemPrompt
	if system == "" {
		system = chat_prompts.UcodeBuilderSystemPrompt()
	}
	tools := ucodeToolDefs()

	var (
		totalTokens int32
		finalText   string
		lastText    string
	)

	for step := 0; step < ucodeBuilderMaxSteps; step++ {
		result, err := s.model.Complete(ctx, ai.CompletionRequest{
			Model:     s.modelID,
			MaxTokens: ucodeBuilderMaxTokens,
			System:    system,
			Messages:  messages,
			Tools:     tools,
			Timeout:   ucodeStepTimeout,
		})
		if result != nil {
			totalTokens += int32(result.Usage.InputTokens + result.Usage.OutputTokens)
			s.recordUsage(result.Usage, fmt.Sprintf("u-code builder step %d", step+1))
		}
		if err != nil {
			if finalText == "" {
				finalText = lastText
			}
			return finalText, totalTokens, err
		}

		if result.Text != "" {
			lastText = result.Text
		}

		messages = append(messages, ai.ConversationMessage{
			Role:      "assistant",
			Text:      result.Text,
			ToolCalls: result.ToolCalls,
		})

		if len(result.ToolCalls) == 0 {
			finalText = result.Text
			break
		}

		if narration := strings.TrimSpace(result.Text); narration != "" {
			s.emit.Emit(SSEEvent{Type: EvProgress, Icon: IconBrain, Message: truncateString(narration, 280)})
		}

		toolResults := make([]ai.ToolResult, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
			content, isErr := s.executeTool(ctx, call)
			if isErr {
				s.emitToolFailure(call.Name, content)
			}
			toolResults = append(toolResults, ai.ToolResult{
				ToolCallID: call.ID,
				Content:    content,
				IsError:    isErr,
			})
		}
		messages = append(messages, ai.ConversationMessage{Role: "user", ToolResults: toolResults})
	}

	if finalText == "" {
		finalText = lastText
	}
	if strings.TrimSpace(finalText) == "" {
		finalText = "Готово."
	}
	return finalText, totalTokens, nil
}

// ucodeToolMeta maps a tool name to its build action, display icon and a Russian
// noun used in progress messages.
func ucodeToolMeta(toolName string) (action, icon, noun string) {
	switch toolName {
	case toolGetSchema:
		return StepActionSchema, IconScanSearch, "схему"
	case toolCreateTable:
		return StepActionTable, IconDatabase, "таблицу"
	case toolCreateLoginTable:
		return StepActionLogin, IconShield, "таблицу входа"
	case toolCreateField:
		return StepActionField, IconColumns, "поле"
	case toolCreateRelation:
		return StepActionRelation, IconLink, "связь"
	case toolCreateMenu:
		return StepActionMenu, IconFolder, "раздел меню"
	case toolInsertItems:
		return StepActionItems, IconPlusCircle, "записи"
	case toolCountItems, toolListItems, toolAggregateItems:
		return StepActionData, IconScanSearch, "данные"
	default:
		return toolName, IconAlertTriangle, toolName
	}
}

// emitToolFailure reports a single failed tool call as a non-terminal warning so
// the developer sees exactly which step failed and why, while the AI keeps going.
func (s *ucodeChatSession) emitToolFailure(toolName, content string) {
	action, _, noun := ucodeToolMeta(toolName)
	reason := strings.TrimSpace(strings.TrimPrefix(content, "error: "))
	if reason == content { // content wasn't a plain "error: ..." string
		reason = ""
	}
	s.emit.Emit(SSEEvent{
		Type:    EvWarning,
		Icon:    IconAlertTriangle,
		Message: fmt.Sprintf("Не удалось обработать %s", noun),
		Value:   truncateString(reason, 200),
		Data:    UcodeStepData{Action: action, Status: StepStatusFailed, Reason: reason},
	})
}

func (s *ucodeChatSession) saveMessage(ctx context.Context, role, content string, tokensUsed int32) (*pbo.Message, error) {
	return s.service.GoObjectBuilderService().AiChat().CreateMessage(ctx, &pbo.CreateMessageRequest{
		ResourceEnvId: s.resourceEnvId,
		ChatId:        s.chatId,
		Role:          role,
		Content:       content,
		TokensUsed:    tokensUsed,
	})
}

func (s *ucodeChatSession) recordUsage(usage models.LLMUsage, description string) {
	if usage.InputTokens == 0 && usage.OutputTokens == 0 {
		return
	}
	go func() {
		_, err := s.service.CompanyService().Billing().RecordAiTokenUsage(
			context.Background(),
			&pbcs.RecordAiTokenUsageRequest{
				ProjectId:    s.projectId,
				CompanyId:    s.companyId,
				InputTokens:  int32(usage.InputTokens),
				OutputTokens: int32(usage.OutputTokens),
				Model:        s.modelID,
				Description:  description,
				Product:      config.PRODUCT_TYPE_UCODE,
			},
		)
		if err != nil {
			log.Printf("[ucode] record token usage: %v", err)
		}
	}()
}

func (s *ucodeChatSession) history(ctx context.Context) ([]ai.ConversationMessage, error) {
	resp, err := s.service.GoObjectBuilderService().AiChat().GetMessages(ctx, &pbo.GetMessagesRequest{
		ResourceEnvId: s.resourceEnvId,
		ChatId:        s.chatId,
	})
	if err != nil {
		return nil, fmt.Errorf("get chat history: %w", err)
	}

	msgs := resp.GetMessages()
	if len(msgs) > ucodeHistoryWindow {
		msgs = msgs[len(msgs)-ucodeHistoryWindow:]
	}

	out := make([]ai.ConversationMessage, 0, len(msgs))
	for _, m := range msgs {
		role := m.GetRole()
		if role != "user" && role != "assistant" {
			continue
		}
		out = append(out, ai.ConversationMessage{Role: role, Text: m.GetContent()})
	}
	return out, nil
}

// ==================== HTTP handler ====================

func (h *HandlerV1) CreateUcodeChatMessage(c *gin.Context) {
	var (
		userMessage models.NewMessageReq
		chatId      = c.Param("chat-id")
		ctx         = context.Background()
		streaming   = c.Query("stream") == "true"
	)

	if err := c.ShouldBindJSON(&userMessage); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid request body: "+err.Error())
		return
	}
	if strings.TrimSpace(userMessage.Content) == "" {
		h.HandleResponse(c, status_http.BadRequest, "content is required")
		return
	}

	service, resource, err := h.resolveAiChatService(c)
	if err != nil {
		return
	}
	resourceEnvId := resource.GetResourceEnvironmentId()

	var (
		environmentId = c.GetString("environment_id")
		projectId     = c.GetString("project_id")

		companyId string
	)

	if proj, projErr := h.companyServices.Project().GetById(ctx, &pbcs.GetProjectByIdRequest{ProjectId: projectId}); projErr == nil {
		companyId = proj.GetCompanyId()
	}

	chat, err := service.GoObjectBuilderService().AiChat().GetChat(ctx, &pbo.ChatPrimaryKey{
		ResourceEnvId: resourceEnvId,
		Id:            chatId,
	})
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to get chat: %v", err))
		return
	}

	session := h.newUcodeChatSession(service, resourceEnvId, environmentId, projectId, companyId, chatId, resource.GetNodeType(), int32(resource.GetResourceType()), config.ParseAIProvider(chat.GetModel()))

	history, err := session.history(ctx)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to fetch history: %v", err))
		return
	}
	isFirstMessage := len(history) == 0

	if _, err = session.saveMessage(ctx, "user", userMessage.Content, 0); err != nil {
		chatErr := newSaveMessageError(err)
		if streaming {
			h.sseError(c, chatErr)
			return
		}
		h.HandleResponse(c, status_http.GRPCError, errorResponseBody(chatErr))
		return
	}

	if streaming {
		h.runUcodeStreaming(c, session, userMessage.Content, history, isFirstMessage)
		return
	}

	finalText, totalTokens, runErr := session.run(ctx, userMessage.Content, history)
	if runErr != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("ucode build failed: %v", runErr))
		return
	}

	message, err := session.saveMessage(ctx, "assistant", finalText, totalTokens)
	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("failed to save ai message: %v", err))
		return
	}

	session.maybeTitleChat(ctx, isFirstMessage, userMessage.Content, finalText)

	h.HandleResponse(c, status_http.Created, map[string]any{
		"message": ucodeEnrichedMessage(message, finalText),
		"summary": session.stats,
	})
}

// runUcodeStreaming runs the build in a detached background goroutine and streams
func (h *HandlerV1) runUcodeStreaming(c *gin.Context, session *ucodeChatSession, userText string, history []ai.ConversationMessage, isFirstMessage bool) {
	eventCh := make(chan SSEEvent, 64)
	session.emit = &channelEmitter{ch: eventCh}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")
	c.Status(200)

	startTime := time.Now()

	go func() {
		defer close(eventCh)

		runCtx, cancel := context.WithTimeout(context.Background(), ucodeRunTotalTimeout)
		defer cancel()

		session.emit.Emit(SSEEvent{
			Type:    EvProvider,
			Icon:    IconCPU,
			Message: "Строю объекты u-code",
			Data:    ProviderEventData{Provider: string(session.provider), CoderModel: session.modelID},
		})
		session.emit.Emit(SSEEvent{Type: EvProgress, Icon: IconSparkles, Message: "Анализирую запрос...", Percent: 1})

		finalText, totalTokens, runErr := session.run(runCtx, userText, history)
		if runErr != nil {
			session.emit.Emit(SSEEvent{Type: EvError, Icon: IconAlertCircle, Message: fmt.Sprintf("Не удалось построить объекты: %v", runErr)})
			return
		}

		message, saveErr := session.saveMessage(runCtx, "assistant", finalText, totalTokens)
		if saveErr != nil {
			log.Printf("[ucode] failed to save assistant message: %v", saveErr)
		}

		session.maybeTitleChat(runCtx, isFirstMessage, userText, finalText)

		session.emit.Emit(SSEEvent{
			Type:    EvDone,
			Icon:    IconCheckCircle,
			Percent: 100,
			Message: finalText,
			Data: map[string]any{
				"message":      ucodeEnrichedMessage(message, finalText),
				"summary":      session.stats,
				"duration_sec": int(time.Since(startTime).Seconds()),
			},
		})
	}()

	drainSSE(c, eventCh)
}

func (s *ucodeChatSession) maybeTitleChat(ctx context.Context, isFirstMessage bool, userText, finalText string) {
	if !isFirstMessage {
		return
	}
	_, _ = s.service.GoObjectBuilderService().AiChat().UpdateChat(ctx, &pbo.UpdateChatRequest{
		ResourceEnvId: s.resourceEnvId,
		Id:            s.chatId,
		Title:         truncateString(userText, 100),
		Description:   truncateString(finalText, 255),
		Type:          chatTypeUcode,
	})
}

// ucodeEnrichedMessage builds the HTTP message representation, falling back to the
func ucodeEnrichedMessage(message *pbo.Message, finalText string) models.EnrichedMessage {
	if message == nil {
		return models.EnrichedMessage{Role: "assistant", Content: finalText}
	}
	return models.EnrichedMessage{
		ID:         message.GetId(),
		ChatID:     message.GetChatId(),
		Role:       message.GetRole(),
		Content:    message.GetContent(),
		Images:     message.GetImages(),
		HasFiles:   message.GetHasFiles(),
		TokensUsed: message.GetTokensUsed(),
		CreatedAt:  message.GetCreatedAt(),
	}
}

// drainSSE relays buffered events to the client until the pipeline closes the
func drainSSE(c *gin.Context, eventCh <-chan SSEEvent) {
	clientGone := c.Request.Context().Done()
	keepalive := time.NewTicker(15 * time.Second)
	defer keepalive.Stop()

	var lastSent time.Time
	const minInterval = 400 * time.Millisecond

	for {
		select {
		case ev, ok := <-eventCh:
			if !ok {
				return
			}
			if !lastSent.IsZero() {
				if gap := minInterval - time.Since(lastSent); gap > 0 {
					time.Sleep(gap)
				}
			}
			writeSSEEvent(c.Writer, ev)
			c.Writer.Flush()
			lastSent = time.Now()
		case <-keepalive.C:
			fmt.Fprintf(c.Writer, ": keepalive\n\n")
			c.Writer.Flush()
		case <-clientGone:
			log.Printf("[ucode][SSE] client disconnected — draining remaining events")
			for range eventCh {
			}
			return
		}
	}
}

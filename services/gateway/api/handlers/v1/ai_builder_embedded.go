package v1

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/config"
	pbcs "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
)

// embeddedBuilderMessage is one prior turn replayed by the frontend for context.
type embeddedBuilderMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type embeddedBuilderRequest struct {
	Content string                   `json:"content"`
	History []embeddedBuilderMessage `json:"history"`
}

// CreateEmbeddedBuilderMessage is the end-user surface for the builder assistant
// embedded in a generated admin panel. It runs the u-code builder tool-loop against
// the calling app's own backend and streams the build steps when ?stream=true. The
// conversation is stateless: the frontend replays recent turns as history.
func (h *HandlerV1) CreateEmbeddedBuilderMessage(c *gin.Context) {
	var (
		req       embeddedBuilderRequest
		ctx       = context.Background()
		streaming = c.Query("stream") == "true"
	)

	if err := c.ShouldBindJSON(&req); err != nil {
		h.HandleResponse(c, status_http.BadRequest, "invalid request body: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Content) == "" {
		h.HandleResponse(c, status_http.BadRequest, "content is required")
		return
	}

	// Resolves the app's PostgreSQL backend and auth context from the request's
	// project/environment (set by the V2 API-key middleware); writes its own error.
	service, resource, err := h.resolveAiChatService(c)
	if err != nil {
		return
	}

	var (
		resourceEnvId = resource.GetResourceEnvironmentId()
		environmentId = c.GetString("environment_id")
		projectId     = c.GetString("project_id")

		companyId string
	)
	if proj, projErr := h.companyServices.Project().GetById(ctx, &pbcs.GetProjectByIdRequest{ProjectId: projectId}); projErr == nil {
		companyId = proj.GetCompanyId()
	}

	session := h.newUcodeChatSession(
		service, resourceEnvId, environmentId, projectId, companyId, "",
		resource.GetNodeType(), int32(resource.GetResourceType()), config.AIProviderClaude,
	)
	// End-user replies render in a narrow chat bubble — use the widget-formatted prompt.
	session.systemPrompt = chat_prompts.EmbeddedBuilderSystemPrompt()

	history := embeddedHistoryToConversation(req.History)

	if streaming {
		h.runEmbeddedBuilderStreaming(c, session, req.Content, history)
		return
	}

	reply, _, runErr := session.run(ctx, req.Content, history)
	if runErr != nil {
		h.HandleResponse(c, status_http.GRPCError, fmt.Sprintf("builder assistant failed: %v", runErr))
		return
	}

	h.HandleResponse(c, status_http.OK, map[string]any{"reply": reply, "summary": session.stats})
}

// runEmbeddedBuilderStreaming runs the builder loop in a detached goroutine so it
// survives a dropped connection, and streams its progress as SSE. It persists
// nothing; the terminal event carries the reply and build summary.
func (h *HandlerV1) runEmbeddedBuilderStreaming(c *gin.Context, session *ucodeChatSession, userText string, history []ai.ConversationMessage) {
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

		reply, _, runErr := session.run(runCtx, userText, history)
		if runErr != nil {
			session.emit.Emit(SSEEvent{Type: EvError, Icon: IconAlertCircle, Message: fmt.Sprintf("Не удалось выполнить запрос: %v", runErr)})
			return
		}

		session.emit.Emit(SSEEvent{
			Type:    EvDone,
			Icon:    IconCheckCircle,
			Percent: 100,
			Message: reply,
			Data: map[string]any{
				"reply":        reply,
				"summary":      session.stats,
				"duration_sec": int(time.Since(startTime).Seconds()),
			},
		})
	}()

	drainSSE(c, eventCh)
}

// embeddedHistoryToConversation keeps well-formed user/assistant turns and caps the
// window to what the builder loop replays for context.
func embeddedHistoryToConversation(items []embeddedBuilderMessage) []ai.ConversationMessage {
	if len(items) > ucodeHistoryWindow {
		items = items[len(items)-ucodeHistoryWindow:]
	}
	out := make([]ai.ConversationMessage, 0, len(items))
	for _, m := range items {
		role := strings.ToLower(strings.TrimSpace(m.Role))
		if role != "user" && role != "assistant" {
			continue
		}
		content := strings.TrimSpace(m.Content)
		if content == "" {
			continue
		}
		out = append(out, ai.ConversationMessage{Role: role, Text: content})
	}
	return out
}

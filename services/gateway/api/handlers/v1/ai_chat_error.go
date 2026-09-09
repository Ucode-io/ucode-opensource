package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const (
	ErrCodeTokenLimit       = "TOKEN_LIMIT_EXCEEDED"
	ErrCodeMaxTokens        = "AI_MAX_TOKENS"
	ErrCodeTimeout          = "TIMEOUT"
	ErrCodeRouterFailed     = "ROUTER_FAILED"
	ErrCodeArchitectFailed  = "ARCHITECT_FAILED"
	ErrCodeProvisioning     = "PROVISIONING_FAILED"
	ErrCodeManifestFailed   = "MANIFEST_FAILED"
	ErrCodeCodegenFailed    = "CODEGEN_FAILED"
	ErrCodePublishFailed    = "PUBLISH_FAILED"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
	ErrCodeInternal         = "INTERNAL_ERROR"
)

const (
	PhaseRouting      = "routing"
	PhaseArchitect    = "architect"
	PhaseProvisioning = "provisioning"
	PhaseManifest     = "manifest"
	PhaseCodegen      = "codegen"
	PhasePublish      = "publish"
	PhaseValidation   = "validation"
	PhaseUnknown      = "unknown"
)

func (p *ChatProcessor) persistPipelineError(ctx context.Context, err error) models.AiChatError {
	chatErr := classifyPipelineError(err)
	body := buildErrorChatBody(chatErr)
	if _, saveErr := p.saveMessage(ctx, "assistant", body, nil); saveErr != nil {
		log.Printf("[ai-chat] persist error message (chat=%s, code=%s): %v", p.chatId, chatErr.Code, saveErr)
	}
	return chatErr
}

func classifyPipelineError(err error) models.AiChatError {
	if err == nil {
		return models.AiChatError{}
	}

	var tokenErr *TokenLimitError
	if errors.As(err, &tokenErr) {
		return models.AiChatError{
			Code:       ErrCodeTokenLimit,
			Phase:      PhaseRouting,
			Message:    fmt.Sprintf("Достигнут лимит токенов (%s): использовано %d из %d.", tokenErr.Period, tokenErr.Used, tokenErr.Limit),
			Details:    err.Error(),
			Retryable:  false,
			UserAction: "Дождитесь сброса лимита или повысьте тарифный план.",
		}
	}

	if errors.Is(err, ai.ErrMaxTokens) {
		return models.AiChatError{
			Code:       ErrCodeMaxTokens,
			Phase:      detectPhase(err),
			Message:    "AI-модель не уложилась в лимит ответа.",
			Details:    err.Error(),
			Retryable:  true,
			UserAction: "Попробуйте сократить требования или разбить проект на несколько меньших.",
		}
	}

	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return models.AiChatError{
			Code:       ErrCodeTimeout,
			Phase:      detectPhase(err),
			Message:    "Генерация заняла слишком много времени и была отменена.",
			Details:    err.Error(),
			Retryable:  true,
			UserAction: "Попробуйте ещё раз. Если повторяется — упростите запрос.",
		}
	}

	phase := detectPhase(err)
	details := err.Error()

	switch phase {
	case PhaseArchitect:
		return models.AiChatError{
			Code: ErrCodeArchitectFailed, Phase: phase,
			Message:    "Не удалось спроектировать архитектуру проекта.",
			Details:    details,
			Retryable:  true,
			UserAction: "Попробуйте ещё раз или уточните, что именно нужно построить.",
		}
	case PhaseProvisioning:
		return models.AiChatError{
			Code: ErrCodeProvisioning, Phase: phase,
			Message:    "Не удалось создать ресурсы проекта в инфраструктуре.",
			Details:    details,
			Retryable:  true,
			UserAction: "Попробуйте через минуту. Если ошибка повторится — обратитесь в поддержку.",
		}
	case PhaseManifest:
		return models.AiChatError{
			Code: ErrCodeManifestFailed, Phase: phase,
			Message:    "Не удалось составить план файлов проекта.",
			Details:    details,
			Retryable:  true,
			UserAction: "Попробуйте ещё раз — повторный запуск обычно решает.",
		}
	case PhaseCodegen:
		return models.AiChatError{
			Code: ErrCodeCodegenFailed, Phase: phase,
			Message:    "Не удалось сгенерировать код проекта.",
			Details:    details,
			Retryable:  true,
			UserAction: "Попробуйте ещё раз. Если повторяется — упростите требования.",
		}
	case PhasePublish:
		return models.AiChatError{
			Code: ErrCodePublishFailed, Phase: phase,
			Message:    "Не удалось опубликовать проект для предпросмотра.",
			Details:    details,
			Retryable:  true,
			UserAction: "Попробуйте ещё раз через минуту.",
		}
	case PhaseRouting:
		return models.AiChatError{
			Code: ErrCodeRouterFailed, Phase: phase,
			Message:    "Не удалось определить намерение запроса.",
			Details:    details,
			Retryable:  true,
			UserAction: "Попробуйте переформулировать запрос проще.",
		}
	}

	return models.AiChatError{
		Code: ErrCodeInternal, Phase: PhaseUnknown,
		Message:    "Произошла внутренняя ошибка при обработке запроса.",
		Details:    details,
		Retryable:  true,
		UserAction: "Попробуйте ещё раз. Если ошибка повторится — обратитесь в поддержку.",
	}
}

func detectPhase(err error) string {
	if err == nil {
		return PhaseUnknown
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "architect"):
		return PhaseArchitect
	case strings.Contains(msg, "backend provisioning"), strings.Contains(msg, "create backend project"),
		strings.Contains(msg, "create environment"), strings.Contains(msg, "mcp project"):
		return PhaseProvisioning
	case strings.Contains(msg, "manifest"):
		return PhaseManifest
	case strings.Contains(msg, "microfrontend publish"), strings.Contains(msg, "publish "):
		return PhasePublish
	case strings.Contains(msg, "chunked"), strings.Contains(msg, "generate code"),
		strings.Contains(msg, "feature chunks failed"), strings.Contains(msg, "foundation failed"):
		return PhaseCodegen
	case strings.Contains(msg, "router"):
		return PhaseRouting
	}
	return PhaseUnknown
}

func buildErrorChatBody(e models.AiChatError) string {
	raw, _ := json.Marshal(e)
	headline := strings.TrimSpace(e.Message)
	if headline == "" {
		headline = "Ошибка при обработке запроса."
	}
	return fmt.Sprintf("%s %s\n%s", ai.MarkerError, headline, string(raw))
}

func newSaveMessageError(err error) models.AiChatError {
	return models.AiChatError{
		Code:       ErrCodeInternal,
		Phase:      PhaseUnknown,
		Message:    "Не удалось сохранить сообщение в чате.",
		Details:    err.Error(),
		Retryable:  true,
		UserAction: "Попробуйте ещё раз через минуту.",
	}
}

func errorResponseBody(e models.AiChatError) map[string]any {
	return map[string]any{
		"error":   e,
		"message": e.Message,
	}
}

func errorEventData(e models.AiChatError, extras map[string]any) map[string]any {
	data := map[string]any{"error": e}
	for k, v := range extras {
		data[k] = v
	}
	return data
}

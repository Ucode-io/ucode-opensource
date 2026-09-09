package v1

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"
	"unicode"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

// Clone questionnaire: for multi-page website clone prompts the pipeline asks
// two deterministic questions before building — which of the reference site's
// REAL pages to clone (options come from its captured navigation) and what
// functionality must actually work (auth, database content, cart, forms,
// search). Deterministic Go code builds the questions so the options are always
// real pages and the answer labels can be keyword-matched later
// (see functionalCloneStems / staticCloneStems in reference_capture.go).

// shouldAskCloneQuestions gates the clone questionnaire: only on the very
// first message of a chat (no re-asking loops on follow-ups) and never for an
// explicit single-page landing request, which keeps today's instant flow.
func shouldAskCloneQuestions(prompt string, history []models.ChatMessage) bool {
	return len(history) == 0 && !promptWantsSingleLanding(prompt)
}

// buildCloneQuestionnaire quick-fetches the reference site's HTML (structure
// only — the full capture happens later at build time) and turns its
// navigation into the clone questionnaire. Any failure returns no questions so
// the caller falls back to building directly, exactly like before.
func (p *ChatProcessor) buildCloneQuestionnaire(ctx context.Context, prompt string) (string, []models.AiQuestion) {
	rawURLs := reduceSameHostReferenceURLs(extractReferenceURLs(prompt))
	if len(rawURLs) != 1 {
		return "", nil
	}
	targetURL, err := normalizeReferenceURL(rawURLs[0])
	if err != nil {
		return "", nil
	}

	p.emitter().Emit(SSEEvent{
		Type:    EvProgress,
		Icon:    "scan-eye",
		Percent: 4,
		Message: "Изучаю референс-сайт...",
		Value:   targetURL,
	})

	fetchCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	ref, err := fetchReferenceSiteHTMLForPrompt(fetchCtx, targetURL)
	if err != nil {
		log.Printf("[reference] questionnaire fetch failed url=%s: %v — building directly", targetURL, err)
		return "", nil
	}
	return buildCloneQuestions(ref, prompt, targetURL)
}

// buildCloneQuestions assembles the questionnaire from captured evidence.
// Titles/labels are Russian when the prompt is written in Cyrillic, English
// otherwise; option IDs stay stable English kebab-case either way.
func buildCloneQuestions(ref *models.ReferenceSiteContext, prompt, targetURL string) (string, []models.AiQuestion) {
	ru := containsCyrillic(prompt)

	var questions []models.AiQuestion

	if pageQuestion, ok := buildClonePagesQuestion(ref, ru); ok {
		questions = append(questions, pageQuestion)
	}
	questions = append(questions, buildCloneFunctionalityQuestion(ru))

	intro := fmt.Sprintf("I inspected %s and found its pages. Choose which pages to clone and what should actually work — then I'll build the site.", targetURL)
	if ru {
		intro = fmt.Sprintf("Я изучил %s и нашёл его страницы. Выберите, какие страницы клонировать и какая функциональность должна работать — и я соберу сайт.", targetURL)
	}
	return intro, questions
}

func buildClonePagesQuestion(ref *models.ReferenceSiteContext, ru bool) (models.AiQuestion, bool) {
	title := "Which pages of the site should be cloned?"
	homeLabel := "Home page"
	if ru {
		title = "Какие страницы сайта нужно клонировать?"
		homeLabel = "Главная страница"
	}

	options := []models.QuestionOption{{ID: "home", Label: homeLabel}}
	seen := map[string]bool{"home": true}

	if ref != nil {
		for _, link := range ref.NavLinks {
			if len(options) >= 9 {
				break
			}
			label := strings.TrimSpace(link.Label)
			id := slugify(label)
			if label == "" || id == "" || id == "ai-project" || seen[id] {
				continue
			}
			seen[id] = true
			options = append(options, models.QuestionOption{ID: id, Label: label})
		}
	}

	// Nav extraction found nothing beyond Home — a pages question with one
	// option is noise; the build will still crawl whatever it can find.
	if len(options) < 2 {
		return models.AiQuestion{}, false
	}

	return models.AiQuestion{
		ID:      "clone-pages",
		Title:   title,
		Type:    "multi",
		Options: options,
	}, true
}

// buildCloneFunctionalityQuestion offers the working-product choices. The
// labels double as deterministic detection keywords: the frontend echoes
// selected labels back in the answer text, and functionalCloneStems /
// staticCloneStems in reference_capture.go match on them.
func buildCloneFunctionalityQuestion(ru bool) models.AiQuestion {
	title := "What should actually work in the clone?"
	options := []models.QuestionOption{
		{ID: "static-copy", Label: "Static design copy only (no backend)"},
		{ID: "auth-login", Label: "Login / Registration (working auth)"},
		{ID: "database-content", Label: "Content from database (real data)"},
		{ID: "cart-checkout", Label: "Cart & checkout flow"},
		{ID: "contact-forms", Label: "Working forms (contact / lead)"},
		{ID: "search-filters", Label: "Search & filters"},
	}
	if ru {
		title = "Что должно реально работать в клоне?"
		options = []models.QuestionOption{
			{ID: "static-copy", Label: "Только статичная копия дизайна (без бэкенда)"},
			{ID: "auth-login", Label: "Логин / Регистрация (рабочая авторизация)"},
			{ID: "database-content", Label: "Контент из базы данных (реальные данные)"},
			{ID: "cart-checkout", Label: "Корзина и оформление заказа"},
			{ID: "contact-forms", Label: "Рабочие формы (контакты / заявки)"},
			{ID: "search-filters", Label: "Поиск и фильтры"},
		}
	}
	return models.AiQuestion{
		ID:      "clone-functionality",
		Title:   title,
		Type:    "multi",
		Options: options,
	}
}

func containsCyrillic(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Cyrillic, r) {
			return true
		}
	}
	return false
}

package v1

import (
	"context"
	"strings"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func TestShouldAskCloneQuestions(t *testing.T) {
	tests := []struct {
		name    string
		prompt  string
		history []models.ChatMessage
		want    bool
	}{
		{
			name:   "website clone on first message asks",
			prompt: "Make website page according to this uzum.uz website. Make 1 to 1 logic and design.",
			want:   true,
		},
		{
			name:   "explicit landing clone skips questions",
			prompt: "Make landing page according to this thesecrettrading.de website. Make 1 to 1 logic and design.",
			want:   false,
		},
		{
			name:   "russian landing clone skips questions",
			prompt: "Сделай лендинг 1 в 1 как на сайте example.com",
			want:   false,
		},
		{
			name:   "existing conversation skips questions",
			prompt: "Clone uzum.uz 1:1",
			history: []models.ChatMessage{
				{Role: "user", Content: []models.ContentBlock{{Type: "text", Text: "hello"}}},
			},
			want: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldAskCloneQuestions(tc.prompt, tc.history); got != tc.want {
				t.Fatalf("shouldAskCloneQuestions() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestBuildCloneQuestionsFromNavigation(t *testing.T) {
	ref := &models.ReferenceSiteContext{
		URL: "https://uzum.uz",
		NavLinks: []models.ReferenceNavLink{
			{Label: "Catalog", URL: "https://uzum.uz/catalog"},
			{Label: "Cart", URL: "https://uzum.uz/cart"},
			{Label: "Contacts", URL: "https://uzum.uz/contacts"},
		},
	}

	intro, questions := buildCloneQuestions(ref, "Make website according to uzum.uz 1 to 1", "https://uzum.uz")

	if intro == "" || !strings.Contains(intro, "https://uzum.uz") {
		t.Fatalf("expected intro naming the URL, got %q", intro)
	}
	if len(questions) != 2 {
		t.Fatalf("expected pages + functionality questions, got %d", len(questions))
	}

	pages := questions[0]
	if pages.ID != "clone-pages" || pages.Type != "multi" {
		t.Fatalf("unexpected pages question: %+v", pages)
	}
	if pages.Options[0].ID != "home" {
		t.Fatalf("expected Home first, got %+v", pages.Options[0])
	}
	if len(pages.Options) != 4 {
		t.Fatalf("expected home + 3 nav options, got %#v", pages.Options)
	}
	if pages.Options[1].ID != "catalog" || pages.Options[1].Label != "Catalog" {
		t.Fatalf("expected slugified nav option, got %+v", pages.Options[1])
	}

	functionality := questions[1]
	if functionality.ID != "clone-functionality" {
		t.Fatalf("unexpected functionality question id: %s", functionality.ID)
	}
	hasStatic := false
	for _, opt := range functionality.Options {
		if opt.ID == "static-copy" {
			hasStatic = true
		}
	}
	if !hasStatic {
		t.Fatal("expected static-copy option")
	}
}

func TestBuildCloneQuestionsRussianAndNoNav(t *testing.T) {
	intro, questions := buildCloneQuestions(&models.ReferenceSiteContext{}, "Скопируй сайт uzum.uz один в один", "https://uzum.uz")

	if !strings.Contains(intro, "Я изучил") {
		t.Fatalf("expected Russian intro, got %q", intro)
	}
	// No nav links → the single-option pages question is dropped.
	if len(questions) != 1 || questions[0].ID != "clone-functionality" {
		t.Fatalf("expected only functionality question, got %#v", questions)
	}
	if questions[0].Options[0].Label != "Только статичная копия дизайна (без бэкенда)" {
		t.Fatalf("expected Russian labels, got %+v", questions[0].Options[0])
	}
}

func TestCrawlReferenceSubpagesSameHostOnly(t *testing.T) {
	oldFetch := fetchReferencePageForCrawl
	defer func() { fetchReferencePageForCrawl = oldFetch }()

	var fetched []string
	fetchReferencePageForCrawl = func(_ context.Context, pageURL string) (*models.ReferenceSiteContext, error) {
		fetched = append(fetched, pageURL)
		return &models.ReferenceSiteContext{
			Title:    "Page " + pageURL,
			Sections: []models.ReferenceSection{{Heading: "H " + pageURL}},
		}, nil
	}

	ref := &models.ReferenceSiteContext{
		URL:      "https://uzum.uz",
		FinalURL: "https://uzum.uz/",
		NavLinks: []models.ReferenceNavLink{
			{Label: "Home", URL: "https://uzum.uz/"},                // home path — skipped
			{Label: "Catalog", URL: "https://uzum.uz/catalog"},      // crawled
			{Label: "Cart", URL: "https://www.uzum.uz/cart"},        // www same host — crawled
			{Label: "External", URL: "https://facebook.com/uzum"},   // foreign host — skipped
			{Label: "Catalog dup", URL: "https://uzum.uz/catalog/"}, // duplicate path — skipped
			{Label: "Contacts", URL: "https://uzum.uz/contacts"},    // crawled
		},
	}

	crawlReferenceSubpages(context.Background(), ref, "clone uzum.uz")

	if len(ref.Pages) != 3 {
		t.Fatalf("expected 3 crawled subpages, got %d (%#v)", len(ref.Pages), fetched)
	}
	for _, page := range ref.Pages {
		if strings.Contains(page.URL, "facebook.com") {
			t.Fatalf("foreign host crawled: %s", page.URL)
		}
		if page.Title == "" || len(page.Sections) == 0 {
			t.Fatalf("expected page evidence, got %+v", page)
		}
	}
}

func TestCrawlReferenceSubpagesPrioritizesSelectedPages(t *testing.T) {
	oldFetch := fetchReferencePageForCrawl
	defer func() { fetchReferencePageForCrawl = oldFetch }()

	fetchReferencePageForCrawl = func(_ context.Context, pageURL string) (*models.ReferenceSiteContext, error) {
		return &models.ReferenceSiteContext{Title: pageURL, Sections: []models.ReferenceSection{{Heading: "h"}}}, nil
	}

	ref := &models.ReferenceSiteContext{
		URL: "https://shop.example",
		NavLinks: []models.ReferenceNavLink{
			{Label: "About", URL: "https://shop.example/about"},
			{Label: "Blog", URL: "https://shop.example/blog"},
			{Label: "Delivery", URL: "https://shop.example/delivery"},
			{Label: "Brands", URL: "https://shop.example/brands"},
			{Label: "Careers", URL: "https://shop.example/careers"},
			{Label: "Cart", URL: "https://shop.example/cart"},
		},
	}

	// Answers select Cart — it must be crawled despite being past the cap in nav order.
	crawlReferenceSubpages(context.Background(), ref, "Question: Which pages?\nUser answer: Home, Cart")

	if len(ref.Pages) != maxReferenceSubpages {
		t.Fatalf("expected cap of %d pages, got %d", maxReferenceSubpages, len(ref.Pages))
	}
	if ref.Pages[0].Label != "Cart" {
		t.Fatalf("expected selected page crawled first, got %+v", ref.Pages[0])
	}
}

func TestShouldForceStaticReferenceCloneWithQuestionnaireAnswers(t *testing.T) {
	plan := &models.ArchitectPlan{ProjectType: "web"}

	tests := []struct {
		name   string
		prompt string
		want   bool
	}{
		{
			name:   "cart answer keeps backend",
			prompt: "Clone uzum.uz\n\nQuestion: What should actually work?\nUser answer: Cart & checkout flow",
			want:   false,
		},
		{
			name:   "russian auth answer keeps backend",
			prompt: "Клон uzum.uz\n\nQuestion: Что должно работать?\nUser answer: Логин / Регистрация (рабочая авторизация)",
			want:   false,
		},
		{
			name:   "static copy answer forces static",
			prompt: "Clone example.com\n\nUser answer: Static design copy only (no backend)",
			want:   true,
		},
		{
			name:   "contradictory answers prefer functional",
			prompt: "Clone example.com\n\nUser answer: Static design copy only (no backend), Cart & checkout flow",
			want:   false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := shouldForceStaticReferenceClone(tc.prompt, plan); got != tc.want {
				t.Fatalf("shouldForceStaticReferenceClone() = %v, want %v", got, tc.want)
			}
		})
	}
}

package v1

import (
	"context"
	"log"
	"strings"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const (
	intentAskQuestion     = "ask_question"
	intentPlanRequest     = "plan_request"
	intentClarify         = "clarify"
	intentProjectQuestion = "project_question"
	intentProjectInspect  = "project_inspect"
	intentCodeChange      = "code_change"
	intentDatabaseQuery   = "database_query"
	intentCreateAgent     = "create_agent"
)

func (p *ChatProcessor) routeAndProcess(ctx context.Context, req models.NewMessageReq, chatHistory []models.ChatMessage) (*models.ParsedClaudeResponse, error) {
	p.initTokenBudget(ctx)
	if err := p.Check(); err != nil {
		return nil, err
	}

	if len(req.Context) > 0 {
		if p.mcpProjectId == "" && p.microFrontendId == "" {
			return &models.ParsedClaudeResponse{
				Description: "No project found. Please create a project first before using visual editing.",
			}, nil
		}
		return p.runVisualEdit(ctx, req.Content, req.Context, chatHistory, req.Images)
	}

	// The frontend sends new_project=false both for "New frontend" in the
	// current project and when a microfrontend is auto-selected on page load,
	// so the clone bypass must not depend on new_project alone.
	isClonePrompt := isReferenceClonePrompt(req.Content)
	if isClonePrompt && p.microFrontendId == "" {
		// Multi-page website clones get a short capture-driven questionnaire
		// first (which real pages, what working functionality). Landing clones
		// and questionnaire failures keep the instant direct-build flow.
		if shouldAskCloneQuestions(req.Content, chatHistory) {
			if intro, questions := p.buildCloneQuestionnaire(ctx, req.Content); len(questions) > 0 {
				log.Printf("[ROUTER] clone questionnaire: asking %d question(s) before build", len(questions))
				return &models.ParsedClaudeResponse{
					Description: intro,
					Questions:   questions,
				}, nil
			}
		}
		log.Printf("[ROUTER] reference clone prompt detected — bypassing questionnaire (new_project=%v)", p.newProject)
		if p.newProject {
			return p.buildNewProject(ctx, req.Content, chatHistory, req.Images, "")
		}
		return p.buildMicrofrontendForCurrentProject(ctx, req.Content, chatHistory, req.Images, "")
	}

	var (
		hasImages = len(req.Images) > 0

		fileGraphJSON   = "{}"
		microFrontFiles []models.GitlabFileChange
		err             error
	)

	if p.microFrontendId != "" && p.microFrontendRepoId != "" {
		log.Printf("[ROUTER] microFrontend edit mode — fetching files for repo_id=%s", p.microFrontendRepoId)
		microFrontFiles, err = p.fetchMicrofrontendFiles(ctx)
		if err != nil {
			log.Printf("[ROUTER] failed to fetch microFrontend files: %v", err)
		} else {
			fileGraphJSON = p.buildMicrofrontendFileGraphJSON(microFrontFiles)
		}
	}

	routeResult, err := p.agent.RouteRequest(ctx, models.RouterInput{
		UserMessage:   req.Content,
		FileGraphJSON: fileGraphJSON,
		HasImages:     hasImages,
		History:       chatHistory,
	})
	if err != nil {
		return nil, err
	}

	log.Printf("[ROUTER] intent=%s next_step=%v files_needed=%d", routeResult.Intent, routeResult.NextStep, len(routeResult.FilesNeeded))

	if routeResult.Intent == intentAskQuestion {
		// A clone prompt already names its source of truth — never answer it
		// with a questionnaire, even in microfrontend edit mode.
		if isClonePrompt {
			log.Printf("[ROUTER] router asked questions for a clone prompt — overriding to code change")
			return p.runCodeChange(ctx, req.Content, fileGraphJSON, chatHistory, req.Images, routeResult.ProjectName, microFrontFiles)
		}
		return &models.ParsedClaudeResponse{
			Description: routeResult.Reply,
			Questions:   routeResult.Questions,
		}, nil
	}

	if routeResult.Intent == intentPlanRequest {
		clarified := routeResult.Clarified
		if clarified == "" {
			clarified = req.Content
		}
		// The router's clarified text is a short rewrite; if the original build
		// request was a reference clone, the URL/clone phrasing may have been
		// dropped — and with it the whole reference-capture pipeline.
		clarified = restoreCloneReference(clarified, chatHistory)
		// Keep the raw questionnaire answers verbatim: the selected option
		// labels drive deterministic static-vs-functional clone detection and
		// page selection, and the router's rewrite may paraphrase them away.
		if clarified != req.Content && !strings.Contains(clarified, req.Content) {
			clarified += "\n\nUser questionnaire answers (verbatim):\n" + req.Content
		}
		return p.runCodeChange(ctx, clarified, fileGraphJSON, chatHistory, req.Images, routeResult.ProjectName, microFrontFiles)
	}

	if !routeResult.NextStep {
		return &models.ParsedClaudeResponse{Description: routeResult.Reply}, nil
	}

	switch routeResult.Intent {

	case intentClarify, intentProjectQuestion:
		return &models.ParsedClaudeResponse{Description: routeResult.Reply}, nil

	case intentProjectInspect:
		if p.microFrontendId != "" {
			return p.runMicrofrontendInspect(ctx, req.Content, routeResult.FilesNeeded, chatHistory, req.Images, microFrontFiles)
		}
		return &models.ParsedClaudeResponse{
			Description: "No project exists yet. Please create a project first by describing what you want to build.",
		}, nil

	case intentCodeChange:
		return p.runCodeChange(ctx, routeResult.Clarified, fileGraphJSON, chatHistory, req.Images, routeResult.ProjectName, microFrontFiles)

	case intentDatabaseQuery:
		clarified := strings.TrimSpace(routeResult.Clarified)
		if clarified == "" {
			clarified = req.Content
			log.Printf("[ROUTER] database_query: clarified was empty, using raw content")
		}
		return p.runDatabaseFlow(ctx, clarified, chatHistory)

	case intentCreateAgent:
		clarified := strings.TrimSpace(routeResult.Clarified)
		if clarified == "" {
			clarified = req.Content
			log.Printf("[ROUTER] create_agent: clarified was empty, using raw content")
		}
		return p.runCreateAgent(ctx, clarified, chatHistory, req.Images)
	}

	return &models.ParsedClaudeResponse{Description: routeResult.Reply}, nil
}

// restoreCloneReference re-attaches the user's original clone request when a
// rewritten prompt no longer triggers reference-clone detection. Only used on
// the plan_request (questionnaire-completion) path, where the build is starting
// fresh from the conversation — never on later edits, so an old clone prompt in
// history cannot hijack unrelated changes.
func restoreCloneReference(clarified string, history []models.ChatMessage) string {
	if isReferenceClonePrompt(clarified) {
		return clarified
	}
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Role != "user" {
			continue
		}
		var text strings.Builder
		for _, block := range history[i].Content {
			if block.Type == "text" {
				text.WriteString(block.Text)
			}
		}
		original := text.String()
		if isReferenceClonePrompt(original) {
			log.Printf("[ROUTER] plan_request lost clone reference — restoring original request from history")
			return clarified + "\n\nOriginal user request (authoritative for the reference website URL and clone intent):\n" + original
		}
	}
	return clarified
}

func (p *ChatProcessor) runCodeChange(ctx context.Context, clarified, fileGraphJSON string, chatHistory []models.ChatMessage, imageURLs []string, projectName string, microFrontFiles []models.GitlabFileChange) (*models.ParsedClaudeResponse, error) {
	if p.microFrontendId != "" {
		return p.runMicrofrontendEdit(ctx, clarified, fileGraphJSON, chatHistory, imageURLs, microFrontFiles)
	}

	if p.newProject {
		log.Printf("[CODE] new_project=true — provisioning new ucode project")
		return p.buildNewProject(ctx, clarified, chatHistory, imageURLs, projectName)
	}

	log.Printf("[CODE] new_project=false — creating microFrontend in current project")
	return p.buildMicrofrontendForCurrentProject(ctx, clarified, chatHistory, imageURLs, projectName)
}

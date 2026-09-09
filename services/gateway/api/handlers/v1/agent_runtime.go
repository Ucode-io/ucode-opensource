package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/anthropic"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/gemini"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/openai"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/doc_generator_service"
	nb "github.com/Ucode-io/ucode-opensource/services/gateway/genproto/new_object_builder_service"
	helperFunc "github.com/Ucode-io/ucode-opensource/services/gateway/pkg/helper"
	"github.com/Ucode-io/ucode-opensource/services/gateway/services"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/spf13/cast"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	agentRunMaxTokens    = 52000
	agentStepTimeout     = 520 * time.Second
	defaultAgentMaxSteps = 12

	// maxAgentTruncationRetries is how many times a single run may recover from a
	maxAgentTruncationRetries = 3

	agentStatusSucceeded = "succeeded"
	agentStatusFailed    = "failed"

	pdfRenderTimeout = 120 * time.Second

	agentFilesFolder = "agent-files"

	agentRunTotalTimeout = 20 * time.Minute

	agentFinalizeTimeout = 100 * time.Second
)

// agentTruncationRecoveryNote is appended to the system prompt after a step hit the
const agentTruncationRecoveryNote = "\n\n## URGENT: your previous reply was cut off — it exceeded the output limit\n" +
	"Do NOT write out any reasoning, calculations, tables, or explanations in your message text. " +
	"Compute everything silently. If you need to produce a document, call the create_pdf tool RIGHT NOW with the final, complete HTML and nothing else. " +
	"If you were performing data changes, call the next tool directly. Keep any natural-language text to a single short sentence."

// agentFile is a document an agent produced during a run (e.g. a generated PDF).
type agentFile struct {
	Name        string `json:"name"`
	URL         string `json:"url"`
	ContentType string `json:"content_type"`
}

func (h *HandlerV1) newChatModel(model string) ai.ChatModel {
	switch {
	case strings.HasPrefix(model, "gpt"):
		return openai.NewOpenAIChatModel(h.baseConf)

	case strings.HasPrefix(model, "gemini"):
		return gemini.NewGeminiChatModel(h.baseConf, h.geminiKeyPool)

	default:
		return anthropic.NewAnthropicChatModel(h.baseConf)
	}
}

func (h *HandlerV1) newUcodeChatModel(model string) ai.ChatModel {
	switch {
	case strings.HasPrefix(model, "gpt"):
		return openai.NewOpenAIChatModel(h.baseConf)

	case strings.HasPrefix(model, "gemini"):
		return gemini.NewGeminiChatModel(h.baseConf, h.geminiKeyPool)

	default:
		return anthropic.NewAnthropicChatModelWithKey(h.baseConf, h.baseConf.AnthropicAPIKeyUcode)
	}
}

type agentToolset struct {
	defs  []ai.ToolDef
	perms map[string]*nb.AgentPermission
}

func buildAgentToolset(perms []*nb.AgentPermission) agentToolset {
	permMap := make(map[string]*nb.AgentPermission, len(perms))
	var creatable, readable, listable, updatable, deletable []string

	for _, p := range perms {
		slug := p.GetTableSlug()
		if slug == "" {
			continue
		}
		permMap[slug] = p
		if p.GetCanCreate() {
			creatable = append(creatable, slug)
		}
		if p.GetCanRead() {
			readable = append(readable, slug)
		}
		if p.GetCanList() {
			listable = append(listable, slug)
		}
		if p.GetCanUpdate() {
			updatable = append(updatable, slug)
		}
		if p.GetCanDelete() {
			deletable = append(deletable, slug)
		}
	}

	var defs []ai.ToolDef
	if len(creatable) > 0 {
		defs = append(defs, itemCreateTool(sortStrings(creatable)))
	}
	if len(readable) > 0 {
		defs = append(defs, itemGetTool(sortStrings(readable)))
	}
	if len(listable) > 0 {
		defs = append(defs, itemListTool(sortStrings(listable)))
	}
	if len(updatable) > 0 {
		defs = append(defs, itemUpdateTool(sortStrings(updatable)))
	}
	if len(deletable) > 0 {
		defs = append(defs, itemDeleteTool(sortStrings(deletable)))
	}

	// web_fetch is always available: it lets the agent research up-to-date external
	// data (e.g. exchange rates) that does not live in the project's own tables.
	defs = append(defs, webFetchTool())

	// create_pdf is always available: it turns model-authored HTML into a stored,
	// downloadable PDF so agents can deliver finished documents (proposals, reports).
	defs = append(defs, createPDFTool())

	return agentToolset{defs: defs, perms: permMap}
}

// ── tool definitions ──────────────────────────────────────────────────────────

func tableSlugSchema(tables []string) map[string]any {
	return map[string]any{
		"type":        "string",
		"enum":        tables,
		"description": "Slug of the target table.",
	}
}

func itemCreateTool(tables []string) ai.ToolDef {
	return ai.ToolDef{
		Name:        "item_create",
		Description: "Create a new record in a table. Returns the created record.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_slug": tableSlugSchema(tables),
				"data": map[string]any{
					"type":                 "object",
					"description":          "Field values for the new record.",
					"additionalProperties": true,
				},
			},
			"required": []string{"table_slug", "data"},
		},
	}
}

func itemGetTool(tables []string) ai.ToolDef {
	return ai.ToolDef{
		Name:        "item_get",
		Description: "Fetch a single record by its guid. Returns the record or an error if not found.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_slug": tableSlugSchema(tables),
				"guid":       map[string]any{"type": "string", "description": "The record's guid."},
			},
			"required": []string{"table_slug", "guid"},
		},
	}
}

func itemListTool(tables []string) ai.ToolDef {
	return ai.ToolDef{
		Name:        "item_list",
		Description: "List records in a table, optionally filtered. Returns matching records and a total count.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_slug": tableSlugSchema(tables),
				"filters": map[string]any{
					"type":                 "object",
					"description":          "Optional equality filters keyed by field name.",
					"additionalProperties": true,
				},
				"limit": map[string]any{
					"type":        "integer",
					"description": "Maximum number of records to return (default 20, max 100).",
				},
			},
			"required": []string{"table_slug"},
		},
	}
}

func itemUpdateTool(tables []string) ai.ToolDef {
	return ai.ToolDef{
		Name:        "item_update",
		Description: "Update an existing record identified by guid with the provided fields. Returns the updated record.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_slug": tableSlugSchema(tables),
				"guid":       map[string]any{"type": "string", "description": "The record's guid."},
				"data": map[string]any{
					"type":                 "object",
					"description":          "Field values to change.",
					"additionalProperties": true,
				},
			},
			"required": []string{"table_slug", "guid", "data"},
		},
	}
}

func itemDeleteTool(tables []string) ai.ToolDef {
	return ai.ToolDef{
		Name:        "item_delete",
		Description: "Delete a record identified by guid.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"table_slug": tableSlugSchema(tables),
				"guid":       map[string]any{"type": "string", "description": "The record's guid."},
			},
			"required": []string{"table_slug", "guid"},
		},
	}
}

func createPDFTool() ai.ToolDef {
	return ai.ToolDef{
		Name:        "create_pdf",
		Description: "Render a finished document into a downloadable PDF and store it. Use this whenever the user needs a polished document to download — a commercial proposal (КП), invoice, report, contract, certificate, etc. You author the document yourself as a single complete, self-contained HTML page and pass it in `html`; it is rendered to PDF exactly as given. Returns the public URL of the generated PDF.",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"title": map[string]any{
					"type":        "string",
					"description": "Short human-readable document title, used to name the file (e.g. \"Commercial Proposal - Acme\").",
				},
				"html": map[string]any{
					"type":        "string",
					"description": "A complete standalone HTML document (<!doctype html><html>…</html>) with ALL styling inline or inside a <style> block — no external CSS, JS or web fonts are loaded. Design it to look professional on A4: sensible margins, clear typography, and tables for any figures.",
				},
			},
			"required": []string{"html"},
		},
	}
}

// ── engine ────────────────────────────────────────────────────────────────────

// runAgent executes one agent invocation: it opens an agent_run audit record,
// drives the native tool-use loop until the model produces a final answer (or the
// step budget is exhausted / an error occurs), then finalizes the run. The
// returned AgentRun carries the terminal status, output, steps and token usage;
// the returned files are any documents the agent generated during the run (e.g.
// PDFs) for the caller to surface to the end-user.
func (h *HandlerV1) runAgent(ctx context.Context, service services.ServiceManagerI, resourceEnvId string, agent *nb.Agent, message string, runContext map[string]any) (*nb.AgentRun, []agentFile, error) {
	// Detach the run from the caller's request context. A run can take a while
	// (several model round-trips plus PDF rendering); if the end-user's HTTP
	// connection drops, we still want the agent to finish its server-side work
	// and persist the run. WithoutCancel ignores the parent's cancellation while
	// preserving its values (e.g. auth metadata needed by downstream gRPC calls);
	// the timeout bounds a runaway run.
	runCtx, cancelRun := context.WithTimeout(context.WithoutCancel(ctx), agentRunTotalTimeout)
	defer cancelRun()

	inputMap := map[string]any{"message": message}
	if len(runContext) > 0 {
		inputMap["context"] = runContext
	}
	inputStruct, _ := helperFunc.ConvertMapToStruct(inputMap)

	run, err := service.GoObjectBuilderService().Agent().CreateAgentRun(runCtx, &nb.CreateAgentRunRequest{
		ResourceEnvId: resourceEnvId,
		AgentId:       agent.GetId(),
		ProjectId:     agent.GetProjectId(),
		Input:         inputStruct,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("create agent run: %w", err)
	}

	model := h.newChatModel(agent.GetModel())
	toolset := buildAgentToolset(agent.GetPermissions())
	system := buildAgentSystemPrompt(agent, toolset)

	maxSteps := int(agent.GetMaxSteps())
	if maxSteps <= 0 {
		maxSteps = defaultAgentMaxSteps
	}

	userText := message
	if len(runContext) > 0 {
		if ctxJSON, jErr := json.Marshal(runContext); jErr == nil {
			userText = message + "\n\n## Context provided by the application\n" + string(ctxJSON)
		}
	}
	messages := []ai.ConversationMessage{{Role: "user", Text: userText}}

	var (
		steps             []*nb.AgentRunStep
		producedFiles     []agentFile
		totalTokens       int32
		finalText         string
		lastText          string
		runErr            error
		effectiveSystem   = system
		truncationRetries int
	)

	for step := 0; step < maxSteps; step++ {
		result, callErr := model.Complete(runCtx, ai.CompletionRequest{
			Model:     agent.GetModel(),
			MaxTokens: agentRunMaxTokens,
			System:    effectiveSystem,
			Messages:  messages,
			Tools:     toolset.defs,
			Timeout:   agentStepTimeout,
		})
		if result != nil {
			totalTokens += int32(result.Usage.InputTokens + result.Usage.OutputTokens)
		}
		if callErr != nil {
			// A truncated step (the model hit the output-token limit) is recoverable:
			// it tried to say or build too much at once. Rather than failing the whole
			// run, steer it to stop narrating and emit only its tool call, then retry
			// the step with the SAME conversation. This keeps a verbose model from
			// killing a run that was one create_pdf call away from finishing.
			if errors.Is(callErr, ai.ErrMaxTokens) && truncationRetries < maxAgentTruncationRetries {
				truncationRetries++
				effectiveSystem = system + agentTruncationRecoveryNote
				continue
			}
			runErr = callErr
			break
		}
		// Recovered: a clean step resets the steering so later steps get the plain prompt.
		truncationRetries = 0
		effectiveSystem = system
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

		toolResults := make([]ai.ToolResult, 0, len(result.ToolCalls))
		for _, call := range result.ToolCalls {
			// create_pdf is dispatched here (not in executeAgentTool) because it is the
			// only tool that yields an out-of-band artifact, and this loop owns the
			// collection of produced files.
			var (
				content string
				isErr   bool
			)
			if call.Name == "create_pdf" {
				var file *agentFile
				content, isErr, file = h.executeCreatePDF(runCtx, service, resourceEnvId, call)
				if file != nil {
					producedFiles = append(producedFiles, *file)
				}
			} else {
				content, isErr = h.executeAgentTool(runCtx, service, resourceEnvId, agent.GetProjectId(), toolset, call)
			}
			toolResults = append(toolResults, ai.ToolResult{
				ToolCallID: call.ID,
				Content:    content,
				IsError:    isErr,
			})
			toolInput, _ := helperFunc.ConvertMapToStruct(call.Input)
			steps = append(steps, &nb.AgentRunStep{
				Index:      int32(len(steps)),
				ToolName:   call.Name,
				ToolInput:  toolInput,
				ToolResult: content,
				IsError:    isErr,
			})
		}

		messages = append(messages, ai.ConversationMessage{Role: "user", ToolResults: toolResults})
	}

	if finalText == "" {
		finalText = lastText
	}

	status, errMsg := agentStatusSucceeded, ""
	if runErr != nil {
		status, errMsg = agentStatusFailed, runErr.Error()
	}

	// Persist the run on a fresh, cancellation-immune context so the record is
	// always written — even if the run hit its own deadline above.
	finalizeCtx, cancelFinalize := context.WithTimeout(context.WithoutCancel(ctx), agentFinalizeTimeout)
	defer cancelFinalize()

	updated, err := service.GoObjectBuilderService().Agent().UpdateAgentRun(finalizeCtx, &nb.UpdateAgentRunRequest{
		ResourceEnvId: resourceEnvId,
		Id:            run.GetId(),
		Status:        status,
		Output:        finalText,
		Steps:         steps,
		TokensUsed:    totalTokens,
		Error:         errMsg,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("finalize agent run: %w", err)
	}

	return updated, producedFiles, nil
}

// executeAgentTool runs a single tool call against the project's items, enforcing
// the agent's permission rule for the targeted table. It returns a JSON string
// result (or an error message) and whether the call failed.
func (h *HandlerV1) executeAgentTool(ctx context.Context, service services.ServiceManagerI, resourceEnvId, projectId string, toolset agentToolset, call ai.ToolCall) (string, bool) {

	// web_fetch is not a table operation, so it bypasses the permission matrix.
	if call.Name == "web_fetch" {
		return executeWebFetch(ctx, call)
	}

	tableSlug := cast.ToString(call.Input["table_slug"])
	if tableSlug == "" {
		return "error: table_slug is required", true
	}
	perm := toolset.perms[tableSlug]
	if perm == nil {
		return fmt.Sprintf("error: agent has no permission for table %q", tableSlug), true
	}

	switch call.Name {
	case "item_create":
		if !perm.GetCanCreate() {
			return fmt.Sprintf("error: create not allowed on table %q", tableSlug), true
		}
		data := asStringMap(call.Input["data"])
		if len(data) == 0 {
			return "error: data is required and must be a non-empty object", true
		}
		data["company_service_project_id"] = projectId
		structData, err := helperFunc.ConvertMapToStruct(data)
		if err != nil {
			return "error: " + err.Error(), true
		}
		resp, err := service.GoObjectBuilderService().Items().Create(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			ProjectId: resourceEnvId,
			Data:      structData,
		})
		if err != nil {
			return "error: " + err.Error(), true
		}
		return marshalRecord(resp.GetData(), map[string]any{"status": "created"}), false

	case "item_get":
		if !perm.GetCanRead() {
			return fmt.Sprintf("error: read not allowed on table %q", tableSlug), true
		}
		guid := cast.ToString(call.Input["guid"])
		if guid == "" {
			return "error: guid is required", true
		}
		item, found, err := h.lookupItem(ctx, service, resourceEnvId, tableSlug, guid)
		if err != nil {
			return "error: " + err.Error(), true
		}
		if !found {
			return fmt.Sprintf("error: no record with guid %q in table %q", guid, tableSlug), true
		}
		return marshalToolResult(item), false

	case "item_list":
		if !perm.GetCanList() {
			return fmt.Sprintf("error: list not allowed on table %q", tableSlug), true
		}
		filters := asStringMap(call.Input["filters"])
		limit := cast.ToInt(call.Input["limit"])
		if limit <= 0 || limit > 100 {
			limit = 20
		}
		filters["limit"] = limit
		filterStruct, err := helperFunc.ConvertMapToStruct(filters)
		if err != nil {
			return "error: " + err.Error(), true
		}
		resp, err := service.GoObjectBuilderService().ObjectBuilder().GetList2(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			ProjectId: resourceEnvId,
			Data:      filterStruct,
		})
		if err != nil {
			return "error: " + err.Error(), true
		}
		listData, _ := helperFunc.ConvertStructToMap(resp.GetData())
		return marshalToolResult(map[string]any{
			"items": extractItemsFromData(listData),
			"count": extractCountFromData(listData),
		}), false

	case "item_update":
		if !perm.GetCanUpdate() {
			return fmt.Sprintf("error: update not allowed on table %q", tableSlug), true
		}
		guid := cast.ToString(call.Input["guid"])
		if guid == "" {
			return "error: guid is required", true
		}
		data := asStringMap(call.Input["data"])
		if len(data) == 0 {
			return "error: data is required and must be a non-empty object", true
		}
		data["guid"] = guid
		data["id"] = guid
		data["company_service_project_id"] = projectId
		structData, err := helperFunc.ConvertMapToStruct(data)
		if err != nil {
			return "error: " + err.Error(), true
		}
		resp, err := service.GoObjectBuilderService().Items().Update(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			ProjectId: resourceEnvId,
			Data:      structData,
		})
		if err != nil {
			return "error: " + err.Error(), true
		}
		return marshalRecord(resp.GetData(), map[string]any{"status": "updated", "guid": guid}), false

	case "item_delete":
		if !perm.GetCanDelete() {
			return fmt.Sprintf("error: delete not allowed on table %q", tableSlug), true
		}
		guid := cast.ToString(call.Input["guid"])
		if guid == "" {
			return "error: guid is required", true
		}
		structData, err := helperFunc.ConvertMapToStruct(map[string]any{
			"id":                         guid,
			"guid":                       guid,
			"company_service_project_id": projectId,
		})
		if err != nil {
			return "error: " + err.Error(), true
		}
		if _, err = service.GoObjectBuilderService().Items().Delete(ctx, &nb.CommonMessage{
			TableSlug: tableSlug,
			ProjectId: resourceEnvId,
			Data:      structData,
		}); err != nil {
			return "error: " + err.Error(), true
		}
		return marshalToolResult(map[string]any{"status": "deleted", "guid": guid}), false

	default:
		return fmt.Sprintf("error: unknown tool %q", call.Name), true
	}
}

// lookupItem fetches a single record by guid via the list endpoint.
func (h *HandlerV1) lookupItem(ctx context.Context, service services.ServiceManagerI, resourceEnvId, tableSlug, guid string) (map[string]any, bool, error) {

	filterStruct, err := helperFunc.ConvertMapToStruct(map[string]any{"guid": guid, "limit": 1})
	if err != nil {
		return nil, false, err
	}
	resp, err := service.GoObjectBuilderService().ObjectBuilder().GetList2(ctx, &nb.CommonMessage{
		TableSlug: tableSlug,
		ProjectId: resourceEnvId,
		Data:      filterStruct,
	})
	if err != nil {
		return nil, false, err
	}
	listData, _ := helperFunc.ConvertStructToMap(resp.GetData())
	items := extractItemsFromData(listData)
	if len(items) == 0 {
		return nil, false, nil
	}
	return items[0], true, nil
}

// executeCreatePDF renders the model-authored HTML into a PDF via the document
// generator service, stores it in the project's environment bucket, and returns a
// JSON tool result for the model plus the produced file for the caller to surface.
func (h *HandlerV1) executeCreatePDF(ctx context.Context, service services.ServiceManagerI, resourceEnvId string, call ai.ToolCall) (string, bool, *agentFile) {

	html := strings.TrimSpace(cast.ToString(call.Input["html"]))
	if html == "" {
		return "error: html is required and must be a complete HTML document", true, nil
	}

	renderCtx, cancel := context.WithTimeout(ctx, pdfRenderTimeout)
	defer cancel()

	resp, err := service.DocGeneratorService().DocumentGenerator().ConvertHtml(renderCtx, &doc_generator_service.ConvertHtmlRequest{
		Data:         []byte(html),
		InputFormat:  "HTML",
		OutputFormat: "PDF",
	})
	if err != nil {
		return "error: pdf rendering failed: " + err.Error(), true, nil
	}
	if !resp.GetSuccess() {
		msg := resp.GetErrorMessage()
		if msg == "" {
			msg = "document generator returned no document"
		}
		return "error: pdf rendering failed: " + msg, true, nil
	}
	pdf := resp.GetData()
	if len(pdf) == 0 {
		return "error: pdf rendering returned an empty document", true, nil
	}

	fileName := pdfFileName(cast.ToString(call.Input["title"]))
	fileURL, err := h.uploadAgentPDF(ctx, resourceEnvId, fileName, pdf)
	if err != nil {
		return "error: storing the generated pdf failed: " + err.Error(), true, nil
	}

	file := &agentFile{Name: fileName, URL: fileURL, ContentType: "application/pdf"}
	return marshalToolResult(map[string]any{
		"status": "created",
		"name":   fileName,
		"url":    fileURL,
	}), false, file
}

// uploadAgentPDF stores PDF bytes in the project's environment bucket under the
// agent-files prefix and returns the public URL.
func (h *HandlerV1) uploadAgentPDF(ctx context.Context, resourceEnvId, fileName string, data []byte) (string, error) {

	minioClient, err := minio.New(h.baseConf.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(h.baseConf.MinioAccessKeyID, h.baseConf.MinioSecretAccessKey, ""),
		Secure: h.baseConf.MinioProtocol,
	})
	if err != nil {
		return "", err
	}

	objectPath := agentFilesFolder + "/" + fileName
	if _, err = minioClient.PutObject(
		ctx,
		resourceEnvId,
		objectPath,
		bytes.NewReader(data),
		int64(len(data)),
		minio.PutObjectOptions{ContentType: "application/pdf"},
	); err != nil {
		return "", err
	}

	return "https://" + h.baseConf.MinioEndpoint + "/" + resourceEnvId + "/" + objectPath, nil
}

// pdfFileName builds a unique, URL-safe PDF file name from an optional title.
func pdfFileName(title string) string {
	slug := slugifyForFile(title)
	if slug == "" {
		slug = "document"
	}
	return uuid.NewString() + "_" + slug + ".pdf"
}

// slugifyForFile reduces an arbitrary (possibly non-ASCII) title to a short,
// lowercase, URL-safe slug. Non-ASCII titles collapse to empty, in which case the
// caller falls back to a default name.
func slugifyForFile(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))

	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '-' || r == '_':
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	out := strings.Trim(b.String(), "-")
	if len(out) > 60 {
		out = strings.Trim(out[:60], "-")
	}
	return out
}

// ── prompt + small helpers ────────────────────────────────────────────────────

func buildAgentSystemPrompt(agent *nb.Agent, toolset agentToolset) string {
	var b strings.Builder
	b.WriteString(agent.GetInstruction())

	if len(toolset.perms) > 0 {
		b.WriteString("\n\n## Data access\n")
		b.WriteString("You can read and modify application data ONLY through the provided tools, ")
		b.WriteString("and ONLY on the tables listed below with the listed operations. ")
		b.WriteString("Never assume access to a table or operation that is not listed.\n\n")

		slugs := make([]string, 0, len(toolset.perms))
		for slug := range toolset.perms {
			slugs = append(slugs, slug)
		}
		sort.Strings(slugs)
		for _, slug := range slugs {
			fmt.Fprintf(&b, "- %s: %s\n", slug, strings.Join(allowedOps(toolset.perms[slug]), ", "))
		}

		b.WriteString("\nWhen the request is to create, update, or delete data, perform that change yourself with these tools rather than describing it or handing a value back — once you call the tool the change is already saved, so never ask the caller to save anything and never hedge about whether it was applied. When a value you produce belongs in a record field, write the clean final value itself (e.g. a concise description), not a report about it.")
		b.WriteString("\nWhen you have fully addressed the request, reply with a final natural-language message and stop calling tools. Keep that reply brief and to the point — a short confirmation of what you did, with no unnecessary markdown tables, headings, or disclaimers.")
	}

	b.WriteString("\n\n## External data\n")
	b.WriteString("Use the web_fetch tool to fetch public web URLs (e.g. JSON APIs or pages) when you need up-to-date external information such as exchange rates, prices, or reference data that is not stored in the application. For anything that lives in the application's own database, use the data tools above instead.")

	b.WriteString("\n\n## Producing documents\n")
	b.WriteString("When the user needs a finished document to download — a commercial proposal (КП), invoice, report, contract, certificate and the like — produce it yourself with the create_pdf tool. ")
	b.WriteString("You write the whole document as ONE complete, self-contained HTML page (all styling inline or in a <style> block; no external CSS, JS or web fonts) and pass it as `html`; it is rendered to PDF exactly as given and stored, and create_pdf returns its download URL. ")
	b.WriteString("Make it genuinely professional and on-topic: a clean letterhead/title, well-structured sections, and proper tables for any line items or figures, laid out for A4. You are responsible for the wording and for any calculations or totals the document needs — compute them yourself and present the final values; do not leave placeholders. ")
	b.WriteString("Do NOT narrate your reasoning or spell out the calculations step by step in your reply text — work them out silently and put the final numbers straight into the document's HTML. Writing out a long calculation walkthrough wastes your output budget and can cut you off before you emit the document; go directly to the create_pdf call with the finished HTML. ")
	b.WriteString("After create_pdf succeeds, just briefly confirm the document is ready — the application delivers the download to the user, so never paste raw HTML or the full document text into your reply.")

	return b.String()
}

func allowedOps(p *nb.AgentPermission) []string {
	var ops []string
	if p.GetCanCreate() {
		ops = append(ops, "create")
	}
	if p.GetCanRead() {
		ops = append(ops, "read")
	}
	if p.GetCanList() {
		ops = append(ops, "list")
	}
	if p.GetCanUpdate() {
		ops = append(ops, "update")
	}
	if p.GetCanDelete() {
		ops = append(ops, "delete")
	}
	if len(ops) == 0 {
		ops = append(ops, "none")
	}
	return ops
}

// asStringMap coerces a tool-argument value into a string-keyed map.
func asStringMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func marshalToolResult(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// marshalRecord returns the record's data as JSON, or a fallback map when the
// service returned no body.
func marshalRecord(data *structpb.Struct, fallback map[string]any) string {
	if m, err := helperFunc.ConvertStructToMap(data); err == nil && len(m) > 0 {
		return marshalToolResult(m)
	}
	return marshalToolResult(fallback)
}

func sortStrings(in []string) []string {
	sort.Strings(in)
	return in
}

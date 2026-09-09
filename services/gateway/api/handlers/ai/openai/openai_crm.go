package openai

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/handlers/ai/chat_prompts"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func (a *OpenAIAgent) CRMQuery(ctx context.Context, in models.CRMAssistantInput) (*models.CRMAssistantPlan, error) {
	content := chat_prompts.BuildCRMAssistantMessage(in)
	messages := buildOpenAIMessages(in.History, buildContentParts(content, in.Images))
	cfg := a.conf.OpenAIAgents.DatabaseAssistant
	if len(in.Images) > 0 {
		// Screenshot-backed field configuration needs reliable OCR and UI
		// understanding. Keep the inexpensive database model for text-only CRM
		// analytics, and use the stronger inspector model only for visual turns.
		cfg = a.conf.OpenAIAgents.Inspector
	}

	raw, usage, err := callTool(ctx, a.conf, cfg, chat_prompts.PromptCRMAssistant, messages, crmAssistantTool())
	a.tracker.RecordUsage(usage, cfg.Model, "CRM assistant")
	a.tracker.Deduct(int64(usage.InputTokens + usage.OutputTokens))
	if err != nil {
		return nil, fmt.Errorf("crm assistant: %w", err)
	}

	var plan models.CRMAssistantPlan
	if err = json.Unmarshal(raw, &plan); err != nil {
		return nil, fmt.Errorf("crm assistant: parse response: %w", err)
	}
	return &plan, nil
}

func crmAssistantTool() chatTool {
	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	pipelineStage := map[string]any{
		"type":     "object",
		"required": []string{"name"},
		"properties": map[string]any{
			"name":        map[string]any{"type": "string"},
			"group":       map[string]any{"type": "string", "enum": []string{"todo", "won", "lost"}},
			"color":       map[string]any{"type": "string"},
			"probability": map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
		},
	}
	pipelineAction := map[string]any{
		"type":     "object",
		"required": []string{"operation", "pipeline_name"},
		"properties": map[string]any{
			"operation":         map[string]any{"type": "string", "enum": []string{"create_pipeline", "rename_pipeline", "delete_pipeline", "add_stage", "update_stage", "delete_stage", "reorder_stages"}},
			"pipeline_name":     map[string]any{"type": "string"},
			"new_pipeline_name": map[string]any{"type": "string"},
			"stage_name":        map[string]any{"type": "string"},
			"new_stage_name":    map[string]any{"type": "string"},
			"stage_group":       map[string]any{"type": "string", "enum": []string{"todo", "won", "lost"}},
			"color":             map[string]any{"type": "string"},
			"probability":       map[string]any{"type": "integer", "minimum": 0, "maximum": 100},
			"position":          map[string]any{"type": "integer", "minimum": 1},
			"stages":            map[string]any{"type": "array", "items": pipelineStage},
		},
	}
	recordAction := map[string]any{
		"type":     "object",
		"required": []string{"operation", "table"},
		"properties": map[string]any{
			"operation":   map[string]any{"type": "string", "enum": []string{"create", "update", "delete"}},
			"table":       map[string]any{"type": "string", "enum": []string{"deals", "contacts", "companies", "tasks"}},
			"record_guid": map[string]any{"type": "string"},
			"data":        map[string]any{"type": "object", "additionalProperties": true},
			"records": map[string]any{
				"type":     "array",
				"minItems": 1,
				"maxItems": 50,
				"items":    map[string]any{"type": "object", "additionalProperties": true},
			},
		},
	}
	batchAction := map[string]any{
		"type":     "object",
		"required": []string{"pipeline_action", "record_action"},
		"properties": map[string]any{
			"pipeline_action": pipelineAction,
			"record_action":   recordAction,
		},
	}
	return chatTool{
		Type: "function",
		Function: functionDef{
			Name:        "respond_crm_assistant",
			Description: "Return the next safe CRM database step, pipeline/stage operation, combined import, final answer, or card field visibility action.",
			Parameters: map[string]any{
				"type":     "object",
				"required": []string{"action", "needs_more_data", "reply"},
				"properties": map[string]any{
					"action":          map[string]any{"type": "string", "enum": []string{"query", "record_action", "pipeline_action", "batch_action", "answer", "client_action", "schema"}},
					"sql":             map[string]any{"type": "string"},
					"sql_params":      map[string]any{"type": "array", "items": map[string]any{}},
					"needs_more_data": map[string]any{"type": "boolean"},
					"query_plan":      map[string]any{"type": "string"},
					"reply":           map[string]any{"type": "string"},
					"success_message": map[string]any{"type": "string"},
					"cancel_message":  map[string]any{"type": "string"},
					"pipeline_action": pipelineAction,
					"record_action":   recordAction,
					"batch_action":    batchAction,
					"client_actions": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":     "object",
							"required": []string{"type", "table"},
							"properties": map[string]any{
								"type":        map[string]any{"type": "string", "enum": []string{"set_card_field_visibility"}},
								"table":       map[string]any{"type": "string"},
								"show_fields": stringArray,
								"hide_fields": stringArray,
								"field_order": stringArray,
							},
						},
					},
				},
			},
		},
	}
}

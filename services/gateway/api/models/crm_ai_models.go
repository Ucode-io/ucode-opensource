package models

type (
	CRMAssistantMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}

	CRMAssistantPageContext struct {
		Path             string                `json:"path,omitempty"`
		Table            string                `json:"table,omitempty"`
		Timezone         string                `json:"timezone,omitempty"`
		Now              string                `json:"now,omitempty"`
		HiddenFields     []string              `json:"hidden_fields,omitempty"`
		FieldOrder       []string              `json:"field_order,omitempty"`
		CardFields       []CRMCardFieldContext `json:"card_fields,omitempty"`
		SelectedPipeline string                `json:"selected_pipeline,omitempty"`
	}

	CRMCardFieldContext struct {
		Slug  string `json:"slug"`
		Label string `json:"label,omitempty"`
	}

	CRMAssistantRequest struct {
		Message     string                  `json:"message"`
		Images      []string                `json:"images,omitempty"`
		History     []CRMAssistantMessage   `json:"history,omitempty"`
		PageContext CRMAssistantPageContext `json:"page_context,omitempty"`
	}

	CRMClientAction struct {
		Type           string             `json:"type"`
		Table          string             `json:"table"`
		ShowFields     []string           `json:"show_fields,omitempty"`
		HideFields     []string           `json:"hide_fields,omitempty"`
		FieldOrder     []string           `json:"field_order,omitempty"`
		PipelineAction *CRMPipelineAction `json:"pipeline_action,omitempty"`
		RecordAction   *CRMRecordAction   `json:"record_action,omitempty"`
		BatchAction    *CRMBatchAction    `json:"batch_action,omitempty"`
	}

	CRMPipelineStageInput struct {
		Name        string `json:"name"`
		Group       string `json:"group,omitempty"`
		Color       string `json:"color,omitempty"`
		Probability int    `json:"probability,omitempty"`
	}

	CRMPipelineAction struct {
		Operation       string                  `json:"operation"`
		PipelineName    string                  `json:"pipeline_name"`
		NewPipelineName string                  `json:"new_pipeline_name,omitempty"`
		StageName       string                  `json:"stage_name,omitempty"`
		NewStageName    string                  `json:"new_stage_name,omitempty"`
		StageGroup      string                  `json:"stage_group,omitempty"`
		Color           string                  `json:"color,omitempty"`
		Probability     *int                    `json:"probability,omitempty"`
		Position        int                     `json:"position,omitempty"`
		Stages          []CRMPipelineStageInput `json:"stages,omitempty"`
	}

	CRMRecordAction struct {
		Operation  string           `json:"operation"`
		Table      string           `json:"table"`
		RecordGUID string           `json:"record_guid,omitempty"`
		Data       map[string]any   `json:"data,omitempty"`
		Records    []map[string]any `json:"records,omitempty"`
	}

	CRMBatchAction struct {
		PipelineAction *CRMPipelineAction `json:"pipeline_action,omitempty"`
		RecordAction   *CRMRecordAction   `json:"record_action,omitempty"`
	}

	CRMAssistantPlan struct {
		Action         string             `json:"action"`
		SQL            string             `json:"sql,omitempty"`
		SQLParams      []any              `json:"sql_params,omitempty"`
		NeedsMoreData  bool               `json:"needs_more_data"`
		QueryPlan      string             `json:"query_plan,omitempty"`
		Reply          string             `json:"reply,omitempty"`
		SuccessMessage string             `json:"success_message,omitempty"`
		CancelMessage  string             `json:"cancel_message,omitempty"`
		ClientActions  []CRMClientAction  `json:"client_actions,omitempty"`
		PipelineAction *CRMPipelineAction `json:"pipeline_action,omitempty"`
		RecordAction   *CRMRecordAction   `json:"record_action,omitempty"`
		BatchAction    *CRMBatchAction    `json:"batch_action,omitempty"`
	}

	CRMAssistantInput struct {
		Message     string
		SchemaText  string
		DataContext string
		Images      []string
		History     []ChatMessage
		PageContext CRMAssistantPageContext
	}

	CRMActionProposal struct {
		ID          string `json:"id"`
		Description string `json:"description"`
		Kind        string `json:"kind"`
	}

	CRMAssistantResponse struct {
		Reply         string             `json:"reply"`
		ClientActions []CRMClientAction  `json:"client_actions,omitempty"`
		PendingAction *CRMActionProposal `json:"pending_action,omitempty"`
	}

	CRMConfirmActionRequest struct {
		Confirmed bool `json:"confirmed"`
	}

	CRMFieldPreferences struct {
		HiddenFields []string `json:"hidden_fields"`
		FieldOrder   []string `json:"field_order"`
	}

	CRMFieldPreferencesResponse struct {
		Found        bool     `json:"found"`
		Table        string   `json:"table"`
		HiddenFields []string `json:"hidden_fields"`
		FieldOrder   []string `json:"field_order"`
	}
)

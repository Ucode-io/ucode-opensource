package v1

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func newCRMBatchPendingAction(
	request string,
	action *models.CRMBatchAction,
	schema []models.TableSchema,
	resourceEnvID string,
) (*models.PendingAction, error) {
	if action == nil || action.PipelineAction == nil || action.RecordAction == nil {
		return nil, fmt.Errorf("batch_action requires pipeline_action and record_action")
	}
	pipeline, err := normalizeCRMPipelineAction(*action.PipelineAction)
	if err != nil {
		return nil, fmt.Errorf("pipeline action: %w", err)
	}
	if pipeline.Operation != "create_pipeline" {
		return nil, fmt.Errorf("batch_action currently supports create_pipeline only")
	}
	record, err := normalizeCRMRecordAction(*action.RecordAction, schema)
	if err != nil {
		return nil, fmt.Errorf("record action: %w", err)
	}
	if record.Operation != "create" || record.Table != "deals" || len(record.Records) == 0 {
		return nil, fmt.Errorf("batch_action requires bulk deal creation")
	}
	if requested := requestedCRMRecordCreateCount(request); requested > 1 && len(record.Records) != requested {
		return nil, fmt.Errorf("bulk create requested %d records; records must contain exactly %d objects", requested, requested)
	}
	for index, item := range record.Records {
		pipelineRaw, hasPipeline := item["pipeline"]
		if !hasPipeline || pipelineRaw == nil || strings.TrimSpace(fmt.Sprint(pipelineRaw)) == "" {
			item["pipeline"] = pipeline.PipelineName
		} else if !crmRecordValueContainsString(pipelineRaw, pipeline.PipelineName) {
			return nil, fmt.Errorf("record %d targets pipeline %q instead of %q", index+1, fmt.Sprint(pipelineRaw), pipeline.PipelineName)
		}
	}
	normalized := models.CRMBatchAction{PipelineAction: &pipeline, RecordAction: &record}
	count := len(record.Records)
	description := fmt.Sprintf("CRM o‘zgarishini tasdiqlang: %q pipeline va %d ta deal yaratiladi.", pipeline.PipelineName, count)
	success := fmt.Sprintf("%q pipeline yaratildi va %d ta deal qo‘shildi.", pipeline.PipelineName, count)
	switch detectCRMRequestLanguage(request) {
	case "ru":
		description = fmt.Sprintf("Подтвердите изменение CRM: создать воронку %q и %d сделок.", pipeline.PipelineName, count)
		success = fmt.Sprintf("Воронка %q создана, добавлено сделок: %d.", pipeline.PipelineName, count)
	case "en":
		description = fmt.Sprintf("Confirm the CRM change: create pipeline %q and %d deals.", pipeline.PipelineName, count)
		success = fmt.Sprintf("Pipeline %q created with %d deals.", pipeline.PipelineName, count)
	}
	return &models.PendingAction{
		Action:             "client_batch",
		TableSlug:          "deals",
		Data:               map[string]any{"batch_action": normalized},
		ResourceEnvID:      resourceEnvID,
		ProjectID:          resourceEnvID,
		Description:        description,
		ConfirmationPrompt: description,
		SuccessMessage:     success,
		CancelMessage:      crmMutationCancelMessage(request),
	}, nil
}

func crmRecordValueContainsString(value any, expected string) bool {
	switch typed := value.(type) {
	case []string:
		return len(typed) == 1 && typed[0] == expected
	case []any:
		return len(typed) == 1 && fmt.Sprint(typed[0]) == expected
	default:
		return fmt.Sprint(value) == expected
	}
}

func decodeStoredCRMBatchAction(data map[string]any) (*models.CRMBatchAction, error) {
	raw, ok := data["batch_action"]
	if !ok {
		return nil, fmt.Errorf("stored batch action is missing")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var action models.CRMBatchAction
	if err = json.Unmarshal(encoded, &action); err != nil {
		return nil, err
	}
	if action.PipelineAction == nil || action.RecordAction == nil {
		return nil, fmt.Errorf("stored batch action is incomplete")
	}
	pipeline, err := normalizeCRMPipelineAction(*action.PipelineAction)
	if err != nil {
		return nil, err
	}
	record, err := normalizeCRMRecordAction(*action.RecordAction, nil)
	if err != nil {
		return nil, err
	}
	action.PipelineAction = &pipeline
	action.RecordAction = &record
	return &action, nil
}

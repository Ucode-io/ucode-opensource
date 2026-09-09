package v1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func newCRMRecordPendingAction(
	request string,
	action *models.CRMRecordAction,
	schema []models.TableSchema,
	resourceEnvID string,
) (*models.PendingAction, error) {
	if action == nil {
		return nil, fmt.Errorf("record_action is required")
	}
	normalized, err := normalizeCRMRecordAction(*action, schema)
	if err != nil {
		return nil, err
	}
	if requested := requestedCRMRecordCreateCount(request); normalized.Operation == "create" && requested > 1 && len(normalized.Records) != requested {
		return nil, fmt.Errorf("bulk create requested %d records; records must contain exactly %d objects", requested, requested)
	}
	description, success := crmRecordActionMessages(request, normalized)
	return &models.PendingAction{
		Action:             "client_record",
		TableSlug:          normalized.Table,
		Data:               map[string]any{"record_action": normalized},
		ResourceEnvID:      resourceEnvID,
		ProjectID:          resourceEnvID,
		Description:        description,
		ConfirmationPrompt: description,
		SuccessMessage:     success,
		CancelMessage:      crmMutationCancelMessage(request),
	}, nil
}

func normalizeCRMRecordAction(action models.CRMRecordAction, schema []models.TableSchema) (models.CRMRecordAction, error) {
	action.Operation = strings.ToLower(strings.TrimSpace(action.Operation))
	action.Table = strings.ToLower(strings.TrimSpace(action.Table))
	action.RecordGUID = strings.TrimSpace(action.RecordGUID)
	if action.Operation != "create" && action.Operation != "update" && action.Operation != "delete" {
		return action, fmt.Errorf("unsupported record operation %q", action.Operation)
	}
	if action.Table != "deals" && action.Table != "contacts" && action.Table != "companies" && action.Table != "tasks" {
		return action, fmt.Errorf("unsupported CRM table %q", action.Table)
	}
	if action.Operation != "create" && (action.RecordGUID == "" || len(action.RecordGUID) > 128) {
		return action, fmt.Errorf("record_guid is required for update/delete")
	}
	if action.Operation == "delete" {
		action.Data = nil
		action.Records = nil
		return action, nil
	}
	if action.Operation != "create" && len(action.Records) > 0 {
		return action, fmt.Errorf("records is supported only for create")
	}
	if len(action.Data) == 0 && len(action.Records) == 0 {
		return action, fmt.Errorf("record data is required")
	}
	if len(action.Records) > 50 {
		return action, fmt.Errorf("too many records")
	}
	if len(action.Data) > 100 {
		return action, fmt.Errorf("too many record fields")
	}

	allowedFields := map[string]models.FieldSchema{}
	if len(schema) > 0 {
		table, ok := findCRMSchemaTable(schema, action.Table)
		if !ok {
			return action, fmt.Errorf("table %q is not in the project schema", action.Table)
		}
		for _, field := range table.Fields {
			allowedFields[field.Slug] = field
		}
	}
	protected := map[string]bool{
		"guid": true, "id": true, "project_id": true, "environment_id": true,
		"created_at": true, "updated_at": true, "deleted_at": true,
	}
	cleanRecord := func(record map[string]any) (map[string]any, error) {
		if len(record) == 0 {
			return nil, fmt.Errorf("record data is required")
		}
		if len(record) > 100 {
			return nil, fmt.Errorf("too many record fields")
		}
		cleaned := make(map[string]any, len(record))
		for rawKey, value := range record {
			key := strings.TrimSpace(rawKey)
			if !isCRMPreferenceSlug(key) || protected[strings.ToLower(key)] {
				return nil, fmt.Errorf("field %q cannot be changed", rawKey)
			}
			if len(allowedFields) > 0 {
				field, ok := allowedFields[key]
				if !ok {
					return nil, fmt.Errorf("field %q is not in table %q", key, action.Table)
				}
				var valueErr error
				value, valueErr = normalizeCRMRecordFieldValue(field, value)
				if valueErr != nil {
					return nil, valueErr
				}
			}
			cleaned[key] = value
		}
		return cleaned, nil
	}
	if len(action.Data) > 0 {
		cleaned, cleanErr := cleanRecord(action.Data)
		if cleanErr != nil {
			return action, cleanErr
		}
		action.Data = cleaned
	}
	if len(action.Records) > 0 {
		cleanedRecords := make([]map[string]any, 0, len(action.Records))
		for index, record := range action.Records {
			cleaned, cleanErr := cleanRecord(record)
			if cleanErr != nil {
				return action, fmt.Errorf("record %d: %w", index+1, cleanErr)
			}
			cleanedRecords = append(cleanedRecords, cleaned)
		}
		action.Records = cleanedRecords
		action.Data = nil
	}
	return action, nil
}

func normalizeCRMRecordFieldValue(field models.FieldSchema, value any) (any, error) {
	fieldType := strings.ToLower(strings.TrimSpace(field.Type))
	// Ucode's MULTISELECT columns are PostgreSQL arrays. The item API expects
	// an array even when the user supplies a single option (for example Source).
	if strings.Contains(fieldType, "multi") {
		switch typed := value.(type) {
		case string:
			if strings.TrimSpace(typed) == "" {
				return []string{}, nil
			}
			return []string{typed}, nil
		case nil:
			return []string{}, nil
		}
		return value, nil
	}
	if values, ok := value.([]any); ok {
		if len(values) == 1 {
			return values[0], nil
		}
		return nil, fmt.Errorf("field %q is scalar; use records for bulk create instead of an array", field.Slug)
	}
	return value, nil
}

var crmBulkCreateCountPattern = regexp.MustCompile(`(?i)\b([2-9]|[1-4][0-9]|50)\s*(?:ta\s*)?(?:leads?|lids?|lidlar|deals?|records?|yozuv|kontakt|contacts?|companies|kompaniy|tasks?|vazifa)`)
var crmSpreadsheetRowCountPattern = regexp.MustCompile(`(?i)\brow_count=([1-9]|[1-4][0-9]|50)\b`)

func requestedCRMRecordCreateCount(request string) int {
	if match := crmSpreadsheetRowCountPattern.FindStringSubmatch(request); len(match) == 2 {
		count, _ := strconv.Atoi(match[1])
		return count
	}
	match := crmBulkCreateCountPattern.FindStringSubmatch(strings.ToLower(request))
	if len(match) != 2 {
		return 0
	}
	count, _ := strconv.Atoi(match[1])
	return count
}

func crmRecordActionMessages(request string, action models.CRMRecordAction) (string, string) {
	operation := action.Operation
	if operation == "create" {
		count := len(action.Records)
		if count == 0 {
			count = 1
		}
		operation = fmt.Sprintf("create %d %s", count, action.Table)
	} else {
		operation += " " + action.Table + " / " + action.RecordGUID
	}
	switch detectCRMRequestLanguage(strings.ToLower(request)) {
	case "ru":
		return fmt.Sprintf("Подтвердите изменение CRM: %s.", operation), "Запись CRM обновлена."
	case "en":
		return fmt.Sprintf("Confirm the CRM change: %s.", operation), "CRM record updated."
	default:
		return fmt.Sprintf("CRM o‘zgarishini tasdiqlang: %s.", operation), "CRM yozuvi yangilandi."
	}
}

func decodeStoredCRMRecordAction(data map[string]any) (*models.CRMRecordAction, error) {
	raw, ok := data["record_action"]
	if !ok {
		return nil, fmt.Errorf("stored record action is missing")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var action models.CRMRecordAction
	if err = json.Unmarshal(encoded, &action); err != nil {
		return nil, err
	}
	normalized, err := normalizeCRMRecordAction(action, nil)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

package v1

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

var quotedCRMPipelineName = regexp.MustCompile(`["“”']([^"“”']{1,120})["“”']`)
var inlineCRMPipelineStageSeparator = regexp.MustCompile(`\s+[-–—•]\s+`)

var crmPipelineStageColors = []string{
	"#0d9488", "#3b82f6", "#f59e0b", "#8b5cf6", "#10b981", "#ef4444",
}

// buildCommonCRMPipelineAction makes the canonical multi-line pipeline command
// deterministic. It also prevents a word such as "stages" from being mistaken
// for a request to group last week's leads by status.
func buildCommonCRMPipelineAction(
	req models.CRMAssistantRequest,
	resourceEnvID string,
) (*crmAssistantResult, bool) {
	message := strings.TrimSpace(req.Message)
	lower := strings.ToLower(message)
	// Spreadsheet imports need one atomic batch action (pipeline + rows). Let the
	// model map the attached headers/rows instead of consuming the request as a
	// standalone deterministic pipeline mutation.
	if strings.Contains(lower, "spreadsheet_import_data") {
		return nil, false
	}
	if !containsAnyFold(lower, "pipeline", "pipleline", "voronka", "воронк") ||
		!containsAnyFold(lower, "create", "generate", "yarat", "qo‘sh", "qo'sh", "qosh", "созда", "добав") {
		return nil, false
	}

	nameMatch := quotedCRMPipelineName.FindStringSubmatch(message)
	if len(nameMatch) < 2 {
		return nil, false
	}

	lines := strings.Split(message, "\n")
	stages := make([]models.CRMPipelineStageInput, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimLeft(line, "-*•–—"))
		if line == "" || strings.Contains(strings.ToLower(line), "stage") || strings.Contains(strings.ToLower(line), "этап") {
			continue
		}
		// The first sentence contains the quoted pipeline name, not a stage.
		if strings.Contains(line, nameMatch[0]) {
			continue
		}
		stages = append(stages, models.CRMPipelineStageInput{Name: line})
	}
	if len(lines) == 1 {
		stageListStart := -1
		for _, marker := range []string{"stages", "stage:", "bosqichlar", "этапы", "этапами"} {
			if markerIndex := strings.Index(lower, marker); markerIndex >= 0 {
				stageListStart = markerIndex + len(marker)
				break
			}
		}
		if stageListStart >= 0 && stageListStart < len(message) {
			for _, stageName := range inlineCRMPipelineStageSeparator.Split(strings.TrimSpace(message[stageListStart:]), -1) {
				stageName = strings.TrimSpace(strings.TrimLeft(stageName, "-*•–—:,"))
				if stageName != "" {
					stages = append(stages, models.CRMPipelineStageInput{Name: stageName})
				}
			}
		}
	}
	if len(stages) == 0 {
		return nil, false
	}

	pending, err := newCRMPipelinePendingAction(message, &models.CRMPipelineAction{
		Operation:    "create_pipeline",
		PipelineName: strings.TrimSpace(nameMatch[1]),
		Stages:       stages,
	}, resourceEnvID)
	if err != nil {
		return nil, false
	}
	return &crmAssistantResult{reply: pending.Description, pendingAction: pending}, true
}

func newCRMPipelinePendingAction(
	request string,
	action *models.CRMPipelineAction,
	resourceEnvID string,
) (*models.PendingAction, error) {
	if action == nil {
		return nil, fmt.Errorf("pipeline_action is required")
	}
	normalized, err := normalizeCRMPipelineAction(*action)
	if err != nil {
		return nil, err
	}

	description, success := crmPipelineActionMessages(request, normalized)
	return &models.PendingAction{
		Action:             "client_pipeline",
		TableSlug:          "project_pipeline",
		Data:               map[string]any{"pipeline_action": normalized},
		ResourceEnvID:      resourceEnvID,
		ProjectID:          resourceEnvID,
		Description:        description,
		ConfirmationPrompt: description,
		SuccessMessage:     success,
		CancelMessage:      crmMutationCancelMessage(request),
	}, nil
}

func normalizeCRMPipelineAction(action models.CRMPipelineAction) (models.CRMPipelineAction, error) {
	action.Operation = strings.ToLower(strings.TrimSpace(action.Operation))
	action.PipelineName = strings.TrimSpace(action.PipelineName)
	action.NewPipelineName = strings.TrimSpace(action.NewPipelineName)
	action.StageName = strings.TrimSpace(action.StageName)
	action.NewStageName = strings.TrimSpace(action.NewStageName)
	if action.Operation == "add_stage" {
		probability := 0
		if action.Probability != nil {
			probability = *action.Probability
		}
		action.StageGroup = normalizeCRMPipelineStageGroup(action.StageGroup, action.StageName, probability, false)
		action.Color = normalizeCRMPipelineColor(action.Color, 0)
		normalizedProbability := clampCRMProbability(probability, action.StageGroup)
		action.Probability = &normalizedProbability
	} else {
		action.StageGroup = strings.ToLower(strings.TrimSpace(action.StageGroup))
		if action.StageGroup != "" && action.StageGroup != "todo" && action.StageGroup != "won" && action.StageGroup != "lost" {
			return action, fmt.Errorf("invalid stage_group %q", action.StageGroup)
		}
		if strings.TrimSpace(action.Color) != "" {
			if matched, _ := regexp.MatchString(`^#[0-9a-fA-F]{6}$`, action.Color); !matched {
				return action, fmt.Errorf("invalid stage color")
			}
			action.Color = strings.ToLower(action.Color)
		}
		if action.Probability != nil && (*action.Probability < 0 || *action.Probability > 100) {
			return action, fmt.Errorf("probability must be between 0 and 100")
		}
	}

	if action.PipelineName == "" || len([]rune(action.PipelineName)) > 120 {
		return action, fmt.Errorf("pipeline_name is required and must be at most 120 characters")
	}

	allowed := map[string]bool{
		"create_pipeline": true,
		"rename_pipeline": true,
		"delete_pipeline": true,
		"add_stage":       true,
		"update_stage":    true,
		"delete_stage":    true,
		"reorder_stages":  true,
	}
	if !allowed[action.Operation] {
		return action, fmt.Errorf("unsupported pipeline operation %q", action.Operation)
	}
	if action.Operation == "rename_pipeline" && action.NewPipelineName == "" {
		return action, fmt.Errorf("new_pipeline_name is required")
	}
	if containsAnyFold(action.Operation, "stage") && action.Operation != "reorder_stages" && action.StageName == "" && action.Operation != "add_stage" {
		return action, fmt.Errorf("stage_name is required")
	}
	if action.Operation == "add_stage" && action.StageName == "" && len(action.Stages) == 0 {
		return action, fmt.Errorf("stage_name is required")
	}
	seenStages := make(map[string]struct{}, len(action.Stages))
	normalizedStages := make([]models.CRMPipelineStageInput, 0, len(action.Stages))
	for index, stage := range action.Stages {
		stage.Name = strings.TrimSpace(stage.Name)
		if stage.Name == "" || len([]rune(stage.Name)) > 120 {
			return action, fmt.Errorf("every stage name is required and must be at most 120 characters")
		}
		key := strings.ToLower(stage.Name)
		if _, exists := seenStages[key]; exists {
			return action, fmt.Errorf("duplicate stage %q", stage.Name)
		}
		seenStages[key] = struct{}{}
		stage.Group = normalizeCRMPipelineStageGroup(stage.Group, stage.Name, stage.Probability, index == len(action.Stages)-1)
		stage.Color = normalizeCRMPipelineColor(stage.Color, index)
		stage.Probability = clampCRMProbability(stage.Probability, stage.Group)
		normalizedStages = append(normalizedStages, stage)
	}
	action.Stages = normalizedStages
	if action.Operation == "create_pipeline" && len(action.Stages) == 0 {
		action.Stages = []models.CRMPipelineStageInput{
			{Name: "New lead", Group: "todo", Color: crmPipelineStageColors[0], Probability: 10},
			{Name: "In progress", Group: "todo", Color: crmPipelineStageColors[1], Probability: 50},
			{Name: "Won", Group: "won", Color: crmPipelineStageColors[4], Probability: 100},
			{Name: "Lost", Group: "lost", Color: crmPipelineStageColors[5], Probability: 0},
		}
	}
	return action, nil
}

func normalizeCRMPipelineStageGroup(group, name string, probability int, last bool) string {
	group = strings.ToLower(strings.TrimSpace(group))
	if group == "todo" || group == "won" || group == "lost" {
		return group
	}
	lowerName := strings.ToLower(name)
	if containsAnyFold(lowerName, "lost", "lose", "rejected", "unqualified", "проигр", "отказ", "yo‘qot", "yo'qot") {
		return "lost"
	}
	if probability >= 100 || containsAnyFold(lowerName, "won", "closed", "sold", "paid", "payment", "выигр", "успеш", "опла", "sotildi", "to‘lov", "to'lov") {
		return "won"
	}
	if last && containsAnyFold(lowerName, "contract", "shartnoma", "договор") {
		return "won"
	}
	return "todo"
}

func normalizeCRMPipelineColor(color string, index int) string {
	color = strings.ToLower(strings.TrimSpace(color))
	if matched, _ := regexp.MatchString(`^#[0-9a-f]{6}$`, color); matched {
		return color
	}
	return crmPipelineStageColors[index%len(crmPipelineStageColors)]
}

func clampCRMProbability(value int, group string) int {
	if value >= 0 && value <= 100 && value != 0 {
		return value
	}
	if group == "won" {
		return 100
	}
	if group == "lost" {
		return 0
	}
	return 50
}

func crmPipelineActionMessages(request string, action models.CRMPipelineAction) (string, string) {
	detail := action.PipelineName
	if action.StageName != "" {
		detail += " / " + action.StageName
	}
	switch detectCRMRequestLanguage(strings.ToLower(request)) {
	case "ru":
		return fmt.Sprintf("Подтвердите изменение воронки: %s.", detail), "Воронка и этапы обновлены."
	case "en":
		return fmt.Sprintf("Confirm the pipeline change: %s.", detail), "Pipeline and stages updated."
	default:
		return fmt.Sprintf("Pipeline o‘zgarishini tasdiqlang: %s.", detail), "Pipeline va bosqichlar yangilandi."
	}
}

func decodeStoredCRMPipelineAction(data map[string]any) (*models.CRMPipelineAction, error) {
	raw, ok := data["pipeline_action"]
	if !ok {
		return nil, fmt.Errorf("stored pipeline action is missing")
	}
	encoded, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var action models.CRMPipelineAction
	if err = json.Unmarshal(encoded, &action); err != nil {
		return nil, err
	}
	normalized, err := normalizeCRMPipelineAction(action)
	if err != nil {
		return nil, err
	}
	return &normalized, nil
}

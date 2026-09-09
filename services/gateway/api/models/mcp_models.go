package models

type (
	ProjectScope struct {
		ProjectId     string
		EnvironmentId string
		APIKey        string
	}

	// ========================== MCP Requests ==========================

	MCPRequestWithPlanning struct {
		Prompt       string            `json:"prompt"`
		ImageURLs    []string          `json:"image_urls,omitempty"`
		Method       string            `json:"method,omitempty"`
		Context      *[]InspectContext `json:"context,omitempty"`
		BackendPlan  string            `json:"backend_plan,omitempty"`
		FrontendPlan string            `json:"frontend_plan,omitempty"`
	}

	MCPRequest struct {
		Method    string            `json:"method"`
		Prompt    string            `json:"prompt"`
		ImageURLs []string          `json:"image_urls,omitempty"`
		Context   *[]InspectContext `json:"context,omitempty"`
	}

	InspectContext struct {
		TargetFile      string `json:"target_file"`
		TargetElementId string `json:"target_element_id"`
		CodeFragment    string `json:"code_fragment"`
		Tag             string `json:"tag"`
		DOMPath         string `json:"dom_path"`
		Line            int    `json:"line"`
		Column          int    `json:"column"`
		ElementName     string `json:"name"`
	}

	// ========================== Prompt Building Requests ==========================

	GeneratePromptRequest struct {
		ProjectId     string `json:"project_id"`
		EnvironmentId string `json:"environment_id"`
		APIKey        string `json:"api_key"`
		UserPrompt    string `json:"user_prompt"`
		Method        string `json:"method"`
		BaseURL       string `json:"base_url"`
	}

	AnalyzeFrontendPromptRequest struct {
		UserRequest string            `json:"user_request"`
		FileGraph   map[string]any    `json:"file_graph"`
		ProjectName string            `json:"project_name"`
		Context     *[]InspectContext `json:"context,omitempty"`
	}

	UpdateFrontendPromptRequest struct {
		UserRequest    string            `json:"user_request"`
		FilesToUpdate  []ProjectFile     `json:"files_to_update"`
		AnalysisResult AnalysisResult    `json:"analysis_result"`
		ProjectName    string            `json:"project_name"`
		Context        *[]InspectContext `json:"context,omitempty"`
	}

	// ========================== MCP Analysis Results ==========================

	AnalysisResult struct {
		AnalysisSummary      string         `json:"analysis_summary"`
		FilesToModify        []FileToModify `json:"files_to_modify"`
		NewFilesNeeded       []FileToCreate `json:"new_files_needed"`
		FilesToDelete        []FileToDelete `json:"files_to_delete"`
		AffectedDependencies []string       `json:"affected_dependencies"`
		EstimatedComplexity  string         `json:"estimated_complexity"`
		Risks                []string       `json:"risks"`
	}

	UpdateResult struct {
		UpdatedFiles     []ProjectFile  `json:"updated_files"`
		NewFiles         []ProjectFile  `json:"new_files"`
		DeletedFiles     []string       `json:"deleted_files"`
		FileGraphUpdates map[string]any `json:"file_graph_updates"`
		IntegrationNotes []string       `json:"integration_notes"`
	}

	FileToModify struct {
		Path       string `json:"path"`
		Reason     string `json:"reason"`
		ChangeType string `json:"change_type"`
		Priority   string `json:"priority"`
	}

	FileToCreate struct {
		Path       string `json:"path"`
		Reason     string `json:"reason"`
		ChangeType string `json:"change_type"`
	}

	FileToDelete struct {
		Path   string `json:"path"`
		Reason string `json:"reason"`
	}

	ValidateApiKeyResponse struct {
		Valid         bool   `json:"valid"`
		AppId         string `json:"app_id"`
		ProjectId     string `json:"project_id"`
		EnvironmentId string `json:"environment_id"`
	}
)

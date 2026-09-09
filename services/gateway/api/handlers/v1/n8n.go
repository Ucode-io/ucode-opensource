package v1

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"github.com/Ucode-io/ucode-opensource/services/gateway/api/status_http"
	"github.com/Ucode-io/ucode-opensource/services/gateway/genproto/company_service"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/util"

	"github.com/gin-gonic/gin"
)

type WorkflowResponse struct {
	Data []Workflow `json:"data"`
}

type Workflow struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Active    bool   `json:"active"`
	CreatedAt string `json:"createdAt"`
	UpdatedAt string `json:"updatedAt"`
	Nodes     []Node `json:"nodes"`
}

type Node struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	WebhookID        string `json:"webhookId"`
	Disabled         bool   `json:"disabled"`
	NotesInFlow      bool   `json:"notesInFlow"`
	Notes            string `json:"notes"`
	Type             string `json:"type"`
	TypeVersion      int    `json:"typeVersion"`
	ExecuteOnce      bool   `json:"executeOnce"`
	AlwaysOutputData bool   `json:"alwaysOutputData"`
}

func (h *HandlerV1) GetWorkflows(c *gin.Context) {
	projectId, ok := c.Get("project_id")
	if !ok && !util.IsValidUUID(projectId.(string)) {
		h.HandleResponse(c, status_http.BadRequest, "project id is required")
		return
	}

	resp, err := h.companyServices.Project().GetById(
		c.Request.Context(),
		&company_service.GetProjectByIdRequest{
			ProjectId: projectId.(string),
		},
	)

	if err != nil {
		h.HandleResponse(c, status_http.GRPCError, err.Error())
		return
	}

	queryParams := map[string]string{
		"projectId": resp.ProjectId,
	}

	workflows, err := getWorkflows(
		h.baseConf.N8NApiKey,
		h.baseConf.N8NBaseURL,
		queryParams,
	)
	if err != nil {
		h.HandleResponse(c, status_http.InternalServerError, err.Error())
		return
	}

	h.HandleResponse(c, status_http.OK, workflows)
}

func getWorkflows(apiKey, baseURL string, queryParams map[string]string) (*WorkflowResponse, error) {
	// Build the URL with query parameters
	u, err := url.Parse(baseURL + "/workflows")
	if err != nil {
		return nil, err
	}

	q := u.Query()
	for key, value := range queryParams {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()

	// Create request
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}

	// Set API Key header
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read and decode JSON
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var result WorkflowResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	return &result, nil
}

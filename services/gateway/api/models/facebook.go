package models

import "fmt"

func (e *FacebookAPIError) Error() string {
	if e == nil {
		return ""
	}
	return fmt.Sprintf("facebook graph error %d (%s): %s", e.Code, e.Type, e.Message)
}

// Facebook (Meta) Lead Ads webhook payload models.
// Reference: https://developers.facebook.com/docs/graph-api/webhooks/reference/page/#leadgen

type (
	FacebookLeadWebhookEvent struct {
		Object string                 `json:"object"` // always "page" for Lead Ads
		Entry  []FacebookWebhookEntry `json:"entry"`
	}

	FacebookWebhookEntry struct {
		ID      string                  `json:"id"`   // Page ID
		Time    int64                   `json:"time"` // event time, unix seconds
		Changes []FacebookWebhookChange `json:"changes"`
	}

	FacebookWebhookChange struct {
		Field string                  `json:"field"` // "leadgen" for lead ads
		Value FacebookLeadChangeValue `json:"value"`
	}

	FacebookLeadChangeValue struct {
		LeadgenID   string `json:"leadgen_id"`
		PageID      string `json:"page_id"`
		FormID      string `json:"form_id"`
		AdgroupID   string `json:"adgroup_id"`
		AdID        string `json:"ad_id"`
		CreatedTime int64  `json:"created_time"` // unix seconds
	}
)

// Graph API responses (proxied to the frontend for the connect/mapping flow).
type (
	FacebookTokenResponse struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
	}

	FacebookUser struct {
		ID    string            `json:"id"`
		Name  string            `json:"name"`
		Error *FacebookAPIError `json:"error,omitempty"`
	}

	FacebookPageList struct {
		Data   []FacebookPage    `json:"data"`
		Paging FacebookPaging    `json:"paging"`
		Error  *FacebookAPIError `json:"error,omitempty"`
	}

	FacebookPage struct {
		ID          string   `json:"id"`
		Name        string   `json:"name"`
		AccessToken string   `json:"access_token"`
		Tasks       []string `json:"tasks"`
	}

	FacebookFormList struct {
		Data   []FacebookForm    `json:"data"`
		Paging FacebookPaging    `json:"paging"`
		Error  *FacebookAPIError `json:"error,omitempty"`
	}

	FacebookForm struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Status string `json:"status"`
		Locale string `json:"locale"`
	}

	FacebookFormQuestions struct {
		ID        string                 `json:"id"`
		Name      string                 `json:"name"`
		Status    string                 `json:"status"`
		Locale    string                 `json:"locale"`
		Questions []FacebookFormQuestion `json:"questions"`
		Error     *FacebookAPIError      `json:"error,omitempty"`
	}

	FacebookFormQuestion struct {
		Key     string                       `json:"key"`
		Label   string                       `json:"label"`
		Type    string                       `json:"type"`
		Options []FacebookFormQuestionOption `json:"options,omitempty"`
	}

	FacebookFormQuestionOption struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	}

	FacebookLead struct {
		ID          string              `json:"id"`
		CreatedTime string              `json:"created_time"`
		AdID        string              `json:"ad_id"`
		AdName      string              `json:"ad_name"`
		FormID      string              `json:"form_id"`
		Platform    string              `json:"platform"`
		IsOrganic   bool                `json:"is_organic"`
		FieldData   []FacebookFieldData `json:"field_data"`
		Error       *FacebookAPIError   `json:"error,omitempty"`
	}

	FacebookFieldData struct {
		Name   string   `json:"name"`
		Values []string `json:"values"`
	}

	FacebookPaging struct {
		Next     string `json:"next,omitempty"`
		Previous string `json:"previous,omitempty"`
	}

	FacebookAPIError struct {
		Message   string `json:"message"`
		Type      string `json:"type"`
		Code      int    `json:"code"`
		FbtraceID string `json:"fbtrace_id"`
	}

	FacebookTokenDebugResponse struct {
		Data FacebookTokenDebugData `json:"data"`
	}

	FacebookTokenDebugData struct {
		IsValid   bool              `json:"is_valid"`
		ExpiresAt int64             `json:"expires_at"` // unix seconds, 0 means it never expires
		Error     *FacebookAPIError `json:"error,omitempty"`
	}
)

// Gateway request/response contracts for the connect and mapping flow.
type (
	FacebookSubscribeRequest struct {
		PageId   string `json:"page_id" binding:"required"`
		PageName string `json:"page_name"`
	}

	FacebookMappingRequest struct {
		PageId string                `json:"page_id" binding:"required"`
		Forms  []FacebookFormMapping `json:"forms" binding:"required,dive"`
	}

	FacebookFormMapping struct {
		FormId    string                 `json:"form_id" binding:"required"`
		FormName  string                 `json:"form_name"`
		TableSlug string                 `json:"table_slug" binding:"required"`
		Fields    []FacebookFieldMapping `json:"fields" binding:"required,dive"`
	}

	FacebookFieldMapping struct {
		LeadField  string `json:"lead_field" binding:"required"`
		TableField string `json:"table_field" binding:"required"`
		Required   bool   `json:"required"`
	}

	// FacebookCrmMapping is the per-project mapping for the amoCRM-style flow:
	// which tables/relations a lead is written to and which pipeline/stage a new
	// deal lands in. Any blank field falls back to the platform default, so the
	// integration is not bound to one project's schema. Used for both the GET
	// (response) and PUT (request) of /v1/facebook/crm-mapping.
	FacebookCrmMapping struct {
		ContactsTable    string `json:"contacts_table"`
		DealsTable       string `json:"deals_table"`
		LeadFormsTable   string `json:"lead_forms_table"`
		DealContactField string `json:"deal_contact_field"`
		DealFormField    string `json:"deal_form_field"`
		PipelineField    string `json:"pipeline_field"`
		PipelineValue    string `json:"pipeline_value"`
		StageField       string `json:"stage_field"`
		StageValue       string `json:"stage_value"`
		SourceField      string `json:"source_field"`
	}

	// FacebookConnectionStatus reports whether the stored user token is still
	// active; Reason explains the inactive state when Active is false.
	FacebookConnectionStatus struct {
		Connected bool   `json:"connected"`
		Active    bool   `json:"active"`
		ExpiresAt int64  `json:"expires_at,omitempty"`
		Reason    string `json:"reason,omitempty"`
	}

	FacebookIntegrationStatus struct {
		Connected   bool                  `json:"connected"`
		ResourceId  string                `json:"resource_id,omitempty"`
		PageId      string                `json:"page_id,omitempty"`
		PageName    string                `json:"page_name,omitempty"`
		Status      string                `json:"status,omitempty"`
		ConnectedAt string                `json:"connected_at,omitempty"`
		Forms       []FacebookFormMapping `json:"forms,omitempty"`
	}

	// FacebookOAuthState is the project/environment/user scope carried through the
	// OAuth flow (serialized into the state token). RedirectURL is the exact page
	// the user started from; the callback returns them there with the result.
	FacebookOAuthState struct {
		ProjectId     string `json:"project_id"`
		EnvironmentId string `json:"environment_id"`
		UserId        string `json:"user_id"`
		RedirectURL   string `json:"redirect_url,omitempty"`
	}
)

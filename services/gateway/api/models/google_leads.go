package models

import (
	"bytes"
	"encoding/json"
)

// Google Lead Form Ads webhook payload and gateway request/response contracts.
// Reference: https://support.google.com/google-ads/answer/9423234 (webhook lead delivery)
//
// Unlike Meta, Google pushes the full lead data in the webhook body, so there is
// no second fetch. Authentication and routing both rely on google_key — a secret
// we generate and the advertiser pastes into the Google Ads lead form settings.

// FlexString accepts a JSON value that is either a quoted string or a bare number
// and stores it as a string. Google sends ids (form_id, campaign_id, ...) as
// numbers in test payloads and as strings in real ones, so we normalize to string.
type FlexString string

func (f *FlexString) UnmarshalJSON(data []byte) error {
	data = bytes.TrimSpace(data)
	if len(data) == 0 || string(data) == "null" {
		*f = ""
		return nil
	}
	if data[0] == '"' {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*f = FlexString(s)
		return nil
	}
	*f = FlexString(data)
	return nil
}

func (f FlexString) String() string { return string(f) }

type (
	GoogleLeadWebhookEvent struct {
		LeadID         string                 `json:"lead_id"`
		APIVersion     string                 `json:"api_version"`
		FormID         FlexString             `json:"form_id"`
		CampaignID     FlexString             `json:"campaign_id"`
		GclID          string                 `json:"gcl_id"`
		AdgroupID      FlexString             `json:"adgroup_id"`
		CreativeID     FlexString             `json:"creative_id"`
		IsTest         bool                   `json:"is_test"`
		GoogleKey      string                 `json:"google_key"`
		UserColumnData []GoogleLeadColumnData `json:"user_column_data"`
	}

	GoogleLeadColumnData struct {
		ColumnID    string `json:"column_id"`
		ColumnName  string `json:"column_name"`
		StringValue string `json:"string_value"`
	}
)

// Gateway request/response contracts for the connect and mapping flow.
type (
	GoogleLeadsCreateRequest struct {
		FormId    string                   `json:"form_id"`
		FormName  string                   `json:"form_name"`
		TableSlug string                   `json:"table_slug" binding:"required"`
		Fields    []GoogleLeadFieldMapping `json:"fields" binding:"required,dive"`
	}

	GoogleLeadsMappingRequest struct {
		FormId    string                   `json:"form_id"`
		FormName  string                   `json:"form_name"`
		TableSlug string                   `json:"table_slug" binding:"required"`
		Fields    []GoogleLeadFieldMapping `json:"fields" binding:"required,dive"`
	}

	GoogleLeadFieldMapping struct {
		LeadColumn string `json:"lead_column" binding:"required"`
		TableField string `json:"table_field" binding:"required"`
		Required   bool   `json:"required"`
	}

	GoogleLeadsIntegrationStatus struct {
		Connected   bool                     `json:"connected"`
		ResourceId  string                   `json:"resource_id,omitempty"`
		GoogleKey   string                   `json:"google_key,omitempty"`
		WebhookURL  string                   `json:"webhook_url,omitempty"`
		FormId      string                   `json:"form_id,omitempty"`
		FormName    string                   `json:"form_name,omitempty"`
		TableSlug   string                   `json:"table_slug,omitempty"`
		Status      string                   `json:"status,omitempty"`
		ConnectedAt string                   `json:"connected_at,omitempty"`
		Fields      []GoogleLeadFieldMapping `json:"fields,omitempty"`
	}

	// GoogleLeadColumn is one standard Google column_id offered to the UI when the
	// user builds the field mapping. Custom form questions use their own ids.
	GoogleLeadColumn struct {
		ColumnID string `json:"column_id"`
		Label    string `json:"label"`
	}
)

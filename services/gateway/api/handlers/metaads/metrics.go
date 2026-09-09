package metaads

import (
	"sort"
	"strconv"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

const metaLeadsCustomConversionAction = "offsite_complete_registration_add_meta_leads"

func aggregateMetrics(insights []graphInsight, leadActionTypes map[string]struct{}) models.MetaAdsMetrics {
	metrics := models.MetaAdsMetrics{}
	for _, insight := range insights {
		metrics.Spend += decimalValue(insight.Spend)
		metrics.Impressions += integerValue(insight.Impressions)
		metrics.Reach += integerValue(insight.Reach)
		metrics.Clicks += integerValue(insight.Clicks)
		metrics.LinkClicks += integerValue(insight.InlineLinkClicks)
		matchedConfiguredLeadAction := false
		metaLeadsCustomConversion := int64(0)
		for _, action := range insight.Actions {
			value := integerValue(action.Value)
			if _, ok := leadActionTypes[action.ActionType]; ok {
				metrics.Leads += value
				matchedConfiguredLeadAction = true
			}
			if action.ActionType == metaLeadsCustomConversionAction {
				metaLeadsCustomConversion = value
			}
			if action.ActionType == "landing_page_view" {
				metrics.LandingPageViews += value
			}
		}
		// Meta omits the generic `lead` action for the Target landing-page
		// funnel and reports the conversion under this account's dedicated
		// custom event instead. Use it only when none of the configured lead
		// actions is present, so historical rows that contain both are not
		// counted twice.
		if !matchedConfiguredLeadAction {
			metrics.Leads += metaLeadsCustomConversion
		}
	}
	if metrics.Reach > 0 {
		metrics.Frequency = float64(metrics.Impressions) / float64(metrics.Reach)
	}
	if metrics.Impressions > 0 {
		metrics.CTR = float64(metrics.Clicks) / float64(metrics.Impressions) * 100
		metrics.CPM = metrics.Spend / float64(metrics.Impressions) * 1000
	}
	if metrics.Clicks > 0 {
		metrics.CPC = metrics.Spend / float64(metrics.Clicks)
	}
	if metrics.Leads > 0 {
		cpl := metrics.Spend / float64(metrics.Leads)
		metrics.CPL = &cpl
	}
	return metrics
}

func observedActionTypes(groups ...[]graphInsight) []string {
	unique := make(map[string]struct{})
	for _, group := range groups {
		for _, insight := range group {
			for _, action := range insight.Actions {
				if action.ActionType != "" {
					unique[action.ActionType] = struct{}{}
				}
			}
		}
	}
	result := make([]string, 0, len(unique))
	for actionType := range unique {
		result = append(result, actionType)
	}
	sort.Strings(result)
	return result
}

func decimalValue(value string) float64 {
	parsed, _ := strconv.ParseFloat(value, 64)
	return parsed
}

func integerValue(value string) int64 {
	parsed, _ := strconv.ParseInt(value, 10, 64)
	return parsed
}

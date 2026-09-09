package metaads

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAggregateMetrics(t *testing.T) {
	metrics := aggregateMetrics([]graphInsight{{
		Spend:            "120.45",
		Impressions:      "50210",
		Reach:            "40100",
		Clicks:           "814",
		InlineLinkClicks: "700",
		Actions: []graphAction{
			{ActionType: "lead", Value: "37"},
			{ActionType: "landing_page_view", Value: "520"},
		},
	}}, map[string]struct{}{"lead": {}})

	require.InDelta(t, 120.45, metrics.Spend, 0.0001)
	require.Equal(t, int64(50210), metrics.Impressions)
	require.Equal(t, int64(37), metrics.Leads)
	require.Equal(t, int64(520), metrics.LandingPageViews)
	require.NotNil(t, metrics.CPL)
	require.InDelta(t, 120.45/37, *metrics.CPL, 0.0001)
	require.InDelta(t, float64(814)/50210*100, metrics.CTR, 0.0001)
}

func TestAggregateMetricsReturnsNilCPLWithoutLeads(t *testing.T) {
	metrics := aggregateMetrics([]graphInsight{{Spend: "12.50"}}, map[string]struct{}{"lead": {}})

	require.Nil(t, metrics.CPL)
}

func TestAggregateMetricsUsesLeadAllowlist(t *testing.T) {
	insight := graphInsight{Actions: []graphAction{
		{ActionType: "lead", Value: "10"},
		{ActionType: "onsite_conversion.lead_grouped", Value: "7"},
	}}

	metrics := aggregateMetrics([]graphInsight{insight}, map[string]struct{}{"onsite_conversion.lead_grouped": {}})

	require.Equal(t, int64(7), metrics.Leads)
}

func TestAggregateMetricsFallsBackToMetaLeadsCustomConversion(t *testing.T) {
	insight := graphInsight{Actions: []graphAction{
		{ActionType: metaLeadsCustomConversionAction, Value: "3"},
		{ActionType: "complete_registration", Value: "3"},
		{ActionType: "omni_complete_registration", Value: "3"},
	}}

	metrics := aggregateMetrics([]graphInsight{insight}, map[string]struct{}{"lead": {}})

	require.Equal(t, int64(3), metrics.Leads)
}

func TestAggregateMetricsPrefersConfiguredLeadActionWithoutDoubleCountingFallback(t *testing.T) {
	insight := graphInsight{Actions: []graphAction{
		{ActionType: "lead", Value: "5"},
		{ActionType: metaLeadsCustomConversionAction, Value: "3"},
	}}

	metrics := aggregateMetrics([]graphInsight{insight}, map[string]struct{}{"lead": {}})

	require.Equal(t, int64(5), metrics.Leads)
}

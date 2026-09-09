package metaads

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

func (c *graphClient) fetchAccount(ctx context.Context) (graphAccount, error) {
	var account graphAccount
	err := c.request(ctx, http.MethodGet, "act_"+c.accountID, url.Values{"fields": {accountFields}}, &account)
	return account, err
}

func (c *graphClient) fetchCampaigns(ctx context.Context) ([]graphCampaign, error) {
	return fetchAll[graphCampaign](ctx, c, "act_"+c.accountID+"/campaigns", url.Values{
		"fields": {campaignFields},
		"limit":  {"100"},
	})
}

func (c *graphClient) fetchAdSets(ctx context.Context) ([]graphAdSet, error) {
	return fetchAll[graphAdSet](ctx, c, "act_"+c.accountID+"/adsets", url.Values{
		"fields": {adSetFields},
		"limit":  {"100"},
	})
}

func (c *graphClient) fetchAds(ctx context.Context) ([]graphAd, error) {
	return fetchAll[graphAd](ctx, c, "act_"+c.accountID+"/ads", url.Values{
		"fields": {adFields},
		"limit":  {"100"},
	})
}

func (c *graphClient) fetchInsights(ctx context.Context, query dashboardQuery, level string, daily bool, breakdowns []string, attributionWindows []string) ([]graphInsight, error) {
	timeRange, _ := json.Marshal(map[string]string{
		"since": query.Since.Format("2006-01-02"),
		"until": query.Until.Format("2006-01-02"),
	})
	values := url.Values{
		"level":                           {level},
		"time_range":                      {string(timeRange)},
		"fields":                          {insightFields},
		"limit":                           {defaultPageLimit},
		"use_account_attribution_setting": {"true"},
	}
	if daily {
		values.Set("time_increment", "1")
	}
	if len(breakdowns) > 0 {
		values.Set("breakdowns", strings.Join(breakdowns, ","))
	}
	if filters := graphFilters(query); len(filters) > 0 {
		encoded, _ := json.Marshal(filters)
		values.Set("filtering", string(encoded))
	}
	if len(attributionWindows) > 0 {
		encoded, _ := json.Marshal(attributionWindows)
		values.Set("action_attribution_windows", string(encoded))
	}
	return fetchAll[graphInsight](ctx, c, "act_"+c.accountID+"/insights", values)
}

func graphFilters(query dashboardQuery) []graphFilter {
	filters := make([]graphFilter, 0, 11)
	appendFilter := func(field string, values []string) {
		if len(values) > 0 {
			filters = append(filters, graphFilter{Field: field, Operator: "IN", Value: values})
		}
	}
	appendFilter("campaign.id", oneValue(query.CampaignID))
	appendFilter("adset.id", oneValue(query.AdSetID))
	appendFilter("ad.id", oneValue(query.AdID))
	appendFilter("campaign.objective", query.Objectives)
	appendFilter("campaign.effective_status", query.EffectiveStatuses)
	appendFilter("age", query.Ages)
	appendFilter("gender", query.Genders)
	appendFilter("country", query.Countries)
	appendFilter("region", query.Regions)
	appendFilter("publisher_platform", query.PublisherPlatforms)
	appendFilter("platform_position", query.PlatformPositions)
	appendFilter("device_platform", query.DevicePlatforms)
	return filters
}

func oneValue(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

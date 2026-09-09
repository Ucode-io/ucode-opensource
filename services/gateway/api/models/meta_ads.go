package models

type MetaAdsDashboardResponse struct {
	GeneratedAt string                  `json:"generated_at"`
	Source      string                  `json:"source"`
	Stale       bool                    `json:"stale"`
	Warning     string                  `json:"warning,omitempty"`
	DateRange   MetaAdsDateRange        `json:"date_range"`
	Account     MetaAdsAccount          `json:"account"`
	Attribution MetaAdsAttribution      `json:"attribution"`
	KPIs        MetaAdsMetrics          `json:"kpis"`
	Trends      []MetaAdsTrendPoint     `json:"trends"`
	Campaigns   []MetaAdsCampaignNode   `json:"campaigns"`
	Breakdowns  []MetaAdsBreakdownGroup `json:"breakdowns"`
	ActionTypes []string                `json:"observed_action_types"`
}

type MetaAdsDateRange struct {
	Since string `json:"since"`
	Until string `json:"until"`
}

type MetaAdsAttribution struct {
	UseAccountSetting bool     `json:"use_account_setting"`
	Windows           []string `json:"windows,omitempty"`
}

type MetaAdsAccount struct {
	ID             string  `json:"id"`
	Name           string  `json:"name"`
	Status         int     `json:"status"`
	Currency       string  `json:"currency"`
	Timezone       string  `json:"timezone"`
	TimezoneID     int     `json:"timezone_id"`
	TimezoneOffset float64 `json:"timezone_offset_hours"`
}

type MetaAdsMetrics struct {
	Spend            float64  `json:"spend"`
	Impressions      int64    `json:"impressions"`
	Reach            int64    `json:"reach"`
	Frequency        float64  `json:"frequency"`
	Clicks           int64    `json:"clicks"`
	LinkClicks       int64    `json:"link_clicks"`
	LandingPageViews int64    `json:"landing_page_views"`
	Leads            int64    `json:"leads"`
	CTR              float64  `json:"ctr"`
	CPC              float64  `json:"cpc"`
	CPM              float64  `json:"cpm"`
	CPL              *float64 `json:"cpl"`
}

type MetaAdsTrendPoint struct {
	Date    string         `json:"date"`
	Metrics MetaAdsMetrics `json:"metrics"`
}

type MetaAdsCampaignNode struct {
	ID              string             `json:"id"`
	Name            string             `json:"name"`
	Status          string             `json:"status"`
	EffectiveStatus string             `json:"effective_status"`
	Objective       string             `json:"objective"`
	DailyBudget     float64            `json:"daily_budget"`
	LifetimeBudget  float64            `json:"lifetime_budget"`
	StartTime       string             `json:"start_time,omitempty"`
	StopTime        string             `json:"stop_time,omitempty"`
	Metrics         MetaAdsMetrics     `json:"metrics"`
	AdSets          []MetaAdsAdSetNode `json:"ad_sets"`
}

type MetaAdsAdSetNode struct {
	ID               string          `json:"id"`
	CampaignID       string          `json:"campaign_id"`
	Name             string          `json:"name"`
	Status           string          `json:"status"`
	EffectiveStatus  string          `json:"effective_status"`
	OptimizationGoal string          `json:"optimization_goal"`
	BillingEvent     string          `json:"billing_event"`
	DailyBudget      float64         `json:"daily_budget"`
	LifetimeBudget   float64         `json:"lifetime_budget"`
	StartTime        string          `json:"start_time,omitempty"`
	StopTime         string          `json:"stop_time,omitempty"`
	Metrics          MetaAdsMetrics  `json:"metrics"`
	Ads              []MetaAdsAdNode `json:"ads"`
}

type MetaAdsAdNode struct {
	ID              string           `json:"id"`
	CampaignID      string           `json:"campaign_id"`
	AdSetID         string           `json:"ad_set_id"`
	Name            string           `json:"name"`
	Status          string           `json:"status"`
	EffectiveStatus string           `json:"effective_status"`
	Metrics         MetaAdsMetrics   `json:"metrics"`
	Creative        *MetaAdsCreative `json:"creative,omitempty"`
}

type MetaAdsCreative struct {
	ID           string `json:"id"`
	Name         string `json:"name,omitempty"`
	Title        string `json:"title,omitempty"`
	Body         string `json:"body,omitempty"`
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ImageURL     string `json:"image_url,omitempty"`
}

type MetaAdsBreakdownGroup struct {
	Name string                `json:"name"`
	Rows []MetaAdsBreakdownRow `json:"rows"`
}

type MetaAdsBreakdownRow struct {
	Dimensions map[string]string `json:"dimensions"`
	Metrics    MetaAdsMetrics    `json:"metrics"`
}

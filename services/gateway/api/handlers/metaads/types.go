package metaads

import "time"

type dashboardQuery struct {
	Since              time.Time
	Until              time.Time
	CampaignID         string
	AdSetID            string
	AdID               string
	Objectives         []string
	EffectiveStatuses  []string
	Ages               []string
	Genders            []string
	Countries          []string
	Regions            []string
	PublisherPlatforms []string
	PlatformPositions  []string
	DevicePlatforms    []string
	Breakdowns         []string
}

type graphAccount struct {
	ID                string  `json:"id"`
	Name              string  `json:"name"`
	AccountStatus     int     `json:"account_status"`
	Currency          string  `json:"currency"`
	TimezoneName      string  `json:"timezone_name"`
	TimezoneID        int     `json:"timezone_id"`
	TimezoneOffsetUTC float64 `json:"timezone_offset_hours_utc"`
}

type graphCampaign struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Status          string `json:"status"`
	EffectiveStatus string `json:"effective_status"`
	Objective       string `json:"objective"`
	DailyBudget     string `json:"daily_budget"`
	LifetimeBudget  string `json:"lifetime_budget"`
	StartTime       string `json:"start_time"`
	StopTime        string `json:"stop_time"`
}

type graphAdSet struct {
	ID               string `json:"id"`
	Name             string `json:"name"`
	CampaignID       string `json:"campaign_id"`
	Status           string `json:"status"`
	EffectiveStatus  string `json:"effective_status"`
	DailyBudget      string `json:"daily_budget"`
	LifetimeBudget   string `json:"lifetime_budget"`
	OptimizationGoal string `json:"optimization_goal"`
	BillingEvent     string `json:"billing_event"`
	StartTime        string `json:"start_time"`
	StopTime         string `json:"stop_time"`
}

type graphAd struct {
	ID              string         `json:"id"`
	Name            string         `json:"name"`
	AdSetID         string         `json:"adset_id"`
	CampaignID      string         `json:"campaign_id"`
	Status          string         `json:"status"`
	EffectiveStatus string         `json:"effective_status"`
	Creative        *graphCreative `json:"creative"`
}

type graphCreative struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	ThumbnailURL string `json:"thumbnail_url"`
	ImageURL     string `json:"image_url"`
}

type graphAction struct {
	ActionType string `json:"action_type"`
	Value      string `json:"value"`
}

type graphInsight struct {
	CampaignID        string        `json:"campaign_id"`
	CampaignName      string        `json:"campaign_name"`
	AdSetID           string        `json:"adset_id"`
	AdSetName         string        `json:"adset_name"`
	AdID              string        `json:"ad_id"`
	AdName            string        `json:"ad_name"`
	Spend             string        `json:"spend"`
	Impressions       string        `json:"impressions"`
	Reach             string        `json:"reach"`
	Frequency         string        `json:"frequency"`
	Clicks            string        `json:"clicks"`
	InlineLinkClicks  string        `json:"inline_link_clicks"`
	CTR               string        `json:"ctr"`
	CPC               string        `json:"cpc"`
	CPM               string        `json:"cpm"`
	Actions           []graphAction `json:"actions"`
	DateStart         string        `json:"date_start"`
	DateStop          string        `json:"date_stop"`
	Age               string        `json:"age"`
	Gender            string        `json:"gender"`
	Country           string        `json:"country"`
	Region            string        `json:"region"`
	PublisherPlatform string        `json:"publisher_platform"`
	PlatformPosition  string        `json:"platform_position"`
	DevicePlatform    string        `json:"device_platform"`
}

type graphFilter struct {
	Field    string   `json:"field"`
	Operator string   `json:"operator"`
	Value    []string `json:"value"`
}

package metaads

const (
	accountFields  = "id,name,account_status,currency,timezone_name,timezone_id,timezone_offset_hours_utc"
	campaignFields = "id,name,status,effective_status,objective,daily_budget,lifetime_budget,start_time,stop_time"
	adSetFields    = "id,name,campaign_id,status,effective_status,daily_budget,lifetime_budget,optimization_goal,billing_event,start_time,stop_time"
	adFields       = "id,name,adset_id,campaign_id,status,effective_status,creative{id,name,title,body,thumbnail_url,image_url}"
	insightFields  = "campaign_id,campaign_name,adset_id,adset_name,ad_id,ad_name,spend,impressions,reach,frequency,clicks,inline_link_clicks,ctr,cpc,cpm,actions,cost_per_action_type,date_start,date_stop"

	defaultPageLimit = "500"
	maxGraphPages    = 1000
	maxResponseBytes = 20 << 20
)

func supportedBreakdowns() map[string][]string {
	return map[string][]string{
		"age_gender": {"age", "gender"},
		"country":    {"country"},
		"region":     {"region"},
		"placement":  {"publisher_platform", "platform_position"},
		"device":     {"device_platform"},
	}
}

func defaultBreakdownNames() []string {
	return []string{"age_gender", "country", "region", "placement", "device"}
}

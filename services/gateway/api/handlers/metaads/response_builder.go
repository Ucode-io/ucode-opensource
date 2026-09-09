package metaads

import (
	"sort"
	"strings"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
)

func (s *service) trends(insights []graphInsight) []models.MetaAdsTrendPoint {
	byDate := make(map[string][]graphInsight)
	for _, insight := range insights {
		byDate[insight.DateStart] = append(byDate[insight.DateStart], insight)
	}
	dates := make([]string, 0, len(byDate))
	for date := range byDate {
		dates = append(dates, date)
	}
	sort.Strings(dates)
	result := make([]models.MetaAdsTrendPoint, 0, len(dates))
	for _, date := range dates {
		result = append(result, models.MetaAdsTrendPoint{Date: date, Metrics: aggregateMetrics(byDate[date], s.leadActionTypes)})
	}
	return result
}

func (s *service) hierarchy(campaigns []graphCampaign, adSets []graphAdSet, ads []graphAd, campaignInsights, adSetInsights, adInsights []graphInsight) []models.MetaAdsCampaignNode {
	campaignByID := make(map[string]graphCampaign, len(campaigns))
	for _, campaign := range campaigns {
		campaignByID[campaign.ID] = campaign
	}
	adSetByID := make(map[string]graphAdSet, len(adSets))
	for _, adSet := range adSets {
		adSetByID[adSet.ID] = adSet
	}
	adByID := make(map[string]graphAd, len(ads))
	for _, ad := range ads {
		adByID[ad.ID] = ad
	}

	adNodes := make(map[string][]models.MetaAdsAdNode)
	for _, insight := range adInsights {
		ad := adByID[insight.AdID]
		node := models.MetaAdsAdNode{
			ID:              insight.AdID,
			CampaignID:      firstNonEmpty(ad.CampaignID, insight.CampaignID),
			AdSetID:         firstNonEmpty(ad.AdSetID, insight.AdSetID),
			Name:            firstNonEmpty(ad.Name, insight.AdName),
			Status:          ad.Status,
			EffectiveStatus: ad.EffectiveStatus,
			Metrics:         aggregateMetrics([]graphInsight{insight}, s.leadActionTypes),
			Creative:        creativeModel(ad.Creative),
		}
		adNodes[node.AdSetID] = append(adNodes[node.AdSetID], node)
	}

	adSetNodes := make(map[string][]models.MetaAdsAdSetNode)
	for _, insight := range adSetInsights {
		adSet := adSetByID[insight.AdSetID]
		node := models.MetaAdsAdSetNode{
			ID:               insight.AdSetID,
			CampaignID:       firstNonEmpty(adSet.CampaignID, insight.CampaignID),
			Name:             firstNonEmpty(adSet.Name, insight.AdSetName),
			Status:           adSet.Status,
			EffectiveStatus:  adSet.EffectiveStatus,
			OptimizationGoal: adSet.OptimizationGoal,
			BillingEvent:     adSet.BillingEvent,
			DailyBudget:      decimalValue(adSet.DailyBudget),
			LifetimeBudget:   decimalValue(adSet.LifetimeBudget),
			StartTime:        adSet.StartTime,
			StopTime:         adSet.StopTime,
			Metrics:          aggregateMetrics([]graphInsight{insight}, s.leadActionTypes),
			Ads:              adNodes[insight.AdSetID],
		}
		sort.Slice(node.Ads, func(i, j int) bool { return node.Ads[i].Metrics.Spend > node.Ads[j].Metrics.Spend })
		adSetNodes[node.CampaignID] = append(adSetNodes[node.CampaignID], node)
	}

	result := make([]models.MetaAdsCampaignNode, 0, len(campaignInsights))
	for _, insight := range campaignInsights {
		campaign := campaignByID[insight.CampaignID]
		node := models.MetaAdsCampaignNode{
			ID:              insight.CampaignID,
			Name:            firstNonEmpty(campaign.Name, insight.CampaignName),
			Status:          campaign.Status,
			EffectiveStatus: campaign.EffectiveStatus,
			Objective:       campaign.Objective,
			DailyBudget:     decimalValue(campaign.DailyBudget),
			LifetimeBudget:  decimalValue(campaign.LifetimeBudget),
			StartTime:       campaign.StartTime,
			StopTime:        campaign.StopTime,
			Metrics:         aggregateMetrics([]graphInsight{insight}, s.leadActionTypes),
			AdSets:          adSetNodes[insight.CampaignID],
		}
		sort.Slice(node.AdSets, func(i, j int) bool { return node.AdSets[i].Metrics.Spend > node.AdSets[j].Metrics.Spend })
		result = append(result, node)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Metrics.Spend > result[j].Metrics.Spend })
	return result
}

func accountModel(account graphAccount) models.MetaAdsAccount {
	return models.MetaAdsAccount{
		ID:             account.ID,
		Name:           account.Name,
		Status:         account.AccountStatus,
		Currency:       account.Currency,
		Timezone:       account.TimezoneName,
		TimezoneID:     account.TimezoneID,
		TimezoneOffset: account.TimezoneOffsetUTC,
	}
}

func creativeModel(creative *graphCreative) *models.MetaAdsCreative {
	if creative == nil {
		return nil
	}
	return &models.MetaAdsCreative{
		ID:           creative.ID,
		Name:         creative.Name,
		Title:        creative.Title,
		Body:         creative.Body,
		ThumbnailURL: creative.ThumbnailURL,
		ImageURL:     creative.ImageURL,
	}
}

func breakdownDimensions(insight graphInsight, fields []string) map[string]string {
	values := map[string]string{
		"age":                insight.Age,
		"gender":             insight.Gender,
		"country":            insight.Country,
		"region":             insight.Region,
		"publisher_platform": insight.PublisherPlatform,
		"platform_position":  insight.PlatformPosition,
		"device_platform":    insight.DevicePlatform,
	}
	result := make(map[string]string, len(fields))
	for _, field := range fields {
		result[field] = values[field]
	}
	return result
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

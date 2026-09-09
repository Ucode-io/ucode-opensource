package metaads

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"golang.org/x/sync/errgroup"
)

type graphDataClient interface {
	fetchAccount(ctx context.Context) (graphAccount, error)
	fetchCampaigns(ctx context.Context) ([]graphCampaign, error)
	fetchAdSets(ctx context.Context) ([]graphAdSet, error)
	fetchAds(ctx context.Context) ([]graphAd, error)
	fetchInsights(ctx context.Context, query dashboardQuery, level string, daily bool, breakdowns []string, attributionWindows []string) ([]graphInsight, error)
}

type service struct {
	client             graphDataClient
	cache              dashboardCache
	cacheTTL           time.Duration
	accountID          string
	leadActionTypes    map[string]struct{}
	attributionWindows []string
	log                logger.LoggerI
}

func newService(client graphDataClient, cache dashboardCache, cacheTTL time.Duration, accountID string, leadActionTypes, attributionWindows []string, log logger.LoggerI) *service {
	leads := make(map[string]struct{}, len(leadActionTypes))
	for _, actionType := range leadActionTypes {
		if actionType != "" {
			leads[actionType] = struct{}{}
		}
	}
	return &service{
		client:             client,
		cache:              cache,
		cacheTTL:           cacheTTL,
		accountID:          accountID,
		leadActionTypes:    leads,
		attributionWindows: append([]string(nil), attributionWindows...),
		log:                log,
	}
}

func (s *service) dashboard(ctx context.Context, query dashboardQuery, preferCache bool) (models.MetaAdsDashboardResponse, error) {
	startedAt := time.Now()
	key := dashboardCacheKey(s.accountID, query)
	cached, cacheFound, cacheErr := s.cache.Get(ctx, key)
	if cacheErr != nil {
		s.log.Warn("meta ads: read fallback cache failed", logger.Error(cacheErr))
	}
	if preferCache && cacheFound {
		if response, ok := cachedDashboard(cached, false); ok {
			s.updateLastSuccessfulCaches(ctx, cached, query)
			return response, nil
		}
	}
	if preferCache && !cacheFound {
		if fallback, ok := s.lastSuccessfulDashboard(ctx, query); ok {
			s.log.Warn("meta ads: serving last successful period before refresh",
				logger.String("requested_since", query.Since.Format("2006-01-02")),
				logger.String("requested_until", query.Until.Format("2006-01-02")),
				logger.String("fallback_since", fallback.DateRange.Since),
				logger.String("fallback_until", fallback.DateRange.Until),
			)
			return fallback, nil
		}
	}

	fresh, err := s.fetchDashboard(ctx, query)
	if err == nil {
		payload, marshalErr := json.Marshal(fresh)
		if marshalErr == nil {
			if setErr := s.cache.Set(ctx, key, payload, s.cacheTTL); setErr != nil {
				s.log.Warn("meta ads: update fallback cache failed", logger.Error(setErr))
			}
			s.updateLastSuccessfulCaches(ctx, payload, query)
		}
		s.log.Info("meta ads: dashboard refreshed",
			logger.String("since", query.Since.Format("2006-01-02")),
			logger.String("until", query.Until.Format("2006-01-02")),
			logger.Int("campaign_count", len(fresh.Campaigns)),
			logger.Int("breakdown_count", len(fresh.Breakdowns)),
			logger.String("latency", time.Since(startedAt).String()),
		)
		return fresh, nil
	}
	if cacheFound {
		if fallback, ok := cachedDashboard(cached, true); ok {
			s.log.Warn("meta ads: serving stale fallback", logger.Error(err))
			return fallback, nil
		}
	}
	if fallback, ok := s.lastSuccessfulDashboard(ctx, query); ok {
		s.log.Warn("meta ads: serving last successful period",
			logger.String("requested_since", query.Since.Format("2006-01-02")),
			logger.String("requested_until", query.Until.Format("2006-01-02")),
			logger.String("fallback_since", fallback.DateRange.Since),
			logger.String("fallback_until", fallback.DateRange.Until),
			logger.Error(err),
		)
		return fallback, nil
	}
	return models.MetaAdsDashboardResponse{}, err
}

func (s *service) lastSuccessfulDashboard(ctx context.Context, query dashboardQuery) (models.MetaAdsDashboardResponse, bool) {
	keys := []string{dashboardFallbackCacheKey(s.accountID), dashboardWideFallbackCacheKey(s.accountID)}
	if dashboardRangeDays(query) >= 14 {
		keys[0], keys[1] = keys[1], keys[0]
	}
	var fallback models.MetaAdsDashboardResponse
	for _, key := range keys {
		payload, found, err := s.cache.Get(ctx, key)
		if err != nil {
			s.log.Warn("meta ads: read last successful cache failed", logger.Error(err))
			continue
		}
		if !found {
			continue
		}
		var ok bool
		fallback, ok = cachedDashboard(payload, true)
		if ok {
			break
		}
	}
	if fallback.DateRange.Since == "" || fallback.DateRange.Until == "" {
		return models.MetaAdsDashboardResponse{}, false
	}
	fallback.Warning = fmt.Sprintf(
		"Meta API is temporarily unavailable for %s to %s; showing the last successful period %s to %s",
		query.Since.Format("2006-01-02"),
		query.Until.Format("2006-01-02"),
		fallback.DateRange.Since,
		fallback.DateRange.Until,
	)
	return fallback, true
}

func (s *service) updateLastSuccessfulCaches(ctx context.Context, payload []byte, query dashboardQuery) {
	if setErr := s.cache.Set(ctx, dashboardFallbackCacheKey(s.accountID), payload, s.fallbackCacheTTL()); setErr != nil {
		s.log.Warn("meta ads: update last successful cache failed", logger.Error(setErr))
	}
	if dashboardRangeDays(query) < 14 {
		return
	}
	if setErr := s.cache.Set(ctx, dashboardWideFallbackCacheKey(s.accountID), payload, s.fallbackCacheTTL()); setErr != nil {
		s.log.Warn("meta ads: update wide fallback cache failed", logger.Error(setErr))
	}
}

func dashboardRangeDays(query dashboardQuery) int {
	return int(query.Until.Sub(query.Since).Hours()/24) + 1
}

func (s *service) fallbackCacheTTL() time.Duration {
	const minimum = 7 * 24 * time.Hour
	if s.cacheTTL > minimum {
		return s.cacheTTL
	}
	return minimum
}

func cachedDashboard(payload []byte, stale bool) (models.MetaAdsDashboardResponse, bool) {
	var response models.MetaAdsDashboardResponse
	if json.Unmarshal(payload, &response) != nil {
		return models.MetaAdsDashboardResponse{}, false
	}
	response.Source = "cache"
	response.Stale = stale
	response.Warning = ""
	if stale {
		response.Warning = "Meta API is temporarily unavailable; returning the last successful response"
	}
	return response, true
}

func (s *service) account(ctx context.Context) (models.MetaAdsAccount, error) {
	account, err := s.client.fetchAccount(ctx)
	if err != nil {
		return models.MetaAdsAccount{}, err
	}
	return accountModel(account), nil
}

func (s *service) fetchDashboard(ctx context.Context, query dashboardQuery) (models.MetaAdsDashboardResponse, error) {
	var (
		account             graphAccount
		campaigns           []graphCampaign
		adSets              []graphAdSet
		ads                 []graphAd
		accountInsights     []graphInsight
		dailyInsights       []graphInsight
		campaignInsights    []graphInsight
		adSetInsights       []graphInsight
		adInsights          []graphInsight
		accountErr          error
		campaignsErr        error
		adSetsErr           error
		adsErr              error
		accountInsightsErr  error
		dailyInsightsErr    error
		campaignInsightsErr error
		adSetInsightsErr    error
		adInsightsErr       error
	)

	// A dashboard is composed from independent Meta datasets. Do not cancel all
	// useful data when an optional hierarchy request fails or gets throttled.
	// Account metadata and account-level insights are the only critical pieces;
	// all other datasets degrade independently and are reported in Warning.
	group := new(errgroup.Group)
	group.SetLimit(4)
	group.Go(func() error { account, accountErr = s.client.fetchAccount(ctx); return nil })
	group.Go(func() error { campaigns, campaignsErr = s.client.fetchCampaigns(ctx); return nil })
	group.Go(func() error { adSets, adSetsErr = s.client.fetchAdSets(ctx); return nil })
	group.Go(func() error { ads, adsErr = s.client.fetchAds(ctx); return nil })
	group.Go(func() error {
		accountInsights, accountInsightsErr = s.client.fetchInsights(ctx, query, "account", false, nil, s.attributionWindows)
		return nil
	})
	group.Go(func() error {
		dailyInsights, dailyInsightsErr = s.client.fetchInsights(ctx, query, "account", true, nil, s.attributionWindows)
		return nil
	})
	group.Go(func() error {
		campaignInsights, campaignInsightsErr = s.client.fetchInsights(ctx, query, "campaign", false, nil, s.attributionWindows)
		return nil
	})
	group.Go(func() error {
		adSetInsights, adSetInsightsErr = s.client.fetchInsights(ctx, query, "adset", false, nil, s.attributionWindows)
		return nil
	})
	group.Go(func() error {
		adInsights, adInsightsErr = s.client.fetchInsights(ctx, query, "ad", false, nil, s.attributionWindows)
		return nil
	})
	_ = group.Wait()

	if accountErr != nil {
		return models.MetaAdsDashboardResponse{}, fmt.Errorf("fetch Meta account: %w", accountErr)
	}
	if accountInsightsErr != nil {
		return models.MetaAdsDashboardResponse{}, fmt.Errorf("fetch Meta account insights: %w", accountInsightsErr)
	}

	unavailableDatasets := make([]string, 0, 7)
	optionalErrors := []struct {
		name string
		err  error
	}{
		{name: "campaign metadata", err: campaignsErr},
		{name: "ad set metadata", err: adSetsErr},
		{name: "ad metadata", err: adsErr},
		{name: "daily trends", err: dailyInsightsErr},
		{name: "campaign metrics", err: campaignInsightsErr},
		{name: "ad set metrics", err: adSetInsightsErr},
		{name: "ad metrics", err: adInsightsErr},
	}
	for _, item := range optionalErrors {
		if item.err == nil {
			continue
		}
		unavailableDatasets = append(unavailableDatasets, item.name)
		s.log.Warn("meta ads: optional dataset unavailable",
			logger.String("dataset", item.name),
			logger.Error(item.err),
		)
	}

	breakdowns, unavailableBreakdowns := s.fetchBreakdowns(ctx, query)
	response := models.MetaAdsDashboardResponse{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Source:      "meta",
		Stale:       false,
		DateRange: models.MetaAdsDateRange{
			Since: query.Since.Format("2006-01-02"),
			Until: query.Until.Format("2006-01-02"),
		},
		Account: accountModel(account),
		Attribution: models.MetaAdsAttribution{
			UseAccountSetting: true,
			Windows:           append([]string(nil), s.attributionWindows...),
		},
		KPIs:        aggregateMetrics(accountInsights, s.leadActionTypes),
		Trends:      s.trends(dailyInsights),
		Campaigns:   s.hierarchy(campaigns, adSets, ads, campaignInsights, adSetInsights, adInsights),
		Breakdowns:  breakdowns,
		ActionTypes: observedActionTypes(accountInsights, dailyInsights, campaignInsights, adSetInsights, adInsights),
	}
	warnings := make([]string, 0, 2)
	if len(unavailableDatasets) > 0 {
		warnings = append(warnings, "Unavailable datasets: "+strings.Join(unavailableDatasets, ", "))
	}
	if len(unavailableBreakdowns) > 0 {
		warnings = append(warnings, "Unavailable breakdowns: "+strings.Join(unavailableBreakdowns, ", "))
	}
	response.Warning = strings.Join(warnings, "; ")
	return response, nil
}

func (s *service) fetchBreakdowns(ctx context.Context, query dashboardQuery) ([]models.MetaAdsBreakdownGroup, []string) {
	supported := supportedBreakdowns()
	result := make([]models.MetaAdsBreakdownGroup, len(query.Breakdowns))
	unavailable := make([]string, 0)
	group, groupCtx := errgroup.WithContext(ctx)
	group.SetLimit(3)
	var lock sync.Mutex
	for index, name := range query.Breakdowns {
		index, name := index, name
		group.Go(func() error {
			insights, err := s.client.fetchInsights(groupCtx, query, "account", false, supported[name], s.attributionWindows)
			if err != nil {
				s.log.Warn("meta ads: breakdown unavailable", logger.String("breakdown", name), logger.Error(err))
				lock.Lock()
				unavailable = append(unavailable, name)
				lock.Unlock()
				return nil
			}
			rows := make([]models.MetaAdsBreakdownRow, 0, len(insights))
			for _, insight := range insights {
				rows = append(rows, models.MetaAdsBreakdownRow{
					Dimensions: breakdownDimensions(insight, supported[name]),
					Metrics:    aggregateMetrics([]graphInsight{insight}, s.leadActionTypes),
				})
			}
			lock.Lock()
			result[index] = models.MetaAdsBreakdownGroup{Name: name, Rows: rows}
			lock.Unlock()
			return nil
		})
	}
	_ = group.Wait()
	filtered := result[:0]
	for _, item := range result {
		if item.Name != "" {
			filtered = append(filtered, item)
		}
	}
	sort.Strings(unavailable)
	return filtered, unavailable
}

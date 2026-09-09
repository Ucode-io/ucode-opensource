package metaads

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Ucode-io/ucode-opensource/services/gateway/api/models"
	"github.com/Ucode-io/ucode-opensource/services/gateway/pkg/logger"

	"github.com/stretchr/testify/require"
)

type fakeGraphClient struct {
	lock          sync.Mutex
	accountCalls  int
	err           error
	failOperation string
}

func (c *fakeGraphClient) operationError(operation string) error {
	if c.err != nil {
		return c.err
	}
	if c.failOperation == operation {
		return errors.New(operation + " unavailable")
	}
	return nil
}

func (c *fakeGraphClient) fetchAccount(context.Context) (graphAccount, error) {
	c.lock.Lock()
	c.accountCalls++
	c.lock.Unlock()
	return graphAccount{ID: "act_1", Name: "Ads", Currency: "USD", TimezoneName: "Asia/Tashkent"}, c.operationError("account-metadata")
}

func (c *fakeGraphClient) fetchCampaigns(context.Context) ([]graphCampaign, error) {
	return []graphCampaign{{ID: "campaign-1", Name: "Campaign"}}, c.operationError("campaigns")
}

func (c *fakeGraphClient) fetchAdSets(context.Context) ([]graphAdSet, error) {
	return []graphAdSet{{ID: "adset-1", CampaignID: "campaign-1", Name: "Ad set"}}, c.operationError("adsets")
}

func (c *fakeGraphClient) fetchAds(context.Context) ([]graphAd, error) {
	return []graphAd{{ID: "ad-1", AdSetID: "adset-1", CampaignID: "campaign-1", Name: "Ad"}}, c.operationError("ads")
}

func (c *fakeGraphClient) fetchInsights(_ context.Context, _ dashboardQuery, level string, daily bool, breakdowns []string, _ []string) ([]graphInsight, error) {
	operation := "insights-" + level
	if daily {
		operation += "-daily"
	}
	if len(breakdowns) > 0 {
		operation = "breakdown"
	}
	if err := c.operationError(operation); err != nil {
		return nil, err
	}
	insight := graphInsight{
		CampaignID:       "campaign-1",
		CampaignName:     "Campaign",
		AdSetID:          "adset-1",
		AdSetName:        "Ad set",
		AdID:             "ad-1",
		AdName:           "Ad",
		Spend:            "10",
		Impressions:      "1000",
		Reach:            "800",
		Clicks:           "50",
		InlineLinkClicks: "40",
		DateStart:        "2026-08-01",
		Actions:          []graphAction{{ActionType: "lead", Value: "5"}},
	}
	if len(breakdowns) > 0 {
		insight.Age = "25-34"
		insight.Gender = "male"
	}
	if daily || level != "" {
		return []graphInsight{insight}, nil
	}
	return nil, nil
}

type memoryDashboardCache struct {
	lock     sync.Mutex
	value    []byte
	found    bool
	setCalls int
}

type keyedMemoryDashboardCache struct {
	lock   sync.Mutex
	values map[string][]byte
}

func (c *keyedMemoryDashboardCache) Get(_ context.Context, key string) ([]byte, bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	value, found := c.values[key]
	return append([]byte(nil), value...), found, nil
}

func (c *keyedMemoryDashboardCache) Set(_ context.Context, key string, value []byte, _ time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	if c.values == nil {
		c.values = make(map[string][]byte)
	}
	c.values[key] = append([]byte(nil), value...)
	return nil
}

func (c *memoryDashboardCache) Get(context.Context, string) ([]byte, bool, error) {
	c.lock.Lock()
	defer c.lock.Unlock()
	return append([]byte(nil), c.value...), c.found, nil
}

func (c *memoryDashboardCache) Set(_ context.Context, _ string, value []byte, _ time.Duration) error {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.value = append([]byte(nil), value...)
	c.found = true
	c.setCalls++
	return nil
}

func TestDashboardAlwaysFetchesFreshDataAndUpdatesFallback(t *testing.T) {
	client := &fakeGraphClient{}
	cache := &memoryDashboardCache{found: true, value: []byte(`{"source":"cache","stale":true}`)}
	service := newService(client, cache, time.Minute, "1", []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))
	query := dashboardQuery{
		Since:      time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until:      time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
		Breakdowns: []string{"age_gender"},
	}

	first, err := service.dashboard(context.Background(), query, false)
	require.NoError(t, err)
	second, err := service.dashboard(context.Background(), query, false)
	require.NoError(t, err)

	require.Equal(t, "meta", first.Source)
	require.False(t, first.Stale)
	require.Equal(t, int64(5), first.KPIs.Leads)
	require.Len(t, first.Campaigns, 1)
	require.Equal(t, "meta", second.Source)
	require.Equal(t, 2, client.accountCalls)
	require.Equal(t, 4, cache.setCalls)
}

func TestDashboardReturnsCacheBeforeRefreshWhenPreferred(t *testing.T) {
	cached := models.MetaAdsDashboardResponse{
		GeneratedAt: "2026-08-01T00:00:00Z",
		Source:      "meta",
		Stale:       true,
		Warning:     "old warning",
		KPIs:        models.MetaAdsMetrics{Spend: 15},
	}
	payload, err := json.Marshal(cached)
	require.NoError(t, err)
	client := &fakeGraphClient{}
	cache := &memoryDashboardCache{found: true, value: payload}
	service := newService(client, cache, time.Minute, "1", []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	response, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
	}, true)

	require.NoError(t, err)
	require.Equal(t, "cache", response.Source)
	require.False(t, response.Stale)
	require.Empty(t, response.Warning)
	require.Equal(t, 15.0, response.KPIs.Spend)
	require.Zero(t, client.accountCalls)
	require.Equal(t, 1, cache.setCalls)
}

func TestDashboardReturnsCachedResponseWhenMetaFails(t *testing.T) {
	cached := models.MetaAdsDashboardResponse{GeneratedAt: "2026-08-01T00:00:00Z", Source: "meta", KPIs: models.MetaAdsMetrics{Spend: 15}}
	payload, err := json.Marshal(cached)
	require.NoError(t, err)
	client := &fakeGraphClient{err: errors.New("meta unavailable")}
	cache := &memoryDashboardCache{found: true, value: payload}
	service := newService(client, cache, time.Minute, "1", []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	response, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
	}, false)

	require.NoError(t, err)
	require.Equal(t, "cache", response.Source)
	require.True(t, response.Stale)
	require.Equal(t, 15.0, response.KPIs.Spend)
	require.NotEmpty(t, response.Warning)
}

func TestDashboardKeepsCoreMetricsWhenOptionalDatasetFails(t *testing.T) {
	client := &fakeGraphClient{failOperation: "insights-ad"}
	cache := &memoryDashboardCache{}
	service := newService(client, cache, time.Minute, "1", []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	response, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
	}, false)

	require.NoError(t, err)
	require.Equal(t, "meta", response.Source)
	require.Equal(t, int64(5), response.KPIs.Leads)
	require.Len(t, response.Campaigns, 1)
	require.Contains(t, response.Warning, "ad metrics")
	require.Equal(t, 2, cache.setCalls)
}

func TestDashboardReturnsLastSuccessfulPeriodWhenRequestedRangeIsNotCached(t *testing.T) {
	accountID := "1"
	cached := models.MetaAdsDashboardResponse{
		GeneratedAt: "2026-08-24T00:00:00Z",
		Source:      "meta",
		DateRange: models.MetaAdsDateRange{
			Since: "2026-08-03",
			Until: "2026-08-24",
		},
		KPIs: models.MetaAdsMetrics{Spend: 950.88},
	}
	payload, err := json.Marshal(cached)
	require.NoError(t, err)
	cache := &keyedMemoryDashboardCache{values: map[string][]byte{
		dashboardFallbackCacheKey(accountID): payload,
	}}
	client := &fakeGraphClient{err: errors.New("meta unavailable")}
	service := newService(client, cache, time.Minute, accountID, []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	response, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	}, true)

	require.NoError(t, err)
	require.Equal(t, "cache", response.Source)
	require.True(t, response.Stale)
	require.Equal(t, "2026-08-03", response.DateRange.Since)
	require.Equal(t, "2026-08-24", response.DateRange.Until)
	require.Equal(t, 950.88, response.KPIs.Spend)
	require.Contains(t, response.Warning, "2026-08-01 to 2026-08-31")
	require.Contains(t, response.Warning, "2026-08-03 to 2026-08-24")
	require.Zero(t, client.accountCalls)
}

func TestDashboardReturnsLastSuccessfulPeriodAfterFreshRequestFails(t *testing.T) {
	accountID := "1"
	cached := models.MetaAdsDashboardResponse{
		Source: "meta",
		DateRange: models.MetaAdsDateRange{
			Since: "2026-08-03",
			Until: "2026-08-24",
		},
		KPIs: models.MetaAdsMetrics{Spend: 950.88},
	}
	payload, err := json.Marshal(cached)
	require.NoError(t, err)
	cache := &keyedMemoryDashboardCache{values: map[string][]byte{
		dashboardFallbackCacheKey(accountID): payload,
	}}
	client := &fakeGraphClient{err: errors.New("meta unavailable")}
	service := newService(client, cache, time.Minute, accountID, []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	response, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	}, false)

	require.NoError(t, err)
	require.Equal(t, "cache", response.Source)
	require.True(t, response.Stale)
	require.Equal(t, 950.88, response.KPIs.Spend)
	require.Equal(t, 1, client.accountCalls)
}

func TestDashboardPrefersWideSnapshotForWideRequestedRange(t *testing.T) {
	accountID := "1"
	encode := func(since, until string, spend float64) []byte {
		payload, err := json.Marshal(models.MetaAdsDashboardResponse{
			Source: "meta",
			DateRange: models.MetaAdsDateRange{
				Since: since,
				Until: until,
			},
			KPIs: models.MetaAdsMetrics{Spend: spend},
		})
		require.NoError(t, err)
		return payload
	}
	cache := &keyedMemoryDashboardCache{values: map[string][]byte{
		dashboardFallbackCacheKey(accountID):     encode("2026-08-24", "2026-08-31", 241.86),
		dashboardWideFallbackCacheKey(accountID): encode("2026-08-03", "2026-08-24", 950.88),
	}}
	client := &fakeGraphClient{err: errors.New("meta unavailable")}
	service := newService(client, cache, time.Minute, accountID, []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	response, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
	}, true)

	require.NoError(t, err)
	require.Equal(t, "2026-08-03", response.DateRange.Since)
	require.Equal(t, "2026-08-24", response.DateRange.Until)
	require.Equal(t, 950.88, response.KPIs.Spend)
	require.Zero(t, client.accountCalls)
}

func TestDashboardStillFailsWhenCoreInsightsFail(t *testing.T) {
	client := &fakeGraphClient{failOperation: "insights-account"}
	cache := &memoryDashboardCache{}
	service := newService(client, cache, time.Minute, "1", []string{"lead"}, nil, logger.NewLogger("test", logger.LevelError))

	_, err := service.dashboard(context.Background(), dashboardQuery{
		Since: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		Until: time.Date(2026, 8, 7, 0, 0, 0, 0, time.UTC),
	}, false)

	require.Error(t, err)
}

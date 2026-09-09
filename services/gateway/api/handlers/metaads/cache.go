package metaads

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	go_redis "github.com/go-redis/redis/v8"
)

type dashboardCache interface {
	Get(ctx context.Context, key string) ([]byte, bool, error)
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
}

type redisDashboardCache struct {
	client *go_redis.Client
}

func (c redisDashboardCache) Get(ctx context.Context, key string) ([]byte, bool, error) {
	if c.client == nil {
		return nil, false, nil
	}
	value, err := c.client.Get(ctx, key).Bytes()
	if err == go_redis.Nil {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return value, true, nil
}

func (c redisDashboardCache) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	if c.client == nil {
		return nil
	}
	return c.client.Set(ctx, key, value, ttl).Err()
}

func dashboardCacheKey(accountID string, query dashboardQuery) string {
	canonical := struct {
		Since             string   `json:"since"`
		Until             string   `json:"until"`
		CampaignID        string   `json:"campaign_id"`
		AdSetID           string   `json:"ad_set_id"`
		AdID              string   `json:"ad_id"`
		Objectives        []string `json:"objectives"`
		EffectiveStatuses []string `json:"effective_statuses"`
		Ages              []string `json:"ages"`
		Genders           []string `json:"genders"`
		Countries         []string `json:"countries"`
		Regions           []string `json:"regions"`
		Publishers        []string `json:"publishers"`
		Positions         []string `json:"positions"`
		Devices           []string `json:"devices"`
		Breakdowns        []string `json:"breakdowns"`
	}{
		Since:             query.Since.Format("2006-01-02"),
		Until:             query.Until.Format("2006-01-02"),
		CampaignID:        query.CampaignID,
		AdSetID:           query.AdSetID,
		AdID:              query.AdID,
		Objectives:        sortedCopy(query.Objectives),
		EffectiveStatuses: sortedCopy(query.EffectiveStatuses),
		Ages:              sortedCopy(query.Ages),
		Genders:           sortedCopy(query.Genders),
		Countries:         sortedCopy(query.Countries),
		Regions:           sortedCopy(query.Regions),
		Publishers:        sortedCopy(query.PublisherPlatforms),
		Positions:         sortedCopy(query.PlatformPositions),
		Devices:           sortedCopy(query.DevicePlatforms),
		Breakdowns:        sortedCopy(query.Breakdowns),
	}
	payload, _ := json.Marshal(canonical)
	hash := sha256.Sum256(payload)
	return "meta-ads:dashboard:" + accountID + ":" + hex.EncodeToString(hash[:])
}

func dashboardFallbackCacheKey(accountID string) string {
	return "meta-ads:dashboard:" + accountID + ":last-successful"
}

func dashboardWideFallbackCacheKey(accountID string) string {
	return "meta-ads:dashboard:" + accountID + ":last-successful-wide"
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}

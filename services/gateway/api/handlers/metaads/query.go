package metaads

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func parseDashboardQuery(c *gin.Context, maxRangeDays int) (dashboardQuery, error) {
	now := time.Now().UTC()
	until := now
	since := now.AddDate(0, 0, -6)
	var err error
	if raw := strings.TrimSpace(c.Query("since")); raw != "" {
		since, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return dashboardQuery{}, fmt.Errorf("since must use YYYY-MM-DD")
		}
	}
	if raw := strings.TrimSpace(c.Query("until")); raw != "" {
		until, err = time.Parse("2006-01-02", raw)
		if err != nil {
			return dashboardQuery{}, fmt.Errorf("until must use YYYY-MM-DD")
		}
	}
	if until.Before(since) {
		return dashboardQuery{}, fmt.Errorf("until must be on or after since")
	}
	if maxRangeDays <= 0 {
		maxRangeDays = 90
	}
	if days := int(until.Sub(since).Hours()/24) + 1; days > maxRangeDays {
		return dashboardQuery{}, fmt.Errorf("date range cannot exceed %d days", maxRangeDays)
	}

	// Breakdowns are expensive, independent Meta Insights requests. Keep the
	// default dashboard request lean and fetch them only when a client asks for
	// specific breakdowns.
	breakdowns := commaValues(c.Query("breakdowns"))
	supported := supportedBreakdowns()
	for _, name := range breakdowns {
		if _, ok := supported[name]; !ok {
			return dashboardQuery{}, fmt.Errorf("unsupported breakdown %q", name)
		}
	}

	return dashboardQuery{
		Since:              since,
		Until:              until,
		CampaignID:         strings.TrimSpace(c.Query("campaign_id")),
		AdSetID:            strings.TrimSpace(c.Query("ad_set_id")),
		AdID:               strings.TrimSpace(c.Query("ad_id")),
		Objectives:         commaValues(c.Query("objective")),
		EffectiveStatuses:  commaValues(c.Query("effective_status")),
		Ages:               commaValues(c.Query("age")),
		Genders:            commaValues(c.Query("gender")),
		Countries:          commaValues(c.Query("country")),
		Regions:            commaValues(c.Query("region")),
		PublisherPlatforms: commaValues(c.Query("publisher_platform")),
		PlatformPositions:  commaValues(c.Query("platform_position")),
		DevicePlatforms:    commaValues(c.Query("device_platform")),
		Breakdowns:         breakdowns,
	}, nil
}

func commaValues(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	seen := make(map[string]struct{}, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if _, ok := seen[part]; ok {
			continue
		}
		seen[part] = struct{}{}
		result = append(result, part)
	}
	return result
}

package metaads

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestParseDashboardQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/?since=2026-08-01&until=2026-08-20&campaign_id=123&gender=male,female&breakdowns=age_gender,country", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := parseDashboardQuery(context, 90)

	require.NoError(t, err)
	require.Equal(t, "123", query.CampaignID)
	require.Equal(t, []string{"male", "female"}, query.Genders)
	require.Equal(t, []string{"age_gender", "country"}, query.Breakdowns)
}

func TestParseDashboardQueryDoesNotFetchBreakdownsByDefault(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/?since=2026-08-01&until=2026-08-20", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	query, err := parseDashboardQuery(context, 90)

	require.NoError(t, err)
	require.Empty(t, query.Breakdowns)
}

func TestParseDashboardQueryRejectsLargeRanges(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/?since=2026-01-01&until=2026-08-20", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseDashboardQuery(context, 90)

	require.EqualError(t, err, "date range cannot exceed 90 days")
}

func TestParseDashboardQueryAllowsFullYear(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/?since=2026-01-01&until=2026-12-31", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseDashboardQuery(context, 366)

	require.NoError(t, err)
}

func TestParseDashboardQueryRejectsUnknownBreakdown(t *testing.T) {
	gin.SetMode(gin.TestMode)
	request := httptest.NewRequest("GET", "/?breakdowns=unsupported", nil)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = request

	_, err := parseDashboardQuery(context, 90)

	require.EqualError(t, err, `unsupported breakdown "unsupported"`)
}

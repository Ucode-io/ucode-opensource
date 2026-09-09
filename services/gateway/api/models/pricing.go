package models

type PricingUsage struct {
	Current float64 `json:"current"`
	Limit   float64 `json:"limit"`
	Unit    string  `json:"unit"`
}

type AllPricingUsage struct {
	Functions       PricingUsage `json:"functions"`
	Microfrontend   PricingUsage `json:"microfrontend"`
	AssetSize       PricingUsage `json:"asset_size"`
	DatabaseSize    PricingUsage `json:"database_size"`
	Users           PricingUsage `json:"users"`
	Items           PricingUsage `json:"items"`
	Tables          PricingUsage `json:"tables"`
	ApiKeys         PricingUsage `json:"api_keys"`
	TodayTokens     PricingUsage `json:"today_tokens"`
	MonthlyTokens   PricingUsage `json:"monthly_tokens"`
	MonthlyApiCalls PricingUsage `json:"monthly_api_calls"`
	AvgResponseTime PricingUsage `json:"avg_response_time"`
	Projects        PricingUsage `json:"projects"`
}

type PerformanceMetricsResponse struct {
	AverageResponseTime float64 `json:"average_response_time"`
	ErrorRate           float64 `json:"error_rate"`
}

type TokenUsage struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
}

type TokenUsageResponse struct {
	Today   TokenUsage `json:"today"`
	Monthly TokenUsage `json:"monthly"`
}

type ApiMetricsResponse struct {
	Rps          float64 `json:"rps"`
	Rpm          int64   `json:"rpm"`
	Rph          int64   `json:"rph"`
	TodayCalls   int64   `json:"today_calls"`
	MonthlyCalls int64   `json:"monthly_calls"`
	LastDayCalls int64   `json:"last_day_calls"`
}

type ApiChartResponse struct {
	Chart []DailyChartPoint `json:"chart"`
}

type DailyChartPoint struct {
	Date  string `json:"date"`
	Count int64  `json:"count"`
}

type CompanyStat struct {
	Current int32 `json:"current"`
	Limit   int32 `json:"limit"`
}

type CompanyTokenStat struct {
	InputTokens  int64 `json:"input_tokens"`
	OutputTokens int64 `json:"output_tokens"`
	// PlanTokens is the portion of usage charged against the fare limit; it excludes
	// pack-funded tokens, so PlanTokens vs Limit is the true fare-budget position.
	PlanTokens   int64 `json:"plan_tokens"`
	Limit        int64 `json:"limit"`
	LimitReached bool  `json:"limit_reached"`
}

type CompanyTokenStats struct {
	Daily   CompanyTokenStat `json:"daily"`
	Monthly CompanyTokenStat `json:"monthly"`
	// PackRemaining is the company's remaining add-on token-pack pool. When a fare
	// limit is reached, usage is automatically funded from this pool.
	PackRemaining int64 `json:"pack_remaining"`
	// ActiveSource is the bucket the next tokens will draw from: "plan" while the
	// fare budget has room, "pack" once a fare limit is reached and the pool has
	// tokens, or "exhausted" when both are spent.
	ActiveSource string `json:"active_source"`
}

type CompanyStatsResponse struct {
	Tokens       CompanyTokenStats `json:"tokens"`
	ProjectCount CompanyStat       `json:"project_count"`
	Builders     CompanyStat       `json:"builders"`
	UserCount    CompanyStat       `json:"user_count"`
}

const (
	PaymentRequiredType = "payment_required"

	PaymentCodeDatabaseLimit   = "database_limit"
	PaymentCodeAssetLimit      = "asset_limit"
	PaymentCodeTableLimit      = "table_limit"
	PaymentCodeApiCallLimit    = "api_call_limit"
	PaymentCodeTokenDayLimit   = "token_day_limit"
	PaymentCodeTokenMonthLimit = "token_month_limit"

	PaymentUnitMB       = "mb"
	PaymentUnitTables   = "tables"
	PaymentUnitRequests = "requests"
	PaymentUnitTokens   = "tokens"
)

type PaymentRequiredData struct {
	Type string `json:"type"`           // always PaymentRequiredType
	Code string `json:"code"`           // PaymentCode* constants
	Unit string `json:"unit,omitempty"` // PaymentUnit* constants
}

// Predefined sentinels — use these instead of inline literals.
var (
	PaymentDatabaseLimit = PaymentRequiredData{Type: PaymentRequiredType, Code: PaymentCodeDatabaseLimit, Unit: PaymentUnitMB}
	PaymentAssetLimit    = PaymentRequiredData{Type: PaymentRequiredType, Code: PaymentCodeAssetLimit, Unit: PaymentUnitMB}
	PaymentTableLimit    = PaymentRequiredData{Type: PaymentRequiredType, Code: PaymentCodeTableLimit, Unit: PaymentUnitTables}
	PaymentApiCallLimit  = PaymentRequiredData{Type: PaymentRequiredType, Code: PaymentCodeApiCallLimit, Unit: PaymentUnitRequests}
)

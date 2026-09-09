package models

type TokenLimitData struct {
	Type       string `json:"type"`   // "payment_required"
	Code       string `json:"code"`   // "token_day_limit" | "token_month_limit"
	Period     string `json:"period"` // "day" | "month"
	Used       int64  `json:"used"`
	Limit      int64  `json:"limit"`
	Unit       string `json:"unit"` // "tokens"
	DayUsed    int64  `json:"day_used"`
	DayLimit   int64  `json:"day_limit"`
	MonthUsed  int64  `json:"month_used"`
	MonthLimit int64  `json:"month_limit"`
	PackRemain int64  `json:"pack_remain"` // remaining add-on pack tokens (0 when blocked)
}

type TokenBudgetSnapshot struct {
	DayLimit   int64
	DayUsed    int64
	MonthLimit int64
	MonthUsed  int64
	PackRemain int64 // company token-pack pool at budget init
}

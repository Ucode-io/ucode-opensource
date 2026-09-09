package models

type PaymentRequiredData struct {
	Type string `json:"type"`          // always "payment_required"
	Code string `json:"code"`          // "user_limit" | "api_key_limit"
	Unit string `json:"unit,omitempty"` // "users" | "api_keys"
}

package config

import "time"

const (
	DatabaseQueryTimeLayout  string = `'YYYY-MM-DD"T"HH24:MI:SS"."MS"Z"TZ'`
	DatabaseTimeLayout       string = time.RFC3339
	LowNodeType              string = "LOW"
	HighNodeType             string = "HIGH"
	EnterPriceNodeType       string = "ENTER_PRICE"
	BillingAlertDay          int    = 5
	BillingFreeTrialAlertDay int    = 16
	// BillingRenewalWarningDays is the lookahead window (in days) within which a
	// project whose funds cannot cover the next renewal charge is flagged.
	BillingRenewalWarningDays int = 10

	// BillingDeadProjectMonths is how many calendar months a project may stay frozen
	// in insufficient_funds (ucode renewal unpaid) before it is deactivated for good.
	BillingDeadProjectMonths int    = 3
	CURRENCY_UZS             string = "UZS"
	CURRENCY_USD             string = "USD"

	// Project statuses (project.status)
	STATUS_ACTIVE             string = "active"
	STATUS_INACTIVE           string = "inactive"
	STATUS_PENDING            string = "pending"
	STATUS_BLOCKED            string = "blocked"
	STATUS_INSUFFICIENT_FUNDS string = "insufficient_funds"

	// Subscription types (subscription.type)
	SUBSCRIPTION_FREE_TRIAL = "free_trial"
	SUBSCRIPTION_PAID       = "paid"

	// Transaction types
	TransactionTypeSubscription        = "subscription"
	TransactionTypeTopup               = "topup"
	TransactionTypeWithdraw            = "withdraw"
	TransactionTypeUpgrade             = "upgrade"
	TransactionTypeTokenPack           = "token_pack"
	TransactionTypeTemplate            = "template_purchase"
	TransactionTypeTemplateRfnd        = "template_refund"
	TransactionTypeUserSeat            = "user_seat_purchase"
	TransactionTypeUserSeatRfnd        = "user_seat_refund"
	TransactionTypeUserSeatMonthly     = "user_seat_monthly"
	TransactionTypeUserSeatMonthlyRfnd = "user_seat_monthly_refund"

	USD_CURRENCY_ID     string = "88c816a3-24e8-4994-ab70-9bc826bb9dc3"
	UZS_CURRENCY_ID     string = "0803582e-29d6-42fe-86ac-b4287b8fa929"
	STRIPE_PAYMENT_TYPE string = "Stripe"
	PRODUCTION          string = "production"

	// Product types
	PRODUCT_TYPE_UCODE string = "ucode"

	// Subscription statuses
	SUBSCRIPTION_STATUS_ACTIVE            = "active"
	SUBSCRIPTION_STATUS_EXPIRED           = "expired"
	SUBSCRIPTION_STATUS_CANCELED          = "canceled"
	SUBSCRIPTION_STATUS_PENDING_DOWNGRADE = "pending_downgrade"
)

var (
	OpenFaaSPlatformID string = "7d4a4c38-dd84-4902-b744-0488b80a4c04"
	CBUURL             string = "https://cbu.uz/uz/arkhiv-kursov-valyut/json"

	PaymeCreateCard             = "cards.create"
	PaymeGetVerifyCode          = "cards.get_verify_code"
	PaymeVerify                 = "cards.verify"
	CardRemove                  = "cards.remove"
	ReceiptCreate               = "receipts.create"
	ReceiptPay                  = "receipts.pay"
	PaymePaymentStatusPending   = "pending"
	PaymePaymentStatusAccepted  = "accepted"
	PaymePaymentStatusCancelled = "cancelled"
	PaymePaymentType            = "Payme"
	PaymeCreatorType            = "server"
	PaymeCurrencyId             = "0803582e-29d6-42fe-86ac-b4287b8fa929"
	PaymeStatusPaid             = 4

	// Ipak Yo'li E-Comm (Visa/Mastercard hosted-page top-ups). Reuses the shared
	// payment_status vocabulary (pending/accepted/cancelled) and UZS currency.
	IpakYuliPaymentType   = "IpakYuli"
	IpakYuliLinkExpiryMin = 30

	// Bank transfer statuses returned by transfer.get.
	IpakStatusSuccess  = "success"
	IpakStatusFailed   = "failed"
	IpakStatusCanceled = "canceled"
	IpakStatusExpired  = "expired"

	// IpakTerminalStatuses map a bank status to the local payment_status a pending
	// transaction should move to. Statuses absent here (await_payment, in_progress,
	// held) leave the transaction pending for the next confirm attempt.
	IpakTerminalStatuses = map[string]string{
		IpakStatusSuccess:  PaymePaymentStatusAccepted,
		IpakStatusFailed:   PaymePaymentStatusCancelled,
		IpakStatusCanceled: PaymePaymentStatusCancelled,
		IpakStatusExpired:  PaymePaymentStatusCancelled,
	}

	ChequeState = map[int]string{
		0:  "Cheque created. Waiting for payment confirmation.",
		1:  "First stage of checks. Creating a transaction in the supplier's billing system.",
		2:  "Debiting funds from the card.",
		3:  "Closing the transaction in the supplier's billing system.",
		4:  "Cheque paid.",
		5:  "Cheque has been coded.",
		6:  "Received a command to hold funds. If the cheque remains in this status for too long, please contact Payme Business technical support.",
		20: "Cheque paused for manual intervention.",
		21: "Cheque queued for cancellation.",
		30: "Cheque queued for closing the transaction in the supplier's billing system.",
		50: "Cheque canceled.",
	}

	SkipTablesSlugs = map[string]bool{
		"object_builders":                       true,
		"template_service":                      true,
		"permissions":                           true,
		"functions":                             true,
		"fields":                                true,
		"files":                                 true,
		"layouts":                               true,
		"automatic_filter":                      true,
		"automatic_filters":                     true,
		"apps":                                  true,
		"client_platforms":                      true,
		"client_types":                          true,
		"connections":                           true,
		"customerrormessages":                   true,
		"customevents":                          true,
		"dashboards":                            true,
		"eventlogs":                             true,
		"events":                                true,
		"incrementseqs":                         true,
		"panels":                                true,
		"projects":                              true,
		"queries":                               true,
		"queryfolders":                          true,
		"relations":                             true,
		"reportsettings":                        true,
		"setting":                               true,
		"table":                                 true,
		"tabs":                                  true,
		"variables":                             true,
		"webpages":                              true,
		"tokens":                                true,
		"test_logins":                           true,
		"views":                                 true,
		"templates":                             true,
		"tables":                                true,
		"sections":                              true,
		"roles":                                 true,
		"viewrelations":                         true,
		"table.folders":                         true,
		"htmltemplates":                         true,
		"roworders":                             true,
		"table.histories":                       true,
		"user_login_tables":                     true,
		"table.versions":                        true,
		"setting.languages":                     true,
		"record_permissions":                    true,
		"users_permissions":                     true,
		"field_permissions":                     true,
		"action_permissions":                    true,
		"menu_permissions":                      true,
		"view_permissions":                      true,
		"global_permissions":                    true,
		"documents":                             true,
		"setting.currencies":                    true,
		"function_service.functions":            true,
		"object_builder_service.menus":          true,
		"template_service.folder_templates":     true,
		"object_builder_service.menu.templates": true,
		"template_service.templates":            true,
		"object_builder_service.versions":       true,
		"function_service.custom_events":        true,
		"object_builder_service.menu.settings":  true,
		"object_builder_service.files":          true,
		"template_service.folder_notes":         true,
		"object_builder.custom_errors":          true,
		"template_service.notes":                true,
		"function_service.folders":              true,
		"view_relation_permissions":             true,
		"setting.timezones":                     true,
		"object_builder_service.version_histories": true,
	}
)

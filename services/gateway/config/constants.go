package config

import (
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	COMMIT_TYPE_APP                      string = "APP"
	COMMIT_TYPE_TABLE                    string = "TABLE"
	COMMIT_TYPE_FIELD                    string = "FIELD"
	COMMIT_TYPE_RELATION                 string = "RELATION"
	COMMIT_TYPE_SECTION                  string = "SECTION"
	COMMIT_TYPE_VIEW                     string = "VIEW"
	COMMIT_TYPE_VIEW_RELATION            string = "VIEW_RELATION"
	COMMIT_TYPE_CLIENT_PLATFORM          string = "CLIENT_PLATFORM"
	COMMIT_TYPE_CLIENT_TYPE              string = "CLIENT_TYPE"
	COMMIT_TYPE_ROLE                     string = "ROLE"
	COMMIT_TYPE_TEST_LOGIN               string = "TEST_LOGIN"
	COMMIT_TYPE_CONNECTION               string = "CONNECTION"
	COMMIT_TYPE_AUTOMATIC_FILTER         string = "AUTOMATIC_FILTER"
	COMMIT_TYPE_CUSTOM_EVENT             string = "CUSTOM_EVENT"
	COMMIT_TYPE_RECORD_PERMISSION        string = "RECORD_PERMISSION"
	COMMIT_TYPE_ACTION_PERMISSION        string = "ACTION_PERMISSION"
	COMMIT_TYPE_FIELD_PERMISSION         string = "FIELD_PERMISSION"
	COMMIT_TYPE_VIEW_PERMISSION          string = "VIEW_PERMISSION"
	COMMIT_TYPE_VIEW_RELATION_PERMISSION string = "VIEW_RELATION_PERMISSION"
	COMMIT_TYPE_DASHBOARD                string = "DASHBOARD"
	COMMIT_TYPE_VARIABLE                 string = "VARIABLE"
	COMMIT_TYPE_PANEL                    string = "PANEL"
	COMMIT_TYPE_FUNCTION                 string = "FUNCTION"
	COMMIT_TYPE_SCENARIO                 string = "SCENARIO"
	LOW_NODE_TYPE                        string = "LOW"
	HIGH_NODE_TYPE                       string = "HIGH"
	ENTER_PRICE_TYPE                     string = "ENTER_PRICE"
	CACHE_WAIT                           string = "WAIT"

	// Project statuses that block write access. Reads are still allowed; writes are
	// rejected with a clear, status-specific message.
	STATUS_INACTIVE           string = "inactive"
	STATUS_INSUFFICIENT_FUNDS string = "insufficient_funds"
	STATUS_BLOCKED            string = "blocked"

	PUBLIC_STATUS string = "unapproved"
)

// ProjectStatusMessages maps each blocking project status to the user-facing
// message shown when access is refused.
var ProjectStatusMessages = map[string]string{
	STATUS_INACTIVE:           "Your project is inactive. Please contact support to reactivate it.",
	STATUS_INSUFFICIENT_FUNDS: "Your project is suspended due to insufficient balance. Please top up your balance to continue.",
	STATUS_BLOCKED:            "Your project has been blocked. Please contact support.",
}

// ProjectStatusMessage returns the message for a blocking project status, or
// ("", false) when the status permits writes.
func ProjectStatusMessage(projectStatus string) (string, bool) {
	message, blocking := ProjectStatusMessages[projectStatus]
	return message, blocking
}

// BlockingStatusMessage translates a PermissionDenied error from the auth service
// into its user-facing message, returning false for any other error. The auth gate
// signals a blocking project status by carrying the raw status string in the
// gRPC error message.
func BlockingStatusMessage(err error) (string, bool) {
	st, ok := status.FromError(err)
	if !ok || st.Code() != codes.PermissionDenied {
		return "", false
	}
	return ProjectStatusMessage(st.Message())
}

const (
	LRU_CACHE_SIZE                       = 10000
	LIMITER_RANGE                        = 100
	RATE_LIMITER_RPS_LIMIT               = 100
	REDIS_TIMEOUT          time.Duration = 5 * time.Minute
	REDIS_KEY_TIMEOUT      time.Duration = 280 * time.Second
	REDIS_WAIT_TIMEOUT     time.Duration = 1 * time.Second
	REDIS_SLEEP            time.Duration = 100 * time.Millisecond

	GRPC_MAX_CALL_SEND_MSG_SIZE = 100 * 1024 * 1024
	GRPC_MAX_CALL_RECV_MSG_SIZE = 100 * 1024 * 1024

	TIME_LAYOUT string = "15:04"

	// Function Types
	FUNCTION string = "FUNCTION"
	KNATIVE  string = "KNATIVE"
	WORKFLOW string = "WORKFLOW"

	// CustomEventTypes
	BEFORE string = "before"
	AFTER  string = "after"

	InactiveStatus   string = "inactive"
	PermissionDenied string = "Permission denied"
	SessionExpired   string = "Session has been expired"

	MainMenuID   = "c57eedc3-a954-4262-a0af-376c65b5a284"
	MainFolderID = "8a6f913a-e3d4-4b73-9fc0-c942f343d0b9"

	KeyRateMin  = "rate:%s:min:%s"  // ProjectID, 2006-01-02-15-04
	KeyRateHour = "rate:%s:hour:%s" // ProjectID, 2006-01-02-15
	KeyRateDay  = "rate:%s:day:%s"  // ProjectID, 2006-01-02

	KeyUsagePending        = "api_usage:pending:%s"
	KeyUsagePendingPattern = "api_usage:pending:*"
	KeyUsagePendingPrefix  = "api_usage:pending:"
	KeyUsageTotalField     = "total"

	AnthropicCachingBeta = "prompt-caching-2024-07-31"

	YandexMetricCountersURL = "https://api-metrika.yandex.net/management/v1/counters"

	UGEN_FREE_PLAN_ID = "07d8a364-ebb2-4291-a452-f44b335cb031"

	// AI products that token usage is attributed to.
	PRODUCT_TYPE_UCODE string = "ucode"
	PRODUCT_TYPE_UGEN  string = "ugen"

	FARE_ASSET_SIZE        string = "asset_size"
	FARE_DATABASE_SIZE     string = "database"
	FARE_REQUEST_PER_MONTH string = "request_per_month"

	KeyBillingDbLimit        = "billing:db_limit:%s" // projectId → "1"(allowed) | "0"(blocked)
	KeyBillingDbLimitPattern = "billing:db_limit:*"
	KeyBillingDbLimitPrefix  = "billing:db_limit:"
	KeyBillingDbCtx          = "billing:db_ctx:%s" // projectId → JSON context for worker

	KeyBillingApiLimit = "billing:api_limit:%s" // projectId → "1"(allowed) | "0"(blocked)
	KeyBillingFareId   = "billing:fare_id:%s"   // projectId → fareId string, TTL=30min

	UgenSuperAdminUserId = "c12c163c-38ee-4b37-8854-1dc9285fc3f8"

	// Meta (Facebook) Lead Ads
	FacebookOAuthStatePrefix      = "facebook-oauth-state:"
	FacebookOAuthStateTTL         = 10 * time.Minute
	FacebookIntegrationName       = "Facebook Lead Ads"
	FacebookOAuthScopes           = "pages_show_list,pages_read_engagement,leads_retrieval,pages_manage_metadata,business_management,ads_read"
	FacebookSubscribedFields      = "leadgen"
	FacebookResourceType          = "META_LEADS"
	FacebookStatusActive          = "active"
	FacebookStatusRevoked         = "revoked"
	FacebookStatusError           = "error"
	FacebookWebhookFieldLead      = "leadgen"
	FacebookSignatureHeader       = "X-Hub-Signature-256"
	FacebookSignaturePrefix       = "sha256="
	FacebookAppReviewLockPrefix   = "facebook-app-review-test-lock:"
	FacebookAppReviewResultPrefix = "facebook-app-review-test-result:"
	FacebookAppReviewLockTTL      = 20 * time.Minute
	FacebookAppReviewResultTTL    = 30 * 24 * time.Hour
	FacebookAppReviewRunTimeout   = 15 * time.Minute
	FacebookAppReviewCallInterval = 150 * time.Millisecond
	FacebookAppReviewTargetCalls  = 500
	FacebookAppReviewMaxAttempts  = 600

	GoogleLeadsStatusActive = "active"

	// Instagram Direct Messaging
	InstagramOAuthStatePrefix = "instagram-oauth-state:"
	InstagramOAuthStateTTL    = 10 * time.Minute
	InstagramIntegrationName  = "Instagram Support"
	InstagramOAuthScopes      = "instagram_business_basic,instagram_business_manage_messages"
	InstagramSubscribedFields = "messages,message_edit,message_reactions,messaging_postbacks,messaging_referral,messaging_seen"
	InstagramStatusPendingUI  = "pending_ui"
	InstagramStatusConnected  = "connected"
	InstagramSignatureHeader  = "X-Hub-Signature-256"
	InstagramSignaturePrefix  = "sha256="

	// KP (commercial proposal) preview/download. requestId -> {projectId,environmentId,
	// title,pageCount,qaStatus,hasHtml,hasPdf,prototype} record, written once by
	// POST /v1/kp-proposals and read by GET /v1/kp-proposals/:requestId (metadata),
	// .../html and .../pdf — so all three can be served by either gateway replica
	// regardless of which replica handled the original POST. TTL is kept longer than
	// the agent's own in-memory artifact retention (KP_AGENT_ARTIFACT_RETENTION_MS on
	// the agent, default 2h) so the agent's own 404/410 is always the source of truth
	// for "is it still there", never a premature cache eviction here.
	KpProposalCachePrefix = "kp:proposal:"
	KpProposalCacheTTL    = 3 * time.Hour
)

var (
	RelationFieldTypes = map[string]bool{
		"LOOKUP":  true,
		"LOOKUPS": true,
	}

	ConvertDocxToPdfUrl    = ""
	ConvertDocxToPdfSecret = ""

	RateLimitSkipFiles = map[string]bool{
		"user":        true,
		"users":       true,
		"role":        true,
		"client_type": true,
	}
)

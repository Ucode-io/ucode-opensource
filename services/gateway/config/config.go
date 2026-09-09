package config

import (
	"os"
	"strings"

	"github.com/spf13/cast"
)

const (
	DebugMode = "debug"
	// TestMode indicates service mode is test.
	TestMode = "test"
	// ReleaseMode indicates service mode is release.
	ReleaseMode = "release"
)

type Config struct {
	DefaultOffset string
	DefaultLimit  string

	ObjectBuilderServiceHost string
	ObjectBuilderGRPCPort    string

	HighObjectBuilderServiceHost string
	HighObjectBuilderGRPCPort    string

	SmsServiceHost string
	SmsGRPCPort    string

	AuthServiceHost string
	AuthGRPCPort    string

	CompanyServiceHost string
	CompanyServicePort string

	TranscoderServiceHost string
	TranscoderServicePort string

	DocGeneratorGrpcHost string
	DocGeneratorGrpcPort string

	MinioEndpoint        string
	MinioAccessKeyID     string
	MinioSecretAccessKey string
	MinioProtocol        bool

	UcodeNamespace string

	SecretKey string

	GoObjectBuilderServiceHost string
	GoObjectBuilderGRPCPort    string

	GetRequestRedisHost     string
	GetRequestRedisPort     string
	GetRequestRedisDatabase int
	GetRequestRedisPassword string

	OpenAIApiKey string

	// Tracker / Audit Logs
	AuditMaxBodySize          int64
	AuditMetricsFlushInterval int
	AuditFlushInterval        int
	AuditLogChannelSize       int
	AuditBatchSize            int
	AuditLogRetentionDays     int
}

type BaseConfig struct {
	ServiceName string
	Environment string
	Version     string

	HTTPBaseURL string
	ServiceHost string
	HTTPPort    string
	HTTPScheme  string

	AuthServiceHost string
	AuthGRPCPort    string

	GoObjectBuilderServiceHost string
	GoObjectBuilderGRPCPort    string

	GoFunctionServiceHost     string
	GoFunctionServiceHTTPPort string

	MinioEndpoint        string
	MinioAccessKeyID     string
	MinioSecretAccessKey string
	MinioProtocol        bool

	DefaultOffset   string
	DefaultLimit    string
	DefaultLimitInt int

	SecretKey string

	UcodeNamespace string
	JaegerHostPort string

	N8NApiKey  string
	N8NBaseURL string

	StripeApiKey        string
	StripeWebhookSecret string

	AnthropicAPIKey      string
	AnthropicAPIKeyUcode string
	AnthropicBeta        string
	AnthropicBaseAPIURL  string
	AnthropicVersion     string

	ClaudeModel string
	Agents      AIAgents

	GeminiAPIKey  string
	GeminiAPIKeys []string
	GeminiAgents  AIAgents

	OpenAIAPIKey  string
	OpenAIBaseURL string
	OpenAIAgents  AIAgents

	AutomationURL    string
	OpenFaaSBaseUrl  string
	KnativeBaseUrl   string
	KnativeBaseUrlUz string
	MCPServerURL     string

	// KP generator agent (standalone HTML KP service, internal ClusterIP).
	KpAgentURL    string
	KpAgentAPIKey string

	// MCP handler token limits — used in mcp.go, not tied to a specific agent.
	MaxTokens                int
	AnalyseProjectMaxTokens  int
	GeneratePlanMaxTokens    int
	ClassifyReqeustMaxTokens int

	UcodeBaseUrl string

	VaultAddress   string
	VaultRoleID    string
	VaultSecretID  string
	VaultMountPath string

	GitlabBaseURL string
	GitlabToken   string

	ShortURLBase string

	GithubClientID           string
	GithubClientSecret       string
	GithubRedirectURI        string
	GithubFrontendSuccessURL string
	GithubFrontendErrorURL   string

	UnsplashAccessKey string

	ReferenceCaptureEnabled        bool
	ReferenceCaptureURL            string
	ReferenceCaptureAPIKey         string
	ReferenceCaptureTimeoutSeconds int

	GoogleDriveClientID           string
	GoogleDriveClientSecret       string
	GoogleDriveRedirectURI        string
	GoogleDriveFrontendSuccessURL string
	GoogleDriveFrontendErrorURL   string
	GoogleDriveServiceAccountJSON string
	GoogleDriveParentFolderID     string
	GoogleDriveVisibility         string

	GoogleCalendarClientID           string
	GoogleCalendarClientSecret       string
	GoogleCalendarRedirectURI        string
	GoogleCalendarFrontendSuccessURL string
	GoogleCalendarFrontendErrorURL   string

	TelegramManagerBotToken      string
	TelegramManagerBotUsername   string
	TelegramManagerWebhookSecret string
	TelegramWebhookBaseURL       string

	YandexMetricToken string

	FacebookAppID           string
	FacebookAppSecret       string
	FacebookRedirectURI     string
	FacebookGraphBaseURL    string
	FacebookAuthBaseURL     string
	FacebookGraphAPIVersion string

	FacebookWebhookVerifyToken string
	FacebookLegacyAppID        string
	FacebookLegacyAppSecret    string
	FacebookLegacyAppSecrets   []string

	// Lead poller: a safety-net that periodically pulls new leads from connected
	// pages' forms in case a webhook was delayed or not delivered. Off by default.
	FacebookLeadPollEnabled     bool
	FacebookLeadPollIntervalSec int

	MetaAdsGraphBaseURL       string
	MetaAdsGraphVersion       string
	MetaAdsAdAccountID        string
	MetaAdsAccessToken        string
	MetaAdsCacheTTLSeconds    int
	MetaAdsRequestTimeoutSec  int
	MetaAdsMaxRangeDays       int
	MetaAdsLeadActionTypes    []string
	MetaAdsAttributionWindows []string

	// GoogleLeadsWebhookURL is the public URL Google posts leads to; returned to
	// the user so they can paste it into the Google Ads lead form settings.
	GoogleLeadsWebhookURL string

	InstagramClientID            string
	InstagramClientSecret        string
	InstagramRedirectURI         string
	InstagramFrontendSuccessURL  string
	InstagramFrontendErrorURL    string
	InstagramGraphBaseURL        string
	InstagramGraphVersion        string
	InstagramWebhookVerifyToken  string
	InstagramLegacyClientSecrets []string
	InstagramOAuthAuthorizeURL   string
	InstagramOAuthAccessTokenURL string
}

func BaseLoad() BaseConfig {
	loadRuntimeEnvironment()

	config := BaseConfig{}

	config.ServiceName = cast.ToString(GetOrReturnDefaultValue("SERVICE_NAME", "ucode_go_api_gateway"))
	config.Environment = cast.ToString(GetOrReturnDefaultValue("ENVIRONMENT", DebugMode))
	config.Version = cast.ToString(GetOrReturnDefaultValue("VERSION", "1.0"))

	config.HTTPBaseURL = cast.ToString(GetOrReturnDefaultValue("HTTP_BASE_URL", ""))
	config.ServiceHost = cast.ToString(GetOrReturnDefaultValue("SERVICE_HOST", ""))
	config.HTTPPort = cast.ToString(GetOrReturnDefaultValue("HTTP_PORT", ":8000"))
	config.HTTPScheme = cast.ToString(GetOrReturnDefaultValue("HTTP_SCHEME", ""))

	config.AuthServiceHost = cast.ToString(GetOrReturnDefaultValue("AUTH_SERVICE_HOST", ""))
	config.AuthGRPCPort = cast.ToString(GetOrReturnDefaultValue("AUTH_GRPC_PORT", ""))

	config.MinioAccessKeyID = cast.ToString(GetOrReturnDefaultValue("MINIO_ACCESS_KEY", ""))
	config.MinioSecretAccessKey = cast.ToString(GetOrReturnDefaultValue("MINIO_SECRET_KEY", ""))
	config.MinioEndpoint = cast.ToString(GetOrReturnDefaultValue("MINIO_ENDPOINT", ""))
	config.MinioProtocol = cast.ToBool(GetOrReturnDefaultValue("MINIO_PROTOCOL", true))

	config.GoFunctionServiceHost = cast.ToString(GetOrReturnDefaultValue("GO_FUNCTION_SERVICE_HOST", "http://localhost"))
	config.GoFunctionServiceHTTPPort = cast.ToString(GetOrReturnDefaultValue("GO_FUNCTION_SERVICE_HTTP_PORT", ":7090"))

	config.GoObjectBuilderServiceHost = cast.ToString(GetOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_HOST", "localhost"))
	config.GoObjectBuilderGRPCPort = cast.ToString(GetOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_PORT", ":7107"))

	config.DefaultOffset = cast.ToString(GetOrReturnDefaultValue("DEFAULT_OFFSET", "0"))
	config.DefaultLimit = "60"
	config.DefaultLimitInt = 20

	config.UcodeNamespace = "u-code"
	config.SecretKey = cast.ToString(GetOrReturnDefaultValue("SECRET_KEY", ""))
	config.JaegerHostPort = cast.ToString(GetOrReturnDefaultValue("JAEGER_URL", ""))

	config.N8NApiKey = cast.ToString(GetOrReturnDefaultValue("N8N_API_KEY", ""))
	config.N8NBaseURL = cast.ToString(GetOrReturnDefaultValue("N8N_BASE_URL", ""))

	config.StripeApiKey = cast.ToString(GetOrReturnDefaultValue("STRIPE_API_KEY", ""))
	config.StripeWebhookSecret = cast.ToString(GetOrReturnDefaultValue("STRIPE_WEBHOOK_SECRET", ""))

	config.AnthropicAPIKey = cast.ToString(GetOrReturnDefaultValue("ANTHROPIC_API_KEY", ""))
	config.AnthropicAPIKeyUcode = cast.ToString(GetOrReturnDefaultValue("ANTHROPIC_API_KEY_UCODE", ""))
	config.AnthropicBeta = cast.ToString(GetOrReturnDefaultValue("ANTHROPIC_BETA", ""))
	config.AnthropicBaseAPIURL = cast.ToString(GetOrReturnDefaultValue("ANTHROPIC_BASE_API_URL", ""))
	config.AnthropicVersion = cast.ToString(GetOrReturnDefaultValue("ANTHROPIC_VERSION", ""))
	config.ClaudeModel = cast.ToString(GetOrReturnDefaultValue("CLAUDE_MODEL", ""))

	config.Agents = loadAIAgents()
	config.GeminiAgents = loadGeminiAgents()

	config.GeminiAPIKeys, config.GeminiAPIKey = loadGeminiKeys()

	config.OpenAIAPIKey = cast.ToString(GetOrReturnDefaultValue("OPENAI_API_KEY", ""))
	config.OpenAIBaseURL = cast.ToString(GetOrReturnDefaultValue("OPENAI_BASE_URL", ""))
	config.OpenAIAgents = loadOpenAIAgents()

	config.AutomationURL = cast.ToString(GetOrReturnDefaultValue("AUTOMATION_URL", ""))
	config.OpenFaaSBaseUrl = cast.ToString(GetOrReturnDefaultValue("OPENFAAS_BASE_URL", ""))
	config.KnativeBaseUrl = cast.ToString(GetOrReturnDefaultValue("KNATIVE_BASE_URL", ""))
	config.KnativeBaseUrlUz = cast.ToString(GetOrReturnDefaultValue("FUNC_BASE_URL_UZB", ""))
	config.MCPServerURL = cast.ToString(GetOrReturnDefaultValue("MCP_SERVER_URL", ""))
	config.KpAgentURL = cast.ToString(GetOrReturnDefaultValue("KP_AGENT_URL", ""))
	config.KpAgentAPIKey = cast.ToString(GetOrReturnDefaultValue("KP_AGENT_API_KEY", ""))
	config.UcodeBaseUrl = cast.ToString(GetOrReturnDefaultValue("UCODE_BASE_URL", "https://admin-api.ucode.run"))

	config.VaultAddress = cast.ToString(GetOrReturnDefaultValue("VAULT_ADDR", ""))
	config.VaultRoleID = cast.ToString(GetOrReturnDefaultValue("VAULT_ROLE_ID", ""))
	config.VaultSecretID = cast.ToString(GetOrReturnDefaultValue("VAULT_SECRET_ID", ""))
	config.VaultMountPath = cast.ToString(GetOrReturnDefaultValue("VAULT_MOUNT_PATH", "approle"))

	config.GitlabBaseURL = cast.ToString(GetOrReturnDefaultValue("GITLAB_BASE_URL", "https://gitlab.udevs.io/"))
	config.GitlabToken = cast.ToString(GetOrReturnDefaultValue("GITLAB_TOKEN", ""))

	config.ShortURLBase = cast.ToString(GetOrReturnDefaultValue("SHORT_URL_BASE", "app.ucode.co"))

	config.GithubClientID = cast.ToString(GetOrReturnDefaultValue("GITHUB_CLIENT_ID", ""))
	config.GithubClientSecret = cast.ToString(GetOrReturnDefaultValue("GITHUB_CLIENT_SECRET", ""))
	config.GithubRedirectURI = cast.ToString(GetOrReturnDefaultValue("GITHUB_REDIRECT_URI", ""))
	config.GithubFrontendSuccessURL = cast.ToString(GetOrReturnDefaultValue("GITHUB_FRONTEND_SUCCESS_URL", "https://app.u-code.io/settings/github-success"))
	config.GithubFrontendErrorURL = cast.ToString(GetOrReturnDefaultValue("GITHUB_FRONTEND_ERROR_URL", "https://app.u-code.io/settings/github-error"))

	config.UnsplashAccessKey = cast.ToString(GetOrReturnDefaultValue("UNSPLASH_ACCESS_KEY", ""))
	config.ReferenceCaptureEnabled = cast.ToBool(GetOrReturnDefaultValue("REFERENCE_CAPTURE_ENABLED", false))
	config.ReferenceCaptureURL = cast.ToString(GetOrReturnDefaultValue("REFERENCE_CAPTURE_URL", ""))
	config.ReferenceCaptureAPIKey = cast.ToString(GetOrReturnDefaultValue("REFERENCE_CAPTURE_API_KEY", ""))
	config.ReferenceCaptureTimeoutSeconds = cast.ToInt(GetOrReturnDefaultValue("REFERENCE_CAPTURE_TIMEOUT_SECONDS", 20))

	config.GoogleDriveClientID = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_CLIENT_ID", ""))
	config.GoogleDriveClientSecret = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_CLIENT_SECRET", ""))
	config.GoogleDriveRedirectURI = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_REDIRECT_URI", ""))
	config.GoogleDriveFrontendSuccessURL = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_FRONTEND_SUCCESS_URL", "https://app.u-code.io/settings/google-drive-success"))
	config.GoogleDriveFrontendErrorURL = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_FRONTEND_ERROR_URL", "https://app.u-code.io/settings/google-drive-error"))
	config.GoogleDriveServiceAccountJSON = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_SERVICE_ACCOUNT_JSON", ""))
	config.GoogleDriveParentFolderID = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_PARENT_FOLDER_ID", ""))
	config.GoogleDriveVisibility = cast.ToString(GetOrReturnDefaultValue("GOOGLE_DRIVE_VISIBILITY", "anyone_with_link"))

	config.GoogleCalendarClientID = cast.ToString(GetOrReturnDefaultValue("GOOGLE_CALENDAR_CLIENT_ID", ""))
	config.GoogleCalendarClientSecret = cast.ToString(GetOrReturnDefaultValue("GOOGLE_CALENDAR_CLIENT_SECRET", ""))
	config.GoogleCalendarRedirectURI = cast.ToString(GetOrReturnDefaultValue("GOOGLE_CALENDAR_REDIRECT_URI", ""))
	config.GoogleCalendarFrontendSuccessURL = cast.ToString(GetOrReturnDefaultValue("GOOGLE_CALENDAR_FRONTEND_SUCCESS_URL", "https://app.u-code.io/settings/google-drive-success"))
	config.GoogleCalendarFrontendErrorURL = cast.ToString(GetOrReturnDefaultValue("GOOGLE_CALENDAR_FRONTEND_ERROR_URL", "https://app.u-code.io/settings/google-drive-error"))

	config.TelegramManagerBotToken = cast.ToString(GetOrReturnDefaultValue("TELEGRAM_MANAGER_BOT_TOKEN", ""))
	config.TelegramManagerBotUsername = strings.TrimPrefix(cast.ToString(GetOrReturnDefaultValue("TELEGRAM_MANAGER_BOT_USERNAME", "")), "@")
	config.TelegramManagerWebhookSecret = cast.ToString(GetOrReturnDefaultValue("TELEGRAM_MANAGER_WEBHOOK_SECRET", ""))
	config.TelegramWebhookBaseURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("TELEGRAM_WEBHOOK_BASE_URL", config.UcodeBaseUrl)), "/")

	config.YandexMetricToken = cast.ToString(GetOrReturnDefaultValue("YANDEX_METRIC_TOKEN", ""))

	legacyFacebookAppID := cast.ToString(GetOrReturnDefaultValue("FACEBOOK_APP_ID", ""))
	legacyFacebookAppSecret := cast.ToString(GetOrReturnDefaultValue("FACEBOOK_APP_SECRET", ""))
	config.FacebookAppID = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_FACEBOOK_APP_ID", "")), legacyFacebookAppID)
	config.FacebookAppSecret = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_FACEBOOK_APP_SECRET", "")), legacyFacebookAppSecret)
	config.FacebookRedirectURI = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_FACEBOOK_REDIRECT_URI", "")), cast.ToString(GetOrReturnDefaultValue("FACEBOOK_REDIRECT_URI", "")))
	config.FacebookAuthBaseURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("FACEBOOK_AUTH_BASE_URL", "https://graph.facebook.com")), "/")
	config.FacebookGraphBaseURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("FACEBOOK_GRAPH_BASE_URL", "https://graph.facebook.com")), "/")
	config.FacebookGraphAPIVersion = cast.ToString(GetOrReturnDefaultValue("FACEBOOK_GRAPH_API_VERSION", "v21.0"))
	config.FacebookWebhookVerifyToken = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_FACEBOOK_WEBHOOK_VERIFY_TOKEN", "")), cast.ToString(GetOrReturnDefaultValue("FACEBOOK_WEBHOOK_VERIFY_TOKEN", "")))
	config.FacebookLegacyAppID = legacyFacebookAppID
	config.FacebookLegacyAppSecret = legacyFacebookAppSecret
	config.FacebookLegacyAppSecrets = legacySecrets(config.FacebookAppSecret, legacyFacebookAppSecret, cast.ToString(GetOrReturnDefaultValue("FACEBOOK_LEGACY_APP_SECRETS", "")))
	config.FacebookLeadPollEnabled = cast.ToBool(GetOrReturnDefaultValue("FACEBOOK_LEAD_POLL_ENABLED", true))
	config.FacebookLeadPollIntervalSec = cast.ToInt(GetOrReturnDefaultValue("FACEBOOK_LEAD_POLL_INTERVAL_SEC", 120))

	config.MetaAdsGraphBaseURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("META_GRAPH_BASE_URL", "https://graph.facebook.com")), "/")
	config.MetaAdsGraphVersion = cast.ToString(GetOrReturnDefaultValue("META_GRAPH_VERSION", "v26.0"))
	config.MetaAdsAdAccountID = strings.TrimPrefix(strings.TrimSpace(cast.ToString(GetOrReturnDefaultValue("META_AD_ACCOUNT_ID", ""))), "act_")
	config.MetaAdsAccessToken = strings.TrimSpace(cast.ToString(GetOrReturnDefaultValue("META_ACCESS_TOKEN", "")))
	config.MetaAdsCacheTTLSeconds = cast.ToInt(GetOrReturnDefaultValue("META_ADS_CACHE_TTL_SEC", 86400))
	config.MetaAdsRequestTimeoutSec = cast.ToInt(GetOrReturnDefaultValue("META_ADS_REQUEST_TIMEOUT_SEC", 30))
	config.MetaAdsMaxRangeDays = cast.ToInt(GetOrReturnDefaultValue("META_ADS_MAX_RANGE_DAYS", 366))
	config.MetaAdsLeadActionTypes = splitCommaSeparated(cast.ToString(GetOrReturnDefaultValue("META_LEAD_ACTION_TYPES", "lead")))
	config.MetaAdsAttributionWindows = splitCommaSeparated(cast.ToString(GetOrReturnDefaultValue("META_ATTRIBUTION_WINDOWS", "")))

	config.GoogleLeadsWebhookURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("GOOGLE_LEADS_WEBHOOK_URL", "")), "/")

	legacyInstagramClientID := cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_CLIENT_ID", ""))
	legacyInstagramClientSecret := cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_CLIENT_SECRET", ""))
	config.InstagramClientID = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_INSTAGRAM_CLIENT_ID", "")), legacyInstagramClientID)
	config.InstagramClientSecret = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_INSTAGRAM_CLIENT_SECRET", "")), legacyInstagramClientSecret)
	config.InstagramRedirectURI = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_INSTAGRAM_REDIRECT_URI", "")), cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_REDIRECT_URI", "")))
	config.InstagramFrontendSuccessURL = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_INSTAGRAM_FRONTEND_SUCCESS_URL", "")), cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_FRONTEND_SUCCESS_URL", "https://app.u-code.io/settings/instagram-success")))
	config.InstagramFrontendErrorURL = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_INSTAGRAM_FRONTEND_ERROR_URL", "")), cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_FRONTEND_ERROR_URL", "https://app.u-code.io/settings/instagram-error")))
	config.InstagramGraphBaseURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_GRAPH_BASE_URL", "https://graph.instagram.com")), "/")
	config.InstagramGraphVersion = cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_GRAPH_VERSION", "v25.0"))
	config.InstagramWebhookVerifyToken = firstNonEmpty(cast.ToString(GetOrReturnDefaultValue("UCODE_INSTAGRAM_WEBHOOK_VERIFY_TOKEN", "")), cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_WEBHOOK_VERIFY_TOKEN", "")))
	config.InstagramLegacyClientSecrets = legacySecrets(config.InstagramClientSecret, legacyInstagramClientSecret, cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_LEGACY_CLIENT_SECRETS", "")))
	config.InstagramOAuthAuthorizeURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_OAUTH_AUTHORIZE_URL", "https://www.instagram.com/oauth/authorize")), "/")
	config.InstagramOAuthAccessTokenURL = strings.TrimRight(cast.ToString(GetOrReturnDefaultValue("INSTAGRAM_OAUTH_ACCESS_TOKEN_URL", "https://api.instagram.com/oauth/access_token")), "/")

	config.MaxTokens = cast.ToInt(GetOrReturnDefaultValue("MAX_TOKENS", 12000))
	config.AnalyseProjectMaxTokens = cast.ToInt(GetOrReturnDefaultValue("ANALYSE_PROJECT_MAX_TOKENS", 5000))
	config.GeneratePlanMaxTokens = cast.ToInt(GetOrReturnDefaultValue("GENERATE_PLAN_MAX_TOKENS", 10000))
	config.ClassifyReqeustMaxTokens = cast.ToInt(GetOrReturnDefaultValue("CLASSIFY_MAX_TOKENS", 3000))

	return config
}

// Load ....
func Load() Config {

	config := Config{}

	config.DefaultOffset = cast.ToString(GetOrReturnDefaultValue("DEFAULT_OFFSET", "0"))
	config.DefaultLimit = "100"

	config.ObjectBuilderServiceHost = cast.ToString(GetOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_LOW_HOST", ""))
	config.ObjectBuilderGRPCPort = cast.ToString(GetOrReturnDefaultValue("OBJECT_BUILDER_LOW_GRPC_PORT", ""))

	config.AuthServiceHost = cast.ToString(GetOrReturnDefaultValue("AUTH_SERVICE_HOST", ""))
	config.AuthGRPCPort = cast.ToString(GetOrReturnDefaultValue("AUTH_GRPC_PORT", ""))

	config.CompanyServiceHost = cast.ToString(GetOrReturnDefaultValue("COMPANY_SERVICE_HOST", ""))
	config.CompanyServicePort = cast.ToString(GetOrReturnDefaultValue("COMPANY_GRPC_PORT", ""))

	config.TranscoderServiceHost = cast.ToString(GetOrReturnDefaultValue("TRANSCODER_SERVICE_HOST", "localhost"))
	config.TranscoderServicePort = cast.ToString(GetOrReturnDefaultValue("TRANSCODER_GRPC_PORT", ":9110"))

	config.HighObjectBuilderServiceHost = cast.ToString(GetOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_HIGHT_HOST", ""))
	config.HighObjectBuilderGRPCPort = cast.ToString(GetOrReturnDefaultValue("OBJECT_BUILDER_HIGH_GRPC_PORT", ""))

	config.SmsServiceHost = cast.ToString(GetOrReturnDefaultValue("SMS_SERVICE_HOST", ""))
	config.SmsGRPCPort = cast.ToString(GetOrReturnDefaultValue("SMS_GRPC_PORT", ":2008"))

	config.GoObjectBuilderServiceHost = cast.ToString(GetOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_HOST", "localhost"))
	config.GoObjectBuilderGRPCPort = cast.ToString(GetOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_PORT", ":7107"))

	config.UcodeNamespace = "cp-region-type-id"
	config.SecretKey = cast.ToString(GetOrReturnDefaultValue("SECRET_KEY", ""))

	config.DocGeneratorGrpcHost = cast.ToString(GetOrReturnDefaultValue("NODE_DOC_GENERATOR_SERVICE_HOST", "localhost"))
	config.DocGeneratorGrpcPort = cast.ToString(GetOrReturnDefaultValue("NODE_DOC_GENERATOR_GRPC_PORT", ":50051"))

	config.GetRequestRedisHost = cast.ToString(GetOrReturnDefaultValue("GET_REQUEST_REDIS_HOST", "localhost"))
	config.GetRequestRedisPort = cast.ToString(GetOrReturnDefaultValue("GET_REQUEST_REDIS_PORT", "6379"))
	config.GetRequestRedisDatabase = cast.ToInt(GetOrReturnDefaultValue("GET_REQUEST_REDIS_DATABASE", 1))
	config.GetRequestRedisPassword = cast.ToString(GetOrReturnDefaultValue("GET_REQUEST_REDIS_PASSWORD", "D8ihWNRZ9stYaDoPket4KXu1A6TChzqg"))

	config.OpenAIApiKey = cast.ToString(GetOrReturnDefaultValue("OPENAI_API_KEY", "sk-proj-a2ma7TfGU0msgfY9GDsST3BlbkFJljmuOgGattnpsfQCnJ2C"))

	config.AuditMaxBodySize = cast.ToInt64(GetOrReturnDefaultValue("AUDIT_MAX_BODY_SIZE", 256*1024))
	config.AuditMetricsFlushInterval = cast.ToInt(GetOrReturnDefaultValue("METRICS_FLUSH_INTERVAL_SEC", 60))
	config.AuditFlushInterval = cast.ToInt(GetOrReturnDefaultValue("AUDIT_FLUSH_INTERVAL_SEC", 5))
	config.AuditLogChannelSize = cast.ToInt(GetOrReturnDefaultValue("AUDIT_LOG_CHANNEL_SIZE", 100000))
	config.AuditBatchSize = cast.ToInt(GetOrReturnDefaultValue("AUDIT_BATCH_SIZE", 100))
	config.AuditLogRetentionDays = cast.ToInt(GetOrReturnDefaultValue("AUDIT_LOG_RETENTION_DAYS", 30))

	return config
}

func GetOrReturnDefaultValue(key string, defaultValue any) any {
	val, exists := os.LookupEnv(key)
	if exists {
		return val
	}
	return defaultValue
}

// loadGeminiKeys reads API keys from env.
// GEMINI_API_KEYS (comma-separated) takes priority; falls back to GEMINI_API_KEY.
// Returns (keys, primaryKey).
func loadGeminiKeys() ([]string, string) {
	if raw := cast.ToString(GetOrReturnDefaultValue("GEMINI_API_KEYS", "")); raw != "" {
		var keys []string
		for _, k := range strings.Split(raw, ",") {
			if k = strings.TrimSpace(k); k != "" {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			return keys, keys[0]
		}
	}
	if single := cast.ToString(GetOrReturnDefaultValue("GEMINI_API_KEY", "")); single != "" {
		return []string{single}, single
	}
	return nil, ""
}

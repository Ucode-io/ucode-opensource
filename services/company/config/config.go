package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

type Config struct {
	NodePostgresHost           string
	NodePostgresPort           int
	NodePostgresDatabase       string
	NodePostgresUser           string
	NodePostgresPassword       string
	NodePostgresMaxConnections int32

	BuilderServiceHost string
	BuilderServicePort string

	HighBuilderServiceHost string
	HighBuilderServicePort string

	FunctionServiceHost string
	FunctionServicePort string

	AuthServiceHost string
	AuthGRPCPort    string

	GoObjectBuilderServiceHost string
	GoObjectBuilderServicePort string

	TranscoderServiceHost string
	TranscoderServicePort string

	UcodeMongoHost     string
	UcodeMongoPort     string
	UcodeMongoUser     string
	UcodeMongoPassword string

	UcodeClickhouseHost     string
	UcodeClickhousePort     string
	UcodeClickhouseHTTPPort string
	UcodeClickhouseUser     string
	UcodeClickhousePassword string

	AirByteApiUrl   string
	AirByteUsername string
	AirBytePassword string
	AirByteEmail    string

	DatalensBaseURL  string
	DatalensUsername string
	DatalensPassword string

	SupersetBaseUrl    string
	SupersetUsername   string
	SupersetPassword   string
	SupersetDbHost     string
	SupersetDbPort     string
	SupersetDbPassword string
	SupersetDbUsername string

	MetabaseBaseUrl    string
	MetabaseUsername   string
	MetabasePassword   string
	MetabaseDbHost     string
	MetabaseDbPort     string
	MetabaseDbPassword string
	MetabaseDbUsername string
	MetabaseApiKey     string

	UcodeNamespace string
}

type BaseConfig struct {
	ServiceName string
	Environment string // develop, staging, production
	LogLevel    string
	RPCPort     string

	// Limits of a self-hosted installation. Zero means no limit. See
	// grpc/service/limits.go for why these are settings of their own.
	MaxCompanies int
	MaxProjects  int

	PostgresHost           string
	PostgresPort           int
	PostgresDatabase       string
	PostgresUser           string
	PostgresPassword       string
	PostgresMaxConnections int32

	JaegerHostPort string

	SecretsProvider string

	Vault struct {
		Address    string
		RoleID     string
		SecretID   string
		MountPath  string
		SecretPath string
	}

	Redis struct {
		Host     string
		Port     string
		Password string
		DB       int
	}

	UcodeMongoHost     string
	UcodeMongoPort     string
	UcodeMongoUser     string
	UcodeMongoPassword string

	UcodeNamespace string

	PaymeApiUrl string
	PaymeXAuth  string
	PaymeKey    string

	StripeApiKey string

	// Ipak Yo'li E-Comm (Visa/Mastercard hosted-page top-ups).
	IpakEcommBaseUrl     string
	IpakEcommAccessToken string
	IpakEcommTimeoutMs   int
	// IpakHttpProxy, when set, routes ONLY the Ipak Yo'li API calls through this
	// forward proxy (e.g. http://user:pass@<uz-ip>:8888). The bank's test API only
	// accepts Uzbek-region source IPs, so we tunnel through a UZ proxy while the
	// service itself stays on Hetzner. Empty = direct connection.
	IpakHttpProxy string
	// IpakCACert is the bank's CA certificate chain (PEM). The bank uses a local
	// Uzbek CA that isn't in the default trust bundle; provide it via env
	// IPAK_CA_CERT_B64 (base64 of the PEM) to trust the bank's TLS certificate.
	IpakCACert string
	// IpakInsecureSkipVerify disables TLS verification for the Ipak client. Staging
	// escape hatch only (use IpakCACert in production).
	IpakInsecureSkipVerify bool

	Minio struct {
		Endpoint  string
		AccessKey string
		SecretKey string
		Bucket    string
		UseSSL    bool
	}
}

func BaseLoad() BaseConfig {
	if err := godotenv.Load("/app/.env"); err != nil {
		fmt.Println("No .env file found", err)
		godotenv.Load("./.env")
		// godotenv.Load("/../../.env")
	}

	c := BaseConfig{}

	c.ServiceName = cast.ToString(getOrReturnDefault("SERVICE_NAME", ""))
	c.Environment = cast.ToString(getOrReturnDefault("ENVIRONMENT", "production"))
	c.LogLevel = cast.ToString(getOrReturnDefault("LOG_LEVEL", "debug"))
	c.RPCPort = cast.ToString(getOrReturnDefault("RPC_PORT", "8092"))

	c.MaxCompanies = cast.ToInt(getOrReturnDefault("MAX_COMPANIES", 1))
	c.MaxProjects = cast.ToInt(getOrReturnDefault("MAX_PROJECTS", 1))

	c.PostgresHost = cast.ToString(getOrReturnDefault("POSTGRES_HOST", ""))
	c.PostgresPort = cast.ToInt(getOrReturnDefault("POSTGRES_PORT", 5432))
	c.PostgresDatabase = cast.ToString(getOrReturnDefault("POSTGRES_DATABASE", "company_service"))
	c.PostgresUser = cast.ToString(getOrReturnDefault("POSTGRES_USER", "company_service"))
	c.PostgresPassword = cast.ToString(getOrReturnDefault("POSTGRES_PASSWORD", ""))
	c.PostgresMaxConnections = 80

	c.JaegerHostPort = cast.ToString(getOrReturnDefault("JAEGER_URL", "localhost:6831"))

	c.SecretsProvider = cast.ToString(getOrReturnDefault("SECRETS_PROVIDER", "redis"))

	c.Vault.Address = cast.ToString(getOrReturnDefault("VAULT_ADDRESS", ""))
	c.Vault.RoleID = cast.ToString(getOrReturnDefault("VAULT_ROLE_ID", ""))
	c.Vault.SecretID = cast.ToString(getOrReturnDefault("VAULT_SECRET_ID", ""))
	c.Vault.MountPath = cast.ToString(getOrReturnDefault("VAULT_MOUNT_PATH", "ucode"))
	c.Vault.SecretPath = cast.ToString(getOrReturnDefault("VAULT_SECRET_PATH", "k8s/ucode-test"))

	c.Redis.Host = cast.ToString(getOrReturnDefault("GET_REQUEST_REDIS_HOST", "localhost"))
	c.Redis.Port = cast.ToString(getOrReturnDefault("GET_REQUEST_REDIS_PORT", "6379"))
	c.Redis.Password = cast.ToString(getOrReturnDefault("GET_REQUEST_REDIS_PASSWORD", ""))
	c.Redis.DB = cast.ToInt(getOrReturnDefault("GET_REQUEST_REDIS_DATABASE", 0))

	c.UcodeMongoHost = cast.ToString(getOrReturnDefault("MONGO_HOST", ""))
	c.UcodeMongoPort = cast.ToString(getOrReturnDefault("MONGO_PORT", ""))
	c.UcodeMongoUser = cast.ToString(getOrReturnDefault("COMPANY_SERVICE_MONGO_USER", ""))
	c.UcodeMongoPassword = cast.ToString(getOrReturnDefault("COMPANY_SERVICE_MONGO_PASSWORD", ""))

	c.UcodeNamespace = "u-code"

	c.PaymeApiUrl = cast.ToString(getOrReturnDefault("PAYME_API_URL", "https://checkout.test.paycom.uz/api"))
	c.PaymeXAuth = cast.ToString(getOrReturnDefault("PAYME_XAUTH", ""))
	c.PaymeKey = cast.ToString(getOrReturnDefault("PAYME_KEY", ""))

	c.StripeApiKey = cast.ToString(getOrReturnDefault("STRIPE_API_KEY", ""))

	c.IpakEcommBaseUrl = cast.ToString(getOrReturnDefault("IPAK_ECOMM_BASE_URL", "https://partner.ecomm.staging.ipakyulibank.uz/api"))
	c.IpakEcommAccessToken = cast.ToString(getOrReturnDefault("IPAK_ECOMM_ACCESS_TOKEN", ""))
	c.IpakEcommTimeoutMs = cast.ToInt(getOrReturnDefault("IPAK_ECOMM_TIMEOUT_MS", 15000))
	c.IpakHttpProxy = cast.ToString(getOrReturnDefault("IPAK_HTTP_PROXY", ""))
	c.IpakInsecureSkipVerify = cast.ToBool(getOrReturnDefault("IPAK_INSECURE_SKIP_VERIFY", false))
	if b64 := cast.ToString(getOrReturnDefault("IPAK_CA_CERT_B64", "")); b64 != "" {
		if pem, err := base64.StdEncoding.DecodeString(b64); err == nil {
			c.IpakCACert = string(pem)
		}
	}

	c.Minio.Endpoint = cast.ToString(getOrReturnDefault("MINIO_ENDPOINT", ""))
	c.Minio.AccessKey = cast.ToString(getOrReturnDefault("MINIO_ACCESS_KEY", ""))
	c.Minio.SecretKey = cast.ToString(getOrReturnDefault("MINIO_SECRET_KEY", ""))
	c.Minio.Bucket = cast.ToString(getOrReturnDefault("MINIO_BUCKET", "exports"))
	c.Minio.UseSSL = cast.ToBool(getOrReturnDefault("MINIO_USE_SSL", false))

	return c
}

func Load() Config {
	if err := godotenv.Load("/app/.env"); err != nil {
		godotenv.Load("../.env")
		fmt.Println("No .env file found", err)
	}

	c := Config{}

	c.NodePostgresHost = cast.ToString(getOrReturnDefault("POSTGRES_HOST", ""))
	c.NodePostgresPort = cast.ToInt(getOrReturnDefault("POSTGRES_PORT", 0))
	c.NodePostgresDatabase = cast.ToString(getOrReturnDefault("POSTGRES_DATABASE", ""))
	c.NodePostgresUser = cast.ToString(getOrReturnDefault("POSTGRES_USER", ""))
	c.NodePostgresPassword = cast.ToString(getOrReturnDefault("POSTGRES_PASSWORD", ""))
	c.NodePostgresMaxConnections = 80

	c.BuilderServiceHost = cast.ToString(getOrReturnDefault("OBJECT_BUILDER_SERVICE_LOW_HOST", ""))
	c.BuilderServicePort = cast.ToString(getOrReturnDefault("OBJECT_BUILDER_LOW_GRPC_PORT", ""))

	c.HighBuilderServiceHost = cast.ToString(getOrReturnDefault("OBJECT_BUILDER_SERVICE_HIGHT_HOST", ""))
	c.HighBuilderServicePort = cast.ToString(getOrReturnDefault("OBJECT_BUILDER_HIGH_GRPC_PORT", ""))

	c.FunctionServiceHost = cast.ToString(getOrReturnDefault("FUNCTION_SERVICE_HOST", ""))
	c.FunctionServicePort = cast.ToString(getOrReturnDefault("FUNCTION_GRPC_PORT", ""))

	c.AuthServiceHost = cast.ToString(getOrReturnDefault("AUTH_SERVICE_HOST", "localhost"))
	c.AuthGRPCPort = cast.ToString(getOrReturnDefault("AUTH_GRPC_PORT", ":9103"))

	c.GoObjectBuilderServiceHost = cast.ToString(getOrReturnDefault("GO_OBJECT_BUILDER_SERVICE_GRPC_HOST", ""))
	c.GoObjectBuilderServicePort = cast.ToString(getOrReturnDefault("GO_OBJECT_BUILDER_SERVICE_GRPC_PORT", ""))

	c.TranscoderServiceHost = cast.ToString(getOrReturnDefault("TRANSCODER_SERVICE_HOST", "localhost"))
	c.TranscoderServicePort = cast.ToString(getOrReturnDefault("TRANSCODER_GRPC_PORT", ":9110"))

	c.UcodeMongoHost = cast.ToString(getOrReturnDefault("MONGO_HOST", ""))
	c.UcodeMongoPort = cast.ToString(getOrReturnDefault("MONGO_PORT", ""))
	c.UcodeMongoUser = cast.ToString(getOrReturnDefault("COMPANY_SERVICE_MONGO_USER", ""))
	c.UcodeMongoPassword = cast.ToString(getOrReturnDefault("COMPANY_SERVICE_MONGO_PASSWORD", ""))

	c.UcodeClickhouseHost = cast.ToString(getOrReturnDefault("CLICK_HOUSE_HOST", ""))
	c.UcodeClickhousePort = cast.ToString(getOrReturnDefault("CLICK_HOUSE_PORT", ""))
	c.UcodeClickhouseHTTPPort = cast.ToString(getOrReturnDefault("CLICK_HOUSE_HTTP_PORT", ""))
	c.UcodeClickhouseUser = cast.ToString(getOrReturnDefault("CLICK_HOUSE_USER", ""))
	c.UcodeClickhousePassword = cast.ToString(getOrReturnDefault("CLICK_HOUSE_PASSWORD", ""))

	c.AirByteApiUrl = cast.ToString(getOrReturnDefault("AIRBYTE_BASE_URL", ""))
	c.AirByteUsername = cast.ToString(getOrReturnDefault("AIRBYTE_USERNAME", ""))
	c.AirBytePassword = cast.ToString(getOrReturnDefault("AIRBYTE_PASSWORD", ""))
	c.AirByteEmail = cast.ToString(getOrReturnDefault("AIRBYTE_EMAIL", ""))

	c.DatalensBaseURL = cast.ToString(getOrReturnDefault("DATALENS_BASE_URL", ""))
	c.DatalensUsername = cast.ToString(getOrReturnDefault("DATALENS_USERNAME", ""))
	c.DatalensPassword = cast.ToString(getOrReturnDefault("DATALENS_PASSWORD", ""))

	c.SupersetBaseUrl = cast.ToString(getOrReturnDefault("SUPERSET_DOMAIN", ""))
	c.SupersetUsername = cast.ToString(getOrReturnDefault("SUPERSET_LOGIN", ""))
	c.SupersetPassword = cast.ToString(getOrReturnDefault("SUPERSET_PASSWORD", ""))
	c.SupersetDbHost = cast.ToString(getOrReturnDefault("SUPERSET_DB_HOST", ""))
	c.SupersetDbPort = cast.ToString(getOrReturnDefault("SUPERSET_DB_PORT", ""))
	c.SupersetDbPassword = cast.ToString(getOrReturnDefault("SUPERSET_DB_PASSWORD", ""))
	c.SupersetDbUsername = cast.ToString(getOrReturnDefault("SUPERSET_DB_USERNAME", ""))

	c.MetabaseBaseUrl = cast.ToString(getOrReturnDefault("METABASE_DOMAIN", ""))
	c.MetabaseUsername = cast.ToString(getOrReturnDefault("METABASE_LOGIN", ""))
	c.MetabasePassword = cast.ToString(getOrReturnDefault("METABASE_PASSWORD", ""))
	c.MetabaseDbHost = cast.ToString(getOrReturnDefault("METABASE_DB_HOST", ""))
	c.MetabaseDbPort = cast.ToString(getOrReturnDefault("METABASE_DB_PORT", ""))
	c.MetabaseDbPassword = cast.ToString(getOrReturnDefault("METABASE_DB_PASSWORD", ""))
	c.MetabaseDbUsername = cast.ToString(getOrReturnDefault("METABASE_DB_USERNAME", ""))
	c.MetabaseApiKey = cast.ToString(getOrReturnDefault("METABASE_API_KEY", ""))

	c.UcodeNamespace = "u-code"

	return c
}

func getOrReturnDefault(key string, defaultValue any) any {
	_, exists := os.LookupEnv(key)
	if exists {
		return os.Getenv(key)
	}

	return defaultValue
}

// Validate reports configuration that the service cannot run without, so a
// missing value surfaces as a named variable instead of a driver-level parse
// error further down.
func (c BaseConfig) Validate() error {
	var missing []string

	if strings.TrimSpace(c.PostgresHost) == "" {
		missing = append(missing, "POSTGRES_HOST")
	}
	if c.PostgresPort == 0 {
		missing = append(missing, "POSTGRES_PORT")
	}
	if strings.TrimSpace(c.PostgresPassword) == "" {
		missing = append(missing, "POSTGRES_PASSWORD")
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required configuration:\n  - %s", strings.Join(missing, "\n  - "))
	}
	return nil
}

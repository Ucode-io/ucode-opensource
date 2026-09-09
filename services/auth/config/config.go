package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/spf13/cast"
)

const (
	// DebugMode indicates service mode is debug.
	DebugMode = "debug"
	// TestMode indicates service mode is test.
	TestMode = "test"
	// ReleaseMode indicates service mode is release.
	ReleaseMode = "release"
)

type Config struct {
	DefaultOffset string
	DefaultLimit  string

	SecretKey string

	PasscodePool   string
	PasscodeLength int

	ObjectBuilderServiceHost string
	ObjectBuilderGRPCPort    string

	HighObjectBuilderServiceHost string
	HighObjectBuilderGRPCPort    string

	SmsServiceHost string
	SmsGRPCPort    string

	WebPageServiceHost string
	WebPageServicePort string

	GoObjectBuilderServiceHost string
	GoObjectBuilderServicePort string

	UcodeAppBaseUrl string
}

type BaseConfig struct {
	ServiceName string
	Environment string
	Version     string

	HTTPPort   string
	HTTPScheme string

	Email         string
	EmailPassword string

	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string

	PostgresMaxConnections int32
	DefaultOffset          string
	DefaultLimit           string

	SmsServiceHost string
	SmsGRPCPort    string

	SecretKey string

	AuthServiceHost string
	AuthGRPCPort    string

	UcodeNamespace string

	CompanyServiceHost string
	CompanyGRPCPort    string

	JaegerHostPort string

	GetRequestRedisHost     string
	GetRequestRedisPort     string
	GetRequestRedisDatabase int
	GetRequestRedisPassword string

	FirebaseAPIKey  string
	FirebaseBaseUrl string

	EImzoBaseUrl  string
	EImzoHost     string
	EImzoUsername string
	EImzoPassword string

	GoogleClientID     string
	GoogleClientSecret string
	GoogleCallbackURL  string
	FrontendURL        string
	CookieDomain       string
	AuthAllowedOrigins string
}

func BaseLoad() BaseConfig {

	if err := godotenv.Load("/app/.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("Error loading .env file")
		}
	}

	config := BaseConfig{}

	config.DefaultOffset = cast.ToString(getOrReturnDefaultValue("DEFAULT_OFFSET", "0"))
	config.DefaultLimit = cast.ToString(getOrReturnDefaultValue("DEFAULT_LIMIT", "100"))

	config.ServiceName = cast.ToString(getOrReturnDefaultValue("SERVICE_NAME", "auth_service"))
	config.Environment = cast.ToString(getOrReturnDefaultValue("ENVIRONMENT", DebugMode))
	config.Version = cast.ToString(getOrReturnDefaultValue("VERSION", "1.0"))

	config.HTTPPort = cast.ToString(getOrReturnDefaultValue("HTTP_PORT", ""))
	config.HTTPScheme = cast.ToString(getOrReturnDefaultValue("HTTP_SCHEME", ""))
	config.Email = cast.ToString(getOrReturnDefaultValue("EMAIL", ""))
	config.EmailPassword = cast.ToString(getOrReturnDefaultValue("EMAIL_PASSWORD", ""))

	config.PostgresHost = cast.ToString(getOrReturnDefaultValue("POSTGRES_HOST", ""))
	config.PostgresPort = cast.ToInt(getOrReturnDefaultValue("POSTGRES_PORT", 0))
	config.PostgresUser = cast.ToString(getOrReturnDefaultValue("POSTGRES_USER", ""))
	config.PostgresPassword = cast.ToString(getOrReturnDefaultValue("POSTGRES_PASSWORD", ""))
	config.PostgresDatabase = cast.ToString(getOrReturnDefaultValue("POSTGRES_DATABASE", ""))
	config.PostgresMaxConnections = cast.ToInt32(getOrReturnDefaultValue("POSTGRES_MAX_CONNECTIONS", 200))

	config.AuthServiceHost = cast.ToString(getOrReturnDefaultValue("AUTH_SERVICE_HOST", ""))
	config.AuthGRPCPort = cast.ToString(getOrReturnDefaultValue("AUTH_GRPC_PORT", ""))

	config.UcodeNamespace = "u-code"

	config.CompanyServiceHost = cast.ToString(getOrReturnDefaultValue("COMPANY_SERVICE_HOST", ""))
	config.CompanyGRPCPort = cast.ToString(getOrReturnDefaultValue("COMPANY_GRPC_PORT", ""))

	config.SmsServiceHost = cast.ToString(getOrReturnDefaultValue("SMS_SERVICE_HOST", ""))
	config.SmsGRPCPort = cast.ToString(getOrReturnDefaultValue("SMS_GRPC_PORT", ""))

	config.SecretKey = cast.ToString(getOrReturnDefaultValue("SECRET_KEY", "snZV9XNmvf"))

	config.JaegerHostPort = cast.ToString(getOrReturnDefaultValue("JAEGER_URL", ""))

	config.GetRequestRedisHost = cast.ToString(getOrReturnDefaultValue("GET_REQUEST_REDIS_HOST", "localhost"))
	config.GetRequestRedisPort = cast.ToString(getOrReturnDefaultValue("GET_REQUEST_REDIS_PORT", "6379"))
	config.GetRequestRedisDatabase = cast.ToInt(getOrReturnDefaultValue("GET_REQUEST_REDIS_DATABASE", 0))
	config.GetRequestRedisPassword = cast.ToString(getOrReturnDefaultValue("GET_REQUEST_REDIS_PASSWORD", ""))

	config.FirebaseAPIKey = cast.ToString(getOrReturnDefaultValue("FIREBASE_API_KEY", ""))
	config.FirebaseBaseUrl = cast.ToString(getOrReturnDefaultValue("FIREBASE_BASE_URL", ""))

	config.EImzoBaseUrl = cast.ToString(getOrReturnDefaultValue("EIMZO_BASE_URL", ""))
	config.EImzoHost = cast.ToString(getOrReturnDefaultValue("EIMZO_HOST", ""))
	config.EImzoUsername = cast.ToString(getOrReturnDefaultValue("EIMZO_USERNAME", ""))
	config.EImzoPassword = cast.ToString(getOrReturnDefaultValue("EIMZO_PASSWORD", ""))

	config.GoogleClientID = cast.ToString(getOrReturnDefaultValue("GOOGLE_CLIENT_ID", ""))
	config.GoogleClientSecret = cast.ToString(getOrReturnDefaultValue("GOOGLE_CLIENT_SECRET", ""))
	config.GoogleCallbackURL = cast.ToString(getOrReturnDefaultValue("GOOGLE_CALLBACK_URL", ""))
	config.FrontendURL = cast.ToString(getOrReturnDefaultValue("FRONTEND_URL", ""))
	config.CookieDomain = cast.ToString(getOrReturnDefaultValue("COOKIE_DOMAIN", ""))
	config.AuthAllowedOrigins = cast.ToString(getOrReturnDefaultValue("AUTH_ALLOWED_ORIGINS", ""))

	return config
}

// Load ....
func Load() Config {
	if err := godotenv.Load("/app/.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("Error loading .env file")
		}
	}

	config := Config{}

	config.DefaultOffset = cast.ToString(getOrReturnDefaultValue("DEFAULT_OFFSET", "0"))
	config.DefaultLimit = cast.ToString(getOrReturnDefaultValue("DEFAULT_LIMIT", "100"))

	config.SecretKey = cast.ToString(getOrReturnDefaultValue("SECRET_KEY", ""))

	config.PasscodePool = cast.ToString(getOrReturnDefaultValue("PASSCODE_POOL", ""))
	config.PasscodeLength = cast.ToInt(getOrReturnDefaultValue("PASSCODE_LENGTH", ""))

	config.ObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_LOW_HOST", ""))
	config.ObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_LOW_GRPC_PORT", ""))

	config.HighObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_HIGHT_HOST", ""))
	config.HighObjectBuilderGRPCPort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_HIGH_GRPC_PORT", ""))

	config.SmsServiceHost = cast.ToString(getOrReturnDefaultValue("SMS_SERVICE_HOST", ""))
	config.SmsGRPCPort = cast.ToString(getOrReturnDefaultValue("SMS_GRPC_PORT", ""))

	config.WebPageServiceHost = cast.ToString(getOrReturnDefaultValue("WEB_PAGE_SERVICE_HOST", ""))
	config.WebPageServicePort = cast.ToString(getOrReturnDefaultValue("WEB_PAGE_GRPC_PORT", ""))

	config.GoObjectBuilderServiceHost = cast.ToString(getOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_HOST", ""))
	config.GoObjectBuilderServicePort = cast.ToString(getOrReturnDefaultValue("GO_OBJECT_BUILDER_SERVICE_GRPC_PORT", ""))

	config.UcodeAppBaseUrl = cast.ToString(getOrReturnDefaultValue("UCODE_APP_BASE_URL", ""))

	return config
}

func getOrReturnDefaultValue(key string, defaultValue any) any {
	val, exists := os.LookupEnv(key)

	if exists {
		return val
	}

	return defaultValue
}

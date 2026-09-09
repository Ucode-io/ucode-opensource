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
	ServiceName string
	ServiceHost string
	ServicePort string

	Environment string // debug, test, release
	Version     string

	JaegerHostPort string

	PostgresHost     string
	PostgresPort     int
	PostgresUser     string
	PostgresPassword string
	PostgresDatabase string

	AuthServiceHost string
	AuthGRPCPort    string

	CompanyServiceHost string
	CompanyServicePort string

	TranscoderServiceHost string
	TranscoderServicePort string

	NodeType     string
	K8sNamespace string

	MinioHost        string
	MinioAccessKeyID string
	MinioSecretKey   string
	MinioSSL         bool

	PostgresMaxConnections int32
}

func (c Config) SafeLogFields() map[string]any {
	return map[string]any{
		"ServiceName":            c.ServiceName,
		"ServiceHost":            c.ServiceHost,
		"ServicePort":            c.ServicePort,
		"Environment":            c.Environment,
		"Version":                c.Version,
		"JaegerHostPort":         c.JaegerHostPort,
		"PostgresHost":           c.PostgresHost,
		"PostgresPort":           c.PostgresPort,
		"PostgresUser":           c.PostgresUser,
		"PostgresPassword":       redact(c.PostgresPassword),
		"PostgresDatabase":       c.PostgresDatabase,
		"AuthServiceHost":        c.AuthServiceHost,
		"AuthGRPCPort":           c.AuthGRPCPort,
		"CompanyServiceHost":     c.CompanyServiceHost,
		"CompanyServicePort":     c.CompanyServicePort,
		"TranscoderServiceHost":  c.TranscoderServiceHost,
		"TranscoderServicePort":  c.TranscoderServicePort,
		"NodeType":               c.NodeType,
		"K8sNamespace":           c.K8sNamespace,
		"MinioHost":              c.MinioHost,
		"MinioAccessKeyID":       redact(c.MinioAccessKeyID),
		"MinioSecretKey":         redact(c.MinioSecretKey),
		"MinioSSL":               c.MinioSSL,
		"PostgresMaxConnections": c.PostgresMaxConnections,
	}
}

func redact(value string) string {
	if value == "" {
		return ""
	}
	return "***"
}

// Load ...
func Load() Config {
	if err := godotenv.Load("/app/.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("No .env file found")
		}
		log.Println("No .env file found")
	}

	config := Config{}

	config.ServiceName = cast.ToString(getOrReturnDefaultValue("SERVICE_NAME", "ucode"))
	config.ServiceHost = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_HOST", "localhost"))
	config.ServicePort = cast.ToString(getOrReturnDefaultValue("OBJECT_BUILDER_SERVICE_PORT", ":7107"))

	config.Environment = cast.ToString(getOrReturnDefaultValue("ENVIRONMENT", DebugMode))
	config.Version = cast.ToString(getOrReturnDefaultValue("VERSION", "1.0"))

	config.JaegerHostPort = cast.ToString(getOrReturnDefaultValue("JAEGER_URL", ""))

	config.PostgresHost = cast.ToString(getOrReturnDefaultValue("POSTGRES_HOST", ""))
	config.PostgresPort = cast.ToInt(getOrReturnDefaultValue("POSTGRES_PORT", 5432))
	config.PostgresUser = cast.ToString(getOrReturnDefaultValue("POSTGRES_USER", ""))
	config.PostgresPassword = cast.ToString(getOrReturnDefaultValue("POSTGRES_PASSWORD", ""))
	config.PostgresDatabase = cast.ToString(getOrReturnDefaultValue("POSTGRES_DATABASE", ""))

	config.AuthServiceHost = cast.ToString(getOrReturnDefaultValue("AUTH_SERVICE_HOST", "localhost"))
	config.AuthGRPCPort = cast.ToString(getOrReturnDefaultValue("AUTH_GRPC_PORT", ":9103"))

	config.CompanyServiceHost = cast.ToString(getOrReturnDefaultValue("COMPANY_SERVICE_HOST", "localhost"))
	config.CompanyServicePort = cast.ToString(getOrReturnDefaultValue("COMPANY_GRPC_PORT", ":8092"))

	config.TranscoderServiceHost = cast.ToString(getOrReturnDefaultValue("TRANSCODER_SERVICE_HOST", "localhost"))
	config.TranscoderServicePort = cast.ToString(getOrReturnDefaultValue("TRANSCODER_GRPC_PORT", ":9110"))

	config.NodeType = cast.ToString(getOrReturnDefaultValue("NODE_TYPE", "LOW"))
	config.K8sNamespace = cast.ToString(getOrReturnDefaultValue("K8S_NAMESPACE", "cp-region-type-id"))

	config.MinioAccessKeyID = cast.ToString(getOrReturnDefaultValue("MINIO_ACCESS_KEY", ""))
	config.MinioSecretKey = cast.ToString(getOrReturnDefaultValue("MINIO_SECRET_KEY", ""))
	config.MinioHost = cast.ToString(getOrReturnDefaultValue("MINIO_ENDPOINT", ""))
	config.MinioSSL = cast.ToBool(getOrReturnDefaultValue("MINIO_SSL", true))

	config.PostgresMaxConnections = cast.ToInt32(getOrReturnDefaultValue("POSTGRES_MAX_CONNECTIONS", 500))

	return config
}

func getOrReturnDefaultValue(key string, defaultValue any) any {
	val, exists := os.LookupEnv(key)

	if exists {
		return val
	}

	return defaultValue
}

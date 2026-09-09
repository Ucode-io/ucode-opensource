package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/manveru/faker"
	"github.com/stretchr/testify/assert"
)

var (
	err      error
	cfg      config.BaseConfig
	strg     storage.StorageI
	fakeData *faker.Faker
)

// POSTGRES_HOST="65.109.239.69"
// POSTGRES_PORT=5432
// POSTGRES_DATABASE="company_service"
// POSTGRES_USER="company_service"
// POSTGRES_PASSWORD="fgd4dfFFDJFSd23o"

func CreateRandomId(t *testing.T) string {
	id, err := uuid.NewRandom()
	assert.NoError(t, err)
	return id.String()
}

func TestMain(m *testing.M) {
	var (
		loggerLevel string
		cfg         = config.BaseLoad()
	)

	switch cfg.Environment {
	case config.DebugMode:
		loggerLevel = logger.LevelDebug
		gin.SetMode(gin.DebugMode)
	case config.TestMode:
		loggerLevel = logger.LevelDebug
		gin.SetMode(gin.TestMode)
	default:
		loggerLevel = logger.LevelInfo
		gin.SetMode(gin.ReleaseMode)
	}

	log := logger.NewLogger(cfg.ServiceName, loggerLevel)
	defer func() {
		_ = logger.Cleanup(log)
	}()

	cfg.PostgresPassword = "Iegfrte45eatr7ieso"
	cfg.PostgresHost = "65.109.239.69"
	cfg.PostgresPort = 5432
	cfg.PostgresDatabase = "auth_service"
	cfg.PostgresUser = "auth_service"

	strg, err = postgres.NewPostgres(context.Background(), cfg, log)

	fakeData, _ = faker.New("en")

	os.Exit(m.Run())
}

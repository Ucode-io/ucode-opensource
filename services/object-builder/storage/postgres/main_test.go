package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage/postgres"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaswdr/faker/v2"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
)

const (
	roleId    = "5381a752-0652-4da2-acfc-0dea5082a21e"
	projectId = "a95e3cba-bfc0-46c8-aeaa-ef75ee9986ed"
)

var (
	err error
	// cfg      config.Config
	strg     storage.StorageI
	fakeData faker.Faker
)

func CreateRandomId(t *testing.T) string {
	id, err := uuid.NewRandom()
	assert.NoError(t, err)
	return id.String()
}

// the code should take the config from the environment
func TestMain(m *testing.M) {
	var (
		loggerLevel string
		cfg         = config.Load()
	)

	cfg.PostgresUser = ""
	cfg.PostgresPassword = ""
	cfg.PostgresHost = ""
	cfg.PostgresPort = 5432
	cfg.PostgresDatabase = "test"
	cfg.PostgresMaxConnections = 10

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

	strg, err = postgres.NewPostgres(context.Background(), cfg, nil, log)
	if err != nil {
		panic(err)
	}

	dbURL := fmt.Sprintf(
		"postgres://%v:%v@%v:%v/%v?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDatabase,
	)

	config, err := pgxpool.ParseConfig(dbURL)
	if err != nil {
		panic(err)
	}

	config.MaxConns = cfg.PostgresMaxConnections

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		panic(err)
	}

	psqlpool.Add(projectId, &psqlpool.Pool{Db: pool})

	fakeData = faker.New()

	os.Exit(m.Run())
}

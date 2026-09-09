//go:build integration

// Package postgres_test integration bootstrap.
//
// Needs a real PostgreSQL, which the suite starts as a throwaway container and
// migrates itself. Run with:
//
//	go test -tags=integration ./...
//
// Docker must be available; nothing else has to be configured.
package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage/postgres"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/lib/pq"
	"github.com/manveru/faker"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

// Relative to this package directory, which is the working directory
// while the test binary runs.
const migrationsPath = "../../migrations/postgres"

// Throwaway credentials for the disposable container. Deliberately generated
// here rather than read from config, .env or anything in the repository: test
// runs must never be able to reach a real database.
var (
	dbName = "ucode_test"
	dbUser = "ucode_test"
	dbPass = uuid.NewString()
)

var (
	err      error
	strg     storage.StorageI
	fakeData *faker.Faker
)

func CreateRandomId(t *testing.T) string {
	id, err := uuid.NewRandom()
	assert.NoError(t, err)
	return id.String()
}

func TestMain(m *testing.M) {
	// run() owns all cleanup; os.Exit below would skip deferred calls.
	os.Exit(run(m))
}

func run(m *testing.M) int {
	ctx := context.Background()

	container, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase(dbName),
		tcpostgres.WithUsername(dbUser),
		tcpostgres.WithPassword(dbPass),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "start postgres container: %v\n", err)
		return 1
	}
	defer func() {
		if err := testcontainers.TerminateContainer(container); err != nil {
			fmt.Fprintf(os.Stderr, "terminate postgres container: %v\n", err)
		}
	}()

	dsn, err := container.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		fmt.Fprintf(os.Stderr, "container connection string: %v\n", err)
		return 1
	}

	if err := applyMigrations(dsn); err != nil {
		fmt.Fprintf(os.Stderr, "apply migrations: %v\n", err)
		return 1
	}

	// Host and port come from the parsed DSN rather than the container API,
	// so this does not depend on testcontainers' port type.
	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "parse pool config: %v\n", err)
		return 1
	}

	// Built explicitly instead of config.BaseLoad(): the suite must not pick up
	// a stray .env or exported variables.
	cfg := config.BaseConfig{
		ServiceName:            "auth-test",
		Environment:            config.TestMode,
		PostgresHost:           poolCfg.ConnConfig.Host,
		PostgresPort:           int(poolCfg.ConnConfig.Port),
		PostgresUser:           dbUser,
		PostgresPassword:       dbPass,
		PostgresDatabase:       dbName,
		PostgresMaxConnections: 10,
	}

	gin.SetMode(gin.TestMode)
	log := logger.NewLogger(cfg.ServiceName, logger.LevelDebug)
	defer func() { _ = logger.Cleanup(log) }()

	strg, err = postgres.NewPostgres(ctx, cfg, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open service storage: %v\n", err)
		return 1
	}

	fakeData, _ = faker.New("en")

	return m.Run()
}

func applyMigrations(dsn string) error {
	mig, err := migrate.New("file://"+migrationsPath, dsn)
	if err != nil {
		return err
	}
	defer func() {
		sourceErr, dbErr := mig.Close()
		if sourceErr != nil {
			fmt.Fprintf(os.Stderr, "close migration source: %v\n", sourceErr)
		}
		if dbErr != nil {
			fmt.Fprintf(os.Stderr, "close migration db: %v\n", dbErr)
		}
	}()

	if err := mig.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

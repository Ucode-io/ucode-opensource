//go:build integration

// Package postgres_test integration bootstrap.
//
// These tests need a real PostgreSQL. Instead of pointing at an external
// server, the suite starts a throwaway container, applies the service
// migrations to it and tears it down afterwards. Run with:
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

	"github.com/Ucode-io/ucode-opensource/services/object-builder/config"
	initialsetup "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/initial_setup"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"
	psqlpool "github.com/Ucode-io/ucode-opensource/services/object-builder/pool"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/storage/postgres"

	"github.com/gin-gonic/gin"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jaswdr/faker/v2"
	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	roleId       = "5381a752-0652-4da2-acfc-0dea5082a21e"
	projectId    = "a95e3cba-bfc0-46c8-aeaa-ef75ee9986ed"
	userId       = "1f0a4c2e-6b3d-4a71-9c58-0e2b7d5a1f34"
	clientTypeId = "8c6d2b19-4e07-4f52-b3a8-91d6c0f7e2ab"

	// Relative to this package directory, which is the working directory
	// while the test binary runs.
	migrationsPath = "../../migrations/postgres"
)

var (
	err      error
	strg     storage.StorageI
	fakeData faker.Faker
)

// Throwaway credentials for the disposable container. Deliberately generated
// here rather than read from config, .env or anything in the repository: test
// runs must never be able to reach a real database.
var (
	dbName = "ucode_test"
	dbUser = "ucode_test"
	dbPass = uuid.NewString()
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

	// Built explicitly instead of config.Load(): the suite must not pick up a
	// stray .env or exported variables, and MinIO stays unset on purpose so a
	// test that reaches for object storage fails loudly rather than silently
	// talking to something real.
	cfg := config.Config{
		ServiceName:            "object-builder-test",
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

	strg, err = postgres.NewPostgres(ctx, cfg, nil, log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open service storage: %v\n", err)
		return 1
	}

	// The engine resolves every per-project connection through psqlpool,
	// so the suite has to register one for the project id the tests use.
	poolCfg.MaxConns = cfg.PostgresMaxConnections

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open project pool: %v\n", err)
		return 1
	}
	defer pool.Close()

	psqlpool.Add(projectId, &psqlpool.Pool{Db: pool})

	// Migrations alone leave a bare schema. Real projects are also seeded with
	// default roles, client types and permissions by the provisioning path
	// (grpc/service/builder_project.go). Without this the suite would be
	// testing a state that never exists in production.
	if err := seedDefaults(pool); err != nil {
		fmt.Fprintf(os.Stderr, "seed default project data: %v\n", err)
		return 1
	}

	fakeData = faker.New()

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

// seedDefaults mirrors helper.InsertDatas, which the real provisioning path runs
// after migrating a new project database (grpc/service/builder_project.go).
//
// The one step left out is initialsetup.CreateFiles: it provisions a MinIO
// bucket, and object storage is irrelevant to the storage-layer CRUD this suite
// covers. Keep this list in sync with helper.InsertDatas when that changes.
func seedDefaults(conn *pgxpool.Pool) error {
	clientPlatformId := uuid.NewString()
	testLoginId := uuid.NewString()

	steps := []struct {
		name string
		run  func() error
	}{
		{"client platform", func() error {
			return initialsetup.CreateDefaultClientPlatform(conn, clientPlatformId, clientTypeId, projectId)
		}},
		{"client type", func() error {
			return initialsetup.CreateDefaultClientType(conn, clientPlatformId, clientTypeId, projectId)
		}},
		{"role", func() error {
			return initialsetup.CreateDefaultRole(conn, roleId, clientPlatformId, clientTypeId, projectId)
		}},
		{"field permission", func() error { return initialsetup.CreateDefaultFieldPermission(conn, roleId) }},
		{"global permission", func() error { return initialsetup.CreateDefaultGlobalPermission(conn, roleId) }},
		{"record permission", func() error { return initialsetup.CreateDefaultRecordPermission(conn, roleId) }},
		{"test login", func() error { return initialsetup.CreateDefaultTestLogin(conn, testLoginId, clientTypeId) }},
		{"user", func() error {
			return initialsetup.CreateDefaultUser(conn, userId, roleId, clientTypeId, clientPlatformId, projectId)
		}},
		{"view relation permission", func() error { return initialsetup.CreateDefaultViewRelationPermission(conn, roleId) }},
		{"view permission", func() error { return initialsetup.CreateDefaultViewPermission(conn, roleId) }},
	}

	for _, step := range steps {
		if err := step.run(); err != nil {
			return fmt.Errorf("%s: %w", step.name, err)
		}
	}
	return nil
}

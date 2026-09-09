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

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/postgres"

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
	err       error
	strg      storage.StorageI
	fakeData  *faker.Faker
	projectId = "d6b56eaa-f81d-4778-bc8a-285c8ac92a8f"
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
	// a stray .env or exported variables. Vault, MinIO and the payment
	// providers stay unset on purpose.
	cfg := config.BaseConfig{
		ServiceName:            "company-test",
		Environment:            "test",
		PostgresHost:           poolCfg.ConnConfig.Host,
		PostgresPort:           int(poolCfg.ConnConfig.Port),
		PostgresUser:           dbUser,
		PostgresPassword:       dbPass,
		PostgresDatabase:       dbName,
		PostgresMaxConnections: 10,
	}

	strg, err = postgres.NewPostgres(ctx, cfg, nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "open service storage: %v\n", err)
		return 1
	}

	if err := seedProject(ctx, poolCfg); err != nil {
		fmt.Fprintf(os.Stderr, "seed project fixture: %v\n", err)
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

// seedProject inserts the project row the suite's fixtures hang off. Rows in
// transaction, fare and friends carry a foreign key to project, so a freshly
// migrated database has nothing they can reference. Previously these tests only
// passed because they ran against a live database where the project happened to
// exist.
func seedProject(ctx context.Context, poolCfg *pgxpool.Config) error {
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	_, err = pool.Exec(ctx,
		`INSERT INTO project (id, title, k8s_namespace)
		 VALUES ($1, 'test project', 'test')
		 ON CONFLICT (id) DO NOTHING`,
		projectId,
	)
	return err
}

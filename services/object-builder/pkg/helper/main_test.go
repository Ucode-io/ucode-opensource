package helper_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/manveru/faker"
	"github.com/stretchr/testify/assert"
)

var (
	conn     *pgxpool.Pool
	fakeData *faker.Faker
)

func CreateRandomId(t *testing.T) string {
	id, err := uuid.NewRandom()
	assert.NoError(t, err)
	return id.String()
}

func TestMain(m *testing.M) {
	postgresPassword := "oka"
	postgresHost := "65.109.239.69"
	postgresPort := 5432
	postgresDatabase := "login_psql_5e9c087aca884920be1936cb20ca56f9_p_postgres_svcs"
	postgresUser := "login_psql_5e9c087aca884920be1936cb20ca56f9_p_postgres_svcs"

	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		postgresUser,
		postgresPassword,
		postgresHost,
		postgresPort,
		postgresDatabase,
	))
	if err != nil {
		return
	}
	config.MaxConns = 30

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return
	}
	defer pool.Close()

	conn = pool

	fakeData, _ = faker.New("en")
	os.Exit(m.Run())
}

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/postgres"

	"github.com/google/uuid"
	_ "github.com/lib/pq"
	"github.com/manveru/faker"
	"github.com/stretchr/testify/assert"
)

var (
	err       error
	cfg       config.BaseConfig
	strg      storage.StorageI
	fakeData  *faker.Faker
	projectId string = "d6b56eaa-f81d-4778-bc8a-285c8ac92a8f"
)

func CreateRandomId(t *testing.T) string {
	id, err := uuid.NewRandom()
	assert.NoError(t, err)
	return id.String()
}

func TestMain(m *testing.M) {
	cfg = config.BaseLoad()

	strg, err = postgres.NewPostgres(context.Background(), cfg, nil)

	fakeData, _ = faker.New("en")

	os.Exit(m.Run())
}

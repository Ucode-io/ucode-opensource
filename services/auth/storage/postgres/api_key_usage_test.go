//go:build integration

package postgres_test

import (
	"context"
	"testing"
	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/stretchr/testify/assert"
)

func apiKeyUsageBulkUpsert(t *testing.T) {
	usage := &pb.ApiKeyUsage{
		// A fixture, not a key: this only has to be a distinct string in a
		// throwaway database. The real format is "P-" and 32 characters.
		ApiKey:       "P-test000000000000000000000000000",
		RequestCount: 21,
	}

	err := strg.ApiKeyUsage().Upsert(context.Background(), usage)
	assert.NoError(t, err)

}
 
func TestApiKeyUsageBulkUpsert(t *testing.T) {
	apiKeyUsageBulkUpsert(t)
}
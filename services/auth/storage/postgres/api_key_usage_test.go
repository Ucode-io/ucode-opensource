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
		ApiKey:       "P-iTSqj09ucK9IySQIoL6cr6T8lk23MYAB",
		RequestCount: 21,
	}

	err := strg.ApiKeyUsage().Upsert(context.Background(), usage)
	assert.NoError(t, err)

}
 
func TestApiKeyUsageBulkUpsert(t *testing.T) {
	apiKeyUsageBulkUpsert(t)
}
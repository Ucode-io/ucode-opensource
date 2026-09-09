package postgres_test

import (
	"context"
	"testing"
	pb "github.com/Ucode-io/ucode-opensource/services/company/genproto/company_service"

	"github.com/stretchr/testify/assert"
)

func TestGetResourceByEnvID(t *testing.T) {
	resp, err := strg.Resource().GetResourceByEnvID(context.Background(), &pb.GetResourceByEnvIDRequest{
		EnvId: "f6461617-c9b9-4bcb-bcaa-b6e443a6f755",
	})
	assert.NoError(t, err)
	assert.NotEmpty(t, resp)
}

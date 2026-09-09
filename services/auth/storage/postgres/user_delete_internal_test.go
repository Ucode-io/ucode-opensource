package postgres

import (
	"strings"
	"testing"

	pb "github.com/Ucode-io/ucode-opensource/services/auth/genproto/auth_service"

	"github.com/stretchr/testify/require"
)

func TestDeleteUserFromProjectQueryUsesExactRegistrationScope(t *testing.T) {
	req := &pb.DeleteSyncUserRequest{
		UserId:        "user-id",
		CompanyId:     "company-id",
		ProjectId:     "project-id",
		ClientTypeId:  "client-type-id",
		RoleId:        "role-id",
		EnvironmentId: "environment-id",
	}

	query, params := deleteUserFromProjectQuery(req)

	for _, column := range []string{
		"user_id",
		"company_id",
		"project_id",
		"client_type_id",
		"role_id",
		"env_id",
	} {
		require.Contains(t, query, column+" = :")
	}
	require.Equal(t, map[string]any{
		"user_id":        "user-id",
		"company_id":     "company-id",
		"project_id":     "project-id",
		"client_type_id": "client-type-id",
		"role_id":        "role-id",
		"env_id":         "environment-id",
	}, params)
}

func TestDeleteUserFromProjectQueryOmitsUnknownOptionalScope(t *testing.T) {
	query, params := deleteUserFromProjectQuery(&pb.DeleteSyncUserRequest{
		UserId:    "user-id",
		ProjectId: "project-id",
	})

	require.NotContains(t, query, "company_id")
	require.NotContains(t, query, "client_type_id")
	require.NotContains(t, query, "role_id")
	require.NotContains(t, query, "env_id")
	require.False(t, strings.HasSuffix(strings.TrimSpace(query), "AND"))
	require.Equal(t, map[string]any{
		"user_id":    "user-id",
		"project_id": "project-id",
	}, params)
}

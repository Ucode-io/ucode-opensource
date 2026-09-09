//go:build integration

package helper_test

import (
	"testing"
	initialsetup "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/initial_setup"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func Test_InitialInsert(t *testing.T) {
	var (
		clientTypeId     = uuid.NewString()
		roleId           = uuid.NewString()
		clientPlatformId = uuid.NewString()
		projectId        = uuid.NewString()
		testLoginId      = uuid.NewString()
		userId           = uuid.NewString()
	)

	err := initialsetup.CreateDefaultClientPlatform(conn, clientPlatformId, clientTypeId, projectId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultClientType(conn, clientPlatformId, clientTypeId, projectId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultRole(conn, roleId, clientPlatformId, clientTypeId, projectId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultFieldPermission(conn, roleId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultGlobalPermission(conn, roleId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultRecordPermission(conn, roleId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultTestLogin(conn, testLoginId, clientTypeId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultUser(conn, userId, roleId, clientTypeId, clientPlatformId, projectId)
	assert.NoError(t, err)

	err = initialsetup.CreateDefaultViewRelationPermission(conn, roleId)
	assert.NoError(t, err)
}
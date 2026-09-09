package helper

import (
	"fmt"
	initialsetup "github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/initial_setup"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func InsertDatas(conn *pgxpool.Pool, userId, projectId, clientTypeId, roleId, resourceEnvId string) error {
	if clientTypeId == "" {
		clientTypeId = uuid.NewString()
	}

	if roleId == "" {
		roleId = uuid.NewString()
	}

	if projectId == "" {
		projectId = uuid.NewString()
	}

	if userId == "" {
		userId = uuid.NewString()
	}

	var (
		clientPlatformId = uuid.NewString()
		testLoginId      = uuid.NewString()
	)

	err := initialsetup.CreateDefaultClientPlatform(conn, clientPlatformId, clientTypeId, projectId)
	if err != nil {
		return fmt.Errorf("CreateDefaultClientPlatform - %v", err)
	}

	err = initialsetup.CreateDefaultClientType(conn, clientPlatformId, clientTypeId, projectId)
	if err != nil {
		return fmt.Errorf("CreateDefaultClientType - %v", err)
	}

	err = initialsetup.CreateDefaultRole(conn, roleId, clientPlatformId, clientTypeId, projectId)
	if err != nil {
		return fmt.Errorf("CreateDefaultRole - %v", err)
	}

	err = initialsetup.CreateDefaultFieldPermission(conn, roleId)
	if err != nil {
		return fmt.Errorf("CreateDefaultFieldPermission - %v", err)
	}

	err = initialsetup.CreateDefaultGlobalPermission(conn, roleId)
	if err != nil {
		return fmt.Errorf("CreateDefaultGlobalPermission - %v", err)
	}

	err = initialsetup.CreateDefaultRecordPermission(conn, roleId)
	if err != nil {
		return fmt.Errorf("CreateDefaultRecordPermission - %v", err)
	}

	err = initialsetup.CreateDefaultTestLogin(conn, testLoginId, clientTypeId)
	if err != nil {
		return fmt.Errorf("CreateDefaultTestLogin - %v", err)
	}

	err = initialsetup.CreateDefaultUser(conn, userId, roleId, clientTypeId, clientPlatformId, projectId)
	if err != nil {
		return fmt.Errorf("CreateDefaultUser - %v", err)
	}

	err = initialsetup.CreateDefaultViewRelationPermission(conn, roleId)
	if err != nil {
		return fmt.Errorf("CreateDefaultViewRelationPermission - %v", err)
	}

	err = initialsetup.CreateDefaultViewPermission(conn, roleId)
	if err != nil {
		return fmt.Errorf("CreateDefaultViewPermission - %v", err)
	}

	err = initialsetup.CreateFiles(conn, resourceEnvId)
	if err != nil {
		return fmt.Errorf("CreateFiles - %v", err)
	}

	return nil
}

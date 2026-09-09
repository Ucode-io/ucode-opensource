package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultRole(conn *pgxpool.Pool, roleId, clientPlatformId, clientTypeId, projectId string) error {
	query := `INSERT INTO role (guid, project_id, client_platform_id, client_type_id, name, is_system) 
	VALUES ($1, $2, $3, $4, 'DEFAULT ADMIN', false)`

	_, err := conn.Exec(context.Background(), query, roleId, projectId, clientPlatformId, clientTypeId)
	if err != nil {
		return err
	}
	return nil
}

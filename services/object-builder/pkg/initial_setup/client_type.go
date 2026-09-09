package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultClientType(conn *pgxpool.Pool, clientPlatformId, clientTypeId, projectId string) error {
	query := `INSERT INTO client_type (guid, project_id, name, confirm_by, self_register, self_recover, client_platform_ids, is_system) 
	VALUES ($1, $2, 'ADMIN', 'PHONE', FALSE, FALSE, $3, TRUE)`

	_, err := conn.Exec(context.Background(), query, clientTypeId, projectId, []string{clientPlatformId})
	if err != nil {
		return err
	}
	return nil
}

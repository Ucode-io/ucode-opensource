package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultClientPlatform(conn *pgxpool.Pool, clientPlatformId, clientTypeId, projectId string) error {
	query := `INSERT INTO client_platform (guid, project_id, name, subdomain, client_type_ids) 
        VALUES ($1, $2, 'Ucode', 'ucode', $3)`

	_, err := conn.Exec(context.Background(), query, clientPlatformId, projectId, []string{clientTypeId})
	if err != nil {
		return err
	}
	return nil
}

func CheckMax(conn *pgxpool.Pool, tableName string) error {
	query := `ALTER TABLE incrementseqs ADD CONSTRAINT increment_by_less_than_max CHECK (increment_by <= max_value);`

	_, err := conn.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	return nil
}

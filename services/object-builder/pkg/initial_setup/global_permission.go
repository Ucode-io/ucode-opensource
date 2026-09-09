package initialsetup

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultGlobalPermission(conn *pgxpool.Pool, roleId string) error {
	query := `INSERT INTO global_permission (guid, role_id) VALUES
	($1, $2)
	`

	_, err := conn.Exec(context.Background(), query, uuid.NewString(), roleId)
	if err != nil {
		return err
	}
	return nil
}

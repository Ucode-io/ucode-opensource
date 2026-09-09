package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultViewRelationPermission(conn *pgxpool.Pool, roleId string) error {
	query := `INSERT INTO "view_relation_permission"( guid, role_id, table_slug, relation_id )
	VALUES ('4a186d1d-8ebd-4828-9a77-a7a6da245976', $1, 'client_platform', '426a0cd6-958d-4317-bf23-3b4ea4720e53')`

	_, err := conn.Exec(context.Background(), query, roleId)
	if err != nil {
		return err
	}
	return nil
}

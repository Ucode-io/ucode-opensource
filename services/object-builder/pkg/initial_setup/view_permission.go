package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultViewPermission(conn *pgxpool.Pool, roleId string) error {
	query := `INSERT INTO "view_permission" ( guid, role_id, view_id )
	VALUES 
		('aaeac0f2-160e-4351-8cff-7fe90291cf09', $1, '8f1fde99-cc81-4bb6-87ff-bb86acaa73ff'),
		('23378136-1b98-46d2-b5af-489282457fb1', $1, '88e3001b-b62c-4d6c-9b12-c7f920f6331f'),
		('e399f76f-539f-4d67-a84a-8bf86af2684e', $1, '0db9b1a2-00cd-4ce0-897c-5a71a764639a')
	`

	_, err := conn.Exec(context.Background(), query, roleId)
	if err != nil {
		return err
	}
	return nil
}

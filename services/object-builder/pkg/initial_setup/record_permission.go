package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultRecordPermission(conn *pgxpool.Pool, roleId string) error {
	query := `INSERT INTO record_permission (table_slug, guid, role_id) 
			  VALUES 
			  ('language_setting', '3d78d446-4e3a-4e9a-a100-cd4ad099e3d1', $1),
			  ('timezone_setting', 'e5806ca9-f7b5-44e8-a88a-876bfb7ec8f9', $1),
			  ('record_permission', '75e835f6-06ab-46d1-88ca-76ed10bf2f06', $1),
			  ('connection', '55243b6e-63fb-4e28-ba07-8625b3e43738', $1),
			  ('role', '380e14ab-71bc-4558-a3b4-537dbd4d57f4', $1),
			  ('client_type', '42fb56bf-bfbc-4f8f-96fb-1c2567d91b79', $1),
			  ('client_platform', '3b850c28-deff-4e2d-8a3c-f6e47e5c4d26', $1),
			  ('project', '1c04479e-f549-48f8-b5cb-5fa7e929e296', $1),
			  ('test_login', '2953079b-0bcd-4b32-8322-751720d4db78', $1),
			  ('user', '9389ed9e-6cf4-442d-9268-d46820b721b2', $1),
			  ('automatic_filter', 'e40b13da-f913-45c9-a3d2-83e1212192c7', $1),
			  ('person', '078eee53-895d-4fe2-b944-eca454db2e91', $1);`

	_, err := conn.Exec(context.Background(), query, roleId)
	if err != nil {
		return err
	}

	return nil
}

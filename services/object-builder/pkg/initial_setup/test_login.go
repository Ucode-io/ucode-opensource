package initialsetup

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func CreateDefaultTestLogin(conn *pgxpool.Pool, testLoginId, clientTypeId string) error {
	query := `INSERT INTO test_login (guid, login_strategy, table_slug, login_label, login_view, password_view, password_label, object_id, client_type_id)
	VALUES 
	($1, 'Login with password', 'user', 'Логин', 'login', 'password', 'Парол', '2546e042-af2f-4cef-be7c-834e6bde951c', $2);
	`

	_, err := conn.Exec(context.Background(), query, testLoginId, clientTypeId)
	if err != nil {
		return err
	}
	return nil
}

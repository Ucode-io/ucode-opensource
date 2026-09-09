package scripts

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// sqlQuoteLiteral экранирует одинарные кавычки для SQL-литералов.
func sqlQuoteLiteral(s string) string {
	return strings.ReplaceAll(s, `'`, `''`)
}

func EditDatabasePermissions(dbName, dbUser string, dbConn *pgxpool.Pool) error {
	u := sqlQuoteLiteral(dbUser)
	d := sqlQuoteLiteral(dbName)

	query := fmt.Sprintf(`
DO $$
DECLARE
  p_user text := '%s';   -- <-- имя пользователя
  p_db   text := '%s';   -- <-- его база
  r record;
  u record;
BEGIN
  -- проверки
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = p_user) THEN
    RAISE EXCEPTION 'role "%%" does not exist', p_user;
  END IF;
  IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = p_db) THEN
    RAISE EXCEPTION 'database "%%" does not exist', p_db;
  END IF;

  -- 1) пользователь НЕ суперюзер (иначе обойдёт ACL)
  EXECUTE format('ALTER ROLE %%I NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION', p_user);

  -- 2) На его БД: только он может CONNECT
  EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM PUBLIC', p_db);
  FOR u IN SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname <> p_user LOOP
    EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM %%I', p_db, u.rolname);
  END LOOP;
  EXECUTE format('GRANT CONNECT ON DATABASE %%I TO %%I', p_db, p_user);

  -- 3) На других БД: запретить этому пользователю, но никого другого не ломать
  FOR r IN SELECT datname FROM pg_database WHERE datistemplate = false AND datname <> p_db LOOP
    EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM %%I', r.datname, p_user);

    -- если на БД был CONNECT у PUBLIC — заменим его явными GRANT для всех логин-ролей (кроме этого пользователя)
    IF EXISTS (
      SELECT 1
      FROM pg_database d
      CROSS JOIN LATERAL aclexplode(d.datacl) AS priv
      WHERE d.datname = r.datname
        AND priv.grantee = 0          -- 0 = PUBLIC
        AND priv.privilege_type = 'CONNECT'
    ) THEN
      EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM PUBLIC', r.datname);
      FOR u IN SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname <> p_user LOOP
        EXECUTE format('GRANT CONNECT ON DATABASE %%I TO %%I', r.datname, u.rolname);
      END LOOP;
    END IF;
  END LOOP;

  -- 4) Владелец БД = пользователь + TEMP/CREATE
  EXECUTE format('ALTER DATABASE %%I OWNER TO %%I', p_db, p_user);
  EXECUTE format('GRANT TEMP ON DATABASE %%I TO %%I', p_db, p_user);
  EXECUTE format('GRANT CREATE ON DATABASE %%I TO %%I', p_db, p_user);
END
$$;`, u, d)

	// без параметров — внутри DO $$ нельзя использовать $1/$2
	_, err := dbConn.Exec(context.Background(), query)
	return err
}

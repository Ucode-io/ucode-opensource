package postgres_client

import (
	"context"
	"fmt"
	"log"
	"github.com/Ucode-io/ucode-opensource/services/company/config"

	"github.com/jackc/pgx/v4/pgxpool"
)

type Store struct {
	db *pgxpool.Pool
}
type CreatePostgresUser struct {
	Db       string
	UserName string
	Pass     string
}

type DeletePostgresUser struct {
	Db       string
	UserName string
}

func NewPostgres(ctx context.Context, cfg config.Config) (*Store, error) {
	log.Printf(
		"NewPostgres: connecting to postgres://%s:***@%s:%d/%s?sslmode=disable",
		cfg.NodePostgresUser,
		cfg.NodePostgresHost,
		cfg.NodePostgresPort,
		cfg.NodePostgresDatabase,
	)

	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.NodePostgresUser,
		cfg.NodePostgresPassword,
		cfg.NodePostgresHost,
		cfg.NodePostgresPort,
		cfg.NodePostgresDatabase,
	))
	if err != nil {
		return nil, err
	}

	config.MaxConns = cfg.NodePostgresMaxConnections

	pool, err := pgxpool.ConnectConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	return &Store{
		db: pool,
	}, err
}

func (s *Store) CloseDB() {
	s.db.Close()
}

func (s *Store) CreateUserAndDatabase(ctx context.Context, user CreatePostgresUser) error {

	var (
		sql = fmt.Sprintf(
			`CREATE USER "%s" WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOREPLICATION PASSWORD '%s'`,
			user.UserName, user.Pass)

		dbSql = fmt.Sprintf(
			`CREATE DATABASE "%s" WITH OWNER "%s" TEMPLATE template1`,
			user.Db, user.UserName)

		iso = fmt.Sprintf(`
DO $$
DECLARE
   p_user text := '%s';
   p_db   text := '%s';
   r record;
   u record;
BEGIN
EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM PUBLIC', p_db);
FOR u IN SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname <> p_user LOOP
   EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM %%I', p_db, u.rolname);
END LOOP;
EXECUTE format('GRANT CONNECT ON DATABASE %%I TO %%I', p_db, p_user);

FOR r IN
    SELECT datname
    FROM pg_database
    WHERE datistemplate = false AND datname <> p_db
LOOP
    EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM %%I', r.datname, p_user);

    IF EXISTS (
       SELECT 1
       FROM pg_database d
       CROSS JOIN LATERAL aclexplode(d.datacl) AS priv
       WHERE d.datname = r.datname
           AND priv.grantee = 0
           AND priv.privilege_type = 'CONNECT'
       ) THEN
    EXECUTE format('REVOKE CONNECT ON DATABASE %%I FROM PUBLIC', r.datname);
    FOR u IN SELECT rolname FROM pg_roles WHERE rolcanlogin AND rolname <> p_user LOOP
        EXECUTE format('GRANT CONNECT ON DATABASE %%I TO %%I', r.datname, u.rolname);
    END LOOP;
    END IF;
END LOOP;
END
$$;`, user.UserName, user.Db)
	)

	if _, err := s.db.Exec(ctx, sql); err != nil {
		log.Printf("credentials(username,password): %s:%s", user.UserName, user.Pass)
		log.Println("error while create postgres user:", err.Error())
		return err
	}

	if _, err := s.db.Exec(ctx, dbSql); err != nil {
		log.Printf("credentials(db): %s", user.Db)
		log.Println("error while create postgres database:", err.Error())
		return err
	}

	if _, err := s.db.Exec(ctx, iso); err != nil {
		log.Println("error while isolating access:", err.Error())
		return err
	}

	return nil
}

func (s *Store) DeleteUserAndDatabase(ctx context.Context, user DeletePostgresUser) error {
	// FIX: терминируем все активные коннекты к БД перед дропом,
	// иначе DROP DATABASE вернёт ошибку "other users are using the database"
	terminateConns := fmt.Sprintf(`
SELECT pg_terminate_backend(pid)
FROM pg_stat_activity
WHERE datname = '%s' AND pid <> pg_backend_pid()`, user.Db)

	if _, err := s.db.Exec(ctx, terminateConns); err != nil {
		log.Printf("credentials(db): %s", user.Db)
		log.Println("error while terminating connections:", err.Error())
		// не фатально — пробуем дропнуть всё равно
	}

	var deleteDatabase = fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, user.Db)
	_, err := s.db.Exec(ctx, deleteDatabase)
	if err != nil {
		log.Printf("credentials(db): %s", user.Db)
		log.Println("error while dropping postgres database", err.Error())
		return err
	}

	// FIX: DROP ROLE CASCADE не существует в PostgreSQL — это и была причина
	// "syntax error at or near CASCADE" в логах.
	// Нужно сначала снять все объекты принадлежащие роли, потом дропать.
	var reassign = fmt.Sprintf(`REASSIGN OWNED BY "%s" TO current_user`, user.UserName)
	if _, err = s.db.Exec(ctx, reassign); err != nil {
		// роль могла уже не иметь объектов — не фатально
		log.Printf("warn: reassign owned by %q: %v", user.UserName, err)
	}

	var dropOwned = fmt.Sprintf(`DROP OWNED BY "%s"`, user.UserName)
	if _, err = s.db.Exec(ctx, dropOwned); err != nil {
		log.Printf("warn: drop owned by %q: %v", user.UserName, err)
	}

	var deleteRole = fmt.Sprintf(`DROP ROLE IF EXISTS "%s"`, user.UserName)
	_, err = s.db.Exec(ctx, deleteRole)
	if err != nil {
		log.Printf("credentials(username): %s", user.UserName)
		log.Println("error while dropping postgres role", err.Error())
		return err
	}

	return nil
}

package postgres

import (
	"context"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/auth/config"
	"github.com/Ucode-io/ucode-opensource/services/auth/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/auth/storage"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opentracing/opentracing-go"
)

type Store struct {
	db             *Pool
	logger         logger.LoggerI
	clientPlatform storage.ClientPlatformRepoI
	clientType     storage.ClientTypeRepoI
	client         storage.ClientRepoI
	user           storage.UserRepoI
	session        storage.SessionRepoI
	company        storage.CompanyRepoI
	project        storage.ProjectRepoI
	apiKeys        storage.ApiKeysRepoI
	appleId        storage.AppleSettingsI
	apiKeyUsage    storage.ApiKeyUsageRepoI
}

type Pool struct {
	db *pgxpool.Pool
}

func (b *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.QueryRow")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", args)

	return b.db.QueryRow(ctx, sql, args...)
}

func (b *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.Query")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", args)

	return b.db.Query(ctx, sql, args...)
}

func (b *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.Exec")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", arguments)

	return b.db.Exec(ctx, sql, arguments...)
}

func (b *Pool) Begin(ctx context.Context) (pgx.Tx, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.Begin")
	defer dbSpan.Finish()

	tx, err := b.db.Begin(ctx)
	if err != nil {
		dbSpan.SetTag("error", true)
		dbSpan.LogKV("error.message", err.Error())
		return nil, err
	}

	return tx, nil
}

func (b *Pool) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.SendBatch")
	defer dbSpan.Finish()

	dbSpan.SetTag("batch_size", batch.Len())
	dbSpan.SetTag("batch_queued_queries", batch.QueuedQueries)

	return b.db.SendBatch(ctx, batch)
}

func NewPostgres(ctx context.Context, cfg config.BaseConfig, logger logger.LoggerI) (storage.StorageI, error) {
	config, err := pgxpool.ParseConfig(fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.PostgresUser,
		cfg.PostgresPassword,
		cfg.PostgresHost,
		cfg.PostgresPort,
		cfg.PostgresDatabase,
	))
	if err != nil {
		return nil, err
	}

	config.MaxConns = cfg.PostgresMaxConnections

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	dbPool := &Pool{
		db: pool,
	}

	return &Store{
		db:     dbPool,
		logger: logger,
	}, err
}

func (s *Store) CloseDB() {
	s.db.db.Close()
}

func (s *Store) ClientPlatform() storage.ClientPlatformRepoI {
	if s.clientPlatform == nil {
		s.clientPlatform = NewClientPlatformRepo(s.db)
	}

	return s.clientPlatform
}

func (s *Store) ClientType() storage.ClientTypeRepoI {
	if s.clientType == nil {
		s.clientType = NewClientTypeRepo(s.db)
	}

	return s.clientType
}

func (s *Store) Client() storage.ClientRepoI {
	if s.client == nil {
		s.client = NewClientRepo(s.db)
	}

	return s.client
}

func (s *Store) User() storage.UserRepoI {
	if s.user == nil {
		s.user = NewUserRepo(s.db, s.logger)
	}

	return s.user
}

func (s *Store) Session() storage.SessionRepoI {
	if s.session == nil {
		s.session = NewSessionRepo(s.db)
	}

	return s.session
}

func (s *Store) Company() storage.CompanyRepoI {
	if s.company == nil {
		s.company = NewCompanyRepo(s.db)
	}
	return s.company
}

func (s *Store) Project() storage.ProjectRepoI {
	if s.project == nil {
		s.project = NewProjectRepo(s.db)
	}
	return s.project
}

func (s *Store) ApiKeys() storage.ApiKeysRepoI {
	if s.apiKeys == nil {
		s.apiKeys = NewApiKeysRepo(s.db)
	}
	return s.apiKeys
}

func (s *Store) AppleSettings() storage.AppleSettingsI {
	if s.appleId == nil {
		s.appleId = NewAppleSettingsRepo(s.db)
	}
	return s.appleId
}

func (s *Store) ApiKeyUsage() storage.ApiKeyUsageRepoI {
	if s.apiKeyUsage == nil {
		s.apiKeyUsage = NewApiKeyUsageRepo(s.db)
	}
	return s.apiKeyUsage
}

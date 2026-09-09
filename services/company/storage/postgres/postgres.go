package postgres

import (
	"context"
	"fmt"

	"github.com/Ucode-io/ucode-opensource/services/company/config"
	"github.com/Ucode-io/ucode-opensource/services/company/pkg/logger"
	"github.com/Ucode-io/ucode-opensource/services/company/storage"
	"github.com/Ucode-io/ucode-opensource/services/company/storage/repo"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opentracing/opentracing-go"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Store struct {
	db                      *Pool
	logger                  logger.Logger
	companyStorageI         repo.CompanyStorageI
	environmentRepo         repo.EnvironmentStorageI
	projectRepo             repo.ProjectStorageI
	resourceRepo            repo.ResourceStorageI
	serviceResourceRepo     repo.ServiceResourceStorageI
	redirectRepo            repo.RedirectStorageI
	airbyteRepo             repo.AirbyteStorageI
	billingRepo             repo.BillingStorageI
	templateMetadataRepo    repo.TemplateStorageI
	integrationResourceRepo repo.IntegrationResourceStorageI
	ugenTemplateRepo        repo.UgenTemplateStorageI
	mfeShortLinkRepo        repo.MfeShortLinkStorageI
}

type Pool struct {
	db     *pgxpool.Pool
	logger logger.Logger
	cfg    config.BaseConfig
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

	return &Tx{Tx: tx, ctx: ctx}, nil
}

func (b *Pool) HandleDatabaseError(err error, message string) error {
	if err == nil {
		return nil
	}

	if err == pgx.ErrNoRows {
		return status.Error(codes.NotFound, "not found")
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		b.logger.Error(message+": "+err.Error(), logger.String("column", pgErr.ColumnName))

		switch pgErr.Code {
		case "23505":
			// Unique violation
			return status.Error(codes.AlreadyExists, err.Error())
		case "23503":
			// Foreign key violation
			return status.Error(codes.FailedPrecondition, fmt.Sprintf("foreign key violation: %v", pgErr.Message))
		case "23514":
			// Check constraint violation
			return status.Error(codes.InvalidArgument, fmt.Sprintf("check constraint violation: %v", pgErr.Message))
		case "23502":
			// Not null violation
			return status.Error(codes.InvalidArgument, fmt.Sprintf("not null violation: %v", pgErr.Message))
		case "08006":
			// Connection failure
			return status.Error(codes.Unavailable, fmt.Sprintf("connection failure: %v", pgErr.Message))
		case "28P01":
			// Invalid password
			return status.Error(codes.Unauthenticated, fmt.Sprintf("invalid password: %v", pgErr.Message))
		case "3D000":
			// Invalid catalog name (Database not found)
			return status.Error(codes.NotFound, fmt.Sprintf("database not found: %v", pgErr.Message))
		case "42P01":
			// Undefined table
			return status.Error(codes.NotFound, fmt.Sprintf("undefined table: %v", pgErr.Message))
		case "42703":
			// Undefined column
			return status.Error(codes.InvalidArgument, fmt.Sprintf("undefined column: %v", pgErr.Message))
		case "40P01":
			// Deadlock detected
			return status.Error(codes.Aborted, fmt.Sprintf("deadlock detected: %v", pgErr.Message))

		// --- Transaction Errors ---
		case "25P01":
			// No active SQL transaction
			return status.Error(codes.FailedPrecondition, "no active transaction")
		case "25P02":
			// Transaction is in an aborted state
			return status.Error(codes.Aborted, "transaction is aborted, commands ignored until end of transaction block")
		case "25P03":
			// Idle in transaction
			return status.Error(codes.FailedPrecondition, "transaction is idle and waiting")
		case "40001":
			// Serialization failure (common in concurrent transactions)
			return status.Error(codes.Aborted, "serialization failure, retry transaction")
		case "40003":
			// Statement completion unknown
			return status.Error(codes.Unknown, "statement completion unknown due to transaction state")
		case "0A000":
			// Feature not supported (e.g., SAVEPOINT in some cases)
			return status.Error(codes.Unimplemented, fmt.Sprintf("feature not supported: %v", pgErr.Message))
		case "22003":
			// Numeric value out of range
			return status.Error(codes.OutOfRange, fmt.Sprintf("numeric value out of range: %v", pgErr.Message))
		case "42601":
			// Syntax error
			return status.Error(codes.InvalidArgument, fmt.Sprintf("syntax error: %v", pgErr.Message))

		// --- Dependency & Schema Errors ---
		case "2BP01":
			// Dependent objects still exist
			return status.Error(codes.FailedPrecondition, "cannot drop or modify the object because dependent objects exist")
		case "42P07":
			// Duplicate table
			return status.Error(codes.AlreadyExists, "table already exists")
		case "42P18":
			// Indeterminate data type
			return status.Error(codes.InvalidArgument, "indeterminate data type error")
		case "42P19":
			// Invalid recursion
			return status.Error(codes.InvalidArgument, "invalid recursion detected")
		case "42P20":
			// Windowing error
			return status.Error(codes.InvalidArgument, "window function error")
		case "42P21":
			// Collation mismatch
			return status.Error(codes.InvalidArgument, "collation mismatch detected")
		case "42P22":
			// Indeterminate collation
			return status.Error(codes.InvalidArgument, "indeterminate collation error")

		default:
			// Handle other PostgreSQL-specific errors
			return status.Error(codes.Internal, fmt.Sprintf("postgres error: %v", pgErr.Message))
		}
	}

	return status.Error(codes.Internal, fmt.Sprintf("%v", err))
}

type Tx struct {
	pgx.Tx
	ctx context.Context
}

func (tx *Tx) Commit(ctx context.Context) error {
	dbSpan, _ := opentracing.StartSpanFromContext(ctx, "pgx.Commit")
	defer dbSpan.Finish()

	err := tx.Tx.Commit(ctx) // Use context for pgx.Tx.Commit
	if err != nil {
		dbSpan.SetTag("error", true)
		dbSpan.LogKV("error.message", err.Error())
	}
	return err
}

func (tx *Tx) Rollback(ctx context.Context) error {
	dbSpan, _ := opentracing.StartSpanFromContext(ctx, "pgx.Rollback")
	defer dbSpan.Finish()

	err := tx.Tx.Rollback(ctx) // Use context for pgx.Tx.Rollback
	if err != nil {
		dbSpan.SetTag("error", true)
		dbSpan.LogKV("error.message", err.Error())
	}
	return err
}

func (tx *Tx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	dbSpan, _ := opentracing.StartSpanFromContext(ctx, "pgx.TxQuery")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", args)

	return tx.Tx.Query(ctx, sql, args...)
}

func (tx *Tx) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	dbSpan, _ := opentracing.StartSpanFromContext(ctx, "pgx.TxExec")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", arguments)

	return tx.Tx.Exec(ctx, sql, arguments...)
}

func NewPostgres(ctx context.Context, cfg config.BaseConfig, logger logger.Logger) (storage.StorageI, error) {
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

	return &Store{
		db: &Pool{
			db:     pool,
			logger: logger,
			cfg:    cfg,
		},
		logger: logger,
	}, err
}

func (s *Store) CloseDB() {
	s.db.db.Close()
}

func (s *Store) Company() repo.CompanyStorageI {
	if s.companyStorageI == nil {
		s.companyStorageI = NewCompanyRepo(s.db)
	}

	return s.companyStorageI
}

func (s *Store) Project() repo.ProjectStorageI {
	if s.projectRepo == nil {
		s.projectRepo = NewProjectRepo(s.db, s.logger)
	}
	return s.projectRepo
}

func (s *Store) Environment() repo.EnvironmentStorageI {
	if s.environmentRepo == nil {
		s.environmentRepo = NewEnvironmentRepo(s.db)
	}
	return s.environmentRepo
}

func (s *Store) Resource() repo.ResourceStorageI {
	if s.resourceRepo == nil {
		s.resourceRepo = NewResourceRepo(s.db)
	}
	return s.resourceRepo
}

func (s *Store) ServiceResource() repo.ServiceResourceStorageI {
	if s.serviceResourceRepo == nil {
		s.serviceResourceRepo = NewServiceResource(s.db)
	}
	return s.serviceResourceRepo
}

func (s *Store) Redirect() repo.RedirectStorageI {
	if s.redirectRepo == nil {
		s.redirectRepo = NewRedirectRepo(s.db)
	}
	return s.redirectRepo
}

func (s *Store) Airbyte() repo.AirbyteStorageI {
	if s.airbyteRepo == nil {
		s.airbyteRepo = NewAirbyteRepo(s.db)
	}
	return s.airbyteRepo
}

func (s *Store) Billing() repo.BillingStorageI {
	if s.billingRepo == nil {
		s.billingRepo = NewBillingRepo(s.db, s.logger, s.db.cfg)
	}
	return s.billingRepo
}

func (s *Store) TemplateMetadata() repo.TemplateStorageI {
	if s.templateMetadataRepo == nil {
		s.templateMetadataRepo = NewTemplateMetadataRepo(s.db)
	}
	return s.templateMetadataRepo
}

func (s *Store) IntegrationResource() repo.IntegrationResourceStorageI {
	if s.integrationResourceRepo == nil {
		s.integrationResourceRepo = NewIntegrationResourceRepo(s.db)
	}
	return s.integrationResourceRepo
}

func (s *Store) UgenTemplate() repo.UgenTemplateStorageI {
	if s.ugenTemplateRepo == nil {
		s.ugenTemplateRepo = NewUgenTemplateRepo(s.db)
	}
	return s.ugenTemplateRepo
}

func (s *Store) MfeShortLink() repo.MfeShortLinkStorageI {
	if s.mfeShortLinkRepo == nil {
		s.mfeShortLinkRepo = NewMfeShortLinkRepo(s.db)
	}
	return s.mfeShortLinkRepo
}

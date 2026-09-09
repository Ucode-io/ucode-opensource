package psqlpool

import (
	"context"
	"errors"
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/opentracing/opentracing-go"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var PsqlPool = make(map[string]*Pool) // there we save psql connections by project_id

type Pool struct {
	Db     *pgxpool.Pool
	Logger logger.LoggerI
}

func (p *Pool) HandleDatabaseError(err error, message string) error {
	if err == nil {
		return nil
	}

	if err == pgx.ErrNoRows {
		p.Logger.Error("not found", logger.Error(err))
		return status.Error(codes.NotFound, "not found")
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		p.Logger.Error(message+": "+err.Error(), logger.String("column", pgErr.ColumnName))

		// fmt.Println("column", pgErr.ColumnName)
		// fmt.Println("constraint", pgErr.ConstraintName)
		// fmt.Println("detail", pgErr.Detail)
		// fmt.Println("hint", pgErr.Hint)
		// fmt.Println("internal query", pgErr.InternalQuery)
		// fmt.Println("line", pgErr.Line)
		// fmt.Println("message", pgErr.Message)
		// fmt.Println("position", pgErr.Position)
		// fmt.Println("routine", pgErr.Routine)
		// fmt.Println("schema", pgErr.SchemaName)
		// fmt.Println("severity", pgErr.Severity)
		// fmt.Println("table", pgErr.TableName)
		// fmt.Println("where", pgErr.Where)
		// fmt.Println("code", pgErr.Code)
		// fmt.Println("file", pgErr.File)
		// fmt.Println("sql state", pgErr.SQLState())
		// fmt.Println("Message", pgErr.Message)

		switch pgErr.Code {
		case "23505":
			// Unique violation
			return status.Error(codes.AlreadyExists, pgErr.Detail)
		case "23503":
			// Foreign key violation
			return status.Error(codes.FailedPrecondition, fmt.Sprintf("foreign key violation: %v", pgErr.Message))
		case "23514":
			// Check constraint violation
			return status.Error(codes.InvalidArgument, pgErr.Message)
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

	return err
}

func (b *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.QueryRow")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", args)

	return b.Db.QueryRow(ctx, sql, args...)
}

func (b *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.Query")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", args)

	return b.Db.Query(ctx, sql, args...)
}

func (b *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.Exec")
	defer dbSpan.Finish()

	dbSpan.SetTag("sql", sql)
	dbSpan.SetTag("args", arguments)

	return b.Db.Exec(ctx, sql, arguments...)
}

func (b *Pool) Begin(ctx context.Context) (pgx.Tx, error) {
	dbSpan, ctx := opentracing.StartSpanFromContext(ctx, "pgx.Begin")
	defer dbSpan.Finish()

	tx, err := b.Db.Begin(ctx)
	if err != nil {
		dbSpan.SetTag("error", true)
		dbSpan.LogKV("error.message", err.Error())
		return nil, err
	}

	return &Tx{Tx: tx, ctx: ctx}, nil
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

func Add(projectId string, conn *Pool) {
	if projectId == "" {
		return
	}

	_, ok := PsqlPool[projectId]
	if ok {
		return
	}

	PsqlPool[projectId] = conn
}

func Get(projectId string) (conn *Pool, err error) {
	if projectId == "" {
		return nil, errors.New("project id is empty")
	}

	_, ok := PsqlPool[projectId]
	if !ok {
		return nil, errors.New("connection not found")
	}

	return PsqlPool[projectId], nil
}

func Remove(projectId string) {
	if projectId == "" {
		return
	}

	_, ok := PsqlPool[projectId]
	if !ok {
		return
	}

	delete(PsqlPool, projectId)
}

func Override(projectId string, conn *Pool) {
	if projectId == "" {
		return
	}

	_, ok := PsqlPool[projectId]
	if !ok {
		return
	}

	PsqlPool[projectId] = conn
}

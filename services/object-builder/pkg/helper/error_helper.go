package helper

import (
	"fmt"
	"github.com/Ucode-io/ucode-opensource/services/object-builder/pkg/logger"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func HandleDatabaseError(err error, log logger.LoggerI, message string) error {
	if err == nil {
		return nil
	}

	if err == pgx.ErrNoRows {
		return status.Error(codes.NotFound, "not found")
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		log.Error(message+": "+err.Error(), logger.String("column", pgErr.ColumnName))

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

	return status.Error(codes.Internal, fmt.Sprintf("unknown error: %v", err))
}

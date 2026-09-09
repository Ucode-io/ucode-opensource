package v1

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

type SQLType string

const (
	SQLTypeSelect SQLType = "select"
	SQLTypeInsert SQLType = "insert"
	SQLTypeUpdate SQLType = "update"
	SQLTypeDelete SQLType = "delete"
)

// ─────────────────────────────────────────────────────────────────────────────
// Compiled regexps — built once at startup, zero allocation per call.
// ─────────────────────────────────────────────────────────────────────────────

var (
	reSQLLineComment  = regexp.MustCompile(`--[^\n]*`)
	reSQLBlockComment = regexp.MustCompile(`(?s)/\*.*?\*/`)

	// Detects a semicolon that is followed by a non-whitespace character,
	// which indicates a second SQL statement hiding after the first one.
	// Example: "SELECT 1; DROP TABLE users" → match at "; D"
	reSQLMultiStatement = regexp.MustCompile(`;\s*\S`)
)

// sqlForbiddenPatterns covers every operation an AI must never generate:
//   - DDL  : DROP, CREATE, ALTER, TRUNCATE
//   - DCL  : GRANT, REVOKE, SET ROLE, RESET ALL, SECURITY DEFINER
//   - Admin: VACUUM, REINDEX, CLUSTER, COPY, LISTEN, NOTIFY, LOAD
//   - FS   : pg_read_file, pg_write_file, lo_export, lo_import
//   - Sys  : pg_catalog, information_schema, pg_shadow, pg_authid
var sqlForbiddenPatterns = func() []*regexp.Regexp {
	raw := []string{
		`(?i)\bDROP\b`,
		`(?i)\bCREATE\b`,
		`(?i)\bALTER\b`,
		`(?i)\bTRUNCATE\b`,
		`(?i)\bGRANT\b`,
		`(?i)\bREVOKE\b`,
		`(?i)\bVACUUM\b`,
		`(?i)\bREINDEX\b`,
		`(?i)\bCLUSTER\b`,
		`(?i)\bCOPY\b`,
		`(?i)\bLISTEN\b`,
		`(?i)\bNOTIFY\b`,
		`(?i)\bLOAD\b`,
		`(?i)\bSECURITY\s+DEFINER\b`,
		`(?i)\bSET\s+ROLE\b`,
		`(?i)\bRESET\s+ALL\b`,
		`(?i)\bpg_read_file\b`,
		`(?i)\bpg_write_file\b`,
		`(?i)\blo_export\b`,
		`(?i)\blo_import\b`,
		// System catalog tables — AI should never touch them
		`(?i)\bpg_catalog\b`,
		`(?i)\binformation_schema\b`,
		`(?i)\bpg_shadow\b`,
		`(?i)\bpg_authid\b`,
		`(?i)\bpg_auth_members\b`,
	}
	compiled := make([]*regexp.Regexp, 0, len(raw))
	for _, p := range raw {
		compiled = append(compiled, regexp.MustCompile(p))
	}
	return compiled
}()

// ─────────────────────────────────────────────────────────────────────────────
// Public API
// ─────────────────────────────────────────────────────────────────────────────

// ValidateAndClassifySQL validates a SQL statement for safety and returns its
// classified type. Two layers of defence:
//
//  1. Structural checks (multi-statement, forbidden patterns, system objects).
//  2. Leading-keyword classification → only SELECT/WITH/INSERT/UPDATE/DELETE pass.
func ValidateAndClassifySQL(rawSQL string) (SQLType, error) {
	if strings.TrimSpace(rawSQL) == "" {
		return "", fmt.Errorf("SQL statement is empty")
	}

	// Strip comments BEFORE pattern matching.
	// Without this, "SELECT 1 /* DROP TABLE x */" would incorrectly fail.
	stripped := stripSQLComments(rawSQL)

	// ── 1. No multiple statements ────────────────────────────────────────────
	// Prevents injection via appended statements: "SELECT 1; DROP TABLE users"
	if reSQLMultiStatement.MatchString(stripped) {
		return "", fmt.Errorf("multiple SQL statements in a single request are not allowed")
	}

	// ── 2. Forbidden operations ──────────────────────────────────────────────
	for _, re := range sqlForbiddenPatterns {
		if re.MatchString(stripped) {
			return "", fmt.Errorf("forbidden SQL operation detected (pattern: %s)", re.String())
		}
	}

	// ── 3. Classify by leading keyword ───────────────────────────────────────
	upper := strings.ToUpper(strings.TrimSpace(stripped))

	switch {
	case strings.HasPrefix(upper, "SELECT"):
		return SQLTypeSelect, nil

	case strings.HasPrefix(upper, "WITH"):
		// CTE — could be WITH ... SELECT or WITH ... INSERT/UPDATE/DELETE
		return classifyCTEStatement(upper), nil

	case strings.HasPrefix(upper, "INSERT"):
		return SQLTypeInsert, nil

	case strings.HasPrefix(upper, "UPDATE"):
		return SQLTypeUpdate, nil

	case strings.HasPrefix(upper, "DELETE"):
		return SQLTypeDelete, nil

	default:
		return "", fmt.Errorf("unsupported SQL statement type: only SELECT, WITH (CTE), INSERT, UPDATE, DELETE are allowed")
	}
}

// IsMutation returns true for INSERT, UPDATE, and DELETE statements.
func IsMutation(t SQLType) bool {
	return t == SQLTypeInsert || t == SQLTypeUpdate || t == SQLTypeDelete
}

// EnsureSelectLimit appends "LIMIT N" to a SELECT/WITH query if it has none.
// This is a safety net — the AI is instructed not to add LIMIT, but we enforce
// it here regardless to protect against accidental full-table scans.
func EnsureSelectLimit(sql string, limit int) string {
	if limit <= 0 {
		limit = defaultSelectLimit
	}
	if strings.Contains(strings.ToUpper(sql), "LIMIT") {
		return sql // already has a LIMIT clause
	}
	trimmed := strings.TrimRightFunc(sql, unicode.IsSpace)
	trimmed = strings.TrimSuffix(trimmed, ";")
	return fmt.Sprintf("%s\nLIMIT %d", trimmed, limit)
}

// EnsureReturning appends "RETURNING guid" to INSERT/UPDATE/DELETE if no
// RETURNING clause is present. This lets us report which records were affected
// back to the AI and user without an extra round-trip SELECT.
func EnsureReturning(sql string) string {
	if strings.Contains(strings.ToUpper(sql), "RETURNING") {
		return sql // AI already included a RETURNING clause
	}
	trimmed := strings.TrimRightFunc(sql, unicode.IsSpace)
	trimmed = strings.TrimSuffix(trimmed, ";")
	return trimmed + "\nRETURNING guid"
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────────────────────────

// stripSQLComments removes both -- line comments and /* */ block comments.
func stripSQLComments(sql string) string {
	s := reSQLBlockComment.ReplaceAllString(sql, " ")
	s = reSQLLineComment.ReplaceAllString(s, " ")
	return s
}

// classifyCTEStatement finds the effective DML type of a WITH ... statement.
//
// Strategy: scan the text after the last closing parenthesis (which ends the
// CTE definition block) and look for the first DML keyword. If none found,
// treat as SELECT (most common CTE pattern).
func classifyCTEStatement(upperSQL string) SQLType {
	lastParen := strings.LastIndex(upperSQL, ")")
	if lastParen < 0 {
		// Malformed CTE — treat as SELECT to avoid blocking it unnecessarily
		return SQLTypeSelect
	}
	tail := upperSQL[lastParen:]
	switch {
	case strings.Contains(tail, "INSERT"):
		return SQLTypeInsert
	case strings.Contains(tail, "UPDATE"):
		return SQLTypeUpdate
	case strings.Contains(tail, "DELETE"):
		return SQLTypeDelete
	default:
		return SQLTypeSelect
	}
}

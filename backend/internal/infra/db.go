package infra

import (
	"context"
	"database/sql"
)

// DB is a minimal interface implemented by *sql.DB and *sql.Tx. It allows
// repositories to depend on a small surface area and keeps transaction support
// flexible.
type DB interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

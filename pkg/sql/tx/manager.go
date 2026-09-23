// Package db provides transaction-aware access to a single SQL database.
//
// Manager lets multiple storage components share one *sql.DB while still
// participating in the same transaction: WithTransaction stores the active
// *sql.Tx in the context, and GetDB returns that transaction instead of the
// raw database. Any component built on the same Manager therefore joins
// the caller's transaction transparently.
//
// The transaction is carried by context.Context: it is scoped to the call
// chain (not to an HTTP request), and it is safe for concurrent use because
// each call chain carries its own context.
package tx

import (
	"context"
	"database/sql"
	"fmt"
)

type contextKeyTransaction struct{}

// Executor is the subset of *sql.DB and *sql.Tx used by storage code.
// Both types satisfy it, which is what allows Manager to switch
// transparently between plain execution and transactional execution.
type Executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Manager owns a *sql.DB and hands out transaction-aware Executors.
//
// All storage components whose writes must commit atomically should share
// the same Manager. A Manager wraps exactly one database; use a separate
// Manager per database.
//
// Manager is safe for concurrent use: the active transaction is carried
// by the caller's context, never by the manager itself.
type Manager struct {
	db *sql.DB
}

// NewManager returns a Manager wrapping db.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db}
}

// Executor returns the Executor for ctx: the transaction started by
// WithTx if ctx carries one, otherwise the underlying database.
//
// Storage code should resolve the Executor per call through Executor rather
// than caching it, so it automatically joins the caller's transaction.
func (m *Manager) Executor(ctx context.Context) Executor {
	if tx := getTxFromContext(ctx); tx != nil {
		return tx
	}
	return m.db
}

// getTxFromContext retrieves the transaction from context if present.
func getTxFromContext(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(contextKeyTransaction{}).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// WithTx runs fn inside a database transaction. The transaction
// is committed if fn returns nil, and rolled back if fn returns an error or
// panics (the panic is re-thrown after rollback).
//
// The transaction is ambient: any storage built on the same Manager that
// fn calls, directly or indirectly, joins this transaction via Executor without
// explicit wiring.
//
// Calls nest safely: if ctx already carries a transaction, fn simply runs
// inside the existing one and no new transaction is started. Note that in
// this case an error from fn aborts the whole outer transaction — nested
// calls do not create savepoints.
func (m *Manager) WithTx(ctx context.Context, fn func(ctx context.Context) error) error {
	if tx := getTxFromContext(ctx); tx != nil {
		return fn(ctx)
	}

	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := fn(context.WithValue(ctx, contextKeyTransaction{}, tx)); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %w", err)
	}
	return nil
}

// Close closes the underlying database.
func (m *Manager) Close() error {
	return m.db.Close()
}

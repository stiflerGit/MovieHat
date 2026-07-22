package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stiflerGit/moviehat/internal/auth/persistence"

	"github.com/google/uuid"
	sqlite "modernc.org/sqlite"
	sqlite3 "modernc.org/sqlite/lib"
)

type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Storage stores authentication data in SQLite.
type Storage struct {
	db       *sql.DB
	executor executor
}

var _ persistence.TransactionalStorage = (*Storage)(nil)

// New creates a SQLite authentication storage.
func New(db *sql.DB) *Storage {
	return &Storage{db: db, executor: db}
}

// WithTx executes authentication storage operations in a transaction.
func (s Storage) WithTx(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
	if fn == nil {
		return errors.New("fn is nil")
	}

	if _, ok := s.executor.(*sql.Tx); ok {
		// already inside a transaction
		return fn(ctx, s)
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("s.db.BeginTx: %w", err)
	}
	defer tx.Rollback()

	storage := &Storage{db: s.db, executor: tx}
	if err = fn(ctx, storage); err != nil {
		return fmt.Errorf("executing function in transaction: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("tx.Commit(): %w", err)
	}

	return nil
}

// InsertUser creates an auth user.
func (s Storage) InsertUser(ctx context.Context, in persistence.InsertUserArg) (persistence.InsertUserRet, error) {
	const query = `INSERT INTO auth_users(id, email, hashed_password, created_at) VALUES(?, ?, ?, ?)`

	uuid := uuid.NewString()
	now := time.Now()
	res, err := s.executor.ExecContext(ctx, query, uuid, in.Email, in.HashedPassword, now)
	if err != nil {
		if sqliteErr, ok := errors.AsType[*sqlite.Error](err); ok {
			if sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
				return persistence.InsertUserRet{}, fmt.Errorf("%w, %w", persistence.ErrAlreadyExists, err)
			}
		}
		return persistence.InsertUserRet{}, fmt.Errorf("r.db.ExecContext: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return persistence.InsertUserRet{}, fmt.Errorf("res.RowsAffected(): %w", err)
	}

	if rowsAffected == 0 {
		// this is bad
		return persistence.InsertUserRet{}, errors.New("rowsAffected=0")
	}

	return persistence.InsertUserRet{
		User: persistence.User{
			ID:             uuid,
			Email:          in.Email,
			HashedPassword: in.HashedPassword,
			CreatedAt:      now,
		},
	}, nil
}

// GetUser returns an auth user selected by id, email, or both.
func (s Storage) GetUser(ctx context.Context, in persistence.GetUserArg) (persistence.GetUserRet, error) {
	var whereStmts []string
	var whereArgs []any
	if in.Email != "" {
		whereArgs = append(whereArgs, in.Email)
		whereStmts = append(whereStmts, "email=?")
	}

	if in.ID != "" {
		whereArgs = append(whereArgs, in.ID)
		whereStmts = append(whereStmts, "id=?")
	}

	if len(whereArgs) == 0 {
		return persistence.GetUserRet{}, errors.New("empty input")
	}

	query := fmt.Sprintf(`SELECT id, email, hashed_password, created_at FROM auth_users WHERE %s`, strings.Join(whereStmts, " AND "))

	var user persistence.User
	err := s.executor.QueryRowContext(ctx, query, whereArgs...).Scan(&user.ID, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.GetUserRet{}, persistence.ErrNotFound
		}
		return persistence.GetUserRet{}, fmt.Errorf("r.db.QueryRowContext: %w", err)
	}

	return persistence.GetUserRet{User: user}, nil
}

// InsertSession creates an auth session.
func (s Storage) InsertSession(ctx context.Context, in persistence.InsertSessionArg) (persistence.InsertSessionRet, error) {
	const query = `INSERT INTO auth_sessions(id, token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?, ?);`

	session := persistence.Session{
		ID:        uuid.NewString(),
		Token:     in.Token,
		UserID:    in.UserID,
		CreatedAt: time.Now(),
		ExpiresAt: in.ExpiresAt,
	}

	_, err := s.executor.ExecContext(ctx, query, session.ID, session.Token, session.UserID, session.CreatedAt, session.ExpiresAt)
	if err != nil {
		if sqliteErr, ok := errors.AsType[*sqlite.Error](err); ok {
			if sqliteErr.Code() == sqlite3.SQLITE_CONSTRAINT_UNIQUE {
				return persistence.InsertSessionRet{}, fmt.Errorf("%w, %w", persistence.ErrAlreadyExists, err)
			}
		}

		return persistence.InsertSessionRet{}, fmt.Errorf("r.db.ExecContext: %w", err)
	}

	return persistence.InsertSessionRet{Session: session}, nil
}

// GetSession returns an auth session selected by id, token, or both.
func (s Storage) GetSession(ctx context.Context, in persistence.GetSessionArg) (persistence.GetSessionRet, error) {
	var whereStmts []string
	var whereArgs []any
	if in.ID != "" {
		whereArgs = append(whereArgs, in.ID)
		whereStmts = append(whereStmts, "id=?")
	}

	if in.Token != "" {
		whereArgs = append(whereArgs, in.Token)
		whereStmts = append(whereStmts, "token=?")
	}

	if len(whereArgs) == 0 {
		return persistence.GetSessionRet{}, errors.New("empty input")
	}

	var session persistence.Session
	var expiresAt, lastAccess sql.NullTime

	where := strings.Join(whereStmts, " AND ")
	query := fmt.Sprintf(`SELECT id, token, user_id, created_at, expires_at, last_access FROM auth_sessions WHERE %s`, where)
	err := s.executor.QueryRowContext(ctx, query, whereArgs...).
		Scan(&session.ID, &session.Token, &session.UserID, &session.CreatedAt, &expiresAt, &lastAccess)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.GetSessionRet{}, persistence.ErrNotFound
		}
		return persistence.GetSessionRet{}, fmt.Errorf("r.db.QueryRowContext: %w", err)
	}

	session.ExpiresAt = expiresAt.Time
	session.LastAccess = lastAccess.Time

	return persistence.GetSessionRet{Session: session}, nil
}

// UpdateSession updates mutable auth session fields.
func (s Storage) UpdateSession(ctx context.Context, in persistence.UpdateSessionArg) (persistence.UpdateSessionRet, error) {
	if in.ID == "" {
		return persistence.UpdateSessionRet{}, errors.New("empty input")
	}

	var setStmts []string
	var setArgs []any
	if in.ExpiresAt != nil && !in.ExpiresAt.IsZero() {
		setArgs = append(setArgs, in.ExpiresAt)
		setStmts = append(setStmts, "expires_at=?")
	}

	if in.LastAccess != nil && !in.LastAccess.IsZero() {
		setArgs = append(setArgs, in.LastAccess)
		setStmts = append(setStmts, "last_access=?")
	}

	if len(setArgs) == 0 {
		return persistence.UpdateSessionRet{}, errors.New("empty input")
	}

	var session persistence.Session
	var lastAccess sql.NullTime

	set := strings.Join(setStmts, ", ")
	query := fmt.Sprintf(`UPDATE auth_sessions SET %s WHERE id=? RETURNING id, token, user_id, created_at, expires_at, last_access`, set)
	args := append(setArgs, in.ID)

	err := s.executor.QueryRowContext(ctx, query, args...).
		Scan(&session.ID, &session.Token, &session.UserID, &session.CreatedAt, &session.ExpiresAt, &lastAccess)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.UpdateSessionRet{}, persistence.ErrNotFound
		}
		return persistence.UpdateSessionRet{}, fmt.Errorf("r.db.QueryRowContext: %w", err)
	}

	session.LastAccess = lastAccess.Time

	return persistence.UpdateSessionRet{Session: session}, nil
}

// DeleteUser deletes an auth user.
func (s Storage) DeleteUser(ctx context.Context, in persistence.DeleteUserArg) (persistence.DeleteUserRet, error) {
	if in.UserID == "" {
		return persistence.DeleteUserRet{}, errors.New("empty input")
	}

	const query = `DELETE FROM auth_users WHERE id=? RETURNING id, email, hashed_password, created_at`

	var user persistence.User
	err := s.executor.QueryRowContext(ctx, query, in.UserID).Scan(&user.ID, &user.Email, &user.HashedPassword, &user.CreatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.DeleteUserRet{}, persistence.ErrNotFound
		}
		return persistence.DeleteUserRet{}, fmt.Errorf("s.db.QueryRowContext: %w", err)
	}

	return persistence.DeleteUserRet{User: user}, nil
}

// InsertVerification creates a consumable verification token.
func (s Storage) InsertVerification(ctx context.Context, in persistence.InsertVerificationArg) (persistence.InsertVerificationRet, error) {
	const query = `
	INSERT INTO auth_verifications(token_hash, expires_at, max_uses, uses_count, author_id)
	VALUES(?, ?, ?, ?, ?)
	RETURNING token_hash, expires_at, max_uses, uses_count, author_id`

	var verification persistence.Verification
	var authorID sql.NullString
	err := s.executor.QueryRowContext(ctx, query, in.TokenHash, in.ExpiresAt, in.MaxUses, 0, in.AuthorID).
		Scan(&verification.TokenHash, &verification.ExpiresAt, &verification.MaxUses, &verification.UsesCount, &authorID)
	if err != nil {
		// a conflict of token hash is too rare so we don't check the error and we return
		return persistence.InsertVerificationRet{}, fmt.Errorf("s.db.QueryRowContext: %w", err)
	}

	verification.AuthorID = authorID.String

	return persistence.InsertVerificationRet{Verification: verification}, nil
}

// GetVerification returns a verification by token hash.
func (s Storage) GetVerification(ctx context.Context, in persistence.GetVerificationArg) (persistence.GetVerificationRet, error) {
	const query = `
	SELECT token_hash, expires_at, max_uses, uses_count, author_id
	FROM auth_verifications
	WHERE token_hash = ?`

	var verification persistence.Verification
	var authorID sql.NullString

	err := s.executor.QueryRowContext(ctx, query, in.TokenHash).
		Scan(&verification.TokenHash, &verification.ExpiresAt, &verification.MaxUses, &verification.UsesCount, &authorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.GetVerificationRet{}, persistence.ErrNotFound
		}
		return persistence.GetVerificationRet{}, fmt.Errorf("s.db.QueryRowContext: %w", err)
	}

	verification.AuthorID = authorID.String

	return persistence.GetVerificationRet{Verification: verification}, nil
}

// UpdateVerification updates mutable verification fields.
func (s Storage) UpdateVerification(ctx context.Context, in persistence.UpdateVerificationArg) (persistence.UpdateVerificationRet, error) {
	var setStmts []string
	var setArgs []any
	if in.ExpiresAt != nil && !in.ExpiresAt.IsZero() {
		setStmts = append(setStmts, "expires_at=?")
		setArgs = append(setArgs, in.ExpiresAt)
	}

	if len(setStmts) == 0 {
		return persistence.UpdateVerificationRet{}, errors.New("empty input")
	}

	var verification persistence.Verification
	var authorID sql.NullString

	set := strings.Join(setStmts, ", ")
	args := append(setArgs, in.TokenHash)
	query := fmt.Sprintf(`UPDATE auth_verifications SET %s WHERE token_hash=? RETURNING token_hash, expires_at, max_uses, uses_count, author_id`, set)
	err := s.executor.QueryRowContext(ctx, query, args...).
		Scan(&verification.TokenHash, &verification.ExpiresAt, &verification.MaxUses, &verification.UsesCount, &authorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.UpdateVerificationRet{}, persistence.ErrNotFound
		}
		return persistence.UpdateVerificationRet{}, fmt.Errorf("s.db.QueryRowContext: %w", err)
	}

	verification.AuthorID = authorID.String

	return persistence.UpdateVerificationRet{Verification: verification}, nil
}

// ConsumeVerification marks one verification use if it is still valid.
func (s Storage) ConsumeVerification(ctx context.Context, in persistence.ConsumeVerificationArg) (persistence.ConsumeVerificationRet, error) {
	if in.TokenHash == "" {
		return persistence.ConsumeVerificationRet{}, fmt.Errorf("empty token hash")
	}

	const query = `
	UPDATE auth_verifications
	SET uses_count=uses_count+1
	WHERE token_hash=? AND expires_at > ? AND uses_count < max_uses
	RETURNING token_hash, expires_at, max_uses, uses_count, author_id`

	var verification persistence.Verification
	var authorID sql.NullString

	err := s.executor.QueryRowContext(ctx, query, in.TokenHash, time.Now()).
		Scan(&verification.TokenHash, &verification.ExpiresAt, &verification.MaxUses, &verification.UsesCount, &authorID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// TODO: what if it is expired? or uses_count == max_uses
			return persistence.ConsumeVerificationRet{}, persistence.ErrNotFound
		}
		return persistence.ConsumeVerificationRet{}, fmt.Errorf("s.db.QueryRowContext: %w", err)
	}

	verification.AuthorID = authorID.String

	return persistence.ConsumeVerificationRet{Verification: verification}, nil
}

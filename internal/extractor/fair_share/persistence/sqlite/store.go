package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stiflerGit/moviehat/internal/extractor/fair_share/persistence"
)

type executor interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Storage stores fair-share scores in SQLite.
type Storage struct {
	db       *sql.DB
	executor executor
}

// New creates a SQLite fair-share storage.
func New(db *sql.DB) *Storage {
	return &Storage{
		db:       db,
		executor: db,
	}
}

// WithTx executes fair-share storage operations in a transaction.
func (r *Storage) WithTx(ctx context.Context, fn func(context.Context, persistence.Repository) error) error {
	tx, err := r.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("r.db.BeginTx(ctx): %w", err)
	}
	defer tx.Rollback()

	err = fn(ctx, &Storage{db: r.db, executor: tx})
	if err != nil {
		return fmt.Errorf("executing transaction function: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("tx.Commit(): %w", err)
	}

	return nil
}

// ListUsersScores lists fair-share scores for users.
func (r Storage) ListUsersScores(ctx context.Context, arg persistence.ListUsersScoresArg) (persistence.ListUsersScoresRet, error) {
	queryBuilder := strings.Builder{}
	_, err := queryBuilder.WriteString(`SELECT user_id, score FROM scores`)
	if err != nil {
		return persistence.ListUsersScoresRet{}, fmt.Errorf("queryBuilder.WriteString(SELECT): %w", err)
	}

	args := make([]any, 0, len(arg.UserIDs))
	if len(arg.UserIDs) > 0 {
		_, err = queryBuilder.WriteString(" WHERE user_id IN (")
		if err != nil {
			return persistence.ListUsersScoresRet{}, fmt.Errorf(`queryBuilder.WriteString("WHERE user_id IN ("): %w`, err)
		}

		for i, uID := range arg.UserIDs {
			_, err = queryBuilder.WriteRune('?')
			if err != nil {
				return persistence.ListUsersScoresRet{}, fmt.Errorf(`queryBuilder.WriteRune('?'): %w`, err)
			}

			r := ','
			if i == len(arg.UserIDs)-1 {
				r = ')'
			}

			_, err = queryBuilder.WriteRune(r)
			if err != nil {
				return persistence.ListUsersScoresRet{}, fmt.Errorf(`queryBuilder.WriteRune(r): %w`, err)
			}
			args = append(args, uID)
		}
	}

	query := queryBuilder.String()

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.ListUsersScoresRet{}, nil
		}
		return persistence.ListUsersScoresRet{}, fmt.Errorf("r.db.QueryContext: %w", err)
	}
	defer rows.Close()

	var userScores []persistence.UserScore
	for rows.Next() {
		var userScore persistence.UserScore
		err = rows.Scan(&userScore.UserID, &userScore.Score)
		if err != nil {
			return persistence.ListUsersScoresRet{}, fmt.Errorf("rows.Scan: %w", err)
		}
		userScores = append(userScores, userScore)
	}

	if rows.Err() != nil {
		return persistence.ListUsersScoresRet{}, fmt.Errorf("rows.Err(): %w", err)
	}

	return persistence.ListUsersScoresRet{UserScores: userScores}, nil
}

// IncreaseUserScore increments a user's fair-share score.
func (r *Storage) IncreaseUserScore(ctx context.Context, in persistence.IncreaseUserScoreArg) (persistence.IncreaseUserScoreRet, error) {
	query := `
	INSERT INTO scores(user_id, score, created_at, updated_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(user_id) DO
		UPDATE SET score=score+?,updated_at=?
	RETURNING user_id, score`

	var userScore persistence.UserScore

	now := time.Now()
	nowTXT := now.Format(time.RFC3339)
	err := r.executor.QueryRowContext(ctx, query, in.UserID, in.Inc, nowTXT, nowTXT, in.Inc, nowTXT).Scan(&userScore.UserID, &userScore.Score)
	if err != nil {
		return persistence.IncreaseUserScoreRet{}, fmt.Errorf("r.db.ExecContext: %w", err)
	}

	return persistence.IncreaseUserScoreRet{UserScore: userScore}, nil
}

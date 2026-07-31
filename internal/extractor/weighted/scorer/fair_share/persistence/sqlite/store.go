package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stiflerGit/moviehat/internal/extractor/weighted/scorer/fair_share/persistence"
	"github.com/stiflerGit/moviehat/pkg/sql/tx"
)

// Storage stores fair-share scores in SQLite.
type Storage struct {
	txManager *tx.Manager
}

// New creates a SQLite fair-share storage.
func New(txManager *tx.Manager) *Storage {
	return &Storage{txManager: txManager}
}

// WithTx executes fair-share storage operations in a transaction.
func (s *Storage) WithTx(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
	return s.txManager.WithTx(ctx, func(ctx context.Context) error {
		return fn(ctx, s)
	})
}

// ListUsersScores lists fair-share scores for users.
func (s Storage) ListUsersScores(ctx context.Context, arg persistence.ListUsersScoresArg) (persistence.ListUsersScoresRet, error) {
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

	rows, err := s.txManager.Executor(ctx).QueryContext(ctx, query, args...)
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
func (s *Storage) IncreaseUserScore(ctx context.Context, in persistence.IncreaseUserScoreArg) (persistence.IncreaseUserScoreRet, error) {
	query := `
	INSERT INTO scores(user_id, score, created_at, updated_at)
	VALUES (?, ?, ?, ?)
	ON CONFLICT(user_id) DO
		UPDATE SET score=score+?,updated_at=?
	RETURNING user_id, score`

	var userScore persistence.UserScore

	now := time.Now()
	err := s.txManager.Executor(ctx).QueryRowContext(ctx, query, in.UserID, in.Inc, now, now, in.Inc, now).Scan(&userScore.UserID, &userScore.Score)
	if err != nil {
		return persistence.IncreaseUserScoreRet{}, fmt.Errorf("r.db.QueryRowContext: %w", err)
	}

	return persistence.IncreaseUserScoreRet{UserScore: userScore}, nil
}

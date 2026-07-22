package persistence

import "context"

// Store combines fair-share repository operations with transaction support.
type Store interface {
	Repository
	Transactor
}

// Repository stores fair-share user scores.
type Repository interface {
	ListUsersScores(ctx context.Context, arg ListUsersScoresArg) (ListUsersScoresRet, error)
	IncreaseUserScore(ctx context.Context, in IncreaseUserScoreArg) (IncreaseUserScoreRet, error)
}

// Transactor executes fair-share storage operations in a transaction.
type Transactor interface {
	WithTx(ctx context.Context, fn func(context.Context, Repository) error) error
}

// UserScore represents a user's fair-share score.
type UserScore struct {
	UserID string
	Score  float64
}

// ListUsersScoresArg selects scores for users.
type ListUsersScoresArg struct {
	UserIDs []string
}

// ListUsersScoresRet contains listed user scores.
type ListUsersScoresRet struct {
	UserScores []UserScore
}

// IncreaseUserScoreArg contains a score increment for a user.
type IncreaseUserScoreArg struct {
	UserID string
	Inc    float64
}

// IncreaseUserScoreRet contains the updated user score.
type IncreaseUserScoreRet struct {
	UserScore UserScore
}

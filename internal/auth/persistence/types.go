package persistence

import (
	"context"
	"time"
)

//go:generate mockgen -package mocks -destination mocks/store.go . TransactionalStorage

// TransactionalStorage combines auth storage operations with transaction support.
type TransactionalStorage interface {
	Transactor
	Storage
}

// Storage groups all authentication persistence capabilities.
type Storage interface {
	UserStorage
	SessionStorage
	VerificationStorage
}

// Transactor executes storage operations in a transaction.
type Transactor interface {
	WithTx(ctx context.Context, fn func(context.Context, Storage) error) error
}

// UserStorage stores authentication users.
type UserStorage interface {
	InsertUser(ctx context.Context, in InsertUserArg) (InsertUserRet, error)
	GetUser(ctx context.Context, in GetUserArg) (GetUserRet, error)
	DeleteUser(ctx context.Context, in DeleteUserArg) (DeleteUserRet, error)
}

// SessionStorage stores authentication sessions.
type SessionStorage interface {
	InsertSession(ctx context.Context, in InsertSessionArg) (InsertSessionRet, error)
	GetSession(ctx context.Context, in GetSessionArg) (GetSessionRet, error)
	UpdateSession(ctx context.Context, in UpdateSessionArg) (UpdateSessionRet, error)
}

// VerificationStorage stores consumable verification tokens.
type VerificationStorage interface {
	InsertVerification(ctx context.Context, in InsertVerificationArg) (InsertVerificationRet, error)
	GetVerification(ctx context.Context, in GetVerificationArg) (GetVerificationRet, error)
	UpdateVerification(ctx context.Context, in UpdateVerificationArg) (UpdateVerificationRet, error)
	ConsumeVerification(ctx context.Context, in ConsumeVerificationArg) (ConsumeVerificationRet, error)
}

// InsertUserArg contains data for creating an auth user.
type InsertUserArg struct {
	Email          string
	HashedPassword string
}

// InsertUserRet contains the created auth user.
type InsertUserRet struct {
	User User
}

// User represents an authentication user.
type User struct {
	ID             string
	Email          string
	HashedPassword string
	CreatedAt      time.Time
}

// GetUserArg selects an auth user by id or email.
type GetUserArg struct {
	ID    string
	Email string
}

// GetUserRet contains the selected auth user.
type GetUserRet struct {
	User User
}

// Session represents a persisted auth session.
type Session struct {
	ID         string
	Token      string
	UserID     string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastAccess time.Time
}

// InsertSessionArg contains data for creating an auth session.
type InsertSessionArg struct {
	Token     string
	UserID    string
	ExpiresAt time.Time
}

// InsertSessionRet contains the created auth session.
type InsertSessionRet struct {
	Session Session
}

// GetSessionArg selects an auth session by id or token.
type GetSessionArg struct {
	ID    string
	Token string
}

// GetSessionRet contains the selected auth session.
type GetSessionRet struct {
	Session Session
}

// UpdateSessionArg contains mutable auth session fields.
type UpdateSessionArg struct {
	ID         string
	ExpiresAt  *time.Time
	LastAccess *time.Time
}

// UpdateSessionRet contains the updated auth session.
type UpdateSessionRet struct {
	Session Session
}

// DeleteUserArg selects an auth user to delete.
type DeleteUserArg struct {
	UserID string
}

// DeleteUserRet contains the deleted auth user.
type DeleteUserRet struct {
	User User
}

// InsertVerificationArg contains data for creating a verification token.
type InsertVerificationArg struct {
	TokenHash string
	ExpiresAt time.Time
	MaxUses   int
	AuthorID  string
}

// InsertVerificationRet contains the created verification.
type InsertVerificationRet struct {
	Verification Verification
}

// Verification represents a consumable verification token.
type Verification struct {
	TokenHash string
	ExpiresAt time.Time
	MaxUses   int
	UsesCount int
	AuthorID  string
}

// GetVerificationArg selects a verification by token hash.
type GetVerificationArg struct {
	TokenHash string
}

// GetVerificationRet contains the selected verification.
type GetVerificationRet struct {
	Verification Verification
}

// UpdateVerificationArg contains mutable verification fields.
type UpdateVerificationArg struct {
	TokenHash string
	ExpiresAt *time.Time
}

// UpdateVerificationRet contains the updated verification.
type UpdateVerificationRet struct {
	Verification Verification
}

// ConsumeVerificationArg selects a verification to consume.
type ConsumeVerificationArg struct {
	TokenHash string
}

// ConsumeVerificationRet contains the consumed verification.
type ConsumeVerificationRet struct {
	Verification Verification
}

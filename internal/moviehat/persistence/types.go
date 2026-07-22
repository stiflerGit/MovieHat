package persistence

import (
	"context"
	"time"
)

//go:generate mockgen -package mocks -destination mocks/store.go . TransactionalStorage

// TransactionalStorage combines MovieHat storage operations with transaction support.
type TransactionalStorage interface {
	Transactor
	Storage
}

// Storage groups all MovieHat persistence capabilities.
type Storage interface {
	UsersStorage
	SessionsStorage
	ParticipantsStorage
	MoviesStorage
}

// Transactor executes storage operations in a transaction.
type Transactor interface {
	WithTx(ctx context.Context, fn func(context.Context, Storage) error) error
}

// UsersStorage stores MovieHat users.
type UsersStorage interface {
	CreateUser(ctx context.Context, req CreateUserArg) (User, error)
	GetUser(ctx context.Context, req GetUserArg) (User, error)
	ListUsers(ctx context.Context, req ListUsersArg) ([]User, error)
	UpdateUser(ctx context.Context, req UpdateUserArg) (User, error)
	DeleteUser(ctx context.Context, req DeleteUserArg) (User, error)
}

// SessionsStorage stores movie selection sessions.
type SessionsStorage interface {
	CreateSession(ctx context.Context, req CreateSessionArg) (Session, error)
	ListSessions(ctx context.Context, req ListSessionsAg) (ListSessionsRet, error)
	GetSession(ctx context.Context, req GetSessionArg) (Session, error)
	UpdateSession(ctx context.Context, req UpdateSessionArg) (Session, error)
	DeleteSession(ctx context.Context, id string) (Session, error)
	CloseSession(ctx context.Context, id string) error
}

// ParticipantsStorage stores session participants.
type ParticipantsStorage interface {
	CreateParticipant(ctx context.Context, req CreateParticipantArg) (Participant, error)
	DeleteParticipant(ctx context.Context, req DeleteParticipantArg) error
	ListParticipants(ctx context.Context, req ListParticipantsArg) (ListParticipantsRet, error)
}

// MoviesStorage stores user movie lists.
type MoviesStorage interface {
	CreateMovie(ctx context.Context, req CreateMovieArg) (Movie, error)
	GetMovie(ctx context.Context, req GetMovieArg) (Movie, error)
	ListMovies(ctx context.Context, req ListMoviesArg) (ListMoviesRet, error)
	UpdateMovie(ctx context.Context, req UpdateMovieArg) (Movie, error)
	DeleteMovie(ctx context.Context, req DeleteMovieArg) (Movie, error)
}

// CreateUserArg contains data for creating a MovieHat user.
type CreateUserArg struct {
	UserID string
}

// User represents a MovieHat user profile.
type User struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

// GetUserArg selects a MovieHat user.
type GetUserArg struct {
	UserID         string
	IncludeDeleted bool
}

// ListUsersArg configures user listing.
type ListUsersArg struct {
	IncludeDeleted bool
}

// UpdateUserArg contains mutable user fields.
type UpdateUserArg struct {
	UserID string
	Name   string
}

// DeleteUserArg selects a MovieHat user to delete.
type DeleteUserArg struct {
	UserID string
}

// CreateSessionArg contains data for creating a movie selection session.
type CreateSessionArg struct{}

// Session represents a movie selection session.
type Session struct {
	ID             string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	ClosedAt       time.Time
	DeletedAt      time.Time
	WinnerID       string
	WatchedMovieID string
}

// ListSessionsAg configures session listing.
type ListSessionsAg struct{}

// ListSessionsRet contains listed movie selection sessions.
type ListSessionsRet struct {
	Sessions []Session
}

// GetSessionArg selects a movie selection session.
type GetSessionArg struct {
	ID             string
	IncludeDeleted bool
}

// UpdateSessionArg contains mutable session fields.
type UpdateSessionArg struct {
	ID             string
	WinnerID       *string
	WatchedMovieID *string
}

// CreateParticipantArg contains data for adding a session participant.
type CreateParticipantArg struct {
	SessionID string
	UserID    string
}

// Participant represents a user in a movie selection session.
type Participant struct {
	ID        string
	UserID    string
	SessionID string
}

// DeleteParticipantArg selects a session participant to remove.
type DeleteParticipantArg struct {
	SessionID string
	UserID    string
}

// ListParticipantsArg selects participants for a session.
type ListParticipantsArg struct {
	SessionID string
}

// ListParticipantsRet contains listed session participants.
type ListParticipantsRet struct {
	Participants []User
	HasMore      bool
}

// CreateMovieArg contains data for adding a movie.
type CreateMovieArg struct {
	UserID     string
	MovieTitle string
}

// GetMovieArg selects a movie.
type GetMovieArg struct {
	MovieID        string
	IncludeDeleted bool
}

// UpdateMovieArg contains mutable movie fields.
type UpdateMovieArg struct {
	ID     string
	Title  *string
	Status *MovieStatus
}

// Movie represents a movie in a user's list.
type Movie struct {
	ID        string
	UserID    string
	Title     string
	Status    MovieStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt time.Time
}

// ListMoviesArg selects movies for a user.
type ListMoviesArg struct {
	UserID         string
	IncludeDeleted bool
}

// ListMoviesRet contains listed movies.
type ListMoviesRet struct {
	Movies []Movie
}

// UpdateMovieListRequest describes a movie-list update request.
type UpdateMovieListRequest struct{}

// UpdateMovieListResponse describes a movie-list update response.
type UpdateMovieListResponse struct{}

// DeleteMovieArg selects a movie to delete.
type DeleteMovieArg struct {
	UserID  string
	MovieID string
}

// DeleteMovieResponse describes a deleted movie.
type DeleteMovieResponse struct{}

// MovieStatus describes whether a movie is pending or watched.
type MovieStatus string

const (
	// MovieStatusPending marks a movie that has not been watched.
	MovieStatusPending MovieStatus = "PENDING"
	// MovieStatusWatched marks a movie that has been watched.
	MovieStatusWatched MovieStatus = "WATCHED"
)

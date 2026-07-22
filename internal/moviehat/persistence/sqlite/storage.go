package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/stiflerGit/moviehat/internal/moviehat/persistence"

	"github.com/google/uuid"
)

type executor interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// Storage stores MovieHat data in SQLite.
type Storage struct {
	db       *sql.DB
	executor executor
}

var _ persistence.Storage = (*Storage)(nil)

// New creates a SQLite MovieHat storage.
func New(db *sql.DB) *Storage {
	return &Storage{db: db, executor: db}
}

// WithTx executes MovieHat storage operations in a transaction.
func (s *Storage) WithTx(ctx context.Context, fn func(context.Context, persistence.Storage) error) error {
	if fn == nil {
		return persistence.ErrInvalidArgument{Err: errors.New("fn is nil")}
	}

	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{})
	if err != nil {
		return fmt.Errorf("r.db.BeginTx(ctx): %w", err)
	}
	defer tx.Rollback()

	err = fn(ctx, &Storage{db: s.db, executor: tx})
	if err != nil {
		return fmt.Errorf("executing function: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("tx.Commit(): %w", err)
	}

	return nil
}

// CreateUser creates a MovieHat user.
func (s *Storage) CreateUser(ctx context.Context, in persistence.CreateUserArg) (persistence.User, error) {
	if in.UserID == "" {
		return persistence.User{}, persistence.ErrInvalidArgument{Err: errors.New("user_id is empty")}
	}

	now := time.Now()
	user := persistence.User{
		ID:        in.UserID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	query := `INSERT INTO users(id, name, created_at, updated_at) VALUES (?, ?, ?, ?)`
	_, err := s.executor.ExecContext(ctx, query, user.ID, user.Name, user.CreatedAt, user.UpdatedAt)
	if err != nil {
		return persistence.User{}, fmt.Errorf("r.executor.ExecContext: %w", err)
	}

	return user, nil
}

// GetUser returns a MovieHat user.
func (s *Storage) GetUser(ctx context.Context, in persistence.GetUserArg) (persistence.User, error) {
	if in.UserID == "" {
		return persistence.User{}, persistence.ErrInvalidArgument{Err: errors.New("user_id is empty")}
	}

	query := `SELECT id, name, created_at, updated_at, deleted_at FROM users WHERE id = ?`
	if !in.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}

	var user persistence.User
	var deletedAt sql.NullTime

	err := s.executor.QueryRowContext(ctx, query, in.UserID).Scan(&user.ID, &user.Name, &user.CreatedAt, &user.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.User{}, persistence.ErrNotFound
		}
		return persistence.User{}, err
	}

	user.DeletedAt = deletedAt.Time

	return user, nil
}

// TODO: introduce pagination
// ListUsers lists MovieHat users.
func (s *Storage) ListUsers(ctx context.Context, in persistence.ListUsersArg) ([]persistence.User, error) {
	query := `SELECT COUNT(id) FROM users`
	if !in.IncludeDeleted {
		query += " WHERE deleted_at IS NULL"
	}

	var count int
	if err := s.executor.QueryRowContext(ctx, query).Scan(&count); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("counting users: %w", err)
	}

	if count == 0 {
		return nil, nil
	}

	query = `SELECT id, name, created_at, updated_at, deleted_at FROM users`
	if !in.IncludeDeleted {
		query += " WHERE deleted_at IS NULL"
	}

	rows, err := s.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("querying all users")
	}
	defer rows.Close()

	users := make([]persistence.User, 0, count)
	for rows.Next() {
		var user persistence.User
		var deletedAt sql.NullTime
		err := rows.Scan(&user.ID, &user.Name, &user.CreatedAt, &user.UpdatedAt, &deletedAt)
		if err != nil {
			return nil, fmt.Errorf("scanning rows: %w", err)
		}

		user.DeletedAt = deletedAt.Time

		users = append(users, user)
	}

	if rows.Err() != nil {
		return users, fmt.Errorf("row.Err(): %w", err)
	}

	return users, nil
}

// UpdateUser updates a MovieHat user.
func (s *Storage) UpdateUser(ctx context.Context, in persistence.UpdateUserArg) (persistence.User, error) {
	if in.UserID == "" {
		return persistence.User{}, persistence.ErrInvalidArgument{Err: errors.New("user_id is empty")}
	}

	if in.Name == "" {
		return persistence.User{}, persistence.ErrInvalidArgument{Err: errors.New("name is empty")}
	}

	const query = `UPDATE users SET name=?, updated_at=? WHERE id=? RETURNING id, name, created_at, updated_at, deleted_at`

	user := persistence.User{
		ID:        in.UserID,
		Name:      in.Name,
		UpdatedAt: time.Now(),
	}

	var deletedAt sql.NullTime

	err := s.executor.QueryRowContext(ctx, query, user.Name, user.UpdatedAt, user.ID).
		Scan(&user.ID, &user.Name, &user.CreatedAt, &user.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.User{}, persistence.ErrNotFound
		}
		return persistence.User{}, fmt.Errorf("running query: %w", err)
	}

	user.DeletedAt = deletedAt.Time

	return user, nil
}

// DeleteUser soft-deletes a MovieHat user.
func (s *Storage) DeleteUser(ctx context.Context, in persistence.DeleteUserArg) (persistence.User, error) {
	if in.UserID == "" {
		return persistence.User{}, persistence.ErrInvalidArgument{Err: errors.New("user_id is empty")}
	}

	const query = `UPDATE users SET updated_at=?, deleted_at=? WHERE id=? RETURNING id, name, created_at, updated_at, deleted_at `

	now := time.Now()
	user := persistence.User{
		ID:        in.UserID,
		UpdatedAt: now,
		DeletedAt: now,
	}

	var deletedAt sql.NullTime

	err := s.executor.QueryRowContext(ctx, query, user.UpdatedAt, user.DeletedAt, user.ID).
		Scan(&user.ID, &user.Name, &user.CreatedAt, &user.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.User{}, persistence.ErrNotFound
		}
		return persistence.User{}, fmt.Errorf("scanning rows: %w", err)
	}

	user.DeletedAt = deletedAt.Time

	return user, nil
}

// CreateSession creates a movie selection session.
func (s *Storage) CreateSession(ctx context.Context, in persistence.CreateSessionArg) (persistence.Session, error) {
	now := time.Now()
	session := persistence.Session{
		ID:        uuid.NewString(),
		CreatedAt: now,
		UpdatedAt: now,
	}

	// not possible to create a session if there is another open session
	query := `INSERT INTO sessions(id, created_at, updated_at)
	SELECT ?, ?, ?
	WHERE NOT EXISTS (SELECT 1 FROM sessions WHERE closed_at IS NULL AND deleted_at IS NULL)`

	result, err := s.executor.ExecContext(ctx, query, session.ID, session.CreatedAt, session.UpdatedAt)
	if err != nil {
		return session, fmt.Errorf("inserting row in sessions table: %w", err)
	}

	count, err := result.RowsAffected()
	if err != nil {
		return session, fmt.Errorf("result.RowsAffected(): %w", err)
	}

	if count == 0 {
		return session, persistence.ErrSessionAlreadyExists
	}

	return session, nil
}

// ListSessions lists movie selection sessions.
func (s *Storage) ListSessions(ctx context.Context, in persistence.ListSessionsAg) (persistence.ListSessionsRet, error) {
	query := `SELECT COUNT(id) FROM sessions WHERE deleted_at IS NULL`

	var count int
	err := s.executor.QueryRowContext(ctx, query).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.ListSessionsRet{}, nil
		}
		return persistence.ListSessionsRet{}, fmt.Errorf("r.executor.QueryRowContext COUNT: %w", err)
	}

	query = `
	SELECT id, created_at, updated_at, closed_at, deleted_at, winner_id, watched_movie_id
	FROM sessions WHERE deleted_at IS NULL
	ORDER BY id
	`

	rows, err := s.executor.QueryContext(ctx, query)
	if err != nil {
		return persistence.ListSessionsRet{}, fmt.Errorf("r.executor.QueryContext: %w", err)
	}
	defer rows.Close()

	sessions := make([]persistence.Session, 0, count)
	for rows.Next() {
		var session persistence.Session

		var closedAt, deletedAt sql.NullTime
		var winnerID, watchedMovieID sql.NullString
		err = rows.Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt, &closedAt, &deletedAt, &winnerID, &watchedMovieID)
		if err != nil {
			return persistence.ListSessionsRet{}, fmt.Errorf("rows.Scan: %w", err)
		}

		session.ClosedAt = closedAt.Time
		session.DeletedAt = deletedAt.Time
		session.WinnerID = winnerID.String
		session.WatchedMovieID = watchedMovieID.String

		sessions = append(sessions, session)
	}

	if rows.Err() != nil {
		return persistence.ListSessionsRet{}, fmt.Errorf("rows.Err(): %w", err)
	}

	return persistence.ListSessionsRet{Sessions: sessions}, nil
}

// GetSession returns a movie selection session.
func (s *Storage) GetSession(ctx context.Context, in persistence.GetSessionArg) (persistence.Session, error) {
	if err := uuid.Validate(in.ID); err != nil {
		return persistence.Session{}, persistence.ErrInvalidArgument{Err: fmt.Errorf("id is not a valid uuid: %w", err)}
	}

	query := `SELECT id, created_at, updated_at, closed_at, deleted_at, winner_id, watched_movie_id FROM sessions WHERE id=?`
	if !in.IncludeDeleted {
		query += " AND deleted_at IS NULL"
	}

	session := persistence.Session{}
	var closedAt, deletedAt sql.NullTime
	var winnerID, watchedMovieID sql.NullString
	err := s.executor.QueryRowContext(ctx, query, in.ID).
		Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt, &closedAt, &deletedAt, &winnerID, &watchedMovieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session, persistence.ErrNotFound
		}
		return session, fmt.Errorf("row.Scan: %w", err)
	}

	session.ClosedAt = closedAt.Time
	session.DeletedAt = deletedAt.Time
	session.WinnerID = winnerID.String
	session.WatchedMovieID = watchedMovieID.String

	return session, nil
}

// UpdateSession updates mutable session fields.
func (s *Storage) UpdateSession(ctx context.Context, in persistence.UpdateSessionArg) (persistence.Session, error) {
	if err := uuid.Validate(in.ID); err != nil {
		return persistence.Session{}, persistence.ErrInvalidArgument{Err: fmt.Errorf("id is not a valid uuid: %w", err)}
	}

	hasValidWinnerID := in.WinnerID != nil && *in.WinnerID != "" && uuid.Validate(*in.WinnerID) == nil
	hasValidWatchedMovieID := in.WatchedMovieID != nil && *in.WatchedMovieID != "" && uuid.Validate(*in.WatchedMovieID) == nil
	if !hasValidWinnerID && !hasValidWatchedMovieID {
		return persistence.Session{}, persistence.ErrInvalidArgument{Err: errors.New("no valid update argument")}
	}

	now := time.Now()

	whereStatements := []string{"id=?"}
	whereArgs := []any{in.ID}
	setStatements := []string{"updated_at=?"}
	setArgs := []any{now}

	if hasValidWinnerID || hasValidWatchedMovieID {
		whereStatements = append(whereStatements, "closed_at IS NOT NULL")

		if hasValidWinnerID {
			setStatements = append(setStatements, "winner_id=?")
			setArgs = append(setArgs, *in.WinnerID)
		}

		if hasValidWatchedMovieID {
			setStatements = append(setStatements, "watched_movie_id=?")
			setArgs = append(setArgs, *in.WatchedMovieID)
		}
	}

	set := strings.Join(setStatements, ",")
	where := strings.Join(whereStatements, " AND ")

	query := fmt.Sprintf(`UPDATE sessions SET %s WHERE %s RETURNING id, created_at, updated_at, closed_at, deleted_at, winner_id, watched_movie_id`, set, where)
	args := append(setArgs, whereArgs...)

	var session persistence.Session
	var closedAt, deletedAt sql.NullTime
	var winnerID, watchedMovieID sql.NullString
	err := s.executor.QueryRowContext(ctx, query, args...).
		Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt, &closedAt, &deletedAt, &winnerID, &watchedMovieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session, persistence.ErrNotFound
		}
		return session, fmt.Errorf("row.Scan: %w", err)
	}

	session.ClosedAt = closedAt.Time
	session.DeletedAt = deletedAt.Time
	session.WinnerID = winnerID.String
	session.WatchedMovieID = watchedMovieID.String

	return session, nil
}

// DeleteSession soft-deletes a movie selection session.
func (s *Storage) DeleteSession(ctx context.Context, id string) (persistence.Session, error) {
	if err := uuid.Validate(id); err != nil {
		return persistence.Session{}, persistence.ErrInvalidArgument{Err: fmt.Errorf("id is not a valid uuid: %w", err)}
	}

	var session persistence.Session
	now := time.Now()

	// TODO: should we hard delete sessions?
	// It's not as dynamic as participants but I think they are quite dynamic as well
	query := `UPDATE sessions SET deleted_at=?, updated_at=? WHERE id=? RETURNING id, created_at, updated_at, closed_at, deleted_at, winner_id, watched_movie_id`

	var closedAt, deletedAt sql.NullTime
	var winnerID, watchedMovieID sql.NullString

	err := s.executor.QueryRowContext(ctx, query, now, now, id).
		Scan(&session.ID, &session.CreatedAt, &session.UpdatedAt, &closedAt, &deletedAt, &winnerID, &watchedMovieID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return session, persistence.ErrNotFound
		}
		return session, fmt.Errorf("row.Scan: %w", err)
	}

	session.ClosedAt = closedAt.Time
	session.DeletedAt = deletedAt.Time
	session.WinnerID = winnerID.String
	session.WatchedMovieID = watchedMovieID.String

	return session, nil
}

// CloseSession closes a movie selection session.
func (s *Storage) CloseSession(ctx context.Context, id string) error {
	if err := uuid.Validate(id); err != nil {
		return persistence.ErrInvalidArgument{Err: fmt.Errorf("id is not a valid uuid: %w", err)}
	}

	now := time.Now()

	const query = `UPDATE sessions SET closed_at=?, updated_at=? WHERE id=? AND closed_at IS NULL AND deleted_at IS NULL`

	res, err := s.executor.ExecContext(ctx, query, now, now, id)
	if err != nil {
		return fmt.Errorf("r.executor.ExecContext: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("res.RowsAffected: %w", err)
	}

	if count == 0 {
		_, err := s.GetSession(ctx, persistence.GetSessionArg{ID: id})
		if err != nil {
			return err
		}
		return persistence.ErrSessionClosed
	}

	return nil
}

// CreateParticipant adds a user to a movie selection session.
func (r *Storage) CreateParticipant(ctx context.Context, in persistence.CreateParticipantArg) (persistence.Participant, error) {
	if err := uuid.Validate(in.SessionID); err != nil {
		return persistence.Participant{}, persistence.ErrInvalidArgument{Err: fmt.Errorf("session_id is not a valid uuid: %w", err)}
	}

	if in.UserID == "" {
		return persistence.Participant{}, persistence.ErrInvalidArgument{Err: errors.New("user_id is empty")}
	}

	participant := persistence.Participant{
		ID:        uuid.NewString(),
		SessionID: in.SessionID,
		UserID:    in.UserID,
	}

	const query = `
	INSERT INTO participants(id, user_id, session_id, created_at)
	SELECT ?, ?, ?, ?
	WHERE EXISTS (
		SELECT 1 FROM users WHERE id = ? AND deleted_at IS NULL
	)
	AND EXISTS (
		SELECT 1
		FROM sessions
		WHERE id = ?
		AND closed_at IS NULL
		AND deleted_at IS NULL
	)`

	now := time.Now()
	res, err := r.executor.ExecContext(ctx, query, participant.ID, participant.UserID, participant.SessionID, now, participant.UserID, in.SessionID)
	if err != nil {
		return participant, fmt.Errorf("r.db.ExecContext: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return participant, fmt.Errorf("res.RowsAffected(): %w", err)
	}

	if count == 0 {
		s, err := r.GetSession(ctx, persistence.GetSessionArg{ID: in.SessionID})
		if err != nil {
			return participant, err
		}
		if !s.ClosedAt.IsZero() {
			return participant, persistence.ErrSessionClosed
		}

		_, err = r.GetUser(ctx, persistence.GetUserArg{UserID: in.UserID})
		if err != nil {
			return participant, err
		}

		return participant, persistence.ErrNotFound
	}

	return participant, nil
}

// DeleteParticipant removes a user from a movie selection session.
func (s *Storage) DeleteParticipant(ctx context.Context, in persistence.DeleteParticipantArg) error {
	if err := uuid.Validate(in.SessionID); err != nil {
		return persistence.ErrInvalidArgument{Err: fmt.Errorf("session_id is not a valid uuid: %w", err)}
	}

	if in.UserID == "" {
		return persistence.ErrInvalidArgument{Err: errors.New("user_id is empty")}
	}

	// participants can be deleted only if session is not closed
	// participants table is higly dynamic so hard delete is desired here
	const query = `
	DELETE FROM participants
	WHERE session_id = ?
	AND user_id = ?
	AND EXISTS (
		SELECT 1
		FROM sessions
		WHERE id = ?
		AND closed_at IS NULL
		AND deleted_at IS NULL
	)`

	res, err := s.executor.ExecContext(ctx, query, in.SessionID, in.UserID, in.SessionID)
	if err != nil {
		return fmt.Errorf("r.db.ExecContext: %w", err)
	}

	count, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("res.RowsAffected(): %w", err)
	}

	if count == 0 {
		s, err := s.GetSession(ctx, persistence.GetSessionArg{ID: in.SessionID})
		if err != nil {
			return fmt.Errorf("r.GetSession: %w", err)
		}
		if !s.ClosedAt.IsZero() {
			return persistence.ErrSessionClosed
		}
		return persistence.ErrNotFound
	}

	return nil
}

// ListParticipants lists users in a movie selection session.
func (s *Storage) ListParticipants(ctx context.Context, in persistence.ListParticipantsArg) (persistence.ListParticipantsRet, error) {
	if err := uuid.Validate(in.SessionID); err != nil {
		return persistence.ListParticipantsRet{}, persistence.ErrInvalidArgument{Err: fmt.Errorf("session_id is not a valid uuid: %w", err)}
	}

	query := `SELECT COUNT(id) FROM participants WHERE session_id=?`

	var count int
	err := s.executor.QueryRowContext(ctx, query, in.SessionID).Scan(&count)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.ListParticipantsRet{}, nil
		}
		return persistence.ListParticipantsRet{}, fmt.Errorf("r.db.QueryRowContext COUNT: %w", err)
	}

	if count == 0 {
		return persistence.ListParticipantsRet{}, nil
	}

	query = `SELECT u.id, u.name FROM participants p JOIN users u ON p.user_id=u.id WHERE session_id=?`
	rows, err := s.executor.QueryContext(ctx, query, in.SessionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.ListParticipantsRet{}, persistence.ErrNotFound
		}
		return persistence.ListParticipantsRet{}, fmt.Errorf("r.db.QueryRowContext Participants: %w", err)
	}
	defer rows.Close()

	participants := make([]persistence.User, 0, count)
	for rows.Next() {
		var u persistence.User
		err = rows.Scan(&u.ID, &u.Name)
		if err != nil {
			return persistence.ListParticipantsRet{}, fmt.Errorf("rows.Scan: %w", err)
		}
		participants = append(participants, u)
	}

	if rows.Err() != nil {
		return persistence.ListParticipantsRet{}, fmt.Errorf("rows.Err(): %w", err)
	}

	return persistence.ListParticipantsRet{Participants: participants}, nil
}

// CreateMovie adds a movie to a user's list.
func (s *Storage) CreateMovie(ctx context.Context, in persistence.CreateMovieArg) (persistence.Movie, error) {
	if in.UserID == "" {
		return persistence.Movie{}, persistence.ErrInvalidArgument{Err: errors.New("user_id title is empty")}
	}

	if in.MovieTitle == "" {
		return persistence.Movie{}, persistence.ErrInvalidArgument{Err: errors.New("movie title is empty")}
	}

	now := time.Now()

	const query = `
	INSERT INTO movies(id, owner_id, title, status, created_at, updated_at)
	SELECT ?, ?, ?, ?, ?, ?
	WHERE EXISTS (
		SELECT 1 FROM users WHERE id = ? AND deleted_at IS NULL
	)`

	movie := persistence.Movie{
		ID:        uuid.NewString(),
		UserID:    in.UserID,
		Title:     in.MovieTitle,
		Status:    persistence.MovieStatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	}

	res, err := s.executor.ExecContext(ctx, query, movie.ID, movie.UserID, movie.Title, string(movie.Status), movie.CreatedAt, movie.UpdatedAt, movie.UserID)
	if err != nil {
		return movie, fmt.Errorf("r.db.ExecContext: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return persistence.Movie{}, fmt.Errorf("res.RowsAffected(): %w", err)
	}

	if rowsAffected == 0 {
		return persistence.Movie{}, fmt.Errorf("user: %w", persistence.ErrNotFound)
	}

	return movie, nil
}

// GetMovie returns a movie.
func (s *Storage) GetMovie(ctx context.Context, in persistence.GetMovieArg) (persistence.Movie, error) {
	if in.MovieID == "" {
		return persistence.Movie{}, persistence.ErrInvalidArgument{Err: errors.New("movie_id title is empty")}
	}

	whereStmts := []string{
		"id=?",
	}
	args := []any{
		in.MovieID,
	}

	if !in.IncludeDeleted {
		whereStmts = append(whereStmts, "deleted_at IS NULL")
	}

	where := strings.Join(whereStmts, " AND ")

	query := fmt.Sprintf(`
	SELECT id, owner_id, title, status, created_at, updated_at, deleted_at
	FROM movies
	WHERE %s`, where)

	var movie persistence.Movie
	var deletedAt sql.NullTime

	err := s.executor.QueryRowContext(ctx, query, args...).
		Scan(&movie.ID, &movie.UserID, &movie.Title, &movie.Status, &movie.CreatedAt, &movie.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.Movie{}, persistence.ErrNotFound
		}
		return persistence.Movie{}, fmt.Errorf("s.executor.QueryRowContext: %w", err)
	}

	movie.DeletedAt = deletedAt.Time
	return movie, nil
}

// ListMovies lists movies for a user.
func (s *Storage) ListMovies(ctx context.Context, in persistence.ListMoviesArg) (persistence.ListMoviesRet, error) {
	whereStmts := []string{
		"owner_id=?",
	}
	args := []any{
		in.UserID,
	}

	if !in.IncludeDeleted {
		whereStmts = append(whereStmts, "deleted_at IS NULL")
	}

	where := strings.Join(whereStmts, " AND ")
	query := fmt.Sprintf(`SELECT COUNT(id) FROM movies WHERE %s`, where)

	var count int
	if err := s.executor.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return persistence.ListMoviesRet{}, fmt.Errorf("r.db.QueryRowContext COUNT: %w", err)
	}

	if count == 0 {
		return persistence.ListMoviesRet{}, persistence.ErrNotFound
	}

	query = fmt.Sprintf(`
		SELECT id, owner_id, title, status, created_at, updated_at, deleted_at
		FROM movies
		WHERE %s`, where)

	rows, err := s.executor.QueryContext(ctx, query, args...)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.ListMoviesRet{}, persistence.ErrNotFound
		}
		return persistence.ListMoviesRet{}, fmt.Errorf("r.db.QueryContext: %w", err)
	}
	defer rows.Close()

	movies := make([]persistence.Movie, 0, count)
	for rows.Next() {
		var movie persistence.Movie

		var deletedAt sql.NullTime
		err = rows.Scan(&movie.ID, &movie.UserID, &movie.Title, &movie.Status, &movie.CreatedAt, &movie.UpdatedAt, &deletedAt)
		if err != nil {
			return persistence.ListMoviesRet{}, fmt.Errorf("r.db.QueryRowContext: %w", err)
		}

		movie.DeletedAt = deletedAt.Time
		movies = append(movies, movie)
	}

	if rows.Err() != nil {
		return persistence.ListMoviesRet{}, fmt.Errorf("rows.Err(): %w", err)
	}

	return persistence.ListMoviesRet{Movies: movies}, nil
}

// UpdateMovie updates mutable movie fields.
func (s *Storage) UpdateMovie(ctx context.Context, in persistence.UpdateMovieArg) (persistence.Movie, error) {
	now := time.Now()
	setStmts := []string{
		"updated_at=?",
	}
	args := []any{
		now,
	}

	if in.Title != nil && *in.Title != "" {
		setStmts = append(setStmts, "title=?")
		args = append(args, *in.Title)
	}

	if in.Status != nil && *in.Status != "" {
		setStmts = append(setStmts, "status=?")
		args = append(args, string(*in.Status))
	}

	set := strings.Join(setStmts, ",")
	query := fmt.Sprintf("UPDATE movies SET %s WHERE id=? RETURNING id, owner_id, title, status, created_at, updated_at, deleted_at", set)
	args = append(args, in.ID)

	var movie persistence.Movie
	var deletedAt sql.NullTime

	err := s.executor.QueryRowContext(ctx, query, args...).Scan(&movie.ID, &movie.UserID, &movie.Title, &movie.Status, &movie.CreatedAt, &movie.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.Movie{}, persistence.ErrNotFound
		}
		return movie, fmt.Errorf("r.db.QueryRowContext: %w", err)
	}

	movie.DeletedAt = deletedAt.Time

	return movie, nil
}

// DeleteMovie soft-deletes a movie from a user's list.
func (s *Storage) DeleteMovie(ctx context.Context, in persistence.DeleteMovieArg) (persistence.Movie, error) {
	if in.MovieID == "" {
		return persistence.Movie{}, persistence.ErrInvalidArgument{Err: errors.New("movie title is empty")}
	}

	now := time.Now()

	const query = `
	UPDATE movies
	SET deleted_at=?, updated_at=?
	WHERE owner_id=? AND id=? AND deleted_at IS NULL
	RETURNING id, owner_id, title, status, created_at, updated_at, deleted_at`

	var movie persistence.Movie
	var deletedAt sql.NullTime
	err := s.executor.QueryRowContext(ctx, query, now, now, in.UserID, in.MovieID).
		Scan(&movie.ID, &movie.UserID, &movie.Title, &movie.Status, &movie.CreatedAt, &movie.UpdatedAt, &deletedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return persistence.Movie{}, persistence.ErrNotFound
		}
		return persistence.Movie{}, fmt.Errorf("r.db.QueryRowContext: %w", err)
	}

	movie.DeletedAt = deletedAt.Time

	return movie, nil
}

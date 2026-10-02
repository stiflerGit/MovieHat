package list

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/persistence"
)

// Handler provides MovieHat user, movie, and session operations.
type Handler struct {
	repository persistence.UserMovieListsStorage
	logger     *slog.Logger
}

// New creates a MovieHat handler.
func New(
	repository persistence.UserMovieListsStorage,
	options ...Option,
) *Handler {
	h := &Handler{
		repository: repository,
		logger:     slog.Default().With("component", "list"),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(h)
	}

	return h
}

type AddUserMovieRequest struct {
	UserID  string
	MovieID string
	Note    string
}

// AddUserMovie adds a movie to the current user's list.
func (h *Handler) AddUserMovie(ctx context.Context, req AddUserMovieRequest) (*pb.AddUserMovieResponse, error) {
	// TODO: verify that movie with that ID exists? Or should an orchestrator/interactor/whatever on a upper level do that?
	// 	movie, err := h.movieGetter.GetByID(ctx, req.GetMovieId())
	// 	if err != nil {
	// 		return nil, repoErrorToAPIError(err)
	// 	}
	//
	// 	h.logger.InfoContext(ctx, "movie found", "movie", movie.String())

	movieDB, err := h.repository.AddMovie(ctx, persistence.AddMovieArg{UserID: req.UserID, MovieID: req.MovieID, Note: req.Note})
	if err != nil {
		return nil, fmt.Errorf("h.repository.AddMovie: %w", err)
	}

	h.logger.InfoContext(ctx, "movie added", "user_id", req.UserID, "movie_id", req.MovieID)
	return &pb.AddUserMovieResponse{Movie: &pb.Movie{Id: movieDB.ID}}, nil
}

// ListUserMovies lists movies for a user.
func (h *Handler) ListUserMovies(ctx context.Context, req *pb.ListUserMoviesRequest) (*pb.ListUserMoviesResponse, error) {
	listMoviesRet, err := h.repository.GetMovieList(ctx, persistence.GetMovieListArg{UserID: req.UserId})
	if err != nil {
		return nil, fmt.Errorf("h.repository.AddMovie: %w", err)
	}

	return &pb.ListUserMoviesResponse{Movies: repoMoviesToPBListUserMoviesResponseMovieStatus(listMoviesRet.Movies)}, nil
}

type DeleteUserMovieRequest struct {
	UserID  string
	MovieID string
}

// DeleteUserMovie deletes a movie from the current user's list.
func (h *Handler) DeleteUserMovie(ctx context.Context, req DeleteUserMovieRequest) (*pb.DeleteUserMovieResponse, error) {
	_, err := h.repository.DeleteMovie(ctx, persistence.DeleteMovieArg{UserID: req.UserID, MovieID: req.MovieID})
	if err != nil {
		return nil, fmt.Errorf("h.repository.DeleteMovie: %w", err)
	}

	h.logger.InfoContext(ctx, "movie deleted", "user_id", req.UserID, "movie_title", req.MovieID)
	return &pb.DeleteUserMovieResponse{}, nil
}

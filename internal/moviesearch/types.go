package moviesearch

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound is returned when a movie id cannot be resolved.
var (
	ErrNotFound = errors.New("not found")
)

// MoviesSearcher performs paginated movie searches.
//
//go:generate mockgen -package mocks -destination mocks/movie_searcher.go  . MoviesSearcher

type MoviesSearcher interface {
	SearchMovies(ctx context.Context, arg SearchMoviesArg) (SearchMoviesRet, error)
}

// MovieDetailsGetter resolves a movie id to its details.
//
//go:generate mockgen -package mocks -destination mocks/movie_details_getter.go  . MovieDetailsGetter

type MovieDetailsGetter interface {
	GetDetails(ctx context.Context, arg GetDetailsArg) (GetDetailsRet, error)
}

// SearchMoviesArg selects a search window: the first Limit items starting at
// the 0-indexed Offset for Query.
type SearchMoviesArg struct {
	Query  string
	Offset int
	Limit  int
}

// SearchMoviesRet is one page of search results. HasMore reports whether more
// results exist past this page.
type SearchMoviesRet struct {
	Results []SearchMoviesRetResult
	HasMore bool
}

// SearchMoviesRetResult is a single search hit.
type SearchMoviesRetResult struct {
	ID          string
	Title       string
	ReleaseDate time.Time
	PosterPath  string
}

// GetDetailsArg selects a movie by provider id.
type GetDetailsArg struct {
	ID string
}

// GetDetailsRet carries the resolved movie details.
type GetDetailsRet struct {
	Result SearchMoviesRetResult
}

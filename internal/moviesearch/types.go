package moviesearch

import (
	"context"
	"errors"
	"time"
)

var (
	NotFoundErr = errors.New("not found")
)

//go:generate mockgen -package mocks -destination mocks/movie_searcher.go  . MoviesSearcher

type MoviesSearcher interface {
	SearchMovies(ctx context.Context, arg SearchMoviesArg) (SearchMoviesRet, error)
}

//
//go:generate mockgen -package mocks -destination mocks/movie_details_getter.go  . MovieDetailsGetter
type MovieDetailsGetter interface {
	GetDetails(ctx context.Context, arg GetDetailsArg) (GetDetailsRet, error)
}

type SearchMoviesArg struct {
	Query  string
	Offset int
	Limit  int
}

type SearchMoviesRet struct {
	Results []SearchMoviesRetResult
	HasMore bool
}

type SearchMoviesRetResult struct {
	ID          string
	Title       string
	ReleaseDate time.Time
	PosterPath  string
}

type GetDetailsArg struct {
	ID string
}

type GetDetailsRet struct {
	Result SearchMoviesRetResult
}

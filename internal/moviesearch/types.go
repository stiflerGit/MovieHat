package moviesearch

import (
	"context"
	"errors"
	"time"
)

var (
	NotFoundErr = errors.New("not found")
)

type Provider interface {
	MovieSearcher
	MovieDetailsGetter
}

type MovieSearcher interface {
	Search(ctx context.Context, arg SearchArg) (SearchRet, error)
}

type MovieDetailsGetter interface {
	GetDetails(ctx context.Context, arg GetDetailsArg) (GetDetailsRet, error)
}

type SearchArg struct {
	Query   string
	Page    int
	PerPage int
}

type SearchRet struct {
	Page         int
	PerPage      int
	TotalPages   int
	TotalResults int
	Results      []SearchRetResult
}

type SearchRetResult struct {
	ID          string
	Title       string
	ReleaseDate time.Time
	PosterPath  string
}

type GetDetailsArg struct {
	ID string
}

type GetDetailsRet struct {
	Result SearchRetResult
}

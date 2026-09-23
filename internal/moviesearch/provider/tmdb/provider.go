// Package tmdb implements the moviesearch domain over the TMDB HTTP API.
package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"
	"github.com/stiflerGit/moviehat/internal/moviesearch"
	"github.com/stiflerGit/moviehat/pkg/pagination"
)

const (
	// tmdbPageSize is the number of items TMDB returns per page.
	tmdbPageSize = 200 // TODO: verify this info
)

// ClientInterface is the subset of the generated TMDB client the provider needs.
//
//go:generate mockgen -destination mocks/client.go -package mocks . ClientInterface
type ClientInterface interface {
	SearchMovie(ctx context.Context, params *tmdb.SearchMovieParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
	MovieDetails(ctx context.Context, movieId int32, params *tmdb.MovieDetailsParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
}

// Provider is a moviesearch provider backed by TMDB.
type Provider struct {
	tmdbClient ClientInterface
}

// New returns a Provider using the given TMDB client.
func New(client ClientInterface) *Provider {
	return &Provider{
		tmdbClient: client,
	}
}

// SearchMovies returns the requested offset window of results for arg.Query.
func (p *Provider) SearchMovies(ctx context.Context, arg moviesearch.SearchMoviesArg) (moviesearch.SearchMoviesRet, error) {
	// TODO: skip mapping each time since there is a change that the adapter drop some of
	// 	items of the page. Map once adapter.Fetch return
	paginationAdapter := pagination.NewAdapter(tmdbPageSize, func(ctx context.Context, page int) ([]moviesearch.SearchMoviesRetResult, error) {
		httpResp, err := p.tmdbClient.SearchMovie(ctx,
			&tmdb.SearchMovieParams{
				Query:        arg.Query,
				Page:         new(int32(page)),
				IncludeAdult: new(false),
			},
		)
		if err != nil {
			return nil, fmt.Errorf("tmdbClient.SearchMulti: %w", err)
		}

		searchMovieResponse, err := tmdb.ParseSearchMovieResponse(httpResp)
		if err != nil {
			return nil, fmt.Errorf("tmdb.ParseSearchMultiResponse: %w", err)
		}

		if err := validateSearchMovieResponse(searchMovieResponse); err != nil {
			return nil, err
		}

		v := mapTMDBSearchMovieResponseToDomain(searchMovieResponse)
		return v.Results, nil
	})

	items, hasMore, err := paginationAdapter.Fetch(ctx, arg.Offset, arg.Limit)
	if err != nil {
		return moviesearch.SearchMoviesRet{}, fmt.Errorf("paginationAdapter.Fetch: %w", err)
	}

	return moviesearch.SearchMoviesRet{Results: items, HasMore: hasMore}, nil
}

// GetDetails resolves a movie id to its details, returning moviesearch.ErrNotFound
// when TMDB reports the movie unknown.
func (p *Provider) GetDetails(ctx context.Context, arg moviesearch.GetDetailsArg) (moviesearch.GetDetailsRet, error) {
	idAsInt, err := strconv.Atoi(arg.ID)
	if err != nil {
		return moviesearch.GetDetailsRet{}, fmt.Errorf("id is not an int")
	}

	ret, err := p.getMovieDetail(ctx, int32(idAsInt))
	if err != nil {
		return moviesearch.GetDetailsRet{}, fmt.Errorf("getMovieDetail: %w", err)
	}

	return *ret, nil
}

func (p *Provider) getMovieDetail(ctx context.Context, id int32) (*moviesearch.GetDetailsRet, error) {
	httpResp, err := p.tmdbClient.MovieDetails(ctx, id, &tmdb.MovieDetailsParams{})
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode == http.StatusNotFound {
		return nil, moviesearch.ErrNotFound
	}

	movieDetailsResponse, err := tmdb.ParseMovieDetailsResponse(httpResp)
	if err != nil {
		return nil, fmt.Errorf("tmdb.ParseMovieDetailsResponse: %w", err)
	}

	if err = validateMovieDetailsResponse(movieDetailsResponse); err != nil {
		return nil, fmt.Errorf("validateMovieDetailsResponse: %w", err)
	}

	resp, err := mapTMDBMovieDetailsResponseToDomain(movieDetailsResponse)
	if err != nil {
		return nil, fmt.Errorf("mapTMDBMovieDetailsResponseToDomain`: %w", err)
	}

	return &resp, nil
}

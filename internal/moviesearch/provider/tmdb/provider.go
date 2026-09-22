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
	tmdbPageSize = 200 // TODO: verify this info
)

//go:generate mockgen -destination mocks/client.go -package mocks . ClientInterface

type ClientInterface interface {
	SearchMovie(ctx context.Context, params *tmdb.SearchMovieParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
	MovieDetails(ctx context.Context, movieId int32, params *tmdb.MovieDetailsParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
}

type Provider struct {
	tmdbClient ClientInterface
}

func New(client ClientInterface) *Provider {
	return &Provider{
		tmdbClient: client,
	}
}

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
		return nil, moviesearch.NotFoundErr
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

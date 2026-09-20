package tmdb

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"
	"github.com/stiflerGit/moviehat/internal/moviesearch"
)

type ClientInterface interface {
	SearchMulti(ctx context.Context, params *tmdb.SearchMultiParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
	MovieDetails(ctx context.Context, movieId int32, params *tmdb.MovieDetailsParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
	TvSeriesDetails(ctx context.Context, seriesId int32, params *tmdb.TvSeriesDetailsParams, reqEditors ...tmdb.RequestEditorFn) (*http.Response, error)
}

type Provider struct {
	tmdbClient ClientInterface
}

func New(client ClientInterface) *Provider {
	return &Provider{
		tmdbClient: client,
	}
}

func (p *Provider) Search(ctx context.Context, arg moviesearch.SearchArg) (moviesearch.SearchRet, error) {
	// TODO: handle per page
	httpResp, err := p.tmdbClient.SearchMulti(ctx,
		&tmdb.SearchMultiParams{
			Query: arg.Query,
			Page: func() *int32 {
				if arg.Page != 0 {
					v := int32(arg.Page)
					return &v
				}
				return nil
			}(),
			IncludeAdult: new(false),
		},
	)
	if err != nil {
		return moviesearch.SearchRet{}, fmt.Errorf("tmdbClient.SearchMulti: %w", err)
	}

	searchMultiResponse, err := tmdb.ParseSearchMultiResponse(httpResp)
	if err != nil {
		return moviesearch.SearchRet{}, fmt.Errorf("tmdb.ParseSearchMultiResponse: %w", err)
	}

	if err := validateSearchMultiResponse(searchMultiResponse); err != nil {
		return moviesearch.SearchRet{}, err
	}

	return mapTMDBSearchMultiResponseToDomain(searchMultiResponse), nil
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

	if ret == nil {
		// movie not found. Try with tv series
		ret, err = p.getTVSeriesDetail(ctx, int32(idAsInt))
		if err != nil {
			return moviesearch.GetDetailsRet{}, fmt.Errorf("getMovieDetail: %w", err)
		}
		// tv and movie not found. Time to return an error
		if ret == nil {
			return moviesearch.GetDetailsRet{}, moviesearch.NotFoundErr
		}
	}

	return *ret, nil
}

func (p *Provider) getMovieDetail(ctx context.Context, id int32) (*moviesearch.GetDetailsRet, error) {
	httpResp, err := p.tmdbClient.MovieDetails(ctx, id, &tmdb.MovieDetailsParams{})
	if err != nil {
		return nil, err
	}

	if httpResp.StatusCode == http.StatusNotFound {
		return nil, nil
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

func (p *Provider) getTVSeriesDetail(ctx context.Context, id int32) (*moviesearch.GetDetailsRet, error) {
	// TODO: can it happen that a series have the same id of a movie?
	httpResp, err := p.tmdbClient.TvSeriesDetails(ctx, id, &tmdb.TvSeriesDetailsParams{})
	if err != nil {
		return nil, fmt.Errorf("p.tmdbClient.TvSeriesDetails: %w", err)
	}

	if httpResp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	tvSeriesDetailsResponse, err := tmdb.ParseTvSeriesDetailsResponse(httpResp)
	if err != nil {
		return nil, fmt.Errorf("tmdb.ParseMovieDetailsResponse: %w", err)
	}

	if err = validateTVSeriesDetailsResponse(tvSeriesDetailsResponse); err != nil {
		return nil, fmt.Errorf("validateTVSeriesDetailsResponse`: %w", err)
	}

	resp, err := mapTMDBTvSeriesDetailsResponseToDomain(tvSeriesDetailsResponse)
	if err != nil {
		return nil, fmt.Errorf("mapTMDBTvSeriesDetailsResponseToDomain`: %w", err)
	}

	return &resp, nil
}

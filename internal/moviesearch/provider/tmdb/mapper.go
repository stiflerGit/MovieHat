package tmdb

import (
	"fmt"
	"strconv"
	"time"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"
	"github.com/stiflerGit/moviehat/internal/moviesearch"
)

func mapTMDBSearchMovieResponseToDomain(r *tmdb.SearchMovieResponse) moviesearch.SearchMoviesRet {
	searchRet := moviesearch.SearchMoviesRet{}

	searchRet.Results = make([]moviesearch.SearchMoviesRetResult, 0, len(*r.JSON200.Results))
	for _, r := range *r.JSON200.Results {
		searchRet.Results = append(searchRet.Results,
			moviesearch.SearchMoviesRetResult{
				ID: func() string {
					if r.Id == nil {
						return ""
					}
					return strconv.Itoa(*r.Id)
				}(),
				Title: func() string {
					if r.Title != nil && *r.Title != "" {
						return *r.Title
					}
					return ""
				}(),
				ReleaseDate: func() time.Time {
					if r.ReleaseDate == nil {
						return time.Time{}
					}

					releaseDate, err := time.Parse(time.DateOnly, *r.ReleaseDate)
					if err != nil {
						return time.Time{}
					}
					return releaseDate
				}(),
				PosterPath: func() string {
					if r.PosterPath == nil {
						return ""
					}
					return *r.PosterPath
				}(),
			},
		)
	}

	return searchRet
}

func mapTMDBMovieDetailsResponseToDomain(r *tmdb.MovieDetailsResponse) (moviesearch.GetDetailsRet, error) {
	var releaseDate time.Time
	var err error

	if r.JSON200.ReleaseDate != nil {
		releaseDate, err = time.Parse(time.DateOnly, *r.JSON200.ReleaseDate)
		if err != nil {
			return moviesearch.GetDetailsRet{}, fmt.Errorf("parsing release date: %w", err)
		}
	}

	var posterPath string
	if r.JSON200.PosterPath != nil {
		posterPath = *r.JSON200.PosterPath
	}

	return moviesearch.GetDetailsRet{
		Result: moviesearch.SearchMoviesRetResult{
			ID:          strconv.Itoa(*r.JSON200.Id),
			Title:       *r.JSON200.Title,
			ReleaseDate: releaseDate,
			PosterPath:  posterPath,
		},
	}, nil
}

func mapTMDBTvSeriesDetailsResponseToDomain(r *tmdb.TvSeriesDetailsResponse) (moviesearch.GetDetailsRet, error) {
	var releaseDate time.Time
	var err error

	if r.JSON200.FirstAirDate != nil {
		releaseDate, err = time.Parse(time.DateOnly, *r.JSON200.FirstAirDate)
		if err != nil {
			return moviesearch.GetDetailsRet{}, fmt.Errorf("parsing release date: %w", err)
		}
	}

	var posterPath string
	if r.JSON200.PosterPath != nil {
		posterPath = *r.JSON200.PosterPath
	}

	return moviesearch.GetDetailsRet{
		Result: moviesearch.SearchMoviesRetResult{
			ID:          strconv.Itoa(*r.JSON200.Id),
			Title:       *r.JSON200.Name,
			ReleaseDate: releaseDate,
			PosterPath:  posterPath,
		},
	}, nil
}

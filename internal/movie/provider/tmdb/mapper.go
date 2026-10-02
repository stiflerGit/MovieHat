package tmdb

import (
	"fmt"
	"strconv"
	"time"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"
	"github.com/stiflerGit/moviehat/internal/movie/provider"
)

func mapTMDBSearchMovieResponseToDomain(r *tmdb.SearchMovieResponse) provider.SearchMoviesRet {
	var searchRet provider.SearchMoviesRet

	searchRet.Results = make([]provider.SearchMoviesRetResult, 0, len(*r.JSON200.Results))
	for _, r := range *r.JSON200.Results {
		searchRet.Results = append(searchRet.Results,
			provider.SearchMoviesRetResult{
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

// mapTMDBMovieDetailsResponseToDomain assumes validateMovieDetailsResponse passed.
func mapTMDBMovieDetailsResponseToDomain(r *tmdb.MovieDetailsResponse) (provider.GetDetailsRet, error) {
	var releaseDate time.Time
	var err error

	if r.JSON200.ReleaseDate != nil {
		releaseDate, err = time.Parse(time.DateOnly, *r.JSON200.ReleaseDate)
		if err != nil {
			return provider.GetDetailsRet{}, fmt.Errorf("parsing release date: %w", err)
		}
	}

	var posterPath string
	if r.JSON200.PosterPath != nil {
		posterPath = *r.JSON200.PosterPath
	}

	return provider.GetDetailsRet{
		Result: provider.SearchMoviesRetResult{
			ID:          strconv.Itoa(*r.JSON200.Id),
			Title:       *r.JSON200.Title,
			ReleaseDate: releaseDate,
			PosterPath:  posterPath,
		},
	}, nil
}

package tmdb

import (
	"errors"

	tmdb "github.com/stiflerGit/moviehat/gen/tmdb"
)

func validateSearchMovieResponse(r *tmdb.SearchMovieResponse) error {
	if r == nil {
		return errors.New("response is nil")
	}

	if r.JSON200 == nil {
		return errors.New("json response is nil")
	}

	if r.JSON200.Page == nil {
		return errors.New("page is nil")
	}

	if r.JSON200.TotalPages == nil {
		return errors.New("total_pages is nil")
	}

	if r.JSON200.TotalResults == nil {
		return errors.New("total_results is nil")
	}

	return nil
}

func validateMovieDetailsResponse(r *tmdb.MovieDetailsResponse) error {
	if r == nil {
		return errors.New("response is nil")
	}

	if r.JSON200 == nil {
		return errors.New("json response is nil")
	}

	if r.JSON200.Id == nil {
		return errors.New("id is nil")
	}

	if r.JSON200.Title == nil {
		return errors.New("title is nil")
	}

	return nil
}

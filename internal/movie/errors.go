package movie

import (
	"errors"
)

var (
	// ErrNotFound is returned when a movie id cannot be resolved.
	ErrNotFound = errors.New("not found")
)

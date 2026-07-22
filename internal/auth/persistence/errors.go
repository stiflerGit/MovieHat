package persistence

import "errors"

var (
	// ErrNotFound reports that a requested auth record does not exist.
	ErrNotFound = errors.New("resource not found")
	// ErrAlreadyExists reports that an auth record already exists.
	ErrAlreadyExists = errors.New("resource already exist")
)

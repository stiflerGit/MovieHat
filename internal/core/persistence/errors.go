package persistence

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound reports that a requested record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrSessionClosed reports that a session is already closed.
	ErrSessionClosed = errors.New("session closed")
	// ErrAlreadyExists reports that the resource already exists
	ErrAlreadyExists = errors.New("resource already exists")
)

// ErrInvalidArgument reports invalid storage input.
type ErrInvalidArgument struct {
	Err error
}

// Error returns the invalid argument message.
func (e ErrInvalidArgument) Error() string {
	return fmt.Sprintf("invalid argument: %v", e.Err)
}

// Unwrap returns the underlying invalid argument error.
func (e ErrInvalidArgument) Unwrap() error {
	return e.Err
}

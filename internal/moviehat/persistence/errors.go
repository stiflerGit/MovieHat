package persistence

import (
	"errors"
	"fmt"
)

var (
	// ErrNotFound reports that a requested record does not exist.
	ErrNotFound = errors.New("not found")
	// ErrSessionAlreadyExists reports that an open session already exists.
	ErrSessionAlreadyExists = errors.New("there is another open session")
	// ErrSessionClosed reports that a session is already closed.
	ErrSessionClosed = errors.New("session closed")
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

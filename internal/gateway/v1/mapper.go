package v1

import (
	"errors"

	"connectrpc.com/connect"
	"github.com/stiflerGit/moviehat/internal/persistence"
	"github.com/stiflerGit/moviehat/internal/session"
)

func repoErrorToAPIError(err error) error {
	_, ok := errors.AsType[*connect.Error](err)
	if ok {
		// error is already an API error
		return err
	}

	code := connect.CodeInternal
	switch {
	case errors.As(err, &persistence.ErrInvalidArgument{}):
		code = connect.CodeInvalidArgument
	case errors.Is(err, persistence.ErrNotFound):
		code = connect.CodeNotFound
	case errors.Is(err, persistence.ErrAlreadyExists):
		code = connect.CodeAlreadyExists
	case errors.Is(err, persistence.ErrSessionClosed) ||
		errors.As(err, &session.ErrFailedPrecondition{}):
		code = connect.CodeFailedPrecondition
	}
	return connect.NewError(code, err)
}

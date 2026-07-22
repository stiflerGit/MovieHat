package v1

import (
	"errors"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/moviehat/persistence"
	"google.golang.org/protobuf/types/known/timestamppb"

	"connectrpc.com/connect"
)

func pbUserToRepoUser(in *pb.User) persistence.User {
	return persistence.User{
		ID:   in.Id,
		Name: in.Name,
	}
}

func repoUserToPBUser(in persistence.User) *pb.User {
	return &pb.User{
		Id:   in.ID,
		Name: in.Name,
	}
}

func repoUsersToPBUsers(in ...persistence.User) []*pb.User {
	pbUsers := make([]*pb.User, 0, len(in))
	for _, u := range in {
		pbUsers = append(pbUsers, repoUserToPBUser(u))
	}
	return pbUsers
}

func repoSessionsToPB(in ...persistence.Session) []*pb.Session {
	if len(in) == 0 {
		return nil
	}

	sessions := make([]*pb.Session, 0, len(in))
	for _, s := range in {
		sessions = append(sessions, repoSessionToPBSession(s))
	}

	return sessions
}

func repoSessionToPBSession(in persistence.Session) *pb.Session {
	session := &pb.Session{
		Id:        in.ID,
		CreatedAt: timestamppb.New(in.CreatedAt),
		Winner:    &pb.User{Id: in.WinnerID},
	}

	if !in.ClosedAt.IsZero() {
		session.ClosedAt = timestamppb.New(in.ClosedAt)
	}

	return session
}

func repoMovieToPB(in persistence.Movie) *pb.Movie {
	return &pb.Movie{
		Id:    in.ID,
		Title: in.Title,
	}
}

func repoMoviesToPB(in ...persistence.Movie) []*pb.Movie {
	movies := make([]*pb.Movie, 0, len(in))
	for _, m := range in {
		movies = append(movies, repoMovieToPB(m))
	}
	return movies
}

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
	case errors.Is(err, persistence.ErrSessionAlreadyExists):
		code = connect.CodeAlreadyExists
	case errors.Is(err, persistence.ErrSessionClosed):
		code = connect.CodeFailedPrecondition
	}
	return connect.NewError(code, err)
}

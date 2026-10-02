package user

import (
	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/persistence"
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

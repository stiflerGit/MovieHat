package user

import (
	"context"
	"fmt"
	"log/slog"

	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/persistence"
)

// Handler provides MovieHat user, movie, and session operations.
type Handler struct {
	repository persistence.UsersStorage
	logger     *slog.Logger
}

// New creates a User handler.
func New(
	repository persistence.UsersStorage,
	options ...Option,
) *Handler {
	h := &Handler{
		repository: repository,
		logger:     slog.Default().With("component", "user"),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(h)
	}

	return h
}

// CreateUserRequest contains the auth user id to initialize in MovieHat.
type CreateUserRequest struct {
	UserID string
}

// CreateUserResponse contains the created MovieHat user.
type CreateUserResponse struct {
	User *pb.User
}

// CreateUser creates a MovieHat user for an auth user.
func (h *Handler) CreateUser(ctx context.Context, req CreateUserRequest) (CreateUserResponse, error) {
	user, err := h.repository.CreateUser(ctx, persistence.CreateUserArg{UserID: req.UserID})
	if err != nil {
		return CreateUserResponse{}, fmt.Errorf("h.repository.CreateUser: %w", err)
	}

	h.logger.InfoContext(ctx, "user created", "user_id", user.ID)

	return CreateUserResponse{User: repoUserToPBUser(user)}, nil
}

// ListUsers lists MovieHat users.
func (h *Handler) ListUsers(ctx context.Context, req *pb.ListUsersRequest) (*pb.ListUsersResponse, error) {
	users, err := h.repository.ListUsers(ctx, persistence.ListUsersArg{})
	if err != nil {
		return nil, fmt.Errorf("h.repository.ListUsers: %w", err)
	}

	v1Users := make([]*pb.User, 0, len(users))
	for _, u := range users {
		v1Users = append(v1Users, &pb.User{
			Id:   u.ID,
			Name: u.Name,
		})
	}

	return &pb.ListUsersResponse{Users: v1Users}, nil
}

type UpdateUserRequest struct {
	UserID string
	Name   string
}

// UpdateUser updates the current user's MovieHat profile.
func (h *Handler) UpdateUser(ctx context.Context, req UpdateUserRequest) (*pb.UpdateUserResponse, error) {
	user, err := h.repository.UpdateUser(ctx, persistence.UpdateUserArg{UserID: req.UserID, Name: req.Name})
	if err != nil {
		return nil, fmt.Errorf("h.repository.UpdateUser: %w", err)
	}

	h.logger.InfoContext(ctx, "user updated", "user_id", user.ID)

	return &pb.UpdateUserResponse{
		User: &pb.User{
			Id:   user.ID,
			Name: user.Name,
		},
	}, nil
}

type DeleteUserRequest struct {
	UserID string
}

// DeleteUser deletes the current user's MovieHat profile.
func (h *Handler) DeleteUser(ctx context.Context, req DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	user, err := h.repository.DeleteUser(ctx, persistence.DeleteUserArg{UserID: req.UserID})
	if err != nil {
		return nil, fmt.Errorf("h.repository.DeleteUser: %w", err)
	}

	h.logger.InfoContext(ctx, "user deleted", "user_id", user.ID)

	return &pb.DeleteUserResponse{
		User: &pb.User{
			Id:   user.ID,
			Name: user.Name,
		},
	}, nil
}

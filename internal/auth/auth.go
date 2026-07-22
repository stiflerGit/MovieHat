package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"buf.build/go/protovalidate"
	pb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/internal/auth/persistence"
	"google.golang.org/protobuf/types/known/timestamppb"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"golang.org/x/crypto/bcrypt"
)

const (
	defaultSessionDuration    = 7 * 24 * time.Hour
	defaultInvitationDuration = 15 * time.Minute
	defaultInvitationMaxUses  = 10
)

// Handler provides authentication, session, and invitation operations.
type Handler struct {
	storage persistence.TransactionalStorage
	secret  string
	logger  *slog.Logger
}

// New creates an authentication handler.
func New(storage persistence.TransactionalStorage, secret string, options ...Option) (*Handler, error) {
	if secret == "" {
		return nil, errors.New("secret is empty")
	}

	h := &Handler{
		storage: storage,
		secret:  secret,
		logger:  slog.Default().With("component", "auth"),
	}

	for _, opt := range options {
		if opt == nil {
			continue
		}
		opt(h)
	}

	return h, nil
}

// BootstrapCreateUserRequest describes the initial user to create during bootstrap.
type BootstrapCreateUserRequest struct {
	Email    string
	Password string
}

// BootstrapCreateUserResponse describes the user selected by bootstrap.
type BootstrapCreateUserResponse struct {
	UserID  string
	Created bool
}

// BootstrapCreateUser creates or returns the initial authentication user.
func (h *Handler) BootstrapCreateUser(ctx context.Context, req *BootstrapCreateUserRequest) (BootstrapCreateUserResponse, error) {
	if req == nil {
		return BootstrapCreateUserResponse{}, errors.New("request is nil")
	}

	if err := protovalidate.Validate(&pb.SignInRequest{Email: req.Email, Password: req.Password}); err != nil {
		return BootstrapCreateUserResponse{}, fmt.Errorf("validate request: %w", err)
	}

	cryptedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return BootstrapCreateUserResponse{}, fmt.Errorf("hashing password: %w", err)
	}

	resp := BootstrapCreateUserResponse{}
	err = h.storage.WithTx(ctx, func(ctx context.Context, storage persistence.Storage) error {
		getUserRet, err := storage.GetUser(ctx, persistence.GetUserArg{Email: req.Email})
		if err == nil {
			resp.UserID = getUserRet.User.ID
			resp.Created = false
			return nil
		}
		if !errors.Is(err, persistence.ErrNotFound) {
			return fmt.Errorf("storage.GetUser: %w", err)
		}

		insertUserRet, err := storage.InsertUser(ctx,
			persistence.InsertUserArg{
				Email:          req.Email,
				HashedPassword: string(cryptedPassword),
			},
		)
		if err != nil {
			return fmt.Errorf("storage.InsertUser: %w", err)
		}

		resp.UserID = insertUserRet.User.ID
		resp.Created = true
		return nil
	})
	if err != nil {
		return BootstrapCreateUserResponse{}, err
	}

	return resp, nil
}

// CreateInvitation creates a short-lived invitation token for new users.
func (h *Handler) CreateInvitation(ctx context.Context, req *pb.CreateInvitationRequest) (*pb.CreateInvitationResponse, error) {
	session, ok := authn.GetInfo(ctx).(Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session"))
	}

	token, err := generateCryptoSecureRandomString()
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("generateCryptoSecureRandomString: %w", err))
	}

	tokenHash, err := tokenHash(token, h.secret)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("tokenHash: %w", err))
	}

	insertVerificationRet, err := h.storage.InsertVerification(ctx,
		persistence.InsertVerificationArg{
			TokenHash: tokenHash,
			ExpiresAt: time.Now().Add(defaultInvitationDuration),
			MaxUses:   defaultInvitationMaxUses,
			AuthorID:  session.UserId,
		},
	)
	if err != nil {
		h.logger.ErrorContext(ctx, "CreateInvitation h.storage.InsertVerification", "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("h.storage.InsertVerification: %w", err))
	}

	return &pb.CreateInvitationResponse{
		InvitationToken: token,
		ExpiresAt:       timestamppb.New(insertVerificationRet.Verification.ExpiresAt),
		MaxUses:         int32(insertVerificationRet.Verification.MaxUses),
	}, nil
}

// SignUpResponse contains the created user and session token.
type SignUpResponse struct {
	UserID string
	Token  string
}

// SignUp creates an invited user account and returns a session token.
func (h *Handler) SignUp(ctx context.Context, req *pb.SignUpRequest) (SignUpResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return SignUpResponse{}, connect.NewError(connect.CodeInvalidArgument, err)
	}

	invitationTokenHash, err := tokenHash(req.InvitationToken, h.secret)
	if err != nil {
		h.logger.ErrorContext(ctx, "SignUp tokenHash", "error", err)
		return SignUpResponse{}, connect.NewError(connect.CodeInternal, fmt.Errorf("tokenHash: %w", err))
	}

	cryptedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return SignUpResponse{}, connect.NewError(connect.CodeInternal, errors.New("hashing the password"))
	}

	var userID, sessionToken string
	err = h.storage.WithTx(ctx, func(ctx context.Context, storage persistence.Storage) error {
		_, err := storage.ConsumeVerification(ctx, persistence.ConsumeVerificationArg{TokenHash: invitationTokenHash})
		if err != nil {
			if errors.Is(err, persistence.ErrNotFound) {
				return connect.NewError(connect.CodeFailedPrecondition, errors.New("invitation is not valid"))
			}
			h.logger.ErrorContext(ctx, "SignUp storage.ConsumeVerification", "error", err)
			return connect.NewError(connect.CodeInternal, fmt.Errorf("storage.ConsumeVerification: %w", err))
		}

		insertUserRet, err := storage.InsertUser(ctx,
			persistence.InsertUserArg{
				Email:          req.Email,
				HashedPassword: string(cryptedPassword),
			},
		)
		if err != nil {
			if errors.Is(err, persistence.ErrAlreadyExists) {
				h.logger.WarnContext(ctx, "sign up failed: user already exists", "email", req.Email)
				return connect.NewError(connect.CodeAlreadyExists, err)
			}
			h.logger.ErrorContext(ctx, "sign up failed", "email", req.Email, "error", err)
			return connect.NewError(connect.CodeInternal, err)
		}

		userID = insertUserRet.User.ID
		sessionToken, err = h.createSession(ctx, storage, insertUserRet.User.ID)
		if err != nil {
			h.logger.ErrorContext(ctx, "SignUp h.createSession", "error", err)
			// let's not return an error. The user will retry to login
			return err
		}

		return nil
	})
	if err != nil {
		return SignUpResponse{}, err
	}

	h.logger.InfoContext(ctx, "user signed up", "user_id", userID)

	return SignUpResponse{UserID: userID, Token: sessionToken}, nil
}

func (h *Handler) createSession(ctx context.Context, storage persistence.Storage, userID string) (token string, err error) {
	token, err = generateCryptoSecureRandomString()
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("generateCryptoSecureRandomString: %w", err))
	}

	tokenHash, err := tokenHash(token, h.secret)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("tokenHash: %w", err))
	}

	clock := time.Now()
	expiresAt := clock.Add(defaultSessionDuration)

	_, err = storage.InsertSession(
		ctx,
		persistence.InsertSessionArg{
			Token:     tokenHash,
			UserID:    userID,
			ExpiresAt: expiresAt,
		},
	)
	if err != nil {
		return "", connect.NewError(connect.CodeInternal, fmt.Errorf("h.repository.InsertSession: %w", err))
	}

	return token, nil
}

// SignIn authenticates a user and returns a session token.
func (h *Handler) SignIn(ctx context.Context, req *pb.SignInRequest) (*pb.SignInResponse, error) {
	if err := protovalidate.Validate(req); err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	getUserRet, err := h.storage.GetUser(ctx, persistence.GetUserArg{Email: req.Email})
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			h.logger.WarnContext(ctx, "sign in failed", "email", req.Email, "reason", "user_not_found")
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid email or password"))
		}
		h.logger.ErrorContext(ctx, "sign in failed", "email", req.Email, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf(" h.repository.GetUser: %w", err))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(getUserRet.User.HashedPassword), []byte(req.Password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			h.logger.WarnContext(ctx, "sign in failed", "email", req.Email, "reason", "invalid_password")
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid email or password"))
		}
		h.logger.ErrorContext(ctx, "sign in password comparison failed", "email", req.Email, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("bcrypt.CompareHashAndPassword: %w", err))
	}

	token, err := h.createSession(ctx, h.storage, getUserRet.User.ID)
	if err != nil {
		h.logger.ErrorContext(ctx, "SignIn h.createSession failed", "email", req.Email, "error", err)
		return nil, err
	}

	return &pb.SignInResponse{Token: token}, nil
}

// SignOut expires the current session.
func (h *Handler) SignOut(ctx context.Context, req *pb.SignOutRequest) (*pb.SignOutResponse, error) {
	session, ok := authn.GetInfo(ctx).(Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing token"))
	}

	now := time.Now()
	_, err := h.storage.UpdateSession(
		ctx,
		persistence.UpdateSessionArg{
			ID:         session.ID,
			LastAccess: &now,
			ExpiresAt:  &now,
		},
	)
	if err != nil {
		h.logger.ErrorContext(ctx, "sign out failed", "session_id", session.ID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, fmt.Errorf("h.repository.InsertSession: %w", err))
	}

	h.logger.InfoContext(ctx, "user signed out", "session_id", session.ID)
	return &pb.SignOutResponse{}, err
}

func (h *Handler) getSession(ctx context.Context, token string) (persistence.Session, error) {
	tokenHash, err := tokenHash(token, h.secret)
	if err != nil {
		return persistence.Session{}, connect.NewError(connect.CodeInternal, fmt.Errorf("tokenHash: %w", err))
	}

	getSessionRet, err := h.storage.GetSession(ctx, persistence.GetSessionArg{Token: tokenHash})
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			return persistence.Session{}, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
		}
		return persistence.Session{}, connect.NewError(connect.CodeInternal, err)
	}

	return getSessionRet.Session, nil
}

// ValidateSessionRequest contains the bearer token to validate.
type ValidateSessionRequest struct {
	Token string
}

// ValidateSessionResponse contains the validated session.
type ValidateSessionResponse struct {
	Session Session
}

// Session describes an authenticated user session.
type Session struct {
	ID         string
	Token      string
	UserId     string
	CreatedAt  time.Time
	ExpiresAt  time.Time
	LastAccess time.Time
}

// ValidateSession validates a bearer token and extends the session lifetime.
func (h *Handler) ValidateSession(ctx context.Context, req *ValidateSessionRequest) (*ValidateSessionResponse, error) {
	if len(req.Token) == 0 {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("token is empty"))
	}

	session, err := h.getSession(ctx, req.Token)
	if err != nil {
		return nil, err
	}

	if time.Now().After(session.ExpiresAt) {
		h.logger.WarnContext(ctx, "session validation failed", "session_id", session.ID, "reason", "expired")
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("token expired"))
	}

	now := time.Now()
	expiresAt := now.Add(defaultSessionDuration)
	updateSessionRet, err := h.storage.UpdateSession(ctx, persistence.UpdateSessionArg{ID: session.ID, LastAccess: &now, ExpiresAt: &expiresAt})
	if err != nil {
		h.logger.ErrorContext(ctx, "session validation failed to update session", "session_id", session.ID, "error", err)
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	return &ValidateSessionResponse{
		Session: Session{
			ID:         updateSessionRet.Session.ID,
			Token:      updateSessionRet.Session.Token,
			UserId:     updateSessionRet.Session.UserID,
			CreatedAt:  updateSessionRet.Session.CreatedAt,
			ExpiresAt:  updateSessionRet.Session.ExpiresAt,
			LastAccess: updateSessionRet.Session.LastAccess,
		},
	}, nil
}

// DeleteUser deletes the authenticated user's auth account.
func (h *Handler) DeleteUser(ctx context.Context, req *pb.DeleteUserRequest) (*pb.DeleteUserResponse, error) {
	session, ok := authn.GetInfo(ctx).(Session)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing session in request"))
	}

	deleteUserRet, err := h.storage.DeleteUser(ctx, persistence.DeleteUserArg{UserID: session.UserId})
	if err != nil {
		if errors.Is(err, persistence.ErrNotFound) {
			h.logger.WarnContext(ctx, "delete user failed", "user_id", session.UserId, "reason", "not_found")
			return nil, connect.NewError(connect.CodeNotFound, errors.New("user not found"))
		}
		h.logger.ErrorContext(ctx, "delete user failed", "user_id", session.UserId, "error", err)
		return nil, connect.NewError(connect.CodeInternal, errors.New("delete user failed"))
	}

	h.logger.InfoContext(ctx, "auth user deleted", "user_id", deleteUserRet.User.ID)
	return &pb.DeleteUserResponse{
		User: &pb.User{
			Id: deleteUserRet.User.ID,
		},
	}, nil
}

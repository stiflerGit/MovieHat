package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"

	gatewaypb "github.com/stiflerGit/moviehat/api/gateway/v1"
	"github.com/stiflerGit/moviehat/api/gateway/v1/gatewayv1connect"
	"github.com/stiflerGit/moviehat/internal/auth"
	authstorage "github.com/stiflerGit/moviehat/internal/auth/persistence/sqlite"
	extractor "github.com/stiflerGit/moviehat/internal/extractor/fair_share"
	extractorstorage "github.com/stiflerGit/moviehat/internal/extractor/fair_share/persistence/sqlite"
	gateway "github.com/stiflerGit/moviehat/internal/gateway/v1"
	appmigrations "github.com/stiflerGit/moviehat/internal/migrations"
	moviehat "github.com/stiflerGit/moviehat/internal/moviehat"
	moviehatsqlite "github.com/stiflerGit/moviehat/internal/moviehat/persistence/sqlite"

	"connectrpc.com/authn"
	"connectrpc.com/connect"
	"connectrpc.com/grpchealth"
	"connectrpc.com/validate"
	env "github.com/caarlos0/env/v11"
	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

// Config contains server settings loaded from the environment.
type Config struct {
	Addr                  string `env:"ADDR" envDefault:"0.0.0.0:8080"`
	DBPath                string `env:"DB_PATH" envDefault:"./moviehat.db"`
	AuthSecret            string `env:"AUTH_SECRET,required"`
	FrontendInvitationURL string `env:"FRONTEND_INVITATION_URL" envDefault:"https://moviehat.app/invite"`
	BootstrapEnabled      bool   `env:"BOOTSTRAP_ENABLED" envDefault:"false"`
	BootstrapEmail        string `env:"BOOTSTRAP_EMAIL"`
	BootstrapPassword     string `env:"BOOTSTRAP_PASSWORD"`
}

func main() {
	ctx := context.Background()

	var config Config
	err := env.Parse(&config)
	if err != nil {
		slog.ErrorContext(ctx, "env.Parse", "error", err)
		os.Exit(1)
	}

	if err := run(ctx, config); err != nil {
		slog.ErrorContext(ctx, "run failed", "error", err)
		os.Exit(1)
	}

	os.Exit(0)
}

func run(ctx context.Context, config Config) error {
	db, err := initDB(ctx, config.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	slog.InfoContext(ctx, "starting moviehat server", "addr", config.Addr, "dbPath", config.DBPath)

	logger := slog.Default()

	authHandler, err := auth.New(authstorage.New(db), config.AuthSecret, auth.WithLogger(logger))
	if err != nil {
		return fmt.Errorf("auth.New: %w", err)
	}
	movieHatHandler := moviehat.New(moviehatsqlite.New(db), extractor.New(extractorstorage.New(db)), moviehat.WithLogger(logger))

	if err := bootstrapInitialUser(ctx, config, authHandler, movieHatHandler); err != nil {
		return fmt.Errorf("bootstrapInitialUser: %w", err)
	}

	frontendInvitationURL, err := url.Parse(config.FrontendInvitationURL)
	if err != nil {
		return fmt.Errorf("url.Parse(config.FrontendInvitationURL): %w", err)
	}

	gatewayHandler := gateway.New(authHandler, movieHatHandler, gateway.WithLogger(logger), gateway.WithInvitationBaseURL(frontendInvitationURL))

	path, handler := gatewayv1connect.NewGatewayServiceHandler(
		gatewayHandler,
		connect.WithInterceptors(
			validate.NewInterceptor(),
		),
	)

	publicProcedures := map[string]bool{
		gatewayv1connect.GatewayServiceSignUpProcedure: true,
		gatewayv1connect.GatewayServiceSignInProcedure: true,
	}

	handler = authn.NewMiddleware(func(ctx context.Context, req *http.Request) (any, error) {
		procedure, ok := authn.InferProcedure(req.URL)
		if ok && publicProcedures[procedure] {
			return nil, nil
		}

		token, ok := authn.BearerToken(req)
		if !ok || token == "" {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing token"))
		}

		session, err := authHandler.ValidateSession(ctx, &auth.ValidateSessionRequest{Token: token})
		if err != nil || session == nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid token"))
		}

		return session.Session, nil
	}).Wrap(handler)

	mux := http.NewServeMux()
	mux.Handle(path, handler)
	mux.Handle(grpchealth.NewHandler(grpchealth.NewStaticChecker(gatewayv1connect.GatewayServiceName)))

	p := new(http.Protocols)

	p.SetHTTP1(true)
	// Use h2c so we can serve HTTP/2 without TLS.
	p.SetUnencryptedHTTP2(true)
	s := http.Server{
		Addr:      config.Addr,
		Handler:   mux,
		Protocols: p,
	}

	ctx, cancel := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer cancel()

	errorsCh := make(chan error, 1)
	go func() {
		slog.InfoContext(ctx, "http server listening", "addr", config.Addr)
		errorsCh <- s.ListenAndServe()
	}()

	select {
	case err := <-errorsCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("s.ListenAndServe: %w", err)

	case <-ctx.Done():
		slog.InfoContext(context.Background(), "shutdown signal received")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := s.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("server shutdown: %w", err)
		}

		err := <-errorsCh
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("listen and serve after shutdown: %w", err)
		}

		slog.InfoContext(context.Background(), "server stopped")
		return nil
	}
}

func runMigrations(ctx context.Context, db *sql.DB) error {
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, appmigrations.FS)
	if err != nil {
		return fmt.Errorf("goose.NewProvider: %w", err)
	}

	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("provider.Up: %w", err)
	}
	return nil
}

func initDB(ctx context.Context, dbPath string) (*sql.DB, error) {
	dbFile, err := os.OpenFile(dbPath, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		return nil, fmt.Errorf("os.OpenFile: %w", err)
	}
	defer dbFile.Close()

	dsn := "file:" + dbPath + "?_pragma=foreign_keys(1)"

	slog.InfoContext(ctx, "opening database", "dsn", dsn)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("sql.Open: %w", err)
	}

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("db ping: %w", err)
	}

	if err := runMigrations(ctx, db); err != nil {
		return nil, fmt.Errorf("runMigrations: %w", err)
	}

	return db, nil
}

func bootstrapInitialUser(ctx context.Context, config Config, authHandler *auth.Handler, movieHatHandler *moviehat.Handler) error {
	if !config.BootstrapEnabled {
		return nil
	}

	if config.BootstrapEmail == "" || config.BootstrapPassword == "" {
		return errors.New("BOOTSTRAP_EMAIL and BOOTSTRAP_PASSWORD are required when BOOTSTRAP_ENABLED=true")
	}

	bootstrapRet, err := authHandler.BootstrapCreateUser(ctx,
		&auth.BootstrapCreateUserRequest{
			Email:    config.BootstrapEmail,
			Password: config.BootstrapPassword,
		},
	)
	if err != nil {
		return fmt.Errorf("authHandler.BootstrapCreateUser: %w", err)
	}

	listUsersRet, err := movieHatHandler.ListUsers(ctx, &gatewaypb.ListUsersRequest{})
	if err != nil {
		return fmt.Errorf("movieHatHandler.ListUsers: %w", err)
	}

	for _, user := range listUsersRet.Users {
		if user.Id == bootstrapRet.UserID {
			slog.InfoContext(ctx, "bootstrap user ready", "auth_user_id", bootstrapRet.UserID, "created_auth_user", bootstrapRet.Created, "created_moviehat_user", false)
			return nil
		}
	}

	_, err = movieHatHandler.CreateUser(ctx, moviehat.CreateUserRequest{UserID: bootstrapRet.UserID})
	if err != nil {
		return fmt.Errorf("movieHatHandler.CreateUser: %w", err)
	}

	slog.InfoContext(ctx, "bootstrap user ready", "auth_user_id", bootstrapRet.UserID, "created_auth_user", bootstrapRet.Created, "created_moviehat_user", true)
	return nil
}

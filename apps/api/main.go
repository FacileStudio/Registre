// Command api is registre, the Facile Suite's OpenID Provider.
//
// Read SPEC.md before changing anything here. The contract this binary has to
// satisfy is porte's, and it is frozen: SPEC §6 lists what breaks silently if
// a line of it is missed.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/FacileStudio/Registre/apps/api/internal/clients"
	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/crypto"
	"github.com/FacileStudio/Registre/apps/api/internal/database"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/internal/identity"
	"github.com/FacileStudio/Registre/apps/api/internal/keys"
	"github.com/FacileStudio/Registre/apps/api/internal/seed"
	"github.com/FacileStudio/Registre/apps/api/internal/web"
	"github.com/FacileStudio/Registre/apps/api/migrations"
	"github.com/FacileStudio/Registre/apps/api/modules/login"
	"github.com/FacileStudio/Registre/apps/api/modules/oidc"

	"github.com/FacileStudio/Journal/sdk/journal"
	"github.com/FacileStudio/tronc/health"
	"github.com/FacileStudio/tronc/healthcheck"
	"github.com/FacileStudio/tronc/httpx"
	"github.com/FacileStudio/tronc/logger"
	troncmiddleware "github.com/FacileStudio/tronc/middleware"
	"github.com/FacileStudio/tronc/migrate"
	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"
)

// tokenRecordRetention is how long a token's row outlives the token itself.
// Revoking something already expired is a no-op; answering "who called this
// app last Tuesday" is not, and that answer has to survive the night.
const tokenRecordRetention = 30 * 24 * time.Hour

// shutdownGrace is how long an in-flight token request has to finish.
const shutdownGrace = 10 * time.Second

func main() {
	if healthcheck.Handle(os.Args) {
		return
	}
	os.Exit(run())
}

// observe builds the process logger, ships it to Journal when the instance is
// configured, and makes it the default.
//
// The default matters as much as the return value. zitadel/oidc emits every
// protocol error — a refused client, a scope the register does not grant, an
// unsupported grant — through the package-level slog rather than through any
// logger it is handed, because op.WithLogger accepts one and discards it.
// Without SetDefault those lines keep slog's own text format, ignore LOG_LEVEL
// and never reach Journal, and for an identity provider they are the lines
// that matter most.
//
// The returned client is nil unless Journal is configured, and is the caller's
// to close.
func observe(cfg env.Config) (*slog.Logger, *journal.Client) {
	var client *journal.Client
	log := logger.New(logger.Config{
		Level: cfg.LogLevel,
		Wrap: func(handler slog.Handler) slog.Handler {
			if cfg.JournalURL == "" || cfg.JournalToken == "" {
				return handler
			}
			client = journal.New(journal.Config{URL: cfg.JournalURL, Token: cfg.JournalToken})
			return journal.NewHandler(client, handler)
		},
	})
	slog.SetDefault(log)
	return log, client
}

func run() int {
	cfg, err := env.Load()
	log, shipper := observe(cfg)
	if shipper != nil {
		defer shipper.Close()
	}
	if err != nil {
		log.Error("registre: failed to load the configuration", slog.Any("error", err))
		return 1
	}
	root, err := crypto.KeyFromBase64(cfg.EncryptionKey)
	if err != nil {
		log.Error("registre: invalid ENCRYPTION_KEY", slog.Any("error", err))
		return 1
	}

	db, err := database.Open(cfg.DatabaseURL)
	if err != nil {
		log.Error("registre: failed to open the database", slog.Any("error", err))
		return 1
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Error("registre: failed to reach the database handle", slog.Any("error", err))
		return 1
	}
	defer func() {
		if err := sqlDB.Close(); err != nil {
			log.Error("registre: failed to close the database", slog.Any("error", err))
		}
	}()

	if code, handled := migrateSchema(sqlDB, log); handled {
		return code
	}

	verifier := credentials.New(cfg.VerifyConcurrency)
	if len(os.Args) > 1 && os.Args[1] == "user-create" {
		return runUserCreate(context.Background(), db, verifier, log)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router, err := assemble(ctx, db, root, cfg, services{verifier: verifier, log: log})
	if err != nil {
		log.Error("registre: failed to start", slog.Any("error", err))
		return 1
	}
	return serve(router, cfg, log)
}

// migrateSchema runs the schema forward, or answers a migrate subcommand and
// reports that the process is done. A failed migration exits 1 rather than
// serving on a schema nobody described.
func migrateSchema(sqlDB *sql.DB, log *slog.Logger) (int, bool) {
	cfg := migrate.Config{DB: sqlDB, FS: migrations.FS, Logger: log}
	if handled, err := migrate.Command(context.Background(), os.Args, cfg); handled {
		if err != nil {
			log.Error("registre: migrate", slog.Any("error", err))
			return 1, true
		}
		return 0, true
	}
	if err := migrate.Run(context.Background(), cfg); err != nil {
		log.Error("registre: failed to run the migrations", slog.Any("error", err))
		return 1, true
	}
	return 0, false
}

// assemble builds every service and returns the router they are mounted on.
// services bundles the process-wide dependencies assemble wires into routes,
// so the function stays under the parameter ceiling as registre grows.
type services struct {
	verifier *credentials.Verifier
	log      *slog.Logger
}

// assemble wires the process's services into routes. The dependencies travel
// together as one value so the function stays inside the parameter ceiling.
//
// Route precedence is explicit here because chi is last-registration-wins:
// the embedded client mounts first and login.Mount second, so the
// server-rendered form owns GET and POST /login as the no-JS fallback while
// the client app serves every other /login path, assets included. Swapping
// the two registrations silently deletes the fallback.
func assemble(
	ctx context.Context,
	db *gorm.DB,
	root []byte,
	cfg env.Config,
	svc services,
) (chi.Router, error) {
	keyring := keys.New(db, root, keys.Config{
		RotateEvery:  cfg.RotationEvery,
		PublishDelay: cfg.PublishDelay,
		Retention:    cfg.KeyRetention,
	}, svc.log)
	if err := keyring.Ensure(ctx); err != nil {
		return nil, err
	}
	keyring.Start(ctx)

	register := clients.New(db, svc.verifier)
	if err := applySeed(ctx, register, cfg.SeedFile, svc.log); err != nil {
		return nil, err
	}
	users := identity.New(db, svc.verifier)

	storage := oidc.NewStorage(db, oidc.Services{
		Keys: keyring, Register: register, Users: users,
	}, oidc.Config{
		Issuer:         cfg.Issuer,
		AccessTokenTTL: cfg.AccessTokenTTL,
	}, svc.log)
	storage.StartPurge(ctx, tokenRecordRetention)

	provider, err := oidc.NewProvider(storage, oidc.ProviderConfig{
		Issuer:   cfg.Issuer,
		RootKey:  root,
		Insecure: cfg.InsecureIssuer(),
	})
	if err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	router := httpx.NewRouter(httpx.Config{
		TrustedProxies: cfg.TrustedProxies,
		CDNProxies:     cfg.CDNProxies,
		CDNHeader:      cfg.CDNHeader,
		Logger:         svc.log,
		CORS: troncmiddleware.CORSConfig{
			AllowedOrigins:   cfg.CORSAllowedOrigins,
			AllowCredentials: true,
		},
	})
	health.Mount(router, health.DB(sqlDB))
	oidc.Mount(router, provider)
	router.Handle("/login", web.Handler())
	router.Handle("/login/*", web.Handler())
	login.Mount(router, login.New(users, svc.log))
	return router, nil
}

// applySeed brings the register up to the file, when one is configured.
func applySeed(ctx context.Context, register *clients.Service, path string, log *slog.Logger) error {
	if path == "" {
		log.Info("registre: no seed file configured, the database is the only source of clients")
		return nil
	}
	file, err := seed.Load(path)
	if err != nil {
		return err
	}
	return seed.Apply(ctx, register, file, log)
}

// serve runs the HTTP server until it is signalled to stop.
func serve(router chi.Router, cfg env.Config, log *slog.Logger) int {
	addr := ":" + strconv.Itoa(cfg.Port)
	server := &http.Server{
		Addr:              addr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	failed := make(chan error, 1)
	go func() { failed <- server.ListenAndServe() }()

	stopping, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	log.Info("registre: listening", slog.String("addr", addr), slog.String("issuer", cfg.Issuer))
	select {
	case err := <-failed:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Error("registre: the server stopped", slog.Any("error", err))
			return 1
		}
	case <-stopping.Done():
		graceful, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		if err := server.Shutdown(graceful); err != nil {
			log.Error("registre: shutdown failed", slog.Any("error", err))
			return 1
		}
		log.Info("registre: stopped")
	}
	return 0
}

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

	"github.com/FacileStudio/Registre/apps/api/internal/credentials"
	"github.com/FacileStudio/Registre/apps/api/internal/crypto"
	"github.com/FacileStudio/Registre/apps/api/internal/database"
	"github.com/FacileStudio/Registre/apps/api/internal/env"
	"github.com/FacileStudio/Registre/apps/api/migrations"

	"github.com/FacileStudio/Journal/sdk/journal"
	"github.com/FacileStudio/tronc/healthcheck"
	"github.com/FacileStudio/tronc/logger"
	"github.com/FacileStudio/tronc/migrate"
	"github.com/go-chi/chi/v5"
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
	cfg, plans, err := env.LoadAll()
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

	db, err := database.Open(cfg.DatabaseURL, database.Pool{
		MaxOpen: cfg.DBMaxOpen, MaxIdle: cfg.DBMaxIdle, MaxLifetime: cfg.DBMaxLifetime,
	})
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

	verifier := credentials.New(cfg.VerifyConcurrency).WithQueueWait(cfg.VerifyWait)
	if len(os.Args) > 1 && os.Args[1] == "user-create" {
		return runUserCreate(context.Background(), db, verifier, log)
	}
	if len(os.Args) > 1 && os.Args[1] == "user-disable" {
		return runUserDisable(context.Background(), db, verifier, log)
	}
	if len(os.Args) > 1 && os.Args[1] == "break-glass" {
		return runBreakGlass(context.Background(), db, verifier, log)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	router, err := assemble(ctx, db, root, cfg, services{verifier: verifier, log: log, billing: plans})
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

package main

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/api"
	"probakgo/internal/config"
	dbpkg "probakgo/internal/db"
	"probakgo/internal/service"
	"probakgo/internal/store"
	"probakgo/internal/web"
)

var version = "0.0.1"

// web/ is at the project root, same directory as this file.
//
//go:embed web
var webFS embed.FS

func main() {
	_ = godotenv.Load()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		slog.Warn("unknown timezone, falling back to UTC", "tz", cfg.Timezone)
		loc = time.UTC
	}

	db, err := dbpkg.Open(cfg.DBPath)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	st := store.New(db)

	if err := ensureDefaults(st); err != nil {
		slog.Error("bootstrap defaults", "err", err)
		os.Exit(1)
	}

	web.InitSessions(cfg.SessionKey)

	authSvc := service.NewAuth(st)
	reportSvc := service.NewReport(st, loc)

	// Static sub-FS so /static/... maps to web/static/...
	staticSub, err := fs.Sub(webFS, "web/static")
	if err != nil {
		slog.Error("static sub-fs", "err", err)
		os.Exit(1)
	}

	apiSrv := api.NewServer(st, authSvc, reportSvc)
	webRouter, err := web.NewRouter(st, reportSvc, webFS, staticSub)
	if err != nil {
		slog.Error("build web router", "err", err)
		os.Exit(1)
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	service.StartEmailScheduler(appCtx, st, reportSvc)
	service.StartCleanupScheduler(appCtx, st)

	mux := http.NewServeMux()
	mux.Handle("/api/", http.StripPrefix("/api", apiSrv.Router()))
	mux.Handle("/", webRouter)

	addr := fmt.Sprintf("%s:%s", cfg.APIHost, cfg.APIPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	slog.Info("probakgo started", "addr", "http://"+addr)

	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	slog.Info("shutting down...")
	appCancel()
	shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutCancel()
	_ = srv.Shutdown(shutCtx)
}

func ensureDefaults(st *store.Store) error {
	hasUsers, err := st.HasUsers()
	if err != nil {
		return err
	}
	if !hasUsers {
		hash, _ := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if _, err := st.CreateUser("probakgo", string(hash), "admin"); err != nil {
			return err
		}
		slog.Warn("⚠  default user created - CHANGE PASSWORD IMMEDIATELY",
			"username", "probakgo", "password", "admin123")
	}
	hasKey, err := st.HasAdminKey()
	if err != nil {
		return err
	}
	if !hasKey {
		k, err := st.CreateAPIKey("Admin key", "admin", "")
		if err != nil {
			return err
		}
		slog.Warn("⚠  admin API key created - store it securely", "key", k.Key)
	}
	return nil
}

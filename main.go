package main

import (
	"context"
	"crypto/rand"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/api"
	"probakgo/internal/config"
	dbpkg "probakgo/internal/db"
	"probakgo/internal/selfupdate"
	"probakgo/internal/service"
	"probakgo/internal/store"
	"probakgo/internal/web"
)

var version = "0.0.5"

// web/ is at the project root, same directory as this file.
//
//go:embed web
var webFS embed.FS

const (
	serverCronPath    = "/etc/cron.d/probakgo"
	serverServicePath = "/etc/systemd/system/probakgo.service"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "update" {
		if err := selfupdate.Run("Nestorm18/probakgo", "probakgo", version); err != nil {
			slog.Error("update failed", "err", err)
			os.Exit(1)
		}
		restartService()
		return
	}

	_ = godotenv.Load()
	ensureSessionKey()

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

	web.InitSessions(cfg.SessionKey, cfg.SecureSession)

	authSvc := service.NewAuth(st)
	reportSvc := service.NewReport(st, loc)

	// Static sub-FS so /static/... maps to web/static/...
	staticSub, err := fs.Sub(webFS, "web/static")
	if err != nil {
		slog.Error("static sub-fs", "err", err)
		os.Exit(1)
	}

	apiSrv := api.NewServer(st, authSvc, reportSvc)
	webRouter, err := web.NewRouter(st, reportSvc, webFS, staticSub, cfg.SessionKey, cfg.SecureSession)
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

	ensureUpdateCron()
	ensureSystemdService()
	slog.Info("probakgo started", "addr", "http://"+addr, "version", version)

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

// ensureSessionKey generates a SESSION_KEY and persists it to .env if not already set.
func ensureSessionKey() {
	if os.Getenv("SESSION_KEY") != "" {
		return
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return
	}
	key := hex.EncodeToString(b)
	os.Setenv("SESSION_KEY", key)

	f, err := os.OpenFile(".env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		slog.Warn("SESSION_KEY generated but could not persist to .env", "err", err)
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "SESSION_KEY=%s\n", key)
	slog.Info("SESSION_KEY generated and saved to .env")
}

// ensureSystemdService installs the systemd service on first startup when running as root.
func ensureSystemdService() {
	if os.Getuid() != 0 {
		return
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	if _, err := os.Stat(serverServicePath); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	workDir := filepath.Dir(exe)

	content := fmt.Sprintf(`[Unit]
Description=probakgo Proxmox Monitor
After=network.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, workDir, exe)

	if err := os.WriteFile(serverServicePath, []byte(content), 0644); err != nil {
		slog.Warn("could not install systemd service", "err", err)
		return
	}
	exec.Command("systemctl", "daemon-reload").Run()
	exec.Command("systemctl", "enable", "probakgo").Run()
	slog.Info("systemd service installed and enabled", "path", serverServicePath)
}

// ensureUpdateCron writes /etc/cron.d/probakgo on first startup when running as root.
func ensureUpdateCron() {
	if os.Getuid() != 0 {
		return
	}
	if _, err := os.Stat(serverCronPath); err == nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	content := fmt.Sprintf("0 1 * * * root %s update >> /var/log/probakgo-update.log 2>&1\n", exe)
	if err := os.WriteFile(serverCronPath, []byte(content), 0644); err != nil {
		slog.Warn("could not install update cron", "err", err)
	} else {
		slog.Info("auto-update cron installed", "path", serverCronPath, "schedule", "01:00 daily")
	}
}

// restartService attempts to restart the probakgo systemd service after an update.
func restartService() {
	if _, err := exec.LookPath("systemctl"); err != nil {
		slog.Info("update applied — restart the service manually to use the new version")
		return
	}
	slog.Info("update applied — restarting service...")
	if err := exec.Command("systemctl", "restart", "probakgo").Run(); err != nil {
		slog.Warn("systemctl restart failed — restart manually", "err", err)
	}
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

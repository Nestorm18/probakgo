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
	"probakgo/internal/session"
	"probakgo/internal/store"
	"probakgo/internal/web"
)

var version = "0.0.168"

// web/ is at the project root, same directory as this file.
//
//go:embed web
var webFS embed.FS

const (
	serverCronPath    = "/etc/cron.d/probakgo"
	serverServicePath = "/etc/systemd/system/probakgo.service"
)

func main() {
	loadEnv()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("probakgo v%s\n", version)
			return
		case "update":
			updated, err := selfupdate.Run("Nestorm18/probakgo", "probakgo", version)
			if err != nil {
				slog.Error("update failed", "err", err)
				os.Exit(1)
			}
			if updated {
				restartService()
			}
			return
		case "doctor":
			if err := runServerDoctor(); err != nil {
				os.Exit(1)
			}
			return
		case "unlock2fa":
			if len(os.Args) < 3 {
				fmt.Fprintln(os.Stderr, "usage: probakgo unlock2fa <usuario>")
				os.Exit(2)
			}
			if err := unlock2FA(os.Args[2]); err != nil {
				fmt.Fprintln(os.Stderr, "unlock2fa:", err)
				os.Exit(1)
			}
			fmt.Printf("2FA disabled for user %q.\n", os.Args[2])
			return
		}
	}

	ensureSessionKey()

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		slog.Error("invalid configuration", "err", err)
		os.Exit(1)
	}

	loc, _ := time.LoadLocation(cfg.Timezone)

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

	session.Init(cfg.SessionKey, cfg.SecureSession)

	authSvc := service.NewAuth(st)
	reportSvc := service.NewReport(st, loc)

	// Static sub-FS so /static/... maps to web/static/...
	staticSub, err := fs.Sub(webFS, "web/static")
	if err != nil {
		slog.Error("static sub-fs", "err", err)
		os.Exit(1)
	}

	apiSrv := api.NewServer(st, authSvc, reportSvc, cfg.TrustedProxies)
	webRouter, err := web.NewRouter(st, reportSvc, webFS, staticSub, cfg.SessionKey, cfg.SecureSession, cfg.TrustedOrigins, cfg.TrustedProxies, version, cfg.Dev, loc)
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
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    16 << 10,
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

func loadEnv() {
	_ = godotenv.Load()
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	_ = godotenv.Load(filepath.Join(filepath.Dir(exe), ".env"))
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
ExecStart="%s"
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
	workDir := filepath.Dir(exe)
	content := fmt.Sprintf("0 1 * * * root cd \"%s\" && \"%s\" update >> /var/log/probakgo-update.log 2>&1\n", workDir, exe)
	if existing, err := os.ReadFile(serverCronPath); err == nil && string(existing) == content {
		return
	}
	if err := os.WriteFile(serverCronPath, []byte(content), 0644); err != nil {
		slog.Warn("could not install update cron", "err", err)
	} else {
		slog.Info("auto-update cron installed", "path", serverCronPath, "schedule", "01:00 daily")
	}
}

// restartService attempts to restart the probakgo systemd service after an update.
func restartService() {
	if _, err := exec.LookPath("systemctl"); err != nil {
		slog.Info("update applied - restart the service manually to use the new version")
		return
	}
	slog.Info("update applied - restarting service...")
	if err := exec.Command("systemctl", "restart", "probakgo").Run(); err != nil {
		slog.Warn("systemctl restart failed - restart manually", "err", err)
	}
}

func ensureDefaults(st *store.Store) error {
	ctx := context.Background()
	hasUsers, err := st.HasUsers(ctx)
	if err != nil {
		return err
	}
	if !hasUsers {
		pass := randomPassword()
		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		if _, err := st.CreateUser(ctx, "probakgo", string(hash), "admin"); err != nil {
			return err
		}
		slog.Warn("⚠  default user created - CHANGE PASSWORD IMMEDIATELY",
			"username", "probakgo", "password", pass)
	}
	return nil
}

func unlock2FA(username string) error {
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		return err
	}
	db, err := dbpkg.Open(cfg.DBPath)
	if err != nil {
		return err
	}
	defer db.Close()
	ok, err := store.New(db).DisableUserTOTPByUsername(context.Background(), username)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	return nil
}

func randomPassword() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "changeme-restart-server"
	}
	for i, v := range b {
		b[i] = chars[v%byte(len(chars))]
	}
	return string(b)
}

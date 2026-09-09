package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"embed"
	"encoding/hex"
	"fmt"
	"io/fs"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/api"
	"probakgo/internal/config"
	dbpkg "probakgo/internal/db"
	"probakgo/internal/schedule"
	"probakgo/internal/selfupdate"
	"probakgo/internal/service"
	"probakgo/internal/session"
	"probakgo/internal/store"
	appversion "probakgo/internal/version"
	"probakgo/internal/web"
)

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
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "version":
			fmt.Printf("probakgo v%s\n", appversion.Version)
			return
		case "update":
			updated, err := selfupdate.Run("Nestorm18/probakgo", "probakgo", appversion.Version)
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
		case "initial-password":
			pass, err := consumeInitialPassword(initialPasswordPath())
			if err != nil {
				fmt.Fprintln(os.Stderr, "initial-password:", err)
				os.Exit(1)
			}
			fmt.Println(pass)
			return
		}
	}

	ensureSessionKey()
	if err := ensureDataEncryptionKey(); err != nil {
		slog.Error("DATA_ENCRYPTION_KEY could not be persisted", "err", err)
		os.Exit(1)
	}

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

	st, err := newStore(db, cfg)
	if err != nil {
		slog.Error("initialize encrypted store", "err", err)
		os.Exit(1)
	}
	if err := st.ProtectLegacySecrets(context.Background()); err != nil {
		slog.Error("protect legacy secrets", "err", err)
		os.Exit(1)
	}

	if err := ensureDefaults(st); err != nil {
		slog.Error("bootstrap defaults", "err", err)
		os.Exit(1)
	}

	session.Init(cfg.SessionKey, cfg.SecureSession)

	authSvc := service.NewAuth(st)
	reportSvc := service.NewReport(st, loc)

	// Web Push (PWA) sender. It boots lazily on the first subscription, but
	// we register it eagerly so the alert engine can fan critical alerts out
	// to subscribed browsers in parallel with email.
	pushSender := service.NewPushSender(st)
	service.SetPushSender(pushSender)
	telegramSender := service.NewTelegramSender(st)
	service.SetTelegramSender(telegramSender)

	// Static sub-FS so /static/... maps to web/static/...
	staticSub, err := fs.Sub(webFS, "web/static")
	if err != nil {
		slog.Error("static sub-fs", "err", err)
		os.Exit(1)
	}

	apiSrv := api.NewServer(st, authSvc, reportSvc, cfg.TrustedProxies)
	webRouter, err := web.NewRouter(st, reportSvc, webFS, staticSub, cfg.SessionKey, cfg.SecureSession, cfg.TrustedOrigins, cfg.TrustedProxies, appversion.Version, cfg.Dev, loc)
	if err != nil {
		slog.Error("build web router", "err", err)
		os.Exit(1)
	}

	appCtx, appCancel := context.WithCancel(context.Background())
	defer appCancel()
	service.StartEmailScheduler(appCtx, st, reportSvc)
	service.StartCleanupScheduler(appCtx, st)
	service.StartNASBackupScheduler(appCtx, st)
	service.StartAlertScheduler(appCtx, st, reportSvc)

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

	ensureSystemdService()
	ensureUpdateCron()
	slog.Info("probakgo started", "addr", "http://"+addr, "version", appversion.Version)

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

func ensureDataEncryptionKey() error {
	if os.Getenv("DATA_ENCRYPTION_KEY") != "" {
		return nil
	}
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return fmt.Errorf("generate DATA_ENCRYPTION_KEY: %w", err)
	}
	key := hex.EncodeToString(b)
	f, err := os.OpenFile(".env", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(f, "DATA_ENCRYPTION_KEY=%s\n", key); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Setenv("DATA_ENCRYPTION_KEY", key); err != nil {
		return err
	}
	slog.Info("DATA_ENCRYPTION_KEY generated and saved to .env")
	return nil
}

// ensureSystemdService installs or refreshes the hardened systemd service when
// running as root. The standard /opt/probakgo deployment uses a dedicated user.
func ensureSystemdService() {
	if os.Getuid() != 0 {
		return
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	workDir := filepath.Dir(exe)

	serviceUser := "root"
	if filepath.Clean(workDir) == "/opt/probakgo" {
		if err := ensureServiceAccount(workDir); err != nil {
			slog.Warn("could not configure dedicated systemd user; keeping root service", "err", err)
		} else {
			serviceUser = "probakgo"
		}
	}
	content := systemdServiceContent(exe, workDir, serviceUser)
	if existing, err := os.ReadFile(serverServicePath); err == nil && string(existing) == content {
		return
	}
	if err := os.WriteFile(serverServicePath, []byte(content), 0644); err != nil {
		slog.Warn("could not install systemd service", "err", err)
		return
	}
	_ = exec.Command("systemctl", "daemon-reload").Run()
	_ = exec.Command("systemctl", "enable", "probakgo").Run()
	slog.Info("systemd service installed and enabled", "path", serverServicePath, "user", serviceUser)
}

func systemdServiceContent(exe, workDir, serviceUser string) string {
	return fmt.Sprintf(`[Unit]
Description=probakgo Proxmox Monitor
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=%s
ExecStart="%s"
User=%s
Group=%s
UMask=0077
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateDevices=true
PrivateTmp=true
ProtectClock=true
ProtectControlGroups=true
ProtectHome=true
ProtectHostname=true
ProtectKernelLogs=true
ProtectKernelModules=true
ProtectKernelTunables=true
ProtectSystem=strict
ReadWritePaths=%s
RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6
RestrictNamespaces=true
RestrictRealtime=true
RestrictSUIDSGID=true
LockPersonality=true
MemoryDenyWriteExecute=true
SystemCallArchitectures=native

[Install]
WantedBy=multi-user.target
`, workDir, exe, serviceUser, serviceUser, workDir)
}

func ensureServiceAccount(workDir string) error {
	account, err := user.Lookup("probakgo")
	if err != nil {
		useradd, lookupErr := exec.LookPath("useradd")
		if lookupErr != nil {
			return lookupErr
		}
		output, addErr := exec.Command(useradd,
			"--system",
			"--home-dir", workDir,
			"--no-create-home",
			"--shell", "/usr/sbin/nologin",
			"probakgo",
		).CombinedOutput()
		if addErr != nil {
			return fmt.Errorf("create probakgo user: %w: %s", addErr, strings.TrimSpace(string(output)))
		}
		account, err = user.Lookup("probakgo")
		if err != nil {
			return err
		}
	}
	uid, err := strconv.Atoi(account.Uid)
	if err != nil {
		return fmt.Errorf("parse probakgo uid: %w", err)
	}
	gid, err := strconv.Atoi(account.Gid)
	if err != nil {
		return fmt.Errorf("parse probakgo gid: %w", err)
	}
	return filepath.WalkDir(workDir, func(path string, _ fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		return os.Chown(path, uid, gid)
	})
}

// ensureUpdateCron writes /etc/cron.d/probakgo on first startup when running as root.
func ensureUpdateCron() {
	if os.Getuid() != 0 {
		return
	}
	exe, err := os.Executable()
	if err != nil {
		return
	}
	exe, _ = filepath.EvalSymlinks(exe)
	workDir := filepath.Dir(exe)
	minute := schedule.DailyMinute(schedule.HostSeed() + ":probakgo-server-update")
	cronUser := "root"
	if filepath.Clean(workDir) == "/opt/probakgo" {
		if _, err := user.Lookup("probakgo"); err == nil {
			cronUser = "probakgo"
		}
	}
	content := fmt.Sprintf("%d 1 * * * %s cd \"%s\" && \"%s\" update >> \"%s/probakgo-update.log\" 2>&1\n", minute, cronUser, workDir, exe, workDir)
	if existing, err := os.ReadFile(serverCronPath); err == nil && string(existing) == content {
		return
	}
	if err := os.WriteFile(serverCronPath, []byte(content), 0644); err != nil {
		slog.Warn("could not install update cron", "err", err)
	} else {
		slog.Info("auto-update cron installed", "path", serverCronPath, "schedule", fmt.Sprintf("01:%02d daily", minute), "user", cronUser)
	}
}

// restartService attempts to restart the probakgo systemd service after an update.
func restartService() {
	if _, err := exec.LookPath("systemctl"); err != nil {
		slog.Info("update applied - restart the service manually to use the new version")
		return
	}
	if os.Geteuid() != 0 {
		if err := signalSystemdMainProcess(); err != nil {
			slog.Warn("could not signal systemd service - restart manually", "err", err)
		} else {
			slog.Info("systemd service signalled for restart")
		}
		return
	}
	slog.Info("update applied - restarting service...")
	if err := exec.Command("systemctl", "restart", "probakgo").Run(); err != nil {
		if signalErr := signalSystemdMainProcess(); signalErr == nil {
			slog.Info("systemd service signalled for restart")
			return
		}
		slog.Warn("systemctl restart failed - restart manually", "err", err)
	}
}

func signalSystemdMainProcess() error {
	output, err := exec.Command("systemctl", "show", "--property=MainPID", "--value", "probakgo").Output()
	if err != nil {
		return err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
	if err != nil || pid <= 1 || pid == os.Getpid() {
		return fmt.Errorf("invalid probakgo MainPID")
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGTERM)
}

func ensureDefaults(st *store.Store) error {
	ctx := context.Background()
	hasUsers, err := st.HasUsers(ctx)
	if err != nil {
		return err
	}
	if !hasUsers {
		pass, err := randomPassword()
		if err != nil {
			return err
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		passwordPath := initialPasswordPath()
		if err := writeInitialPassword(passwordPath, pass); err != nil {
			return fmt.Errorf("store initial admin password: %w", err)
		}
		if _, err := st.CreateUser(ctx, "probakgo", string(hash), "admin"); err != nil {
			_ = os.Remove(passwordPath)
			return err
		}
		slog.Warn("⚠  default user created - CHANGE PASSWORD IMMEDIATELY",
			"username", "probakgo",
			"password_file", passwordPath,
			"retrieve_command", "probakgo initial-password")
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
	st, err := newStore(db, cfg)
	if err != nil {
		return err
	}
	ok, err := st.DisableUserTOTPByUsername(context.Background(), username)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("user %q not found", username)
	}
	return nil
}

func newStore(db *sql.DB, cfg *config.Config) (*store.Store, error) {
	if cfg.DataKey == "" {
		return store.New(db), nil
	}
	return store.NewEncrypted(db, cfg.DataKey)
}

func randomPassword() (string, error) {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 16)
	limit := big.NewInt(int64(len(chars)))
	for i := range b {
		n, err := rand.Int(rand.Reader, limit)
		if err != nil {
			return "", fmt.Errorf("generate random password: %w", err)
		}
		b[i] = chars[n.Int64()]
	}
	return string(b), nil
}

func initialPasswordPath() string {
	exe, err := os.Executable()
	if err != nil {
		return ".initial-admin-password"
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return ".initial-admin-password"
	}
	return filepath.Join(filepath.Dir(exe), ".initial-admin-password")
}

func writeInitialPassword(path, password string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintln(f, password); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

func consumeInitialPassword(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	password := strings.TrimSpace(string(data))
	if password == "" {
		return "", fmt.Errorf("%s is empty", path)
	}
	if err := os.Remove(path); err != nil {
		return "", fmt.Errorf("remove %s after reading: %w", path, err)
	}
	return password, nil
}

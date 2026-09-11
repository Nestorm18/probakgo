package service

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/joho/godotenv"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/store"
)

func TestNASBackupSchedule(t *testing.T) {
	zone := time.FixedZone("server", 2*60*60)
	now := time.Date(2026, 9, 9, 3, 0, 0, 0, zone)
	for _, tc := range []struct {
		name, last, hour string
		enabled, want    bool
	}{
		{"first", "", "03:00", true, true},
		{"before", "", "03:01", true, false},
		{"disabled", "", "03:00", false, false},
		{"restart", "2026-09-09T00:59:00Z", "03:00", true, false},
		{"yesterday", "2026-09-08T00:59:00Z", "03:00", true, true},
		{"invalid", "", "invalid", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := nasBackupDue(domain.NASBackupConfig{Enabled: tc.enabled, SendTime: tc.hour, LastScheduledAttempt: tc.last}, now); got != tc.want {
				t.Fatalf("due=%v", got)
			}
		})
	}
}

func testNASServer(t *testing.T) domain.NASBackupConfig {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ssh.ServerConfig{PasswordCallback: func(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if c.User() != "backup" || string(password) != "secret" {
			return nil, errors.New("credentials")
		}
		return nil, nil
	}}
	cfg.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	fs := sftp.InMemHandler()
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go func() {
				defer conn.Close()
				server, channels, requests, err := ssh.NewServerConn(conn, cfg)
				if err != nil {
					return
				}
				defer server.Close()
				go ssh.DiscardRequests(requests)
				for channel := range channels {
					if channel.ChannelType() != "session" {
						channel.Reject(ssh.UnknownChannelType, "session required")
						continue
					}
					ch, reqs, err := channel.Accept()
					if err != nil {
						return
					}
					go func() {
						defer ch.Close()
						for req := range reqs {
							var subsystem struct{ Name string }
							ok := req.Type == "subsystem" && ssh.Unmarshal(req.Payload, &subsystem) == nil && subsystem.Name == "sftp"
							req.Reply(ok, nil)
							if ok {
								srv := sftp.NewRequestServer(ch, fs)
								defer srv.Close()
								srv.Serve()
								return
							}
						}
					}()
				}
			}()
		}
	}()
	host, port, _ := net.SplitHostPort(listener.Addr().String())
	n, _ := strconv.Atoi(port)
	return domain.NASBackupConfig{Enabled: true, Host: host, Port: n, Username: "backup", Password: "secret", Directory: "/", SendTime: "03:00"}
}

func TestNASBackupSFTP(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATA_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_KEY", "test-session-key-32-bytes-long!!")
	c := testNASServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := TestNASBackup(ctx, c); err != nil {
		t.Fatal(err)
	}
	database, _ := openTestStore(t)
	st, err := store.NewEncrypted(database, os.Getenv("DATA_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SMTPPass: "restore-test-password"}); err != nil {
		t.Fatal(err)
	}
	oldArchive := "/probakgo_20000101_000000_" + strings.Repeat("a", 24) + ".zip"
	if err := withNASSFTP(ctx, c, func(ftp *sftp.Client) error {
		f, err := ftp.Create(oldArchive)
		if err != nil {
			return err
		}
		return f.Close()
	}); err != nil {
		t.Fatal(err)
	}
	invalid := c
	invalid.Password = "wrong"
	if err := uploadNASBackup(ctx, st, invalid); err == nil {
		t.Fatal("expected upload failure")
	}
	if err := withNASSFTP(ctx, c, func(ftp *sftp.Client) error {
		_, err := ftp.Stat(oldArchive)
		return err
	}); err != nil {
		t.Fatal("failed upload removed the existing backup", err)
	}
	if err := uploadNASBackup(ctx, st, c); err != nil {
		t.Fatal(err)
	}
	if err := withNASSFTP(ctx, c, func(ftp *sftp.Client) error {
		files, err := ftp.ReadDir("/")
		if err != nil {
			return err
		}
		if len(files) != 1 || !strings.HasSuffix(files[0].Name(), ".zip") {
			t.Fatalf("unexpected files: %v", files)
		}
		f, err := ftp.Open("/" + files[0].Name())
		if err != nil {
			return err
		}
		defer f.Close()
		data, err := io.ReadAll(f)
		if err != nil {
			return err
		}
		archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return err
		}
		contents := make(map[string][]byte)
		for _, entry := range archive.File {
			if entry.Mode().Perm() != 0600 {
				t.Fatalf("unsafe archive mode: %s", entry.Name)
			}
			r, err := entry.Open()
			if err != nil {
				return err
			}
			contents[entry.Name], err = io.ReadAll(r)
			r.Close()
			if err != nil {
				return err
			}
		}
		if len(contents) != 2 || !bytes.HasPrefix(contents["probakgo_data.db"], []byte("SQLite format 3\x00")) {
			t.Fatal("missing database in backup")
		}
		env, err := godotenv.Unmarshal(string(contents[".env"]))
		if err != nil || env["DATA_ENCRYPTION_KEY"] != os.Getenv("DATA_ENCRYPTION_KEY") || env["SESSION_KEY"] != os.Getenv("SESSION_KEY") {
			t.Fatal("missing restoration keys")
		}
		restoredPath := filepath.Join(t.TempDir(), "restored.db")
		if err := os.WriteFile(restoredPath, contents["probakgo_data.db"], 0600); err != nil {
			return err
		}
		restored, err := dbpkg.Open(restoredPath)
		if err != nil {
			return err
		}
		defer restored.Close()
		restoredStore, err := store.NewEncrypted(restored, env["DATA_ENCRYPTION_KEY"])
		if err != nil {
			return err
		}
		cfg, err := restoredStore.GetEmailConfig(ctx)
		if err != nil || cfg.SMTPPass != "restore-test-password" {
			t.Fatal("restored database secrets cannot be decrypted", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	c.Password = "incorrect-password"
	if err := TestNASBackup(ctx, c); err == nil {
		t.Fatal("accepted incorrect password")
	}
}

func TestNASRetentionOnlyRemovesExpiredArchives(t *testing.T) {
	c := testNASServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	now := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	id := strings.Repeat("a", 24)
	name := func(date string) string { return "probakgo_" + date + "_" + id + ".zip" }
	old := name("20260901_120000")
	keep := []string{name("20260902_120000"), name("20260909_120000"), name("20260910_120000"), "personal.zip", "probakgo_20260901_120000_invalid.zip", name("20260230_120000"), old + ".partial", "probakgo_data.db"}
	err := withNASSFTP(ctx, c, func(ftp *sftp.Client) error {
		for _, filename := range append([]string{old}, keep...) {
			f, err := ftp.Create("/" + filename)
			if err != nil {
				return err
			}
			if err := f.Close(); err != nil {
				return err
			}
		}
		directory := name("20260801_120000")
		if err := ftp.Mkdir("/" + directory); err != nil {
			return err
		}
		if err := pruneNASBackups(ftp, "/", now); err != nil {
			return err
		}
		if _, err := ftp.Stat("/" + old); !os.IsNotExist(err) {
			t.Fatal("expired archive remains", err)
		}
		for _, filename := range append(keep, directory) {
			if _, err := ftp.Stat("/" + filename); err != nil {
				t.Fatalf("deleted retained file %s: %v", filename, err)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestManualNASBackupUsesSavedSettingsWhileDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATA_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_KEY", "test-session-key-32-bytes-long!!")
	c := testNASServer(t)
	c.Enabled = false
	database, _ := openTestStore(t)
	st, err := store.NewEncrypted(database, os.Getenv("DATA_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.SaveNASBackupConfig(ctx, c); err != nil {
		t.Fatal(err)
	}
	nasBackupLock.Lock()
	err = StartManualNASBackup(ctx, st)
	nasBackupLock.Unlock()
	if err == nil {
		t.Fatal("accepted overlapping backup")
	}
	if err := StartManualNASBackup(ctx, st); err != nil {
		t.Fatal(err)
	}
	// The job holds this lock until upload and status persistence have finished.
	nasBackupLock.Lock()
	nasBackupLock.Unlock()
	got, err := st.GetNASBackupConfig(ctx)
	if err != nil || got.LastSuccess == "" || got.LastError != "" || got.Enabled {
		t.Fatalf("manual backup result: %+v, %v", got, err)
	}
}

func TestManualBackupDoesNotSkipEveningSchedule(t *testing.T) {
	zone := time.FixedZone("Madrid", 2*60*60)
	c := domain.NASBackupConfig{Enabled: true, SendTime: "22:00", LastAttempt: "2026-09-09T17:16:46+02:00"}
	now := time.Date(2026, 9, 9, 22, 0, 0, 0, zone)
	if !nasBackupDue(c, now) {
		t.Fatal("manual copy suppressed scheduled backup")
	}
	c.LastScheduledAttempt = now.Format(time.RFC3339)
	if nasBackupDue(c, now.Add(time.Minute)) {
		t.Fatal("scheduled copy repeated")
	}
}

func TestScheduledNASBackupAfterManualCopy(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Setenv("DATA_ENCRYPTION_KEY", "0123456789abcdef0123456789abcdef")
	t.Setenv("SESSION_KEY", "test-session-key-32-bytes-long!!")
	c := testNASServer(t)
	c.Enabled, c.SendTime = true, "00:00"
	database, _ := openTestStore(t)
	st, err := store.NewEncrypted(database, os.Getenv("DATA_ENCRYPTION_KEY"))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := st.SaveNASBackupConfig(ctx, c); err != nil {
		t.Fatal(err)
	}
	if err := StartManualNASBackup(ctx, st); err != nil {
		t.Fatal(err)
	}
	nasBackupLock.Lock()
	nasBackupLock.Unlock()
	before, err := st.GetNASBackupConfig(ctx)
	if err != nil || before.LastSuccess == "" || before.LastScheduledAttempt != "" {
		t.Fatal("manual copy failed or consumed schedule", err)
	}
	runNASBackup(ctx, st, time.UTC)
	after, err := st.GetNASBackupConfig(ctx)
	if err != nil || after.LastScheduledAttempt == "" || after.LastSuccess != after.LastScheduledAttempt || after.LastError != "" {
		t.Fatal("scheduled upload failed after manual copy", err)
	}
	runNASBackup(ctx, st, time.UTC)
	again, err := st.GetNASBackupConfig(ctx)
	if err != nil || again.LastScheduledAttempt != after.LastScheduledAttempt {
		t.Fatal("scheduled upload repeated", err)
	}
}

package service

import (
	"archive/zip"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"probakgo/internal/config"
	"probakgo/internal/domain"
	"probakgo/internal/store"
)

var nasBackupLock sync.Mutex
var nasArchiveName = regexp.MustCompile(`^probakgo_(\d{8}_\d{6})_[0-9a-f]{24}\.zip$`)

type nasRetentionError struct{}

func (*nasRetentionError) Error() string {
	return "Copia completada, pero no se pudieron eliminar todas las copias de más de 7 días. Revisa los permisos del NAS."
}

// Only complete archives named by Probakgo are eligible; never follow links.
func pruneNASBackups(ftp *sftp.Client, directory string, now time.Time) error {
	files, err := ftp.ReadDir(directory)
	if err != nil {
		return &nasRetentionError{}
	}
	cutoff := now.UTC().Add(-7 * 24 * time.Hour)
	failed := false
	for _, file := range files {
		if !file.Mode().IsRegular() {
			continue
		}
		match := nasArchiveName.FindStringSubmatch(file.Name())
		if match == nil {
			continue
		}
		created, err := time.Parse("20060102_150405", match[1])
		if err != nil || !created.Before(cutoff) {
			continue
		}
		if err := ftp.Remove(path.Join(directory, file.Name())); err != nil {
			failed = true
		}
	}
	if failed {
		return &nasRetentionError{}
	}
	return nil
}

func ValidateNASBackupConfig(c domain.NASBackupConfig) error {
	if c.Host == "" || strings.ContainsAny(c.Host, " /\\\r\n\t") || c.Port < 1 || c.Port > 65535 || c.Username == "" || c.Password == "" {
		return errors.New("indica servidor, puerto válido, usuario y contraseña SFTP")
	}
	if !path.IsAbs(c.Directory) || strings.ContainsAny(c.Directory, "\x00\r\n") {
		return errors.New("indica una carpeta remota absoluta, por ejemplo /backups/probakgo")
	}
	if _, err := time.Parse("15:04", c.SendTime); err != nil {
		return errors.New("indica una hora válida HH:MM")
	}
	return nil
}

func withNASSFTP(ctx context.Context, c domain.NASBackupConfig, run func(*sftp.Client) error) error {
	if err := ValidateNASBackupConfig(c); err != nil {
		return err
	}
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	conn, err := (&net.Dialer{Timeout: 15 * time.Second}).DialContext(ctx, "tcp", addr)
	if err != nil {
		return errors.New("no se pudo conectar al NAS")
	}
	defer conn.Close()
	stop := context.AfterFunc(ctx, func() { conn.Close() })
	defer stop()
	deadline := time.Now().Add(10 * time.Minute)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	if err := conn.SetDeadline(deadline); err != nil {
		return errors.New("no se pudo preparar la conexión al NAS")
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, &ssh.ClientConfig{
		User: c.Username, Auth: []ssh.AuthMethod{ssh.Password(c.Password)},
		// Host-key verification is disabled by the requested NAS configuration policy.
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	})
	if err != nil {
		return errors.New("conexión SSH rechazada: revisa usuario y contraseña del NAS")
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()
	ftp, err := sftp.NewClient(client)
	if err != nil {
		return errors.New("no se pudo iniciar SFTP en el NAS")
	}
	defer ftp.Close()
	return run(ftp)
}

func nasFileID() (string, error) {
	var b [12]byte
	_, err := rand.Read(b[:])
	return hex.EncodeToString(b[:]), err
}

// TestNASBackup checks creation and removal in the configured directory.
func TestNASBackup(ctx context.Context, c domain.NASBackupConfig) error {
	return withNASSFTP(ctx, c, func(ftp *sftp.Client) error {
		id, err := nasFileID()
		if err != nil {
			return err
		}
		name := path.Join(c.Directory, ".probakgo-test-"+id)
		f, err := ftp.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return errors.New("no se puede escribir en la carpeta del NAS; comprueba que existe y sus permisos")
		}
		closeErr := f.Close()
		removeErr := ftp.Remove(name)
		if closeErr != nil || removeErr != nil {
			return errors.New("no se pudo cerrar o eliminar el archivo de prueba del NAS")
		}
		return nil
	})
}

func uploadNASBackup(ctx context.Context, st *store.Store, c domain.NASBackupConfig) error {
	env, err := config.BackupEnv()
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "probakgo-nas-")
	if err != nil {
		return errors.New("no se pudo crear la carpeta temporal")
	}
	defer os.RemoveAll(dir)
	local := filepath.Join(dir, "backup.db")
	if err := st.BackupTo(ctx, local); err != nil {
		return errors.New("no se pudo generar la copia de SQLite")
	}
	archive := filepath.Join(dir, "backup.zip")
	if err := writeNASBackupArchive(archive, local, env); err != nil {
		return err
	}
	f, err := os.Open(archive)
	if err != nil {
		return errors.New("no se pudo leer la copia temporal")
	}
	defer f.Close()
	return withNASSFTP(ctx, c, func(ftp *sftp.Client) error {
		id, err := nasFileID()
		if err != nil {
			return err
		}
		name := path.Join(c.Directory, "probakgo_"+time.Now().UTC().Format("20060102_150405")+"_"+id+".zip")
		tmp := name + ".partial"
		out, err := ftp.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL)
		if err != nil {
			return errors.New("no se pudo crear la copia en la carpeta del NAS")
		}
		defer ftp.Remove(tmp)
		if err := ftp.Chmod(tmp, 0600); err != nil {
			out.Close()
			return errors.New("no se pudieron restringir los permisos de la copia en el NAS")
		}
		_, copyErr := io.Copy(out, f)
		closeErr := out.Close()
		if copyErr != nil || closeErr != nil {
			return errors.New("transferencia SFTP incompleta")
		}
		if err := ftp.Rename(tmp, name); err != nil {
			return errors.New("no se pudo finalizar el archivo de copia en el NAS")
		}
		return pruneNASBackups(ftp, c.Directory, time.Now())
	})
}

func writeNASBackupArchive(destination, database string, env []byte) error {
	f, err := os.OpenFile(destination, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("no se pudo crear el archivo de copia")
	}
	defer f.Close()
	z := zip.NewWriter(f)
	defer z.Close()
	add := func(name string, source io.Reader) error {
		header := &zip.FileHeader{Name: name, Method: zip.Deflate}
		header.SetMode(0600)
		entry, err := z.CreateHeader(header)
		if err != nil {
			return err
		}
		_, err = io.Copy(entry, source)
		return err
	}
	db, err := os.Open(database)
	if err != nil {
		return errors.New("no se pudo leer la base de datos de la copia")
	}
	defer db.Close()
	if err := add("probakgo_data.db", db); err != nil {
		return errors.New("no se pudo incluir la base de datos en la copia")
	}
	if err := add(".env", strings.NewReader(string(env))); err != nil {
		return errors.New("no se pudo incluir el .env en la copia")
	}
	if err := z.Close(); err != nil {
		return errors.New("no se pudo completar el archivo de copia")
	}
	if err := f.Close(); err != nil {
		return errors.New("no se pudo guardar el archivo de copia")
	}
	return nil
}

func nasBackupDue(c domain.NASBackupConfig, now time.Time) bool {
	if !c.Enabled {
		return false
	}
	t, err := time.Parse("15:04", c.SendTime)
	if err != nil {
		return false
	}
	due := time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location())
	last, err := time.Parse(time.RFC3339, c.LastScheduledAttempt)
	return !now.Before(due) && (err != nil || last.In(now.Location()).Format("2006-01-02") < now.Format("2006-01-02"))
}

func runNASBackup(parent context.Context, st *store.Store, loc *time.Location) {
	if !nasBackupLock.TryLock() {
		return
	}
	defer nasBackupLock.Unlock()
	ctx, cancel := context.WithTimeout(parent, 10*time.Minute)
	defer cancel()
	c, err := st.GetNASBackupConfig(ctx)
	if err != nil {
		slog.Error("NAS backup: load config", "err", err)
		return
	}
	now := time.Now().In(loc)
	if !nasBackupDue(*c, now) {
		return
	}
	attempt := now.Format(time.RFC3339)
	claimed, err := st.ClaimNASBackup(ctx, c.LastAttempt, attempt)
	if err != nil {
		slog.Error("NAS backup: claim", "err", err)
		return
	}
	if !claimed {
		return
	}
	performNASBackup(ctx, st, *c, attempt)
}

// Manual copies use saved settings and run outside the HTTP request timeout.
func StartManualNASBackup(ctx context.Context, st *store.Store) error {
	if !nasBackupLock.TryLock() {
		return errors.New("ya hay una copia al NAS en curso")
	}
	started := false
	defer func() {
		if !started {
			nasBackupLock.Unlock()
		}
	}()
	c, err := st.GetNASBackupConfig(ctx)
	if err != nil {
		return errors.New("no se pudo cargar la configuración del NAS")
	}
	if err := ValidateNASBackupConfig(*c); err != nil {
		return errors.New("guarda primero una configuración válida del NAS")
	}
	attempt := time.Now().Format(time.RFC3339Nano)
	claimed, err := st.ClaimManualNASBackup(ctx, c.LastAttempt, attempt)
	if err != nil || !claimed {
		return errors.New("no se pudo iniciar la copia al NAS")
	}
	started = true
	go func() {
		defer nasBackupLock.Unlock()
		jobCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		performNASBackup(jobCtx, st, *c, attempt)
	}()
	return nil
}

func performNASBackup(ctx context.Context, st *store.Store, c domain.NASBackupConfig, attempt string) {
	failure := ""
	completed := true
	if err := uploadNASBackup(ctx, st, c); err != nil {
		var retentionErr *nasRetentionError
		completed = errors.As(err, &retentionErr)
		failure = err.Error()
		slog.Error("NAS backup failed", "err", err)
	}
	statusCtx, statusCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer statusCancel()
	if err := st.FinishNASBackupResult(statusCtx, attempt, completed, failure); err != nil {
		slog.Error("NAS backup: save result", "err", err)
	}
}

// Daily in the configured application timezone. A missed run is caught up at startup.
func StartNASBackupScheduler(ctx context.Context, st *store.Store, loc *time.Location) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			runNASBackup(ctx, st, loc)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

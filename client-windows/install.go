package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"
)

const (
	reportTaskName = "Probakgo Windows Report"
	updateTaskName = "Probakgo Windows Update"
)

func installWindowsClient(cfg Config) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("install is only supported on windows")
	}
	dir := installDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := restrictWindowsInstallDir(dir); err != nil {
		return err
	}
	exePath := filepath.Join(dir, "probakgo-windows-client.exe")
	self, err := os.Executable()
	if err != nil {
		return err
	}
	endScheduledTask(reportTaskName)
	endScheduledTask(updateTaskName)
	if err := replaceInstalledBinary(self, exePath); err != nil {
		return err
	}
	env := fmt.Sprintf("API_URL=%s\nAPI_KEY=%s\n", cfg.APIURL, cfg.APIKey)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0600); err != nil {
		return err
	}
	if err := createReportScheduledTask(exePath); err != nil {
		return err
	}
	if err := createUpdateScheduledTask(exePath); err != nil {
		return err
	}
	fmt.Println("Directories ready:", dir)
	fmt.Println("Binary installed:", exePath)
	fmt.Println(".env written:", filepath.Join(dir, ".env"))
	fmt.Println("Log:", logPath())
	fmt.Println("Scheduled task installed: Probakgo Windows Report (every 5 min)")
	fmt.Println("Scheduled task installed: Probakgo Windows Update (daily at 04:17)")
	fmt.Println("Test:", exePath)
	return nil
}

func createReportScheduledTask(exePath string) error {
	taskCmd := fmt.Sprintf(`"%s"`, exePath)
	cmd := exec.Command("schtasks.exe",
		"/Create",
		"/TN", reportTaskName,
		"/TR", taskCmd,
		"/SC", "MINUTE",
		"/MO", "5",
		"/RU", "SYSTEM",
		"/F",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create report scheduled task: %w: %s", err, string(out))
	}
	return nil
}

func createUpdateScheduledTask(exePath string) error {
	taskCmd := fmt.Sprintf(`"%s" update`, exePath)
	cmd := exec.Command("schtasks.exe",
		"/Create",
		"/TN", updateTaskName,
		"/TR", taskCmd,
		"/SC", "DAILY",
		"/ST", "04:17",
		"/RU", "SYSTEM",
		"/RL", "HIGHEST",
		"/F",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create update scheduled task: %w: %s", err, string(out))
	}
	return nil
}

func ensureUpdateScheduledTask() error {
	if runtime.GOOS != "windows" {
		return nil
	}
	if err := checkScheduledTask(updateTaskName); err == nil {
		return nil
	}
	return createUpdateScheduledTask(filepath.Join(installDir(), "probakgo-windows-client.exe"))
}

func restrictWindowsInstallDir(dir string) error {
	cmd := exec.Command("icacls.exe", dir,
		"/inheritance:r",
		"/grant:r", "*S-1-5-18:(OI)(CI)F",
		"/grant:r", "*S-1-5-32-544:(OI)(CI)F",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("restrict %s permissions: %w: %s", dir, err, string(out))
	}
	return nil
}

func checkScheduledTask(taskName string) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("only available on windows")
	}
	cmd := exec.Command("schtasks.exe", "/Query", "/TN", taskName)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("query %s: %w: %s", taskName, err, string(out))
	}
	return nil
}

func endScheduledTask(taskName string) {
	if runtime.GOOS != "windows" {
		return
	}
	_ = exec.Command("schtasks.exe", "/End", "/TN", taskName).Run()
}

func replaceInstalledBinary(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat installer binary: %w", err)
	}
	if dstInfo, statErr := os.Stat(dst); statErr == nil && os.SameFile(srcInfo, dstInfo) {
		return nil
	}

	staged := dst + ".install-new"
	backup := dst + ".install-old"
	_ = os.Remove(staged)
	if err := copyFile(src, staged); err != nil {
		return fmt.Errorf("stage client binary: %w", err)
	}
	defer os.Remove(staged)

	if err := os.Remove(backup); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove previous client backup: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt < 60; attempt++ {
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			if err := os.Rename(staged, dst); err != nil {
				return fmt.Errorf("install client binary: %w", err)
			}
			return nil
		}

		if err := os.Rename(dst, backup); err != nil {
			lastErr = err
			time.Sleep(500 * time.Millisecond)
			continue
		}
		if err := os.Rename(staged, dst); err != nil {
			_ = os.Rename(backup, dst)
			return fmt.Errorf("activate client binary: %w", err)
		}
		_ = os.Remove(backup)
		return nil
	}
	return fmt.Errorf("replace installed client after waiting 30s: %w", lastErr)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}

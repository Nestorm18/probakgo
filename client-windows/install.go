package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

func installWindowsClient(cfg Config) error {
	if runtime.GOOS != "windows" {
		return fmt.Errorf("install is only supported on windows")
	}
	dir := installDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	exePath := filepath.Join(dir, "probakgo-windows-client.exe")
	self, err := os.Executable()
	if err != nil {
		return err
	}
	if err := copyFile(self, exePath); err != nil {
		return err
	}
	env := fmt.Sprintf("API_URL=%s\nAPI_KEY=%s\n", cfg.APIURL, cfg.APIKey)
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(env), 0600); err != nil {
		return err
	}
	taskCmd := fmt.Sprintf(`"%s"`, exePath)
	cmd := exec.Command("schtasks.exe",
		"/Create",
		"/TN", "Probakgo Windows Report",
		"/TR", taskCmd,
		"/SC", "MINUTE",
		"/MO", "5",
		"/RU", "SYSTEM",
		"/F",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("create scheduled task: %w: %s", err, string(out))
	}
	fmt.Println("Directories ready:", dir)
	fmt.Println("Binary installed:", exePath)
	fmt.Println(".env written:", filepath.Join(dir, ".env"))
	fmt.Println("Log:", logPath())
	fmt.Println("Scheduled task installed: Probakgo Windows Report (every 5 min)")
	fmt.Println("Test:", exePath)
	return nil
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

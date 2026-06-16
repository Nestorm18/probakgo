package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"probakgo/internal/config"
	dbpkg "probakgo/internal/db"
	"probakgo/internal/netutil"
	"probakgo/internal/store"
)

type doctorResult struct {
	level   string
	subject string
	message string
}

func runServerDoctor() error {
	ctx := context.Background()
	var results []doctorResult

	add := func(level, subject, message string) {
		results = append(results, doctorResult{level: level, subject: subject, message: message})
	}

	cfg := config.Load()
	add("OK", "Version", "probakgo v"+version)
	if err := cfg.Validate(); err != nil {
		add("FAIL", "Configuracion", err.Error())
	} else {
		add("OK", "Configuracion", "variables basicas validas")
	}

	if os.Getenv("SESSION_KEY") == "" {
		add("WARN", "SESSION_KEY", "no esta definida en el entorno; las sesiones pueden perderse al reiniciar")
	} else if len(cfg.SessionKey) >= 32 {
		add("OK", "SESSION_KEY", "definida")
	}

	checkListenAddress(add, cfg)

	dbPath, _ := filepath.Abs(cfg.DBPath)
	if _, err := os.Stat(cfg.DBPath); err != nil {
		add("WARN", "Base de datos", fmt.Sprintf("%s no existe todavia o no se puede leer: %v", dbPath, err))
	} else {
		add("OK", "Base de datos", dbPath)
	}

	db, err := dbpkg.Open(cfg.DBPath)
	if err != nil {
		add("FAIL", "Base de datos", "no se puede abrir o migrar: "+err.Error())
	} else {
		defer db.Close()
		add("OK", "Migraciones", "aplicadas correctamente")
		st := store.New(db)
		if users, err := st.ListUsers(ctx); err != nil {
			add("WARN", "Usuarios", "no se pudieron leer: "+err.Error())
		} else if len(users) == 0 {
			add("WARN", "Usuarios", "no hay usuarios creados")
		} else {
			activeAdmins := 0
			adminsWithoutTOTP := 0
			for _, u := range users {
				if u.Role == "admin" && u.IsActive {
					activeAdmins++
					if !u.TOTPEnabled {
						adminsWithoutTOTP++
					}
				}
			}
			if activeAdmins == 0 {
				add("FAIL", "Usuarios", "no hay ningun administrador activo")
			} else if adminsWithoutTOTP > 0 {
				add("WARN", "2FA", fmt.Sprintf("%d administrador(es) activo(s) sin 2FA", adminsWithoutTOTP))
			} else {
				add("OK", "2FA", "administradores activos con 2FA")
			}
		}

		if emailCfg, err := st.GetEmailConfig(ctx); err != nil {
			add("WARN", "URL publica", "no se pudo leer email_config: "+err.Error())
		} else {
			checkPublicAPIURL(add, emailCfg.PublicAPIURL)
			if emailCfg.SensitiveActionsRequireTOTP {
				add("OK", "Operaciones delicadas", "requieren 2FA")
			} else {
				add("WARN", "Operaciones delicadas", "no requieren 2FA")
			}
		}
	}

	if runtime.GOOS == "linux" {
		checkFile(add, "Servicio systemd", serverServicePath)
		checkFile(add, "Cron update", serverCronPath)
	}

	fail := false
	for _, r := range results {
		fmt.Printf("[%s] %s: %s\n", r.level, r.subject, r.message)
		if r.level == "FAIL" {
			fail = true
		}
	}
	if fail {
		return fmt.Errorf("doctor found failures")
	}
	fmt.Println("Doctor finished without critical failures.")
	return nil
}

func checkListenAddress(add func(string, string, string), cfg *config.Config) {
	if cfg.SecureSession {
		add("OK", "SESSION_SECURE", "true")
	} else {
		add("WARN", "SESSION_SECURE", "false; correcto solo si accedes por NetBird/VPN o HTTP local")
	}

	host := strings.Trim(cfg.APIHost, "[]")
	switch host {
	case "", "0.0.0.0", "::":
		add("WARN", "API_HOST", cfg.APIHost+" escucha en todas las interfaces; revisa firewall/router")
		return
	}
	if ip := net.ParseIP(host); ip != nil {
		if netutil.HostLooksPublic(host) {
			add("WARN", "API_HOST", host+" parece una IP publica; usa HTTPS o restringe por firewall")
			return
		}
		add("OK", "API_HOST", host+" parece interno")
		return
	}
	add("OK", "API_HOST", cfg.APIHost)
}

func checkPublicAPIURL(add func(string, string, string), raw string) {
	if strings.TrimSpace(raw) == "" {
		add("OK", "URL publica", "no configurada; se usa la URL del navegador")
		return
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		add("WARN", "URL publica", "valor no valido: "+raw)
		return
	}
	if u.Scheme == "http" && netutil.HostLooksPublic(u.Host) {
		add("WARN", "URL publica", "HTTP sobre host publico: "+raw)
		return
	}
	if u.Scheme == "http" {
		add("OK", "URL publica", "HTTP interno/VPN: "+raw)
		return
	}
	if u.Scheme == "https" {
		add("OK", "URL publica", "HTTPS: "+raw)
		return
	}
	add("WARN", "URL publica", "esquema no esperado: "+raw)
}

func checkFile(add func(string, string, string), subject, path string) {
	if _, err := os.Stat(path); err != nil {
		add("WARN", subject, path+" no existe")
		return
	}
	add("OK", subject, path)
}

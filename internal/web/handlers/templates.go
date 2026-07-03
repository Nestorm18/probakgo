package webhandlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/csrf"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
	"probakgo/internal/netutil"
)

var standaloneTemplates = map[string]bool{
	"login.html":     true,
	"login_2fa.html": true,
}

// templateActive maps template name → sidebar active key
var templateActive = map[string]string{
	"alerts.html":                "alerts-view",
	"alert_detail.html":          "alerts-view",
	"dashboard.html":             "dashboard",
	"servers_pve.html":           "pve",
	"server_pve_detail.html":     "pve",
	"backup_config.html":         "pve",
	"vm_backup_config_form.html": "pve",
	"servers_pbs.html":           "pbs",
	"server_pbs_detail.html":     "pbs",
	"servers_windows.html":       "windows",
	"server_windows_detail.html": "windows",
	"api_keys.html":              "keys",
	"api_key_created.html":       "keys",
	"users.html":                 "users",
	"settings_hub.html":          "settings",
	"system_settings.html":       "settings",
	"email_settings.html":        "settings",
	"maintenance_settings.html":  "settings",
	"alerts_settings.html":       "settings",
	"ip_bans.html":               "settings",
	"audit_log.html":             "settings",
	"reset_settings.html":        "settings",
	"profile.html":               "",
	"profile_2fa_setup.html":     "",
	"api_key_edit.html":          "keys",
	"reports_pve.html":           "pve",
	"about.html":                 "about",
}

type Templates struct {
	fs                   fs.FS
	funcMap              template.FuncMap
	loc                  *time.Location
	version              string
	secure               bool
	badgeCounts          func() (int, int)
	sensitiveTOTPEnabled func() bool
}

func NewTemplates(fs fs.FS, version string, loc *time.Location, secure bool, badgeCounts func() (int, int), sensitiveTOTPEnabled func() bool) *Templates {
	if loc == nil {
		loc = time.Local
	}
	return &Templates{
		fs:                   fs,
		funcMap:              makeFuncMap(loc),
		loc:                  loc,
		version:              version,
		secure:               secure,
		badgeCounts:          badgeCounts,
		sensitiveTOTPEnabled: sensitiveTOTPEnabled,
	}
}

func makeFuncMap(loc *time.Location) template.FuncMap {
	if loc == nil {
		loc = time.Local
	}
	return template.FuncMap{
		"formatTime": func(v any) string {
			switch t := v.(type) {
			case *time.Time:
				if t == nil {
					return "nunca"
				}
				return t.In(loc).Format("02 Jan 2006 15:04")
			case time.Time:
				return t.In(loc).Format("02 Jan 2006 15:04")
			default:
				return "–"
			}
		},
		"formatBytes": domain.FormatBytes,
		"pct": func(used, total int64) int {
			if total == 0 {
				return 0
			}
			return int(float64(used) / float64(total) * 100)
		},
		"formatDuration": func(secs int64) string {
			if secs == 0 {
				return "–"
			}
			if secs < 60 {
				return fmt.Sprintf("%ds", secs)
			}
			if secs < 3600 {
				return fmt.Sprintf("%dm %ds", secs/60, secs%60)
			}
			return fmt.Sprintf("%dh %dm", secs/3600, (secs%3600)/60)
		},
		"formatUnixTime": func(ts int64) string {
			if ts == 0 {
				return "–"
			}
			return time.Unix(ts, 0).In(loc).Format("02 Jan 2006 15:04")
		},
		"isPast": func(ts int64) bool {
			return ts > 0 && time.Unix(ts, 0).Before(time.Now())
		},
		"daysUntil": func(ts int64) int {
			d := int(time.Until(time.Unix(ts, 0)).Hours() / 24)
			if d < 0 {
				return 0
			}
			return d
		},
		"deref": func(p *int) int {
			if p == nil {
				return 0
			}
			return *p
		},
		"isAdmin": func(role string) bool { return role == "admin" },
		"canEdit": func(role string) bool { return role == "admin" || role == "editor" },
		"not":     func(v bool) bool { return !v },
		"formatTimeAgo": func(v any) string {
			var t time.Time
			switch tv := v.(type) {
			case *time.Time:
				if tv == nil {
					return "nunca"
				}
				t = *tv
			case time.Time:
				t = tv
			default:
				return "–"
			}
			d := time.Since(t)
			switch {
			case d < 2*time.Minute:
				return "hace un momento"
			case d < time.Hour:
				return fmt.Sprintf("hace %dm", int(d.Minutes()))
			case d < 2*time.Hour:
				return "hace 1 hora"
			case d < 24*time.Hour:
				return fmt.Sprintf("hace %dh", int(d.Hours()))
			case d < 48*time.Hour:
				return "ayer"
			default:
				return fmt.Sprintf("hace %dd", int(d.Hours()/24))
			}
		},
		"keyPreview": func(key string) string {
			if len(key) <= 12 {
				return key
			}
			parts := strings.SplitN(key, "-", 2)
			if len(parts) != 2 || len(parts[1]) <= 8 {
				return key[:4] + "..." + key[len(key)-4:]
			}
			tok := parts[1]
			return parts[0] + "-" + tok[:8] + "..." + tok[len(tok)-4:]
		},
	}
}

// Render renders a layout template (base.html + page).
func (t *Templates) Render(w http.ResponseWriter, r *http.Request, name string, data any) {
	if m, ok := data.(map[string]any); ok {
		m["CSRFField"] = csrf.TemplateField(r)
		m["CSRFToken"] = csrf.Token(r)
		m["Version"] = t.version
		if _, has := m["Flash"]; !has {
			m["Flash"] = r.URL.Query().Get("flash")
		}
		if _, has := m["FlashOK"]; !has {
			m["FlashOK"] = r.URL.Query().Get("ok") == "1"
		}
		if _, has := m["ShowSessionSecureWarning"]; !has {
			m["ShowSessionSecureWarning"] = !t.secure && requestLooksPublicHTTPS(r)
		}
		if _, has := m["ShowPublicHTTPWarning"]; !has {
			m["ShowPublicHTTPWarning"] = requestLooksPublicHTTP(r)
		}
		if _, has := m["SensitiveActionsRequireTOTP"]; !has && t.sensitiveTOTPEnabled != nil {
			m["SensitiveActionsRequireTOTP"] = t.sensitiveTOTPEnabled()
		}
		if _, has := m["Active"]; !has {
			m["Active"] = templateActive[name]
		}
		if _, has := m["AlertCritical"]; !has && t.badgeCounts != nil {
			c, w := t.badgeCounts()
			m["AlertCritical"] = c
			m["AlertWarning"] = w
		}
		if di := debug.FromContext(r.Context()); di != nil {
			di.Mu.Lock()
			di.Template = name
			di.Mu.Unlock()
		}

		jsonBytes, err := json.MarshalIndent(m, "", "  ")
		var jsonStr string
		if err != nil {
			jsonStr = "marshal error: " + err.Error()
		} else {
			jsonStr = string(jsonBytes)
			if len(jsonStr) > 8000 {
				jsonStr = jsonStr[:8000] + "\n... (truncated)"
			}
		}
		debug.RecordTemplateData(r.Context(), jsonStr)
	}

	var tmpl *template.Template
	var err error

	if standaloneTemplates[name] {
		tmpl, err = template.New("").Funcs(t.funcMap).
			ParseFS(t.fs, "web/templates/"+name)
		if err != nil {
			renderTemplateError(w, r, name, "parse", err)
			return
		}
		var buf bytes.Buffer
		if err := tmpl.ExecuteTemplate(&buf, name, data); err != nil {
			renderTemplateError(w, r, name, "exec", err)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(buf.Bytes())
		return
	}

	tmpl, err = template.New("").Funcs(t.funcMap).
		ParseFS(t.fs, "web/templates/base.html", "web/templates/"+name)
	if err != nil {
		renderTemplateError(w, r, name, "parse", err)
		return
	}
	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		renderTemplateError(w, r, name, "exec", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(buf.Bytes())
}

func requestLooksPublicHTTPS(r *http.Request) bool {
	return netutil.RequestScheme(r) == "https" && netutil.HostLooksPublic(netutil.HostFromRequest(r))
}

func requestLooksPublicHTTP(r *http.Request) bool {
	return netutil.RequestScheme(r) == "http" && netutil.HostLooksPublic(netutil.HostFromRequest(r))
}

func renderTemplateError(w http.ResponseWriter, r *http.Request, name, phase string, err error) {
	debug.RecordVar(r.Context(), "template_error", phase+": "+err.Error())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusInternalServerError)

	title := "Error renderizando plantilla"
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="es">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
  <style>
    body{font-family:system-ui,-apple-system,Segoe UI,sans-serif;margin:0;background:#f8fafc;color:#0f172a}
    main{max-width:960px;margin:48px auto;padding:0 24px}
    .box{background:#fff;border:1px solid #e2e8f0;border-radius:8px;padding:24px;box-shadow:0 8px 24px rgba(15,23,42,.06)}
    h1{font-size:24px;margin:0 0 8px}
    p{color:#475569;margin:0 0 16px}
    code{background:#f1f5f9;border:1px solid #e2e8f0;border-radius:4px;padding:2px 6px}
    pre{white-space:pre-wrap;word-break:break-word;background:#0f172a;color:#e2e8f0;border-radius:6px;padding:16px;overflow:auto}
  </style>
</head>
<body>
  <main>
    <div class="box">
      <h1>%s</h1>
      <p>Falló la fase <code>%s</code> de <code>%s</code>.</p>
      <pre>%s</pre>
    </div>
  </main>
</body>
</html>`,
		template.HTMLEscapeString(title),
		template.HTMLEscapeString(title),
		template.HTMLEscapeString(phase),
		template.HTMLEscapeString(name),
		template.HTMLEscapeString(err.Error()),
	)
}

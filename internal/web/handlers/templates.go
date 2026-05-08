package webhandlers

import (
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/csrf"
)

var standaloneTemplates = map[string]bool{
	"login.html":           true,
	"api_key_created.html": true,
}

// templateActive maps template name → sidebar active key
var templateActive = map[string]string{
	"alerts.html":             "alerts-view",
	"dashboard.html":          "dashboard",
	"servers_pve.html":        "pve",
	"server_pve_detail.html":  "pve",
	"backup_config.html":      "pve",
	"vm_backup_config_form.html": "pve",
	"servers_pbs.html":        "pbs",
	"server_pbs_detail.html":  "pbs",
	"api_keys.html":           "keys",
	"api_key_created.html":    "keys",
	"users.html":              "users",
	"email_settings.html":        "email",
	"maintenance_settings.html": "maintenance",
	"alerts_settings.html":      "alerts",
	"ip_bans.html":             "ip-bans",
	"profile.html":            "",
	"api_key_edit.html":       "keys",
	"reports_pve.html":        "pve",
}

type Templates struct {
	fs      embed.FS
	funcMap template.FuncMap
	version string
}

func NewTemplates(fs embed.FS, version string) *Templates {
	return &Templates{
		fs:      fs,
		funcMap: makeFuncMap(),
		version: version,
	}
}

func makeFuncMap() template.FuncMap {
	return template.FuncMap{
		"formatTime": func(v any) string {
			switch t := v.(type) {
			case *time.Time:
				if t == nil {
					return "nunca"
				}
				return t.Format("02 Jan 2006 15:04")
			case time.Time:
				return t.Format("02 Jan 2006 15:04")
			default:
				return "–"
			}
		},
		"formatBytes": func(b int64) string {
			const unit = 1000
			if b < unit {
				return fmt.Sprintf("%d B", b)
			}
			div, exp := int64(unit), 0
			for n := b / unit; n >= unit; n /= unit {
				div *= unit
				exp++
			}
			return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
		},
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
			return time.Unix(ts, 0).Format("02 Jan 2006 15:04")
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
		if _, has := m["Active"]; !has {
			m["Active"] = templateActive[name]
		}
	}

	var tmpl *template.Template
	var err error

	if standaloneTemplates[name] {
		tmpl, err = template.New("").Funcs(t.funcMap).
			ParseFS(t.fs, "web/templates/"+name)
		if err != nil {
			http.Error(w, "template parse: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
			http.Error(w, "template exec: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	tmpl, err = template.New("").Funcs(t.funcMap).
		ParseFS(t.fs, "web/templates/base.html", "web/templates/"+name)
	if err != nil {
		http.Error(w, "template parse: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.ExecuteTemplate(w, "base", data); err != nil {
		http.Error(w, "template exec: "+err.Error(), http.StatusInternalServerError)
	}
}

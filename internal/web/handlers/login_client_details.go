package webhandlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"probakgo/internal/domain"
)

const maxLoginClientDetailsBytes = 8 << 10

// loginClientDetails contains non-authentication signals supplied by a browser.
// It is intended to help an administrator investigate a login, not to identify
// a user: every value can be absent or forged by the client.
type loginClientDetails struct {
	Brands              []string `json:"brands,omitempty"`
	Platform            string   `json:"platform,omitempty"`
	PlatformVersion     string   `json:"platform_version,omitempty"`
	Architecture        string   `json:"architecture,omitempty"`
	Bitness             string   `json:"bitness,omitempty"`
	Model               string   `json:"model,omitempty"`
	Mobile              *bool    `json:"mobile,omitempty"`
	Languages           []string `json:"languages,omitempty"`
	Timezone            string   `json:"timezone,omitempty"`
	Screen              string   `json:"screen,omitempty"`
	Viewport            string   `json:"viewport,omitempty"`
	ColorDepth          int      `json:"color_depth,omitempty"`
	TouchPoints         int      `json:"touch_points,omitempty"`
	HardwareConcurrency int      `json:"hardware_concurrency,omitempty"`
	DeviceMemory        float64  `json:"device_memory_gb,omitempty"`
	CookiesEnabled      *bool    `json:"cookies_enabled,omitempty"`
	AcceptLanguage      string   `json:"accept_language,omitempty"`
	ClientHints         string   `json:"client_hints,omitempty"`
	Host                string   `json:"host,omitempty"`
}

type loginClientDetailField struct {
	Label string
	Value string
}

type loginAttemptView struct {
	domain.LoginAttempt
	ClientSummary string
	ClientFields  []loginClientDetailField
}

func (h *WebH) recordLoginAttempt(r *http.Request, username, ip, userAgent, result, reason string) {
	_ = h.store.InsertLoginAttempt(r.Context(), username, ip, userAgent, loginClientDetailsFromRequest(r), result, reason)
}

func loginClientDetailsFromRequest(r *http.Request) string {
	var details loginClientDetails
	raw := r.FormValue("client_details")
	if len(raw) <= maxLoginClientDetailsBytes {
		_ = json.Unmarshal([]byte(raw), &details)
	}

	details.Brands = cleanClientDetailList(details.Brands, 8)
	details.Platform = cleanClientDetail(details.Platform, 100)
	details.PlatformVersion = cleanClientDetail(details.PlatformVersion, 100)
	details.Architecture = cleanClientDetail(details.Architecture, 50)
	details.Bitness = cleanClientDetail(details.Bitness, 20)
	details.Model = cleanClientDetail(details.Model, 100)
	details.Languages = cleanClientDetailList(details.Languages, 12)
	details.Timezone = cleanClientDetail(details.Timezone, 100)
	details.Screen = cleanClientDetail(details.Screen, 50)
	details.Viewport = cleanClientDetail(details.Viewport, 50)
	if details.ColorDepth < 0 || details.ColorDepth > 128 {
		details.ColorDepth = 0
	}
	if details.TouchPoints < 0 || details.TouchPoints > 100 {
		details.TouchPoints = 0
	}
	if details.HardwareConcurrency < 0 || details.HardwareConcurrency > 4096 {
		details.HardwareConcurrency = 0
	}
	if details.DeviceMemory < 0 || details.DeviceMemory > 65536 {
		details.DeviceMemory = 0
	}
	details.AcceptLanguage = cleanClientDetail(r.Header.Get("Accept-Language"), 512)
	details.ClientHints = cleanClientDetail(strings.Join(nonEmptyStrings([]string{
		r.Header.Get("Sec-CH-UA"),
		r.Header.Get("Sec-CH-UA-Mobile"),
		r.Header.Get("Sec-CH-UA-Platform"),
		r.Header.Get("Sec-CH-UA-Platform-Version"),
		r.Header.Get("Sec-CH-UA-Arch"),
		r.Header.Get("Sec-CH-UA-Bitness"),
		r.Header.Get("Sec-CH-UA-Model"),
	}), " | "), 1024)
	details.Host = cleanClientDetail(r.Host, 255)

	encoded, err := json.Marshal(details)
	if err != nil || string(encoded) == "{}" {
		return ""
	}
	return string(encoded)
}

func loginAttemptViews(attempts []domain.LoginAttempt) []loginAttemptView {
	views := make([]loginAttemptView, 0, len(attempts))
	for _, attempt := range attempts {
		view := loginAttemptView{LoginAttempt: attempt, ClientSummary: "Navegador no identificado"}
		var details loginClientDetails
		if attempt.ClientDetails != "" && json.Unmarshal([]byte(attempt.ClientDetails), &details) == nil {
			view.ClientSummary, view.ClientFields = loginClientDetailsView(details)
		}
		views = append(views, view)
	}
	return views
}

func loginClientDetailsView(details loginClientDetails) (string, []loginClientDetailField) {
	fields := make([]loginClientDetailField, 0, 18)
	add := func(label, value string) {
		if value != "" {
			fields = append(fields, loginClientDetailField{Label: label, Value: value})
		}
	}
	brands := strings.Join(details.Brands, ", ")
	add("Navegador", brands)
	add("Plataforma", details.Platform)
	add("Versión de plataforma", details.PlatformVersion)
	add("Arquitectura", details.Architecture)
	add("Bits", details.Bitness)
	add("Modelo", details.Model)
	if details.Mobile != nil {
		add("Tipo de dispositivo", map[bool]string{true: "Móvil", false: "Escritorio"}[*details.Mobile])
	}
	add("Idiomas", strings.Join(details.Languages, ", "))
	add("Zona horaria", details.Timezone)
	add("Pantalla", details.Screen)
	add("Ventana", details.Viewport)
	if details.ColorDepth > 0 {
		add("Profundidad de color", strconv.Itoa(details.ColorDepth)+" bits")
	}
	if details.TouchPoints > 0 {
		add("Puntos táctiles", strconv.Itoa(details.TouchPoints))
	}
	if details.HardwareConcurrency > 0 {
		add("CPU lógica", strconv.Itoa(details.HardwareConcurrency)+" núcleos")
	}
	if details.DeviceMemory > 0 {
		add("Memoria expuesta", strconv.FormatFloat(details.DeviceMemory, 'f', -1, 64)+" GB")
	}
	if details.CookiesEnabled != nil {
		add("Cookies", map[bool]string{true: "Sí", false: "No"}[*details.CookiesEnabled])
	}
	add("Accept-Language", details.AcceptLanguage)
	add("Client Hints", details.ClientHints)
	add("Host solicitado", details.Host)

	summary := brands
	if summary == "" {
		summary = details.Platform
	}
	if summary == "" {
		summary = "Navegador no identificado"
	}
	return summary, fields
}

func cleanClientDetail(value string, max int) string {
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, strings.TrimSpace(value))
	if len(value) > max {
		return value[:max]
	}
	return value
}

func cleanClientDetailList(values []string, maxItems int) []string {
	out := make([]string, 0, min(len(values), maxItems))
	for _, value := range values {
		if value = cleanClientDetail(value, 100); value != "" {
			out = append(out, value)
			if len(out) == maxItems {
				break
			}
		}
	}
	return out
}

func nonEmptyStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

package webhandlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"probakgo/internal/domain"
)

func TestLoginClientDetailsFromRequestCombinesBrowserAndHTTPDetails(t *testing.T) {
	form := url.Values{
		"client_details": {`{"brands":["Chromium 140"],"platform":"Windows","timezone":"Europe/Madrid","screen":"1920 × 1080","hardware_concurrency":8,"cookies_enabled":true}`},
	}
	req := httptest.NewRequest(http.MethodPost, "https://probakgo.example/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept-Language", "es-ES,es;q=0.9")
	req.Header.Set("Sec-CH-UA", `"Chromium";v="140"`)

	var got loginClientDetails
	if err := json.Unmarshal([]byte(loginClientDetailsFromRequest(req)), &got); err != nil {
		t.Fatalf("decode stored client details: %v", err)
	}
	if len(got.Brands) != 1 || got.Brands[0] != "Chromium 140" {
		t.Fatalf("brands = %#v", got.Brands)
	}
	if got.Platform != "Windows" || got.Timezone != "Europe/Madrid" {
		t.Fatalf("browser details = %#v", got)
	}
	if got.AcceptLanguage != "es-ES,es;q=0.9" || got.ClientHints == "" {
		t.Fatalf("HTTP details were not stored: %#v", got)
	}
	if got.Host != "probakgo.example" {
		t.Fatalf("host = %q", got.Host)
	}
}

func TestLoginAttemptViewsShowsStoredDetails(t *testing.T) {
	views := loginAttemptViews([]domain.LoginAttempt{{
		UserAgent:     "test-agent",
		ClientDetails: `{"brands":["Firefox 140"],"platform":"Linux","cookies_enabled":true}`,
	}})
	if len(views) != 1 || views[0].ClientSummary != "Firefox 140" {
		t.Fatalf("views = %#v", views)
	}
	if len(views[0].ClientFields) < 3 {
		t.Fatalf("client fields = %#v", views[0].ClientFields)
	}
}

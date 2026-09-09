package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/store"
	webhandlers "probakgo/internal/web/handlers"
)

func TestNASActionsAcceptValidTOTPWithEncryptedSecret(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })
	st, err := store.NewEncrypted(database, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	id, err := st.CreateUser(ctx, "nas-admin", "unused", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnableUserTOTP(ctx, id, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SensitiveActionsRequireTOTP: true}); err != nil {
		t.Fatal(err)
	}
	user, err := st.GetUser(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	session.Init("test-session-key-32-bytes-long!!", false)
	auth := httptest.NewRecorder()
	if err := session.SetUserWithVersion(auth, httptest.NewRequest("GET", "/", nil), user.Username, user.Role, user.SessionVersion); err != nil {
		t.Fatal(err)
	}
	tmpl := webhandlers.NewTemplates(os.DirFS("../.."), "test", time.UTC, false, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	h := webhandlers.New(st, tmpl, nil)
	handler := RequireLogin(st)(RequireAdmin(RequireTOTPForSensitiveAction(st)(http.HandlerFunc(h.NASBackupSettingsPost))))
	for _, action := range []string{"save", "test"} {
		t.Run(action, func(t *testing.T) {
			var counter [8]byte
			binary.BigEndian.PutUint64(counter[:], uint64(time.Now().Unix()/30))
			mac := hmac.New(sha1.New, []byte("12345678901234567890"))
			mac.Write(counter[:])
			sum := mac.Sum(nil)
			code := fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[sum[len(sum)-1]&15:])&0x7fffffff)%1000000)
			// An invalid NAS config reaches validation after TOTP without connecting anywhere.
			form := url.Values{"action": {action}, "nas_enabled": {"on"}, "totp_code": {code}}
			req := httptest.NewRequest("POST", "/settings/maintenance/nas", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			for _, cookie := range auth.Result().Cookies() {
				req.AddCookie(cookie)
			}
			rr := httptest.NewRecorder()
			handler.ServeHTTP(rr, req)
			if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), "indica servidor, puerto válido, usuario y contraseña SFTP") {
				t.Fatalf("valid TOTP did not reach NAS handler: status=%d location=%s", rr.Code, rr.Header().Get("Location"))
			}
			next := httptest.NewRequest("GET", "/", nil)
			for _, cookie := range rr.Result().Cookies() {
				next.AddCookie(cookie)
			}
			if !session.SensitiveTOTPFresh(next, time.Now()) {
				t.Fatal("TOTP confirmation not preserved")
			}
			if strings.Contains(rr.Body.String(), `name="totp_code" value="`+code) {
				t.Fatal("code rendered back into form")
			}
		})
	}
}

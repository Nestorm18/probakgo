package session

import (
	"crypto/sha256"
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

const sessionName = "probakgo"

var store *sessions.CookieStore

func Init(key string, secure bool) {
	// Domain-separated keys encrypt all cookie contents, including setup secrets.
	authKey := sha256.Sum256([]byte("probakgo/session/auth/v2:" + key))
	encKey := sha256.Sum256([]byte("probakgo/session/encryption/v2:" + key))
	store = sessions.NewCookieStore(authKey[:], encKey[:])
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
	store.MaxAge(86400 * 7)
	resetChallenges()
}

func getSession(r *http.Request) (*sessions.Session, error) {
	return store.Get(r, sessionName)
}

func GetUser(r *http.Request) (username, role string, ok bool) {
	if store == nil {
		return "", "", false
	}
	sess, err := getSession(r)
	if err != nil {
		return "", "", false
	}
	u, uok := sess.Values["username"].(string)
	ro, rok := sess.Values["role"].(string)
	return u, ro, uok && rok && u != ""
}

func SetUser(w http.ResponseWriter, r *http.Request, userID int64, username, role string) error {
	return SetUserWithVersion(w, r, userID, username, role, 1)
}

func SetUserWithVersion(w http.ResponseWriter, r *http.Request, userID int64, username, role string, version int) error {
	sess, _ := getSession(r)
	sess.Values = make(map[any]any)
	sess.Values["user_id"] = userID
	sess.Values["username"] = username
	sess.Values["role"] = role
	sess.Values["session_version"] = version
	return sess.Save(r, w)
}

func UserID(r *http.Request) (int64, bool) {
	if store == nil {
		return 0, false
	}
	sess, err := getSession(r)
	if err != nil {
		return 0, false
	}
	id, ok := sess.Values["user_id"].(int64)
	return id, ok && id > 0
}

func UserVersion(r *http.Request) (int, bool) {
	if store == nil {
		return 0, false
	}
	sess, err := getSession(r)
	if err != nil {
		return 0, false
	}
	switch v := sess.Values["session_version"].(type) {
	case int:
		return v, v > 0
	case int64:
		return int(v), v > 0
	case float64:
		return int(v), v > 0
	default:
		return 0, false
	}
}

func ClearPending2FA(w http.ResponseWriter, r *http.Request) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	discardChallenge(r)
	delete(sess.Values, "pending_2fa_token")
	return sess.Save(r, w)
}

func SetPendingTOTPSetup(w http.ResponseWriter, r *http.Request, secret string) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["pending_totp_secret"] = secret
	sess.Values["pending_totp_expires"] = time.Now().Add(10 * time.Minute).Unix()
	return sess.Save(r, w)
}

func GetPendingTOTPSetup(r *http.Request) (string, bool) {
	if store == nil {
		return "", false
	}
	sess, err := getSession(r)
	if err != nil {
		return "", false
	}
	secret, ok := sess.Values["pending_totp_secret"].(string)
	expires, _ := sess.Values["pending_totp_expires"].(int64)
	return secret, ok && secret != "" && time.Now().Unix() < expires
}

func ClearPendingTOTPSetup(w http.ResponseWriter, r *http.Request) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	delete(sess.Values, "pending_totp_secret")
	delete(sess.Values, "pending_totp_expires")
	return sess.Save(r, w)
}

func SetTelegramPairing(w http.ResponseWriter, r *http.Request, userID int64, code string, expires time.Time) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["telegram_pairing_code"] = code
	sess.Values["telegram_pairing_expires"] = expires.Unix()
	sess.Values["telegram_pairing_user_id"] = userID
	return sess.Save(r, w)
}

func GetTelegramPairing(r *http.Request, userID int64, now time.Time) (string, bool) {
	if store == nil {
		return "", false
	}
	sess, err := getSession(r)
	if err != nil {
		return "", false
	}
	code, _ := sess.Values["telegram_pairing_code"].(string)
	var expires int64
	switch value := sess.Values["telegram_pairing_expires"].(type) {
	case int64:
		expires = value
	case int:
		expires = int64(value)
	case float64:
		expires = int64(value)
	}
	var pairedUserID int64
	switch value := sess.Values["telegram_pairing_user_id"].(type) {
	case int64:
		pairedUserID = value
	case int:
		pairedUserID = int64(value)
	case float64:
		pairedUserID = int64(value)
	}
	return code, code != "" && pairedUserID == userID && time.Unix(expires, 0).After(now)
}

func ClearTelegramPairing(w http.ResponseWriter, r *http.Request) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	delete(sess.Values, "telegram_pairing_code")
	delete(sess.Values, "telegram_pairing_expires")
	delete(sess.Values, "telegram_pairing_user_id")
	return sess.Save(r, w)
}

func SensitiveTOTPFresh(r *http.Request, now time.Time) bool {
	until, ok := SensitiveTOTPUntil(r)
	return ok && until.After(now)
}

// SensitiveTOTPUntil returns the expiry of the recent sensitive-action TOTP
// validation stored in the session.
func SensitiveTOTPUntil(r *http.Request) (time.Time, bool) {
	if store == nil {
		return time.Time{}, false
	}
	sess, err := getSession(r)
	if err != nil {
		return time.Time{}, false
	}
	var unix int64
	switch v := sess.Values["sensitive_totp_until"].(type) {
	case int64:
		unix = v
	case int:
		unix = int64(v)
	case float64:
		unix = int64(v)
	case time.Time:
		unix = v.Unix()
	default:
		return time.Time{}, false
	}
	return time.Unix(unix, 0), true
}

func SetSensitiveTOTPFresh(w http.ResponseWriter, r *http.Request, until time.Time) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["sensitive_totp_until"] = until.Unix()
	return sess.Save(r, w)
}

func Clear(w http.ResponseWriter, r *http.Request) {
	discardChallenge(r)
	sess, _ := getSession(r)
	sess.Options.MaxAge = -1
	_ = sess.Save(r, w)
}

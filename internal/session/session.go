package session

import (
	"net/http"
	"time"

	"github.com/gorilla/sessions"
)

const sessionName = "probakgo"

var store *sessions.CookieStore

func Init(key string, secure bool) {
	store = sessions.NewCookieStore([]byte(key))
	store.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   86400 * 7,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   secure,
	}
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

func SetUser(w http.ResponseWriter, r *http.Request, username, role string) error {
	return SetUserWithVersion(w, r, username, role, 1)
}

func SetUserWithVersion(w http.ResponseWriter, r *http.Request, username, role string, version int) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["username"] = username
	sess.Values["role"] = role
	sess.Values["session_version"] = version
	delete(sess.Values, "pending_2fa_user_id")
	delete(sess.Values, "pending_2fa_next")
	return sess.Save(r, w)
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

func SetPending2FA(w http.ResponseWriter, r *http.Request, userID int64, next string) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["pending_2fa_user_id"] = userID
	sess.Values["pending_2fa_next"] = next
	delete(sess.Values, "username")
	delete(sess.Values, "role")
	return sess.Save(r, w)
}

func GetPending2FA(r *http.Request) (userID int64, next string, ok bool) {
	if store == nil {
		return 0, "", false
	}
	sess, err := getSession(r)
	if err != nil {
		return 0, "", false
	}
	switch v := sess.Values["pending_2fa_user_id"].(type) {
	case int64:
		userID = v
	case int:
		userID = int64(v)
	case float64:
		userID = int64(v)
	default:
		return 0, "", false
	}
	next, _ = sess.Values["pending_2fa_next"].(string)
	return userID, next, userID > 0
}

func ClearPending2FA(w http.ResponseWriter, r *http.Request) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	delete(sess.Values, "pending_2fa_user_id")
	delete(sess.Values, "pending_2fa_next")
	return sess.Save(r, w)
}

func SetPendingTOTPSetup(w http.ResponseWriter, r *http.Request, secret string) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["pending_totp_secret"] = secret
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
	return secret, ok && secret != ""
}

func ClearPendingTOTPSetup(w http.ResponseWriter, r *http.Request) error {
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	delete(sess.Values, "pending_totp_secret")
	return sess.Save(r, w)
}

func SensitiveTOTPFresh(r *http.Request, now time.Time) bool {
	if store == nil {
		return false
	}
	sess, err := getSession(r)
	if err != nil {
		return false
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
		return false
	}
	return time.Unix(unix, 0).After(now)
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
	sess, _ := getSession(r)
	sess.Options.MaxAge = -1
	_ = sess.Save(r, w)
}

package session

import (
	"net/http"

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
	sess, err := getSession(r)
	if err != nil {
		return err
	}
	sess.Values["username"] = username
	sess.Values["role"] = role
	return sess.Save(r, w)
}

func Clear(w http.ResponseWriter, r *http.Request) {
	sess, _ := getSession(r)
	sess.Options.MaxAge = -1
	_ = sess.Save(r, w)
}

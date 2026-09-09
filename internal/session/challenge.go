package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const challengeLifetime = 5 * time.Minute

type loginChallenge struct {
	userID   int64
	version  int
	next     string
	expires  time.Time
	attempts int
}

// Password verification is a short-lived, one-use server-side capability.
// Restarting the process deliberately invalidates incomplete logins.
var challenges = struct {
	sync.Mutex
	items map[string]loginChallenge
}{items: make(map[string]loginChallenge)}

func resetChallenges() {
	challenges.Lock()
	defer challenges.Unlock()
	challenges.items = make(map[string]loginChallenge)
}

func SetPending2FA(w http.ResponseWriter, r *http.Request, userID int64, next string, version int) error {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return err
	}
	token := hex.EncodeToString(raw[:])
	now := time.Now()
	challenges.Lock()
	for id, c := range challenges.items {
		if !c.expires.After(now) {
			delete(challenges.items, id)
		}
	}
	if len(challenges.items) >= 10000 {
		challenges.Unlock()
		return fmt.Errorf("too many pending logins")
	}
	challenges.items[token] = loginChallenge{userID: userID, version: version, next: next, expires: now.Add(challengeLifetime)}
	challenges.Unlock()
	sess, _ := getSession(r)
	discardChallenge(r)
	sess.Values = map[any]any{"pending_2fa_token": token}
	if err := sess.Save(r, w); err != nil {
		challenges.Lock()
		delete(challenges.items, token)
		challenges.Unlock()
		return err
	}
	return nil
}

func challengeToken(r *http.Request) string {
	if store == nil {
		return ""
	}
	sess, err := getSession(r)
	if err != nil {
		return ""
	}
	token, _ := sess.Values["pending_2fa_token"].(string)
	return token
}

func GetPending2FA(r *http.Request) (userID int64, next string, version int, ok bool) {
	token := challengeToken(r)
	challenges.Lock()
	defer challenges.Unlock()
	c, found := challenges.items[token]
	return c.userID, c.next, c.version, found && c.expires.After(time.Now())
}

func ConsumePending2FA(r *http.Request, userID int64, version int) bool {
	token := challengeToken(r)
	challenges.Lock()
	defer challenges.Unlock()
	c, ok := challenges.items[token]
	delete(challenges.items, token)
	return ok && c.userID == userID && c.version == version && c.expires.After(time.Now())
}

// Count attempts against the password challenge, independently of source IP.
func AllowPending2FAAttempt(r *http.Request) bool {
	token := challengeToken(r)
	challenges.Lock()
	defer challenges.Unlock()
	c, ok := challenges.items[token]
	if !ok || !c.expires.After(time.Now()) || c.attempts >= 5 {
		delete(challenges.items, token)
		return false
	}
	c.attempts++
	challenges.items[token] = c
	return true
}

func discardChallenge(r *http.Request) {
	token := challengeToken(r)
	challenges.Lock()
	delete(challenges.items, token)
	challenges.Unlock()
}

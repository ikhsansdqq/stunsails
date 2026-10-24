package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/auth"
	"github.com/ikhsansdqq/stunsails/internal/store"
)

const pendingTTL = 5 * time.Minute

func (s *Server) session(w http.ResponseWriter, r *http.Request) {
	se, ok := s.current(r)
	out := map[string]any{
		"authenticated": ok,
		"configured":    s.TOTP != nil && s.Password != "",
	}
	if ok {
		out["stepup_until"] = se.stepUp.Add(StepUpWindow).Unix()
		out["now"] = s.now().Unix()
	}
	writeJSON(w, http.StatusOK, out)
}

// limited answers 429 when ip is locked out. While wrong tries pile up from
// many addresses, every try is also held back a little first.
func (s *Server) limited(w http.ResponseWriter, r *http.Request, ip string) bool {
	if ok, until := s.Limiter.Allowed(ip, s.now()); !ok {
		mins := int(until.Sub(s.now()).Minutes()) + 1
		fail(w, http.StatusTooManyRequests, fmt.Sprintf("Too many wrong tries. Try again in %d min.", mins))
		return true
	}
	if d := s.Limiter.Delay(s.now()); d > 0 {
		select {
		case <-time.After(d):
		case <-r.Context().Done():
			return true
		}
	}
	return false
}

const limiterKey = "login_fails"

// failed counts a wrong try and saves the count, so a restart doesn't clear a lockout.
func (s *Server) failed(ip string, now time.Time) {
	s.Limiter.Fail(ip, now)
	s.saveLimiter()
}

func (s *Server) saveLimiter() {
	if b, err := json.Marshal(s.Limiter.Snapshot()); err == nil {
		s.Store.SetMeta(limiterKey, string(b))
	}
}

// RestoreLimiter loads the wrong tries saved before a restart.
func (s *Server) RestoreLimiter() {
	var m map[string][]int64
	if json.Unmarshal([]byte(s.Store.Meta(limiterKey)), &m) == nil {
		s.Limiter.Restore(m)
	}
}

// triesLeft ends a "wrong" message with how close the address is to a lockout.
func (s *Server) triesLeft(msg, ip string) string {
	switch n := s.Limiter.Left(ip, s.now()); n {
	case 0:
		return msg + " Too many wrong tries: wait 15 min."
	case 1:
		return msg + " 1 try left before a 15-minute lock."
	default:
		if n <= 3 {
			return fmt.Sprintf("%s %d tries left before a 15-minute lock.", msg, n)
		}
		return msg
	}
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if s.TOTP == nil || s.Password == "" {
		fail(w, http.StatusServiceUnavailable, "StunSails isn't set up yet. On the server run: sudo stunsails passwd, then sudo stunsails enroll.")
		return
	}
	ip := clientIP(r)
	if s.limited(w, r, ip) {
		return
	}
	var body struct{ Password string }
	if !readJSON(w, r, &body) {
		return
	}
	if !auth.CheckPassword(s.Password, body.Password) {
		s.failed(ip, s.now())
		s.audit(r, "login", "password", "", "", errors.New("wrong password"))
		fail(w, http.StatusUnauthorized, s.triesLeft("That password isn't right.", ip))
		return
	}
	tok := auth.Token(32)
	now := s.now()
	s.mu.Lock()
	if s.pending == nil {
		s.pending = map[string]pendingLogin{}
	}
	for k, p := range s.pending {
		if now.After(p.exp) {
			delete(s.pending, k)
		}
	}
	s.pending[hashToken(tok)] = pendingLogin{ip: ip, exp: now.Add(pendingTTL)}
	s.mu.Unlock()
	http.SetCookie(w, &http.Cookie{Name: "stunsails_pending", Value: tok, Path: "/api/login", HttpOnly: true, Secure: s.Secure,
		SameSite: http.SameSiteStrictMode, MaxAge: int(pendingTTL.Seconds())})
	writeJSON(w, http.StatusOK, map[string]string{"next": "totp"})
}

func (s *Server) loginTOTP(w http.ResponseWriter, r *http.Request) {
	ip := clientIP(r)
	if s.limited(w, r, ip) {
		return
	}
	c, err := r.Cookie("stunsails_pending")
	if err != nil || s.TOTP == nil {
		fail(w, http.StatusUnauthorized, "Enter your password first.")
		return
	}
	key := hashToken(c.Value)
	now := s.now()
	s.mu.Lock()
	p, ok := s.pending[key]
	s.mu.Unlock()
	if !ok || now.After(p.exp) {
		fail(w, http.StatusUnauthorized, "That sign-in took too long. Enter your password again.")
		return
	}
	var body struct{ Code string }
	if !readJSON(w, r, &body) {
		return
	}
	if !s.TOTP.Verify(body.Code, now) {
		s.failed(ip, now)
		s.audit(r, "login", "code", "", "", errors.New("wrong authenticator code"))
		fail(w, http.StatusUnauthorized, s.triesLeft("That code isn't right, or it was already used.", ip))
		return
	}
	s.mu.Lock()
	delete(s.pending, key)
	s.mu.Unlock()
	s.Limiter.Reset(ip)
	s.saveLimiter()
	tok := auth.Token(32)
	ua := r.UserAgent()
	if len(ua) > 200 {
		ua = ua[:200]
	}
	if err := s.Store.CreateSession(store.Session{Hash: hashToken(tok), Created: now, Seen: now, StepUp: now, IP: ip, UA: ua}); err != nil {
		fail(w, http.StatusInternalServerError, "could not save the session")
		return
	}
	s.audit(r, "login", "signed in", "", "", nil)
	http.SetCookie(w, &http.Cookie{Name: "stunsails_pending", Value: "", Path: "/api/login", MaxAge: -1, HttpOnly: true, Secure: s.Secure})
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: tok, Path: "/", HttpOnly: true, Secure: s.Secure,
		SameSite: http.SameSiteStrictMode, MaxAge: int((7 * 24 * time.Hour).Seconds())})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if se, ok := s.current(r); ok {
		s.Store.DeleteSession(se.hash)
		s.audit(r, "login", "signed out", "", "", nil)
	}
	http.SetCookie(w, &http.Cookie{Name: s.cookieName(), Value: "", Path: "/", MaxAge: -1, HttpOnly: true, Secure: s.Secure, SameSite: http.SameSiteStrictMode})
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) stepUp(w http.ResponseWriter, r *http.Request, se sess) {
	ip := clientIP(r)
	if s.limited(w, r, ip) {
		return
	}
	var body struct{ Code string }
	if !readJSON(w, r, &body) {
		return
	}
	now := s.now()
	if !s.TOTP.Verify(body.Code, now) {
		s.failed(ip, now)
		s.audit(r, "login", "step-up", "", "", errors.New("wrong authenticator code"))
		fail(w, http.StatusUnauthorized, s.triesLeft("That code isn't right, or it was already used.", ip))
		return
	}
	s.Store.StepUpSession(se.hash, now)
	writeJSON(w, http.StatusOK, map[string]any{"stepup_until": now.Add(StepUpWindow).Unix()})
}

func (s *Server) sessions(w http.ResponseWriter, r *http.Request, se sess) {
	list, err := s.Store.Sessions(se.hash)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) revokeOthers(w http.ResponseWriter, r *http.Request, se sess) {
	n, err := s.Store.DeleteOtherSessions(se.hash)
	s.audit(r, "login", "signed out other sessions", "", fmt.Sprint(n), err)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]int64{"removed": n})
}

// Package api is StunSails' HTTP surface: the JSON API the dashboard calls and
// the embedded single-page app.
package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/alert"
	"github.com/ikhsansdqq/stunsails/internal/auth"
	"github.com/ikhsansdqq/stunsails/internal/cleanup"
	"github.com/ikhsansdqq/stunsails/internal/collector"
	"github.com/ikhsansdqq/stunsails/internal/compose"
	"github.com/ikhsansdqq/stunsails/internal/docker"
	"github.com/ikhsansdqq/stunsails/internal/geo"
	"github.com/ikhsansdqq/stunsails/internal/host"
	"github.com/ikhsansdqq/stunsails/internal/hostlog"
	"github.com/ikhsansdqq/stunsails/internal/store"
	"github.com/ikhsansdqq/stunsails/internal/uptime"
)

// StepUpWindow is how long a fresh authenticator code unlocks changes.
const StepUpWindow = 5 * time.Minute

type Server struct {
	Docker    *docker.Client
	Store     *store.Store
	Collector *collector.Collector
	Host      *host.Sampler // only Info is used here; Sample belongs to the collector
	Compose   *compose.Runner
	TOTP      *auth.TOTP // nil until enrolled
	Password  string     // bcrypt hash; empty until set
	Limiter   *auth.Limiter
	Secure    bool // cookies need HTTPS (false only for local development)
	Static    fs.FS
	Now       func() time.Time
	Alerts    *alert.Notifier
	HostLog   *hostlog.Watcher
	Geo       *geo.DB // nil turns locations off
	Sites     *uptime.Checker
	Cleanup   *cleanup.Scheduler
	Started   time.Time

	mu      sync.Mutex
	pending map[string]pendingLogin
	df      *docker.DF
	dfAt    time.Time
	dfBusy  bool

	selfPrev selfSample
	helperSz int64
	helperAt time.Time
}

type pendingLogin struct {
	ip  string
	exp time.Time
}

func (s *Server) now() time.Time {
	if s.Now != nil {
		return s.Now()
	}
	return time.Now()
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	// Signing in.
	mux.HandleFunc("GET /api/session", s.session)
	mux.HandleFunc("POST /api/login", s.login)
	mux.HandleFunc("POST /api/login/totp", s.loginTOTP)
	mux.HandleFunc("POST /api/logout", s.logout)

	// Reading. Every route below needs a session.
	mux.Handle("GET /api/overview", s.authed(s.overview))
	mux.Handle("GET /api/host/history", s.authed(s.hostHistory))
	mux.Handle("GET /api/containers", s.authed(s.containers))
	mux.Handle("GET /api/containers/{id}", s.authed(s.container))
	mux.Handle("GET /api/containers/{id}/history", s.authed(s.containerHistory))
	mux.Handle("GET /api/containers/{id}/net", s.authed(s.containerNet))
	mux.Handle("GET /api/containers/{id}/logs", s.authed(s.containerLogs))
	mux.Handle("GET /api/stacks", s.authed(s.stacks))
	mux.Handle("GET /api/stacks/{project}", s.authed(s.stack))
	mux.Handle("GET /api/storage", s.authed(s.storage))
	mux.Handle("GET /api/activity", s.authed(s.activity))
	mux.Handle("GET /api/activity/page", s.authed(s.activityPage))
	mux.Handle("GET /api/activity/{id}", s.authed(s.activityDetail))
	mux.Handle("GET /api/sessions", s.authed(s.sessions))
	mux.Handle("GET /api/sites", s.authed(s.sitesList))
	mux.Handle("GET /api/sites/{host}/history", s.authed(s.siteHistory))
	mux.Handle("GET /api/settings", s.authed(s.settings))
	mux.Handle("GET /api/self", s.authed(s.self))
	mux.Handle("GET /api/alerts/feed", s.authed(s.alertFeed))

	// Changing. These also need a fresh authenticator code (step-up).
	mux.Handle("POST /api/stepup", s.mutating(s.stepUp, false))
	mux.Handle("POST /api/containers/{id}/{action}", s.mutating(s.containerAction, true))
	mux.Handle("POST /api/stacks/{project}/forget", s.mutating(s.stackForget, true))
	mux.Handle("POST /api/stacks/{project}/{verb}", s.mutating(s.stackAction, true))
	mux.Handle("DELETE /api/images/{id}", s.mutating(s.imageRemove, true))
	mux.Handle("DELETE /api/volumes/{name}", s.mutating(s.volumeRemove, true))
	mux.Handle("DELETE /api/networks/{id}", s.mutating(s.networkRemove, true))
	mux.Handle("POST /api/prune/{kind}", s.mutating(s.prune, true))
	mux.Handle("POST /api/sessions/revoke-others", s.mutating(s.revokeOthers, true))
	mux.Handle("POST /api/sites", s.mutating(s.siteAdd, true))
	mux.Handle("DELETE /api/sites/{host}", s.mutating(s.siteRemove, true))
	mux.Handle("PUT /api/settings/alerts", s.mutating(s.alertsSave, true))
	mux.Handle("POST /api/settings/alerts/test", s.mutating(s.alertsTest, false))
	mux.Handle("PUT /api/settings/cleanup", s.mutating(s.cleanupSave, true))
	mux.Handle("POST /api/cleanup/run", s.mutating(s.cleanupRun, true))

	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		fail(w, http.StatusNotFound, "no such endpoint")
	})
	mux.Handle("/", s.static())
	return headers(mux)
}

// headers adds the security headers every response carries.
func headers(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; font-src 'self'; connect-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

// sess is the signed-in session for a request.
type sess struct {
	hash   string
	stepUp time.Time
	actor  string // who the app in front says is asking; "" for StunSails' own sessions
}

func (s *Server) cookieName() string {
	if s.Secure {
		return "__Host-stunsails"
	}
	return "stunsails"
}

func hashToken(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// current reads and refreshes the session cookie; ok is false when signed out.
func (s *Server) current(r *http.Request) (sess, bool) {
	if viaSocket(r) {
		return s.proxied(r)
	}
	c, err := r.Cookie(s.cookieName())
	if err != nil || len(c.Value) != 64 {
		return sess{}, false
	}
	h := hashToken(c.Value)
	x, ok := s.Store.Session(h)
	if !ok {
		return sess{}, false
	}
	now := s.now()
	if now.Sub(x.Seen) > collector.SessionIdle || now.Sub(x.Created) > collector.SessionMax {
		s.Store.DeleteSession(h)
		return sess{}, false
	}
	if now.Sub(x.Seen) > time.Minute {
		s.Store.TouchSession(h, now)
	}
	return sess{hash: h, stepUp: x.StepUp}, true
}

type handler func(w http.ResponseWriter, r *http.Request, se sess)

func (s *Server) authed(h handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		se, ok := s.current(r)
		if !ok {
			fail(w, http.StatusUnauthorized, "signed out")
			return
		}
		h(w, r, se)
	})
}

// mutating guards changes: a session, the X-StunSails header (a page on another
// site can't set it without a CORS preflight, which StunSails never answers), and
// when stepUp is set, an authenticator code entered within StepUpWindow.
func (s *Server) mutating(h handler, stepUp bool) http.Handler {
	return s.authed(func(w http.ResponseWriter, r *http.Request, se sess) {
		if r.Header.Get("X-StunSails") != "1" {
			fail(w, http.StatusForbidden, "missing X-StunSails header")
			return
		}
		if stepUp && s.now().Sub(se.stepUp) > StepUpWindow {
			writeJSON(w, http.StatusForbidden, map[string]string{"error": "Enter a fresh authenticator code to make changes.", "code": "stepup_required"})
			return
		}
		h(w, r, se)
	})
}

// clientIP trusts forwarding headers only from the local reverse proxy.
// Cloudflare's header wins; otherwise the address Caddy appended last.
func clientIP(r *http.Request) string {
	if viaSocket(r) {
		return proxyIP(r)
	}
	h, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		h = r.RemoteAddr
	}
	if ip := net.ParseIP(h); ip == nil || !ip.IsLoopback() {
		return h
	}
	if cf := strings.TrimSpace(r.Header.Get("Cf-Connecting-Ip")); net.ParseIP(cf) != nil {
		return cf
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
			return last
		}
	}
	return h
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

// failDocker maps a daemon error to a status the page can show as is.
func failDocker(w http.ResponseWriter, err error) {
	var de *docker.Error
	if errors.As(err, &de) {
		code := de.Status
		if code < 400 || code > 499 {
			code = http.StatusBadGateway
		}
		fail(w, code, de.Message)
		return
	}
	fail(w, http.StatusBadGateway, err.Error())
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		fail(w, http.StatusBadRequest, "request body is not valid JSON")
		return false
	}
	return true
}

func (s *Server) audit(r *http.Request, typ, action, target, name string, err error) {
	a := store.Activity{Kind: "audit", Type: typ, Action: action, Target: target, Name: name, IP: clientIP(r), OK: err == nil}
	if err != nil {
		a.Detail = err.Error()
	}
	if viaSocket(r) {
		if se, ok := s.proxied(r); ok {
			a.Detail = strings.TrimSpace(a.Detail + " (by " + se.actor + ")")
		}
	}
	s.Store.AddActivity(a)
}

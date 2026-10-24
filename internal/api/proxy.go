package api

import (
	"context"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Behind an app. Besides its own TCP listener, StunSails can serve one local
// app over a unix socket (SocketHandler, and SocketListener in
// proxy_linux.go). That app signs people in itself, so requests over the socket
// say who is asking (X-Proxy-User) and when that person last entered a fresh
// authenticator code (X-Proxy-StepUp, unix seconds). StunSails believes them
// because the kernel names the user that opened the connection and only the
// allowed ones are let in. The TCP listener is unchanged and keeps its own
// sign-in, so both can run side by side.

type viaSocketKey struct{}

// SocketHandler is Handler for the socket: every request on it is the app's.
func (s *Server) SocketHandler() http.Handler {
	h := s.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), viaSocketKey{}, true)))
	})
}

func viaSocket(r *http.Request) bool {
	v, _ := r.Context().Value(viaSocketKey{}).(bool)
	return v
}

// proxied is the session the app vouches for: none without X-Proxy-User.
func (s *Server) proxied(r *http.Request) (sess, bool) {
	actor := strings.TrimSpace(r.Header.Get("X-Proxy-User"))
	if actor == "" || len(actor) > 254 {
		return sess{}, false
	}
	se := sess{actor: actor}
	if at, err := strconv.ParseInt(r.Header.Get("X-Proxy-StepUp"), 10, 64); err == nil && at > 0 {
		se.stepUp = time.Unix(at, 0)
	}
	return se, true
}

// signInElsewhere answers the sign-in endpoints on the socket: the app in
// front does that.
func signInElsewhere(w http.ResponseWriter) {
	fail(w, http.StatusNotFound, "signing in is handled by the app in front of StunSails")
}

// proxyIP is the client's address as the app forwards it.
func proxyIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		if last := strings.TrimSpace(parts[len(parts)-1]); net.ParseIP(last) != nil {
			return last
		}
	}
	return "app"
}

// alertFeed serves GET /api/alerts/feed?after=N: the alerts raised since seq N,
// for the app in front to turn into its own notifications.
func (s *Server) alertFeed(w http.ResponseWriter, r *http.Request, se sess) {
	after, _ := strconv.ParseInt(r.URL.Query().Get("after"), 10, 64)
	items, last := s.Alerts.Feed(after)
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "last": last, "started": s.Started.Unix()})
}

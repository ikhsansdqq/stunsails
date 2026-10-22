package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"server-monitor/internal/auth"
	"server-monitor/internal/store"
)

func newServer(t *testing.T) (*Server, *auth.TOTP, *time.Time) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	secret := auth.NewSecret()
	tp, _ := auth.ParseSecret(secret)
	check, _ := auth.ParseSecret(secret) // a twin to make codes without spending them
	hash, _ := auth.HashPassword("correct horse battery")
	now := time.Unix(1_800_000_000, 0)
	s := &Server{Store: st, TOTP: tp, Password: hash, Limiter: auth.NewLimiter(), Secure: true,
		Static: fstest.MapFS{"index.html": {Data: []byte("<!doctype html>app")}}, Now: func() time.Time { return now }}
	return s, check, &now
}

func do(h http.Handler, method, path, body string, cookies []*http.Cookie, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = "127.0.0.1:5555"
	for _, c := range cookies {
		r.AddCookie(c)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestLoginFlowAndGuards(t *testing.T) {
	s, codes, now := newServer(t)
	h := s.Handler()

	if w := do(h, "GET", "/api/containers", "", nil, nil); w.Code != 401 {
		t.Fatalf("signed out read: %d", w.Code)
	}
	if w := do(h, "POST", "/api/login/totp", `{"code":"123456"}`, nil, nil); w.Code != 401 {
		t.Fatalf("code before password: %d", w.Code)
	}
	if w := do(h, "POST", "/api/login", `{"password":"nope"}`, nil, nil); w.Code != 401 {
		t.Fatalf("wrong password: %d", w.Code)
	}
	w := do(h, "POST", "/api/login", `{"password":"correct horse battery"}`, nil, nil)
	if w.Code != 200 {
		t.Fatalf("password: %d %s", w.Code, w.Body)
	}
	pending := w.Result().Cookies()
	if w := do(h, "POST", "/api/login/totp", `{"code":"000000"}`, pending, nil); w.Code != 401 {
		t.Fatalf("wrong code: %d", w.Code)
	}
	w = do(h, "POST", "/api/login/totp", `{"code":"`+codes.Code(*now)+`"}`, pending, nil)
	if w.Code != 200 {
		t.Fatalf("code: %d %s", w.Code, w.Body)
	}
	var sc []*http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "__Host-harbor" {
			if !c.HttpOnly || !c.Secure || c.SameSite != http.SameSiteStrictMode {
				t.Fatal("session cookie flags wrong")
			}
			sc = append(sc, c)
		}
	}
	if len(sc) != 1 {
		t.Fatal("no session cookie")
	}
	w = do(h, "GET", "/api/session", "", sc, nil)
	var info map[string]any
	json.Unmarshal(w.Body.Bytes(), &info)
	if info["authenticated"] != true {
		t.Fatalf("session: %s", w.Body)
	}
	if w := do(h, "GET", "/api/activity", "", sc, nil); w.Code != 200 || !strings.Contains(w.Body.String(), "signed in") {
		t.Fatalf("activity: %d %s", w.Code, w.Body)
	}

	// Changes need the header…
	if w := do(h, "POST", "/api/prune/nonsense", "", sc, nil); w.Code != 403 {
		t.Fatalf("no header: %d", w.Code)
	}
	// …and a code from the last five minutes. Sign-in counts, so move past it.
	*now = now.Add(6 * time.Minute)
	w = do(h, "POST", "/api/prune/images", "", sc, map[string]string{"X-Harbor": "1"})
	if w.Code != 403 || !strings.Contains(w.Body.String(), "stepup_required") {
		t.Fatalf("stale step-up: %d %s", w.Code, w.Body)
	}
	if w := do(h, "POST", "/api/stepup", `{"code":"`+codes.Code(*now)+`"}`, sc, map[string]string{"X-Harbor": "1"}); w.Code != 200 {
		t.Fatalf("step-up: %d %s", w.Code, w.Body)
	}
	if w := do(h, "DELETE", "/api/networks/bridge", "", sc, map[string]string{"X-Harbor": "1"}); w.Code != 409 {
		t.Fatalf("after step-up, built-in network refusal expected: %d %s", w.Code, w.Body)
	}

	// Sign out ends the session.
	do(h, "POST", "/api/logout", "", sc, nil)
	if w := do(h, "GET", "/api/containers", "", sc, nil); w.Code != 401 {
		t.Fatalf("after logout: %d", w.Code)
	}
}

func TestLockout(t *testing.T) {
	s, _, _ := newServer(t)
	h := s.Handler()
	for range 5 {
		do(h, "POST", "/api/login", `{"password":"x"}`, nil, map[string]string{"Cf-Connecting-Ip": "203.0.113.9"})
	}
	if w := do(h, "POST", "/api/login", `{"password":"correct horse battery"}`, nil, map[string]string{"Cf-Connecting-Ip": "203.0.113.9"}); w.Code != 429 {
		t.Fatalf("want 429 after 5 failures, got %d", w.Code)
	}
	if w := do(h, "POST", "/api/login", `{"password":"correct horse battery"}`, nil, map[string]string{"Cf-Connecting-Ip": "198.51.100.1"}); w.Code != 200 {
		t.Fatalf("other address: %d", w.Code)
	}
}

func TestClientIP(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	r.RemoteAddr = "198.51.100.7:1234"
	r.Header.Set("Cf-Connecting-Ip", "1.1.1.1")
	if clientIP(r) != "198.51.100.7" {
		t.Fatal("forwarding headers from a non-loopback peer must be ignored")
	}
	r.RemoteAddr = "127.0.0.1:1"
	if clientIP(r) != "1.1.1.1" {
		t.Fatal("Cloudflare header from the local proxy should win")
	}
	r.Header.Del("Cf-Connecting-Ip")
	r.Header.Set("X-Forwarded-For", "6.6.6.6, 9.9.9.9")
	if clientIP(r) != "9.9.9.9" {
		t.Fatal("want the hop the proxy appended")
	}
}

func TestStaticFallback(t *testing.T) {
	s, _, _ := newServer(t)
	w := do(s.Handler(), "GET", "/containers/abc", "", nil, nil)
	if w.Code != 200 || !strings.Contains(w.Body.String(), "app") || w.Header().Get("Content-Security-Policy") == "" {
		t.Fatalf("spa fallback: %d %s", w.Code, w.Body)
	}
}

func TestProcCPU(t *testing.T) {
	p := filepath.Join(t.TempDir(), "stat")
	// pid (comm with) spaces) state ppid ... utime=14th field, stime=15th
	os.WriteFile(p, []byte("123 (server mon) itor) S 1 2 3 4 5 6 7 8 9 10 250 50 0 0 20 0 8 0\n"), 0o644)
	d, ok := procCPU(p)
	if !ok || d != 3*time.Second {
		t.Fatalf("got %v %v, want 3s", d, ok)
	}
}

func TestLegacySources(t *testing.T) {
	got := legacySources("106.12.9.37 (10 attempts), 89.126.211.166 (6 attempts), +13 more · users tried: root, admin")
	if len(got) != 2 || got[0].IP != "106.12.9.37" || got[0].Attempts != 10 || got[1].Attempts != 6 {
		t.Fatalf("%+v", got)
	}
}

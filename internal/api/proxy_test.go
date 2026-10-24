package api

import (
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/alert"
)

func TestSocketTrustsTheAppHeaders(t *testing.T) {
	s, _, now := newServer(t)
	s.Alerts = alert.New(s.Store, "test")
	tcp, sock := s.Handler(), s.SocketHandler()
	who := map[string]string{"X-Proxy-User": "owner@example.com"}

	// Over TCP the headers mean nothing: StunSails' own sign-in still applies.
	if w := do(tcp, "GET", "/api/activity", "", nil, who); w.Code != 401 {
		t.Fatalf("tcp with proxy headers: %d", w.Code)
	}
	if w := do(sock, "GET", "/api/activity", "", nil, nil); w.Code != 401 {
		t.Fatalf("socket without a user: %d", w.Code)
	}
	if w := do(sock, "GET", "/api/alerts/feed", "", nil, who); w.Code != 200 {
		t.Fatalf("socket read: %d %s", w.Code, w.Body)
	}
	var se map[string]any
	w := do(sock, "GET", "/api/session", "", nil, who)
	json.Unmarshal(w.Body.Bytes(), &se)
	if se["authenticated"] != true || se["configured"] != true {
		t.Fatalf("session over the socket: %v", se)
	}
	for _, p := range []string{"/api/login", "/api/login/totp", "/api/logout"} {
		if w := do(sock, "POST", p, `{}`, nil, who); w.Code != 404 {
			t.Fatalf("%s over the socket: %d", p, w.Code)
		}
	}

	// Changes still need the header and a fresh code, now vouched for by the app.
	change := map[string]string{"X-Proxy-User": "owner@example.com", "X-StunSails": "1"}
	if w := do(sock, "POST", "/api/sites", `{}`, nil, change); w.Code != 403 {
		t.Fatalf("change without step-up: %d", w.Code)
	}
	change["X-Proxy-StepUp"] = strconv.FormatInt(now.Add(-StepUpWindow-time.Second).Unix(), 10)
	if w := do(sock, "POST", "/api/sites", `{}`, nil, change); w.Code != 403 {
		t.Fatalf("change with a stale step-up: %d", w.Code)
	}
	change["X-Proxy-StepUp"] = strconv.FormatInt(now.Add(-time.Minute).Unix(), 10)
	if w := do(sock, "POST", "/api/sites", `not json`, nil, change); w.Code != 400 {
		t.Fatalf("change with a fresh step-up was refused: %d %s", w.Code, w.Body)
	}
	if w := do(sock, "POST", "/api/stepup", `{"code":"123456"}`, nil, change); w.Code != 404 {
		t.Fatalf("step-up over the socket: %d", w.Code)
	}
}

func TestSocketListenerChecksTheUser(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct {
		allow []int
		ok    bool
	}{{[]int{os.Getuid()}, true}, {[]int{os.Getuid() + 1}, false}} {
		path := filepath.Join(dir, "s.sock")
		ln, err := SocketListener(path, -1, tc.allow)
		if err != nil {
			t.Fatal(err)
		}
		srv := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("hi")) })}
		go srv.Serve(ln)
		st, _ := os.Stat(path)
		if st.Mode().Perm() != 0o660 {
			t.Fatalf("socket mode %v", st.Mode().Perm())
		}
		conn, err := net.Dial("unix", path)
		if err != nil {
			t.Fatal(err)
		}
		conn.SetDeadline(time.Now().Add(2 * time.Second))
		conn.Write([]byte("GET / HTTP/1.0\r\n\r\n"))
		buf := make([]byte, 64)
		n, _ := conn.Read(buf)
		conn.Close()
		srv.Close()
		if got := n > 0; got != tc.ok {
			t.Fatalf("allow %v: answered=%v, want %v", tc.allow, got, tc.ok)
		}
	}
}

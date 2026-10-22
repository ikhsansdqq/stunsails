package uptime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"server-monitor/internal/store"
)

func TestParseCaddyfile(t *testing.T) {
	cf := `{
	email me@example.com
}

(common) {
	encode gzip
}

app.example.com {
	reverse_proxy 127.0.0.1:8080 {
		flush_interval -1
	}
}
# old.example.com {
ems.example.com, www.ems.example.com {
	import common
}
*.wild.example.com {
	tls internal
}
:8080 {
	respond "hi"
}
https://shop.example.com:443 {
}
`
	got := ParseCaddyfile(strings.NewReader(cf))
	want := []string{"app.example.com", "ems.example.com", "www.ems.example.com", "shop.example.com"}
	if !slices.Equal(got, want) {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestProbe(t *testing.T) {
	ok := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer ok.Close()
	bad := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(502) }))
	defer bad.Close()
	host := func(s *httptest.Server) string { return strings.TrimPrefix(s.URL, "https://") }
	now := time.Unix(1_800_000_000, 0)
	r := Probe(context.Background(), ok.Client(), host(ok), now)
	if !r.Up || r.Status != 404 || r.CertExpiry == 0 {
		t.Fatalf("404 counts as up with a cert: %+v", r)
	}
	r = Probe(context.Background(), bad.Client(), host(bad), now)
	if r.Up || r.Status != 502 {
		t.Fatalf("502 is down: %+v", r)
	}
	// An untrusted certificate is down, but its expiry is still read.
	r = Probe(context.Background(), &http.Client{}, host(ok), now)
	if r.Up || r.Error == "" || r.CertExpiry == 0 {
		t.Fatalf("untrusted cert: %+v", r)
	}
}

func TestManualSites(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	c := &Checker{Store: st, Caddyfile: "/nonexistent", HTTP: &http.Client{Timeout: time.Millisecond}}
	c.last, c.fails = map[string]store.SiteCheck{}, map[string]int{}
	if err := c.Add("https://Shop.Example.com/"); err != nil {
		t.Fatal(err)
	}
	if err := c.Add("shop.example.com"); err == nil {
		t.Fatal("duplicate should fail")
	}
	if err := c.Add("not a host"); err == nil {
		t.Fatal("invalid host should fail")
	}
	if s := c.Sites(); len(s) != 1 || s[0].Host != "shop.example.com" || s[0].Source != "manual" {
		t.Fatalf("sites %+v", s)
	}
	if err := c.Remove("shop.example.com"); err != nil || len(c.Manual()) != 0 {
		t.Fatal("remove failed")
	}
}

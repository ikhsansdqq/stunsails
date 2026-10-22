package geo

import (
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestLookupWithoutFiles(t *testing.T) {
	d := &DB{Dir: t.TempDir()}
	d.Open()
	if d.Ready() {
		t.Fatal("no files, not ready")
	}
	for _, ip := range []string{"10.0.0.1", "127.0.0.1", "192.168.1.5", "100.64.3.2", "::1", "::ffff:10.1.1.1"} {
		if !d.Lookup(ip).Private {
			t.Fatalf("%s is private", ip)
		}
	}
	if got := d.Lookup("106.12.9.37"); got != (Info{}) {
		t.Fatalf("no database, empty info: %+v", got)
	}
	if got := d.Lookup("not an ip"); got != (Info{}) {
		t.Fatalf("garbage: %+v", got)
	}
}

func TestRefreshRejectsBadFile(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		zw := gzip.NewWriter(w)
		zw.Write([]byte("this is not an mmdb file"))
		zw.Close()
	}))
	defer srv.Close()
	dir := t.TempDir()
	d := &DB{Dir: dir, URL: func(kind, month string) string { return srv.URL + "/" + kind + "-" + month }, Now: func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }}
	d.Refresh(context.Background())
	if d.Err() == "" || d.Ready() {
		t.Fatalf("a bad download must fail and load nothing: %q", d.Err())
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 0 {
		t.Fatalf("temp files left behind: %v", entries)
	}
}

func TestRefreshFallsBackToLastMonth(t *testing.T) {
	var asked []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = append(asked, r.URL.Path)
		http.NotFound(w, r)
	}))
	defer srv.Close()
	d := &DB{Dir: t.TempDir(), URL: func(kind, month string) string { return srv.URL + "/" + kind + "-" + month }, Now: func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }}
	d.Refresh(context.Background())
	if len(asked) != 4 || asked[0] != "/city-2026-10" || asked[1] != "/city-2026-09" {
		t.Fatalf("asked %v", asked)
	}
}

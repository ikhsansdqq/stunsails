package store

import (
	"database/sql"
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/host"
)

func TestForecast(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	start := time.Unix(1_800_000_000, 0)
	// 100 GB disk, 50 GB used, growing 1 GB a day for 3 days, a reading every 10 min.
	const gb = 1 << 30
	for i := 0; i <= 3*144; i++ {
		at := start.Add(time.Duration(i) * 10 * time.Minute)
		used := int64(50*gb + float64(i)/144*gb)
		st.AddHost(host.Sample{At: at.Unix(), DiskUsed: used, DiskTotal: 100 * gb})
	}
	days, err := st.DiskForecast(start.Unix())
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(days-47) > 0.5 {
		t.Fatalf("want ~47 days left, got %.2f", days)
	}
	// Under a day of readings: no forecast.
	if d, _ := st.DiskForecast(start.Add(3*24*time.Hour - time.Hour).Unix()); d != -1 {
		t.Fatalf("want -1 with too few readings, got %v", d)
	}
}

func TestSiteChecks(t *testing.T) {
	st, _ := Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	for i, up := range []bool{true, true, false, true} {
		st.AddSiteCheck("a.example", SiteCheck{At: int64(60 * i), Up: up, Status: 200, Ms: 100})
	}
	if u := st.SiteUptime("a.example", 0); u != 75 {
		t.Fatalf("uptime %v", u)
	}
	if u := st.SiteUptime("b.example", 0); u != -1 {
		t.Fatalf("no checks should be -1, got %v", u)
	}
	h, _ := st.SiteHistory("a.example", 0, 120)
	if len(h) != 2 || !h[0].Up || h[1].Up {
		t.Fatalf("history %+v", h)
	}
}

func TestActivityPage(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for i := range 30 {
		s.AddActivity(Activity{At: int64(1000 + i), Kind: "host", Type: "ssh", Action: "signed in", Name: "ubuntu", IP: "203.0.113.1"})
	}
	s.AddActivity(Activity{At: 2000, Kind: "host", Type: "ssh", Action: "failed logins", Name: "x", Sources: []Source{{IP: "198.51.100.7", Attempts: 9, Users: []string{"root"}}}})
	items, total, err := s.ActivityPage(ActivityQuery{Kind: "host", Limit: 25}, 25, false)
	if err != nil || total != 31 || len(items) != 6 || items[0].At != 1005 {
		t.Fatalf("page 2: %v total=%d n=%d %+v", err, total, len(items), items)
	}
	items, _, _ = s.ActivityPage(ActivityQuery{Limit: 25}, 0, true)
	if items[0].At != 1000 {
		t.Fatalf("oldest first: %+v", items[0])
	}
	_, total, _ = s.ActivityPage(ActivityQuery{Q: "198.51", Limit: 25}, 0, false)
	if total != 0 {
		t.Fatal("sources are not searched; detail and ip are")
	}
	newest, _, _ := s.ActivityPage(ActivityQuery{Limit: 25}, 0, false)
	a, ok, _ := s.ActivityByID(newest[0].ID)
	if !ok || len(a.Sources) != 1 || a.Sources[0].Attempts != 9 || newest[0].Sources != nil {
		t.Fatalf("by id %+v, list %+v", a, newest[0])
	}
	if _, ok, _ := s.ActivityByID(99999); ok {
		t.Fatal("missing id")
	}
}

func TestMigrateOldActivity(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	db, _ := sql.Open("sqlite", "file:"+path)
	db.Exec(`CREATE TABLE activity (id INTEGER PRIMARY KEY AUTOINCREMENT, at INTEGER NOT NULL, kind TEXT NOT NULL, type TEXT NOT NULL,
		action TEXT NOT NULL, target TEXT NOT NULL DEFAULT '', name TEXT NOT NULL DEFAULT '', ip TEXT NOT NULL DEFAULT '',
		ok INTEGER NOT NULL DEFAULT 1, detail TEXT NOT NULL DEFAULT '')`)
	db.Exec(`INSERT INTO activity (at, kind, type, action) VALUES (1, 'host', 'ssh', 'failed logins')`)
	db.Close()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if a, ok, err := s.ActivityByID(1); !ok || err != nil || a.Sources != nil {
		t.Fatalf("old row after migration: %v %v %+v", ok, err, a)
	}
	s.Close()
	if s, err = Open(path); err != nil {
		t.Fatal("opening twice must not migrate twice:", err)
	}
}

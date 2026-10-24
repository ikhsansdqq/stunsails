package hostlog

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stunsails/stunsails/internal/store"
)

func TestParse(t *testing.T) {
	now := time.Unix(1_800_000_000, 0)
	cases := []struct {
		line string
		want Line
		ok   bool
	}{
		{"2026-10-18T12:43:55.056155+08:00 localhost sshd[1758690]: Invalid user developer from 203.0.113.9 port 38346", Line{Kind: "failed", User: "developer", From: "203.0.113.9"}, true},
		{"2026-10-18T12:43:57.101801+08:00 localhost sshd[1758690]: Failed password for invalid user developer from 203.0.113.9 port 38346 ssh2", Line{}, false},
		{"2026-10-18T12:44:00.000000+08:00 localhost sshd[1]: Failed password for root from 198.51.100.4 port 2 ssh2", Line{Kind: "failed", User: "root", From: "198.51.100.4", Method: "password"}, true},
		{"2026-10-18T12:45:00.000000+08:00 localhost sshd[2]: Accepted publickey for ubuntu from 192.0.2.1 port 5 ssh2: ED25519 SHA256:x", Line{Kind: "accepted", User: "ubuntu", From: "192.0.2.1", Method: "publickey"}, true},
		{"2026-10-18T12:42:33.106959+08:00 localhost sudo:   ubuntu : PWD=/home/ubuntu ; USER=root ; COMMAND=/usr/bin/docker ps", Line{Kind: "sudo", User: "ubuntu", Cmd: "/usr/bin/docker ps"}, true},
		{"2026-10-18T12:42:33.112328+08:00 localhost sudo: pam_unix(sudo:session): session opened for user root(uid=0) by ubuntu(uid=1000)", Line{}, false},
	}
	for _, c := range cases {
		got, ok := Parse(c.line, now)
		if ok != c.ok {
			t.Errorf("%q: ok=%v", c.line, ok)
			continue
		}
		if !ok {
			continue
		}
		if got.Kind != c.want.Kind || got.User != c.want.User || got.From != c.want.From || got.Cmd != c.want.Cmd || got.Method != c.want.Method {
			t.Errorf("%q: got %+v", c.line, got)
		}
		if got.At.Equal(now) {
			t.Errorf("%q: timestamp not parsed", c.line)
		}
	}
}

func TestTailFollowsRotation(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(filepath.Join(dir, "t.db"))
	defer st.Close()
	path := filepath.Join(dir, "auth.log")
	os.WriteFile(path, []byte("2026-10-18T10:00:00+08:00 h sshd[1]: Accepted password for old from 1.1.1.1 port 1 ssh2\n"), 0o644)
	w := &Watcher{Store: st, AuthLog: path, Every: 20 * time.Millisecond, fails: map[string]int{}, failUser: map[string]int{}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go w.tail(ctx)
	time.Sleep(60 * time.Millisecond)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	f.WriteString("2026-10-18T10:01:00+08:00 h sshd[2]: Accepted publickey for alice from 2.2.2.2 port 1 ssh2\n")
	f.Close()
	time.Sleep(80 * time.Millisecond)
	// logrotate: move away, new empty file at the path.
	os.Rename(path, path+".1")
	os.WriteFile(path, []byte("2026-10-18T10:02:00+08:00 h sudo:   bob : PWD=/ ; USER=root ; COMMAND=/bin/true\n"), 0o644)
	time.Sleep(120 * time.Millisecond)
	list, _ := st.Activity(store.ActivityQuery{Kind: "host"})
	names := []string{}
	for _, a := range list {
		names = append(names, a.Name)
	}
	got := strings.Join(names, ",")
	if got != "bob,alice" {
		t.Fatalf("want lines after start and after rotation only, got %q", got)
	}
}

func TestSummary(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	w := &Watcher{Store: st, fails: map[string]int{}, failUser: map[string]int{}}
	for i := 0; i < 5; i++ {
		w.record(Line{Kind: "failed", User: "root", From: "203.0.113.9"})
	}
	w.record(Line{Kind: "failed", User: "admin", From: "198.51.100.4"})
	w.summarise(time.Unix(1_800_000_000, 0))
	list, _ := st.Activity(store.ActivityQuery{})
	if len(list) != 1 || list[0].Name != "6 failed SSH logins from 2 addresses" || !strings.HasPrefix(list[0].Detail, "203.0.113.9 (5 attempts)") {
		t.Fatalf("summary %+v", list)
	}
	a, ok, _ := st.ActivityByID(list[0].ID)
	if !ok || len(a.Sources) != 2 || a.Sources[0].IP != "203.0.113.9" || a.Sources[0].Attempts != 5 ||
		strings.Join(a.Sources[0].Users, ",") != "root" || a.Sources[1].IP != "198.51.100.4" || a.Sources[1].Users[0] != "admin" {
		t.Fatalf("sources %+v", a.Sources)
	}
	w.summarise(time.Unix(1_800_000_600, 0))
	if list, _ := st.Activity(store.ActivityQuery{}); len(list) != 1 {
		t.Fatal("a quiet window adds nothing")
	}
}

func TestParseUnits(t *testing.T) {
	u, err := ParseUnits([]byte(`[{"unit":"x.service","load":"loaded","active":"failed","sub":"failed","description":"X"},{"unit":"y.service","active":"active"}]`))
	if err != nil || len(u) != 1 || u[0].Unit != "x.service" {
		t.Fatalf("%v %+v", err, u)
	}
}

package cleanup

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/docker"
	"github.com/ikhsansdqq/stunsails/internal/store"
)

func TestDue(t *testing.T) {
	sun4 := time.Date(2026, 10, 18, 4, 30, 0, 0, time.Local) // a Sunday
	c := Config{Enabled: true, Weekday: 0, Hour: 4}
	if !Due(c, sun4) {
		t.Fatal("should be due")
	}
	c.LastRun = sun4.Add(-time.Hour).Unix()
	if Due(c, sun4) {
		t.Fatal("ran an hour ago")
	}
	c.LastRun = 0
	if Due(c, sun4.Add(time.Hour)) || Due(c, sun4.Add(24*time.Hour)) {
		t.Fatal("wrong hour or day")
	}
	c.Enabled = false
	if Due(c, sun4) {
		t.Fatal("off")
	}
}

func TestSave(t *testing.T) {
	st, _ := store.Open(filepath.Join(t.TempDir(), "t.db"))
	defer st.Close()
	s := &Scheduler{Store: st}
	if c := s.Config(); c.Hour != 4 || len(c.Kinds) != 2 || c.Enabled {
		t.Fatalf("defaults %+v", c)
	}
	if err := s.Save(Config{Enabled: true, Weekday: 3, Hour: 2, Kinds: []docker.PruneKind{docker.PruneImages}}); err == nil {
		t.Fatal("full image prunes can't be scheduled")
	}
	if err := s.Save(Config{Enabled: true, Weekday: 3, Hour: 2}); err == nil {
		t.Fatal("needs a kind")
	}
	if err := s.Save(Config{Enabled: true, Weekday: 3, Hour: 2, Kinds: []docker.PruneKind{docker.PruneBuildCache}, LastRun: 99}); err != nil {
		t.Fatal(err)
	}
	if c := s.Config(); !c.Enabled || c.Weekday != 3 || c.LastRun != 0 {
		t.Fatalf("saved %+v (LastRun must not be settable)", c)
	}
}

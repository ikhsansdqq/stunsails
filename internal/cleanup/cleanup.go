// Package cleanup prunes what Docker no longer needs on a weekly schedule the
// owner sets in StunSails, and on demand.
package cleanup

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stunsails/stunsails/internal/alert"
	"github.com/stunsails/stunsails/internal/docker"
	"github.com/stunsails/stunsails/internal/store"
)

type Config struct {
	Enabled bool               `json:"enabled"`
	Weekday int                `json:"weekday"` // 0 is Sunday
	Hour    int                `json:"hour"`    // server's local time
	Kinds   []docker.PruneKind `json:"kinds"`
	LastRun int64              `json:"last_run"`
	Last    string             `json:"last_result"`
}

// Allowed kinds: everything but full image prunes, which would remove tagged
// images a stopped stack still needs.
var Allowed = []docker.PruneKind{docker.PruneBuildCache, docker.PruneDangling, docker.PruneContainers, docker.PruneNetworks}

type Result struct {
	Kind      docker.PruneKind `json:"kind"`
	Removed   int              `json:"removed"`
	Reclaimed int64            `json:"reclaimed"`
	Error     string           `json:"error,omitempty"`
}

type Scheduler struct {
	Docker *docker.Client
	Store  *store.Store
	Alerts *alert.Notifier
	OnRun  func() // after any run, e.g. to drop cached disk sizes

	mu      sync.Mutex
	running bool
}

const metaKey = "cleanup"

func (s *Scheduler) Config() Config {
	c := Config{Weekday: 0, Hour: 4, Kinds: []docker.PruneKind{docker.PruneBuildCache, docker.PruneDangling}}
	if raw := s.Store.Meta(metaKey); raw != "" {
		json.Unmarshal([]byte(raw), &c)
	}
	if c.Kinds == nil {
		c.Kinds = []docker.PruneKind{}
	}
	return c
}

func (s *Scheduler) save(c Config) {
	b, _ := json.Marshal(c)
	s.Store.SetMeta(metaKey, string(b))
}

// Save updates the schedule, keeping the record of the last run.
func (s *Scheduler) Save(in Config) error {
	if in.Weekday < 0 || in.Weekday > 6 || in.Hour < 0 || in.Hour > 23 {
		return errors.New("pick a day and an hour")
	}
	for _, k := range in.Kinds {
		if !slices.Contains(Allowed, k) {
			return fmt.Errorf("%q can't be scheduled", k)
		}
	}
	if in.Enabled && len(in.Kinds) == 0 {
		return errors.New("pick at least one thing to clean")
	}
	c := s.Config()
	c.Enabled, c.Weekday, c.Hour, c.Kinds = in.Enabled, in.Weekday, in.Hour, in.Kinds
	s.save(c)
	return nil
}

// Due reports whether a run is due at now: the right day and hour, and no run
// in the last 12 hours.
func Due(c Config, now time.Time) bool {
	return c.Enabled && int(now.Weekday()) == c.Weekday && now.Hour() == c.Hour && now.Unix()-c.LastRun > 12*3600
}

func (s *Scheduler) Run(ctx context.Context) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			c := s.Config()
			if Due(c, now) {
				s.RunNow(ctx, c.Kinds, "schedule")
			}
		}
	}
}

var ErrBusy = errors.New("a cleanup is already running")

// RunNow prunes kinds one after another and records the outcome.
func (s *Scheduler) RunNow(ctx context.Context, kinds []docker.PruneKind, by string) ([]Result, error) {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil, ErrBusy
	}
	s.running = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.running = false
		s.mu.Unlock()
	}()
	results := []Result{}
	var total int64
	removed := 0
	failed := []string{}
	for _, k := range kinds {
		if !slices.Contains(Allowed, k) {
			continue
		}
		pctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		r, err := s.Docker.Prune(pctx, k)
		cancel()
		res := Result{Kind: k, Removed: r.Removed, Reclaimed: r.Reclaimed}
		if err != nil {
			res.Error = err.Error()
			failed = append(failed, string(k))
		}
		total += r.Reclaimed
		removed += r.Removed
		results = append(results, res)
	}
	summary := fmt.Sprintf("Removed %d items, freed %s", removed, bytes(total))
	if len(failed) > 0 {
		summary += "; failed: " + strings.Join(failed, ", ")
	}
	c := s.Config()
	c.LastRun, c.Last = time.Now().Unix(), summary
	s.save(c)
	s.Store.AddActivity(store.Activity{Kind: "audit", Type: "action", Action: "cleanup (" + by + ")", Name: summary, OK: len(failed) == 0})
	if s.Alerts != nil && by == "schedule" {
		s.Alerts.Once(alert.RuleCleanup, "weekly", "Weekly cleanup: freed "+bytes(total), summary+".", "info")
	}
	if s.OnRun != nil {
		s.OnRun()
	}
	return results, nil
}

func bytes(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%d KB", n>>10)
	}
	return fmt.Sprintf("%d B", n)
}

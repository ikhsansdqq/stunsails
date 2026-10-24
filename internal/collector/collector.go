// Package collector samples the host and every running container on a fixed
// tick, keeps the last hour in memory for live charts, rolls readings up to
// one-minute rows in SQLite, and records Docker's event stream as activity.
package collector

import (
	"context"
	"log"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stunsails/stunsails/internal/alert"
	"github.com/stunsails/stunsails/internal/cgroup"
	"github.com/stunsails/stunsails/internal/docker"
	"github.com/stunsails/stunsails/internal/host"
	"github.com/stunsails/stunsails/internal/store"
)

const (
	Tick     = 10 * time.Second
	ringSize = 360 // one hour at Tick
)

// CtrLive is the latest reading of one running container.
type CtrLive struct {
	CPUPct   float64 `json:"cpu"` // percent of one core, as `docker stats`
	Mem      int64   `json:"mem"`
	MemLimit int64   `json:"mem_limit"`
	IORead   float64 `json:"io_read"` // bytes per second
	IOWrite  float64 `json:"io_write"`
	Pids     int64   `json:"pids"`
}

type CtrPoint struct {
	At  int64   `json:"at"`
	CPU float64 `json:"cpu"`
	Mem int64   `json:"mem"`
}

type ctrState struct {
	name  string
	prev  cgroup.Reading
	at    time.Time
	live  CtrLive
	ring  []CtrPoint
	acc   []CtrPoint // this minute's readings
	valid bool       // live holds a rate (two readings seen)
}

type Collector struct {
	Docker  *docker.Client
	Store   *store.Store
	Sampler *host.Sampler
	Cgroup  string          // "/sys/fs/cgroup"
	Alerts  *alert.Notifier // optional

	health health // what the alert rules have seen; see watch.go

	mu       sync.RWMutex
	ring     []host.Sample
	acc      []host.Sample
	ctrs     map[string]*ctrState
	dockErr  string
	stacksAt time.Time
}

func (c *Collector) Run(ctx context.Context) {
	c.ctrs = map[string]*ctrState{}
	go c.watchEvents(ctx)
	t := time.NewTicker(Tick)
	defer t.Stop()
	lastPrune := time.Time{}
	c.health.init()
	c.tick(ctx, time.Now())
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			ctrs, hs := c.tick(ctx, now)
			c.evaluate(now, hs, ctrs)
			if now.Sub(lastPrune) > time.Hour {
				c.Store.Prune(now, SessionIdle, SessionMax)
				lastPrune = now
			}
		}
	}
}

// Session limits live here so pruning and the API agree on them.
const (
	SessionIdle = 12 * time.Hour
	SessionMax  = 7 * 24 * time.Hour
)

func minute(t int64) int64 { return t / 60 * 60 }

func (c *Collector) tick(ctx context.Context, now time.Time) ([]docker.Container, host.Sample) {
	hs := c.Sampler.Sample(now)

	dctx, cancel := context.WithTimeout(ctx, 8*time.Second)
	ctrs, err := c.Docker.Containers(dctx, true)
	cancel()

	c.mu.Lock()
	defer c.mu.Unlock()

	// The first reading has no rates yet; keep it out of the charts.
	if len(c.ring) > 0 || hs.CPUPct > 0 {
		c.ring = appendRing(c.ring, hs)
	}
	if len(c.acc) > 0 && minute(c.acc[0].At) != minute(hs.At) {
		c.Store.AddHost(avgHost(c.acc))
		c.acc = c.acc[:0]
	}
	if len(c.ring) > 0 {
		c.acc = append(c.acc, hs)
	}

	if err != nil {
		c.dockErr = err.Error()
		return nil, hs
	}
	c.dockErr = ""
	seen := map[string]bool{}
	stacks := map[string]store.StackRow{}
	for _, ct := range ctrs {
		if p := ct.Project; p != "" && ct.Labels[docker.LabelOneoff] != "True" {
			if wd := ct.Labels[docker.LabelWorkingDir]; wd != "" {
				stacks[p] = store.StackRow{Project: p, WorkingDir: wd,
					ConfigFiles: splitList(ct.Labels[docker.LabelConfig]), EnvFiles: splitList(ct.Labels[docker.LabelEnvFile]), LastSeen: now.Unix()}
			}
		}
		if ct.State != "running" {
			continue
		}
		seen[ct.ID] = true
		r, err := cgroup.Read(c.Cgroup, ct.ID)
		if err != nil {
			continue
		}
		st := c.ctrs[ct.ID]
		if st == nil {
			st = &ctrState{}
			c.ctrs[ct.ID] = st
		}
		st.name = ct.Name
		if !st.at.IsZero() {
			dt := now.Sub(st.at)
			if dt > 0 && r.CPUUsec >= st.prev.CPUUsec {
				st.live.CPUPct = float64(r.CPUUsec-st.prev.CPUUsec) / float64(dt.Microseconds()) * 100
				st.live.IORead = rate(r.IORead, st.prev.IORead, dt.Seconds())
				st.live.IOWrite = rate(r.IOWrite, st.prev.IOWrite, dt.Seconds())
				st.valid = true
			}
		}
		st.live.Mem, st.live.MemLimit, st.live.Pids = r.Mem, r.MemLimit, r.Pids
		st.prev, st.at = r, now
		if !st.valid {
			continue
		}
		p := CtrPoint{At: now.Unix(), CPU: st.live.CPUPct, Mem: st.live.Mem}
		st.ring = appendRing(st.ring, p)
		if len(st.acc) > 0 && minute(st.acc[0].At) != minute(p.At) {
			a := avgCtr(st.acc)
			c.Store.AddCtr(a.At, ct.ID, st.name, a.CPU, a.Mem)
			st.acc = st.acc[:0]
		}
		st.acc = append(st.acc, p)
	}
	for id := range c.ctrs {
		if !seen[id] {
			delete(c.ctrs, id)
		}
	}
	// Remember stacks once a minute; their labels rarely change.
	if now.Sub(c.stacksAt) >= time.Minute {
		for _, s := range stacks {
			c.Store.UpsertStack(s)
		}
		c.stacksAt = now
	}
	return ctrs, hs
}

func splitList(s string) []string {
	if s == "" {
		return []string{}
	}
	return strings.Split(s, ",")
}

func rate(cur, prev uint64, dt float64) float64 {
	if cur < prev || dt <= 0 {
		return 0
	}
	return float64(cur-prev) / dt
}

func appendRing[T any](r []T, v T) []T {
	if len(r) >= ringSize {
		r = slices.Delete(r, 0, len(r)-ringSize+1)
	}
	return append(r, v)
}

func avgHost(xs []host.Sample) host.Sample {
	out := xs[len(xs)-1]
	out.At = minute(out.At)
	var cpu, load, rx, tx, rd, wr float64
	var mem, swap int64
	for _, x := range xs {
		cpu += x.CPUPct
		load += x.Load1
		rx += x.NetRx
		tx += x.NetTx
		rd += x.DiskRead
		wr += x.DiskWrite
		mem += x.MemUsed
		swap += x.SwapUsed
	}
	n := float64(len(xs))
	out.CPUPct, out.Load1, out.NetRx, out.NetTx, out.DiskRead, out.DiskWrite = cpu/n, load/n, rx/n, tx/n, rd/n, wr/n
	out.MemUsed, out.SwapUsed = mem/int64(len(xs)), swap/int64(len(xs))
	return out
}

func avgCtr(xs []CtrPoint) CtrPoint {
	var cpu float64
	var mem int64
	for _, x := range xs {
		cpu += x.CPU
		mem += x.Mem
	}
	return CtrPoint{At: minute(xs[0].At), CPU: cpu / float64(len(xs)), Mem: mem / int64(len(xs))}
}

// HostRing is the last hour of host readings, oldest first.
func (c *Collector) HostRing() []host.Sample {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return slices.Clone(c.ring)
}

// Latest is the newest host reading, or false before the first rate is known.
func (c *Collector) Latest() (host.Sample, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.ring) == 0 {
		return host.Sample{}, false
	}
	return c.ring[len(c.ring)-1], true
}

func (c *Collector) DockerError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.dockErr
}

// Live returns the latest reading per running container id.
func (c *Collector) Live() map[string]CtrLive {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make(map[string]CtrLive, len(c.ctrs))
	for id, st := range c.ctrs {
		if st.valid {
			out[id] = st.live
		}
	}
	return out
}

func (c *Collector) CtrRing(id string) []CtrPoint {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if st := c.ctrs[id]; st != nil {
		return slices.Clone(st.ring)
	}
	return []CtrPoint{}
}

// noisy actions fire constantly (health-check execs, attaches, log copies) and
// would drown the feed; network connect/disconnect repeats every start and stop.
func keep(e docker.Event) bool {
	if docker.IsHelper(e.Attrs) {
		return false
	}
	a := e.Action
	switch {
	case strings.HasPrefix(a, "exec_"), a == "attach", a == "detach", a == "resize", a == "top",
		a == "archive-path", a == "extract-to-dir", a == "copy", a == "export", a == "commit",
		a == "connect", a == "disconnect", a == "mount", a == "unmount", a == "prune":
		return false
	}
	return true
}

func (c *Collector) watchEvents(ctx context.Context) {
	backoff := time.Second
	since := time.Now().Unix()
	for ctx.Err() == nil {
		start := time.Now()
		err := c.Docker.Events(ctx, since, func(e docker.Event) {
			since = e.Time + 1
			if !keep(e) {
				return
			}
			c.Store.AddActivity(toActivity(e))
			c.onEvent(e)
		})
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			log.Printf("docker events: %v", err)
		}
		if time.Since(start) > time.Minute {
			backoff = time.Second
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		backoff = min(backoff*2, time.Minute)
	}
}

func toActivity(e docker.Event) store.Activity {
	a := store.Activity{At: e.Time, Kind: "docker", Type: e.Type, Action: e.Action, Target: e.ID, Name: e.Attrs["name"], OK: true}
	// Image events name the repository only; the id is the full reference ("alpine:3.20").
	if a.Name == "" || (e.Type == "image" && strings.Contains(e.ID, ":") && !strings.HasPrefix(e.ID, "sha256:")) {
		a.Name = e.ID
	}
	var detail []string
	switch {
	case e.Type == "container" && e.Action == "die":
		// 137 and 143 are SIGKILL and SIGTERM: a stop or restart, not a crash.
		// Running out of memory also ends in 137, but Docker reports that as its own oom event.
		if code := e.Attrs["exitCode"]; code != "" && code != "0" {
			a.OK = code == "137" || code == "143"
			detail = append(detail, "exit code "+code)
		}
	case e.Type == "container" && e.Action == "oom":
		a.OK = false
	case e.Type == "container" && e.Action == "kill":
		if s := e.Attrs["signal"]; s != "" {
			detail = append(detail, "signal "+s)
		}
	case strings.HasPrefix(e.Action, "health_status"):
		a.OK = !strings.Contains(e.Action, "unhealthy")
	}
	if e.Type == "container" {
		if img := e.Attrs["image"]; img != "" {
			detail = append(detail, img)
		}
		if p := e.Attrs[docker.LabelProject]; p != "" {
			detail = append(detail, "stack "+p)
		}
	}
	a.Detail = strings.Join(detail, " · ")
	return a
}

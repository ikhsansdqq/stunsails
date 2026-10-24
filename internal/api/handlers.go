package api

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/stunsails/stunsails/internal/collector"
	"github.com/stunsails/stunsails/internal/compose"
	"github.com/stunsails/stunsails/internal/docker"
	"github.com/stunsails/stunsails/internal/host"
	"github.com/stunsails/stunsails/internal/hostlog"
	"github.com/stunsails/stunsails/internal/store"
)

func dctx(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), 15*time.Second)
}

// ContainerRow is a container with its latest reading, when running.
type ContainerRow struct {
	docker.Container
	Live    *collector.CtrLive `json:"live"`
	Looping bool               `json:"looping"`
}

func (s *Server) rows(ctrs []docker.Container) []ContainerRow {
	live := s.Collector.Live()
	looping := s.Collector.Looping()
	out := make([]ContainerRow, len(ctrs))
	for i, c := range ctrs {
		out[i] = ContainerRow{Container: c, Looping: looping[c.ID]}
		if l, ok := live[c.ID]; ok {
			out[i].Live = &l
		}
	}
	return out
}

func (s *Server) overview(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	out := map[string]any{"info": s.Host.Info(), "docker_error": ""}
	if hs, ok := s.Collector.Latest(); ok {
		out["host"] = hs
	}
	var (
		wg   sync.WaitGroup
		ver  docker.Version
		ctrs []docker.Container
		cerr error
	)
	wg.Add(2)
	go func() { defer wg.Done(); ver, _ = s.Docker.Version(ctx) }()
	go func() { defer wg.Done(); ctrs, cerr = s.Docker.Containers(ctx, true) }()
	wg.Wait()
	out["docker"] = ver
	if cerr != nil {
		out["docker_error"] = cerr.Error()
		ctrs = []docker.Container{}
	}
	known, _ := s.Store.Stacks()
	stacks := compose.Build(ctrs, known)
	type stackSum struct {
		Project   string `json:"project"`
		State     string `json:"state"`
		Running   int    `json:"running"`
		Total     int    `json:"total"`
		Unhealthy int    `json:"unhealthy"`
	}
	sums := make([]stackSum, 0, len(stacks))
	for _, st := range stacks {
		sums = append(sums, stackSum{st.Project, st.State, st.Running, st.Total, st.Unhealthy})
	}
	counts := map[string]int{"total": len(ctrs)}
	for _, c := range ctrs {
		counts[c.State]++
		if c.Health == "unhealthy" {
			counts["unhealthy"]++
		}
	}
	rows := s.rows(ctrs)
	looping := []ContainerRow{}
	for _, r := range rows {
		if r.Looping {
			looping = append(looping, r)
		}
	}
	counts["looping"] = len(looping)
	out["looping"] = looping
	out["disk_forecast_days"] = s.Collector.Forecast()
	out["failed_units"] = []hostlog.FailedUnit{}
	if s.HostLog != nil {
		out["failed_units"] = s.HostLog.Failed()
	}
	if s.Alerts != nil {
		out["alerts"] = s.Alerts.Public().Active
	}
	if s.Sites != nil {
		up, down := 0, []string{}
		for _, st := range s.Sites.Sites() {
			if st.Last == nil {
				continue
			}
			if st.Last.Up {
				up++
			} else {
				down = append(down, st.Host)
			}
		}
		out["sites"] = map[string]any{"up": up, "down": down}
	}
	running := slices.DeleteFunc(slices.Clone(rows), func(c ContainerRow) bool { return c.Live == nil })
	out["containers"] = running
	out["counts"] = counts
	out["stacks"] = sums
	out["activity"], _ = s.Store.Activity(store.ActivityQuery{Limit: 8})
	writeJSON(w, http.StatusOK, out)
}

// rangeOf maps a chart range to its start and bucket width in seconds.
func rangeOf(r *http.Request, now time.Time) (string, int64, int64) {
	switch r.URL.Query().Get("range") {
	case "24h":
		return "24h", now.Add(-24 * time.Hour).Unix(), 300
	case "7d":
		return "7d", now.Add(-7 * 24 * time.Hour).Unix(), 1800
	}
	return "1h", now.Add(-time.Hour).Unix(), 0
}

func (s *Server) hostHistory(w http.ResponseWriter, r *http.Request, _ sess) {
	rg, from, bucket := rangeOf(r, s.now())
	if rg == "1h" {
		writeJSON(w, http.StatusOK, s.Collector.HostRing())
		return
	}
	xs, err := s.Store.HostHistory(from, bucket)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	if xs == nil {
		xs = []host.Sample{}
	}
	writeJSON(w, http.StatusOK, xs)
}

func (s *Server) containers(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	ctrs, err := s.Docker.Containers(ctx, true)
	if err != nil {
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, s.rows(ctrs))
}

func (s *Server) container(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	d, err := s.Docker.Inspect(ctx, r.PathValue("id"))
	if err != nil {
		failDocker(w, err)
		return
	}
	out := map[string]any{"detail": d, "live": nil}
	if l, ok := s.Collector.Live()[d.ID]; ok {
		out["live"] = l
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) containerHistory(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	// Resolve names and short ids to the full id the readings are keyed by.
	d, err := s.Docker.Inspect(ctx, r.PathValue("id"))
	if err != nil {
		failDocker(w, err)
		return
	}
	rg, from, bucket := rangeOf(r, s.now())
	if rg == "1h" {
		writeJSON(w, http.StatusOK, s.Collector.CtrRing(d.ID))
		return
	}
	xs, err := s.Store.CtrHistory(d.ID, from, bucket)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func (s *Server) containerNet(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	n, err := s.Docker.NetStats(ctx, r.PathValue("id"))
	if err != nil {
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, n)
}

// containerLogs streams output as Server-Sent Events: one JSON line per event,
// and a comment every 20 s so proxies keep the connection open.
func (s *Server) containerLogs(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx := r.Context()
	ictx, cancel := context.WithTimeout(ctx, 10*time.Second)
	d, err := s.Docker.Inspect(ictx, r.PathValue("id"))
	cancel()
	if err != nil {
		failDocker(w, err)
		return
	}
	q := r.URL.Query()
	tail := q.Get("tail")
	if n, err := strconv.Atoi(tail); err != nil || n < 0 {
		tail = "300"
	} else if n > 10000 {
		tail = "10000"
	}
	follow := q.Get("follow") != "0"
	since := q.Get("since")
	if _, err := strconv.ParseInt(since, 10, 64); err != nil {
		since = ""
	}
	flusher, _ := w.(http.Flusher)
	h := w.Header()
	h.Set("Content-Type", "text/event-stream")
	h.Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	var mu sync.Mutex
	write := func(b []byte) error {
		mu.Lock()
		defer mu.Unlock()
		if _, err := w.Write(b); err != nil {
			return err
		}
		if flusher != nil {
			flusher.Flush()
		}
		return nil
	}
	write([]byte(": open\n\n"))
	done := make(chan struct{})
	defer close(done)
	go func() {
		t := time.NewTicker(20 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-done:
				return
			case <-t.C:
				if write([]byte(": ping\n\n")) != nil {
					return
				}
			}
		}
	}()
	err = s.Docker.Logs(ctx, d.ID, tail, since, follow, d.Tty, func(l docker.LogLine) error {
		b, _ := jsonLine(l)
		return write(append(append([]byte("data: "), b...), '\n', '\n'))
	})
	if ctx.Err() == nil {
		msg := "end"
		if err != nil {
			msg = err.Error()
		}
		b, _ := jsonLine(map[string]string{"msg": msg})
		write(append(append([]byte("event: end\ndata: "), b...), '\n', '\n'))
	}
}

func (s *Server) loadStacks(ctx context.Context) ([]compose.Stack, error) {
	ctrs, err := s.Docker.Containers(ctx, true)
	if err != nil {
		return nil, err
	}
	known, err := s.Store.Stacks()
	if err != nil {
		return nil, err
	}
	return compose.Build(ctrs, known), nil
}

// StackView adds live readings to each container of a stack.
type StackView struct {
	compose.Stack
	Live map[string]collector.CtrLive `json:"live"`
}

func (s *Server) stacks(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	st, err := s.loadStacks(ctx)
	if err != nil {
		failDocker(w, err)
		return
	}
	live := s.Collector.Live()
	out := make([]StackView, 0, len(st))
	for _, x := range st {
		out = append(out, StackView{x, liveFor(x, live)})
	}
	writeJSON(w, http.StatusOK, out)
}

func liveFor(st compose.Stack, live map[string]collector.CtrLive) map[string]collector.CtrLive {
	m := map[string]collector.CtrLive{}
	for _, sv := range st.Services {
		for _, c := range sv.Containers {
			if l, ok := live[c.ID]; ok {
				m[c.ID] = l
			}
		}
	}
	return m
}

func (s *Server) findStack(ctx context.Context, project string) (compose.Stack, bool, error) {
	st, err := s.loadStacks(ctx)
	if err != nil {
		return compose.Stack{}, false, err
	}
	i := slices.IndexFunc(st, func(x compose.Stack) bool { return x.Project == project })
	if i < 0 {
		return compose.Stack{}, false, nil
	}
	return st[i], true, nil
}

func (s *Server) stack(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	st, ok, err := s.findStack(ctx, r.PathValue("project"))
	if err != nil {
		failDocker(w, err)
		return
	}
	if !ok {
		fail(w, http.StatusNotFound, "no such stack")
		return
	}
	writeJSON(w, http.StatusOK, StackView{st, liveFor(st, s.Collector.Live())})
}

const dfTTL = 2 * time.Minute

// measureDF runs `docker system df`, which walks every layer and volume and can
// take several seconds, and caches the answer.
func (s *Server) measureDF(ctx context.Context) (*docker.DF, time.Time, error) {
	u, err := s.Docker.DiskUsage(ctx)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dfBusy = false
	if err != nil {
		return s.df, s.dfAt, err
	}
	s.df, s.dfAt = &u, s.now()
	return s.df, s.dfAt, nil
}

// WarmDF measures once in the background, so the first visit to Storage is quick.
func (s *Server) WarmDF(ctx context.Context) {
	s.mu.Lock()
	s.dfBusy = true
	s.mu.Unlock()
	go func() {
		c, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		s.measureDF(c)
	}()
}

// storage is what Docker keeps on disk. A stale measurement is served at once
// while a fresh one runs in the background; ?refresh=1 waits for a new one.
func (s *Server) storage(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	ctrs, err := s.Docker.Containers(ctx, true)
	if err != nil {
		failDocker(w, err)
		return
	}
	s.mu.Lock()
	df, at, busy := s.df, s.dfAt, s.dfBusy
	stale := df == nil || s.now().Sub(at) > dfTTL
	if stale && df != nil && !busy && r.URL.Query().Get("refresh") != "1" {
		s.dfBusy = true
		go func() {
			c, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			s.measureDF(c)
		}()
	}
	s.mu.Unlock()
	dfErr := ""
	if df == nil || r.URL.Query().Get("refresh") == "1" {
		if df, at, err = s.measureDF(ctx); err != nil {
			dfErr = err.Error()
		}
	}
	out := map[string]any{"df": df, "df_error": dfErr, "measured_at": at.Unix()}
	if df != nil {
		out["images"] = df.ImageList(ctrs)
	} else if imgs, err := s.Docker.Images(ctx, ctrs); err == nil {
		out["images"] = imgs
	} else {
		out["images"], out["images_error"] = []docker.Image{}, err.Error()
	}
	if vols, err := s.Docker.Volumes(ctx, ctrs, df); err == nil {
		out["volumes"] = vols
	} else {
		out["volumes"], out["volumes_error"] = []docker.Volume{}, err.Error()
	}
	if nets, err := s.Docker.Networks(ctx, ctrs); err == nil {
		out["networks"] = nets
	} else {
		out["networks"], out["networks_error"] = []docker.Network{}, err.Error()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) activity(w http.ResponseWriter, r *http.Request, _ sess) {
	q := activityQuery(r)
	q.Before, _ = strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	q.Limit = cmp.Or(limit, 100)
	list, err := s.Store.Activity(q)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

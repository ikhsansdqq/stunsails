package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"server-monitor/internal/alert"
	"server-monitor/internal/cleanup"
	"server-monitor/internal/docker"
)

func (s *Server) settings(w http.ResponseWriter, r *http.Request, _ sess) {
	out := map[string]any{}
	if s.Alerts != nil {
		out["alerts"] = s.Alerts.Public()
	}
	if s.Cleanup != nil {
		out["cleanup"] = s.Cleanup.Config()
	}
	if s.HostLog != nil {
		out["auth_log_error"] = s.HostLog.AuthError()
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) alertsSave(w http.ResponseWriter, r *http.Request, _ sess) {
	var u alert.Update
	if !readJSON(w, r, &u) {
		return
	}
	err := s.Alerts.Save(u)
	s.audit(r, "action", "alert settings saved", "", "", err)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.Alerts.Public())
}

func (s *Server) alertsTest(w http.ResponseWriter, r *http.Request, _ sess) {
	err := s.Alerts.Test()
	s.audit(r, "action", "alert test sent", "", "", err)
	if err != nil {
		fail(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) cleanupSave(w http.ResponseWriter, r *http.Request, _ sess) {
	var c cleanup.Config
	if !readJSON(w, r, &c) {
		return
	}
	err := s.Cleanup.Save(c)
	s.audit(r, "action", "cleanup schedule saved", "", "", err)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, s.Cleanup.Config())
}

func (s *Server) cleanupRun(w http.ResponseWriter, r *http.Request, _ sess) {
	var body struct {
		Kinds []docker.PruneKind `json:"kinds"`
	}
	if !readJSON(w, r, &body) {
		return
	}
	if len(body.Kinds) == 0 {
		body.Kinds = s.Cleanup.Config().Kinds
	}
	res, err := s.Cleanup.RunNow(context.WithoutCancel(r.Context()), body.Kinds, "by hand")
	if errors.Is(err, cleanup.ErrBusy) {
		fail(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) sitesList(w http.ResponseWriter, r *http.Request, _ sess) {
	writeJSON(w, http.StatusOK, s.Sites.Sites())
}

func (s *Server) siteHistory(w http.ResponseWriter, r *http.Request, _ sess) {
	_, from, bucket := rangeOf(r, s.now())
	if bucket == 0 {
		bucket = 60
	}
	xs, err := s.Sites.Store.SiteHistory(r.PathValue("host"), from, bucket)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, xs)
}

func (s *Server) siteAdd(w http.ResponseWriter, r *http.Request, _ sess) {
	var body struct{ Host string }
	if !readJSON(w, r, &body) {
		return
	}
	err := s.Sites.Add(body.Host)
	s.audit(r, "action", "site added", body.Host, body.Host, err)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) siteRemove(w http.ResponseWriter, r *http.Request, _ sess) {
	h := r.PathValue("host")
	err := s.Sites.Remove(h)
	s.audit(r, "action", "site removed", h, h, err)
	if err != nil {
		fail(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

type selfSample struct {
	usec uint64
	at   time.Time
}

// self is Harbor's own footprint: what it costs the server to watch it.
func (s *Server) self(w http.ResponseWriter, r *http.Request, _ sess) {
	status := procStatus("/proc/self/status")
	out := map[string]any{
		"rss":        status["VmRSS"],
		"rss_peak":   status["VmHWM"],
		"threads":    status["Threads"] / 1024, // procStatus scales kB; Threads is a count
		"goroutines": runtime.NumGoroutine(),
		"uptime_sec": int64(s.now().Sub(s.Started).Seconds()),
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	out["go_heap"] = ms.HeapAlloc
	out["go_sys"] = ms.Sys
	// Memory as systemd accounts it (with page cache), when Harbor runs as its service.
	if cg := selfCgroup(); strings.HasSuffix(cg, ".service") {
		out["mem_current"] = readInt(cg + "/memory.current")
		out["mem_peak"] = readInt(cg + "/memory.peak")
		out["mem_max"] = readInt(cg + "/memory.max")
	}
	// CPU from the process's own counters, so it's right however Harbor was started.
	if cpu, ok := procCPU("/proc/self/stat"); ok {
		usec := uint64(cpu.Microseconds())
		now := s.now()
		s.mu.Lock()
		prev := s.selfPrev
		s.selfPrev = selfSample{usec, now}
		s.mu.Unlock()
		out["cpu_sec"] = cpu.Seconds()
		if !prev.at.IsZero() && now.Sub(prev.at) > time.Second && usec >= prev.usec {
			out["cpu_pct"] = float64(usec-prev.usec) / float64(now.Sub(prev.at).Microseconds()) * 100
		} else if up := now.Sub(s.Started); up > 0 {
			out["cpu_pct"] = float64(usec) / float64(up.Microseconds()) * 100
		}
	}
	if p := s.Store.Path(); p != "" {
		var db int64
		for _, f := range []string{p, p + "-wal", p + "-shm"} {
			if st, err := os.Stat(f); err == nil {
				db += st.Size()
			}
		}
		out["db_bytes"] = db
	}
	if exe, err := os.Executable(); err == nil {
		if st, err := os.Stat(exe); err == nil {
			out["binary_bytes"] = st.Size()
		}
	}
	s.mu.Lock()
	stale := s.now().Sub(s.helperAt) > 10*time.Minute
	s.mu.Unlock()
	if stale && s.Compose != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		size, _ := s.Docker.ImageSize(ctx, s.Compose.Image)
		cancel()
		s.mu.Lock()
		s.helperSz, s.helperAt = size, s.now()
		s.mu.Unlock()
	}
	s.mu.Lock()
	out["helper_image_bytes"] = s.helperSz
	s.mu.Unlock()
	if s.Compose != nil {
		out["helper_image"] = s.Compose.Image
	}
	writeJSON(w, http.StatusOK, out)
}

// procStatus reads /proc/self/status; kB values are returned in bytes.
func procStatus(path string) map[string]int64 {
	out := map[string]int64{}
	b, err := os.ReadFile(path)
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(b), "\n") {
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		f := strings.Fields(v)
		if len(f) == 0 {
			continue
		}
		n, err := strconv.ParseInt(f[0], 10, 64)
		if err != nil {
			continue
		}
		out[k] = n * 1024
	}
	return out
}

// procCPU is user plus system time from /proc/<pid>/stat (clock ticks are 1/100 s on Linux).
func procCPU(path string) (time.Duration, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	// The command name is in parentheses and may contain spaces; fields follow the last ")".
	i := strings.LastIndexByte(string(b), ')')
	if i < 0 {
		return 0, false
	}
	f := strings.Fields(string(b)[i+1:])
	if len(f) < 13 {
		return 0, false
	}
	ut, _ := strconv.ParseInt(f[11], 10, 64)
	st, _ := strconv.ParseInt(f[12], 10, 64)
	return time.Duration(ut+st) * 10 * time.Millisecond, true
}

func selfCgroup() string {
	b, err := os.ReadFile("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if p, ok := strings.CutPrefix(line, "0::"); ok {
			return "/sys/fs/cgroup" + p
		}
	}
	return ""
}

func readInt(path string) int64 {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	n, _ := strconv.ParseInt(strings.TrimSpace(string(b)), 10, 64)
	return n
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/compose"
	"github.com/ikhsansdqq/stunsails/internal/docker"
)

func jsonLine(v any) ([]byte, error) { return json.Marshal(v) }

func (s *Server) containerAction(w http.ResponseWriter, r *http.Request, _ sess) {
	a := docker.Action(r.PathValue("action"))
	if !a.Valid() {
		fail(w, http.StatusNotFound, "unknown action")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	d, err := s.Docker.Inspect(ctx, r.PathValue("id"))
	if err != nil {
		failDocker(w, err)
		return
	}
	err = s.Docker.Do(ctx, d.ID, a)
	s.audit(r, "action", "container "+string(a), d.ID, d.Name, err)
	if err != nil {
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

// lockedWriter streams NDJSON lines and keeps going quietly if the browser
// has gone: a compose run is never cut off halfway because a tab closed.
type lockedWriter struct {
	mu     sync.Mutex
	w      http.ResponseWriter
	f      http.Flusher
	broken bool
}

func (l *lockedWriter) line(v any) {
	b, _ := json.Marshal(v)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.broken {
		return
	}
	if _, err := l.w.Write(append(b, '\n')); err != nil {
		l.broken = true
		return
	}
	if l.f != nil {
		l.f.Flush()
	}
}

// outWriter turns compose's output into one NDJSON line per text line.
type outWriter struct {
	l   *lockedWriter
	buf strings.Builder
}

func (o *outWriter) Write(p []byte) (int, error) {
	o.buf.Write(p)
	s := o.buf.String()
	for {
		i := strings.IndexByte(s, '\n')
		if i < 0 {
			break
		}
		o.l.line(map[string]string{"t": "out", "l": s[:i]})
		s = s[i+1:]
	}
	o.buf.Reset()
	o.buf.WriteString(s)
	return len(p), nil
}

func (s *Server) stackAction(w http.ResponseWriter, r *http.Request, _ sess) {
	v := compose.Verb(r.PathValue("verb"))
	switch v {
	case compose.Up, compose.Down, compose.Pull, compose.Start, compose.Stop, compose.Restart:
	default:
		fail(w, http.StatusNotFound, "unknown action")
		return
	}
	project := r.PathValue("project")
	lctx, cancel := dctx(r)
	st, ok, err := s.findStack(lctx, project)
	cancel()
	if err != nil {
		failDocker(w, err)
		return
	}
	if !ok {
		fail(w, http.StatusNotFound, "no such stack")
		return
	}
	if !st.Manageable {
		fail(w, http.StatusConflict, "StunSails doesn't know where this stack's compose file is.")
		return
	}
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	f, _ := w.(http.Flusher)
	lw := &lockedWriter{w: w, f: f}
	out := &outWriter{l: lw}
	// The run outlives the request so a closed tab doesn't leave a stack half up.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Minute)
	defer cancel()
	err = s.Compose.Run(ctx, st, v, out)
	if out.buf.Len() > 0 {
		lw.line(map[string]string{"t": "out", "l": out.buf.String()})
	}
	s.audit(r, "action", "stack "+string(v), st.Project, st.Project, err)
	done := map[string]any{"t": "done", "ok": err == nil}
	if err != nil {
		done["error"] = err.Error()
	}
	lw.line(done)
}

func (s *Server) stackForget(w http.ResponseWriter, r *http.Request, _ sess) {
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
	if st.Total > 0 {
		fail(w, http.StatusConflict, "Take the stack down before forgetting it.")
		return
	}
	err = s.Store.ForgetStack(st.Project)
	s.audit(r, "action", "stack forget", st.Project, st.Project, err)
	if err != nil {
		fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) imageRemove(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := context.WithTimeout(r.Context(), 60*time.Second)
	defer cancel()
	id := r.PathValue("id")
	err := s.Docker.RemoveImage(ctx, id)
	s.audit(r, "action", "image remove", id, r.URL.Query().Get("name"), err)
	if err == nil {
		s.mu.Lock()
		if s.df != nil {
			// Drop it from the cached list now; the sizes are re-measured in the background.
			s.df, s.dfAt = s.df.WithoutImage(id), time.Time{}
		}
		s.mu.Unlock()
	}
	if err != nil {
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) volumeRemove(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	name := r.PathValue("name")
	ctrs, err := s.Docker.Containers(ctx, true)
	if err != nil {
		failDocker(w, err)
		return
	}
	vols, err := s.Docker.Volumes(ctx, ctrs, nil)
	if err != nil {
		failDocker(w, err)
		return
	}
	for _, v := range vols {
		if v.Name == name && len(v.Containers) > 0 {
			fail(w, http.StatusConflict, "Containers still use this volume: "+strings.Join(v.Containers, ", "))
			return
		}
	}
	err = s.Docker.RemoveVolume(ctx, name)
	s.audit(r, "action", "volume remove", name, name, err)
	s.staleDF()
	if err != nil {
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) networkRemove(w http.ResponseWriter, r *http.Request, _ sess) {
	ctx, cancel := dctx(r)
	defer cancel()
	id := r.PathValue("id")
	if id == "bridge" || id == "host" || id == "none" {
		fail(w, http.StatusConflict, "Docker's built-in networks can't be removed.")
		return
	}
	err := s.Docker.RemoveNetwork(ctx, id)
	s.audit(r, "action", "network remove", id, r.URL.Query().Get("name"), err)
	if err != nil {
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (s *Server) prune(w http.ResponseWriter, r *http.Request, _ sess) {
	kind := docker.PruneKind(r.PathValue("kind"))
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 10*time.Minute)
	defer cancel()
	res, err := s.Docker.Prune(ctx, kind)
	s.audit(r, "action", "prune "+string(kind), "", fmt.Sprintf("%d removed, %d bytes", res.Removed, res.Reclaimed), err)
	s.invalidateDF()
	if err != nil {
		var de *docker.Error
		if !errors.As(err, &de) && strings.HasPrefix(err.Error(), "unknown prune") {
			fail(w, http.StatusNotFound, err.Error())
			return
		}
		failDocker(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// staleDF keeps the cached sizes but has the next read refresh them in the background.
func (s *Server) staleDF() {
	s.mu.Lock()
	s.dfAt = time.Time{}
	s.mu.Unlock()
}

// InvalidateDF is invalidateDF for other packages (the cleanup scheduler).
func (s *Server) InvalidateDF() { s.invalidateDF() }

// invalidateDF drops the cached sizes; the next read waits for a new measurement.
func (s *Server) invalidateDF() {
	s.mu.Lock()
	s.df = nil
	s.mu.Unlock()
}

// Package compose groups containers into compose stacks by their labels and
// runs `docker compose` for a stack inside a throwaway helper container. The
// helper gets the stack's folder mounted read-only at the same path, so
// relative binds and env files resolve as they did when the stack was started,
// and StunSails itself never needs read access to anyone's home folder.
//
// Start, stop and restart skip the compose CLI and act on the stack's existing
// containers through the Engine API. Compose would parse the whole file first,
// and a file that needs a variable only its own deploy script sets (FDS's
// ${FDS_TAG:?}) would refuse even a stop.
package compose

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"github.com/ikhsansdqq/stunsails/internal/docker"
	"github.com/ikhsansdqq/stunsails/internal/store"
)

type Stack struct {
	Project     string    `json:"project"`
	WorkingDir  string    `json:"working_dir"`
	ConfigFiles []string  `json:"config_files"`
	EnvFiles    []string  `json:"env_files"`
	Services    []Service `json:"services"`
	Running     int       `json:"running"`
	Total       int       `json:"total"`
	Unhealthy   int       `json:"unhealthy"`
	State       string    `json:"state"` // running, partial, stopped, down
	LastSeen    int64     `json:"last_seen"`
	Manageable  bool      `json:"manageable"` // working dir and compose files are known
}

type Service struct {
	Name       string             `json:"name"`
	Containers []docker.Container `json:"containers"`
}

// Build merges live containers with stacks remembered from earlier, so a stack
// taken down still shows (as "down") and can be brought back up.
func Build(ctrs []docker.Container, known []store.StackRow) []Stack {
	by := map[string]*Stack{}
	get := func(p string) *Stack {
		if s := by[p]; s != nil {
			return s
		}
		s := &Stack{Project: p, ConfigFiles: []string{}, EnvFiles: []string{}, Services: []Service{}}
		by[p] = s
		return s
	}
	for _, k := range known {
		s := get(k.Project)
		s.WorkingDir, s.ConfigFiles, s.EnvFiles, s.LastSeen = k.WorkingDir, k.ConfigFiles, k.EnvFiles, k.LastSeen
	}
	for _, c := range ctrs {
		if c.Project == "" || c.Labels[docker.LabelOneoff] == "True" {
			continue
		}
		s := get(c.Project)
		if wd := c.Labels[docker.LabelWorkingDir]; wd != "" {
			s.WorkingDir = wd
			if cf := c.Labels[docker.LabelConfig]; cf != "" {
				s.ConfigFiles = strings.Split(cf, ",")
			}
			if ef := c.Labels[docker.LabelEnvFile]; ef != "" {
				s.EnvFiles = strings.Split(ef, ",")
			}
		}
		i := slices.IndexFunc(s.Services, func(sv Service) bool { return sv.Name == c.Service })
		if i < 0 {
			s.Services = append(s.Services, Service{Name: c.Service})
			i = len(s.Services) - 1
		}
		s.Services[i].Containers = append(s.Services[i].Containers, c)
		s.Total++
		if c.State == "running" {
			s.Running++
		}
		if c.Health == "unhealthy" {
			s.Unhealthy++
		}
	}
	out := make([]Stack, 0, len(by))
	for _, s := range by {
		switch {
		case s.Total == 0:
			s.State = "down"
		case s.Running == s.Total:
			s.State = "running"
		case s.Running == 0:
			s.State = "stopped"
		default:
			s.State = "partial"
		}
		s.Manageable = paths(*s) == nil
		slices.SortFunc(s.Services, func(a, b Service) int { return strings.Compare(a.Name, b.Name) })
		out = append(out, *s)
	}
	slices.SortFunc(out, func(a, b Stack) int { return strings.Compare(a.Project, b.Project) })
	return out
}

var projectRe = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]*$`)

// paths checks the folders the helper would mount: absolute, clean, not the root.
func paths(s Stack) error {
	if !projectRe.MatchString(s.Project) {
		return errors.New("project name is not one compose accepts")
	}
	if s.WorkingDir == "" || len(s.ConfigFiles) == 0 {
		return errors.New("the stack's folder isn't known yet; start it once with docker compose")
	}
	for _, p := range append([]string{s.WorkingDir}, append(s.ConfigFiles, s.EnvFiles...)...) {
		if !filepath.IsAbs(p) || filepath.Clean(p) != p || p == "/" || strings.ContainsAny(p, ":,\n") {
			return fmt.Errorf("unsafe path %q", p)
		}
	}
	return nil
}

// Binds lists read-only mounts for the stack folder and any compose or env file outside it.
func Binds(s Stack) []string {
	dirs := []string{s.WorkingDir}
	for _, f := range append(slices.Clone(s.ConfigFiles), s.EnvFiles...) {
		d := filepath.Dir(f)
		if d == "/" {
			continue
		}
		inside := slices.ContainsFunc(dirs, func(x string) bool { return d == x || strings.HasPrefix(d, x+"/") })
		if !inside {
			dirs = append(dirs, d)
		}
	}
	binds := []string{"/var/run/docker.sock:/var/run/docker.sock"}
	for _, d := range dirs {
		binds = append(binds, d+":"+d+":ro")
	}
	return binds
}

// Command is the docker compose argv for a stack.
func Command(s Stack, args ...string) []string {
	cmd := []string{"docker", "compose", "-p", s.Project, "--project-directory", s.WorkingDir}
	for _, f := range s.ConfigFiles {
		cmd = append(cmd, "-f", f)
	}
	for _, f := range s.EnvFiles {
		cmd = append(cmd, "--env-file", f)
	}
	return append(cmd, args...)
}

// Verb is a stack action. Up, down and pull run the compose CLI; start, stop
// and restart act on the containers directly (see lifecycle).
type Verb string

const (
	Up      Verb = "up"   // create and start, recreating what changed
	Down    Verb = "down" // stop and remove containers and networks; volumes stay
	Pull    Verb = "pull" // pull newer images, then up
	Start   Verb = "start"
	Stop    Verb = "stop"
	Restart Verb = "restart"
)

func (v Verb) steps() [][]string {
	switch v {
	case Up:
		return [][]string{{"up", "-d", "--remove-orphans"}}
	case Down:
		return [][]string{{"down", "--remove-orphans"}}
	case Pull:
		return [][]string{{"pull"}, {"up", "-d", "--remove-orphans"}}
	case Start, Stop, Restart:
		return [][]string{{string(v)}}
	}
	return nil
}

type Runner struct {
	Docker *docker.Client
	Image  string // a docker CLI image with the compose plugin, e.g. docker:29-cli

	mu   sync.Mutex
	busy map[string]bool
}

var ErrBusy = errors.New("another action is still running on this stack")

// Run executes a verb for s, writing compose's output to w. Only one action
// runs per stack at a time.
func (r *Runner) Run(ctx context.Context, s Stack, v Verb, w io.Writer) error {
	steps := v.steps()
	if steps == nil {
		return fmt.Errorf("unknown action %q", v)
	}
	if err := paths(s); err != nil {
		return err
	}
	r.mu.Lock()
	if r.busy == nil {
		r.busy = map[string]bool{}
	}
	if r.busy[s.Project] {
		r.mu.Unlock()
		return ErrBusy
	}
	r.busy[s.Project] = true
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.busy, s.Project)
		r.mu.Unlock()
	}()

	switch v {
	case Start, Stop, Restart:
		return r.lifecycle(ctx, s, v, w)
	}
	if err := r.helperImage(ctx, w); err != nil {
		return err
	}
	for _, args := range steps {
		cmd := Command(s, args...)
		fmt.Fprintf(w, "$ %s\n", strings.Join(cmd, " "))
		code, err := r.Docker.Run(ctx, docker.RunSpec{Image: r.Image, Cmd: cmd, WorkingDir: s.WorkingDir, Binds: Binds(s)}, w)
		if err != nil {
			return err
		}
		if code != 0 {
			return fmt.Errorf("docker compose %s exited with code %d", args[0], code)
		}
	}
	return nil
}

func (r *Runner) helperImage(ctx context.Context, w io.Writer) error {
	if ok, err := r.Docker.HasImage(ctx, r.Image); err != nil {
		return err
	} else if !ok {
		fmt.Fprintf(w, "Pulling %s for the compose runner…\n", r.Image)
		return r.Docker.Pull(ctx, r.Image, w)
	}
	return nil
}

// lifecycle starts, stops or restarts the stack's existing containers in the
// order compose would: dependencies start first and stop last. Each stop gets
// the container's own grace period (see docker.Client.Do).
func (r *Runner) lifecycle(ctx context.Context, s Stack, v Verb, w io.Writer) error {
	if s.Total == 0 {
		return errors.New("the stack has no containers; bring it up first")
	}
	order := StartOrder(s)
	stop := slices.Clone(order)
	slices.Reverse(stop)
	each := func(list []docker.Container, a docker.Action, verb string) error {
		for _, c := range list {
			fmt.Fprintf(w, "%s %s\n", verb, c.Name)
			if err := r.Docker.Do(ctx, c.ID, a); err != nil {
				return fmt.Errorf("%s %s: %w", strings.ToLower(verb), c.Name, err)
			}
		}
		return nil
	}
	switch v {
	case Stop:
		return each(stop, docker.Stop, "Stopping")
	case Start:
		return each(order, docker.Start, "Starting")
	default:
		if err := each(stop, docker.Stop, "Stopping"); err != nil {
			return err
		}
		return each(order, docker.Start, "Starting")
	}
}

// StartOrder lists the stack's containers so each service comes after the
// services it depends on (compose's depends_on label), ties broken by name.
// A dependency cycle or an unknown dependency is ignored rather than fatal.
func StartOrder(s Stack) []docker.Container {
	deps := map[string][]string{}
	for _, sv := range s.Services {
		for _, c := range sv.Containers {
			for _, d := range strings.Split(c.Labels[docker.LabelDependsOn], ",") {
				if name, _, _ := strings.Cut(d, ":"); name != "" && !slices.Contains(deps[sv.Name], name) {
					deps[sv.Name] = append(deps[sv.Name], name)
				}
			}
		}
	}
	byName := map[string]Service{}
	for _, sv := range s.Services {
		byName[sv.Name] = sv
	}
	var out []docker.Container
	seen := map[string]bool{}
	var visit func(name string)
	visit = func(name string) {
		sv, ok := byName[name]
		if !ok || seen[name] {
			return
		}
		seen[name] = true
		for _, d := range deps[name] {
			visit(d)
		}
		out = append(out, sv.Containers...)
	}
	for _, sv := range s.Services { // already sorted by name
		visit(sv.Name)
	}
	return out
}

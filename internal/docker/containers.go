package docker

import (
	"context"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

// Compose labels; stacks are grouped by these.
const (
	LabelProject    = "com.docker.compose.project"
	LabelService    = "com.docker.compose.service"
	LabelWorkingDir = "com.docker.compose.project.working_dir"
	LabelConfig     = "com.docker.compose.project.config_files"
	LabelEnvFile    = "com.docker.compose.project.environment_file"
	LabelOneoff     = "com.docker.compose.oneoff"
	LabelDependsOn  = "com.docker.compose.depends_on"
	// LabelHelper marks Harbor's own throwaway compose runners, hidden everywhere.
	LabelHelper = "harbor.helper"
)

type Port struct {
	IP      string `json:"ip,omitempty"`
	Private int    `json:"private"`
	Public  int    `json:"public,omitempty"`
	Type    string `json:"type"`
}

// Container is one row of `docker ps -a`.
type Container struct {
	ID       string            `json:"id"`
	Name     string            `json:"name"`
	Image    string            `json:"image"`
	ImageID  string            `json:"image_id"`
	Command  string            `json:"command"`
	Created  int64             `json:"created"`
	State    string            `json:"state"`  // running, exited, restarting, paused, created, dead
	Status   string            `json:"status"` // "Up 3 hours (healthy)"
	Health   string            `json:"health"` // healthy, unhealthy, starting, or ""
	Ports    []Port            `json:"ports"`
	Project  string            `json:"project"`
	Service  string            `json:"service"`
	Networks []string          `json:"networks"`
	Labels   map[string]string `json:"-"`

	volumes []string // named volumes it mounts
}

var healthRe = regexp.MustCompile(`\((healthy|unhealthy|health: starting)\)`)

func health(status string) string {
	m := healthRe.FindStringSubmatch(status)
	if m == nil {
		return ""
	}
	if m[1] == "health: starting" {
		return "starting"
	}
	return m[1]
}

// Containers lists every container, running or not, except Harbor's helpers.
func (c *Client) Containers(ctx context.Context, all bool) ([]Container, error) {
	var raw []struct {
		ID      string `json:"Id"`
		Names   []string
		Image   string
		ImageID string
		Command string
		Created int64
		State   string
		Status  string
		Ports   []struct {
			IP          string
			PrivatePort int
			PublicPort  int
			Type        string
		}
		Labels          map[string]string
		Mounts          []struct{ Type, Name string }
		NetworkSettings struct {
			Networks map[string]struct{}
		}
	}
	q := url.Values{}
	if all {
		q.Set("all", "1")
	}
	if err := c.get(ctx, "/containers/json", q, &raw); err != nil {
		return nil, err
	}
	out := make([]Container, 0, len(raw))
	for _, r := range raw {
		if r.Labels[LabelHelper] != "" {
			continue
		}
		ct := Container{
			ID: r.ID, Image: r.Image, ImageID: r.ImageID, Command: r.Command, Created: r.Created,
			State: r.State, Status: r.Status, Health: health(r.Status), Labels: r.Labels,
			Project: r.Labels[LabelProject], Service: r.Labels[LabelService], Ports: []Port{}, Networks: []string{},
		}
		if len(r.Names) > 0 {
			ct.Name = strings.TrimPrefix(r.Names[0], "/")
		}
		// Docker lists a published port once per address family; keep one row per mapping.
		seen := map[string]bool{}
		for _, p := range r.Ports {
			k := strconv.Itoa(p.PrivatePort) + "/" + p.Type + ">" + strconv.Itoa(p.PublicPort)
			if seen[k] {
				continue
			}
			seen[k] = true
			ip := p.IP
			if ip == "0.0.0.0" || ip == "::" {
				ip = ""
			}
			ct.Ports = append(ct.Ports, Port{IP: ip, Private: p.PrivatePort, Public: p.PublicPort, Type: p.Type})
		}
		slices.SortFunc(ct.Ports, func(a, b Port) int { return a.Private - b.Private })
		for n := range r.NetworkSettings.Networks {
			ct.Networks = append(ct.Networks, n)
		}
		slices.Sort(ct.Networks)
		for _, m := range r.Mounts {
			if m.Type == "volume" && m.Name != "" {
				ct.volumes = append(ct.volumes, m.Name)
			}
		}
		out = append(out, ct)
	}
	return out, nil
}

// Detail is what the container page shows from `docker inspect`. Environment
// values are left out on purpose: they're often secrets.
type Detail struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	ImageID       string            `json:"image_id"`
	Created       string            `json:"created"`
	Command       []string          `json:"command"`
	State         State             `json:"state"`
	RestartCount  int               `json:"restart_count"`
	RestartPolicy string            `json:"restart_policy"`
	MemoryLimit   int64             `json:"memory_limit"`
	CPULimit      float64           `json:"cpu_limit"` // cores; 0 is unlimited
	Privileged    bool              `json:"privileged"`
	NetworkMode   string            `json:"network_mode"`
	User          string            `json:"user"`
	WorkingDir    string            `json:"working_dir"`
	Tty           bool              `json:"tty"`
	EnvKeys       []string          `json:"env_keys"`
	Labels        map[string]string `json:"labels"`
	Mounts        []Mount           `json:"mounts"`
	Ports         []PortBinding     `json:"ports"`
	Networks      []NetworkAttach   `json:"networks"`
	Project       string            `json:"project"`
	Service       string            `json:"service"`
	Healthcheck   []string          `json:"healthcheck"`
}

type State struct {
	Status     string       `json:"status"`
	Running    bool         `json:"running"`
	Paused     bool         `json:"paused"`
	Restarting bool         `json:"restarting"`
	OOMKilled  bool         `json:"oom_killed"`
	Pid        int          `json:"pid"`
	ExitCode   int          `json:"exit_code"`
	Error      string       `json:"error"`
	StartedAt  string       `json:"started_at"`
	FinishedAt string       `json:"finished_at"`
	Health     *HealthState `json:"health"`
}

type HealthState struct {
	Status        string        `json:"status"`
	FailingStreak int           `json:"failing_streak"`
	Log           []HealthProbe `json:"log"`
}

type HealthProbe struct {
	Start    string `json:"start"`
	End      string `json:"end"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output"`
}

type Mount struct {
	Type        string `json:"type"`
	Name        string `json:"name,omitempty"`
	Source      string `json:"source"`
	Destination string `json:"destination"`
	RW          bool   `json:"rw"`
}

type PortBinding struct {
	Container string `json:"container"` // "80/tcp"
	HostIP    string `json:"host_ip"`
	HostPort  string `json:"host_port"`
}

type NetworkAttach struct {
	Name    string   `json:"name"`
	IP      string   `json:"ip"`
	Gateway string   `json:"gateway"`
	MAC     string   `json:"mac"`
	Aliases []string `json:"aliases"`
}

func (c *Client) Inspect(ctx context.Context, id string) (Detail, error) {
	var r struct {
		ID      string `json:"Id"`
		Name    string
		Created string
		Path    string
		Args    []string
		Image   string
		State   struct {
			Status                                 string
			Running, Paused, Restarting, OOMKilled bool
			Pid, ExitCode                          int
			Error, StartedAt, FinishedAt           string
			Health                                 *struct {
				Status        string
				FailingStreak int
				Log           []struct {
					Start, End string
					ExitCode   int
					Output     string
				}
			}
		}
		RestartCount int
		HostConfig   struct {
			RestartPolicy struct {
				Name              string
				MaximumRetryCount int
			}
			Memory      int64
			NanoCpus    int64
			Privileged  bool
			NetworkMode string
		}
		Config struct {
			Image       string
			Env         []string
			Labels      map[string]string
			User        string
			WorkingDir  string
			Tty         bool
			Healthcheck *struct{ Test []string }
		}
		Mounts []struct {
			Type, Name, Source, Destination string
			RW                              bool
		}
		NetworkSettings struct {
			Ports    map[string][]struct{ HostIp, HostPort string }
			Networks map[string]struct {
				IPAddress, Gateway, MacAddress string
				Aliases                        []string
			}
		}
	}
	if err := c.get(ctx, "/containers/"+url.PathEscape(id)+"/json", nil, &r); err != nil {
		return Detail{}, err
	}
	d := Detail{
		ID: r.ID, Name: strings.TrimPrefix(r.Name, "/"), Image: r.Config.Image, ImageID: r.Image, Created: r.Created,
		Command: append([]string{r.Path}, r.Args...), RestartCount: r.RestartCount,
		RestartPolicy: r.HostConfig.RestartPolicy.Name, MemoryLimit: r.HostConfig.Memory,
		CPULimit: float64(r.HostConfig.NanoCpus) / 1e9, Privileged: r.HostConfig.Privileged,
		NetworkMode: r.HostConfig.NetworkMode, User: r.Config.User, WorkingDir: r.Config.WorkingDir, Tty: r.Config.Tty,
		Labels: r.Config.Labels, Project: r.Config.Labels[LabelProject], Service: r.Config.Labels[LabelService],
		EnvKeys: []string{}, Mounts: []Mount{}, Ports: []PortBinding{}, Networks: []NetworkAttach{}, Healthcheck: []string{},
		State: State{
			Status: r.State.Status, Running: r.State.Running, Paused: r.State.Paused, Restarting: r.State.Restarting,
			OOMKilled: r.State.OOMKilled, Pid: r.State.Pid, ExitCode: r.State.ExitCode, Error: r.State.Error,
			StartedAt: r.State.StartedAt, FinishedAt: r.State.FinishedAt,
		},
	}
	if d.Labels == nil {
		d.Labels = map[string]string{}
	}
	if p := r.HostConfig.RestartPolicy; p.Name == "on-failure" && p.MaximumRetryCount > 0 {
		d.RestartPolicy += ":" + strconv.Itoa(p.MaximumRetryCount)
	}
	if h := r.State.Health; h != nil {
		d.State.Health = &HealthState{Status: h.Status, FailingStreak: h.FailingStreak, Log: []HealthProbe{}}
		for _, l := range h.Log {
			d.State.Health.Log = append(d.State.Health.Log, HealthProbe{l.Start, l.End, l.ExitCode, strings.TrimSpace(l.Output)})
		}
	}
	if hc := r.Config.Healthcheck; hc != nil {
		d.Healthcheck = hc.Test
	}
	for _, e := range r.Config.Env {
		k, _, _ := strings.Cut(e, "=")
		d.EnvKeys = append(d.EnvKeys, k)
	}
	slices.Sort(d.EnvKeys)
	for _, m := range r.Mounts {
		d.Mounts = append(d.Mounts, Mount{m.Type, m.Name, m.Source, m.Destination, m.RW})
	}
	for port, binds := range r.NetworkSettings.Ports {
		if len(binds) == 0 {
			d.Ports = append(d.Ports, PortBinding{Container: port})
		}
		seen := map[string]bool{}
		for _, b := range binds {
			if seen[b.HostPort] {
				continue
			}
			seen[b.HostPort] = true
			d.Ports = append(d.Ports, PortBinding{port, b.HostIp, b.HostPort})
		}
	}
	slices.SortFunc(d.Ports, func(a, b PortBinding) int { return strings.Compare(a.Container, b.Container) })
	for name, n := range r.NetworkSettings.Networks {
		al := n.Aliases
		if al == nil {
			al = []string{}
		}
		d.Networks = append(d.Networks, NetworkAttach{name, n.IPAddress, n.Gateway, n.MacAddress, al})
	}
	slices.SortFunc(d.Networks, func(a, b NetworkAttach) int { return strings.Compare(a.Name, b.Name) })
	return d, nil
}

// Action is a lifecycle verb the dashboard may send.
type Action string

const (
	Start   Action = "start"
	Stop    Action = "stop"
	Restart Action = "restart"
	Kill    Action = "kill"
	Pause   Action = "pause"
	Unpause Action = "unpause"
	Remove  Action = "remove"
)

func (a Action) Valid() bool {
	switch a {
	case Start, Stop, Restart, Kill, Pause, Unpause, Remove:
		return true
	}
	return false
}

// Do runs a lifecycle action. Stop and restart leave the grace period to the
// engine, which uses the container's own stop timeout (compose's
// stop_grace_period) and 10 s when it has none. Remove forces (stopping a
// running container) but keeps anonymous volumes.
func (c *Client) Do(ctx context.Context, id string, a Action) error {
	p := "/containers/" + url.PathEscape(id)
	switch a {
	case Remove:
		return c.call(ctx, http.MethodDelete, p, url.Values{"force": {"1"}}, nil, nil)
	default:
		return c.call(ctx, http.MethodPost, p+"/"+string(a), nil, nil, nil)
	}
}

// NetStats is a one-shot network counter read for one container.
type NetStats struct {
	Rx uint64 `json:"rx"`
	Tx uint64 `json:"tx"`
	At int64  `json:"at"`
}

func (c *Client) NetStats(ctx context.Context, id string) (NetStats, error) {
	var s struct {
		Networks map[string]struct {
			RxBytes uint64 `json:"rx_bytes"`
			TxBytes uint64 `json:"tx_bytes"`
		} `json:"networks"`
	}
	q := url.Values{"stream": {"false"}, "one-shot": {"true"}}
	if err := c.get(ctx, "/containers/"+url.PathEscape(id)+"/stats", q, &s); err != nil {
		return NetStats{}, err
	}
	out := NetStats{At: time.Now().UnixMilli()}
	for _, n := range s.Networks {
		out.Rx += n.RxBytes
		out.Tx += n.TxBytes
	}
	return out, nil
}

package docker

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

type Image struct {
	ID         string   `json:"id"`
	Tags       []string `json:"tags"`
	Created    int64    `json:"created"`
	Size       int64    `json:"size"`
	Containers []string `json:"containers"` // names of containers made from it, running or not
	Dangling   bool     `json:"dangling"`
}

type rawImage struct {
	ID         string `json:"Id"`
	RepoTags   []string
	Created    int64
	Size       int64
	SharedSize int64
	Containers int
}

// Images lists images. /images/json can take seconds on a busy daemon; Storage
// uses DF.ImageList instead, from the `system df` answer it caches anyway.
func (c *Client) Images(ctx context.Context, ctrs []Container) ([]Image, error) {
	var raw []rawImage
	if err := c.get(ctx, "/images/json", nil, &raw); err != nil {
		return nil, err
	}
	return imageList(raw, ctrs), nil
}

// WithoutImage is a copy of d with one image left out, for after a removal.
func (d *DF) WithoutImage(id string) *DF {
	c := *d
	c.images = slices.DeleteFunc(slices.Clone(d.images), func(r rawImage) bool { return r.ID == id })
	return &c
}

// ImageList is the image list from the same `system df` reading.
func (d *DF) ImageList(ctrs []Container) []Image { return imageList(d.images, ctrs) }

func imageList(raw []rawImage, ctrs []Container) []Image {
	users := map[string][]string{}
	for _, ct := range ctrs {
		users[ct.ImageID] = append(users[ct.ImageID], ct.Name)
	}
	out := make([]Image, 0, len(raw))
	for _, r := range raw {
		tags := []string{}
		for _, t := range r.RepoTags {
			if t != "<none>:<none>" {
				tags = append(tags, t)
			}
		}
		u := users[r.ID]
		if u == nil {
			u = []string{}
		}
		out = append(out, Image{ID: r.ID, Tags: tags, Created: r.Created, Size: r.Size, Containers: u, Dangling: len(tags) == 0})
	}
	slices.SortFunc(out, func(a, b Image) int { return cmp.Compare(b.Size, a.Size) })
	return out
}

// RemoveImage deletes an image (or one tag of it) that no container uses.
func (c *Client) RemoveImage(ctx context.Context, ref string) error {
	return c.call(ctx, http.MethodDelete, "/images/"+url.PathEscape(ref), nil, nil, nil)
}

type Volume struct {
	Name       string            `json:"name"`
	Driver     string            `json:"driver"`
	Mountpoint string            `json:"mountpoint"`
	Created    string            `json:"created"`
	Labels     map[string]string `json:"labels"`
	Size       int64             `json:"size"` // -1 when Docker hasn't measured it
	Containers []string          `json:"containers"`
	Project    string            `json:"project"`
}

func (c *Client) Volumes(ctx context.Context, ctrs []Container, df *DF) ([]Volume, error) {
	var raw struct {
		Volumes []struct {
			Name, Driver, Mountpoint, CreatedAt string
			Labels                              map[string]string
		}
	}
	if err := c.get(ctx, "/volumes", nil, &raw); err != nil {
		return nil, err
	}
	sizes := map[string]int64{}
	if df != nil {
		sizes = df.volumeSizes
	}
	users := map[string][]string{}
	for _, ct := range ctrs {
		for _, v := range ct.volumes {
			users[v] = append(users[v], ct.Name)
		}
	}
	out := make([]Volume, 0, len(raw.Volumes))
	for _, r := range raw.Volumes {
		size, ok := sizes[r.Name]
		if !ok {
			size = -1
		}
		u := users[r.Name]
		if u == nil {
			u = []string{}
		}
		lb := r.Labels
		if lb == nil {
			lb = map[string]string{}
		}
		out = append(out, Volume{r.Name, r.Driver, r.Mountpoint, r.CreatedAt, lb, size, u, lb["com.docker.compose.project"]})
	}
	slices.SortFunc(out, func(a, b Volume) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// RemoveVolume deletes a volume; Docker itself refuses one a container still uses.
func (c *Client) RemoveVolume(ctx context.Context, name string) error {
	return c.call(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(name), nil, nil, nil)
}

type Network struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Driver     string   `json:"driver"`
	Scope      string   `json:"scope"`
	Internal   bool     `json:"internal"`
	Subnets    []string `json:"subnets"`
	Created    string   `json:"created"`
	Containers []string `json:"containers"`
	Project    string   `json:"project"`
	Builtin    bool     `json:"builtin"` // bridge, host, none: Docker won't remove these
}

func (c *Client) Networks(ctx context.Context, ctrs []Container) ([]Network, error) {
	var raw []struct {
		ID       string `json:"Id"`
		Name     string
		Driver   string
		Scope    string
		Internal bool
		Created  string
		Labels   map[string]string
		IPAM     struct{ Config []struct{ Subnet string } }
	}
	if err := c.get(ctx, "/networks", nil, &raw); err != nil {
		return nil, err
	}
	users := map[string][]string{}
	for _, ct := range ctrs {
		for _, n := range ct.Networks {
			users[n] = append(users[n], ct.Name)
		}
	}
	out := make([]Network, 0, len(raw))
	for _, r := range raw {
		n := Network{ID: r.ID, Name: r.Name, Driver: r.Driver, Scope: r.Scope, Internal: r.Internal, Created: r.Created,
			Subnets: []string{}, Containers: users[r.Name], Project: r.Labels["com.docker.compose.project"],
			Builtin: r.Name == "bridge" || r.Name == "host" || r.Name == "none"}
		if n.Containers == nil {
			n.Containers = []string{}
		}
		for _, cfg := range r.IPAM.Config {
			if cfg.Subnet != "" {
				n.Subnets = append(n.Subnets, cfg.Subnet)
			}
		}
		out = append(out, n)
	}
	slices.SortFunc(out, func(a, b Network) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

func (c *Client) RemoveNetwork(ctx context.Context, id string) error {
	return c.call(ctx, http.MethodDelete, "/networks/"+url.PathEscape(id), nil, nil, nil)
}

// DF is `docker system df` in bytes, bucketed like the freelance dashboard's server page.
type DF struct {
	Images      Bucket `json:"images"`
	Containers  Bucket `json:"containers"`
	Volumes     Bucket `json:"volumes"`
	BuildCache  Bucket `json:"build_cache"`
	TotalBytes  int64  `json:"total_bytes"`
	Reclaimable int64  `json:"reclaimable_bytes"`

	volumeSizes map[string]int64
	images      []rawImage
}

type Bucket struct {
	Count       int   `json:"count"`
	Active      int   `json:"active"`
	Bytes       int64 `json:"bytes"`
	Reclaimable int64 `json:"reclaimable_bytes"`
}

func (c *Client) DiskUsage(ctx context.Context) (DF, error) {
	var df struct {
		LayersSize int64
		Images     []rawImage
		Containers []struct {
			SizeRw int64
			State  string
		}
		Volumes []struct {
			Name      string
			UsageData struct{ Size, RefCount int64 }
		}
		BuildCache []struct {
			Size   int64
			InUse  bool
			Shared bool
		}
	}
	if err := c.get(ctx, "/system/df", nil, &df); err != nil {
		return DF{}, err
	}
	u := DF{volumeSizes: map[string]int64{}, images: df.Images}
	u.Images.Bytes = df.LayersSize
	for _, img := range df.Images {
		u.Images.Count++
		if img.Containers > 0 {
			u.Images.Active++
		} else {
			u.Images.Reclaimable += img.Size - max(img.SharedSize, 0)
		}
	}
	for _, c := range df.Containers {
		u.Containers.Count++
		u.Containers.Bytes += c.SizeRw
		if c.State == "running" {
			u.Containers.Active++
		} else {
			u.Containers.Reclaimable += c.SizeRw
		}
	}
	for _, v := range df.Volumes {
		u.Volumes.Count++
		if v.UsageData.Size >= 0 {
			u.volumeSizes[v.Name] = v.UsageData.Size
			u.Volumes.Bytes += v.UsageData.Size
		}
		if v.UsageData.RefCount > 0 {
			u.Volumes.Active++
		} else if v.UsageData.Size > 0 {
			u.Volumes.Reclaimable += v.UsageData.Size
		}
	}
	for _, c := range df.BuildCache {
		u.BuildCache.Count++
		if c.Shared {
			continue // counted with the images
		}
		u.BuildCache.Bytes += c.Size
		if c.InUse {
			u.BuildCache.Active++
		} else {
			u.BuildCache.Reclaimable += c.Size
		}
	}
	for _, b := range []Bucket{u.Images, u.Containers, u.Volumes, u.BuildCache} {
		u.TotalBytes += b.Bytes
		u.Reclaimable += b.Reclaimable
	}
	return u, nil
}

// PruneKind is what a prune clears. Volumes are deliberately absent: a volume
// prune deletes data, so volumes are removed one at a time.
type PruneKind string

const (
	PruneContainers PruneKind = "containers" // stopped containers
	PruneImages     PruneKind = "images"     // images no container uses, tagged or not
	PruneDangling   PruneKind = "dangling"   // untagged images only
	PruneBuildCache PruneKind = "build-cache"
	PruneNetworks   PruneKind = "networks"
)

type PruneResult struct {
	Removed   int   `json:"removed"`
	Reclaimed int64 `json:"reclaimed"`
}

func (c *Client) Prune(ctx context.Context, kind PruneKind) (PruneResult, error) {
	var path string
	var q url.Values
	switch kind {
	case PruneContainers:
		path = "/containers/prune"
	case PruneImages:
		path, q = "/images/prune", filters(map[string][]string{"dangling": {"false"}})
	case PruneDangling:
		path = "/images/prune"
	case PruneBuildCache:
		path, q = "/build/prune", url.Values{"all": {"true"}}
	case PruneNetworks:
		path = "/networks/prune"
	default:
		return PruneResult{}, fmt.Errorf("unknown prune %q", kind)
	}
	var r struct {
		ContainersDeleted []string
		ImagesDeleted     []json.RawMessage
		CachesDeleted     []string
		NetworksDeleted   []string
		SpaceReclaimed    int64
	}
	if err := c.call(ctx, http.MethodPost, path, q, nil, &r); err != nil {
		return PruneResult{}, err
	}
	n := len(r.ContainersDeleted) + len(r.ImagesDeleted) + len(r.CachesDeleted) + len(r.NetworksDeleted)
	return PruneResult{Removed: n, Reclaimed: r.SpaceReclaimed}, nil
}

// HasImage reports whether ref is present locally.
func (c *Client) HasImage(ctx context.Context, ref string) (bool, error) {
	err := c.call(ctx, http.MethodGet, "/images/"+url.PathEscape(ref)+"/json", nil, nil, nil)
	if IsNotFound(err) {
		return false, nil
	}
	return err == nil, err
}

// ImageSize is the size of a local image, or 0 when it isn't pulled.
func (c *Client) ImageSize(ctx context.Context, ref string) (int64, error) {
	var img struct{ Size int64 }
	err := c.get(ctx, "/images/"+url.PathEscape(ref)+"/json", nil, &img)
	if IsNotFound(err) {
		return 0, nil
	}
	return img.Size, err
}

// Pull fetches an image, writing Docker's progress messages to w.
func (c *Client) Pull(ctx context.Context, ref string, w io.Writer) error {
	name, tag, _ := strings.Cut(ref, ":")
	if tag == "" {
		tag = "latest"
	}
	resp, err := c.do(ctx, http.MethodPost, "/images/create", url.Values{"fromImage": {name}, "tag": {tag}}, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	for {
		var m struct {
			Status, ID, Error string
		}
		if err := dec.Decode(&m); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
		if m.Error != "" {
			return fmt.Errorf("pull %s: %s", ref, m.Error)
		}
		if w != nil && m.Status != "" && !strings.HasPrefix(m.Status, "Downloading") && !strings.HasPrefix(m.Status, "Extracting") {
			fmt.Fprintf(w, "%s %s\n", m.ID, m.Status)
		}
	}
}

// RunSpec is a throwaway container Harbor starts for itself.
type RunSpec struct {
	Image      string
	Cmd        []string
	WorkingDir string
	Binds      []string
	Env        []string
}

// Run creates and starts a helper container, streams its output to w, waits for
// it to exit and removes it. It returns the exit code.
func (c *Client) Run(ctx context.Context, spec RunSpec, w io.Writer) (int, error) {
	body := map[string]any{
		"Image": spec.Image, "Cmd": spec.Cmd, "WorkingDir": spec.WorkingDir, "Env": spec.Env,
		"Labels":     map[string]string{LabelHelper: "1"},
		"HostConfig": map[string]any{"Binds": spec.Binds, "NetworkMode": "host"},
	}
	var created struct{ Id string }
	if err := c.call(ctx, http.MethodPost, "/containers/create", url.Values{"name": {"harbor-run-" + randName()}}, body, &created); err != nil {
		return -1, err
	}
	id := created.Id
	defer func() {
		// Clean up even when the caller has gone away.
		c.call(context.WithoutCancel(ctx), http.MethodDelete, "/containers/"+id, url.Values{"force": {"1"}}, nil, nil)
	}()
	if err := c.call(ctx, http.MethodPost, "/containers/"+id+"/start", nil, nil, nil); err != nil {
		return -1, err
	}
	logErr := c.Logs(ctx, id, "all", "", true, false, func(l LogLine) error {
		_, err := io.WriteString(w, l.L+"\n")
		return err
	})
	var waited struct{ StatusCode int }
	if err := c.call(ctx, http.MethodPost, "/containers/"+id+"/wait", nil, nil, &waited); err != nil {
		return -1, err
	}
	return waited.StatusCode, logErr
}

func randName() string {
	var b [6]byte
	rand.Read(b[:])
	return fmt.Sprintf("%x", b)
}

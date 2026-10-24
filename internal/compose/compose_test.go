package compose

import (
	"slices"
	"testing"

	"github.com/stunsails/stunsails/internal/docker"
	"github.com/stunsails/stunsails/internal/store"
)

func ctr(name, project, service, state string, extra map[string]string) docker.Container {
	l := map[string]string{docker.LabelProject: project, docker.LabelService: service,
		docker.LabelWorkingDir: "/srv/" + project, docker.LabelConfig: "/srv/" + project + "/compose.yml"}
	for k, v := range extra {
		l[k] = v
	}
	return docker.Container{ID: name, Name: name, Project: project, Service: service, State: state, Labels: l}
}

func TestBuild(t *testing.T) {
	ctrs := []docker.Container{
		ctr("web-1", "shop", "web", "running", nil),
		ctr("db-1", "shop", "db", "exited", nil),
		ctr("api-1", "blog", "api", "running", nil),
		ctr("run-1", "blog", "api", "exited", map[string]string{docker.LabelOneoff: "True"}),
		{ID: "loose", Name: "loose", State: "running"},
	}
	known := []store.StackRow{{Project: "old", WorkingDir: "/srv/old", ConfigFiles: []string{"/srv/old/compose.yml"}}}
	got := Build(ctrs, known)
	names := []string{}
	for _, s := range got {
		names = append(names, s.Project+":"+s.State)
	}
	if want := []string{"blog:running", "old:down", "shop:partial"}; !slices.Equal(names, want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	if got[2].Total != 2 || got[2].Running != 1 || len(got[2].Services) != 2 || !got[2].Manageable {
		t.Fatalf("shop stack wrong: %+v", got[2])
	}
	if got[0].Total != 1 {
		t.Fatal("one-off run containers must not count")
	}
}

func TestPathsAndBinds(t *testing.T) {
	s := Stack{Project: "shop", WorkingDir: "/srv/shop", ConfigFiles: []string{"/srv/shop/compose.yml", "/etc/shared/base.yml"}, EnvFiles: []string{"/srv/shop/.env"}}
	if err := paths(s); err != nil {
		t.Fatal(err)
	}
	want := []string{"/var/run/docker.sock:/var/run/docker.sock", "/srv/shop:/srv/shop:ro", "/etc/shared:/etc/shared:ro"}
	if got := Binds(s); !slices.Equal(got, want) {
		t.Fatalf("binds %v", got)
	}
	for _, bad := range []Stack{
		{Project: "Shop", WorkingDir: "/srv/shop", ConfigFiles: []string{"/srv/shop/c.yml"}},
		{Project: "shop", WorkingDir: "/", ConfigFiles: []string{"/c.yml"}},
		{Project: "shop", WorkingDir: "/srv/../etc", ConfigFiles: []string{"/srv/c.yml"}},
		{Project: "shop", WorkingDir: "/srv/a:b", ConfigFiles: []string{"/srv/c.yml"}},
		{Project: "shop", WorkingDir: "/srv/shop"},
	} {
		if paths(bad) == nil {
			t.Errorf("want error for %+v", bad)
		}
	}
	cmd := Command(s, "up", "-d")
	if want := []string{"docker", "compose", "-p", "shop", "--project-directory", "/srv/shop", "-f", "/srv/shop/compose.yml", "-f", "/etc/shared/base.yml", "--env-file", "/srv/shop/.env", "up", "-d"}; !slices.Equal(cmd, want) {
		t.Fatalf("cmd %v", cmd)
	}
}

func TestStartOrder(t *testing.T) {
	// app depends on guard (FDS's shape); web depends on app and a service with no containers.
	ctrs := []docker.Container{
		ctr("web-1", "fds", "web", "running", map[string]string{docker.LabelDependsOn: "app:service_started:false,gone:service_started:false"}),
		ctr("app-1", "fds", "app", "running", map[string]string{docker.LabelDependsOn: "guard:service_healthy:false"}),
		ctr("guard-1", "fds", "guard", "running", nil),
	}
	got := []string{}
	for _, c := range StartOrder(Build(ctrs, nil)[0]) {
		got = append(got, c.Name)
	}
	if want := []string{"guard-1", "app-1", "web-1"}; !slices.Equal(got, want) {
		t.Fatalf("start order %v, want %v", got, want)
	}
	// A cycle still lists every container once.
	cyc := []docker.Container{
		ctr("a-1", "x", "a", "running", map[string]string{docker.LabelDependsOn: "b:service_started:false"}),
		ctr("b-1", "x", "b", "running", map[string]string{docker.LabelDependsOn: "a:service_started:false"}),
	}
	if n := len(StartOrder(Build(cyc, nil)[0])); n != 2 {
		t.Fatalf("cycle listed %d containers, want 2", n)
	}
}

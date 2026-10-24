package setup

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"strings"
	"syscall"
	"time"
)

// Check is one thing init and doctor look at.
type Check struct {
	Name   string
	OK     bool
	Detail string // what was found
	Fix    string // what to do when it isn't OK
}

const cgroup2Magic = 0x63677270 // CGROUP2_SUPER_MAGIC

// Host checks what StunSails needs from the machine itself. It changes nothing.
func Host() []Check {
	d := ReadDistro()
	out := []Check{
		{Name: "Run as root", OK: os.Geteuid() == 0, Fix: "Run with sudo."},
		distroCheck(d),
		{Name: "systemd", OK: exists("/run/systemd/system"), Fix: "StunSails runs as a systemd service; this host doesn't boot with systemd."},
		cgroupCheck(),
		dockerCheck(),
		authLogCheck(),
	}
	return out
}

func distroCheck(d Distro) Check {
	c := Check{Name: "Supported system", OK: d.Supported(), Detail: d.Pretty}
	if !c.OK {
		c.Fix = "Tested on Ubuntu, Debian and Fedora. Others may work; continue at your own risk."
	}
	return c
}

func cgroupCheck() Check {
	var st syscall.Statfs_t
	err := syscall.Statfs("/sys/fs/cgroup", &st)
	c := Check{Name: "cgroup v2", OK: err == nil && st.Type == cgroup2Magic}
	if !c.OK {
		c.Fix = "Per-container readings need cgroup v2 (the default since Ubuntu 21.10, Debian 11 and Fedora 31)."
	}
	return c
}

func dockerCheck() Check {
	c := Check{Name: "Docker Engine"}
	path, err := exec.LookPath("docker")
	if err != nil {
		c.Fix = "Install Docker Engine; `stunsails init` can do it from Docker's own repository."
		return c
	}
	out, err := exec.Command(path, "version", "--format", "{{.Server.Version}} {{.Server.Os}}").Output()
	v := strings.TrimSpace(string(out))
	switch {
	case err != nil:
		c.Fix = "The docker command is installed but the engine doesn't answer: `sudo systemctl enable --now docker`."
	case strings.Contains(strings.ToLower(v), "podman"):
		c.Detail, c.Fix = v, "This is Podman's docker shim. StunSails needs Docker Engine."
	default:
		c.OK, c.Detail = true, v
	}
	return c
}

func authLogCheck() Check {
	c := Check{Name: "SSH sign-in log", OK: true}
	switch {
	case exists("/var/log/auth.log"):
		c.Detail = "/var/log/auth.log"
	case exists("/var/log/journal") || exists("/run/log/journal"):
		c.Detail = "journal only"
		c.OK = false
		c.Fix = "No /var/log/auth.log here; SSH activity needs the journal reader that comes in a later release, or `sudo apt install rsyslog`."
	default:
		c.OK, c.Fix = false, "No auth log found; SSH and sudo activity will be empty."
	}
	return c
}

// Installed checks an installed service. It changes nothing.
func Installed(listen string) []Check {
	var out []Check
	if exists("/etc/systemd/system/server-monitor.service") {
		out = append(out, Check{Name: "Moved from Harbor", Detail: "server-monitor is still installed",
			Fix: "`sudo stunsails init` moves its data, credentials and phone pairing over."})
	}
	out = append(out, []Check{
		{Name: "Binary", OK: exists(BinPath), Detail: BinPath, Fix: "Run `sudo stunsails init`."},
		{Name: "Service unit", OK: exists(UnitPath), Detail: UnitPath, Fix: "Run `sudo stunsails init`."},
		userCheck(),
		credCheck("password"),
		credCheck("totp"),
		activeCheck(),
	}...)
	h := Check{Name: "Dashboard answers", Detail: "http://" + listen}
	if err := Healthy(listen, 3*time.Second); err != nil {
		h.Fix = fmt.Sprintf("%v. See `journalctl -u stunsails -n 50`.", err)
	} else {
		h.OK = true
	}
	return append(out, h)
}

func userCheck() Check {
	c := Check{Name: "Service user", Fix: "Run `sudo stunsails init`."}
	u, err := user.Lookup(User)
	if err != nil {
		return c
	}
	gids, _ := u.GroupIds()
	var names []string
	for _, g := range gids {
		if gr, err := user.LookupGroupId(g); err == nil {
			names = append(names, gr.Name)
		}
	}
	c.OK, c.Detail = true, User+" ("+strings.Join(names, ", ")+")"
	return c
}

func credCheck(name string) Check {
	p := ConfDir + "/" + name
	c := Check{Name: "Credential " + name, Detail: p}
	st, err := os.Stat(p)
	switch {
	case err != nil:
		c.Fix = "Missing. Run `sudo stunsails " + map[string]string{"password": "passwd", "totp": "enroll"}[name] + "`."
	case st.Mode().Perm()&0o077 != 0:
		c.Fix = fmt.Sprintf("Mode %o lets others read it: `sudo chmod 600 %s`.", st.Mode().Perm(), p)
	default:
		c.OK = true
	}
	return c
}

func activeCheck() Check {
	out, _ := exec.Command("systemctl", "is-active", Service).Output()
	state := strings.TrimSpace(string(out))
	return Check{Name: "Service running", OK: state == "active", Detail: state, Fix: "`sudo systemctl start stunsails`, then `journalctl -u stunsails -n 50`."}
}

// Healthy waits up to wait for the dashboard to answer its unauthenticated
// session endpoint.
func Healthy(listen string, wait time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), wait)
	defer cancel()
	var last error
	for {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+listen+"/api/session", nil)
		resp, err := http.DefaultClient.Do(req)
		if err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			err = fmt.Errorf("status %d", resp.StatusCode)
		}
		last = err
		select {
		case <-ctx.Done():
			return fmt.Errorf("no answer on %s: %v", listen, last)
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// Print writes checks as a list and reports whether all passed.
func Print(w io.Writer, cs []Check) bool {
	all := true
	for _, c := range cs {
		mark := "ok  "
		if !c.OK {
			mark, all = "FAIL", false
		}
		line := fmt.Sprintf("  %s  %s", mark, c.Name)
		if c.Detail != "" {
			line += ": " + c.Detail
		}
		fmt.Fprintln(w, line)
		if !c.OK && c.Fix != "" {
			fmt.Fprintln(w, "        "+c.Fix)
		}
	}
	return all
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

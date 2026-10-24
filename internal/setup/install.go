package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/ikhsansdqq/stunsails/deploy"
)

const (
	User     = "stunsails"
	Service  = "stunsails"
	BinPath  = "/usr/local/bin/stunsails"
	UnitPath = "/etc/systemd/system/stunsails.service"
	ConfDir  = "/etc/stunsails"
	StateDir = "/var/lib/stunsails"
	Listen   = "127.0.0.1:8099" // the unit's LISTEN
)

// Options are init's flags.
type Options struct {
	Binary          string // the binary to install; default: this one
	NoDockerInstall bool
	Domain          string // for the reverse proxy snippet
	// Passwd and Enroll set the password and pair an authenticator; they live
	// in package main, which also serves `stunsails passwd` and `enroll`.
	Passwd func(path string) error
	Enroll func(path string) error
}

// Init installs StunSails, or upgrades an install made from a checkout.
func Init(ui *UI, o Options) error {
	ui.Say("StunSails setup. Every command that changes this server is shown before it runs.")

	ui.Step(1, "Checking this server")
	d := ReadDistro()
	host := Host()
	Print(ui.Out, host)
	for _, c := range host {
		switch c.Name {
		case "Run as root", "systemd", "cgroup v2":
			if !c.OK {
				return fmt.Errorf("%s: %s", c.Name, c.Fix)
			}
		case "Supported system":
			if !c.OK && !ui.Confirm("Continue anyway?", false) {
				return errors.New("stopped")
			}
		}
	}

	ui.Step(2, "Docker Engine")
	if c := dockerCheck(); c.OK {
		ui.Say("  Docker %s is running.", c.Detail)
	} else if strings.Contains(c.Detail, "podman") {
		return errors.New(c.Fix)
	} else if o.NoDockerInstall {
		return errors.New("Docker isn't running and --no-docker-install was given: " + c.Fix)
	} else if ui.Confirm("Docker isn't running. Install Docker Engine from Docker's repository now?", true) {
		if err := InstallDocker(ui, d); err != nil {
			return err
		}
	} else {
		return errors.New("StunSails needs Docker Engine")
	}

	ui.Step(3, "Service user and folders")
	migrated, err := migrateHarbor(ui)
	if err != nil {
		return err
	}
	if err := ensureUser(ui); err != nil {
		return err
	}
	if err := os.MkdirAll(ConfDir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(ConfDir, 0o700); err != nil {
		return err
	}

	ui.Step(4, "Sign-in")
	pw, totp := filepath.Join(ConfDir, "password"), filepath.Join(ConfDir, "totp")
	if nonEmpty(pw) {
		ui.Say("  Keeping the password already set.")
	} else {
		ui.Say("  Choose the dashboard password (12 characters or more).")
		if err := o.Passwd(pw); err != nil {
			return err
		}
	}
	if validTOTP(totp) {
		ui.Say("  Keeping the authenticator already paired.")
	} else {
		ui.Say("  Pair an authenticator app (Google Authenticator, Aegis, 1Password, …).")
		if err := o.Enroll(totp); err != nil {
			return err
		}
	}

	ui.Step(5, "Compose helper image")
	// Pulled now so the first stack action doesn't wait for it.
	if err := ui.Run("docker", "pull", "-q", "docker:29-cli"); err != nil {
		ui.Say("  Couldn't pull it now (%v); the first compose action will.", err)
	}

	ui.Step(6, "Installing the service")
	if err := installBinary(ui, o.Binary); err != nil {
		return err
	}
	relabel(ui, BinPath)
	if err := writeUnit(ui); err != nil {
		return err
	}
	if err := ui.Run("systemctl", "daemon-reload"); err != nil {
		return err
	}
	if err := ui.Run("systemctl", "enable", Service); err != nil {
		return err
	}
	if err := ui.Run("systemctl", "restart", Service); err != nil {
		return err
	}
	if err := Healthy(Listen, 15*time.Second); err != nil {
		return fmt.Errorf("installed, but %v; see `journalctl -u stunsails -n 50`", err)
	}
	ui.Say("  Running on http://%s (loopback only).", Listen)

	ui.Step(7, "Reaching it from your browser")
	proxyAdvice(ui, o.Domain)
	firewallAdvice(ui)

	if migrated {
		ui.Say("\nMoved from Harbor (server-monitor). Once StunSails works, remove the old user and copies:")
		ui.Say("  sudo userdel server-monitor")
		ui.Say("  sudo rm /usr/local/bin/server-monitor.prev /etc/systemd/system/server-monitor.service.prev")
	}
	ui.Say("\nDone. Check it any time with `sudo stunsails doctor`.")
	return nil
}

// migrateHarbor moves an install from before the rename (Harbor, installed
// as server-monitor) over once. The old binary and unit stay as .prev.
func migrateHarbor(ui *UI) (bool, error) {
	const oldUnit = "/etc/systemd/system/server-monitor.service"
	if !exists(oldUnit) {
		return false, nil
	}
	ui.Say("  Found Harbor (server-monitor); moving its data and credentials to StunSails.")
	ui.Run("systemctl", "disable", "--now", "server-monitor") // already stopped is fine
	if exists("/var/lib/server-monitor") && !exists(StateDir) {
		src, err := filepath.EvalSymlinks("/var/lib/server-monitor")
		if err != nil {
			return false, err
		}
		if err := ui.Run("mv", src, StateDir); err != nil {
			return false, err
		}
		os.Remove("/var/lib/server-monitor") // the symlink, if it was one
	}
	if exists("/etc/server-monitor") && !exists(ConfDir) {
		if err := ui.Run("mv", "/etc/server-monitor", ConfDir); err != nil {
			return false, err
		}
	}
	if err := ui.Run("mv", oldUnit, oldUnit+".prev"); err != nil {
		return false, err
	}
	if exists("/usr/local/bin/server-monitor") {
		if err := ui.Run("mv", "/usr/local/bin/server-monitor", "/usr/local/bin/server-monitor.prev"); err != nil {
			return false, err
		}
	}
	return true, ui.Run("systemctl", "daemon-reload")
}

func ensureUser(ui *UI) error {
	if _, err := user.Lookup(User); err != nil {
		if err := ui.Run("useradd", "--system", "--no-create-home", "--home-dir", "/nonexistent", "--shell", nologin(), User); err != nil {
			return err
		}
	}
	if gs := existingGroups(); len(gs) > 0 {
		if err := ui.Run("usermod", "-aG", strings.Join(gs, ","), User); err != nil {
			return err
		}
	}
	if exists(StateDir) {
		return ui.Run("chown", "-R", User+":"+User, StateDir)
	}
	return nil
}

func nologin() string {
	for _, p := range []string{"/usr/sbin/nologin", "/sbin/nologin"} {
		if exists(p) {
			return p
		}
	}
	return "/bin/false"
}

// existingGroups are the supplementary groups the service wants that exist on
// this host: systemd won't start a unit that names a missing one.
func existingGroups() []string {
	var out []string
	for _, g := range []string{"docker", "adm", "systemd-journal"} {
		if _, err := user.LookupGroup(g); err == nil {
			out = append(out, g)
		}
	}
	return out
}

// installBinary copies src to BinPath, keeping the one it replaces as .prev.
func installBinary(ui *UI, src string) error {
	if src == "" {
		var err error
		if src, err = os.Executable(); err != nil {
			return err
		}
	}
	src, _ = filepath.EvalSymlinks(src)
	if dst, _ := filepath.EvalSymlinks(BinPath); dst == src {
		ui.Say("  %s is already this binary.", BinPath)
		return nil
	}
	if exists(BinPath) {
		if err := copyFile(BinPath, BinPath+".prev", 0o755); err != nil {
			return err
		}
	}
	ui.Say("  Installing %s", BinPath)
	return copyFile(src, BinPath, 0o755)
}

func writeUnit(ui *UI) error {
	t, err := template.New("unit").Parse(deploy.Unit)
	if err != nil {
		return err
	}
	var b strings.Builder
	if err := t.Execute(&b, struct{ Exec, Groups string }{BinPath, strings.Join(existingGroups(), " ")}); err != nil {
		return err
	}
	if exists(UnitPath) {
		if err := copyFile(UnitPath, UnitPath+".prev", 0o644); err != nil {
			return err
		}
	}
	ui.Say("  Writing %s", UnitPath)
	return writeAtomic(UnitPath, []byte(b.String()), 0o644)
}

// relabel gives the binary its proper SELinux label on Fedora: one moved out
// of /tmp keeps user_tmp_t, and systemd won't run it.
func relabel(ui *UI, path string) {
	if !exists("/sys/fs/selinux/enforce") {
		return
	}
	if _, err := exec.LookPath("restorecon"); err == nil {
		ui.Run("restorecon", "-v", path)
	}
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	b, err := io.ReadAll(in)
	if err != nil {
		return err
	}
	return writeAtomic(dst, b, mode)
}

// writeAtomic writes beside dst and renames, so a running binary or a unit
// systemd is reading is never half written.
func writeAtomic(dst string, b []byte, mode os.FileMode) error {
	tmp := dst + ".new"
	if err := os.WriteFile(tmp, b, mode); err != nil {
		return err
	}
	if err := os.Chmod(tmp, mode); err != nil {
		return err
	}
	return os.Rename(tmp, dst)
}

func nonEmpty(p string) bool {
	st, err := os.Stat(p)
	return err == nil && st.Size() > 0
}

// validTOTP matches install.sh's old check: a base32 secret of 16+ characters.
func validTOTP(p string) bool {
	b, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	s := strings.TrimRight(strings.TrimSpace(string(b)), "=")
	if len(s) < 16 {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= '2' && r <= '7') {
			return false
		}
	}
	return true
}

// Package setup installs, checks, upgrades and removes StunSails on a host:
// what `stunsails init`, `doctor`, `upgrade` and `uninstall` do. It replaces
// deploy/install.sh, so the curl installer and a checkout share one path.
package setup

import (
	"bufio"
	"io"
	"os"
	"slices"
	"strings"
)

// Distro is what /etc/os-release says about the host.
type Distro struct {
	ID       string   // "ubuntu", "debian", "fedora"
	Like     []string // ID_LIKE
	Version  string   // VERSION_ID
	Codename string   // VERSION_CODENAME, or UBUNTU_CODENAME on derivatives
	Pretty   string   // PRETTY_NAME
}

// ReadDistro reads /etc/os-release; a missing file gives an empty Distro.
func ReadDistro() Distro {
	f, err := os.Open("/etc/os-release")
	if err != nil {
		return Distro{}
	}
	defer f.Close()
	return ParseOSRelease(f)
}

// ParseOSRelease reads the KEY=value lines of an os-release file.
func ParseOSRelease(r io.Reader) Distro {
	kv := map[string]string{}
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		k, v, ok := strings.Cut(strings.TrimSpace(sc.Text()), "=")
		if !ok || strings.HasPrefix(k, "#") {
			continue
		}
		kv[k] = strings.Trim(v, `"'`)
	}
	d := Distro{ID: kv["ID"], Like: strings.Fields(kv["ID_LIKE"]), Version: kv["VERSION_ID"], Codename: kv["VERSION_CODENAME"], Pretty: kv["PRETTY_NAME"]}
	if c := kv["UBUNTU_CODENAME"]; c != "" {
		d.Codename = c
	}
	return d
}

// Family is "debian" for Debian, Ubuntu and their derivatives, "fedora" for
// Fedora, or "" for anything else.
func (d Distro) Family() string {
	is := func(id string) bool { return d.ID == id || slices.Contains(d.Like, id) }
	switch {
	case is("debian") || is("ubuntu"):
		return "debian"
	case is("fedora"):
		return "fedora"
	}
	return ""
}

// Supported reports whether StunSails is tested on this distro.
func (d Distro) Supported() bool {
	return d.ID == "ubuntu" || d.ID == "debian" || d.ID == "fedora"
}

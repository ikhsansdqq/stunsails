package setup

import (
	"errors"
	"fmt"
	"slices"
)

// InstallDocker installs Docker Engine from Docker's own repository, as
// docs.docker.com describes for each distro, showing every command first.
func InstallDocker(ui *UI, d Distro) error {
	pkgs := "docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin"
	var steps []string
	switch {
	case d.Family() == "debian":
		// Docker publishes repos for ubuntu and debian only; derivatives use their base.
		base := "debian"
		if d.ID == "ubuntu" || slices.Contains(d.Like, "ubuntu") {
			base = "ubuntu"
		}
		if d.Codename == "" {
			return errors.New("can't tell this release's codename from /etc/os-release; install Docker by hand")
		}
		steps = []string{
			"apt-get update",
			"apt-get install -y ca-certificates curl",
			"install -m 0755 -d /etc/apt/keyrings",
			fmt.Sprintf("curl -fsSL https://download.docker.com/linux/%s/gpg -o /etc/apt/keyrings/docker.asc", base),
			"chmod a+r /etc/apt/keyrings/docker.asc",
			fmt.Sprintf(`echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/%s %s stable" > /etc/apt/sources.list.d/docker.list`, base, d.Codename),
			"apt-get update",
			"apt-get install -y " + pkgs,
		}
	case d.ID == "fedora":
		steps = []string{
			"dnf -y install dnf-plugins-core",
			// dnf5 (Fedora 41+) and dnf4 spell config-manager differently.
			"dnf config-manager addrepo --from-repofile=https://download.docker.com/linux/fedora/docker-ce.repo || dnf config-manager --add-repo https://download.docker.com/linux/fedora/docker-ce.repo",
			"dnf -y install " + pkgs,
		}
	default:
		return fmt.Errorf("no automatic Docker install for %q; see https://docs.docker.com/engine/install/", d.ID)
	}
	steps = append(steps, "systemctl enable --now docker")
	for _, s := range steps {
		if err := ui.Shell(s); err != nil {
			return err
		}
	}
	return nil
}

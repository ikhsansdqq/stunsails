package setup

import (
	"archive/tar"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Repo is where releases are published.
const Repo = "ikhsansdqq/stunsails"

// Release asset names; .goreleaser.yaml writes these.
func archiveName(version string) string {
	return fmt.Sprintf("stunsails_%s_linux_%s.tar.gz", strings.TrimPrefix(version, "v"), runtime.GOARCH)
}

const (
	checksumsName = "checksums.txt"
	bundleName    = "checksums.txt.sigstore.json" // cosign's keyless signature of checksums.txt
)

var httpClient = &http.Client{Timeout: 2 * time.Minute}

// Upgrade replaces the installed binary with a release, verified against its
// checksums (and their signature when cosign is installed). If the new one
// doesn't answer within 15 seconds, the old one is put back.
func Upgrade(ui *UI, current, want string, force bool) error {
	if os.Geteuid() != 0 {
		return errors.New("run with sudo")
	}
	if !exists(BinPath) {
		if exists("/usr/bin/stunsails") {
			return errors.New("this install comes from a .deb or .rpm: upgrade it with apt or dnf")
		}
		return errors.New("StunSails isn't installed here; run `sudo stunsails init`")
	}
	tag, err := releaseTag(want)
	if err != nil {
		return err
	}
	if tag == current && !force {
		ui.Say("Already on %s.", tag)
		return nil
	}
	ui.Say("Upgrading %s → %s", current, tag)

	dir, err := os.MkdirTemp("", "stunsails-upgrade-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	base := fmt.Sprintf("https://github.com/%s/releases/download/%s/", Repo, tag)
	arch := archiveName(tag)
	for _, f := range []string{arch, checksumsName} {
		if err := download(base+f, filepath.Join(dir, f)); err != nil {
			return err
		}
	}
	if err := verifySignature(ui, dir, base); err != nil {
		return err
	}
	if err := verifyChecksum(dir, arch); err != nil {
		return err
	}
	ui.Say("  Checksum matches.")
	bin := filepath.Join(dir, "stunsails")
	if err := extract(filepath.Join(dir, arch), "stunsails", bin); err != nil {
		return err
	}

	if err := copyFile(BinPath, BinPath+".prev", 0o755); err != nil {
		return err
	}
	if err := copyFile(bin, BinPath, 0o755); err != nil {
		return err
	}
	relabel(ui, BinPath)
	if err := writeUnit(ui); err != nil {
		return err
	}
	ui.Run("systemctl", "daemon-reload")
	ui.Run("systemctl", "restart", Service)
	if err := Healthy(Listen, 15*time.Second); err != nil {
		ui.Say("  The new version doesn't answer (%v). Putting %s back.", err, current)
		copyFile(BinPath+".prev", BinPath, 0o755)
		copyFile(UnitPath+".prev", UnitPath, 0o644)
		ui.Run("systemctl", "daemon-reload")
		ui.Run("systemctl", "restart", Service)
		return fmt.Errorf("upgrade to %s rolled back; see `journalctl -u stunsails -n 50`", tag)
	}
	ui.Say("Now on %s. The previous binary is %s.prev.", tag, BinPath)
	return nil
}

// releaseTag is want, or the latest release's tag.
func releaseTag(want string) (string, error) {
	if want != "" {
		if !strings.HasPrefix(want, "v") {
			want = "v" + want
		}
		return want, nil
	}
	resp, err := httpClient.Get("https://api.github.com/repos/" + Repo + "/releases/latest")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("looking up the latest release: %s", resp.Status)
	}
	var r struct {
		Tag string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", err
	}
	if r.Tag == "" {
		return "", errors.New("no release published yet")
	}
	return r.Tag, nil
}

func download(url, dst string) error {
	resp, err := httpClient.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, io.LimitReader(resp.Body, 200<<20)); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// verifySignature checks checksums.txt was signed by this repo's release
// workflow. Without cosign it says so and carries on with the checksum alone.
func verifySignature(ui *UI, dir, base string) error {
	cosign, err := exec.LookPath("cosign")
	if err != nil {
		ui.Say("  cosign isn't installed, so only the checksum is checked. To check the signature too,")
		ui.Say("  install cosign, or run: gh attestation verify %s --repo %s", BinPath, Repo)
		return nil
	}
	bundle := filepath.Join(dir, bundleName)
	if err := download(base+bundleName, bundle); err != nil {
		return err
	}
	err = exec.Command(cosign, "verify-blob",
		"--bundle", bundle,
		"--certificate-identity-regexp", "^https://github.com/"+Repo+"/.github/workflows/release.yml@refs/tags/",
		"--certificate-oidc-issuer", "https://token.actions.githubusercontent.com",
		filepath.Join(dir, checksumsName)).Run()
	if err != nil {
		return errors.New("the release signature doesn't verify; not upgrading")
	}
	ui.Say("  Signature verified (signed by %s's release workflow).", Repo)
	return nil
}

func verifyChecksum(dir, name string) error {
	f, err := os.Open(filepath.Join(dir, checksumsName))
	if err != nil {
		return err
	}
	defer f.Close()
	var want string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if sum, file, ok := strings.Cut(strings.TrimSpace(sc.Text()), "  "); ok && file == name {
			want = sum
		}
	}
	if want == "" {
		return fmt.Errorf("%s isn't listed in %s", name, checksumsName)
	}
	a, err := os.Open(filepath.Join(dir, name))
	if err != nil {
		return err
	}
	defer a.Close()
	h := sha256.New()
	if _, err := io.Copy(h, a); err != nil {
		return err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		return fmt.Errorf("checksum of %s doesn't match; not upgrading", name)
	}
	return nil
}

// extract writes the archive member called name to dst.
func extract(archive, name, dst string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	tr := tar.NewReader(gz)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return fmt.Errorf("%s not found in the archive", name)
		}
		if err != nil {
			return err
		}
		if filepath.Base(h.Name) != name || h.Typeflag != tar.TypeReg {
			continue
		}
		out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, io.LimitReader(tr, 200<<20)); err != nil {
			out.Close()
			return err
		}
		return out.Close()
	}
}

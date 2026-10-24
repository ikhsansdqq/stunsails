package setup

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"text/template"

	"github.com/ikhsansdqq/stunsails/deploy"
)

func TestParseOSRelease(t *testing.T) {
	cases := []struct {
		name, in, id, family, codename string
		supported                      bool
	}{
		{"ubuntu", "ID=ubuntu\nID_LIKE=debian\nVERSION_ID=\"24.04\"\nVERSION_CODENAME=noble\nUBUNTU_CODENAME=noble\n", "ubuntu", "debian", "noble", true},
		{"debian", "PRETTY_NAME=\"Debian GNU/Linux 13 (trixie)\"\nID=debian\nVERSION_CODENAME=trixie\n", "debian", "debian", "trixie", true},
		{"fedora", "ID=fedora\nVERSION_ID=41\n", "fedora", "fedora", "", true},
		{"mint takes ubuntu's codename", "ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nVERSION_CODENAME=wilma\nUBUNTU_CODENAME=noble\n", "linuxmint", "debian", "noble", false},
		{"alpine", "ID=alpine\n", "alpine", "", "", false},
	}
	for _, c := range cases {
		d := ParseOSRelease(strings.NewReader(c.in))
		if d.ID != c.id || d.Family() != c.family || d.Codename != c.codename || d.Supported() != c.supported {
			t.Errorf("%s: got %+v family=%q supported=%v", c.name, d, d.Family(), d.Supported())
		}
	}
}

func TestUnitTemplate(t *testing.T) {
	var b strings.Builder
	tpl := template.Must(template.New("unit").Parse(deploy.Unit))
	if err := tpl.Execute(&b, struct{ Exec, Groups string }{BinPath, "docker systemd-journal"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"ExecStart=/usr/local/bin/stunsails serve", "SupplementaryGroups=docker systemd-journal", "LoadCredential=totp:/etc/stunsails/totp"} {
		if !strings.Contains(b.String(), want) {
			t.Errorf("unit lacks %q", want)
		}
	}
	if strings.Contains(b.String(), "{{") {
		t.Error("unit still has template markers")
	}
}

func TestValidTOTP(t *testing.T) {
	dir := t.TempDir()
	for in, want := range map[string]bool{
		"JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP\n": true,
		"JBSWY3DPEHPK3PXP====\n":             true,
		"short\n":                            false,
		"jbswy3dpehpk3pxp\n":                 false, // lower case isn't what enroll writes
		"JBSWY3DPEHPK3PX1\n":                 false, // 1 isn't base32
	} {
		p := filepath.Join(dir, "totp")
		os.WriteFile(p, []byte(in), 0o600)
		if got := validTOTP(p); got != want {
			t.Errorf("validTOTP(%q) = %v", in, got)
		}
	}
	if validTOTP(filepath.Join(dir, "missing")) {
		t.Error("a missing file counts as paired")
	}
}

func TestChecksumAndExtract(t *testing.T) {
	dir := t.TempDir()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range map[string]string{"LICENSE": "agpl", "stunsails": "#!binary"} {
		tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})
		tw.Write([]byte(body))
	}
	tw.Close()
	gz.Close()
	name := "stunsails_1.2.3_linux_amd64.tar.gz"
	os.WriteFile(filepath.Join(dir, name), buf.Bytes(), 0o644)
	sum := sha256.Sum256(buf.Bytes())
	os.WriteFile(filepath.Join(dir, checksumsName), []byte(hex.EncodeToString(sum[:])+"  "+name+"\n"+strings.Repeat("0", 64)+"  other.tar.gz\n"), 0o644)

	if err := verifyChecksum(dir, name); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "bin")
	if err := extract(filepath.Join(dir, name), "stunsails", out); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(out); string(b) != "#!binary" {
		t.Errorf("extracted %q", b)
	}

	os.WriteFile(filepath.Join(dir, name), append(buf.Bytes(), 0), 0o644)
	if err := verifyChecksum(dir, name); err == nil {
		t.Error("a changed archive passed the checksum")
	}
	if err := verifyChecksum(dir, "missing.tar.gz"); err == nil {
		t.Error("an unlisted archive passed")
	}
}

func TestArchiveName(t *testing.T) {
	if got := archiveName("v0.4.0"); !strings.HasPrefix(got, "stunsails_0.4.0_linux_") || !strings.HasSuffix(got, ".tar.gz") {
		t.Errorf("archiveName = %q", got)
	}
}

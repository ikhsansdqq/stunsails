// Command stunsails is StunSails: one small service that watches the VPS and
// every Docker container and compose stack on it, and lets its owner manage
// them behind a password and an authenticator code.
//
//	stunsails serve    run the dashboard (what systemd starts)
//	stunsails passwd   set the sign-in password
//	stunsails enroll   pair a new authenticator app
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/term"
	"rsc.io/qr"

	"github.com/stunsails/stunsails/internal/alert"
	"github.com/stunsails/stunsails/internal/api"
	"github.com/stunsails/stunsails/internal/auth"
	"github.com/stunsails/stunsails/internal/cleanup"
	"github.com/stunsails/stunsails/internal/collector"
	"github.com/stunsails/stunsails/internal/compose"
	"github.com/stunsails/stunsails/internal/docker"
	"github.com/stunsails/stunsails/internal/geo"
	"github.com/stunsails/stunsails/internal/host"
	"github.com/stunsails/stunsails/internal/hostlog"
	"github.com/stunsails/stunsails/internal/store"
	"github.com/stunsails/stunsails/internal/uptime"
	"github.com/stunsails/stunsails/web"
)

const confDir = "/etc/stunsails"

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func main() {
	log.SetFlags(0)
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}
	var err error
	switch cmd {
	case "serve":
		err = serve()
	case "passwd":
		err = passwd(env("PASSWORD_FILE", filepath.Join(confDir, "password")))
	case "enroll":
		err = enroll(env("TOTP_FILE", filepath.Join(confDir, "totp")), len(os.Args) > 2 && os.Args[2] == "--show")
	default:
		err = fmt.Errorf("unknown command %q (serve, passwd, enroll)", cmd)
	}
	if err != nil {
		log.Fatal(err)
	}
}

// credential reads a file systemd passed with LoadCredential, or the explicit
// path in fileEnv for development.
func credential(name, fileEnv string) string {
	path := os.Getenv(fileEnv)
	if path == "" {
		if dir := os.Getenv("CREDENTIALS_DIRECTORY"); dir != "" {
			path = filepath.Join(dir, name)
		} else {
			path = filepath.Join(confDir, name)
		}
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			log.Printf("reading %s: %v", name, err)
		}
		return ""
	}
	return strings.TrimSpace(string(b))
}

func serve() error {
	dataDir := env("STATE_DIRECTORY", env("DATA_DIR", "."))
	if err := renameLegacyDB(dataDir); err != nil {
		return err
	}
	st, err := store.Open(filepath.Join(dataDir, "stunsails.db"))
	if err != nil {
		return fmt.Errorf("open state: %w", err)
	}
	defer st.Close()

	var totp *auth.TOTP
	if secret := credential("totp", "TOTP_FILE"); secret != "" {
		if totp, err = auth.ParseSecret(secret); err != nil {
			return err
		}
	} else {
		log.Print("no authenticator secret yet: run `sudo stunsails enroll`")
	}
	pw := credential("password", "PASSWORD_FILE")
	if pw == "" {
		log.Print("no password yet: run `sudo stunsails passwd`")
	}

	dc := docker.New(env("DOCKER_SOCKET", "/var/run/docker.sock"))
	sampler := &host.Sampler{Proc: env("PROC", "/proc"), Mounts: []string{"/", "/var/lib/docker"}}
	hostname, _ := os.Hostname()
	alerts := alert.New(st, hostname)
	col := &collector.Collector{Docker: dc, Store: st, Sampler: sampler, Cgroup: env("CGROUP", "/sys/fs/cgroup"), Alerts: alerts}
	hl := &hostlog.Watcher{Store: st, Alerts: alerts, AuthLog: env("AUTH_LOG", "/var/log/auth.log")}
	sites := &uptime.Checker{Store: st, Alerts: alerts, Caddyfile: env("CADDYFILE", "/etc/caddy/Caddyfile")}
	sched := &cleanup.Scheduler{Docker: dc, Store: st, Alerts: alerts}

	static, err := fs.Sub(web.Dist, "dist")
	if err != nil {
		return err
	}
	geoDB := &geo.DB{Dir: filepath.Join(dataDir, "geo")}
	srv := &api.Server{
		Docker: dc, Store: st, Collector: col,
		Host:    &host.Sampler{Proc: sampler.Proc, Mounts: sampler.Mounts},
		Compose: &compose.Runner{Docker: dc, Image: env("COMPOSE_IMAGE", "docker:29-cli")},
		TOTP:    totp, Password: pw, Limiter: auth.NewLimiter(),
		Secure: os.Getenv("INSECURE_COOKIES") != "1",
		Static: static,
		Alerts: alerts, HostLog: hl, Sites: sites, Cleanup: sched, Geo: geoDB, Started: time.Now(),
	}
	sched.OnRun = srv.InvalidateDF
	srv.RestoreLimiter()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	go col.Run(ctx)
	go hl.Run(ctx)
	go geoDB.Run(ctx)
	go sites.Run(ctx)
	go sched.Run(ctx)
	srv.WarmDF(ctx)

	addr := env("LISTEN", "127.0.0.1:8099")
	hs := &http.Server{
		Addr: addr, Handler: srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 2 * time.Minute,
		// No WriteTimeout: logs and compose output stream for as long as they run.
	}
	go func() {
		<-ctx.Done()
		sctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		hs.Shutdown(sctx)
	}()
	log.Printf("stunsails listening on %s", addr)
	if err := hs.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// renameLegacyDB moves the database from before the rename to StunSails
// (harbor.db, with its WAL and shared-memory files) to its new name, once.
func renameLegacyDB(dir string) error {
	newDB, oldDB := filepath.Join(dir, "stunsails.db"), filepath.Join(dir, "harbor.db")
	if _, err := os.Stat(newDB); err == nil {
		return nil
	}
	if _, err := os.Stat(oldDB); err != nil {
		return nil
	}
	for _, suffix := range []string{"-wal", "-shm", ""} { // the main file last
		if err := os.Rename(oldDB+suffix, newDB+suffix); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("rename old database: %w", err)
		}
	}
	log.Print("renamed harbor.db to stunsails.db")
	return nil
}

func readSecret(prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	if term.IsTerminal(int(os.Stdin.Fd())) {
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		return string(b), err
	}
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	return strings.TrimRight(line, "\r\n"), err
}

func passwd(path string) error {
	p1, err := readSecret("New password: ")
	if err != nil {
		return err
	}
	if len(p1) < 12 {
		return errors.New("use at least 12 characters")
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		p2, err := readSecret("Again: ")
		if err != nil {
			return err
		}
		if p1 != p2 {
			return errors.New("the two passwords differ")
		}
	}
	h, err := auth.HashPassword(p1)
	if err != nil {
		return err
	}
	if err := writePrivate(path, h+"\n"); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Saved to %s. Restart to apply: sudo systemctl restart stunsails\n", path)
	return nil
}

func enroll(path string, show bool) error {
	if show {
		// Re-show the existing secret's QR, for pairing a second phone.
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		printQR(strings.TrimSpace(string(b)))
		return nil
	}
	secret := auth.NewSecret()
	printQR(secret)
	t, _ := auth.ParseSecret(secret)
	in := bufio.NewReader(os.Stdin)
	for range 3 {
		fmt.Fprint(os.Stderr, "Enter the 6-digit code the app shows now: ")
		line, err := in.ReadString('\n')
		if err != nil {
			return err
		}
		if t.Verify(line, time.Now()) {
			if err := writePrivate(path, secret+"\n"); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Paired. Saved to %s. Restart to apply: sudo systemctl restart stunsails\n", path)
			return nil
		}
		fmt.Fprintln(os.Stderr, "That code doesn't match. Check the phone's clock and try again.")
	}
	return errors.New("not paired; nothing was changed")
}

func printQR(secret string) {
	host, _ := os.Hostname()
	uri := auth.URI(secret, "StunSails", host)
	code, err := qr.Encode(uri, qr.M)
	if err == nil {
		// Two modules per character cell, light on dark with a quiet zone, so it scans in any terminal theme.
		const quiet = 2
		n := code.Size
		black := func(x, y int) bool { return x >= 0 && y >= 0 && x < n && y < n && code.Black(x, y) }
		var b strings.Builder
		for y := -quiet; y < n+quiet; y += 2 {
			for x := -quiet; x < n+quiet; x++ {
				top, bot := !black(x, y), !black(x, y+1)
				switch {
				case top && bot:
					b.WriteRune('█')
				case top:
					b.WriteRune('▀')
				case bot:
					b.WriteRune('▄')
				default:
					b.WriteRune(' ')
				}
			}
			b.WriteByte('\n')
		}
		fmt.Fprint(os.Stderr, b.String())
	}
	fmt.Fprintf(os.Stderr, "\nScan with Google Authenticator, or add this key by hand:\n  %s\n\n", secret)
}

func writePrivate(path, content string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

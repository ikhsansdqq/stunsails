// Package hostlog watches the host itself, beyond Docker: SSH sign-ins and
// sudo from /var/log/auth.log, and systemd services that have failed.
// Internet-facing SSH sees thousands of password guesses a day, so failed
// logins are summed up every ten minutes instead of recorded one by one.
package hostlog

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/stunsails/stunsails/internal/alert"
	"github.com/stunsails/stunsails/internal/store"
)

// Line is one interesting auth.log entry.
type Line struct {
	At     time.Time
	Kind   string // accepted, failed, sudo
	User   string
	From   string // address, for SSH
	Method string // password, publickey
	Cmd    string // for sudo
}

var (
	acceptedRe = regexp.MustCompile(`sshd\[\d+\]: Accepted (\S+) for (\S+) from (\S+)`)
	failedRe   = regexp.MustCompile(`sshd\[\d+\]: Failed (\S+) for (?:invalid user )?(\S+) from (\S+)`)
	invalidRe  = regexp.MustCompile(`sshd\[\d+\]: Invalid user (\S*) from (\S+)`)
	sudoRe     = regexp.MustCompile(`sudo:\s+(\S+) : .*COMMAND=(.*)$`)
)

// Parse reads one auth.log line; ok is false for lines StunSails ignores.
// An "Invalid user" line is followed by a "Failed … for invalid user" line
// when a password was tried; only the Invalid line is counted, so each
// attempt counts once whichever way it ended.
func Parse(s string, now time.Time) (Line, bool) {
	at := now
	if sp := strings.IndexByte(s, ' '); sp > 0 {
		if t, err := time.Parse(time.RFC3339Nano, s[:sp]); err == nil {
			at = t
		}
	}
	if m := acceptedRe.FindStringSubmatch(s); m != nil {
		return Line{At: at, Kind: "accepted", Method: m[1], User: m[2], From: m[3]}, true
	}
	if m := invalidRe.FindStringSubmatch(s); m != nil {
		return Line{At: at, Kind: "failed", User: m[1], From: m[2]}, true
	}
	if m := failedRe.FindStringSubmatch(s); m != nil && !strings.Contains(s, "for invalid user") {
		return Line{At: at, Kind: "failed", Method: m[1], User: m[2], From: m[3]}, true
	}
	if m := sudoRe.FindStringSubmatch(s); m != nil {
		cmd := strings.TrimSpace(m[2])
		if len(cmd) > 240 {
			cmd = cmd[:240] + "…"
		}
		return Line{At: at, Kind: "sudo", User: m[1], Cmd: cmd}, true
	}
	return Line{}, false
}

// FailedUnit is a systemd unit in the failed state.
type FailedUnit struct {
	Unit        string `json:"unit"`
	Description string `json:"description"`
}

type Watcher struct {
	Store   *store.Store
	Alerts  *alert.Notifier
	AuthLog string // "/var/log/auth.log"
	Every   time.Duration

	mu       sync.Mutex
	failed   []FailedUnit
	fails    map[string]int // failed SSH attempts per address since the last summary
	failUser map[string]int
	ipUsers  map[string]map[string]bool // usernames each address tried
	authErr  string
}

const summaryEvery = 10 * time.Minute

func (w *Watcher) Run(ctx context.Context) {
	w.fails, w.failUser, w.ipUsers = map[string]int{}, map[string]int{}, map[string]map[string]bool{}
	go w.tail(ctx)
	units := time.NewTicker(time.Minute)
	summary := time.NewTicker(summaryEvery)
	defer units.Stop()
	defer summary.Stop()
	w.checkUnits(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-units.C:
			w.checkUnits(ctx)
		case <-summary.C:
			w.summarise(time.Now())
		}
	}
}

// Failed lists units failed at the last check.
func (w *Watcher) Failed() []FailedUnit {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.failed == nil {
		return []FailedUnit{} // not checked yet
	}
	return slices.Clone(w.failed)
}

// AuthError is why auth.log can't be read, or "".
func (w *Watcher) AuthError() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.authErr
}

func (w *Watcher) record(l Line) {
	switch l.Kind {
	case "accepted":
		w.Store.AddActivity(store.Activity{At: l.At.Unix(), Kind: "host", Type: "ssh", Action: "signed in", Name: l.User, IP: l.From, OK: true, Detail: "with " + l.Method})
	case "sudo":
		w.Store.AddActivity(store.Activity{At: l.At.Unix(), Kind: "host", Type: "sudo", Action: "ran", Name: l.User, OK: true, Detail: l.Cmd})
	case "failed":
		w.mu.Lock()
		w.fails[l.From]++
		w.failUser[l.User]++
		if w.ipUsers == nil {
			w.ipUsers = map[string]map[string]bool{}
		}
		if w.ipUsers[l.From] == nil {
			w.ipUsers[l.From] = map[string]bool{}
		}
		if len(w.ipUsers[l.From]) < maxUsersPerIP {
			w.ipUsers[l.From][l.User] = true
		}
		w.mu.Unlock()
	}
}

// summarise records the failed SSH attempts of the last window as one line.
func (w *Watcher) summarise(now time.Time) {
	w.mu.Lock()
	fails, users, ipUsers := w.fails, w.failUser, w.ipUsers
	w.fails, w.failUser, w.ipUsers = map[string]int{}, map[string]int{}, map[string]map[string]bool{}
	w.mu.Unlock()
	total := 0
	for _, n := range fails {
		total += n
	}
	if total == 0 {
		if w.Alerts != nil {
			w.Alerts.Resolve(alert.RuleSSH, "ssh", "Failed SSH logins are back to normal")
		}
		return
	}
	detail := fmt.Sprintf("%s · users tried: %s", top(fails, "attempts", 3), top(users, "", 5))
	w.Store.AddActivity(store.Activity{At: now.Unix(), Kind: "host", Type: "ssh", Action: "failed logins",
		Name: fmt.Sprintf("%d failed SSH logins from %d addresses", total, len(fails)), OK: false, Detail: detail,
		Sources: sources(fails, ipUsers)})
	// Bots try all day; only a burst well above that is worth a message.
	if w.Alerts != nil && total >= 300 {
		w.Alerts.Fire(alert.RuleSSH, "ssh", fmt.Sprintf("%d failed SSH logins in 10 minutes", total), detail, "warning")
	}
}

const (
	maxSources    = 500 // addresses kept per summary, most attempts first
	maxUsersPerIP = 20
)

// sources lists every address of a summary with its attempts and the users it tried.
func sources(fails map[string]int, ipUsers map[string]map[string]bool) []store.Source {
	out := make([]store.Source, 0, len(fails))
	for ip, n := range fails {
		users := make([]string, 0, len(ipUsers[ip]))
		for u := range ipUsers[ip] {
			users = append(users, u)
		}
		slices.Sort(users)
		out = append(out, store.Source{IP: ip, Attempts: n, Users: users})
	}
	slices.SortFunc(out, func(a, b store.Source) int {
		if a.Attempts != b.Attempts {
			return b.Attempts - a.Attempts
		}
		return strings.Compare(a.IP, b.IP)
	})
	if len(out) > maxSources {
		out = out[:maxSources]
	}
	return out
}

// top formats the n largest counts: "1.2.3.4 (40 attempts), …".
func top(m map[string]int, unit string, n int) string {
	type kv struct {
		k string
		v int
	}
	xs := make([]kv, 0, len(m))
	for k, v := range m {
		xs = append(xs, kv{k, v})
	}
	slices.SortFunc(xs, func(a, b kv) int { return b.v - a.v })
	parts := []string{}
	for i, x := range xs {
		if i == n {
			parts = append(parts, fmt.Sprintf("+%d more", len(xs)-n))
			break
		}
		if unit != "" {
			parts = append(parts, fmt.Sprintf("%s (%d %s)", x.k, x.v, unit))
		} else {
			parts = append(parts, x.k)
		}
	}
	return strings.Join(parts, ", ")
}

// tail follows auth.log from its end, reopening it after logrotate.
func (w *Watcher) tail(ctx context.Context) {
	every := w.Every
	if every == 0 {
		every = 2 * time.Second
	}
	var f *os.File
	var r *bufio.Reader
	var ino uint64
	var off int64
	setErr := func(e string) {
		w.mu.Lock()
		w.authErr = e
		w.mu.Unlock()
	}
	open := func(fromEnd bool) bool {
		nf, err := os.Open(w.AuthLog)
		if err != nil {
			setErr(err.Error())
			return false
		}
		st, _ := nf.Stat()
		if f != nil {
			f.Close()
		}
		f, r, ino, off = nf, bufio.NewReaderSize(nf, 64<<10), inode(st), 0
		if fromEnd {
			off, _ = f.Seek(0, io.SeekEnd)
		}
		setErr("")
		return true
	}
	opened := open(true)
	t := time.NewTicker(every)
	defer t.Stop()
	partial := ""
	drain := func() {
		for {
			line, err := r.ReadString('\n')
			off += int64(len(line))
			if err != nil {
				partial += line
				return
			}
			if l, ok := Parse(partial+strings.TrimRight(line, "\n"), time.Now()); ok {
				w.record(l)
			}
			partial = ""
		}
	}
	for {
		if opened {
			drain()
		}
		select {
		case <-ctx.Done():
			if f != nil {
				f.Close()
			}
			return
		case <-t.C:
		}
		// Rotated (new file at the path) or truncated: start the new file from its top.
		if st, err := os.Stat(w.AuthLog); err == nil && (!opened || inode(st) != ino || st.Size() < off) {
			if opened && inode(st) != ino {
				drain() // what was written to the old file just before it moved
			}
			opened = open(false)
			partial = ""
		} else if err != nil {
			setErr(err.Error())
		}
	}
}

func inode(st os.FileInfo) uint64 {
	if st == nil {
		return 0
	}
	if s, ok := st.Sys().(*syscall.Stat_t); ok {
		return s.Ino
	}
	return 0
}

func (w *Watcher) checkUnits(ctx context.Context) {
	cctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	out, err := exec.CommandContext(cctx, "systemctl", "list-units", "--failed", "--output=json", "--no-pager").Output()
	if err != nil {
		return
	}
	units, err := ParseUnits(out)
	if err != nil {
		return
	}
	w.mu.Lock()
	prev := w.failed
	w.failed = units
	first := prev == nil
	w.mu.Unlock()
	if first {
		prev = []FailedUnit{}
	}
	has := func(xs []FailedUnit, u string) bool {
		return slices.ContainsFunc(xs, func(x FailedUnit) bool { return x.Unit == u })
	}
	for _, u := range units {
		if !has(prev, u.Unit) {
			w.Store.AddActivity(store.Activity{Kind: "host", Type: "unit", Action: "failed", Name: u.Unit, OK: false, Detail: u.Description})
		}
		if w.Alerts != nil {
			w.Alerts.Fire(alert.RuleUnit, u.Unit, u.Unit+" failed", u.Description+". Check: journalctl -u "+u.Unit, "critical")
		}
	}
	for _, u := range prev {
		if !has(units, u.Unit) {
			w.Store.AddActivity(store.Activity{Kind: "host", Type: "unit", Action: "recovered", Name: u.Unit, OK: true, Detail: u.Description})
			if w.Alerts != nil {
				w.Alerts.Resolve(alert.RuleUnit, u.Unit, u.Unit+" is running again")
			}
		}
	}
}

// ParseUnits reads `systemctl list-units --output=json`.
func ParseUnits(b []byte) ([]FailedUnit, error) {
	var raw []struct {
		Unit        string `json:"unit"`
		Description string `json:"description"`
		Active      string `json:"active"`
	}
	if err := json.Unmarshal(b, &raw); err != nil {
		return nil, err
	}
	out := []FailedUnit{}
	for _, u := range raw {
		if u.Active == "failed" {
			out = append(out, FailedUnit{u.Unit, u.Description})
		}
	}
	return out, nil
}

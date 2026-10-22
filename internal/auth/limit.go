package auth

import (
	"crypto/rand"
	"encoding/hex"
	"slices"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// CheckPassword compares against a bcrypt hash.
func CheckPassword(hash, password string) bool {
	return hash != "" && bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

func HashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), 12)
	return string(b), err
}

// Token returns n random bytes as hex.
func Token(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Limiter locks an address out after too many failed tries in a window, and
// slows everyone once failures across all addresses pile up (someone rotating
// addresses to dodge the per-address limit). The global cap only slows tries
// down: a hard stop would let anyone lock the owner out by spraying guesses.
type Limiter struct {
	PerIP  int           // failures allowed per address per window
	Global int           // failures across all addresses per window before tries slow down
	Window time.Duration // also how long a lockout lasts
	Slow   time.Duration // the wait added to every try while over Global

	mu    sync.Mutex
	fails map[string][]time.Time
	all   []time.Time
}

func NewLimiter() *Limiter {
	return &Limiter{PerIP: 5, Global: 50, Window: 15 * time.Minute, Slow: 2 * time.Second, fails: map[string][]time.Time{}}
}

func (l *Limiter) prune(ts []time.Time, now time.Time) []time.Time {
	i := 0
	for i < len(ts) && now.Sub(ts[i]) >= l.Window {
		i++
	}
	return ts[i:]
}

// Allowed reports whether ip may try now, and if not, when it may.
func (l *Limiter) Allowed(ip string, now time.Time) (bool, time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.all = l.prune(l.all, now)
	f := l.prune(l.fails[ip], now)
	if len(f) == 0 {
		delete(l.fails, ip)
	} else {
		l.fails[ip] = f
	}
	if len(f) >= l.PerIP {
		return false, f[len(f)-l.PerIP].Add(l.Window)
	}
	return true, time.Time{}
}

// Delay is how long to hold a try back: Slow while failures across all
// addresses are over Global, otherwise nothing.
func (l *Limiter) Delay(now time.Time) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.all = l.prune(l.all, now)
	if len(l.all) >= l.Global {
		return l.Slow
	}
	return 0
}

// Left is how many tries ip has before it's locked out.
func (l *Limiter) Left(ip string, now time.Time) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return max(l.PerIP-len(l.prune(l.fails[ip], now)), 0)
}

// Snapshot is the failures per address, as Unix seconds, to keep across restarts.
func (l *Limiter) Snapshot() map[string][]int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make(map[string][]int64, len(l.fails))
	for ip, ts := range l.fails {
		for _, t := range ts {
			out[ip] = append(out[ip], t.Unix())
		}
	}
	return out
}

// Restore loads a Snapshot taken before a restart.
func (l *Limiter) Restore(m map[string][]int64) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for ip, ts := range m {
		for _, t := range ts {
			l.fails[ip] = append(l.fails[ip], time.Unix(t, 0))
			l.all = append(l.all, time.Unix(t, 0))
		}
	}
	slices.SortFunc(l.all, func(a, b time.Time) int { return a.Compare(b) })
}

func (l *Limiter) Fail(ip string, now time.Time) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.fails[ip] = append(l.fails[ip], now)
	l.all = append(l.all, now)
}

// Reset clears an address after it signs in.
func (l *Limiter) Reset(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.fails, ip)
}

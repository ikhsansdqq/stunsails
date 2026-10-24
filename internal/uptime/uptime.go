// Package uptime checks the sites this server serves: every minute it fetches
// each one over HTTPS as a visitor would, and records whether it answered, how
// fast, and when the certificate visitors see expires. The sites come from
// Caddy's config, plus any added by hand in Settings.
package uptime

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/stunsails/stunsails/internal/alert"
	"github.com/stunsails/stunsails/internal/store"
)

// Site is a site and its latest check.
type Site struct {
	Host     string           `json:"host"`
	Source   string           `json:"source"` // caddy or manual
	Last     *store.SiteCheck `json:"last"`
	Uptime24 float64          `json:"uptime_24h"` // percent, -1 before any check
}

type Checker struct {
	Store     *store.Store
	Alerts    *alert.Notifier
	Caddyfile string // "/etc/caddy/Caddyfile"
	HTTP      *http.Client
	Every     time.Duration

	mu    sync.Mutex
	last  map[string]store.SiteCheck
	fails map[string]int // consecutive failures
	now   func() time.Time
}

const manualKey = "sites"

var hostRe = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?(\.[a-z0-9]([a-z0-9-]*[a-z0-9])?)+$`)

// ParseCaddyfile returns the site hostnames of a Caddyfile: the addresses
// that open a top-level block, minus wildcards, bare ports and snippets.
func ParseCaddyfile(r io.Reader) []string {
	out := []string{}
	depth := 0
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		if i := strings.Index(line, "#"); i >= 0 {
			line = line[:i]
		}
		line = strings.TrimSpace(line)
		if depth == 0 && strings.HasSuffix(line, "{") {
			for _, a := range strings.FieldsFunc(strings.TrimSuffix(line, "{"), func(r rune) bool { return r == ',' || r == ' ' || r == '\t' }) {
				a = strings.TrimPrefix(strings.TrimPrefix(a, "https://"), "http://")
				if h, _, ok := strings.Cut(a, ":"); ok {
					a = h
				}
				a = strings.ToLower(strings.TrimSuffix(a, "/"))
				if hostRe.MatchString(a) && !slices.Contains(out, a) {
					out = append(out, a)
				}
			}
		}
		depth += strings.Count(line, "{") - strings.Count(line, "}")
		if depth < 0 {
			depth = 0
		}
	}
	return out
}

// ValidHost reports whether s is a hostname StunSails will check.
func ValidHost(s string) bool { return hostRe.MatchString(s) }

func (c *Checker) clock() time.Time {
	if c.now != nil {
		return c.now()
	}
	return time.Now()
}

// Manual returns the sites added by hand.
func (c *Checker) Manual() []string {
	out := []string{}
	if raw := c.Store.Meta(manualKey); raw != "" {
		json.Unmarshal([]byte(raw), &out)
	}
	return out
}

func (c *Checker) setManual(xs []string) {
	b, _ := json.Marshal(xs)
	c.Store.SetMeta(manualKey, string(b))
}

func (c *Checker) Add(host string) error {
	host = strings.ToLower(strings.TrimSpace(host))
	host = strings.TrimSuffix(strings.TrimPrefix(strings.TrimPrefix(host, "https://"), "http://"), "/")
	if !ValidHost(host) {
		return errors.New("enter a hostname such as shop.example.com")
	}
	m := c.Manual()
	if slices.Contains(m, host) || slices.Contains(c.caddy(), host) {
		return errors.New("that site is already checked")
	}
	c.setManual(append(m, host))
	go c.check(context.Background(), host)
	return nil
}

func (c *Checker) Remove(host string) error {
	m := c.Manual()
	i := slices.Index(m, host)
	if i < 0 {
		return errors.New("only sites added by hand can be removed; Caddy's come from its config")
	}
	c.setManual(slices.Delete(m, i, i+1))
	c.mu.Lock()
	delete(c.last, host)
	c.mu.Unlock()
	return nil
}

func (c *Checker) caddy() []string {
	f, err := os.Open(c.Caddyfile)
	if err != nil {
		return []string{}
	}
	defer f.Close()
	return ParseCaddyfile(f)
}

// Sites lists every checked site with its latest result.
func (c *Checker) Sites() []Site {
	day := c.clock().Add(-24 * time.Hour).Unix()
	c.mu.Lock()
	last := c.last
	c.mu.Unlock()
	var out []Site
	add := func(h, src string) {
		s := Site{Host: h, Source: src, Uptime24: c.Store.SiteUptime(h, day)}
		c.mu.Lock()
		if l, ok := last[h]; ok {
			s.Last = &l
		}
		c.mu.Unlock()
		out = append(out, s)
	}
	for _, h := range c.caddy() {
		add(h, "caddy")
	}
	for _, h := range c.Manual() {
		add(h, "manual")
	}
	if out == nil {
		out = []Site{}
	}
	return out
}

func (c *Checker) Run(ctx context.Context) {
	c.mu.Lock()
	c.last, c.fails = map[string]store.SiteCheck{}, map[string]int{}
	c.mu.Unlock()
	if c.HTTP == nil {
		c.HTTP = &http.Client{
			Timeout: 10 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return http.ErrUseLastResponse
				}
				return nil
			},
			Transport: &http.Transport{MaxIdleConns: 4, IdleConnTimeout: 90 * time.Second, TLSHandshakeTimeout: 8 * time.Second},
		}
	}
	every := c.Every
	if every == 0 {
		every = time.Minute
	}
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		c.checkAll(ctx)
		select {
		case <-ctx.Done():
			return
		case <-t.C:
		}
	}
}

func (c *Checker) checkAll(ctx context.Context) {
	hosts := append(c.caddy(), c.Manual()...)
	sem := make(chan struct{}, 4)
	var wg sync.WaitGroup
	for _, h := range hosts {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer func() { <-sem; wg.Done() }()
			c.check(ctx, h)
		}()
	}
	wg.Wait()
}

// Probe fetches https://host/ once.
func Probe(ctx context.Context, cl *http.Client, host string, now time.Time) store.SiteCheck {
	res := store.SiteCheck{At: now.Unix()}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://"+host+"/", nil)
	req.Header.Set("User-Agent", "StunSails uptime check")
	start := time.Now()
	resp, err := cl.Do(req)
	res.Ms = time.Since(start).Milliseconds()
	if err != nil {
		res.Error = shortErr(err)
		var cert *tls.CertificateVerificationError
		if errors.As(err, &cert) && len(cert.UnverifiedCertificates) > 0 {
			res.CertExpiry = cert.UnverifiedCertificates[0].NotAfter.Unix()
		}
		return res
	}
	io.Copy(io.Discard, io.LimitReader(resp.Body, 256<<10))
	resp.Body.Close()
	res.Status = resp.StatusCode
	// A page that answers, even with 401 or 404, is up; 5xx means the app behind it isn't.
	res.Up = resp.StatusCode < 500
	if !res.Up {
		res.Error = resp.Status
	}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		res.CertExpiry = resp.TLS.PeerCertificates[0].NotAfter.Unix()
	}
	return res
}

func shortErr(err error) string {
	s := err.Error()
	if i := strings.LastIndex(s, ": "); i >= 0 && strings.Contains(s, "Get \"") {
		s = s[i+2:]
	}
	return s
}

func (c *Checker) check(ctx context.Context, host string) {
	now := c.clock()
	res := Probe(ctx, c.HTTP, host, now)
	if ctx.Err() != nil {
		return
	}
	c.Store.AddSiteCheck(host, res)
	c.mu.Lock()
	c.last[host] = res
	wasDown := c.fails[host] >= 2
	if res.Up {
		c.fails[host] = 0
	} else {
		c.fails[host]++
	}
	fails := c.fails[host]
	c.mu.Unlock()

	// One failed check is often a blip; two in a row is an outage.
	switch {
	case !res.Up && fails == 2:
		c.Store.AddActivity(store.Activity{At: now.Unix(), Kind: "host", Type: "site", Action: "down", Name: host, OK: false, Detail: res.Error})
		if c.Alerts != nil {
			c.Alerts.Fire(alert.RuleSite, host, host+" is down", fmt.Sprintf("Two checks in a row failed: %s.", res.Error), "critical")
		}
	case res.Up && wasDown:
		c.Store.AddActivity(store.Activity{At: now.Unix(), Kind: "host", Type: "site", Action: "back up", Name: host, OK: true, Detail: fmt.Sprintf("%d in %d ms", res.Status, res.Ms)})
		if c.Alerts != nil {
			c.Alerts.Resolve(alert.RuleSite, host, host+" is back up")
		}
	}
	if c.Alerts != nil && res.CertExpiry > 0 {
		days := time.Unix(res.CertExpiry, 0).Sub(now).Hours() / 24
		if days < 14 {
			c.Alerts.Fire(alert.RuleTLS, host, fmt.Sprintf("%s certificate expires in %d days", host, int(days)), "Renew it, or check why Caddy hasn't.", "warning")
		} else {
			c.Alerts.Resolve(alert.RuleTLS, host, host+" certificate renewed")
		}
	}
}

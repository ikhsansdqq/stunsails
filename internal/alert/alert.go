// Package alert tells the owner when something on the server needs them: by
// email over SMTP and by webhook (Discord, Slack, ntfy, n8n). Each problem is
// sent once, again only after a cooldown, and once more when it clears.
package alert

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/smtp"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ikhsansdqq/stunsails/internal/store"
)

// Rules an owner can switch on or off.
const (
	RuleCrash     = "crash"     // a container exited with an error
	RuleUnhealthy = "unhealthy" // a health check is failing
	RuleOOM       = "oom"       // a container ran out of memory
	RuleLoop      = "loop"      // a container keeps restarting
	RuleDisk      = "disk"      // the disk is over 90%, or forecast to fill within a week
	RuleMemory    = "memory"    // memory over 90% for five minutes
	RuleSite      = "site"      // a site is down
	RuleTLS       = "tls"       // a certificate expires within 14 days
	RuleUnit      = "unit"      // a systemd service failed
	RuleSSH       = "ssh"       // a burst of failed SSH logins
	RuleCleanup   = "cleanup"   // the scheduled cleanup ran (a summary, not a problem)
)

// Rules lists every rule with a label, in the order Settings shows them.
var Rules = []struct{ ID, Label string }{
	{RuleCrash, "A container crashes"},
	{RuleLoop, "A container keeps restarting"},
	{RuleUnhealthy, "A health check fails"},
	{RuleOOM, "A container runs out of memory"},
	{RuleDisk, "The disk is nearly full, or will be within a week"},
	{RuleMemory, "Memory stays over 90% for 5 minutes"},
	{RuleSite, "A site stops answering"},
	{RuleTLS, "A certificate expires within 14 days"},
	{RuleUnit, "A system service fails"},
	{RuleSSH, "Many failed SSH logins at once"},
	{RuleCleanup, "The scheduled cleanup ran"},
}

type SMTP struct {
	Enabled  bool     `json:"enabled"`
	Host     string   `json:"host"`
	Port     int      `json:"port"` // 465 is implicit TLS; anything else uses STARTTLS when offered
	User     string   `json:"user"`
	Password string   `json:"password,omitempty"`
	From     string   `json:"from"`
	To       []string `json:"to"`
}

type Webhook struct {
	Enabled bool   `json:"enabled"`
	URL     string `json:"url,omitempty"`
}

type Config struct {
	SMTP    SMTP            `json:"smtp"`
	Webhook Webhook         `json:"webhook"`
	Rules   map[string]bool `json:"rules"`
}

// Public is the config as the browser may see it: secrets replaced by whether they're set.
type Public struct {
	SMTP struct {
		SMTP
		PasswordSet bool `json:"password_set"`
	} `json:"smtp"`
	Webhook struct {
		Enabled bool   `json:"enabled"`
		URLSet  bool   `json:"url_set"`
		Host    string `json:"host"` // only the host of the URL, to recognise it
	} `json:"webhook"`
	Rules    map[string]bool              `json:"rules"`
	RuleList []struct{ ID, Label string } `json:"rule_list"`
	Active   []Active                     `json:"active"`
}

// Active is a problem that has been reported and hasn't cleared.
type Active struct {
	Rule   string `json:"rule"`
	Target string `json:"target"`
	Title  string `json:"title"`
	Since  int64  `json:"since"`
}

const metaKey = "alerts"

// Cooldown is how long the same problem stays quiet after it was sent.
const Cooldown = 30 * time.Minute

type Notifier struct {
	Store *store.Store
	HTTP  *http.Client
	Host  string // the server's name, in subjects
	Now   func() time.Time

	mu     sync.Mutex
	cfg    Config
	active map[string]*state
	send   func(Message) error // replaced in tests
	feed   []FeedItem          // the newest alerts, for a proxying app (Feed)
	seq    int64
}

type state struct {
	Active
	lastSent time.Time
}

// Message is one notification.
type Message struct {
	Rule     string    `json:"rule"`
	Title    string    `json:"title"`
	Text     string    `json:"message"`
	Severity string    `json:"severity"` // critical, warning, resolved, info
	Target   string    `json:"target"`
	At       time.Time `json:"at"`
}

func New(st *store.Store, hostname string) *Notifier {
	n := &Notifier{Store: st, Host: hostname, HTTP: &http.Client{Timeout: 15 * time.Second}, active: map[string]*state{}}
	n.cfg = defaults()
	if raw := st.Meta(metaKey); raw != "" {
		var c Config
		if json.Unmarshal([]byte(raw), &c) == nil {
			n.cfg = merge(defaults(), c)
		}
	}
	return n
}

func defaults() Config {
	rules := map[string]bool{}
	for _, r := range Rules {
		rules[r.ID] = r.ID != RuleCleanup
	}
	return Config{SMTP: SMTP{Port: 587, To: []string{}}, Rules: rules}
}

func merge(base, c Config) Config {
	for k, v := range c.Rules {
		base.Rules[k] = v
	}
	c.Rules = base.Rules
	if c.SMTP.To == nil {
		c.SMTP.To = []string{}
	}
	return c
}

func (n *Notifier) now() time.Time {
	if n.Now != nil {
		return n.Now()
	}
	return time.Now()
}

// Public returns the settings without secrets, and what's currently alerting.
func (n *Notifier) Public() Public {
	n.mu.Lock()
	defer n.mu.Unlock()
	var p Public
	p.SMTP.SMTP = n.cfg.SMTP
	p.SMTP.PasswordSet = n.cfg.SMTP.Password != ""
	p.SMTP.Password = ""
	p.Webhook.Enabled = n.cfg.Webhook.Enabled
	p.Webhook.URLSet = n.cfg.Webhook.URL != ""
	if u, err := url.Parse(n.cfg.Webhook.URL); err == nil {
		p.Webhook.Host = u.Host
	}
	p.Rules = map[string]bool{}
	for k, v := range n.cfg.Rules {
		p.Rules[k] = v
	}
	p.RuleList = Rules
	p.Active = []Active{}
	for _, s := range n.active {
		if s.Title != "" { // one-offs (crashes, cleanups) are only kept for their cooldown
			p.Active = append(p.Active, s.Active)
		}
	}
	slices.SortFunc(p.Active, func(a, b Active) int { return int(a.Since - b.Since) })
	return p
}

// Update is what the Settings form sends. Empty secrets keep the saved ones;
// the Clear flags remove them.
type Update struct {
	SMTP          SMTP            `json:"smtp"`
	Webhook       Webhook         `json:"webhook"`
	Rules         map[string]bool `json:"rules"`
	ClearPassword bool            `json:"clear_password"`
	ClearWebhook  bool            `json:"clear_webhook"`
}

func (n *Notifier) Save(u Update) error {
	if u.SMTP.Enabled {
		if u.SMTP.Host == "" || u.SMTP.From == "" || len(u.SMTP.To) == 0 {
			return errors.New("email needs a server, a sender and at least one recipient")
		}
		if u.SMTP.Port <= 0 || u.SMTP.Port > 65535 {
			return errors.New("the SMTP port isn't valid")
		}
	}
	if u.Webhook.URL != "" {
		p, err := url.Parse(u.Webhook.URL)
		if err != nil || (p.Scheme != "https" && p.Scheme != "http") || p.Host == "" {
			return errors.New("the webhook needs a full http(s) URL")
		}
	}
	n.mu.Lock()
	c := n.cfg
	pw, hook := c.SMTP.Password, c.Webhook.URL
	c.SMTP = u.SMTP
	c.SMTP.To = cleanList(u.SMTP.To)
	switch {
	case u.ClearPassword:
		c.SMTP.Password = ""
	case u.SMTP.Password == "":
		c.SMTP.Password = pw
	}
	c.Webhook.Enabled = u.Webhook.Enabled
	switch {
	case u.ClearWebhook:
		c.Webhook.URL = ""
	case u.Webhook.URL == "":
		c.Webhook.URL = hook
	default:
		c.Webhook.URL = u.Webhook.URL
	}
	if c.Webhook.Enabled && c.Webhook.URL == "" {
		n.mu.Unlock()
		return errors.New("add the webhook URL before turning it on")
	}
	for k, v := range u.Rules {
		if _, ok := c.Rules[k]; ok {
			c.Rules[k] = v
		}
	}
	n.cfg = c
	b, _ := json.Marshal(c)
	n.mu.Unlock()
	n.Store.SetMeta(metaKey, string(b))
	return nil
}

func cleanList(xs []string) []string {
	out := []string{}
	for _, x := range xs {
		for _, p := range strings.FieldsFunc(x, func(r rune) bool { return r == ',' || r == ';' || r == ' ' || r == '\n' }) {
			out = append(out, p)
		}
	}
	return out
}

func key(rule, target string) string { return rule + "\x00" + target }

// Fire reports a problem. The same rule and target stay quiet for Cooldown.
func (n *Notifier) Fire(rule, target, title, text, severity string) {
	n.mu.Lock()
	k := key(rule, target)
	now := n.now()
	for k2, s2 := range n.active {
		if s2.Title == "" && now.Sub(s2.lastSent) >= Cooldown {
			delete(n.active, k2) // a one-off whose cooldown is over
		}
	}
	s := n.active[k]
	if s != nil && now.Sub(s.lastSent) < Cooldown {
		n.mu.Unlock()
		return
	}
	if s == nil {
		s = &state{Active: Active{Rule: rule, Target: target, Title: title, Since: now.Unix()}}
		n.active[k] = s
	}
	s.lastSent = now
	enabled := n.cfg.Rules[rule]
	n.mu.Unlock()
	if enabled {
		n.dispatch(Message{Rule: rule, Title: title, Text: text, Severity: severity, Target: target, At: now})
	}
}

// Once reports something that has no "resolved" state (a crash, a cleanup summary).
func (n *Notifier) Once(rule, target, title, text, severity string) {
	n.Fire(rule, target, title, text, severity)
	n.mu.Lock()
	if s := n.active[key(rule, target)]; s != nil {
		// Keep the cooldown, but don't list it as an ongoing problem.
		s.Title = ""
	}
	n.mu.Unlock()
}

// Resolve reports that a problem cleared, if it had been reported.
func (n *Notifier) Resolve(rule, target, title string) {
	n.mu.Lock()
	k := key(rule, target)
	s := n.active[k]
	if s == nil || s.Title == "" {
		n.mu.Unlock()
		return
	}
	delete(n.active, k)
	enabled := n.cfg.Rules[rule]
	since := time.Unix(s.Since, 0)
	n.mu.Unlock()
	if enabled {
		n.dispatch(Message{Rule: rule, Title: title, Text: fmt.Sprintf("Cleared after %s.", n.now().Sub(since).Round(time.Minute)), Severity: "resolved", Target: target, At: n.now()})
	}
}

// IsActive reports whether a problem is currently reported.
func (n *Notifier) IsActive(rule, target string) bool {
	n.mu.Lock()
	defer n.mu.Unlock()
	s := n.active[key(rule, target)]
	return s != nil && s.Title != ""
}

// dispatch sends in the background and records the outcome in Activity.
func (n *Notifier) dispatch(m Message) {
	n.remember(m)
	go func() {
		err := n.Send(m)
		if errors.Is(err, errNoChannel) {
			return
		}
		a := store.Activity{At: m.At.Unix(), Kind: "alert", Type: m.Rule, Action: m.Severity, Target: m.Target, Name: m.Title, OK: err == nil}
		if err != nil {
			a.Detail = err.Error()
			log.Printf("alert %q: %v", m.Title, err)
		}
		n.Store.AddActivity(a)
	}()
}

var errNoChannel = errors.New("no alert channel is turned on")

// Send delivers m on every enabled channel now, and reports what failed.
func (n *Notifier) Send(m Message) error {
	if n.send != nil {
		return n.send(m)
	}
	n.mu.Lock()
	c := n.cfg
	n.mu.Unlock()
	if !c.SMTP.Enabled && !c.Webhook.Enabled {
		return errNoChannel
	}
	var errs []string
	if c.SMTP.Enabled {
		if err := sendMail(c.SMTP, n.Host, m); err != nil {
			errs = append(errs, "email: "+err.Error())
		}
	}
	if c.Webhook.Enabled {
		if err := n.postWebhook(c.Webhook.URL, m); err != nil {
			errs = append(errs, "webhook: "+err.Error())
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

// Test sends a test message and waits for the result.
func (n *Notifier) Test() error {
	err := n.Send(Message{Rule: "test", Title: "StunSails test alert", Text: "Alerts from " + n.Host + " reach you here.", Severity: "info", Target: "test", At: n.now()})
	if errors.Is(err, errNoChannel) {
		return errors.New("turn on email or the webhook first")
	}
	return err
}

func icon(sev string) string {
	switch sev {
	case "critical":
		return "🔴"
	case "warning":
		return "🟠"
	case "resolved":
		return "🟢"
	}
	return "🔵"
}

// WebhookBody is the JSON posted; content and text make it readable in Discord and Slack as is.
func WebhookBody(host string, m Message) ([]byte, error) {
	line := fmt.Sprintf("%s %s · %s\n%s", icon(m.Severity), m.Title, host, m.Text)
	return json.Marshal(map[string]any{
		"title": m.Title, "message": m.Text, "severity": m.Severity, "rule": m.Rule, "target": m.Target,
		"host": host, "at": m.At.UTC().Format(time.RFC3339), "content": line, "text": line,
	})
}

func (n *Notifier) postWebhook(u string, m Message) error {
	body, err := WebhookBody(n.Host, m)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "StunSails/2")
	resp, err := n.HTTP.Do(req)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("the webhook answered %s", resp.Status)
	}
	return nil
}

// MailBody builds the RFC 5322 message.
func MailBody(c SMTP, host string, m Message) []byte {
	var b strings.Builder
	subject := fmt.Sprintf("[StunSails] %s %s", icon(m.Severity), m.Title)
	fmt.Fprintf(&b, "From: StunSails <%s>\r\n", c.From)
	fmt.Fprintf(&b, "To: %s\r\n", strings.Join(c.To, ", "))
	fmt.Fprintf(&b, "Subject: %s\r\n", mimeWord(subject))
	fmt.Fprintf(&b, "Date: %s\r\n", m.At.Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: 8bit\r\n\r\n")
	fmt.Fprintf(&b, "%s\r\n\r\nServer: %s\r\nWhat: %s\r\nWhen: %s\r\n", m.Text, host, m.Target, m.At.Format("2 Jan 2006 15:04 MST"))
	return []byte(b.String())
}

func mimeWord(s string) string {
	for _, r := range s {
		if r > 127 {
			return "=?utf-8?q?" + strings.NewReplacer("=", "=3D", "?", "=3F", "_", "=5F", " ", "_").Replace(qp(s)) + "?="
		}
	}
	return s
}

func qp(s string) string {
	var b strings.Builder
	for _, c := range []byte(s) {
		if c > 127 {
			fmt.Fprintf(&b, "=%02X", c)
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

func sendMail(c SMTP, host string, m Message) error {
	addr := net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
	d := &net.Dialer{Timeout: 10 * time.Second}
	var conn net.Conn
	var err error
	if c.Port == 465 {
		conn, err = tls.DialWithDialer(d, "tcp", addr, &tls.Config{ServerName: c.Host})
	} else {
		conn, err = d.Dial("tcp", addr)
	}
	if err != nil {
		return err
	}
	conn.SetDeadline(time.Now().Add(30 * time.Second))
	cl, err := smtp.NewClient(conn, c.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer cl.Close()
	if c.Port != 465 {
		if ok, _ := cl.Extension("STARTTLS"); ok {
			if err := cl.StartTLS(&tls.Config{ServerName: c.Host}); err != nil {
				return err
			}
		}
	}
	if c.User != "" {
		if err := cl.Auth(smtp.PlainAuth("", c.User, c.Password, c.Host)); err != nil {
			return err
		}
	}
	if err := cl.Mail(c.From); err != nil {
		return err
	}
	for _, to := range c.To {
		if err := cl.Rcpt(to); err != nil {
			return err
		}
	}
	w, err := cl.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(MailBody(c, host, m)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return cl.Quit()
}

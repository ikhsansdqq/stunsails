package alert

import (
	"bufio"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stunsails/stunsails/internal/store"
)

func newNotifier(t *testing.T) (*Notifier, *[]Message, *time.Time) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	n := New(st, "vps")
	now := time.Unix(1_800_000_000, 0)
	n.Now = func() time.Time { return now }
	var mu sync.Mutex
	sent := []Message{}
	n.send = func(m Message) error {
		mu.Lock()
		sent = append(sent, m)
		mu.Unlock()
		return nil
	}
	return n, &sent, &now
}

func wait(t *testing.T, sent *[]Message, n int) {
	t.Helper()
	for i := 0; i < 100 && len(*sent) < n; i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	if len(*sent) != n {
		t.Fatalf("want %d messages, got %d: %+v", n, len(*sent), *sent)
	}
}

func TestCooldownAndResolve(t *testing.T) {
	n, sent, now := newNotifier(t)
	n.Fire(RuleDisk, "/", "Disk 93% full", "x", "critical")
	n.Fire(RuleDisk, "/", "Disk 94% full", "x", "critical")
	wait(t, sent, 1)
	if !n.IsActive(RuleDisk, "/") || len(n.Public().Active) != 1 {
		t.Fatal("should be active")
	}
	*now = now.Add(Cooldown)
	n.Fire(RuleDisk, "/", "Disk 95% full", "x", "critical")
	wait(t, sent, 2)
	n.Resolve(RuleDisk, "/", "Disk below 90%")
	wait(t, sent, 3)
	if (*sent)[2].Severity != "resolved" || n.IsActive(RuleDisk, "/") {
		t.Fatalf("want resolved, got %+v", (*sent)[2])
	}
	n.Resolve(RuleDisk, "/", "again")
	wait(t, sent, 3)
	n.Once(RuleCrash, "web", "web crashed", "x", "critical")
	wait(t, sent, 4)
	if len(n.Public().Active) != 0 {
		t.Fatalf("one-offs must not be listed as ongoing: %+v", n.Public().Active)
	}
}

func TestDisabledRuleStaysQuiet(t *testing.T) {
	n, sent, _ := newNotifier(t)
	if err := n.Save(Update{Rules: map[string]bool{RuleSSH: false}}); err != nil {
		t.Fatal(err)
	}
	n.Fire(RuleSSH, "all", "SSH", "x", "warning")
	n.Once(RuleCleanup, "weekly", "Cleanup", "x", "info") // off by default
	wait(t, sent, 0)
}

func TestSaveKeepsSecretsAndHidesThem(t *testing.T) {
	n, _, _ := newNotifier(t)
	err := n.Save(Update{SMTP: SMTP{Enabled: true, Host: "smtp.example", Port: 587, From: "a@x", To: []string{"b@x, c@x"}, Password: "pw1"},
		Webhook: Webhook{Enabled: true, URL: "https://hooks.example/abc?token=secret"}})
	if err != nil {
		t.Fatal(err)
	}
	// A later save without secrets keeps them.
	if err := n.Save(Update{SMTP: SMTP{Enabled: true, Host: "smtp.example", Port: 587, From: "a@x", To: []string{"b@x"}}, Webhook: Webhook{Enabled: true}}); err != nil {
		t.Fatal(err)
	}
	if n.cfg.SMTP.Password != "pw1" || !strings.Contains(n.cfg.Webhook.URL, "secret") {
		t.Fatalf("secrets lost: %+v", n.cfg)
	}
	b, _ := json.Marshal(n.Public())
	if strings.Contains(string(b), "pw1") || strings.Contains(string(b), "secret") {
		t.Fatalf("public config leaks a secret: %s", b)
	}
	p := n.Public()
	if !p.SMTP.PasswordSet || !p.Webhook.URLSet || p.Webhook.Host != "hooks.example" {
		t.Fatalf("public flags wrong: %+v", p)
	}
	// Reloaded from the store.
	n2 := New(n.Store, "vps")
	if n2.cfg.SMTP.Password != "pw1" || len(n2.cfg.SMTP.To) != 1 {
		t.Fatalf("not persisted: %+v", n2.cfg)
	}
	if err := n.Save(Update{Webhook: Webhook{Enabled: true}, ClearWebhook: true}); err == nil {
		t.Fatal("turning on a webhook without a URL must fail")
	}
	if err := n.Save(Update{SMTP: SMTP{Enabled: true, Host: "h"}}); err == nil {
		t.Fatal("email without recipients must fail")
	}
}

func TestWebhookDelivery(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()
	n, _, _ := newNotifier(t)
	n.send = nil
	n.Save(Update{Webhook: Webhook{Enabled: true, URL: srv.URL}})
	if err := n.Test(); err != nil {
		t.Fatal(err)
	}
	if got["title"] != "StunSails test alert" || !strings.Contains(got["content"].(string), "vps") || got["text"] != got["content"] {
		t.Fatalf("payload %+v", got)
	}
}

// A minimal SMTP server: enough of the dialogue for net/smtp, recording the message.
func fakeSMTP(t *testing.T) (string, *string) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	var data string
	go func() {
		c, err := l.Accept()
		if err != nil {
			return
		}
		defer c.Close()
		r := bufio.NewReader(c)
		w := func(s string) { io.WriteString(c, s+"\r\n") }
		w("220 fake")
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				return
			}
			cmd := strings.ToUpper(strings.TrimSpace(line))
			switch {
			case strings.HasPrefix(cmd, "EHLO"), strings.HasPrefix(cmd, "HELO"):
				w("250 fake")
			case cmd == "DATA":
				w("354 go")
				var b strings.Builder
				for {
					l, _ := r.ReadString('\n')
					if l == ".\r\n" {
						break
					}
					b.WriteString(l)
				}
				data = b.String()
				w("250 ok")
			case cmd == "QUIT":
				w("221 bye")
				return
			default:
				w("250 ok")
			}
		}
	}()
	return l.Addr().String(), &data
}

func TestSMTPDelivery(t *testing.T) {
	addr, data := fakeSMTP(t)
	host, port, _ := net.SplitHostPort(addr)
	p := 0
	for _, c := range port {
		p = p*10 + int(c-'0')
	}
	err := sendMail(SMTP{Enabled: true, Host: host, Port: p, From: "stunsails@x", To: []string{"me@x"}}, "vps",
		Message{Title: "Disk 93% full", Text: "Only 4 GB left.", Severity: "critical", Target: "/", At: time.Unix(1_800_000_000, 0)})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(*data, "Only 4 GB left.") || !strings.Contains(*data, "Subject: =?utf-8?q?") || !strings.Contains(*data, "To: me@x") {
		t.Fatalf("message:\n%s", *data)
	}
}

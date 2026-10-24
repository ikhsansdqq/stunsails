package setup

import (
	"os/exec"
	"strings"
)

// proxyAdvice prints the reverse proxy block for whichever proxy is here.
// It doesn't edit the proxy's config: a hand-written Caddyfile or Nginx site
// is the owner's, and changing it waits for the backed-up, validated editor.
func proxyAdvice(ui *UI, domain string) {
	if domain == "" {
		domain = "stunsails.example.com"
	}
	caddy, nginx := exists("/etc/caddy/Caddyfile"), exists("/etc/nginx")
	switch {
	case caddy:
		ui.Say("  Caddy found. Add this to /etc/caddy/Caddyfile, then `sudo systemctl reload caddy`:\n")
		ui.Say("%s {\n\treverse_proxy %s {\n\t\theader_up Cf-Connecting-Ip {client_ip}\n\t}\n}\n", domain, Listen)
		ui.Say("  Caddy gets the certificate on its own.")
	case nginx:
		ui.Say("  Nginx found. Add a server block like this to /etc/nginx/conf.d/stunsails.conf,")
		ui.Say("  get a certificate (certbot), then `sudo nginx -t && sudo systemctl reload nginx`:\n")
		ui.Say(`server {
    listen 443 ssl;
    http2 on;
    server_name %s;
    ssl_certificate     /etc/letsencrypt/live/%s/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/%s/privkey.pem;
    location / {
        proxy_pass http://%s;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Cf-Connecting-Ip $remote_addr;
        proxy_buffering off;
        proxy_read_timeout 1h;
    }
}
`, domain, domain, domain, Listen)
	default:
		ui.Say("  No Caddy or Nginx found. Sign-in needs HTTPS: install Caddy (it handles certificates)")
		ui.Say("  and point a domain at it. Guide: https://ikhsansdqq.github.io/stunsails/guide/reverse-proxy")
		// Chrome and Firefox treat http://localhost as secure, so the Secure cookies work through a tunnel.
		ui.Say("  To look now without one: ssh -L 8099:%s <this-server>, then open http://localhost:8099", Listen)
		ui.Say("  in Chrome or Firefox (Safari won't keep the sign-in cookie over plain http).")
	}
}

// firewallAdvice says which web ports to open, and never opens one itself:
// a firewall may already allow 80/443 to a CDN's ranges only, on purpose.
func firewallAdvice(ui *UI) {
	ui.Say("\n  Port 8099 stays closed: StunSails only listens on loopback.")
	if out, err := exec.Command("ufw", "status").Output(); err == nil && strings.Contains(string(out), "Status: active") {
		if !strings.Contains(string(out), "443") {
			ui.Say("\n  ufw is on and doesn't allow 443 yet. Open the web ports with:\n    sudo ufw allow 80,443/tcp")
		}
		return
	}
	if err := exec.Command("firewall-cmd", "--state").Run(); err == nil {
		out, _ := exec.Command("firewall-cmd", "--list-services").Output()
		if !strings.Contains(string(out), "https") {
			ui.Say("\n  firewalld is on and doesn't allow https yet. Open the web ports with:")
			ui.Say("    sudo firewall-cmd --permanent --add-service=http --add-service=https && sudo firewall-cmd --reload")
		}
	}
}

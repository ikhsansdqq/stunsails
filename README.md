# Harbor

One small service that watches this VPS and every Docker container and compose
stack on it, and lets its owner manage them. Live at
server.majusolusinusantara.com (Cloudflare → Caddy → 127.0.0.1:8099).

- **Go backend** (`cmd/`, `internal/`): host readings from `/proc`, per-container
  CPU, memory and I/O from cgroup v2 files, the Docker Engine API over the unix
  socket, SQLite (`modernc.org/sqlite`) for one-minute history (7 days), the
  activity feed (30 days) and sessions.
- **Dashboard** (`web/`): React + Vite with freelance-dashboard's tokens and UI
  components, embedded into the binary with `go:embed`. UI rules are in `web/DESIGN.md`.

## Sign-in

A password (bcrypt) and then a Google Authenticator code. Every change
(container actions, stack actions, removals, prunes) also needs a code entered
within the last five minutes. Five wrong tries from one address lock it out for
15 minutes, and the lockout survives a restart. When over 50 wrong tries pile up across
all addresses, every try waits 2 seconds instead of anyone being locked out. Every sign-in and change is recorded in Activity.

```sh
sudo server-monitor passwd        # set or change the password
sudo server-monitor enroll        # pair a new phone (replaces the old key)
sudo server-monitor enroll --show # show the current key again, for a second phone
sudo systemctl restart server-monitor
```

## What it manages

- Containers: start, stop, restart, pause, kill, remove; live logs; resources; inspect (env names only, never values).
- Compose stacks: up, pull and update, restart, start, stop, down. Compose runs in
  a short-lived `docker:29-cli` container with the stack's folder mounted
  read-only at the same path; stacks taken down stay listed so they can come back up.
- Storage: Docker's disk use, images, volumes (deleted one at a time, never pruned in bulk), networks, prunes.

## Watching and alerting

- **Alerts** by email (SMTP) and webhook (Discord, Slack, ntfy, n8n), set in Settings:
  crashes, restart loops (3 deaths in 15 min), failing health checks, out-of-memory,
  disk over 90% or forecast full within a week, memory over 90% for 5 min, sites down
  (two failed checks), certificates within 14 days, failed systemd services, SSH
  login bursts. Each problem is sent once, again after 30 min if it persists, and
  once more when it clears.
- **Server activity**: SSH sign-ins and sudo from `/var/log/auth.log` (failed SSH
  logins summed every 10 min, keeping every address with its attempts and the users it
  tried), failed systemd units, site outages. The Activity table pages on the server;
  opening a row shows each address with its country, city and network.
- **IP locations**: DB-IP's free Lite city and ASN databases (CC BY 4.0), downloaded to
  `/var/lib/server-monitor/geo/` at start and refreshed monthly. No address leaves the
  server for a lookup.
- **Sites**: every Caddy site (from `/etc/caddy/Caddyfile`) plus any added by hand,
  fetched over HTTPS every minute: status, response time, certificate expiry.
- **Scheduled cleanup**: weekly prune of build cache, untagged images, stopped
  containers or unused networks. Volumes are never pruned in bulk.
- **Harbor's own footprint** in Settings. Measured 2026-10-18: ~10 MB memory,
  ~0.05–0.1% of one core, ~1 MB of history per 10 min of a 7-day window.

UI rules are in `web/DESIGN.md` (no table inside a card, animated icons, toast
and dialog for every action, OTP input).

## Build and deploy

Go isn't installed on the host; `build.sh` builds in the `golang:1.27` image.

```sh
./build.sh                 # web + binary → ./server-monitor
./build.sh test            # go vet + go test
sudo ./deploy/install.sh   # install or upgrade the systemd service (runs as the
                           # server-monitor system user, groups docker and adm)
```

Local run against the real Docker (http, so insecure cookies):

```sh
sudo env LISTEN=127.0.0.1:8098 DATA_DIR=/tmp/harbor TOTP_FILE=… PASSWORD_FILE=… INSECURE_COOKIES=1 ./server-monitor serve
cd web && npm run dev      # proxies /api to 8098
```

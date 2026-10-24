# Environment variables

StunSails reads its settings from the environment. In production they are set in
the systemd unit (`Environment=` lines); everything has a default.

| Variable | Default | What it does |
|---|---|---|
| `LISTEN` | `127.0.0.1:8099` | Address and port the dashboard listens on. Keep it on loopback behind a proxy. |
| `DATA_DIR` | `.` | Where `stunsails.db` and the `geo/` databases live. Under systemd, `STATE_DIRECTORY` (`/var/lib/stunsails`) wins. |
| `DOCKER_SOCKET` | `/var/run/docker.sock` | Docker Engine API socket. |
| `PROC` | `/proc` | Where host readings come from. |
| `CGROUP` | `/sys/fs/cgroup` | cgroup v2 root, for per-container readings. |
| `AUTH_LOG` | `/var/log/auth.log` | SSH and sudo log. |
| `CADDYFILE` | `/etc/caddy/Caddyfile` | Caddy config to find sites in. A missing file is fine. |
| `COMPOSE_IMAGE` | `docker:29-cli` | Image compose actions run in. |
| `PASSWORD_FILE` | `/etc/stunsails/password` | Password hash, when not passed by systemd `LoadCredential`. |
| `TOTP_FILE` | `/etc/stunsails/totp` | Authenticator key, likewise. |
| `INSECURE_COOKIES` | unset | `1` drops the `Secure` flag from cookies, for plain-HTTP development only. |
| `GOMEMLIMIT` | `48MiB` in the unit | Go runtime soft memory limit. |

## Changing one

Use a drop-in so upgrades don't overwrite it:

```sh
sudo systemctl edit stunsails
```

```ini
[Service]
Environment=LISTEN=127.0.0.1:9000
Environment=CADDYFILE=/opt/caddy/Caddyfile
```

```sh
sudo systemctl restart stunsails
```

## Settings in the dashboard

Alerts, extra sites to check, and the cleanup schedule are set in **Settings**
and stored in `stunsails.db`, not in the environment. See [Alerts](./alerts) and
[Storage and cleanup](../features/storage).

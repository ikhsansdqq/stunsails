# Environment variables

StunSails reads its settings from the environment. In production they are set in
the systemd unit (`Environment=` lines); everything has a default.

| Variable | Default | What it does |
|---|---|---|
| `LISTEN` | `127.0.0.1:8099` | Address and port the dashboard listens on. Keep it on loopback behind a proxy. `off` serves only `SOCKET`. |
| `SOCKET` | (none) | Also serve the API on this unix socket, for one local app that signs people in itself. See [Behind an app](#behind-an-app). |
| `SOCKET_ALLOW_UIDS` | (none) | Comma-separated user ids that may connect to `SOCKET`. Required with it. |
| `SOCKET_GROUP` | (none) | Group name or id that owns the socket file (mode 0660). |
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

## Behind an app

An app that has its own sign-in can put StunSails inside its pages instead of sending people to the StunSails dashboard. Set `SOCKET`, and StunSails also serves its API on that unix socket, next to the normal `LISTEN` address:

- Only the user ids in `SOCKET_ALLOW_UIDS` can connect. The kernel reports which user opened each connection, and StunSails closes any other user's connection unread.
- Each request says who is asking in `X-Proxy-User`, and when that person last entered a fresh authenticator code in `X-Proxy-StepUp` (unix seconds). StunSails trusts both headers only on the socket. Over `LISTEN` they mean nothing.
- Changes still need `X-StunSails: 1` and a step-up within five minutes. The audit log names the person.
- Signing in, step-up and sessions answer 404 on the socket, because the app handles them.
- `GET /api/alerts/feed?after=N` returns the alerts raised since sequence `N`, so the app can show them as its own notifications.

With systemd, add a drop-in such as `/etc/systemd/system/stunsails.service.d/socket.conf`:

```ini
[Service]
RuntimeDirectory=stunsails
RuntimeDirectoryMode=0755
Environment=SOCKET=/run/stunsails/api.sock
Environment=SOCKET_ALLOW_UIDS=10001
Environment=SOCKET_GROUP=maju-agent
SupplementaryGroups=maju-agent
```

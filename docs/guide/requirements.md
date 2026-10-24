# Requirements

| | Needed |
|---|---|
| OS | Linux with systemd and cgroup v2 (Ubuntu 22.04+, Debian 12+) |
| Docker | Docker Engine with the compose plugin images available (`docker:29-cli` is pulled at install) |
| Architecture | amd64 or arm64 |
| Memory | about 10 MB for StunSails; the unit caps it at 96 MB |
| Reverse proxy | anything that terminates HTTPS and forwards to `127.0.0.1:8099` |
| Phone | an authenticator app (Google Authenticator, Aegis, 1Password, …) |

To build from source you need Docker only. `build.sh` builds the dashboard in
`node:24-alpine` and the binary in `golang:1.27`, so neither Node nor Go has to
be on the server.

## Check cgroup v2

```sh
stat -fc %T /sys/fs/cgroup
# cgroup2fs
```

Anything else means per-container readings will be missing.

## Optional

- `/var/log/auth.log` (Debian and Ubuntu default) for SSH sign-ins and sudo.
- A Caddyfile at `/etc/caddy/Caddyfile`, so StunSails finds your sites on its own.
- Outbound HTTPS to `download.db-ip.com` once a month for IP locations.

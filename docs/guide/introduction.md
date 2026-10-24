# Introduction

StunSails is one small service that watches a Linux server and every Docker
container and compose stack on it, and lets its owner manage them from a web
dashboard.

It is built for one person or a small team running their own VPS: a few apps
in Docker, a reverse proxy in front, no Kubernetes. It replaces the habit of
SSHing in to run `docker ps`, `docker compose up -d` and `tail -f` with one
page, and tells you when something breaks.

## How it fits together

```
Browser ──HTTPS──> reverse proxy (Caddy, Nginx) ──> 127.0.0.1:8099 StunSails
                                                        │
                    /proc, /sys/fs/cgroup  <────────────┤  readings
                    /var/run/docker.sock   <────────────┤  containers, stacks
                    /var/log/auth.log      <────────────┤  SSH and sudo
                    SQLite (/var/lib/stunsails) <───────┘  history, activity, sessions
```

- **Backend**: Go. Reads `/proc` and cgroup v2 files directly, talks to the
  Docker Engine API over its unix socket, keeps state in SQLite.
- **Dashboard**: React and Vite, embedded into the binary with `go:embed`.
- **Compose**: runs in a short-lived `docker:29-cli` container with the stack's
  folder mounted read-only, so StunSails never needs access to your home folder.

## What it is not

- Not a multi-server fleet manager. One StunSails watches one server.
- Not a PaaS. It does not build your code from git; it manages what Docker and
  compose already run.
- Not a replacement for Prometheus and Grafana on a large estate. It keeps
  seven days of one-minute history, which is enough to answer "what happened
  last night".

Next: [Requirements](./requirements).

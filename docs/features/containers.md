# Containers

**Containers** lists every container Docker knows, running or not, with CPU,
memory, network and disk I/O.

## Readings

- CPU, memory and I/O come straight from each container's cgroup v2 files, not
  from `docker stats`, so reading them costs almost nothing.
- One-minute history is kept for 7 days; open a container to see its charts.

## Actions

Start, stop, restart, pause, unpause, kill and remove. Each needs a fresh
authenticator code, asks first when it is destructive, and is recorded in
Activity.

## Logs

Live logs stream from the Engine API. They follow until you close them.

## Inspect

Image, command, ports, mounts, networks, labels, restart policy and health.
Environment variables show **names only, never values**, so a secret in an env
var cannot leak through the dashboard.

---
layout: home
hero:
  name: StunSails
  text: A small control plane for one server
  tagline: Watch the host and every Docker container and compose stack on it, and manage them behind a password and an authenticator code.
  actions:
    - theme: brand
      text: Install
      link: /guide/install
    - theme: alt
      text: What it does
      link: /guide/introduction
features:
  - title: Host and containers
    details: Host readings from /proc, per-container CPU, memory and I/O from cgroup v2, seven days of one-minute history.
  - title: Compose stacks
    details: Up, pull and update, restart, stop, down. Stacks taken down stay listed so they can come back up.
  - title: Alerts that mean something
    details: Crashes, restart loops, out-of-memory, disk filling, sites down, certificates expiring, failed services, SSH bursts. By email or webhook.
  - title: Every change needs a code
    details: Password, then Google Authenticator. Each change needs a code entered in the last five minutes, and is recorded in Activity.
  - title: Tiny
    details: One static Go binary with the dashboard embedded. About 10 MB of memory and 0.1% of one core.
  - title: Nothing leaves the server
    details: IP locations come from a local DB-IP database. No telemetry, no cloud account.
---

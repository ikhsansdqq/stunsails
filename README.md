# StunSails

One small service that watches a Linux server and every Docker container and
compose stack on it, and lets its owner manage them. Password plus
authenticator code to sign in, a fresh code for every change, and about 10 MB
of memory.

- **Watch**: host and per-container CPU, memory and I/O (cgroup v2), 7 days of
  one-minute history, sites and certificates, SSH sign-ins and sudo.
- **Manage**: containers (start, stop, restart, pause, kill, remove, logs),
  compose stacks (up, pull and update, restart, stop, down), images, volumes,
  networks, prunes.
- **Alert**: crashes, restart loops, OOM, disk filling, memory, sites down,
  certificates expiring, failed services, SSH bursts. By email or webhook
  (Discord, Slack, ntfy, n8n).

**Documentation:** [`docs/`](docs/), published to GitHub Pages. Build it locally
with `./build.sh docs`.

## Quick start

Needs Linux with systemd, cgroup v2 and Docker. Go and Node aren't needed on the
host: `build.sh` builds in Docker images.

```sh
./build.sh                 # dashboard + binary → ./stunsails
sudo ./deploy/install.sh   # install the service, set a password, pair a phone
```

StunSails listens on `127.0.0.1:8099`. Put Caddy or Nginx in front for HTTPS; see
[Reverse proxy](docs/guide/reverse-proxy.md).

```sh
./build.sh test            # gofmt, go vet, go test
./build.sh docs            # docs site → docs/.vitepress/dist
```

## License

Copyright (C) 2026 Ikhsan Assidiqie

StunSails is free software: you can redistribute it and/or modify it under the
terms of the GNU Affero General Public License as published by the Free
Software Foundation, version 3 or (at your option) any later version. See
`LICENSE`. If you run a modified version for others over a network, you must
offer them its source.

Contributions need a signed [CLA](CLA.md). The name and logo are covered by
[TRADEMARKS.md](TRADEMARKS.md), not by the code license.

IP location data is DB-IP Lite (CC BY 4.0), downloaded at run time and not
part of this repository.

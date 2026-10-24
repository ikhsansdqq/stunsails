# Contributing

## Layout

| Path | What |
|---|---|
| `cmd/stunsails/` | entry point and subcommands (`serve`, `passwd`, `enroll`) |
| `internal/api/` | HTTP handlers, sessions, step-up |
| `internal/collector/` | sampling loop and container watch (crashes, loops, OOM) |
| `internal/docker/`, `internal/compose/` | Engine API client, compose runner |
| `internal/host/`, `internal/cgroup/` | host and container readings |
| `internal/alert/`, `internal/uptime/`, `internal/hostlog/` | alerts, site checks, auth log |
| `internal/store/` | SQLite |
| `web/` | the dashboard; UI rules in `web/DESIGN.md` |
| `docs/` | this site |

## Build and test

```sh
./build.sh          # everything
./build.sh test     # gofmt, go vet, go test
./build.sh docs     # this site → docs/.vitepress/dist
```

Code must pass `gofmt`. Keep new features as small as the rest: StunSails'
budget is about 10 MB of memory.

## Docs

Pages are Markdown in `docs/`. To preview while you write:

```sh
cd docs && npm install && npm run docs:dev
```

## License

The code is released under the GNU AGPL v3 or later. Before your first pull
request is merged, you sign the [Contributor License Agreement](https://github.com/ikhsansdqq/stunsails/blob/main/CLA.md)
once: you keep the copyright in your work, and the project may also release it
under other terms (for example in the hosted service). A bot asks you on the
pull request.

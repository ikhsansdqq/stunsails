# Contributing

Thanks for helping. The full guide (layout, build, tests, docs) is in
[docs/contributing.md](docs/contributing.md).

## Before your first pull request

Sign the [Contributor License Agreement](CLA.md). A bot asks you on the pull
request; it takes one click and covers all your future contributions. You keep
the copyright in your work.

## In short

```sh
./build.sh test     # gofmt, go vet, go test
./build.sh docs     # docs site
```

- Code must pass `gofmt`.
- Keep features as small as the rest: the whole service runs in about 10 MB.
- UI changes follow `web/DESIGN.md`.
- Security issues: don't open a public issue. Use GitHub's private
  **Security → Report a vulnerability** form.

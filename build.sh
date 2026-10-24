#!/bin/sh
# Builds StunSails: the dashboard in the node image, then a static Go binary in the
# golang image (neither runs on the host, so no web/node_modules is left behind).
# Output: ./stunsails
#   ./build.sh          web + binary
#   ./build.sh go       binary only (reuses web/dist)
#   ./build.sh test     go vet + go test
#   ./build.sh docs     the docs site → docs/.vitepress/dist
set -eu
cd "$(dirname "$0")"
GO_IMAGE=golang:1.27
NODE_IMAGE=node:24-alpine
CACHE="$HOME/.cache/stunsails-go"
NPM_CACHE="$HOME/.cache/stunsails-npm"
mkdir -p "$CACHE/mod" "$CACHE/build" "$NPM_CACHE"

if [ "${1:-}" = "docs" ]; then
	# Same as the dashboard: node_modules only lives for this run.
	sudo docker run --rm -u "$(id -u):$(id -g)" \
		-e HOME=/tmp -e npm_config_cache=/cache \
		-v "$NPM_CACHE:/cache" -v "$PWD/docs:/docs" -w /docs "$NODE_IMAGE" \
		sh -c 'trap "rm -rf node_modules" EXIT; if [ -f package-lock.json ]; then npm ci --no-audit --no-fund; else npm install --no-audit --no-fund; fi && npm run docs:build'
	exit
fi

if [ "${1:-}" != "go" ] && [ "${1:-}" != "test" ]; then
	# Stale bundles would otherwise pile up in the embedded dist/.
	find web/dist -mindepth 1 ! -name .keep -delete
	# node_modules lives only for this run; the npm cache keeps the next install fast.
	sudo docker run --rm -u "$(id -u):$(id -g)" \
		-e HOME=/tmp -e npm_config_cache=/cache \
		-v "$NPM_CACHE:/cache" -v "$PWD/web:/web" -w /web "$NODE_IMAGE" \
		sh -c 'trap "rm -rf node_modules" EXIT; npm ci --no-audit --no-fund && npm run build'
fi

go_run() {
	sudo docker run --rm -u "$(id -u):$(id -g)" \
		-e GOCACHE=/cache/build -e GOMODCACHE=/cache/mod -e GOFLAGS=-mod=mod -e CGO_ENABLED=0 -e HOME=/tmp \
		-v "$CACHE:/cache" -v "$PWD:/src" -w /src "$GO_IMAGE" sh -c "$1"
}

if [ "${1:-}" = "test" ]; then
	go_run 'out=$(gofmt -l cmd internal web/embed.go); if [ -n "$out" ]; then echo "gofmt needed:"; echo "$out"; exit 1; fi; go vet ./... && go test ./...'
	exit
fi
go_run "go build -trimpath -ldflags='-s -w' -o stunsails ./cmd/stunsails"
ls -lh stunsails

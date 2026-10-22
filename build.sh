#!/bin/sh
# Builds Harbor: the dashboard with the host's Node, then a static Go binary in
# the golang image (Go isn't installed on the host). Output: ./server-monitor
#   ./build.sh          web + binary
#   ./build.sh go       binary only (reuses web/dist)
#   ./build.sh test     go vet + go test
set -eu
cd "$(dirname "$0")"
GO_IMAGE=golang:1.27
CACHE="$HOME/.cache/server-monitor-go"
mkdir -p "$CACHE/mod" "$CACHE/build"

if [ "${1:-}" != "go" ] && [ "${1:-}" != "test" ]; then
	# Stale bundles would otherwise pile up in the embedded dist/.
	find web/dist -mindepth 1 ! -name .keep -delete
	(cd web && { [ -d node_modules ] || npm ci; } && npm run build)
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
go_run "go build -trimpath -ldflags='-s -w' -o server-monitor ./cmd/server-monitor"
ls -lh server-monitor

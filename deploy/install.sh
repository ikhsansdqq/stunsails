#!/bin/sh
# Installs or upgrades StunSails from this checkout. Build first (./build.sh), then:
#   sudo ./deploy/install.sh
# The steps live in `stunsails init`, shared with the curl installer; any flags
# (--yes, --domain, --no-docker-install) pass through.
set -eu
cd "$(dirname "$0")/.."
[ "$(id -u)" = 0 ] || { echo "Run with sudo." >&2; exit 1; }
[ -x ./stunsails ] || { echo "Build first: ./build.sh" >&2; exit 1; }
exec ./stunsails init --binary ./stunsails "$@"

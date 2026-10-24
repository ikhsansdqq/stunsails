#!/bin/sh
# StunSails installer: downloads a release, checks it, and starts `stunsails init`.
#
#   curl -fsSL https://ikhsansdqq.github.io/stunsails/install.sh | sudo sh
#
# Flags for init go after `sh -s --`, e.g. `| sudo sh -s -- --domain ops.example.com`.
# `| sudo STUNSAILS_VERSION=v0.2.0 sh` pins a release. Read this script before piping it to sh;
# it is short on purpose.
set -eu

repo=ikhsansdqq/stunsails
say() { printf '%s\n' "$*" >&2; }
die() { say "stunsails: $*"; exit 1; }

[ "$(uname -s)" = Linux ] || die "StunSails runs on Linux. On Windows, use WSL2: see the docs."
[ "$(id -u)" = 0 ] || die "run it with sudo: curl -fsSL https://ikhsansdqq.github.io/stunsails/install.sh | sudo sh"
case "$(uname -m)" in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) die "no build for $(uname -m); amd64 and arm64 only" ;;
esac
command -v curl >/dev/null || die "needs curl"
command -v sha256sum >/dev/null || die "needs sha256sum (coreutils)"

version=${STUNSAILS_VERSION:-}
if [ -z "$version" ]; then
	version=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -n1)
	[ -n "$version" ] || die "couldn't find the latest release"
fi
archive="stunsails_${version#v}_linux_${arch}.tar.gz"
base="https://github.com/$repo/releases/download/$version"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
say "Downloading StunSails $version ($arch)"
curl -fsSL -o "$tmp/$archive" "$base/$archive"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"

# The signature first: it proves checksums.txt came from this repo's release workflow.
if command -v cosign >/dev/null; then
	curl -fsSL -o "$tmp/checksums.txt.sigstore.json" "$base/checksums.txt.sigstore.json"
	cosign verify-blob --bundle "$tmp/checksums.txt.sigstore.json" \
		--certificate-identity-regexp "^https://github.com/$repo/.github/workflows/release.yml@refs/tags/" \
		--certificate-oidc-issuer https://token.actions.githubusercontent.com \
		"$tmp/checksums.txt" >/dev/null 2>&1 || die "the release signature doesn't verify"
	say "Signature verified."
else
	say "cosign isn't installed, so only the checksum is checked."
	say "To check the signature too: gh attestation verify /usr/local/bin/stunsails --repo $repo"
fi
(cd "$tmp" && grep " $archive\$" checksums.txt | sha256sum -c - >/dev/null) || die "checksum doesn't match"
say "Checksum matches."

tar -xzf "$tmp/$archive" -C "$tmp" stunsails
[ -f /usr/local/bin/stunsails ] && cp -p /usr/local/bin/stunsails /usr/local/bin/stunsails.prev
install -m 755 "$tmp/stunsails" /usr/local/bin/stunsails

# Piped into sh, stdin is this script: give init the terminal for its questions.
if [ -t 1 ] && [ -r /dev/tty ]; then
	exec /usr/local/bin/stunsails init "$@" </dev/tty
fi
exec /usr/local/bin/stunsails init "$@"

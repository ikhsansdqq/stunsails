#!/bin/sh
# Lets ports 80 and 443 in only from Cloudflare's edge, so nobody can reach the
# sites around Cloudflare. Adds the current ranges before dropping stale ones,
# so the sites never go dark mid-run. Safe to run again (weekly is plenty).
#   sudo sh deploy/ufw-cloudflare.sh
set -eu
v4=$(curl -fsS --max-time 20 https://www.cloudflare.com/ips-v4)
v6=$(curl -fsS --max-time 20 https://www.cloudflare.com/ips-v6)
[ -n "$v4" ] && [ -n "$v6" ] || { echo "empty range list, nothing changed" >&2; exit 1; }
for net in $v4 $v6; do
	ufw allow proto tcp from "$net" to any port 80,443 comment cloudflare >/dev/null
done
# The old open-to-everyone rules.
for rule in "80/tcp" "443/tcp"; do
	while ufw status | grep -qE "^$rule +ALLOW +Anywhere"; do ufw delete allow "$rule" >/dev/null; done
done
echo "80/443 now open to $(echo $v4 $v6 | wc -w) Cloudflare ranges only:"
ufw status | grep -E "80,443|^22|OpenSSH"

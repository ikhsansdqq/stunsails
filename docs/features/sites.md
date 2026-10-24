# Sites and certificates

StunSails fetches every site over HTTPS once a minute and records the status,
response time and certificate expiry.

## Where sites come from

- **Caddy**: every site address that opens a top-level block in
  `/etc/caddy/Caddyfile` (or `CADDYFILE`). Wildcards, bare ports and snippets
  are skipped.
- **By hand**: add any hostname in **Sites**. Only hand-added sites can be
  removed there; Caddy's come from its config.

## Alerts

- **Down**: after two failed checks in a row.
- **Certificate**: when it expires within 14 days.

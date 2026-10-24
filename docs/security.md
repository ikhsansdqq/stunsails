# Security model

StunSails can start, stop and remove anything in Docker. Access to it is close to
root on the server. This page says what protects it and where the edges are.

## What StunSails can do

- Talk to the Docker socket. **The docker group is root-equivalent**: a client
  of the socket can run a privileged container that mounts `/`.
- Read `/proc`, cgroup files, `/var/log/auth.log`, and the Caddyfile.
- Write only its own state folder, `/var/lib/stunsails`.

## What stands in front of it

1. **Loopback only.** It listens on `127.0.0.1`; a reverse proxy with HTTPS is
   the only way in.
2. **Password** (bcrypt) **then authenticator code** (TOTP).
3. **Step-up for changes.** Every change needs a code entered within the last
   five minutes, even in a signed-in session.
4. **Lockouts.** Five wrong tries per address lock it for 15 minutes, persisted
   across restarts; over 50 across all addresses slows every try to 2 seconds.
5. **Audit.** Every sign-in and change is recorded in Activity.
6. **Sandbox.** The systemd unit drops all capabilities and makes the file
   system read-only. See [systemd unit](./configuration/systemd).
7. **Headers.** Strict CSP, `frame-ancestors 'none'`, `nosniff`, no referrer,
   HSTS behind HTTPS.

## What it never shows

- Environment variable **values**. Inspect shows names only.
- The SMTP password and webhook URL after saving.

## Reporting a vulnerability

Please do not open a public issue. Use the repository's private
**Security → Report a vulnerability** form on GitHub.

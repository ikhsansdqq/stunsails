# Troubleshooting

Start with the journal:

```sh
journalctl -u stunsails -n 100 --no-pager
```

## "no password yet" or "no authenticator secret yet"

Run `sudo stunsails passwd` or `sudo stunsails enroll`, then
`sudo systemctl restart stunsails`.

## The code is always wrong

Authenticator codes depend on the clock. Check the server:

```sh
timedatectl   # "System clock synchronized: yes"
```

and that the phone sets its time automatically.

## Sign-in succeeds but the page logs me out

Cookies are `Secure`. Open StunSails over HTTPS, or set `INSECURE_COOKIES=1` for a
local plain-HTTP run only.

## Locked out after wrong tries

Wait 15 minutes. The lockout is per address and survives a restart.

## Container CPU and memory are empty

StunSails needs cgroup v2: `stat -fc %T /sys/fs/cgroup` must print `cgroup2fs`.

## No SSH activity

The service user must be in `adm` (`id stunsails`), and the system must
log to `/var/log/auth.log`. Set `AUTH_LOG` if yours logs elsewhere.

## No failed-service alerts

`systemctl --failed` needs the system D-Bus. Don't switch the unit to
`DynamicUser`.

## Compose action fails with a missing variable

`up`, `pull` and `down` parse the compose file. If it needs a variable your
deploy script sets, put it in the stack's `.env` file.

## Sites from Caddy don't show

Check `CADDYFILE` points at the file Caddy runs, and that the site address
opens its own block (`example.com {`).

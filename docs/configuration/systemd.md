# systemd unit

`deploy/stunsails.service.tmpl` (installed by `stunsails init`) runs StunSails as its own system user inside a
tight sandbox. This page explains each choice so you can judge it, or loosen
one line with a drop-in (`sudo systemctl edit stunsails`) when your setup
needs it.

## Who it runs as

```ini
User=stunsails
Group=stunsails
SupplementaryGroups=docker adm
```

- `docker`: to list and manage containers and run compose. **Membership of the
  docker group is equivalent to root**: anything that can talk to the socket can
  start a privileged container. StunSails puts a password, an authenticator code,
  and a fresh code for every change in front of it.
- `adm`: to read `/var/log/auth.log`.
- Not `DynamicUser`: dynamic users can't reach the system D-Bus, which
  `systemctl --failed` needs for the failed-services alert.

## Credentials

```ini
LoadCredential=password:/etc/stunsails/password
LoadCredential=totp:/etc/stunsails/totp
```

systemd copies the two files into a private directory only the service sees.
`/etc/stunsails` itself stays `0700 root`.

## Limits

```ini
MemoryMax=96M
CPUQuota=20%
Environment=GOMEMLIMIT=48MiB
```

Compose runs in its own helper container, so its memory isn't counted here.

## Sandbox

| Line | Effect |
|---|---|
| `ProtectSystem=strict` | The whole file system is read-only except `StateDirectory`. |
| `ProtectHome=yes` | Home folders are invisible. Compose gets stack folders through its helper container instead. |
| `PrivateTmp=yes`, `PrivateDevices=yes` | Own `/tmp`, no device nodes. |
| `NoNewPrivileges=yes`, `CapabilityBoundingSet=` | No capabilities, no setuid. |
| `ProtectProc=invisible` | Other users' processes are hidden. Host totals still come from `/proc`. |
| `ProtectKernel*`, `ProtectControlGroups`, `ProtectClock`, `ProtectHostname` | Kernel and system settings are read-only. |
| `RestrictAddressFamilies=AF_UNIX AF_INET AF_INET6` | Only the Docker socket and the network. |
| `MemoryDenyWriteExecute=yes`, `SystemCallFilter=@system-service` | No JIT, only ordinary system calls. |

Check the score with:

```sh
systemd-analyze security stunsails
```

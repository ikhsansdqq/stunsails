# Upgrade and uninstall

## Upgrade

```sh
sudo stunsails upgrade                  # the latest release
sudo stunsails upgrade --version v0.2.0 # a given one
```

`upgrade` downloads the release, checks its checksum (and its signature when
`cosign` is installed), keeps the current binary and unit as `.prev`, swaps in
the new binary and restarts. If the new version doesn't answer within 15
seconds, it puts the old one back on its own.

Your password, authenticator and data are kept. Installed from a `.deb` or
`.rpm`? Upgrade with `apt` or `dnf` instead.

From a checkout:

```sh
git pull && ./build.sh && sudo ./deploy/install.sh
```

## Roll back by hand

```sh
sudo cp /usr/local/bin/stunsails.prev /usr/local/bin/stunsails
sudo cp /etc/systemd/system/stunsails.service.prev /etc/systemd/system/stunsails.service
sudo systemctl daemon-reload && sudo systemctl restart stunsails
```

## Uninstall

```sh
sudo stunsails uninstall
```

This stops and removes the service and binary. It keeps `/var/lib/stunsails`
and `/etc/stunsails`, so `sudo stunsails init` later picks up where you left
off.

::: danger
`--purge` also deletes the history, activity log, password, authenticator key
and the service user. It cannot be undone.
:::

```sh
sudo stunsails uninstall --purge
```

Your containers and stacks keep running. StunSails only manages them; it does
not own them.

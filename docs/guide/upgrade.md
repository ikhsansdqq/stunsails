# Upgrade and uninstall

## Upgrade

```sh
cd stunsails
git pull
./build.sh
sudo ./deploy/install.sh
```

The installer keeps your password, authenticator and data. It saves the
previous binary and unit as `.prev` before replacing them.

## Roll back

```sh
sudo cp /usr/local/bin/stunsails.prev /usr/local/bin/stunsails
sudo cp /etc/systemd/system/stunsails.service.prev /etc/systemd/system/stunsails.service
sudo systemctl daemon-reload && sudo systemctl restart stunsails
```

## Uninstall

::: danger
The last two commands delete StunSails' history, activity log and credentials.
They cannot be undone.
:::

```sh
sudo systemctl disable --now stunsails
sudo rm /etc/systemd/system/stunsails.service /usr/local/bin/stunsails*
sudo systemctl daemon-reload
sudo userdel stunsails
sudo rm -r /var/lib/stunsails /etc/stunsails
```

Your containers and stacks keep running. StunSails only manages them; it does not
own them.

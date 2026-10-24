# Install

One command downloads a signed release, checks it, and walks you through setup:

```sh
curl -fsSL https://ikhsansdqq.github.io/stunsails/install.sh | sudo sh
```

Read the script first if you like; it is short:
[install.sh](https://ikhsansdqq.github.io/stunsails/install.sh).

Pass setup flags after `sh -s --`, and pin a release with `STUNSAILS_VERSION`:

```sh
curl -fsSL https://ikhsansdqq.github.io/stunsails/install.sh | sudo STUNSAILS_VERSION=v0.1.0 sh -s -- --domain ops.example.com
```

## What setup does

The installer puts the binary in `/usr/local/bin/stunsails`, then runs
`stunsails init`. It shows every command that changes the server before it
runs it:

1. **Checks the server**: root, systemd, cgroup v2, a supported distro, Docker,
   and where SSH sign-ins are logged.
2. **Docker Engine**: if it is missing, offers to install it from Docker's own
   repository (`apt` on Ubuntu and Debian, `dnf` on Fedora). Podman's docker
   shim is not enough.
3. **Service user and folders**: the `stunsails` system user, in the `docker`,
   `adm` and `systemd-journal` groups (those that exist), and
   `/etc/stunsails` (`0700`).
4. **Sign-in**: asks for a password (12 characters or more) and shows a QR code
   to pair your authenticator app. Both are kept on reinstall.
5. **Compose helper image**: pulls `docker:29-cli`.
6. **Service**: writes `/etc/systemd/system/stunsails.service`, keeping the
   previous binary and unit as `.prev`, starts it, and waits until it answers.
   On Fedora it also restores the binary's SELinux label.
7. **Reaching it**: prints the Caddy or Nginx block for your domain, and the
   firewall command if web ports are closed. It never edits your proxy or
   firewall itself.

StunSails now listens on `127.0.0.1:8099`. Next: [Reverse proxy](./reverse-proxy).

| Flag | Effect |
|---|---|
| `--yes` | take every default without asking |
| `--no-docker-install` | stop instead of installing Docker |
| `--domain` | the domain to use in the proxy snippet |
| `--binary` | the binary to install (default: the one running) |

## Check it

```sh
sudo stunsails doctor
```

`doctor` changes nothing. It checks the server again, then the binary, unit,
service user, credential permissions, that the service runs, and that the
dashboard answers. It exits non-zero when something fails, with what to do.

## From source

```sh
git clone https://github.com/ikhsansdqq/stunsails.git
cd stunsails
./build.sh                 # dashboard + binary → ./stunsails, built in Docker images
sudo ./deploy/install.sh   # runs ./stunsails init with this binary
```

## Moving from Harbor

StunSails used to be called Harbor and installed as `server-monitor`. `init`
finds that install and moves it over: it stops the old service, moves
`/var/lib/server-monitor` and `/etc/server-monitor` to the new paths (your
history, password and phone pairing come along), and keeps the old binary and
unit as `.prev`. `doctor` points this out while the old install is still there.

## Run without installing

For development, against the real Docker, over plain HTTP:

```sh
sudo env LISTEN=127.0.0.1:8098 DATA_DIR=/tmp/stunsails \
  TOTP_FILE=/tmp/stunsails/totp PASSWORD_FILE=/tmp/stunsails/password \
  INSECURE_COOKIES=1 ./stunsails serve
cd web && npm run dev      # proxies /api to 8098
```

Create the two files first with `./stunsails passwd` and
`./stunsails enroll`, with `PASSWORD_FILE` and `TOTP_FILE` set the same way.

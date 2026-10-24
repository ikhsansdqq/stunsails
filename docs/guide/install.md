# Install

StunSails installs as a systemd service that runs as its own `stunsails`
system user, in the `docker` and `adm` groups.

## 1. Get the source and build

```sh
git clone https://github.com/stunsails/stunsails.git
cd stunsails
./build.sh          # dashboard + binary → ./stunsails
```

`./build.sh` runs both builds in Docker images and leaves no `node_modules`
behind. Other targets:

```sh
./build.sh go       # binary only, reusing web/dist
./build.sh test     # gofmt check, go vet, go test
```

## 2. Install the service

```sh
sudo ./deploy/install.sh
```

The script:

1. Creates the `stunsails` system user and adds it to `docker` and `adm`.
2. Asks for a sign-in password the first time (12 characters or more).
3. Shows a QR code to pair your authenticator app, unless one is paired already.
4. Pulls `docker:29-cli`, the image compose actions run in.
5. Copies the binary to `/usr/local/bin/stunsails` and the unit to
   `/etc/systemd/system/stunsails.service`, keeping the previous ones as
   `.prev`.
6. Enables and restarts the service.

StunSails now listens on `127.0.0.1:8099`. It is not reachable from outside until
you put a reverse proxy in front: see [Reverse proxy](./reverse-proxy).

## 3. Check it runs

```sh
systemctl status stunsails
journalctl -u stunsails -f
curl -s 127.0.0.1:8099/api/session
```

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

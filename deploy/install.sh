#!/bin/sh
# Installs or upgrades Harbor from this checkout. Build first (./build.sh), then:
#   sudo ./deploy/install.sh
# Asks for a password the first time, and pairs an authenticator only when no
# valid secret is in /etc/server-monitor/totp already.
set -eu
cd "$(dirname "$0")/.."
[ "$(id -u)" = 0 ] || { echo "Run with sudo." >&2; exit 1; }
[ -x ./server-monitor ] || { echo "Build first: ./build.sh" >&2; exit 1; }

# Harbor v2 ran as a DynamicUser of the same name, which only exists while the
# service runs: stop it first, then create the real system user.
if ! grep -q '^server-monitor:' /etc/passwd; then
	systemctl stop server-monitor 2>/dev/null || true
	useradd --system --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin server-monitor
fi
usermod -aG docker,adm server-monitor

install -d -m 700 /etc/server-monitor
if [ ! -s /etc/server-monitor/password ]; then
	echo "Choose Harbor's sign-in password (12 characters or more)."
	./server-monitor passwd
fi
if ! grep -Eq '^[A-Z2-7]{16,}=*$' /etc/server-monitor/totp 2>/dev/null; then
	echo "Pair Google Authenticator."
	./server-monitor enroll
else
	echo "Keeping the authenticator already paired."
fi

# The helper image compose actions run in; pulled now so the first action is quick.
docker pull -q docker:29-cli >/dev/null

if [ -f /usr/local/bin/server-monitor ]; then
	cp -p /usr/local/bin/server-monitor /usr/local/bin/server-monitor.prev
	cp -p /etc/systemd/system/server-monitor.service /etc/systemd/system/server-monitor.service.prev
fi
install -m 755 ./server-monitor /usr/local/bin/server-monitor
# Files Harbor v1 left in its state folder.
rm -f /var/lib/private/server-monitor/history.bin /var/lib/private/server-monitor/session.key
install -m 644 deploy/server-monitor.service /etc/systemd/system/server-monitor.service
systemctl daemon-reload
systemctl enable server-monitor >/dev/null 2>&1
systemctl restart server-monitor
sleep 2
systemctl --no-pager --lines=5 status server-monitor
echo
echo "Rollback: sudo cp /usr/local/bin/server-monitor.prev /usr/local/bin/server-monitor"
echo "          sudo cp /etc/systemd/system/server-monitor.service.prev /etc/systemd/system/server-monitor.service"
echo "          sudo systemctl daemon-reload && sudo systemctl restart server-monitor"

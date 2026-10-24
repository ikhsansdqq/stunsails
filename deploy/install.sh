#!/bin/sh
# Installs or upgrades StunSails from this checkout. Build first (./build.sh), then:
#   sudo ./deploy/install.sh
# Asks for a password the first time, and pairs an authenticator only when no
# valid secret is in /etc/stunsails/totp already.
set -eu
cd "$(dirname "$0")/.."
[ "$(id -u)" = 0 ] || { echo "Run with sudo." >&2; exit 1; }
[ -x ./stunsails ] || { echo "Build first: ./build.sh" >&2; exit 1; }

# Before the rename to StunSails this was Harbor, installed as server-monitor:
# stop it and move its state and credentials over once. The old binary and unit
# stay as .prev for a rollback, and the old user is left for you to remove.
if [ -f /etc/systemd/system/server-monitor.service ]; then
	echo "Moving Harbor (server-monitor) to StunSails."
	systemctl disable --now server-monitor 2>/dev/null || true
	if [ -e /var/lib/server-monitor ] && [ ! -e /var/lib/stunsails ]; then
		mv "$(readlink -f /var/lib/server-monitor)" /var/lib/stunsails
		rm -f /var/lib/server-monitor
	fi
	if [ -d /etc/server-monitor ] && [ ! -e /etc/stunsails ]; then
		mv /etc/server-monitor /etc/stunsails
	fi
	mv /etc/systemd/system/server-monitor.service /etc/systemd/system/server-monitor.service.prev
	[ -f /usr/local/bin/server-monitor ] && mv /usr/local/bin/server-monitor /usr/local/bin/server-monitor.prev
	systemctl daemon-reload
	migrated=1
fi

if ! grep -q '^stunsails:' /etc/passwd; then
	useradd --system --no-create-home --home-dir /nonexistent --shell /usr/sbin/nologin stunsails
fi
usermod -aG docker,adm stunsails
[ -d /var/lib/stunsails ] && chown -R stunsails:stunsails /var/lib/stunsails

install -d -m 700 /etc/stunsails
if [ ! -s /etc/stunsails/password ]; then
	echo "Choose StunSails' sign-in password (12 characters or more)."
	./stunsails passwd
fi
if ! grep -Eq '^[A-Z2-7]{16,}=*$' /etc/stunsails/totp 2>/dev/null; then
	echo "Pair Google Authenticator."
	./stunsails enroll
else
	echo "Keeping the authenticator already paired."
fi

# The helper image compose actions run in; pulled now so the first action is quick.
docker pull -q docker:29-cli >/dev/null

if [ -f /usr/local/bin/stunsails ]; then
	cp -p /usr/local/bin/stunsails /usr/local/bin/stunsails.prev
	cp -p /etc/systemd/system/stunsails.service /etc/systemd/system/stunsails.service.prev
fi
install -m 755 ./stunsails /usr/local/bin/stunsails
install -m 644 deploy/stunsails.service /etc/systemd/system/stunsails.service
systemctl daemon-reload
systemctl enable stunsails >/dev/null 2>&1
systemctl restart stunsails
sleep 2
systemctl --no-pager --lines=5 status stunsails
echo
echo "Rollback: sudo cp /usr/local/bin/stunsails.prev /usr/local/bin/stunsails"
echo "          sudo cp /etc/systemd/system/stunsails.service.prev /etc/systemd/system/stunsails.service"
echo "          sudo systemctl daemon-reload && sudo systemctl restart stunsails"
if [ "${migrated:-}" = 1 ]; then
	echo
	echo "Moved from server-monitor. Once StunSails works, remove the old user and copies:"
	echo "  sudo userdel server-monitor"
	echo "  sudo rm /usr/local/bin/server-monitor.prev /etc/systemd/system/server-monitor.service.prev"
	echo "To go back to Harbor instead:"
	echo "  sudo systemctl disable --now stunsails"
	echo "  sudo mv /var/lib/stunsails /var/lib/server-monitor && sudo mv /etc/stunsails /etc/server-monitor"
	echo "  sudo chown -R server-monitor:server-monitor /var/lib/server-monitor"
	echo "  sudo mv /var/lib/server-monitor/stunsails.db /var/lib/server-monitor/harbor.db"
	echo "  sudo mv /usr/local/bin/server-monitor.prev /usr/local/bin/server-monitor"
	echo "  sudo mv /etc/systemd/system/server-monitor.service.prev /etc/systemd/system/server-monitor.service"
	echo "  sudo systemctl daemon-reload && sudo systemctl enable --now server-monitor"
fi

# Activity

**Activity** is the server's log of who did what. It keeps 30 days and pages on
the server.

## What it records

- StunSails sign-ins, failed tries, lockouts, and every change made from the
  dashboard.
- SSH sign-ins and `sudo` from `/var/log/auth.log`.
- Failed SSH logins, summed every 10 minutes. Each summary keeps every address
  with its attempts and the users it tried.
- Failed systemd units and site outages.

Open a row to see each address with its country, city and network.

## IP locations

StunSails uses DB-IP's free **Lite** city and ASN databases (CC BY 4.0). They are
downloaded to `/var/lib/stunsails/geo/` at start and refreshed monthly.
**No address leaves the server for a lookup.**

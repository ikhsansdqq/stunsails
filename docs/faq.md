# FAQ

## How is this different from Portainer, Coolify or Dokploy?

StunSails is smaller and narrower. It doesn't build from git or run a PaaS. It
watches one server, manages what Docker and compose already run, alerts on what
breaks, and puts an authenticator code in front of every change. It uses about
10 MB of memory.

## Can it watch more than one server?

Not yet. Run one StunSails per server.

## Does it send anything anywhere?

Only what you set up: alert emails and webhooks. Plus a monthly download of the
DB-IP database. No telemetry.

## How much disk does history use?

About 1 MB per 10 minutes of a 7-day window, so it levels off. Activity keeps 30
days.

## Can I use it without a reverse proxy?

Not safely. StunSails serves plain HTTP on loopback; HTTPS has to come from
somewhere.

## What license is it under?

GNU AGPL version 3 or later. You can use, change and self-host it freely. If
you offer a modified version to others over a network, you must offer them its
source too. DB-IP Lite data is CC BY 4.0.

# Compose stacks

StunSails finds compose stacks on its own from the labels compose puts on every
container (`com.docker.compose.project`, `…working_dir`, `…config_files`).
Nothing to register.

## Actions

| Action | How it runs |
|---|---|
| Up | `docker compose up -d` |
| Pull and update | `docker compose pull`, then `up -d` |
| Restart, start, stop | directly on the stack's containers through the Engine API |
| Down | `docker compose down` |

Compose runs in a short-lived `docker:29-cli` container with the stack's folder
mounted **read-only at the same path**, so relative binds and `.env` files
resolve as they did when you started the stack. StunSails itself never needs read
access to your home folder.

Start, stop and restart skip the compose CLI on purpose: compose parses the
whole file first, and a file that needs a variable only your deploy script sets
(`${TAG:?}`) would refuse even a stop.

Output streams to the dashboard while it runs.

## Stacks that are down

A stack taken down keeps its row (state **down**) so you can bring it back up.
**Forget** removes the row.

## Manageable or not

A stack is manageable when StunSails knows its working folder and compose files.
Stacks started some other way (or by an old compose) show read-only.

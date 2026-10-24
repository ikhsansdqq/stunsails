# Alerts

Set in **Settings → Alerts**. StunSails sends by email (SMTP), by webhook, or both.
Saving needs a fresh authenticator code; **Send test** does not.

## What it watches

| Rule | Fires when |
|---|---|
| A container crashes | it exits with a non-zero code other than 137 or 143 (a deliberate kill or stop) |
| A container keeps restarting | it dies 3 times within 15 minutes |
| A health check fails | Docker reports it `unhealthy` |
| A container runs out of memory | the kernel OOM-kills it |
| The disk is nearly full | over 90%, or forecast to fill within a week |
| Memory stays high | over 90% for 5 minutes |
| A site stops answering | two checks in a row fail |
| A certificate expires soon | within 14 days |
| A system service fails | `systemctl --failed` lists it |
| Many failed SSH logins | a burst in the auth log |
| The scheduled cleanup ran | a summary after each run |

Each rule can be switched off. Each problem is sent once, again after 30
minutes if it persists, and once more when it clears.

## Email

| Field | Notes |
|---|---|
| Host, Port | Port `465` uses implicit TLS; any other port uses STARTTLS when the server offers it. |
| User, Password | The password is stored in `stunsails.db` and never sent back to the browser. |
| From, To | Several recipients allowed. |

## Webhook

StunSails posts JSON to the URL:

```json
{
  "title": "api-1 crashed",
  "message": "It exited with code 1 (image api:latest).",
  "severity": "critical",
  "rule": "crash",
  "target": "api-1",
  "host": "vps-1",
  "at": "2026-10-24T03:12:00Z",
  "content": "🔴 api-1 crashed · vps-1\nIt exited with code 1 (image api:latest).",
  "text": "🔴 api-1 crashed · vps-1\nIt exited with code 1 (image api:latest)."
}
```

`content` and `text` make it readable as is in **Discord** and **Slack**
incoming webhooks. For **ntfy**, use `https://ntfy.sh/<topic>` with a JSON
template, or route through **n8n** and use the structured fields.

`severity` is `critical`, `warning`, `resolved` or `info`.

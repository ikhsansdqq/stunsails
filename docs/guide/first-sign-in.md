# First sign-in

Open the address your reverse proxy serves. Sign-in has two steps: the
password, then the six-digit code from your authenticator app.

## Password and authenticator

```sh
sudo stunsails passwd        # set or change the password
sudo stunsails enroll        # pair a new phone (replaces the old key)
sudo stunsails enroll --show # show the current key again, for a second phone
sudo systemctl restart stunsails
```

The password is stored as a bcrypt hash in `/etc/stunsails/password`; the
authenticator key in `/etc/stunsails/totp`. Both are mode `0600` and reach
the service through systemd `LoadCredential`, so the service user cannot read
them from `/etc`.

::: warning
`enroll` without `--show` makes a new key. Any phone paired with the old key
stops working.
:::

## Codes for changes

Looking needs only a signed-in session. Changing anything — container and stack
actions, removals, prunes, settings — also needs a code entered within the last
five minutes. The dashboard asks for one when it is due.

## Lockouts

- Five wrong tries from one address lock that address out for 15 minutes. The
  lockout survives a restart.
- Over 50 wrong tries across all addresses: every try waits 2 seconds instead,
  so an attacker spraying addresses cannot lock you out.

Every sign-in, failed try and change is recorded in [Activity](../features/activity).

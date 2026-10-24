# Reverse proxy

StunSails listens on `127.0.0.1:8099` and serves plain HTTP. Put a proxy in front
that terminates HTTPS. Session cookies are `Secure`, so sign-in only works over
HTTPS (or with `INSECURE_COOKIES=1` for local development).

StunSails sends `Strict-Transport-Security` when the proxy sets
`X-Forwarded-Proto: https`.

## The visitor's address

Lockouts and Activity need the real visitor address. StunSails trusts forwarding
headers **only when the request comes from loopback**, i.e. from your proxy:

1. `Cf-Connecting-Ip`, if set, wins.
2. Otherwise the **last** address in `X-Forwarded-For` (the one your proxy
   appended).

::: warning
Your proxy must **overwrite** `Cf-Connecting-Ip` on every request, or clear it.
If it passes the header through from the visitor, anyone can send a fake one and
dodge the lockout.
:::

## Caddy

```txt
stunsails.example.com {
	reverse_proxy 127.0.0.1:8099 {
		header_up Cf-Connecting-Ip {client_ip}
	}
}
```

Caddy gets and renews the certificate on its own, and appends the client to
`X-Forwarded-For`. StunSails also reads `/etc/caddy/Caddyfile` (or `CADDYFILE`) to
find every site you serve and check it every minute: see
[Sites and certificates](../features/sites).

## Nginx

```nginx
server {
    listen 443 ssl;
    http2 on;
    server_name stunsails.example.com;

    ssl_certificate     /etc/letsencrypt/live/stunsails.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/stunsails.example.com/privkey.pem;

    location / {
        proxy_pass http://127.0.0.1:8099;
        proxy_set_header Host $host;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_set_header Cf-Connecting-Ip $remote_addr;
        # Logs and compose output stream.
        proxy_buffering off;
        proxy_read_timeout 1h;
    }
}
```

## Behind Cloudflare

When Cloudflare proxies the domain, the address your proxy sees is Cloudflare's.
Trust Cloudflare's edge ranges in Caddy's global options, and take the visitor
from `CF-Connecting-IP` only when the request really came from one of them. Then
`{client_ip}` is the visitor and can't be forged by going around Cloudflare:

```txt
{
	servers {
		# https://www.cloudflare.com/ips/ — keep this list current.
		trusted_proxies static 173.245.48.0/20 103.21.244.0/22 103.22.200.0/22 103.31.4.0/22 141.101.64.0/18 108.162.192.0/18 190.93.240.0/20 188.114.96.0/20 197.234.240.0/22 198.41.128.0/17 162.158.0.0/15 104.16.0.0/13 104.24.0.0/14 172.64.0.0/13 131.0.72.0/22 2400:cb00::/32 2606:4700::/32 2803:f800::/32 2405:b500::/32 2405:8100::/32 2a06:98c0::/29 2c0f:f248::/32
		client_ip_headers CF-Connecting-IP
	}
}

(real_ip) {
	header_up X-Forwarded-For {client_ip}
	header_up Cf-Connecting-Ip {client_ip}
}

stunsails.example.com {
	reverse_proxy 127.0.0.1:8099 {
		import real_ip
	}
}
```

To accept web traffic only from Cloudflare, `deploy/ufw-cloudflare.sh` allows
ports 80 and 443 from Cloudflare's published ranges in UFW:

```sh
sudo sh deploy/ufw-cloudflare.sh
```

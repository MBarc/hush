# Deploying Hush

Hush is one container and one data volume. Everything lives under `/data`:
the encrypted SQLite database, the master key, and the local admin socket.

## Compose (recommended)

```yaml
services:
  hush:
    image: ghcr.io/mbarc/hush:latest
    container_name: hush
    ports:
      - "4874:4874"
    volumes:
      - hush-data:/data
    environment:
      - HUSH_NETWORK_CIDR=192.168.1.0/24   # optional, enables device discovery
      - HUSH_POLL_INTERVAL=5m
    restart: unless-stopped

volumes:
  hush-data:
```

```sh
docker compose up -d
docker compose logs hush | grep password
```

## Device discovery and host networking

The network poller sweeps `HUSH_NETWORK_CIDR` to build the device inventory.
On the default bridge network it only sees other containers, not your LAN.
For real device discovery, run on the host network:

```yaml
services:
  hush:
    image: ghcr.io/mbarc/hush:latest
    network_mode: host          # poller now sees the real LAN
    volumes:
      - hush-data:/data
    environment:
      - HUSH_NETWORK_CIDR=192.168.1.0/24
```

With host networking the port mapping is dropped; Hush listens on
`0.0.0.0:4874` directly. Change the port with `HUSH_LISTEN=:8200`.

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `HUSH_LISTEN` | `:4874` | Listen address |
| `HUSH_DATA` | `/data` | Data directory (db, master key, socket) |
| `HUSH_ADMIN_PASSWORD` | generated | First-boot admin password (else printed to logs) |
| `HUSH_NETWORK_CIDR` | unset | LAN subnet to poll; unset disables discovery |
| `HUSH_POLL_INTERVAL` | `5m` | How often to sweep the network |
| `HUSH_SOCKET` | `/data/hush.sock` | Local admin socket; `off` to disable |
| `HUSH_TLS_CERT` / `HUSH_TLS_KEY` | unset | Serve HTTPS directly |

## TLS

Two options:

1. **Reverse proxy** (recommended for homelabs). Terminate TLS at Caddy,
   Nginx Proxy Manager, or Traefik and forward to `hush:4874`. Hush honors
   the first `X-Forwarded-For` hop for audit logging. Note that device
   identity uses the real connection IP, so devices must reach Hush
   directly, not through a proxy that rewrites the source address.

2. **Direct TLS**. Mount a cert and key and set `HUSH_TLS_CERT` and
   `HUSH_TLS_KEY`.

## Backups

Everything is in the `/data` volume. Stop the container (or accept a
slightly inconsistent copy) and archive it:

```sh
docker run --rm -v hush-data:/data -v "$PWD":/backup alpine \
  tar czf /backup/hush-backup.tar.gz -C /data .
```

`master.key` is inside the volume. The database is useless without it, so
back them up together and store the archive somewhere safe.

## Upgrades

```sh
docker compose pull
docker compose up -d
```

The schema migrates forward automatically on start.

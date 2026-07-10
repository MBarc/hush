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

The poller finds a host two ways: a TCP probe it answers (or refuses), and
the kernel ARP table. The probe pass forces the subnet to ARP-resolve, so a
host that silently drops probes (a firewalled desktop) still answers ARP and
shows up. Host networking is required for the ARP pass to see the real table.

A ready-made host-networking compose file is included:

```sh
docker compose -f docker-compose.host.yml up -d --build
```

### Docker Desktop (Windows / Mac)

Docker Desktop runs containers inside a NAT'd Linux VM, so by default a
container - even with `network_mode: host` - cannot reach your physical
LAN. Two settings fix that:

1. **WSL mirrored networking** (Windows 11 22H2+). Add to `~/.wslconfig`:

   ```ini
   [wsl2]
   networkingMode=mirrored
   dnsTunneling=true
   firewall=true
   ```

2. **Docker Desktop host networking**. Settings, Resources, Network,
   enable "host networking" (or `EnableHostNetworking: true` in
   `%APPDATA%\Docker\settings-store.json`).

Then apply both by fully restarting the WSL backend:

```powershell
wsl --shutdown
```

Docker Desktop restarts the VM automatically. Note that `wsl --shutdown`
stops every running container, so do it when nothing else is mid-task.
After it comes back, `docker compose -f docker-compose.host.yml up -d`
and the poller will see your LAN.

## Environment variables

| Variable | Default | Purpose |
|----------|---------|---------|
| `HUSH_LISTEN` | `:4874` | Listen address |
| `HUSH_DATA` | `/data` | Data directory (db, master key, socket) |
| `HUSH_ADMIN_PASSWORD` | generated | First-boot admin password (else printed to logs) |
| `HUSH_NETWORK_CIDR` | unset | LAN subnet to poll; unset disables discovery |
| `HUSH_POLL_INTERVAL` | `5m` | How often to sweep the network |
| `HUSH_AUDIT_RETENTION_DAYS` | `90` | Delete audit entries older than this; `0` keeps forever |
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

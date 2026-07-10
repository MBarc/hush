<p align="center">
  <img src="assets/logo.svg" alt="hush" width="260">
</p>

<p align="center"><b>A quiet little vault for your homelab.</b></p>

<p align="center">
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/license-MIT-8F6FFF"></a>
  <a href=".github/workflows/ci.yml"><img alt="CI" src="https://github.com/MBarc/hush/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="status" src="https://img.shields.io/badge/status-pre--release-1D1730">
</p>

---

Hush is an open source secrets vault built only for homelabs. It stores your
passwords, hands them out over a simple API, and writes down everything that
asked. No unseal ceremonies, no clustering, no enterprise tiers. One
container, one volume, done.

## Why another vault?

- **Vault** is built for fleets and platform teams. You do not need Shamir
  key shares to protect your Jellyfin admin password.
- **Bitwarden** is built for humans with browsers. Your cron jobs, containers,
  and AI agents cannot click an extension.
- **Hush** is built for the machines in your house, and for the person who
  runs them.

## What it does

- **Secrets over an API.** Every password is one authenticated GET away.
  Scripts, containers, and agents fetch what they need, use it, and forget it.
- **AI agent access, on your terms.** An agent token lives in a folder and can
  read that folder and everything beneath it, and nothing else. Put the token
  where you want the agent to reach; revoke it to cut access instantly.
- **Device identity.** Hush polls your network and keeps a device inventory.
  Trust a device and your NAS can fetch its secrets by hostname alone, with
  no token to store. Claimed hostnames are verified against the connection's
  source IP, and grants carry method limits (GET or GET/POST) and expiry.
- **Rotation.** Per-secret rotation policies, on demand or on a schedule,
  with full version history and an optional HMAC-signed webhook so your
  automation can push the new password into the real service.
- **Folders, like a filesystem.** `infra/proxmox/root`, granted and browsed
  the way you already think.
- **Local accounts only.** Admins run the vault. Readonly users see only the
  folder subtrees granted to them, and grants cascade.
- **Audit everything.** Every read, write, rotation, login, and device access
  is logged with who, what, when, and from where.
- **CLI parity.** Everything the web UI does, `hush` does in a terminal. On
  the vault host, `docker exec hush hush ls` is already admin. That is the
  homelab way.

## Quickstart

On a Linux host that sits on your LAN (a Raspberry Pi, homelab server, or
VM), one command clones, builds, detects your subnet, and starts Hush with
device discovery on:

```sh
curl -fsSL https://raw.githubusercontent.com/MBarc/hush/main/scripts/install.sh | sh
```

It prints your one-time admin password at the end. Open `http://<host>:4874`,
sign in as `admin`, and change it. Why port 4874? Spell HUSH on a phone keypad.

Prefer to do it by hand, or just trying it out without network scanning?

```sh
git clone https://github.com/MBarc/hush.git
cd hush
docker compose up -d
docker compose logs hush | grep password   # your one-time admin password
```

> Device discovery needs Hush on your real LAN, so it wants a Linux host with
> host networking (what the installer sets up). It does not work from Docker
> Desktop on Windows or Mac, where containers sit behind a NAT. See
> [docs/DEPLOY.md](docs/DEPLOY.md).

### Give an AI agent a secret

Everything in Hush lives in a folder: secrets, credentials, and tokens alike.
An agent token is just an item you place in a folder.

1. Put the secrets the agent needs in a folder, like `infra/dns`.
2. Mint an agent token inside that folder:

   ```sh
   hush token create infra/dns/claude --type agent
   ```
3. The agent fetches what it needs, then forgets it:

   ```sh
   curl -H "Authorization: Bearer $HUSH_TOKEN" \
     http://vault:4874/api/v1/secrets/infra/dns/cloudflare
   ```

The token reads `infra/dns` and everything beneath it, and nothing else. A
read of any other path returns 404, as if it did not exist. Revoke the token
to cut access instantly. Every read is in the audit log.

### Let a device fetch by hostname, no token

Point Hush at your LAN and it builds a device inventory:

```yaml
# docker-compose.yml
environment:
  - HUSH_NETWORK_CIDR=192.168.1.0/24
```

Trust a discovered device, then it asks by name:

```sh
hush device trust nas01 --scope 'infra/nas/*' --allow-write
curl -H "X-Hush-Device: nas01" http://vault:4874/api/v1/secrets/infra/nas/backup-key
```

Hush only honors the claim if the request arrives from the IP it last saw
that hostname at, so a name alone is not enough to impersonate a device.

> Device discovery works best with `network_mode: host` so the poller sees
> your real LAN rather than the Docker bridge. See `docs/DEPLOY.md`.

## Using the CLI

On the vault host, the CLI talks to the server over a local socket and is
automatically admin. No login:

```sh
docker exec hush hush ls infra/
docker exec hush hush get infra/proxmox/root
docker exec hush hush rotate infra/proxmox/root
```

From another machine, point it at the server and log in once (this stores a
personal token in `~/.hush/config.json`):

```sh
hush login --addr http://vault:4874 --username admin
hush ls infra/
```

Everything the web UI can do, the CLI can do. Add `--json` to any command
for scripting.

## Documentation

- [docs/DEPLOY.md](docs/DEPLOY.md) - compose recipes, host networking, TLS, backups
- [docs/API.md](docs/API.md) - the REST API the UI, CLI, and agents all use
- [docs/DESIGN.md](docs/DESIGN.md) - visual design language and palette

## Status

Pre-release and moving fast. Current progress:

- [x] Brand, design tokens, containerized skeleton
- [x] Encrypted storage core (SQLite, envelope encryption)
- [x] Auth: users, sessions, tokens, folder grants
- [x] CLI with full parity
- [x] Device identity: network poller + hostname access
- [x] Rotation: policies, scheduler, webhooks
- [x] Web UI (dark, violet, quiet)
- [x] Release workflow (multi-arch image to GHCR on tag)

Design language and palette live in [docs/DESIGN.md](docs/DESIGN.md).

## License

[MIT](LICENSE)

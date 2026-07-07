<p align="center">
  <img src="assets/logo.svg" alt="hush" width="260">
</p>

<p align="center"><b>A quiet little vault for your homelab.</b></p>

<p align="center">
  <a href="LICENSE"><img alt="MIT license" src="https://img.shields.io/badge/license-MIT-8F6FFF"></a>
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
- **AI agent access, on your terms.** Agent tokens are scoped to folder paths
  and only work on secrets you have individually marked as agent-accessible.
  Flip one toggle and an agent can never see that secret, regardless of scope.
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

```sh
git clone https://github.com/MBarc/hush.git
cd hush
docker compose up -d
```

Then open `http://<host>:4874`. Why 4874? Spell HUSH on a phone keypad.

A published image on GHCR arrives with the first release.

## Status

Pre-release and moving fast. Current progress:

- [x] Brand, design tokens, containerized skeleton
- [x] Encrypted storage core (SQLite, envelope encryption)
- [x] Auth: users, sessions, tokens, folder grants
- [x] CLI with full parity
- [x] Device identity: network poller + hostname access
- [x] Rotation: policies, scheduler, webhooks
- [ ] Web UI (dark, violet, quiet)
- [ ] v1 on GHCR

Design language and palette live in [docs/DESIGN.md](docs/DESIGN.md).

## License

[MIT](LICENSE)

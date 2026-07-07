# Hush design language

Dark-first visual identity for the Hush web UI, CLI output, logo, and docs.

## Market research: how the neighbors dress

| Tool | Identity | Takeaway |
|------|----------|----------|
| Bitwarden | Trust blue (#175DDC) on white/dark | Blue is the default "password manager" color |
| 1Password | Trust blue (#0364D3, #0572EC) | Same territory as Bitwarden |
| CyberArk | Corporate blue/teal | Enterprise blue again |
| HashiCorp Vault | Yellow on black | Owns yellow-on-dark in the vault space |
| Infisical | Yellow/amber on dark | Crowds the same yellow lane |
| Doppler | Terminal green (2023 rebrand) | Owns the "code green" lane |

Blue says corporate trust, yellow is taken twice, green is taken once.
Nobody in the space owns **violet on near-black**, and violet is the natural
color of the name: night, quiet, secrecy. Hush claims it.

## Palette

Base surfaces carry a subtle violet cast rather than being pure gray, so the
UI feels branded even before any accent color appears.

| Token | Hex | Use |
|-------|-----|-----|
| `bg-base` | `#0D0B12` | App background |
| `bg-surface` | `#151020` | Cards, panels, sidebar |
| `bg-raised` | `#1D1730` | Hover states, inputs, modals |
| `border` | `#2A2140` | Default borders, dividers |
| `border-strong` | `#3A2F57` | Focused/active borders |
| `text-primary` | `#EDE9F8` | Headings, values |
| `text-secondary` | `#A79BC4` | Labels, descriptions |
| `text-muted` | `#6E6389` | Placeholders, timestamps |
| `accent` | `#8F6FFF` | Primary actions, links, focus rings, logo |
| `accent-hover` | `#A78BFF` | Hover on primary actions |
| `accent-deep` | `#5F3DD6` | Pressed states, gradients |
| `agent` | `#35D0BA` | Everything AI-agent related (see below) |
| `success` | `#3ECF8E` | Healthy, rotation succeeded |
| `warning` | `#F2B24E` | Rotation due, expiring tokens |
| `danger` | `#F0546C` | Destructive actions, failed logins |

### The agent teal rule

Teal (`#35D0BA`) is reserved exclusively for AI-agent surface area: agent
tokens, the per-secret agent-access toggle, agent rows in the audit log.
Nothing else may use it. A user scanning any screen can answer "what can an
AI touch?" by color alone.

## Typography

- UI: `Inter, system-ui, sans-serif`
- Secret paths, values, tokens, audit entries: `"JetBrains Mono", ui-monospace, monospace`
- Wordmark: lowercase `hush`, bold, slightly tightened letter spacing

## Shape

- Cards and panels: 10px radius
- Inputs and buttons: 6px radius
- Spacing on a 4px grid
- Masked secrets render as five dots in the mono font, never asterisks

## Logo

A speech bubble holding three masked-password dots: something said, but
hidden. Violet gradient bubble (`#A78BFF` to `#7C5CFF`) with `bg-base` dots.
Reads clearly at 16x16 (favicon) and inverts cleanly for light contexts.
Files: `assets/logo.svg` (icon + wordmark), `assets/icon.svg` (icon only,
used as the favicon).

#!/bin/sh
# Hush installer for a Linux host (Raspberry Pi, homelab server, or VM).
#
# It clones or updates the repo, detects your LAN subnet automatically,
# then builds and runs Hush on host networking so the device poller can
# see your network.
#
# One-liner:
#   curl -fsSL https://raw.githubusercontent.com/MBarc/hush/main/scripts/install.sh | sh
#
# Or from a checkout:
#   ./scripts/install.sh
#
# Overrides (optional environment variables):
#   HUSH_NETWORK_CIDR  LAN subnet, like 192.168.1.0/24 (else auto-detected)
#   HUSH_DIR           where to clone (default: ~/hush)
#   HUSH_LISTEN        listen address (default: :4874)
set -eu

REPO_URL="https://github.com/MBarc/hush.git"
INSTALL_DIR="${HUSH_DIR:-$HOME/hush}"

info() { printf '\033[38;5;99m==>\033[0m %s\n' "$1"; }
warn() { printf '\033[38;5;179m!\033[0m %s\n' "$1"; }
die()  { printf '\033[38;5;203mx\033[0m %s\n' "$1" >&2; exit 1; }

# --- 1. Prerequisites -------------------------------------------------------

command -v git >/dev/null 2>&1 || die "git is not installed. Try: sudo apt install -y git"
command -v docker >/dev/null 2>&1 || \
  die "Docker is not installed. Install it first: curl -fsSL https://get.docker.com | sh"
docker compose version >/dev/null 2>&1 || \
  die "Docker Compose v2 is required (it ships with modern Docker)."
if ! docker info >/dev/null 2>&1; then
  die "Cannot talk to Docker. Run this script with sudo, or add yourself to the docker group: sudo usermod -aG docker \$USER (then log out and back in)."
fi

# --- 2. Get the source ------------------------------------------------------

if [ -f "docker-compose.host.yml" ] && [ -d "cmd/hush" ]; then
  info "Using the Hush checkout in $(pwd)"
elif [ -d "$INSTALL_DIR/.git" ]; then
  info "Updating existing checkout in $INSTALL_DIR"
  git -C "$INSTALL_DIR" pull --ff-only
  cd "$INSTALL_DIR"
else
  info "Cloning Hush into $INSTALL_DIR"
  git clone --depth 1 "$REPO_URL" "$INSTALL_DIR"
  cd "$INSTALL_DIR"
fi

# --- 3. Detect the LAN subnet ----------------------------------------------

cidr="${HUSH_NETWORK_CIDR:-}"
if [ -z "$cidr" ]; then
  iface=$(ip route show default 2>/dev/null | awk '{print $5; exit}')
  if [ -n "${iface:-}" ]; then
    # e.g. 192.168.5.124/22 - Hush computes the network from the host form.
    cidr=$(ip -o -f inet addr show "$iface" 2>/dev/null | awk '{print $4; exit}')
  fi
fi
[ -n "$cidr" ] || die "Could not detect your LAN subnet. Re-run with HUSH_NETWORK_CIDR=192.168.1.0/24 set."
info "Device discovery will scan: $cidr"

# --- 4. Build and run -------------------------------------------------------

info "Building and starting Hush. First run takes a few minutes..."
HUSH_NETWORK_CIDR="$cidr" \
HUSH_LISTEN="${HUSH_LISTEN:-:4874}" \
  docker compose -f docker-compose.host.yml up -d --build

# --- 5. Wait for it, then show how to get in --------------------------------

info "Waiting for Hush to come up..."
i=0
until docker logs hush 2>&1 | grep -q "listening on"; do
  i=$((i + 1))
  [ "$i" -gt 60 ] && die "Hush did not start. Check: docker logs hush"
  sleep 2
done

pw=$(docker logs hush 2>&1 | sed -n 's/.*password: //p' | tail -1)
host_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
: "${host_ip:=localhost}"

echo ""
info "Hush is running."
echo "   Web UI:   http://${host_ip}:4874"
echo "   Username: admin"
if [ -n "$pw" ]; then
  echo "   Password: ${pw}"
  echo "             (shown once - change it after you log in)"
else
  echo "   Password: already set on a previous run (docker logs hush, or reset in the UI)"
fi
echo ""
echo "   CLI on this host: docker exec hush hush device ls"
echo "   Stop:             docker compose -f docker-compose.host.yml down"
echo ""

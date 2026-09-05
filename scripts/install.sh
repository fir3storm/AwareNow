#!/usr/bin/env bash
# Build AwareNow (a rebranded, Gophish-compatible campaign engine) from source
# and install it as a systemd service, without touching the existing nginx
# :80/:443 listeners or unrelated application ports.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RUNTIME_DIR="${RUNTIME_DIR:-/opt/awarenow/runtime}"
ADMIN_HOST="${GOPHISH_ADMIN_HOST:-admin.itsupport.insec.in}"
PHISH_HOST="${GOPHISH_PHISH_HOST:-itsupport.insec.in}"
ADMIN_PORT="${GOPHISH_ADMIN_PORT:-3333}"
PHISH_PORT="${GOPHISH_PHISH_PORT:-8082}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo bash $0" >&2
  exit 1
fi

# --- Preflight: Go toolchain (must be pre-installed by the operator) -------
# We deliberately do NOT auto-install Go here: picking/managing a compiler
# version safely is out of scope for a root-run install script, and distro
# package managers often ship a Go far older than this module requires.
REQUIRED_GO_MAJOR=1
REQUIRED_GO_MINOR=21

if ! command -v go >/dev/null 2>&1; then
  echo "ERROR: Go is required to build AwareNow from source but was not found on PATH." >&2
  echo "  Install Go ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}+ from https://go.dev/dl/ or your distro's package manager, then re-run this script." >&2
  exit 1
fi

GO_VERSION_RAW="$(go version | awk '{print $3}')"          # e.g. "go1.21.5"
GO_VERSION_NUM="${GO_VERSION_RAW#go}"                        # e.g. "1.21.5"
GO_MAJOR="$(echo "$GO_VERSION_NUM" | cut -d. -f1)"
GO_MINOR="$(echo "$GO_VERSION_NUM" | cut -d. -f2)"

if [[ -z "$GO_MAJOR" || -z "$GO_MINOR" ]] || ! [[ "$GO_MAJOR" =~ ^[0-9]+$ && "$GO_MINOR" =~ ^[0-9]+$ ]]; then
  echo "ERROR: Could not parse Go version from '${GO_VERSION_RAW}'." >&2
  echo "  Install Go ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}+ from https://go.dev/dl/, then re-run this script." >&2
  exit 1
fi

if (( GO_MAJOR < REQUIRED_GO_MAJOR || (GO_MAJOR == REQUIRED_GO_MAJOR && GO_MINOR < REQUIRED_GO_MINOR) )); then
  echo "ERROR: Go ${REQUIRED_GO_MAJOR}.${REQUIRED_GO_MINOR}+ is required to build AwareNow (found ${GO_VERSION_NUM})." >&2
  echo "  Install a newer Go from https://go.dev/dl/ or your distro's package manager, then re-run this script." >&2
  exit 1
fi

apt-get update -qq
apt-get install -y -qq build-essential ca-certificates

# --- Preflight: C compiler (needed for the cgo sqlite3 driver) -------------
# build-essential was just installed above, so this should always pass on
# Debian/Ubuntu; kept as a defensive check for non-apt distros / odd images.
if ! command -v cc >/dev/null 2>&1 && ! command -v gcc >/dev/null 2>&1; then
  echo "ERROR: A C compiler (gcc) is required to build AwareNow's cgo-based sqlite3 driver." >&2
  echo "  Install a C toolchain (e.g. 'build-essential' on Debian/Ubuntu), then re-run this script." >&2
  exit 1
fi

if ! id -u gophish >/dev/null 2>&1; then
  useradd --system --home "$RUNTIME_DIR" --shell /usr/sbin/nologin gophish
fi

mkdir -p "$RUNTIME_DIR"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Building AwareNow from source..."
( cd "$REPO_ROOT" && CGO_ENABLED=1 go build -o "$TMP/gophish" . )

# --- Best-effort: stage the future control-plane SPA (web/) ---------------
# Nothing serves this yet (the Go server renders templates/ directly via
# template.ParseFiles, with no reference to web/dist), so this step must
# never abort the install of the actual campaign-engine service below.
if command -v npm >/dev/null 2>&1; then
  if ( cd "$REPO_ROOT/web" && npm ci && npm run build ); then
    mkdir -p "$RUNTIME_DIR/web"
    rm -rf "$RUNTIME_DIR/web/dist"
    cp -a "$REPO_ROOT/web/dist" "$RUNTIME_DIR/web/dist"
  else
    echo "WARNING: web/ frontend build failed, skipping (does not affect the campaign engine)." >&2
  fi
else
  echo "WARNING: npm not found, skipping web/ frontend build (does not affect the campaign engine)." >&2
fi

# Keep existing DB/config on re-run.
if [[ -f "$RUNTIME_DIR/gophish.db" ]]; then
  cp -a "$RUNTIME_DIR/gophish.db" "$TMP/gophish.db.bak"
fi
if [[ -f "$RUNTIME_DIR/config.json" ]]; then
  cp -a "$RUNTIME_DIR/config.json" "$TMP/config.json.bak"
fi

cp -a "$TMP/gophish" "$RUNTIME_DIR/gophish"
chmod +x "$RUNTIME_DIR/gophish"
rm -rf "$RUNTIME_DIR/templates" "$RUNTIME_DIR/static" "$RUNTIME_DIR/db"
cp -a "$REPO_ROOT/templates" "$RUNTIME_DIR/templates"
cp -a "$REPO_ROOT/static" "$RUNTIME_DIR/static"
cp -a "$REPO_ROOT/db" "$RUNTIME_DIR/db"

if [[ -f "$TMP/gophish.db.bak" ]]; then
  cp -a "$TMP/gophish.db.bak" "$RUNTIME_DIR/gophish.db"
fi
# Always refresh listen addresses / trusted origin so hostname changes apply.
# SQLite DB is restored above and is not in this file.
sed -e "s/__ADMIN_PORT__/${ADMIN_PORT}/g" \
    -e "s/__PHISH_PORT__/${PHISH_PORT}/g" \
    -e "s/__ADMIN_HOST__/${ADMIN_HOST}/g" \
    "$REPO_ROOT/deploy/config.json.example" > "$RUNTIME_DIR/config.json"

chown -R gophish:gophish "$RUNTIME_DIR"

install -m 0644 "$REPO_ROOT/deploy/gophish.service" /etc/systemd/system/gophish.service

NGINX_DEST="/etc/nginx/sites-available/gophish"
sed -e "s/__ADMIN_HOST__/${ADMIN_HOST}/g" \
    -e "s/__PHISH_HOST__/${PHISH_HOST}/g" \
    -e "s/__ADMIN_PORT__/${ADMIN_PORT}/g" \
    -e "s/__PHISH_PORT__/${PHISH_PORT}/g" \
    "$REPO_ROOT/deploy/nginx-gophish.conf.example" > "$NGINX_DEST"
ln -sfn "$NGINX_DEST" /etc/nginx/sites-enabled/gophish

systemctl daemon-reload
systemctl enable --now gophish

if command -v nginx >/dev/null 2>&1; then
  nginx -t
  systemctl reload nginx
fi

sleep 2
echo
echo "AwareNow is installed."
echo "  Admin (local):  http://127.0.0.1:${ADMIN_PORT}"
echo "  Admin (public): http://${ADMIN_HOST}   (needs DNS A record)"
echo "  Phish (public): http://${PHISH_HOST}   (needs DNS A record)"
echo
echo "Initial admin password is printed once on first start:"
journalctl -u gophish -n 80 --no-pager | grep -E "Please login with|password|username" || true
echo
echo "Then:"
echo "  1. Add DNS A records for ${ADMIN_HOST} and ${PHISH_HOST} → this VPS IP"
echo "  2. certbot --nginx -d ${ADMIN_HOST} -d ${PHISH_HOST}"
echo "  3. Open https://${ADMIN_HOST}  (user: admin)"

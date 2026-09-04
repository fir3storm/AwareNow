#!/usr/bin/env bash
# Install official Gophish v0.12.1 on the AwareCheck VPS without touching
# AwareCheck, nginx :80/:443, or ports 8000/8001/9000.
set -euo pipefail

GOPHISH_VERSION="0.12.1"
GOPHISH_SHA256="44f598c1eeb72c3b08fa73d57049022d96cea2872283b87a73d21af78a2c6d47"
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
RUNTIME_DIR="${RUNTIME_DIR:-/opt/gophish/runtime}"
ADMIN_HOST="${GOPHISH_ADMIN_HOST:-admin.itsupport.insec.in}"
PHISH_HOST="${GOPHISH_PHISH_HOST:-itsupport.insec.in}"
ADMIN_PORT="${GOPHISH_ADMIN_PORT:-3333}"
PHISH_PORT="${GOPHISH_PHISH_PORT:-8082}"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo bash $0" >&2
  exit 1
fi

apt-get update -qq
apt-get install -y -qq unzip curl ca-certificates

if ! id -u gophish >/dev/null 2>&1; then
  useradd --system --home "$RUNTIME_DIR" --shell /usr/sbin/nologin gophish
fi

mkdir -p "$RUNTIME_DIR"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ZIP="$TMP/gophish.zip"
echo "Downloading Gophish v${GOPHISH_VERSION}..."
curl -fsSL -o "$ZIP" \
  "https://github.com/gophish/gophish/releases/download/v${GOPHISH_VERSION}/gophish-v${GOPHISH_VERSION}-linux-64bit.zip"

echo "${GOPHISH_SHA256}  ${ZIP}" | sha256sum -c -

unzip -q -o "$ZIP" -d "$TMP/extract"
# Release zip is either a flat folder or a single top-level directory.
SRC="$TMP/extract"
if [[ "$(find "$TMP/extract" -mindepth 1 -maxdepth 1 | wc -l)" -eq 1 ]] && [[ -d "$(find "$TMP/extract" -mindepth 1 -maxdepth 1)" ]]; then
  SRC="$(find "$TMP/extract" -mindepth 1 -maxdepth 1 -type d)"
fi

# Keep existing DB/config on re-run.
if [[ -f "$RUNTIME_DIR/gophish.db" ]]; then
  cp -a "$RUNTIME_DIR/gophish.db" "$TMP/gophish.db.bak"
fi
if [[ -f "$RUNTIME_DIR/config.json" ]]; then
  cp -a "$RUNTIME_DIR/config.json" "$TMP/config.json.bak"
fi

cp -a "$SRC"/. "$RUNTIME_DIR/"
chmod +x "$RUNTIME_DIR/gophish"

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
echo "Gophish is installed."
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

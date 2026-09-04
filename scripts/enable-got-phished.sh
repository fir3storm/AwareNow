#!/usr/bin/env bash
# Add /got-phished to live nginx for itsupport.insec.in (HTTP + HTTPS).
# Does not overwrite the site file, so certbot certificates stay.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if [[ "$(id -u)" -ne 0 ]]; then
  echo "Run as root: sudo bash $0" >&2
  exit 1
fi

python3 "$REPO_ROOT/scripts/enable-got-phished.py"
nginx -t
systemctl reload nginx

echo
echo "Education page should now be: https://itsupport.insec.in/got-phished"
curl -sI --max-time 10 https://itsupport.insec.in/got-phished | head -n 8 || true

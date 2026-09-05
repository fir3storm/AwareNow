#!/usr/bin/env bash
# Fail if browser-served assets contain legacy product branding.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"

if command -v rg >/dev/null 2>&1; then
  SEARCH=(rg -n -i 'gophish|awarecheck|awarechck')
else
  SEARCH=(grep -RInE 'gophish|awarecheck|awarechck')
fi

if "${SEARCH[@]}" \
  "$REPO_ROOT/static" \
  "$REPO_ROOT/templates/insec" \
  "$REPO_ROOT/web/src" \
  "$REPO_ROOT/web/public" \
  "$REPO_ROOT/web/index.html" \
  "$REPO_ROOT/web/README.md"; then
  echo "Legacy browser branding found." >&2
  exit 1
fi

echo "Browser branding audit passed: AwareNow only."

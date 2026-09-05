#!/usr/bin/env bash
# Fail if first-party browser-served assets contain legacy product branding.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PATTERN='gophish|awarecheck|awarechck'
found=0

if command -v rg >/dev/null 2>&1; then
  if rg -n -i "$PATTERN" \
    "$REPO_ROOT/static" \
    "$REPO_ROOT/web/src" \
    "$REPO_ROOT/web/public" \
    "$REPO_ROOT/web/index.html" \
    "$REPO_ROOT/web/README.md"; then
    found=1
  fi

  # Scan all first-party server-rendered HTML, excluding vendor packs. The
  # legacy asset filenames below are upstream compatibility paths, not UI text.
  # NOTE: the vendor exclude glob must be rooted with '**/' -- ripgrep anchors
  # an unrooted 'vendor/**' glob against the full path it was invoked with
  # (which already includes the "templates/" segment), so it never matches.
  if rg -n -i "$PATTERN" "$REPO_ROOT/templates" -g '*.html' -g '!**/vendor/**' |
    rg -v 'css/dist/gophish\.css|js/dist/app/gophish\.min\.js'; then
    found=1
  fi
else
  if grep -RInE "$PATTERN" \
    "$REPO_ROOT/static" \
    "$REPO_ROOT/web/src" \
    "$REPO_ROOT/web/public" \
    "$REPO_ROOT/web/index.html" \
    "$REPO_ROOT/web/README.md"; then
    found=1
  fi
  if grep -RInE "$PATTERN" "$REPO_ROOT/templates" \
    --include='*.html' --exclude-dir=vendor |
    grep -Ev '/css/dist/gophish\.css|/js/dist/app/gophish\.min\.js'; then
    found=1
  fi
fi

if [[ "$found" -ne 0 ]]; then
  echo "Legacy browser branding found." >&2
  exit 1
fi

echo "Browser branding audit passed: AwareNow only."

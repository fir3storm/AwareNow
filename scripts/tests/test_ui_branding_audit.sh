#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
AUDIT="$REPO_ROOT/scripts/check-ui-branding.sh"

grep -F '"$REPO_ROOT/templates"' "$AUDIT"
grep -F 'vendor' "$AUDIT"
grep -F 'awarenow-editor' "$REPO_ROOT/templates/templates.html"
if grep -F 'gophish-editor' "$REPO_ROOT/templates/templates.html"; then
  echo "templates.html still references the legacy editor class." >&2
  exit 1
fi

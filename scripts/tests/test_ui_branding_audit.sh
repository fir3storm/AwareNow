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

# End-to-end: actually execute the audit script against the real repo tree
# and confirm it passes. This is the regression test for the vendor-glob
# bug where an unrooted 'vendor/**' rg glob silently failed to exclude
# templates/vendor/**, letting upstream-attribution content (which legitimately
# contains "GoPhish") leak into the audit and produce false failures.
VENDOR_FILE="$REPO_ROOT/templates/vendor/hailbytes/landing-pages/education-notification.html"
if [ ! -f "$VENDOR_FILE" ]; then
  echo "expected vendor fixture file is missing: $VENDOR_FILE" >&2
  exit 1
fi
if ! grep -qi 'gophish' "$VENDOR_FILE"; then
  echo "vendor fixture no longer contains a 'GoPhish' reference -- test is no longer exercising the exclusion path; update the fixture path." >&2
  exit 1
fi

AUDIT_OUT="$(mktemp)"
trap 'rm -f "$AUDIT_OUT"' EXIT
if ! bash "$AUDIT" >"$AUDIT_OUT" 2>&1; then
  echo "check-ui-branding.sh failed on a clean tree (it should pass):" >&2
  cat "$AUDIT_OUT" >&2
  exit 1
fi

echo "ui branding audit test: OK"

# AwareNow Browser Branding Audit

**Scope:** Browser-served assets in `static/`, the first-party campaign assets
under `templates/insec/`, and the current frontend under `web/`.

The audit rejects legacy `Gophish`, `AwareCheck`, and `awarechck` strings in
those browser-facing paths. Vendor template packs are intentionally excluded:
their provenance, licensing, and upstream compatibility text must remain
unaltered. Runtime/API compatibility identifiers such as `GOPHISH_API_KEY`,
the `gophish` executable, and the systemd unit are also outside this browser
branding check.

Run from the repository root:

```bash
bash scripts/check-ui-branding.sh
```

The audit was run after the Task 3 normalization and passed with the message
`Browser branding audit passed: AwareNow only.`

#!/usr/bin/env python3
"""Keep only English email templates and landing pages.

Deletes HailBytes Spanish/Portuguese packs from this repo. With --gophish,
also removes already-imported non-English records from the live Gophish API.

  python3 scripts/keep-english-templates.py --dry-run
  python3 scripts/keep-english-templates.py
  GOPHISH_API_KEY=... python3 scripts/keep-english-templates.py --gophish
"""
from __future__ import annotations

import argparse
import json
import os
import re
import shutil
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TEMPLATES = ROOT / "templates"
HAILBYTES = TEMPLATES / "vendor" / "hailbytes"
API = os.environ.get("GOPHISH_URL", "http://127.0.0.1:3333").rstrip("/")
API_KEY = os.environ.get("GOPHISH_API_KEY", "").strip()
LANG_RE = re.compile(r"<html[^>]*\blang=['\"]([^'\"]+)['\"]", re.I)


def is_english_tag(tag: str | None) -> bool:
    if not tag:
        return True
    t = tag.strip().lower()
    return t == "en" or t.startswith("en-")


def hailbytes_non_english_dirs() -> list[Path]:
    found: list[Path] = []
    if not HAILBYTES.is_dir():
        return found
    for path in sorted(p for p in HAILBYTES.iterdir() if p.is_dir()):
        meta_path = path / "metadata.json"
        lang = None
        if meta_path.exists():
            try:
                lang = json.loads(meta_path.read_text(encoding="utf-8")).get("language")
            except json.JSONDecodeError:
                lang = None
        if not is_english_tag(lang if isinstance(lang, str) else None):
            found.append(path)
    return found


def gophish_names_for_dir(category_dir: Path) -> set[str]:
    names: set[str] = set()
    meta_path = category_dir / "metadata.json"
    if meta_path.exists():
        try:
            data = json.loads(meta_path.read_text(encoding="utf-8"))
        except json.JSONDecodeError:
            data = {}
        for item in data.get("templates") or []:
            name = item.get("name") or Path(item.get("filename") or "").stem
            if name:
                names.add(f"HailBytes - {name}"[:190])
    for html_path in category_dir.glob("*.html"):
        names.add(f"HailBytes - {html_path.stem.replace('_', ' ').title()}"[:190])
    edu = category_dir / "education"
    if edu.is_dir():
        for html_path in edu.glob("*.html"):
            names.add(f"HailBytes edu - {category_dir.name} {html_path.stem}"[:190])
    return names


def api(method: str, path: str, payload: dict | None = None):
    data = None if payload is None else json.dumps(payload).encode("utf-8")
    req = urllib.request.Request(
        f"{API}{path}",
        data=data,
        method=method,
        headers={
            "Authorization": f"Bearer {API_KEY}",
            "Content-Type": "application/json",
        },
    )
    opener = urllib.request.build_opener(urllib.request.HTTPHandler)
    try:
        with opener.open(req, timeout=60) as resp:
            raw = resp.read().decode("utf-8")
            return json.loads(raw) if raw else None
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {path} -> HTTP {exc.code}: {body}") from exc


def delete_gophish(drop_names: set[str], dry_run: bool) -> int:
    removed = 0
    for kind, list_path, del_path in (
        ("email", "/api/templates/", "/api/templates/"),
        ("page", "/api/pages/", "/api/pages/"),
    ):
        records = api("GET", list_path) or []
        for rec in records:
            name = rec.get("name") or ""
            html = rec.get("html") or ""
            lang_m = LANG_RE.search(html)
            lang = lang_m.group(1) if lang_m else None
            by_name = name in drop_names or "latam-spanish" in name or "latam-portuguese" in name
            by_lang = not is_english_tag(lang)
            if not by_name and not by_lang:
                continue
            rid = rec.get("id")
            print(f"{'would delete' if dry_run else 'delete'} {kind}  {name}")
            if dry_run:
                removed += 1
                continue
            try:
                api("DELETE", f"{del_path}{rid}")
            except RuntimeError as exc:
                print(f"FAIL {kind}  {name}: {exc}", file=sys.stderr)
                continue
            removed += 1
    return removed


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Print what would be removed, do not delete",
    )
    parser.add_argument(
        "--gophish",
        action="store_true",
        help="Also delete non-English emails/pages already imported into Gophish",
    )
    args = parser.parse_args()

    dirs = hailbytes_non_english_dirs()
    drop_names: set[str] = set()
    for path in dirs:
        drop_names |= gophish_names_for_dir(path)

    if not dirs:
        print("No non-English HailBytes packs left on disk.")
    for path in dirs:
        rel = path.relative_to(ROOT)
        print(f"{'would remove' if args.dry_run else 'remove'} {rel}/")
        if not args.dry_run:
            shutil.rmtree(path)

    api_removed = 0
    if args.gophish:
        if not API_KEY:
            print(
                "Set GOPHISH_API_KEY to clean the live Gophish lists.",
                file=sys.stderr,
            )
            return 1
        api_removed = delete_gophish(drop_names, args.dry_run)

    print(
        f"\nDone. dirs={len(dirs)} gophish_records={api_removed}"
        + (" (dry-run)" if args.dry_run else "")
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

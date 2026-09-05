#!/usr/bin/env python3
"""Import email templates and landing pages into a local AwareNow instance.

Usage on the VPS (as root):

  export GOPHISH_API_KEY=$(sqlite3 /opt/awarenow/runtime/gophish.db "SELECT api_key FROM users WHERE username='admin';")
  python3 /opt/awarenow/scripts/import-templates.py
"""
from __future__ import annotations

import json
import os
import re
import sys
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
TEMPLATES = ROOT / "templates"
API = os.environ.get("GOPHISH_URL", "http://127.0.0.1:3333").rstrip("/")
API_KEY = os.environ.get("GOPHISH_API_KEY", "").strip()
REDIRECT = os.environ.get(
    "GOPHISH_REDIRECT_URL", "https://itsupport.insec.in/got-phished"
)


def die(msg: str, code: int = 1) -> None:
    print(msg, file=sys.stderr)
    sys.exit(code)


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
    # The compatible API uses StrictSlash; do not follow redirects (urllib would turn POST into GET).
    opener = urllib.request.build_opener(urllib.request.HTTPHandler)
    try:
        with opener.open(req, timeout=60) as resp:
            status = getattr(resp, "status", 200)
            raw = resp.read().decode("utf-8")
            body = json.loads(raw) if raw else None
    except urllib.error.HTTPError as exc:
        body = exc.read().decode("utf-8", errors="replace")
        raise RuntimeError(f"{method} {path} -> HTTP {exc.code}: {body}") from exc
    if method == "POST":
        if not isinstance(body, dict) or not body.get("id"):
            raise RuntimeError(f"{method} {path} did not create a record: {body!r}")
        if status not in (200, 201):
            raise RuntimeError(f"{method} {path} -> HTTP {status}: {body!r}")
    return body


def load_json(path: Path) -> dict:
    if path.exists():
        return json.loads(path.read_text(encoding="utf-8"))
    return {}


def title_from_html(html: str, fallback: str) -> str:
    m = re.search(r"<title[^>]*>(.*?)</title>", html, re.I | re.S)
    if m:
        t = re.sub(r"\s+", " ", m.group(1)).strip()
        if t:
            return t[:120]
    return fallback.replace("-", " ").replace("_", " ")[:120]


GOPHISH_FIELDS = {
    "From",
    "URL",
    "Tracker",
    "TrackingURL",
    "RId",
    "BaseURL",
    "Email",
    "FirstName",
    "LastName",
    "Position",
}


def sanitize_gophish_html(html: str) -> str:
    """Drop HailBytes/etc variables Gophish 0.12 cannot execute (e.g. {{.CampaignType}})."""

    def repl(match: re.Match[str]) -> str:
        inner = match.group(1)
        names = re.findall(r"\.(\w+)", inner)
        if names and any(n not in GOPHISH_FIELDS for n in names):
            return ""
        return match.group(0)

    return re.sub(r"\{\{(.*?)\}\}", repl, html, flags=re.S)


def ensure_tracker(html: str) -> str:
    if "{{.Tracker}}" in html:
        return html
    if re.search(r"</body>", html, re.I):
        return re.sub(r"</body>", "{{.Tracker}}\n</body>", html, count=1, flags=re.I)
    return html + "\n{{.Tracker}}\n"


def looks_like_login(html: str) -> bool:
    h = html.lower()
    return "<form" in h and ("password" in h or 'type="password"' in h)


class Importer:
    def __init__(self) -> None:
        templates = api("GET", "/api/templates/") or []
        pages = api("GET", "/api/pages/") or []
        self.email_names = {t.get("name") for t in templates}
        self.page_names = {p.get("name") for p in pages}
        self.created_email = 0
        self.created_page = 0
        self.skipped = 0
        self.failed = 0

    def add_email(self, name: str, subject: str, html: str) -> None:
        name = name[:190]
        if name in self.email_names:
            print(f"skip email  {name}")
            self.skipped += 1
            return
        html = ensure_tracker(sanitize_gophish_html(html))
        try:
            api(
                "POST",
                "/api/templates/",
                {"name": name, "subject": subject[:200], "text": "", "html": html},
            )
        except RuntimeError as exc:
            self.failed += 1
            print(f"FAIL email  {name}: {exc}")
            return
        self.email_names.add(name)
        self.created_email += 1
        print(f"email       {name}")

    def add_page(self, name: str, html: str, capture: bool, passwords: bool, redirect: str) -> None:
        name = name[:190]
        if name in self.page_names:
            print(f"skip page   {name}")
            self.skipped += 1
            return
        html = sanitize_gophish_html(html)
        try:
            api(
                "POST",
                "/api/pages/",
                {
                    "name": name,
                    "html": html,
                    "capture_credentials": capture,
                    "capture_passwords": passwords,
                    "redirect_url": redirect or "",
                },
            )
        except RuntimeError as exc:
            self.failed += 1
            print(f"FAIL page   {name}: {exc}")
            return
        self.page_names.add(name)
        self.created_page += 1
        print(f"page        {name}")


def import_insec(imp: Importer) -> None:
    for html_path in sorted((TEMPLATES / "insec" / "emails").glob("*.html")):
        meta = load_json(html_path.with_suffix(".json"))
        html = html_path.read_text(encoding="utf-8")
        imp.add_email(
            meta.get("name") or f"INSEC - {html_path.stem}",
            meta.get("subject") or title_from_html(html, html_path.stem),
            html,
        )
    for html_path in sorted((TEMPLATES / "insec" / "landing").glob("*.html")):
        meta = load_json(html_path.with_suffix(".json"))
        html = html_path.read_text(encoding="utf-8")
        capture = bool(meta.get("capture_credentials", looks_like_login(html)))
        imp.add_page(
            meta.get("name") or f"INSEC - {html_path.stem}",
            html,
            capture,
            bool(meta.get("capture_passwords", capture)),
            meta.get("redirect_url") or (REDIRECT if capture else ""),
        )


def hailbytes_subjects(category_dir: Path) -> dict[str, tuple[str, str]]:
    """filename -> (name, subject)"""
    meta_path = category_dir / "metadata.json"
    out: dict[str, tuple[str, str]] = {}
    if not meta_path.exists():
        return out
    try:
        data = json.loads(meta_path.read_text(encoding="utf-8"))
    except json.JSONDecodeError:
        return out
    for item in data.get("templates") or []:
        fn = item.get("filename")
        if not fn:
            continue
        subjects = item.get("suggested_subject_lines") or []
        out[fn] = (
            item.get("name") or Path(fn).stem,
            subjects[0] if subjects else item.get("name") or Path(fn).stem,
        )
    return out


SKIP_DIRS = {
    ".github",
    "docs",
    "tools",
    "tests",
    "campaign-guides",
    "ai-tools",
    "landing-pages",
}


def import_hailbytes(imp: Importer) -> None:
    root = TEMPLATES / "vendor" / "hailbytes"
    if not root.exists():
        return
    lp = root / "landing-pages"
    if lp.exists():
        for html_path in sorted(lp.glob("*.html")):
            html = html_path.read_text(encoding="utf-8", errors="replace")
            capture = looks_like_login(html)
            imp.add_page(
                f"HailBytes - {html_path.stem}",
                html,
                capture,
                capture,
                REDIRECT if capture else "",
            )
    for category in sorted(p for p in root.iterdir() if p.is_dir() and p.name not in SKIP_DIRS):
        meta_path = category / "metadata.json"
        if meta_path.exists():
            try:
                lang = json.loads(meta_path.read_text(encoding="utf-8")).get("language") or "en"
            except json.JSONDecodeError:
                lang = "en"
            lang = str(lang).strip().lower()
            if lang and lang != "en" and not lang.startswith("en-"):
                print(f"skip pack   HailBytes {category.name} ({lang})")
                continue
        mapping = hailbytes_subjects(category)
        for html_path in sorted(category.glob("*.html")):
            html = html_path.read_text(encoding="utf-8", errors="replace")
            name, subject = mapping.get(
                html_path.name,
                (html_path.stem.replace("_", " ").title(), title_from_html(html, html_path.stem)),
            )
            imp.add_email(f"HailBytes - {name}", subject, html)
        edu = category / "education"
        if edu.is_dir():
            for html_path in sorted(edu.glob("*.html")):
                html = html_path.read_text(encoding="utf-8", errors="replace")
                imp.add_page(
                    f"HailBytes edu - {category.name} {html_path.stem}",
                    html,
                    False,
                    False,
                    "",
                )


def import_linksec(imp: Importer) -> None:
    root = TEMPLATES / "vendor" / "linksec" / "emails"
    if not root.exists():
        return
    for html_path in sorted(root.rglob("*.html")):
        html = html_path.read_text(encoding="utf-8", errors="replace")
        label = html_path.stem.replace("-modified", "").replace("-", " ")
        imp.add_email(f"LinkSec - {label}", title_from_html(html, label), html)


def import_piyush(imp: Importer) -> None:
    root = TEMPLATES / "vendor" / "piyush27pawar"
    if not root.exists():
        return
    for html_path in sorted((root / "Email_Templates").glob("*.html")):
        html = html_path.read_text(encoding="utf-8", errors="replace")
        imp.add_email(
            f"Piyush - {html_path.stem.replace('_', ' ')}",
            title_from_html(html, html_path.stem),
            html,
        )
    for html_path in sorted((root / "Landing_Pages").glob("*.html")):
        html = html_path.read_text(encoding="utf-8", errors="replace")
        capture = looks_like_login(html)
        imp.add_page(
            f"Piyush - {html_path.stem.replace('_', ' ')}",
            html,
            capture,
            capture,
            REDIRECT if capture else "",
        )


def main() -> None:
    if not API_KEY:
        die(
            "Set GOPHISH_API_KEY for the same user you log into in the UI (usually admin).\n"
            "  sqlite3 /opt/awarenow/runtime/gophish.db \"SELECT username, api_key FROM users;\"\n"
            "Or copy the key from AwareNow → Account Settings."
        )
    print(f"API {API}")
    imp = Importer()
    print(
        f"this API user already has emails={len(imp.email_names)} pages={len(imp.page_names)}"
    )
    import_insec(imp)
    import_hailbytes(imp)
    import_linksec(imp)
    import_piyush(imp)
    print(
        f"\nDone. created emails={imp.created_email} pages={imp.created_page} "
        f"skipped={imp.skipped} failed={imp.failed}"
    )


if __name__ == "__main__":
    main()

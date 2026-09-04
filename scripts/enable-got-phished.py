#!/usr/bin/env python3
"""Insert /got-phished into live nginx for itsupport.insec.in only.

Does not rewrite the whole site file, so certbot TLS stays intact.
Run as root via scripts/enable-got-phished.sh
"""
from __future__ import annotations

import os
import re
import sys

PHISH_HOST = os.environ.get("GOPHISH_PHISH_HOST", "itsupport.insec.in")
HTML_PATH = os.environ.get(
    "GOPHISH_GOT_PHISHED_HTML",
    "/opt/gophish/templates/insec/static/got-phished.html",
)
SNIPPET_PATH = "/etc/nginx/snippets/gophish-got-phished.conf"
INCLUDE = f"    include {SNIPPET_PATH};\n"
NGINX_ROOTS = (
    "/etc/nginx/sites-available",
    "/etc/nginx/sites-enabled",
    "/etc/nginx/conf.d",
)

SNIPPET = f"""location = /got-phished {{
    default_type text/html;
    alias {HTML_PATH};
}}
"""


def server_names(block: str) -> list[str]:
    names: list[str] = []
    for match in re.finditer(r"server_name\s+([^;]+);", block):
        names.extend(match.group(1).split())
    return names


def find_server_blocks(text: str) -> list[tuple[int, int]]:
    spans: list[tuple[int, int]] = []
    i = 0
    while True:
        m = re.search(r"\bserver\s*\{", text[i:])
        if not m:
            break
        start = i + m.start()
        brace_at = i + m.end() - 1
        depth = 0
        end = None
        for j in range(brace_at, len(text)):
            if text[j] == "{":
                depth += 1
            elif text[j] == "}":
                depth -= 1
                if depth == 0:
                    end = j + 1
                    break
        if end is None:
            break
        spans.append((start, end))
        i = end
    return spans


def already_patched(block: str) -> bool:
    return SNIPPET_PATH in block or "location = /got-phished" in block


def patch_block(block: str) -> str:
    if already_patched(block):
        return block
    m = re.search(r"(^[ \t]*location\s+/\s*\{)", block, re.M)
    if m:
        return block[: m.start()] + INCLUDE + block[m.start() :]
    m = re.match(r"(server\s*\{\s*\n?)", block)
    if not m:
        return block
    return block[: m.end()] + INCLUDE + block[m.end() :]


def patch_file(text: str) -> tuple[str, int]:
    spans = find_server_blocks(text)
    if not spans:
        return text, 0
    pieces: list[str] = []
    last = 0
    count = 0
    for start, end in spans:
        pieces.append(text[last:start])
        block = text[start:end]
        if PHISH_HOST in server_names(block):
            new = patch_block(block)
            if new != block:
                count += 1
            pieces.append(new)
        else:
            pieces.append(block)
        last = end
    pieces.append(text[last:])
    return "".join(pieces), count


def nginx_files() -> list[str]:
    found: list[str] = []
    seen: set[str] = set()
    for root in NGINX_ROOTS:
        if not os.path.isdir(root):
            continue
        for name in os.listdir(root):
            path = os.path.join(root, name)
            if not os.path.isfile(path):
                continue
            real = os.path.realpath(path)
            if real in seen:
                continue
            seen.add(real)
            found.append(real)
    return found


def main() -> int:
    if hasattr(os, "geteuid") and os.geteuid() != 0:
        print("Run as root.", file=sys.stderr)
        return 1
    if not os.path.isfile(HTML_PATH):
        print(f"Missing {HTML_PATH}. git pull the gophish repo first.", file=sys.stderr)
        return 1

    os.makedirs(os.path.dirname(SNIPPET_PATH), exist_ok=True)
    with open(SNIPPET_PATH, "w", encoding="utf-8") as f:
        f.write(SNIPPET)

    patched_files: list[str] = []
    for path in nginx_files():
        with open(path, encoding="utf-8") as f:
            original = f.read()
        if PHISH_HOST not in original:
            continue
        updated, n = patch_file(original)
        if n:
            with open(path, "w", encoding="utf-8") as f:
                f.write(updated)
            patched_files.append(f"{path} ({n} server block(s))")

    print(f"Wrote {SNIPPET_PATH}")
    if patched_files:
        print("Patched:")
        for line in patched_files:
            print(f"  {line}")
    else:
        print(
            "No new inserts (already present, or no "
            f"{PHISH_HOST} server_name)."
        )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

"""Regression tests for the HailBytes English-only cleanup script."""

from __future__ import annotations

import importlib.util
import json
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from tempfile import TemporaryDirectory
from unittest.mock import patch


SCRIPT = Path(__file__).parents[1] / "keep-english-templates.py"
SPEC = importlib.util.spec_from_file_location("keep_english_templates", SCRIPT)
assert SPEC and SPEC.loader
cleanup = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(cleanup)


class GophishServer:
    def __init__(self, templates: list[dict], pages: list[dict]):
        self.records = {"templates": templates, "pages": pages}
        server_state = self.records

        class Handler(BaseHTTPRequestHandler):
            def do_GET(self):
                kind = self.path.rstrip("/").rsplit("/", 1)[-1]
                if kind not in server_state:
                    self.send_error(404)
                    return
                self._send_json(server_state[kind])

            def do_DELETE(self):
                parts = self.path.rstrip("/").rsplit("/", 2)
                kind, record_id = parts[-2:]
                server_state[kind][:] = [
                    record for record in server_state[kind]
                    if str(record["id"]) != record_id
                ]
                self._send_json({})

            def _send_json(self, payload):
                body = json.dumps(payload).encode("utf-8")
                self.send_response(200)
                self.send_header("Content-Type", "application/json")
                self.send_header("Content-Length", str(len(body)))
                self.end_headers()
                self.wfile.write(body)

            def log_message(self, format, *args):
                pass

        self.server = ThreadingHTTPServer(("127.0.0.1", 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever)

    @property
    def url(self) -> str:
        host, port = self.server.server_address
        return f"http://{host}:{port}"

    def __enter__(self):
        self.thread.start()
        return self

    def __exit__(self, *args):
        self.server.shutdown()
        self.thread.join()
        self.server.server_close()


class KeepEnglishTemplatesTests(unittest.TestCase):
    def test_gophish_retains_manually_created_non_english_record(self):
        """Removing language matching would delete an unrelated user record."""
        manual_record = {
            "id": 41,
            "name": "Customer Spanish welcome email",
            "html": "<html lang='es'><body>Bienvenido</body></html>",
        }
        with TemporaryDirectory() as temp_dir, GophishServer([manual_record], []) as server:
            missing_hailbytes = Path(temp_dir) / "hailbytes"
            with (
                patch.object(cleanup, "API", server.url),
                patch.object(cleanup, "API_KEY", "test-key"),
                patch.object(cleanup, "HAILBYTES", missing_hailbytes),
                patch.object(sys, "argv", [str(SCRIPT), "--gophish"]),
            ):
                self.assertEqual(cleanup.main(), 0)

            self.assertEqual(server.records["templates"], [manual_record])

    def test_gophish_requires_api_key_before_removing_local_packs(self):
        """Moving validation below deletion would destroy packs on key mistakes."""
        with TemporaryDirectory() as temp_dir:
            root = Path(temp_dir)
            hailbytes = root / "templates" / "vendor" / "hailbytes"
            retired_pack = hailbytes / "latam-spanish"
            retired_pack.mkdir(parents=True)
            (retired_pack / "metadata.json").write_text(
                json.dumps({"language": "es"}), encoding="utf-8"
            )

            with (
                patch.object(cleanup, "ROOT", root),
                patch.object(cleanup, "TEMPLATES", root / "templates"),
                patch.object(cleanup, "HAILBYTES", hailbytes),
                patch.object(cleanup, "API_KEY", ""),
                patch.object(sys, "argv", [str(SCRIPT), "--gophish"]),
            ):
                self.assertEqual(cleanup.main(), 1)

            self.assertTrue(retired_pack.is_dir())


if __name__ == "__main__":
    unittest.main()

#!/usr/bin/env python3
# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

"""Serve the ABFS data overview with live license and uploader-config status."""

import base64
import html
import json
import os
import ssl
import urllib.error
import urllib.parse
import urllib.request
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path

API = "https://kubernetes.default.svc"
TOKEN_PATH = Path("/var/run/secrets/kubernetes.io/serviceaccount/token")
CA_PATH = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
PAGE_PATH = Path("/content/index.html")
PLACEHOLDER = "<!--ABFS-STATUS-->"
LICENSE_NS = os.environ.get("LICENSE_NAMESPACE", "horizon")
LICENSE_SECRET = os.environ.get("LICENSE_SECRET", "abfs-license")
UPLOADER_NS = os.environ.get("UPLOADER_CONFIG_NAMESPACE", LICENSE_NS)
UPLOADER_CONFIGMAP = os.environ.get("UPLOADER_CONFIGMAP", "abfs-uploader-config")
CUSTOM_ROW = "AndroidCustom"


def api_get(path):
    """GET one Kubernetes API path and return its JSON, or None on any failure."""
    try:
        token = TOKEN_PATH.read_text().strip()
    except OSError:
        return None
    request = urllib.request.Request(
        API + path,
        headers={"Authorization": f"Bearer {token}"},
    )
    try:
        with urllib.request.urlopen(
            request, context=ssl.create_default_context(cafile=CA_PATH), timeout=2
        ) as response:
            return json.load(response)
    except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, ValueError):
        return None


def read_core_object(namespace, kind, name):
    """Read one core v1 object (Secret or ConfigMap) by namespace, kind, and name."""
    return api_get(
        f"/api/v1/namespaces/{urllib.parse.quote(namespace)}/"
        f"{kind}/{urllib.parse.quote(name)}"
    )


def unquote(value):
    """Strip surrounding single or double quotes from a YAML scalar."""
    value = value.strip()
    if len(value) >= 2 and value[0] == value[-1] and value[0] in ('"', "'"):
        return value[1:-1]
    return value


def b64url(data):
    """Decode a base64url string, adding padding when the source omitted it."""
    pad = "=" * (-len(data) % 4)
    return base64.urlsafe_b64decode(data + pad)


def load_claims(raw):
    """Return license claims from a JWT, JSON object, or base64 JSON blob."""
    raw = raw.strip()
    if not raw:
        return None
    if raw.count(".") == 2 and not raw.startswith("{"):
        try:
            parsed = json.loads(b64url(raw.split(".")[1]))
        except (ValueError, json.JSONDecodeError):
            return None
        return parsed if isinstance(parsed, dict) else None
    for decoder in (json.loads, lambda text: json.loads(b64url(text))):
        try:
            parsed = decoder(raw)
        except (ValueError, json.JSONDecodeError):
            continue
        if isinstance(parsed, dict):
            return parsed
    return None


def parse_expiry(value):
    """Turn an expiry claim (unix time or ISO date) into a UTC datetime."""
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, (int, float)) or (
        isinstance(value, str) and value.strip().lstrip("-").isdigit()
    ):
        try:
            number = float(value)
            if number > 1e12:
                number = number / 1000.0
            return datetime.fromtimestamp(number, tz=timezone.utc)
        except (TypeError, ValueError, OverflowError):
            return None
    if isinstance(value, str):
        text = value.strip()
        if text.endswith("Z"):
            text = text[:-1] + "+00:00"
        try:
            parsed = datetime.fromisoformat(text)
        except (TypeError, ValueError, OverflowError):
            return None
        if parsed.tzinfo is None:
            parsed = parsed.replace(tzinfo=timezone.utc)
        return parsed
    return None


def license_payload():
    """Return (state, text). state is Present, Empty, or Unknown. text is the license text."""
    try:
        token = TOKEN_PATH.read_text().strip()
    except OSError:
        return "Unknown", ""
    path = (
        f"/api/v1/namespaces/{urllib.parse.quote(LICENSE_NS)}/"
        f"secrets/{urllib.parse.quote(LICENSE_SECRET)}"
    )
    request = urllib.request.Request(
        API + path,
        headers={"Authorization": f"Bearer {token}"},
    )
    try:
        with urllib.request.urlopen(
            request, context=ssl.create_default_context(cafile=CA_PATH), timeout=2
        ) as response:
            obj = json.load(response)
    except urllib.error.HTTPError as error:
        missing = error.code == 404
        error.close()
        return ("Empty", "") if missing else ("Unknown", "")
    except (urllib.error.URLError, TimeoutError, ValueError):
        return "Unknown", ""
    encoded = (obj.get("data") or {}).get("license", "")
    if not isinstance(encoded, str) or not encoded.strip():
        return "Empty", ""
    try:
        text = base64.b64decode(encoded).decode("utf-8", errors="replace").strip()
    except (ValueError, TypeError):
        return "Unknown", ""
    if not text:
        return "Empty", ""
    return "Present", text


def license_fields(state, payload):
    """Return Secret Present, Empty, or Unknown, and the Valid Until date for the status card."""
    if state == "Unknown":
        return "Unknown", "Unknown"
    if state == "Empty" or not payload:
        return "Empty", "—"
    claims = load_claims(payload)
    expiry = None
    if isinstance(claims, dict):
        expiry = parse_expiry(claims.get("expiry_threshold", claims.get("expiryThreshold")))
    valid_until = expiry.astimezone(timezone.utc).strftime("%Y-%m-%d") if expiry else "Unknown"
    return "Present", valid_until


def table_blocks(text):
    """Split the uploader table YAML into one block per '- name:' row."""
    blocks = []
    current = []
    for line in text.splitlines():
        if line.strip().startswith("- name:") and current:
            blocks.append(current)
            current = [line]
        elif line.strip() or current:
            current.append(line)
    if current:
        blocks.append(current)
    return blocks


def block_field(block, key):
    """Read the first scalar for key from one table row, or '' when it is absent."""
    prefix = key + ":"
    for line in block:
        stripped = line.strip()
        if stripped.startswith("- " + prefix) or stripped.startswith(prefix):
            return unquote(stripped.split(":", 1)[1])
    return ""


def listing_without_flags(block):
    """Return the row text without the active and synced flags."""
    kept = []
    for line in block:
        stripped = line.strip()
        if stripped.startswith("active:") or stripped.startswith("synced:"):
            continue
        kept.append(line.rstrip())
    return "\n".join(kept).strip()


def pusher_config():
    """Active uploader-config row. AndroidCustom is custom; any other row is predefined."""
    obj = read_core_object(UPLOADER_NS, "configmaps", UPLOADER_CONFIGMAP)
    text = ((obj or {}).get("data") or {}).get("table", "")
    if not text.strip():
        return "Not Rendered", "Not Rendered"
    for block in table_blocks(text):
        if block_field(block, "active").lower() != "true":
            continue
        name = block_field(block, "name")
        kind = "Custom" if name == CUSTOM_ROW else "Predefined"
        return kind, listing_without_flags(block) or "Not Rendered"
    return "Not Rendered", "Not Rendered"


def license_status():
    """JSON fields for Secret present/empty and Valid Until. No license payload."""
    secret, valid_until = license_fields(*license_payload())
    return {"secret": secret, "validUntil": valid_until}


def build_status():
    """Build the Status card HTML from the current license and pusher config."""
    secret, valid_until = license_fields(*license_payload())
    config_type, config_content = pusher_config()
    timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")
    return f"""
        <article class="card">
          <h2>Status</h2>
          <p class="status-meta">Cluster status read at {html.escape(timestamp)}. Refresh the portal page to update.</p>
          <h3>ABFS License</h3>
          <ul>
            <li><strong>Secret:</strong> {html.escape(secret)}</li>
            <li><strong>Valid Until:</strong> {html.escape(valid_until)}</li>
          </ul>
          <h3>ABFS Pusher Config</h3>
          <ul>
            <li><strong>Config Type:</strong> {html.escape(config_type)}</li>
            <li><strong>Config Content:</strong></li>
          </ul>
          <pre>{html.escape(config_content)}</pre>
        </article>
    """


def request_path(handler):
    """Return the request path without the query string."""
    return urllib.parse.urlparse(handler.path).path


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        """Serve /health, /license-status, or the overview page with a fresh status card."""
        path = request_path(self)
        if path == "/health":
            body = b"ok\n"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if path == "/license-status":
            body = json.dumps(license_status()).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json; charset=utf-8")
            self.send_header("Cache-Control", "no-store")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            return
        if path not in ("/", "/index.html"):
            self.send_error(404)
            return
        page = PAGE_PATH.read_text()
        body = page.replace(PLACEHOLDER, build_status()).encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/html; charset=utf-8")
        self.send_header("Cache-Control", "no-store")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, format_string, *args):
        """Print each request to stdout so the pod log shows it immediately."""
        print(f"{self.address_string()} - {format_string % args}", flush=True)


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()

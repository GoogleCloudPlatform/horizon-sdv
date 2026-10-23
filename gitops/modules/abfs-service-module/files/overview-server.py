#!/usr/bin/env python3
# Copyright (c) 2026 Accenture, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Serve the ABFS overview with current, non-sensitive Kubernetes status.

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
ABFS_NS = os.environ.get("ABFS_NAMESPACE", "abfs")
DATA_OVERVIEW_URL = os.environ.get("DATA_OVERVIEW_URL", "")
# abfs-ui is disabled until it is functional. Skip its status rows.
UI_ENABLED = os.environ.get("UI_ENABLED", "false").lower() == "true"


def api_get(path):
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


def namespaced(api_path, namespace, kind, name):
    return api_get(
        f"/apis/{api_path}/namespaces/{urllib.parse.quote(namespace)}/"
        f"{kind}/{urllib.parse.quote(name)}"
    )


def core(namespace, kind, name):
    return api_get(
        f"/api/v1/namespaces/{urllib.parse.quote(namespace)}/"
        f"{kind}/{urllib.parse.quote(name)}"
    )


def deployment_status(name):
    obj = namespaced("apps/v1", ABFS_NS, "deployments", name)
    if not obj:
        return "Not Rendered"
    status = obj.get("status", {})
    desired = obj.get("spec", {}).get("replicas", 0)
    return f"Available {status.get('availableReplicas', 0)}/{desired}"


def statefulset_status(name):
    obj = namespaced("apps/v1", ABFS_NS, "statefulsets", name)
    if not obj:
        return "Not Rendered"
    status = obj.get("status", {})
    desired = obj.get("spec", {}).get("replicas", 0)
    return f"Ready {status.get('readyReplicas', 0)}/{desired}"


def daemonset_status(name):
    obj = namespaced("apps/v1", ABFS_NS, "daemonsets", name)
    if not obj:
        return "Not Rendered"
    status = obj.get("status", {})
    return (
        f"Ready {status.get('numberReady', 0)}/"
        f"{status.get('desiredNumberScheduled', 0)}"
    )


def pod_lines(component):
    """One status line per pod for a component, sorted by name."""
    query = urllib.parse.urlencode(
        {"labelSelector": f"app.kubernetes.io/component={component}"}
    )
    result = api_get(f"/api/v1/namespaces/{ABFS_NS}/pods?{query}") or {}
    lines = []
    for pod in result.get("items", []):
        name = pod.get("metadata", {}).get("name", "Unknown")
        phase = pod.get("status", {}).get("phase", "Unknown")
        reasons = []
        restarts = 0
        for container in pod.get("status", {}).get("containerStatuses", []):
            restarts += container.get("restartCount", 0)
            state = container.get("state", {})
            last_state = container.get("lastState", {})
            reason = (
                state.get("waiting", {}).get("reason")
                or state.get("terminated", {}).get("reason")
                or last_state.get("terminated", {}).get("reason")
            )
            if reason:
                reasons.append(reason)
        suffix = f", restarts={restarts}"
        if reasons:
            suffix += f", reason={','.join(sorted(set(reasons)))}"
        lines.append(f"{name}: {phase}{suffix}")
    return sorted(lines)


def pod_list(lines):
    """Nested list of pod lines, or a single 'none' item when no pods exist."""
    shown = lines or ["None"]
    items = "\n".join(f"<li>{html.escape(line)}</li>" for line in shown)
    return f"<ul>\n{items}\n</ul>"


def job_finished(status, condition_type):
    """True when the Job condition of this type is set, which means the Job has stopped."""
    for condition in status.get("conditions") or []:
        if condition.get("type") == condition_type and condition.get("status") == "True":
            return True
    return False


def job_status(name):
    """Completed on success. Running while a pod is active. Failed only after the Job stops without success."""
    obj = namespaced("batch/v1", ABFS_NS, "jobs", name)
    if not obj:
        return "Not Rendered"
    status = obj.get("status", {})
    if status.get("succeeded", 0) or job_finished(status, "Complete"):
        return "Completed"
    if status.get("active", 0) or status.get("failed", 0):
        if not job_finished(status, "Failed"):
            return "Running"
    if job_finished(status, "Failed"):
        return "Failed"
    return "Pending"


def license_fields():
    """Read Secret present/empty and Valid Until from the abfs-data overview."""
    try:
        with urllib.request.urlopen(DATA_OVERVIEW_URL, timeout=2) as response:
            payload = json.load(response)
    except (urllib.error.HTTPError, urllib.error.URLError, TimeoutError, ValueError):
        return "Not Rendered", "Not Rendered"
    if not isinstance(payload, dict):
        return "Not Rendered", "Not Rendered"
    secret = payload.get("secret")
    valid_until = payload.get("validUntil")
    if secret not in ("Present", "Empty", "Unknown"):
        return "Not Rendered", "Not Rendered"
    if secret == "Unknown":
        return "Unknown", "Unknown"
    if not isinstance(valid_until, str) or not valid_until:
        valid_until = "—" if secret == "Empty" else "Unknown"
    return secret, valid_until


def gce_metadata_status():
    """Applied when the metadata ConfigMap says so. Not Applied when that metadata is absent. Unknown when the API call fails."""
    try:
        token = TOKEN_PATH.read_text().strip()
    except OSError:
        return "Unknown"
    path = (
        f"/api/v1/namespaces/{urllib.parse.quote(ABFS_NS)}/"
        "configmaps/abfs-license-metadata-status"
    )
    request = urllib.request.Request(
        API + path,
        headers={"Authorization": f"Bearer {token}"},
    )
    try:
        with urllib.request.urlopen(
            request, context=ssl.create_default_context(cafile=CA_PATH), timeout=2
        ) as response:
            metadata = json.load(response)
    except urllib.error.HTTPError as error:
        if error.code == 404:
            return "Not Applied"
        return "Unknown"
    except (urllib.error.URLError, TimeoutError, ValueError):
        return "Unknown"
    status = ((metadata or {}).get("data") or {}).get("status", "")
    return "Applied" if status == "applied" else "Not Applied"


def ui_card():
    """ABFS UI card. Hidden while the UI is disabled; still carries deployment and pod status."""
    classes = "card" if UI_ENABLED else "card status-hidden"
    hidden = "" if UI_ENABLED else " hidden"
    return f"""
        <article class="{classes}"{hidden}>
          <h2>ABFS UI</h2>
          <ul>
            <li><strong>UI Deployment:</strong> {html.escape(deployment_status("abfs-ui"))}</li>
            <li><strong>UI pods:</strong>
              {pod_list(pod_lines("ui"))}
            </li>
          </ul>
        </article>
    """


def build_status():
    """Build the Status card and the hidden ABFS UI card from the current cluster."""
    secret, valid_until = license_fields()
    timestamp = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%SZ")
    return f"""
        <article class="card">
          <h2>Status</h2>
          <p class="status-meta">Cluster status read at {html.escape(timestamp)}. Refresh the portal page to update.</p>
          <h3>ABFS License</h3>
          <ul>
            <li><strong>Secret:</strong> {html.escape(secret)}</li>
            <li><strong>Valid Until:</strong> {html.escape(valid_until)}</li>
            <li><strong>GCE metadata:</strong> {html.escape(gce_metadata_status())}</li>
          </ul>
          <h3>ABFS Infrastructure</h3>
          <ul>
            <li><strong>ABFS Server:</strong><br>Deployment: {html.escape(deployment_status("abfs-server"))}</li>
            <li><strong>ABFS Uploaders:</strong><br>StatefulSet: {html.escape(statefulset_status("abfs-gerrit-uploader"))}
              {pod_list(pod_lines("uploader"))}
            </li>
            <li><strong>CASFS:</strong><br>DaemonSet: {html.escape(daemonset_status("abfs-casfs-installer"))}</li>
          </ul>
          <h3>ABFS Pusher</h3>
          <ul>
            <li><strong>Job Status:</strong> {html.escape(job_status("abfs-pusher-config"))}</li>
          </ul>
        </article>
        {ui_card()}
    """


def request_path(handler):
    return urllib.parse.urlparse(handler.path).path


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        path = request_path(self)
        if path == "/health":
            body = b"ok\n"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain; charset=utf-8")
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
        print(f"{self.address_string()} - {format_string % args}", flush=True)


if __name__ == "__main__":
    ThreadingHTTPServer(("0.0.0.0", 8080), Handler).serve_forever()

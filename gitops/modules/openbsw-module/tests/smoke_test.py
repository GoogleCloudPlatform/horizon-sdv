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

"""Smoke checks for openbsw-module.

Runs helm lint and helm template, checks the openbsw-module ModuleCatalog
entry, and checks that the child Application is labeled and finalized so
enable renders it and disable prunes it with the parent.

When the argo CLI is installed it also runs `argo lint --offline` and a
client-side `argo submit --dry-run --from workflowtemplate/...` for both
WorkflowTemplates, and simulates a webhook event for each Sensor. The dry-run
talks to a local fake Kubernetes API that serves the rendered templates, so no
cluster is needed or contacted.
"""

import copy
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile
import threading
from http.server import BaseHTTPRequestHandler, HTTPServer
from pathlib import Path

import yaml

ROOT = Path(__file__).resolve().parents[4]
CHART = ROOT / "gitops/modules/openbsw-module"
LINT_VALUES = (
    ROOT
    / ".agents/plugins/horizon-dev/skills/horizon-create-module/scripts/lint-values.yaml"
)
MODULE_MANAGER = ROOT / "gitops/apps/module-manager"
CHILD_FINALIZER = "resources-finalizer.argocd.argoproj.io"


def fail(message):
    print("FAIL:", message, file=sys.stderr)
    sys.exit(1)


def run(cmd):
    result = subprocess.run(cmd, cwd=ROOT, text=True, capture_output=True)
    if result.returncode != 0:
        fail(
            "$ "
            + " ".join(cmd)
            + "\n"
            + result.stdout
            + result.stderr
        )
    return result.stdout


def docs(text):
    return [doc for doc in yaml.safe_load_all(text) if doc]


def lint():
    run(["helm", "lint", str(CHART), "-f", str(LINT_VALUES)])
    run(["helm", "lint", str(CHART / "argo-workflows")])
    print("helm lint: ok")


def render(module_name, sub_environment):
    cmd = [
        "helm",
        "template",
        "openbsw-module",
        str(CHART),
        "-f",
        str(LINT_VALUES),
        "--set",
        f"moduleName={module_name}",
        "--set",
        f"config.isSubEnvironment={str(sub_environment).lower()}",
    ]
    return docs(run(cmd))


def by_kind(rendered, kind):
    return [doc for doc in rendered if doc.get("kind") == kind]


def check_render():
    rendered = render("openbsw-module", False)
    kinds = sorted(doc["kind"] for doc in rendered)
    for kind in (
        "Namespace",
        "ServiceAccount",
        "DaemonSet",
        "ConfigMap",
        "Service",
        "Deployment",
        "NetworkPolicy",
        "Application",
    ):
        if kind not in kinds:
            fail(f"helm template did not render {kind}; got {kinds}")

    namespaces = {doc["metadata"]["name"]: doc for doc in by_kind(rendered, "Namespace")}
    openbsw = namespaces.get("openbsw")
    if openbsw is None or "openbsw-overview" not in namespaces:
        fail(f"expected namespaces openbsw and openbsw-overview, got {sorted(namespaces)}")
    labels = openbsw["metadata"]["labels"]
    if labels.get("pod-security.kubernetes.io/enforce") != "privileged":
        fail("openbsw namespace must enforce the privileged Pod Security profile")
    if openbsw["metadata"]["annotations"].get("horizon-sdv.io/node-pool") != "sdv-openbsw-build-node-pool":
        fail("openbsw namespace is missing the OpenBSW node pool annotation")

    accounts = by_kind(rendered, "ServiceAccount")
    if len(accounts) != 1 or accounts[0]["metadata"]["name"] != "openbsw-can-kernel":
        fail("expected ServiceAccount openbsw-can-kernel")
    if accounts[0].get("automountServiceAccountToken") is not False:
        fail("openbsw-can-kernel must not mount an API token")

    daemonsets = by_kind(rendered, "DaemonSet")
    if len(daemonsets) != 1 or daemonsets[0]["metadata"]["name"] != "openbsw-can-kernel":
        fail("expected DaemonSet openbsw-can-kernel")
    selector = daemonsets[0]["spec"]["template"]["spec"]["nodeSelector"]
    if selector.get("workloadLabel") != "openbsw" or selector.get("cloud.google.com/gke-nodepool") != "sdv-openbsw-build-node-pool":
        fail(f"CAN DaemonSet nodeSelector does not match the OpenBSW pool: {selector}")

    services = by_kind(rendered, "Service")
    if len(services) != 1:
        fail("expected one overview Service")
    service = services[0]
    if service["metadata"]["name"] != "mod-openbsw-module-overview" or service["metadata"]["namespace"] != "openbsw-overview":
        fail("overview Service name or namespace does not match the catalog")
    if service["spec"]["ports"][0]["port"] != 80:
        fail("overview Service must listen on port 80")

    html = next(
        doc["data"]["index.html"]
        for doc in by_kind(rendered, "ConfigMap")
        if doc["metadata"]["name"] == "mod-openbsw-module-overview-html"
    )
    if "<h1>openbsw-module</h1>" not in html:
        fail("overview ConfigMap does not contain the portal page")

    if render("openbsw-module", True):
        fail("a sub-environment must render no resources")
    print("helm template: ok")


def check_catalog():
    rendered = docs(run([
        "helm",
        "template",
        "module-manager",
        str(MODULE_MANAGER),
        "-s",
        "templates/module-catalog.yaml",
    ]))
    catalogs = [doc for doc in rendered if doc.get("kind") == "ModuleCatalog"]
    if len(catalogs) != 1:
        fail(f"expected one ModuleCatalog, got {len(catalogs)}")
    modules = catalogs[0].get("spec", {}).get("modules")
    if not isinstance(modules, list):
        fail("ModuleCatalog spec.modules must be a list")
    match = [entry for entry in modules if isinstance(entry, dict) and entry.get("name") == "openbsw-module"]
    if len(match) != 1:
        fail("ModuleCatalog must contain exactly one openbsw-module entry")
    entry = match[0]
    expected = {
        "path": "gitops/modules/openbsw-module",
        "overviewPath": "portal/overview.html",
        "overviewService": "mod-openbsw-module-overview",
        "overviewServiceNamespace": "openbsw-overview",
        "hardDependencies": ["workloads-common"],
    }
    for key, value in expected.items():
        if entry.get(key) != value:
            fail(f"openbsw-module {key}={entry.get(key)!r}, want {value!r}")
    print("openbsw-module catalog entry: ok")


def check_applications():
    rendered = render("openbsw-module", False)
    apps = by_kind(rendered, "Application")
    if len(apps) != 1:
        fail(f"expected one child Application, got {len(apps)}")
    app = apps[0]
    meta = app["metadata"]
    if meta["name"] != "mod-openbsw-module-argo-workflows":
        fail(f"child Application name {meta['name']!r}")
    if meta["namespace"] != "argocd":
        fail("child Application must be created in argocd")
    labels = meta.get("labels", {})
    if labels.get("horizon-sdv.io/module") != "openbsw-module":
        fail("child Application module label")
    if labels.get("horizon-sdv.io/app-role") != "child":
        fail("child Application app-role label")
    if labels.get("horizon-sdv.io/module-manager-managed") != "true":
        fail("child Application must be module-manager managed")
    if CHILD_FINALIZER not in meta.get("finalizers", []):
        fail("child Application is missing the Argo CD resources finalizer")
    automated = app["spec"]["syncPolicy"]["automated"]
    if automated.get("prune") is not True:
        fail("child Application must prune on delete")
    if app["spec"]["source"]["path"] != "gitops/modules/openbsw-module/argo-workflows":
        fail("child Application source path")
    if app["spec"]["destination"]["namespace"] != "workflows":
        fail("child Application destination namespace")
    print("enable/disable Applications: child is rendered with prune and the resources finalizer")


def check_workflows():
    rendered = docs(run([
        "helm",
        "template",
        "mod-openbsw-module-argo-workflows",
        str(CHART / "argo-workflows"),
        "--set", "moduleName=openbsw-module",
        "--set", "cloud.project=p1",
        "--set", "cloud.region=europe-west1",
        "--set", "pipelineRepo.url=https://example.com/horizon-sdv.git",
        "--set", "pipelineRepo.revision=main",
        "--set", "git.authMethod=app",
    ]))
    templates = {doc["metadata"]["name"]: doc for doc in by_kind(rendered, "WorkflowTemplate")}
    sensors = {doc["metadata"]["name"]: doc for doc in by_kind(rendered, "Sensor")}
    expected = {
        "openbsw-bsw-builder": "webhook-openbsw-bsw-builder",
        "openbsw-builder-image": "webhook-openbsw-builder-image",
    }
    if sorted(templates) != sorted(expected) or sorted(sensors) != sorted(expected.values()):
        fail(f"expected WorkflowTemplates {sorted(expected)} and Sensors {sorted(expected.values())}, got {sorted(templates)} and {sorted(sensors)}")

    for name, sensor_name in expected.items():
        template = templates[name]
        labels = template["metadata"].get("labels", {})
        if labels.get("horizon-sdv.io/expose") != "true" or labels.get("horizon-sdv.io/module") != "openbsw-module":
            fail(f"{name} must carry the expose and openbsw-module labels")
        if template["metadata"]["namespace"] != "workflows":
            fail(f"{name} must be in the workflows namespace")
        if template["spec"]["templateDefaults"]["nodeSelector"].get("workloadLabel") != "openbsw":
            fail(f"{name} pods must select the OpenBSW node pool")

        sensor = sensors[sensor_name]
        if sensor["metadata"]["namespace"] != "argo-events":
            fail(f"{sensor_name} must be in the argo-events namespace")
        trigger = sensor["spec"]["triggers"][0]["template"]["argoWorkflow"]
        workflow = trigger["source"]["resource"]
        if workflow["spec"]["workflowTemplateRef"]["name"] != name:
            fail(f"{sensor_name} must submit {name}")
        names = [p["name"] for p in workflow["spec"]["arguments"]["parameters"]]
        template_names = [p["name"] for p in template["spec"]["arguments"]["parameters"]]
        if names != template_names:
            fail(f"{sensor_name} parameters do not match {name}")
        for mapping in trigger["parameters"]:
            if not mapping["dest"].startswith("spec.arguments.parameters."):
                continue
            index = int(mapping["dest"].split(".")[3])
            key = mapping["src"]["dataKey"].removeprefix("body.")
            if names[index] != key:
                fail(f"{sensor_name} maps body.{key} to parameter {names[index]}")

    params = {p["name"]: p for p in templates["openbsw-bsw-builder"]["spec"]["arguments"]["parameters"]}
    if params["aiReviewRuntime"]["value"] != "agy":
        fail("openbsw-bsw-builder aiReviewRuntime must default to agy")
    if "ai-review" not in run(["helm", "template", str(CHART / "argo-workflows"), "--set", "cloud.project=p1"]):
        fail("openbsw-bsw-builder must render the ai-review step")
    print("WorkflowTemplates and Sensors: ok")

WORKFLOW_CHART = CHART / "argo-workflows"
COMMON_IMAGE_CHART = ROOT / "workloads/common/common_docker_image/helm"
GIT_CREDS_CHART = ROOT / "gitops/modules/workloads-common/prepare-github-app-git-creds"
TEMPLATE_KINDS = ("WorkflowTemplate", "ClusterWorkflowTemplate")


def render_workflow_resources(auth_method):
    """Render the OpenBSW chart plus the ClusterWorkflowTemplates it calls."""
    own = docs(run([
        "helm", "template", "mod-openbsw-module-argo-workflows", str(WORKFLOW_CHART),
        "--set", "moduleName=openbsw-module",
        "--set", "cloud.project=p1",
        "--set", "cloud.region=europe-west1",
        "--set", "pipelineRepo.url=https://example.com/horizon-sdv.git",
        "--set", "pipelineRepo.revision=main",
        "--set", f"git.authMethod={auth_method}",
    ]))
    common = docs(run(["helm", "template", "common", str(COMMON_IMAGE_CHART), "--set", "cloud.project=p1"]))
    creds = docs(run(["helm", "template", "creds", str(GIT_CREDS_CHART), "--set", "scm.authMethod=app"]))
    return own, [d for d in common + creds if d.get("kind") == "ClusterWorkflowTemplate"]


class FakeKubeApi(BaseHTTPRequestHandler):
    """Serves GETs for WorkflowTemplates and ClusterWorkflowTemplates only."""

    store = {}
    path_re = re.compile(r"/apis/argoproj.io/v1alpha1/(?:namespaces/[^/]+/)?(clusterworkflowtemplates|workflowtemplates)/([^/?]+)")
    kinds = {"clusterworkflowtemplates": "ClusterWorkflowTemplate", "workflowtemplates": "WorkflowTemplate"}

    def do_GET(self):
        match = self.path_re.match(self.path)
        doc = match and self.store.get((self.kinds[match.group(1)], match.group(2)))
        code, body = (200, doc) if doc else (404, {"kind": "Status", "apiVersion": "v1", "status": "Failure", "code": 404, "reason": "NotFound"})
        data = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, *args):
        pass


def argo(args, env, check=True):
    result = subprocess.run(["argo", *args], cwd=ROOT, text=True, capture_output=True, env=env, timeout=120)
    if check and result.returncode != 0:
        fail("$ argo " + " ".join(args) + "\n" + result.stdout + result.stderr)
    return result


def simulate_event(sensor, body):
    """Apply a Sensor trigger to a webhook body. Returns the Workflow, or None when the filter drops the event."""
    dependency = sensor["spec"]["dependencies"][0]
    for flt in dependency["filters"]["data"]:
        key = flt["path"].removeprefix("body.")
        if not any(re.search(pattern, str(body.get(key, ""))) for pattern in flt["value"]):
            return None
    trigger = sensor["spec"]["triggers"][0]["template"]["argoWorkflow"]
    workflow = copy.deepcopy(trigger["source"]["resource"])
    for mapping in trigger["parameters"]:
        dest = mapping["dest"]
        if not dest.startswith("spec.arguments.parameters."):
            continue
        key = mapping["src"]["dataKey"].removeprefix("body.")
        value = body.get(key, mapping["src"].get("value", ""))
        workflow["spec"]["arguments"]["parameters"][int(dest.split(".")[3])]["value"] = str(value)
    return workflow


def check_argo():
    if shutil.which("argo") is None:
        print("argo lint / dry-run submit / Sensor events: SKIPPED (argo CLI not installed)")
        return
    for auth_method in ("app", ""):
        label = f"git.authMethod={auth_method or 'none'}"
        own, cluster_templates = render_workflow_resources(auth_method)
        templates = [d for d in own if d.get("kind") in TEMPLATE_KINDS] + cluster_templates
        sensors = {d["metadata"]["name"]: d for d in by_kind(own, "Sensor")}

        with tempfile.TemporaryDirectory() as tmp:
            for doc in templates:
                (Path(tmp) / f"{doc['kind']}-{doc['metadata']['name']}.yaml").write_text(yaml.safe_dump(doc))
            kubeconfig = Path(tmp) / "kubeconfig"

            env = {**os.environ, "KUBECONFIG": "/dev/null"}
            argo(["lint", "--offline", tmp], env)

            FakeKubeApi.store = {(d["kind"], d["metadata"]["name"]): d for d in templates}
            server = HTTPServer(("127.0.0.1", 0), FakeKubeApi)
            threading.Thread(target=server.serve_forever, daemon=True).start()
            try:
                kubeconfig.write_text(yaml.safe_dump({
                    "apiVersion": "v1", "kind": "Config",
                    "clusters": [{"name": "c", "cluster": {"server": f"http://127.0.0.1:{server.server_port}"}}],
                    "users": [{"name": "u", "user": {"token": "x"}}],
                    "contexts": [{"name": "x", "context": {"cluster": "c", "user": "u"}}],
                    "current-context": "x",
                }))
                env = {**os.environ, "KUBECONFIG": str(kubeconfig)}

                for name in ("openbsw-bsw-builder", "openbsw-builder-image"):
                    argo(["submit", "--dry-run", "-n", "workflows", "--from", f"workflowtemplate/{name}", "-o", "name"], env)

                missing = argo(["submit", "--dry-run", "-n", "workflows", "--from", "workflowtemplate/no-such-template", "-o", "name"], env, check=False)
                if missing.returncode == 0:
                    fail("dry-run of an unknown WorkflowTemplate must fail")

                for sensor_name, sensor in sensors.items():
                    name = sensor_name.removeprefix("webhook-")
                    other = next(n for n in sensors if n != sensor_name).removeprefix("webhook-")
                    body = {"workflowTemplateName": name, "horizonSubmittedFrom": "cli"}
                    workflow = simulate_event(sensor, body)
                    if workflow is None:
                        fail(f"{sensor_name} must trigger on workflowTemplateName={name}")
                    for body_name in (other, name + "-extra", "x" + name, ""):
                        if simulate_event(sensor, {"workflowTemplateName": body_name}) is not None:
                            fail(f"{sensor_name} must not trigger on workflowTemplateName={body_name!r}")
                    if workflow["spec"]["workflowTemplateRef"]["name"] != name:
                        fail(f"{sensor_name} must submit {name}")
                    params = {p["name"]: p["value"] for p in workflow["spec"]["arguments"]["parameters"]}
                    if params["horizonSubmittedFrom"] != "cli":
                        fail(f"{sensor_name} did not pass body.horizonSubmittedFrom")
                    event_path = Path(tmp) / f"event-{name}.yaml"
                    workflow["metadata"].pop("namespace", None)
                    event_path.write_text(yaml.safe_dump(workflow))
                    argo(["submit", "--dry-run", "-n", "workflows", str(event_path), "-o", "name"], env)
                    for key, value in {"imageTag": "custom-tag", "rtosPlatform": "rust", "dryRun": "true"}.items():
                        if key in params:
                            changed = simulate_event(sensor, {**body, key: value})
                            got = {p["name"]: p["value"] for p in changed["spec"]["arguments"]["parameters"]}
                            if got[key] != value:
                                fail(f"{sensor_name} did not map body.{key}")
            finally:
                server.shutdown()
        print(f"argo lint, dry-run submit and Sensor events ({label}): ok")


def main():
    lint()
    check_render()
    check_catalog()
    check_applications()
    check_workflows()
    check_argo()
    print("openbsw-module smoke: ok")


if __name__ == "__main__":
    main()

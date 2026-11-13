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
"""

import subprocess
import sys
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


def main():
    lint()
    check_render()
    check_catalog()
    check_applications()
    print("openbsw-module smoke: ok")


if __name__ == "__main__":
    main()

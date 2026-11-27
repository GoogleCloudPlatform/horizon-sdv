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
client-side `argo submit --dry-run --from workflowtemplate/...` for every
WorkflowTemplate, and simulates a webhook event for each Sensor. The dry-run
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
WORKFLOW_TEMPLATES = {
    "openbsw-bsw-builder": "webhook-openbsw-bsw-builder",
    "openbsw-builder-image": "webhook-openbsw-builder-image",
    "openbsw-posix-test": "webhook-openbsw-posix-test",
    "openbsw-dev-instance": "webhook-openbsw-dev-instance",
    "openbsw-delete-mtkc-testbench": "webhook-openbsw-delete-mtkc-testbench",
}
# Only calls the MTK Connect API, so it does not schedule onto the OpenBSW pool.
DEFAULT_POOL_TEMPLATES = {"openbsw-delete-mtkc-testbench"}
# MTK Connect session templates: one long-running pod, plus an exit handler that deletes the testbench.
MTK_SESSIONS = {"openbsw-posix-test": "test", "openbsw-dev-instance": "instance"}
MTK_SECRET = "workflow-mtk-connect-apikey"
SUBMITTED_BY_NAME_DEST = r"metadata.annotations.horizon-sdv\.io/submitted-by-name"
# Jenkins job.groovy for each WorkflowTemplate. Every Jenkins parameter must exist as its camelCase name.
JENKINS_JOBS = {
    "openbsw-bsw-builder": "builds/bsw_builder",
    "openbsw-builder-image": "environment/docker_image_template",
    "openbsw-posix-test": "tests/posix",
    "openbsw-dev-instance": "environment/dev_instance",
    "openbsw-delete-mtkc-testbench": "environment/delete_mtkc_testbench",
}
PIPELINE_REPO_SET = [
    "--set", "pipelineRepo.url=https://example.com/horizon-sdv.git",
    "--set", "pipelineRepo.revision=main",
]


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
    run(["helm", "lint", str(CHART / "argo-workflows"), *PIPELINE_REPO_SET])
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
    if openbsw is None or len(namespaces) != 1:
        fail(f"expected only namespace openbsw (it also holds the overview), got {sorted(namespaces)}")
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
    if service["metadata"]["name"] != "mod-openbsw-overview" or service["metadata"]["namespace"] != "openbsw":
        fail("overview Service name or namespace does not match the catalog")
    if service["spec"]["ports"][0]["port"] != 80:
        fail("overview Service must listen on port 80")
    policies = {doc["metadata"]["name"]: doc for doc in by_kind(rendered, "NetworkPolicy")}
    overview_policy = policies.get("allow-overview-from-module-manager")
    if overview_policy is None:
        fail("openbsw has default-deny-all, so the overview needs allow-overview-from-module-manager")
    peer = overview_policy["spec"]["ingress"][0]["from"][0]["namespaceSelector"]["matchLabels"]
    if peer.get("kubernetes.io/metadata.name") != "module-manager":
        fail(f"overview ingress must come from the module-manager namespace, got {peer}")

    script = daemonsets[0]["spec"]["template"]["spec"]["containers"][0]["command"][2]
    if "set -e" in script or "sleep infinity" not in script:
        fail("CAN DaemonSet must keep running when a module cannot be loaded (no set -e, then sleep infinity)")

    html = next(
        doc["data"]["index.html"]
        for doc in by_kind(rendered, "ConfigMap")
        if doc["metadata"]["name"] == "mod-openbsw-overview-html"
    )
    if "<h1>openbsw-module</h1>" not in html:
        fail("overview ConfigMap does not contain the portal page")

    nginx = yaml.safe_load(LINT_VALUES.read_text())["config"]["nginx"]
    overview = next(doc for doc in by_kind(rendered, "Deployment") if doc["metadata"]["name"] == "mod-openbsw-overview")
    image = overview["spec"]["template"]["spec"]["containers"][0]["image"]
    if image != f"{nginx['image']}:{nginx['tag']}":
        fail(f"overview nginx image must come from config.nginx (versions.yaml tools.nginx.tag), got {image}")

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
        "overviewService": "mod-openbsw-overview",
        "overviewServiceNamespace": "openbsw",
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
    source = app["spec"]["source"]
    child_values = yaml.safe_load(source["helm"]["values"])
    lint_repo = yaml.safe_load(LINT_VALUES.read_text())["repo"]
    if source["repoURL"] != lint_repo["url"] or source["targetRevision"] != lint_repo["revision"]:
        fail("child Application must sync from repo.url@repo.revision")
    pipeline_repo = child_values["pipelineRepo"]
    if pipeline_repo["url"] != lint_repo["url"] or pipeline_repo["revision"] != lint_repo["revision"]:
        fail(f"pipelineRepo must be repo.url@repo.revision so templates and scripts share a revision, got {pipeline_repo}")
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
    expected = WORKFLOW_TEMPLATES
    if sorted(templates) != sorted(expected) or sorted(sensors) != sorted(expected.values()):
        fail(f"expected WorkflowTemplates {sorted(expected)} and Sensors {sorted(expected.values())}, got {sorted(templates)} and {sorted(sensors)}")

    for name, sensor_name in expected.items():
        template = templates[name]
        labels = template["metadata"].get("labels", {})
        if labels.get("horizon-sdv.io/expose") != "true" or labels.get("horizon-sdv.io/module") != "openbsw-module":
            fail(f"{name} must carry the expose and openbsw-module labels")
        if template["metadata"]["namespace"] != "workflows":
            fail(f"{name} must be in the workflows namespace")
        if name in DEFAULT_POOL_TEMPLATES:
            if "templateDefaults" in template["spec"]:
                fail(f"{name} only calls the MTK Connect API and must not select the OpenBSW node pool")
        else:
            defaults = template["spec"]["templateDefaults"]
            if defaults["nodeSelector"].get("workloadLabel") != "openbsw":
                fail(f"{name} pods must select the OpenBSW node pool")
            if defaults["metadata"]["labels"].get("openbsw_pod") != "true":
                fail(f"{name} pods must carry the openbsw_pod label shared with Jenkins")
            if defaults["metadata"]["annotations"].get("cluster-autoscaler.kubernetes.io/safe-to-evict") != "false":
                fail(f"{name} pods must not be evicted by a scale-down")
            term = defaults["affinity"]["podAntiAffinity"]["requiredDuringSchedulingIgnoredDuringExecution"][0]
            if term["topologyKey"] != "kubernetes.io/hostname" or term["labelSelector"]["matchExpressions"][0]["key"] != "openbsw_pod":
                fail(f"{name} pods need required anti-affinity on openbsw_pod per node")
        if template["spec"].get("serviceAccountName") != "workflow-executor":
            fail(f"{name} must run as workflow-executor; only ai-review uses the elevated account")

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
        # TAA-2457: the Developer Portal shows startedBy from this annotation (display only).
        name_mapping = [m for m in trigger["parameters"] if m["dest"] == SUBMITTED_BY_NAME_DEST]
        if len(name_mapping) != 1 or name_mapping[0]["src"].get("dataKey") != "body.submittedBy" or name_mapping[0]["src"].get("value") != "":
            fail(f"{sensor_name} must map body.submittedBy (default \"\") to {SUBMITTED_BY_NAME_DEST}")

    # build-defaults mints {{workflow.uid}}-pipeline-git-creds itself, so the builder's own creds task must
    # run after build-image: no overlapping writes, and a token that is fresh when fetch-pipeline starts.
    def dag_tasks(template, name):
        dag = next(t for t in template["spec"]["templates"] if t["name"] == name)["dag"]
        return {task["name"]: task for task in dag["tasks"]}

    main = dag_tasks(templates["openbsw-bsw-builder"], "main")
    if main["prepare-pipeline-git-creds"].get("depends") != "build-image.Succeeded || build-image.Skipped":
        fail("prepare-pipeline-git-creds must depend on build-image.Succeeded || build-image.Skipped")
    if "prepare-pipeline-git-creds.Succeeded" not in main["fetch-pipeline"]["depends"]:
        fail("fetch-pipeline must wait for prepare-pipeline-git-creds")
    # The workflow-level mutex only covers standalone image runs. build-defaults is called from
    # openbsw-bsw-builder through templateRef, so it needs the same mutex at template level.
    image_spec = templates["openbsw-builder-image"]["spec"]
    workflow_mutex = image_spec["synchronization"]["mutex"]["name"]
    build_defaults = next(t for t in image_spec["templates"] if t["name"] == "build-defaults")
    if build_defaults.get("synchronization", {}).get("mutex", {}).get("name") != workflow_mutex:
        fail("build-defaults must lock the same mutex as the openbsw-builder-image workflow")
    if "synchronization" in next(t for t in image_spec["templates"] if t["name"] == "build"):
        fail("the build entrypoint must not lock again: the workflow-level mutex already covers it")
    image_tasks = dag_tasks(templates["openbsw-builder-image"], "build-defaults")
    if "depends" in image_tasks["prepare-pipeline-git-creds"]:
        fail("prepare-pipeline-git-creds in the image build must run first")
    if image_tasks["build-image"]["depends"] != "prepare-pipeline-git-creds.Succeeded":
        fail("the image build must wait for its git credentials")

    steps = {t["name"]: t for t in templates["openbsw-bsw-builder"]["spec"]["templates"]}
    check_image = steps["check-image"]["script"]
    if check_image["image"] != "gcr.io/google.com/cloudsdktool/google-cloud-cli:slim" or "@sha256:" in check_image["image"]:
        fail(f"check-image must use the moving Cloud SDK tag, got {check_image['image']}")
    if "resources" not in check_image:
        fail("check-image must set resources")
    fail_step = steps["fail-workflow"]["container"]
    if "openbsw-builder" in fail_step["image"] or "resources" not in fail_step:
        fail(f"fail-workflow must use a small image and set resources, got {fail_step['image']}")

    params = {p["name"]: p for p in templates["openbsw-bsw-builder"]["spec"]["arguments"]["parameters"]}
    if params["aiReviewRuntime"]["value"] != "agy":
        fail("openbsw-bsw-builder aiReviewRuntime must default to agy")
    if steps["ai-review"].get("serviceAccountName") != "workflow-executor-elevated":
        fail("ai-review must run as workflow-executor-elevated (Vertex AI)")
    image_params = {p["name"]: p["value"] for p in image_spec["arguments"]["parameters"]}
    if image_params.get("noPush") != "true" or "dryRun" in image_params:
        fail("openbsw-builder-image must expose noPush, default true (Jenkins NO_PUSH)")
    for template_name, push in (("build", "{{workflow.parameters.noPush}}"), ("build-defaults", "false")):
        task = dag_tasks(templates["openbsw-builder-image"], template_name)["build-image"]
        args = {p["name"]: p["value"] for p in task["arguments"]["parameters"]}
        if args.get("openbswPool") != "true":
            fail(f"openbsw-builder-image {template_name} must pass openbswPool=true")
        if args.get("dryRun") != push:
            fail(f"openbsw-builder-image {template_name} must pass dryRun={push}, got {args.get('dryRun')}")
    if "ai-review" not in run(["helm", "template", str(CHART / "argo-workflows"), "--set", "cloud.project=p1", *PIPELINE_REPO_SET]):
        fail("openbsw-bsw-builder must render the ai-review step")
    for missing in ("pipelineRepo.url", "pipelineRepo.revision"):
        unset = subprocess.run(
            ["helm", "template", str(CHART / "argo-workflows"), "--set", "cloud.project=p1", *PIPELINE_REPO_SET, "--set", f"{missing}="],
            cwd=ROOT, text=True, capture_output=True,
        )
        if unset.returncode == 0 or f"{missing} is required" not in unset.stderr:
            fail(f"rendering without {missing} must fail with a 'required' error")
    check_mtk(templates, creds=True)
    check_jenkins_parameters(templates)
    check_gemini_command_line(steps["ai-review"]["container"])
    print("WorkflowTemplates and Sensors: ok")


def jenkins_camel(name):
    first, *rest = name.lower().split("_")
    return first + "".join(part.capitalize() for part in rest)


def check_jenkins_parameters(templates):
    for name, job in JENKINS_JOBS.items():
        groovy = (ROOT / "workloads/openbsw/pipelines" / job / "groovy/job.groovy").read_text()
        jenkins = [p for kind, p in re.findall(r"(\w+)\s*\{\s*name\('([A-Za-z0-9_]+)'\)", groovy) if kind != "separator"]
        argo = {p["name"] for p in templates[name]["spec"]["arguments"]["parameters"]}
        missing = [p for p in jenkins if jenkins_camel(p) not in argo]
        if not jenkins or missing:
            fail(f"{name} must expose every Jenkins parameter of {job} in camelCase; missing {missing}")
    print("Jenkins parameters: all exposed as workflow parameters")


def check_gemini_command_line(container):
    """GEMINI_COMMAND_LINE is eval'd in the elevated ai-review pod, so only plain gemini options pass."""
    if container_env(container)["GEMINI_COMMAND_LINE"].get("value") != "{{workflow.parameters.geminiCommandLine}}":
        fail("ai-review GEMINI_COMMAND_LINE must come from the geminiCommandLine parameter")
    script = container["args"][0].replace('bash "${PIPELINE_REPO_ROOT}/workloads/common/agentic-ai/gemini/run_ai_review.sh"', "echo accepted")
    cases = [
        ("gemini-cli", "gemini --yolo --output-format json", True),
        ("gemini-cli", "gemini --model gemini-3.8-flash --yolo --output-format json --debug", True),
        ("agy", "gemini; id", True),
        ("gemini-cli", "gemini --yolo; id", False),
        ("gemini-cli", "gemini $(id)", False),
        ("gemini-cli", "gemini `id`", False),
        ("gemini-cli", "gemini --yolo | sh", False),
        ("gemini-cli", "gemini --yolo\nid", False),
        ("gemini-cli", "gemini 'x'", False),
        ("gemini-cli", "sh -c id", False),
        ("gemini-cli", "", False),
    ]
    for runtime, command_line, accepted in cases:
        result = subprocess.run(
            ["bash", "-euo", "pipefail", "-c", script],
            env={"PATH": os.environ["PATH"], "AI_REVIEW_RUNTIME": runtime, "GEMINI_COMMAND_LINE": command_line},
            text=True, capture_output=True,
        )
        if (result.returncode == 0 and "accepted" in result.stdout) != accepted:
            fail(f"ai-review with {runtime} and geminiCommandLine={command_line!r} must be {'accepted' if accepted else 'rejected'}")


def container_env(container):
    return {e["name"]: e for e in container.get("env", [])}


def check_mtk_secret(name, container):
    env = container_env(container)
    for var in ("MTK_CONNECT_USERNAME", "MTK_CONNECT_PASSWORD"):
        ref = env.get(var, {}).get("valueFrom", {}).get("secretKeyRef", {})
        if ref.get("name") != MTK_SECRET or "value" in env.get(var, {}):
            fail(f"{name}: {var} must come from Secret {MTK_SECRET}")
    # Jenkins passed MTK_CONNECT_PASSWORD=... on the sudo command line, where ps shows it.
    if re.search(r"MTK_CONNECT_PASSWORD=\S", container["args"][0]):
        fail(f"{name}: the MTK Connect password must reach mtk_connect.sh through the environment only")


def check_mtk(templates, creds):
    """MTK Connect wiring of the POSIX test, development instance and delete templates."""
    for name, session in MTK_SESSIONS.items():
        spec = templates[name]["spec"]
        steps = {t["name"]: t for t in spec["templates"]}
        if spec.get("onExit") != "exit-handler":
            fail(f"{name} must run exit-handler, which deletes a testbench left by a failed session")
        container = steps[session]["container"]
        if container.get("securityContext", {}).get("privileged") is not True:
            fail(f"{name}: {session} must be privileged like the Jenkins pod (tap and vcan0)")
        if not any(m["name"] == "host-lib-modules" and m.get("readOnly") for m in container.get("volumeMounts", [])):
            fail(f"{name}: {session} must mount /lib/modules read-only")
        if "terminationGracePeriodSeconds" not in steps[session].get("podSpecPatch", ""):
            fail(f"{name}: {session} needs a grace period so mtk_connect.sh --stop runs after SIGTERM")
        args = container["args"][0]
        for needle in ("mtk_connect.sh --start", "mtk_connect.sh --stop", "trap ", "--preserve-env"):
            if needle not in args:
                fail(f"{name}: {session} script is missing {needle!r}")
        check_mtk_secret(name, container)
        testbench = container_env(container)["MTK_CONNECT_TESTBENCH"]["value"]
        if not testbench.startswith("OpenBSW/") or not testbench.endswith("-{{workflow.name}}"):
            fail(f"{name}: testbench must be OpenBSW/...-{{{{workflow.name}}}}, got {testbench}")

        handler = [step for group in steps["exit-handler"]["steps"] for step in group]
        names = [step["name"] for step in handler]
        want = (["prepare-pipeline-git-creds"] if creds else []) + ["mtk-cleanup"]
        if names != want:
            fail(f"{name}: exit-handler steps {names}, want {want}")
        for step in handler:
            if "'{{workflow.status}}' != 'Succeeded'" not in step.get("when", ""):
                fail(f"{name}: exit-handler step {step['name']} must only run when the workflow did not succeed")
        if name == "openbsw-dev-instance" and "mtkConnectEnable" not in handler[-1]["when"]:
            fail(f"{name}: mtk-cleanup must only run when mtkConnectEnable is true")
        cleanup = steps["mtk-cleanup"]["container"]
        if container_env(cleanup)["MTK_CONNECT_TESTBENCH"]["value"] != testbench:
            fail(f"{name}: mtk-cleanup must delete the session's own testbench")
        check_mtk_secret(f"{name} mtk-cleanup", cleanup)

    spec = templates["openbsw-delete-mtkc-testbench"]["spec"]
    if spec.get("synchronization", {}).get("mutex", {}).get("name") != "openbsw-delete-mtkc-testbench":
        fail("openbsw-delete-mtkc-testbench must run one at a time (Jenkins blockOn)")
    delete = next(t for t in spec["templates"] if t["name"] == "delete")["container"]
    if not delete["image"].endswith("/openbsw-builder:{{workflow.parameters.imageTag}}"):
        fail(f"openbsw-delete-mtkc-testbench must run the imageTag builder image (Jenkins IMAGE_TAG), got {delete['image']}")
    if delete.get("securityContext", {}).get("privileged"):
        fail("openbsw-delete-mtkc-testbench only calls the MTK Connect API and must not be privileged")
    for needle in ("mtk_connect.sh --delete", "MTK_CONNECT_DELETE_OFFLINE_TESTBENCHES=true", "sudo --preserve-env="):
        if needle not in delete["args"][0]:
            fail(f"openbsw-delete-mtkc-testbench delete script is missing {needle!r}")
    check_mtk_secret("openbsw-delete-mtkc-testbench", delete)

    none = docs(run(["helm", "template", str(WORKFLOW_CHART), "--set", "cloud.project=p1", *PIPELINE_REPO_SET]))
    none_templates = {d["metadata"]["name"]: d for d in by_kind(none, "WorkflowTemplate")}
    for name in MTK_SESSIONS:
        handler = next(t for t in none_templates[name]["spec"]["templates"] if t["name"] == "exit-handler")
        if [s["name"] for g in handler["steps"] for s in g] != ["mtk-cleanup"]:
            fail(f"{name}: without git credentials the exit handler must only run mtk-cleanup")
    print("MTK Connect session, exit handler and delete templates: ok")

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
    """Serves template GETs, plus the SelfSubjectAccessReview POST from argo CLI v3.7+.

    v3.5.10 only GETs WorkflowTemplates. v3.7.10 posts a SelfSubjectAccessReview
    (get/list/watch on clusterworkflowtemplates) before it reads them.
    """

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

    def do_POST(self):
        self.rfile.read(int(self.headers.get("Content-Length", 0)))
        data = json.dumps({"kind": "SelfSubjectAccessReview", "apiVersion": "authorization.k8s.io/v1", "status": {"allowed": True}}).encode()
        self.send_response(201)
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
        key = mapping["src"]["dataKey"].removeprefix("body.")
        if key not in body and "value" not in mapping["src"]:
            continue
        value = str(body.get(key, mapping["src"].get("value", "")))
        if dest.startswith("spec.arguments.parameters."):
            workflow["spec"]["arguments"]["parameters"][int(dest.split(".")[3])]["value"] = value
        elif dest.startswith("metadata.annotations."):
            annotation = dest.removeprefix("metadata.annotations.").replace("\\.", ".")
            workflow["metadata"].setdefault("annotations", {})[annotation] = value
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

                for name in WORKFLOW_TEMPLATES:
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
                    if workflow["metadata"].get("annotations", {}).get("horizon-sdv.io/submitted-by-name") != "":
                        fail(f"{sensor_name} must still submit, with an empty submitted-by-name, when body.submittedBy is absent")
                    named = simulate_event(sensor, {**body, "submittedBy": "dev-alice", "horizonSubmittedBy": "c7eae7a2"})
                    annotations = named["metadata"].get("annotations", {})
                    if annotations.get("horizon-sdv.io/submitted-by-name") != "dev-alice" or annotations.get("horizon-sdv.io/submitted-by") != "c7eae7a2":
                        fail(f"{sensor_name} must map body.submittedBy to submitted-by-name and body.horizonSubmittedBy to submitted-by, got {annotations}")
                    event_path = Path(tmp) / f"event-{name}.yaml"
                    workflow["metadata"].pop("namespace", None)
                    event_path.write_text(yaml.safe_dump(workflow))
                    argo(["submit", "--dry-run", "-n", "workflows", str(event_path), "-o", "name"], env)
                    for key, value in {
                        "imageTag": "custom-tag",
                        "rtosPlatform": "rust",
                        "noPush": "false",
                        "openbswDownloadUrl": "gs://p1-openbsw/OpenBSW/Builds/BSW_Builder/x/posix/",
                        "mtkConnectTestbench": "OpenBSW/Tests/POSIX-x",
                        "mtkConnectEnable": "true",
                        "submittedBy": "dev@example.com",
                        "geminiCommandLine": "gemini --model m --yolo --output-format json",
                        "openbswArtifactStorageSolution": "",
                    }.items():
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

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

"""Turns `helm template` output of the module charts into what the kind harness applies.

Test-only changes, nothing here is used by the platform:
  * drops Argo CD Applications (the harness applies the child chart directly, there is no Argo CD);
  * controller: TEST_NFS_SERVER / TEST_NFS_PATH (plain NFS instead of Filestore) and imagePullPolicy IfNotPresent
    (images are loaded with `kind load`);
  * WorkflowTemplate containers: GIT_SSL_NO_VERIFY=true (the e2e Gitea uses a self-signed certificate).

usage: render.py module|workflows IN.yaml OUT.yaml [--nfs-server IP]
"""

import argparse
import sys

import yaml


def patch_module(docs, nfs_server):
    out = []
    for d in docs:
        if d.get("kind") == "Application":
            continue
        if d.get("kind") == "Deployment" and d["metadata"]["name"] == "storage-mirror-module":
            c = d["spec"]["template"]["spec"]["containers"][0]
            c["imagePullPolicy"] = "IfNotPresent"
            c["env"] = [e for e in c["env"] if e["name"] not in ("TEST_NFS_SERVER", "TEST_NFS_PATH")]
            if not nfs_server:
                sys.exit("--nfs-server is required for the module chart")
            c["env"] += [{"name": "TEST_NFS_SERVER", "value": nfs_server}, {"name": "TEST_NFS_PATH", "value": "/"}]
        out.append(d)
    return out


def patch_workflows(docs):
    for d in docs:
        if d.get("kind") != "WorkflowTemplate":
            continue
        for t in d["spec"]["templates"]:
            c = t.get("container")
            if not c:
                continue
            env = c.setdefault("env", [])
            if not any(e["name"] == "GIT_SSL_NO_VERIFY" for e in env):
                env.append({"name": "GIT_SSL_NO_VERIFY", "value": "true"})
    return docs


def child_values(docs):
    """The helm values the parent chart hands to mod-storage-mirror-argo-workflows (same as in production)."""
    for d in docs:
        if d.get("kind") == "Application" and d["metadata"]["name"].endswith("-argo-workflows"):
            return d["spec"]["source"]["helm"]["values"]
    sys.exit("child Application not found in the module chart output")


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("chart", choices=["module", "workflows", "child-values"])
    ap.add_argument("src")
    ap.add_argument("dst")
    ap.add_argument("--nfs-server", default="")
    a = ap.parse_args()
    with open(a.src) as f:
        docs = [d for d in yaml.safe_load_all(f) if d]
    if a.chart == "child-values":
        with open(a.dst, "w") as f:
            f.write(child_values(docs))
        print(f"{a.dst}: child chart values")
        return
    docs = patch_module(docs, a.nfs_server) if a.chart == "module" else patch_workflows(docs)
    with open(a.dst, "w") as f:
        yaml.safe_dump_all(docs, f, sort_keys=False)
    print(f"{a.dst}: {len(docs)} objects")


if __name__ == "__main__":
    main()

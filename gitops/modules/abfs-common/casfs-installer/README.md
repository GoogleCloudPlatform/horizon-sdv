<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# abfs-casfs-installer (library chart)

One implementation of the `casfs` kernel-module installer for all ABFS modules: a ConfigMap with
the loader script and the privileged DaemonSet that runs it on every node of a pool. It is a Helm
library chart, so it deploys nothing by itself; each module renders it in its own Argo CD
Application, namespace and node pool.

| Consumer chart | DaemonSet / ConfigMap | Namespace | Mode |
|----------------|-----------------------|-----------|------|
| `abfs-service-module/abfs` | `abfs-casfs-installer` | `abfs` | source build (uploader pool, Ubuntu) |
| `abfs-client-module-ubuntu` | `abfs-casfs-installer-client` | `module-manager` | prebuilt package, then source build (build pool, Ubuntu) |

`abfs-client-module-cos` does not use it: COS nodes ship casfs in the node image, so no installer
runs there.

Every DaemonSet selects its pods with the same two labels (`app.kubernetes.io/name: abfs`,
`app.kubernetes.io/component: casfs-installer`). Use the chart once per namespace; two consumers
in one namespace would select each other's pods.

## Use it in a chart

`Chart.yaml` (path relative to the consuming chart):

```yaml
dependencies:
  - name: abfs-casfs-installer
    version: 0.1.0
    repository: file://../abfs-common/casfs-installer
```

Two template files, each with one line:

```yaml
{{ include "abfs-casfs-installer.configmap" . }}
```
```yaml
{{ include "abfs-casfs-installer.daemonset" . }}
```

Argo CD runs `helm dependency build` for the chart, so nothing else is committed (no `charts/`
directory, no `Chart.lock`). For a local render run `helm dependency build <chart>` first.

## Values read from the consuming chart

| Value | Meaning |
|-------|---------|
| `casfs.provider` | `daemonset`: install casfs on the node (Ubuntu). Any other value, for example `image` (the node image ships casfs), renders nothing. |
| `casfs.daemonset.name` | Name of the DaemonSet and ConfigMap. Default `abfs-casfs-installer`. |
| `casfs.daemonset.image`, `imagePullPolicy`, `resources` | Loader container. The image needs `bash`, `chroot` and `gcloud`. Pull policy defaults to `image.pullPolicy`, then `IfNotPresent`. |
| `casfs.daemonset.sourceUrl` | Directory that holds the source tarball. Required. |
| `casfs.daemonset.version` | casfs version. Required. The tarball is `<sourceUrl>/casfs-<version>.tar.gz` in every module. |
| `casfs.daemonset.aptProject`, `aptLocation`, `aptRepository` | Artifact Registry apt repository with prebuilt `casfs-kmod-<kernel>_<version>` packages. When `aptRepository` is set the package is tried first; when it is empty casfs is always built from source. Only `abfs-client-module-ubuntu` sets it. |
| `casfs.daemonset.script` | Replaces the loader script. |
| `casfs.daemonset.nodeSelector`, `tolerations` | Where the DaemonSet runs. Default: the chart's `nodeSelector` / `tolerations`. |
| `commonLabels`, `commonAnnotations`, `podLabels`, `podAnnotations`, `imagePullSecrets`, `priorityClassName` | Same meaning as in the ABFS charts. |

## What the DaemonSet does

- Ubuntu nodes (`UBUNTU_CONTAINERD`, `casfs.provider: daemonset`): it installs the prebuilt apt
  package for the running kernel when `aptRepository` is set, otherwise or when the package does
  not load it builds casfs from source on the node, then runs `modprobe casfs`.
- The node needs Secure Boot off and a node service account with read access to the package
  repository and the source bucket.
- The pod is privileged, uses the host PID and network namespaces and mounts the host root,
  because it installs a package and inserts a kernel module on the node.
- COS nodes (`COS_CONTAINERD`, `casfs.provider: image`): casfs is shipped and signed in the node
  image (COS 129 LTS, GKE ≥ 1.36); nothing is rendered.

The script is the same text in every module; the DaemonSet passes the settings as `CASFS_*`
environment variables. The pod stays `Running` after casfs is loaded and exits with an error
(so it restarts and shows `CrashLoopBackOff`) when casfs cannot be loaded.

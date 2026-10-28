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

# How to create a new module

This guide is for module authors. It explains the sample modules, then walks through building, registering and enabling a new one. For the concepts behind modules (dependencies, Module Manager, pinning), read [Horizon modules overview](01_overview.md) first. For the portal screens used along the way, see the [Horizon Portal guide](02_horizon-portal.md).

## Table of Contents

- [Module manager](#module-manager)
  - [Sample modules and dependencies](#sample-modules-and-dependencies)
  - [Change git ref in developer portal](#change-git-ref-in-developer-portal)
- [Create a new module](#create-a-new-module)
- [Workflows via Developer Portal](#workflows-via-developer-portal)
- [Workflows via Horizon CLI](#workflows-via-horizon-cli)
- [Expose label (catalog and portal links)](#expose-label-catalog-and-portal-links)

## Module manager

Each module is a separate Argo CD Application (app-of-apps) that Module Manager creates or deletes when you enable or disable the module in **Administration → Modules** of the Developer Portal (`https://<your-horizon-domain>/developer-portal/`). The modules on offer come from the Module Catalog; the [Horizon Portal guide](02_horizon-portal.md#description) lists the ones this release ships. How the module cards, statuses and switches behave is described under [Administration](02_horizon-portal.md#administration).

### Sample modules and dependencies

Three sample modules show how [hard and soft dependencies](01_overview.md#how-modules-depend-on-each-other) behave. `sample` has a hard dependency on `sample-hard` and a soft dependency on `sample-soft`:

|  | sample-hard | sample-soft | sample |
|---|---|---|---|
| **Dependency type** | Hard dep *target* | Soft dep *target* | Parent with both |
| **Required for sample?** | Yes | No | — |
| **Auto-enabled with sample?** | Yes | No | — |
| **Can be disabled while sample is on?** | No. The API returns `409` with `hardDependents: ["sample"]`, and the portal shows *"Cannot disable: required by: sample"*. | Yes | — |
| **Horizon-exposed workflows** | No (`sample-hard-smoke-test` is internal) | No | Yes (`sample-smoke-test`, `sample-smoke-test-alt`) |
| **When unused** | Auto-disabled when `sample` is disabled | Auto-disabled when `sample` is disabled | Stays while you want the app |

`sample` also lists `sample-data` and `storage-gcs` as soft dependencies, to show a module using GCS. Both `sample-hard` and `sample-soft` set `autoDisableWhenUnused: true` in the catalog. Auto-disable is optional, and modules without that flag stay enabled when nothing depends on them.

When `sample-soft` is enabled, `sample` sees it through the [soft-feature contract](04_module-architecture.md#soft-feature-contract). Its Hello World application shows the result; see [Hello-world application](#hello-world-application).

### Change git ref in developer portal

Each module can be pinned to its own branch, tag or commit from **Administration → Modules**, or reset to follow the platform branch. Module Manager updates the module's Argo CD Application to the chosen ref, and Argo CD re-syncs it. The buttons are described under [Administration → Modules](02_horizon-portal.md#modules), and how pinning interacts with platform upgrades under [Pinning and Resetting module Git refs](04_module-architecture.md#pinning-and-resetting-module-git-refs).

## Create a new module

Modules are not discovered automatically. You add a Helm chart under `gitops/modules/`, register it in the `ModuleCatalog` CR, commit to the GitOps repo, then enable it in **Administration → Modules**. Partner isolation (adding a module without changing Horizon core) is not shipped; the catalog file is still required.

Use `gitops/modules/sample-hard-module` as the copy source unless you need hard+soft dependencies (`sample-module`) or GCS (`storage-gcs-module` / `sample-data-module`).

### What you are adding

On enable, Module Manager creates one **parent** Argo CD Application named `mod-<moduleName>` (underscores in the name become hyphens). That parent deploys your module chart. The chart usually creates **child** Applications (workflows, hello-world, and similar) plus an in-cluster nginx Service that serves overview HTML.

Module Manager injects Helm values into the parent:

- `moduleName`.
- `config`, the cluster `MODULE_CONFIG`.
- `overviewNamespace`, from the catalog, when the module has an overview Service.
- `repo.url` and `repo.revision`.
- `softFeaturesEnabled`, when the module has soft dependencies; see the [soft-feature contract](04_module-architecture.md#soft-feature-contract).

Do not hard-code the Git repo URL or revision in a way that ignores those values.

### 1. Choose a name

Pick a DNS-1123 token used as the catalog `name` and as `horizon-sdv.io/module` labels (for example `example`).

- Parent Application: `mod-example`
- Typical chart directory: `gitops/modules/example-module` (directory name need not match catalog name; `workloads-android` does not use a `-module` suffix)
- Overview Service name: `mod-example-overview` (must match catalog `overviewService`)

### 2. Copy a sample chart

From the repository root:

```bash
cp -R gitops/modules/sample-hard-module gitops/modules/example-module
```

Edit `gitops/modules/example-module/Chart.yaml`: set `name` to `example-module` and update `description`.

### 3. Rewire Helm identity and child Application paths

In `gitops/modules/example-module/values.yaml`:

- Keep `moduleName: ""` — Module Manager sets it on enable.
- Set `overviewServiceName: mod-example-overview`.
- Set `overviewNamespace` and any hello-world namespace (for example `example-module-hello`) so they match where those workloads actually run.
- Leave `repo.url` / `repo.revision` as placeholders; the parent Application fills them.

In every child Application template under `templates/` (for example `application-argo-workflows.yaml`, `application-hello-world.yaml`):

- Change `spec.source.path` from `gitops/modules/sample-hard-module/...` to `gitops/modules/example-module/...`.
- Keep labels `horizon-sdv.io/module: {{ .Values.moduleName }}`, `horizon-sdv.io/app-role: child`, and `horizon-sdv.io/module-manager-managed: "true"`. Module Manager uses them to report the module's status and to find its applications.
- Keep the `resources-finalizer.argocd.argoproj.io` finalizer, so deleting the parent on disable also deletes the child's workloads.
- Keep `spec.source.repoURL` / `targetRevision` on `.Values.repo.*`.
- Destination namespaces should use `config.namespacePrefix` for platform namespaces (`workflows`, `argo-events`) and your dedicated namespace for hello-world.

Rename `helloWorldNamespace` (and the hello-world subchart resources) if you keep that app so they do not collide with `sample-hard`.

### 4. Overview HTML and in-cluster Service

1. Edit `portal/overview.html` (title and body for **example**). Module Manager does not read this file from Git at runtime.
2. Keep `templates/module-overview-http.yaml`. It packages the HTML into a ConfigMap, runs nginx, and exposes a ClusterIP Service on port **80** path `/`.
3. Catalog `overviewService` / `overviewServiceNamespace` must match that Service name and namespace. Module Manager always GETs `http://<service>.<namespace>.svc.cluster.local:80/`.

If either catalog field is missing, **Overview** in the portal shows a configuration message and `GET /modules/{name}/overview` returns 404.

### 5. Workflows and Argo Events sensors

Skip this if the module has no user-facing pipelines.

1. Under `argo-workflows/templates/workflowtemplates.yaml`, set `metadata.name` uniquely (cluster-wide in the workflows namespace). Set label `horizon-sdv.io/module: example`.
2. To list the template in the Developer Portal and Horizon API catalog, add the label `horizon-sdv.io/expose: "true"`. See [Expose label](#expose-label-catalog-and-portal-links). Internal templates (like `sample-hard-smoke-test`) omit expose.
3. Duplicate the Sensor in `argo-workflows/templates/sensors.yaml`. The EventSource filter `body.workflowTemplateName` **must** equal the WorkflowTemplate name. Map submit body fields onto Workflow parameters the same way the sample sensors do (`horizonSubmittedFrom`, your parameters). Horizon API only POSTs to the Argo Events webhook; without a matching Sensor, submit returns success from the API's point of view of dispatch but no Workflow is created.
4. Point the child Application `path` at `gitops/modules/example-module/argo-workflows` and pass `workflowNamespace` / `eventsNamespace` as in the sample.

### 6. Register the module in ModuleCatalog

Edit `gitops/apps/module-manager/templates/module-catalog.yaml`. Add an entry under `spec.modules`:

```yaml
    - name: example
      path: gitops/modules/example-module
      overviewPath: portal/overview.html
      overviewService: mod-example-overview
      overviewServiceNamespace: example-module-hello
      # hardDependencies:
      #   - storage-gcs
      # softDependencies:
      #   - sample-soft
      # autoDisableWhenUnused: true
```

Required: `name`, `path` (repo-relative chart path). `overviewPath` is informational; Module Manager serves the overview from the Service, not from this file.

Optional:

| Field | Effect |
|---|---|
| `hardDependencies` | Those modules enable first; they cannot be disabled while this module is on |
| `softDependencies` | Optional extras; parent still enables without them |
| `autoDisableWhenUnused` | Disable this module when the last enabled module that lists it as a hard or soft dependency is disabled |
| `applications` | Extra portal links (`id`, `url`, optional `title`; `url` may be site-relative `/...`) |
| `softFeaturesPropagation` / `softFeaturesConfigMapNamespaces` | How soft-dep flags reach workloads; see the [soft-feature contract](04_module-architecture.md#soft-feature-contract) |

Catalog name `example` must match labels and Sensor/template ownership. Path must be the parent chart directory, not the `argo-workflows` subchart.

### 7. Ship, sync, and enable

1. Commit and push to the branch Argo CD syncs (`repo.revision` / platform branch).
2. Sync the **module-manager** Application so the `ModuleCatalog` CR updates. The module appears in Administration even before it is enabled.
3. In the Developer Portal: **Administration → Modules → Enable** on `example`. Wait until status is READY (parent and children healthy). Do not reload the browser while enable is in progress.
4. Confirm a sidebar tab for `example`, Overview HTML, and (if exposed) Workflow Templates.

To iterate on Git content after enable: pin or follow the platform ref as in [Change git ref in developer portal](#change-git-ref-in-developer-portal).

### Verify

- `kubectl get modulecatalog cluster -n module-manager -o yaml` lists `example`.
- After enable: `kubectl get application -n <prefix>argocd` shows `mod-example` and children `mod-example-*`.
- Portal Overview loads; with `expose: "true"`, Horizon CLI `horizon catalog get` includes the template.

### Checklist

- [ ] Chart copied; `Chart.yaml` `name` unique
- [ ] Child Application `spec.source.path` values point at `gitops/modules/example-module/...`
- [ ] Child labels include `horizon-sdv.io/module-manager-managed: "true"`
- [ ] Overview Service name/namespace match the catalog
- [ ] WorkflowTemplate names unique; Sensor filter matches; `expose` set only if the template should be submittable
- [ ] `ModuleCatalog` entry added and synced
- [ ] Enable in portal reaches READY

## Workflows via Developer Portal

This walk-through runs the `sample` module's workflows. The screens are described in full under [Module pages](02_horizon-portal.md#module-pages) in the Horizon Portal guide.

### Submit

The module must be enabled and READY to run the workflow.

Open **sample** in the developer-portal sidebar and go to **Workflow Templates**. There are 2 workflows available, each with its own parameters. Click **Configure & submit** to open the form.

| Workflow | Parameters |
|---|---|
| *sample-smoke-test* | horizonSubmittedFrom\*, sampleEnv\*, sampleBuildId\*, sampleNote\*, plus `gcsBucket` and the `largeUpload*` settings, which have defaults |
| *sample-smoke-test-alt* | horizonSubmittedFrom\*, releaseChannel\*, ticketId\*, owner\* |

Fields marked `*` have no default, but the portal does not require them. Leave **horizonSubmittedFrom** empty to record the run as submitted from `developer-portal`; a value you type overrides it.

After submitting, the page switches to the **Running Workflows** tab. Click the run's row (it shows the template name) to open its details. From here you can follow the logs, download artifacts (**Download**) or open them in Cloud Storage (**GCS**).

### History

When the workflow is completed it moves to the **History** tab, with its finish time and **Result**. Finished runs are kept only for a limited time; see [Running Workflows and History](02_horizon-portal.md#running-workflows-and-history).

The run can also be opened from **Open in Argo Workflows** in its details dialog. The templates themselves are in Argo Workflows under **Workflow Templates**, namespace `workflows`.

The artifacts can also be located and downloaded from GCP "Cloud Storage" → "Buckets" → "*-argo-workflows".

### How to abort/delete workflows

Open the run from **Running Workflows** and press **Abort** in the top-right corner to stop it, or open a finished run and press **Delete** to remove it with its artifacts. Both ask for confirmation. See [Workflow details, abort and delete](02_horizon-portal.md#workflow-details-abort-and-delete).

### Hello-world application

*Hello World* application can be found in **sample → Applications** in the Developer Portal sidebar, where **Open** opens it in a new tab. It should show *SOFT_FEATURE_ENABLED_SAMPLE_SOFT* flag with a correct value:

- true — if sample-soft is enabled
- false — if sample-soft is disabled

## Workflows via Horizon CLI

See [Horizon CLI](03_horizon-cli.md).

## Expose label (catalog and portal links)

The Horizon platform controls which **WorkflowTemplates** and child applications are shown in the Developer Portal, the Horizon API catalog and the Module Manager applications list. Set the markers below in your module's chart in Git. Argo CD syncs the chart, so a change made directly on the cluster or in the Argo CD UI is overwritten on the next sync.

### 1. WorkflowTemplate visibility

To expose a WorkflowTemplate, add this label to its `metadata.labels`, next to `horizon-sdv.io/module`:

```yaml
horizon-sdv.io/expose: "true"
```

When the label is set and the module is enabled and READY, the WorkflowTemplate appears in the Horizon API catalog, the CLI, and the **Workflow Templates** tab of the Developer Portal.

To hide it, remove the label or set it to `"false"`. The WorkflowTemplate still exists in the cluster, but it can no longer be listed or submitted through the Horizon API.

Commit the change and sync the module. The catalog picks it up without a restart; refresh the portal to see it.

### 2. Exposing child applications in the portal

A child Argo CD Application appears under the module's **Applications** tab when it carries these labels and annotation:

```yaml
metadata:
  labels:
    horizon-sdv.io/module: {{ .Values.moduleName }}
    horizon-sdv.io/app-role: child
    horizon-sdv.io/expose: "true"
  annotations:
    horizon-sdv.io/portal-url: "<url>"        # required; may be site-relative, e.g. /hello-world
    horizon-sdv.io/portal-title: "Hello World" # optional display name
    horizon-sdv.io/portal-id: hello-world      # optional stable ID
```

`sample-module/templates/application-hello-world.yaml` is a working example. Module Manager merges these applications with any `applications` listed in the catalog entry.

To remove the link, remove the `horizon-sdv.io/portal-url` annotation or set the `horizon-sdv.io/expose` label to `"false"`, then commit and sync.

### 3. Visibility follows module enablement

Even with `horizon-sdv.io/expose: "true"`, a WorkflowTemplate is only accessible while its module is enabled and READY. When the module is disabled:

- The WorkflowTemplate is removed from the Developer Portal UI and the catalog
- Workflow submission requests are blocked
- Requests to its submit path return `404 Not Found`

This behavior ensures that exposed resources are only accessible when the owning module is enabled and available to users.

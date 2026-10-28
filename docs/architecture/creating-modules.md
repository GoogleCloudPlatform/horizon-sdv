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

## Table of Contents

- [Developer Portal](#developer-portal)
  - [Module manager](#module-manager)
  - [Create a new module](#create-a-new-module)
  - [Workflows via Developer Portal](#workflows-via-developer-portal)
  - [Workflows via Horizon CLI](#workflows-via-horizon-cli)
  - [Expose label (catalog and portal links)](#expose-label-catalog-and-portal-links)

## Developer Portal

### Module manager

#### Description

Developer portal is a single UI in front of Horizon API and Module Manager internal services. Users can enable and disable modules, submit workflows and navigate platform views in one UI.

**Modules:**

Each module is a separate ArgoCD Application (app-of-apps) and is created/deleted by Module Manager based on Developer Portal requests.

`https://<your-horizon-domain>/developer-portal/`

To find modules, go to **Administration → Modules**, or click **Open Administration → Modules** on the Welcome page. The modules on offer come from the Module Catalog; see [Description](horizon-portal.md#description) in the Horizon Portal guide for the current list.

#### Browsing

The sidebar always shows Welcome, Landing page, Tools and Administration. Each enabled module gets its own sidebar entry once it is READY. The Landing page, the module statuses and the module pages are described in the [Horizon Portal guide](horizon-portal.md#browsing).

**Dependencies** are how one module relies on another. **Hard** vs **soft** describes how tightly and necessarily that reliance is. For example: `sample` is dependent on `sample-hard`. If `sample` is enabled, then `sample-hard` cannot be disabled and will result in an error in developer-portal.

#### Hard dependency (`sample` → `sample-hard`)

A **hard dependency** means: **the parent cannot be correctly enabled or kept running without the dependency module**.

##### Enable behavior

When `sample` is enabled, Module Manager **recursively enables** `sample-hard` **first** (if it is not already enabled). Enable fails if a hard dependency cannot be satisfied.

##### Disable behavior

`sample-hard` cannot be disabled while `sample` is still enabled. The API returns **409** with `hardDependents: ["sample"]`. The dev portal surfaces this as *"Cannot disable: required by: sample"*.

##### Runtime / deploy

`sample-hard` must be deployed for `sample` to be valid. Its Argo workflows (`sample-hard-smoke-test`) are **not** exposed on the Horizon API catalog — they are internal infrastructure.

##### Auto-disable

`sample-hard` is auto-disabled when **no enabled module lists it as a hard dependency** (only `sample` does).
Auto-disable of modules is optional. By default - the modules do not get disabled if it has no dependency.

#### Soft dependency (`sample` → `sample-soft`)

A **soft dependency** means: **the parent can run without the dependency**, but may **enable extra behavior** when the soft module is on.

##### Enable behavior

Enabling `sample` **does not auto-enable** `sample-soft`. You turn `sample-soft` on separately if you want the optional path.

##### Disable behavior

You **can disable** `sample-soft` while `sample` **stays enabled**. There is no hard-block on disable (unlike hard deps). Parents that list a soft dep are **not** prevented from disabling the soft module.

##### Auto-disable

`sample-soft` is auto-disabled when **no enabled module depends on it** (hard or soft). While `sample` is enabled, `sample-soft` counts as a soft dependent, so it stays eligible to remain on.
Auto-disable of modules is optional. By default - the modules do not get disabled if it has no dependency.

#### How the three modules differ (summary)

|  | sample-hard | sample-soft | sample |
|---|---|---|---|
| **Dependency type** | Hard dep *target* | Soft dep *target* | Parent with both |
| **Required for sample?** | Yes | No | — |
| **Auto-enabled with sample?** | Yes | No | — |
| **Horizon-exposed workflows** | No | No | Yes (`sample-smoke-test`) |
| **When unused** | Auto-disable (no hard dependents) | Auto-disable (no hard/soft dependents) | Stays while you want the app |

#### Change git ref in developer portal

Git reference can be easily changed for the modules. It can be done from Administration tab in Modules section. After the change the Target Revision for this particular app should be aligned automatically in ArgoCD.

### Create a new module

Modules are not discovered automatically. You add a Helm chart under `gitops/modules/`, register it in the `ModuleCatalog` CR, commit to the GitOps repo, then enable it in **Administration → Modules**. Partner isolation (adding a module without changing Horizon core) is not shipped; the catalog file is still required.

Use `gitops/modules/sample-hard-module` as the copy source unless you need hard+soft dependencies (`sample-module`) or GCS (`storage-gcs-module` / `sample-data-module`).

#### What you are adding

On enable, Module Manager creates one **parent** Argo CD Application named `mod-<moduleName>` (underscores in the name become hyphens). That parent deploys your module chart. The chart usually creates **child** Applications (workflows, hello-world, and similar) plus an in-cluster nginx Service that serves overview HTML.

Module Manager injects Helm values into the parent: `moduleName`, `config` (cluster `MODULE_CONFIG`), `overviewNamespace` (from the catalog), `repo.url`, and `repo.revision`. Do not hard-code the Git repo URL or revision in a way that ignores those values.

#### 1. Choose a name

Pick a DNS-1123 token used as the catalog `name` and as `horizon-sdv.io/module` labels (for example `example`).

- Parent Application: `mod-example`
- Typical chart directory: `gitops/modules/example-module` (directory name need not match catalog name; `workloads-android` does not use a `-module` suffix)
- Overview Service name: `mod-example-overview` (must match catalog `overviewService`)

#### 2. Copy a sample chart

From the repository root:

```bash
cp -R gitops/modules/sample-hard-module gitops/modules/example-module
```

Edit `gitops/modules/example-module/Chart.yaml`: set `name` to `example-module` and update `description`.

#### 3. Rewire Helm identity and child Application paths

In `gitops/modules/example-module/values.yaml`:

- Keep `moduleName: ""` — Module Manager sets it on enable.
- Set `overviewServiceName: mod-example-overview`.
- Set `overviewNamespace` and any hello-world namespace (for example `example-module-hello`) so they match where those workloads actually run.
- Leave `repo.url` / `repo.revision` as placeholders; the parent Application fills them.

In every child Application template under `templates/` (for example `application-argo-workflows.yaml`, `application-hello-world.yaml`):

- Change `spec.source.path` from `gitops/modules/sample-hard-module/...` to `gitops/modules/example-module/...`.
- Keep labels `horizon-sdv.io/module: {{ .Values.moduleName }}`, `horizon-sdv.io/app-role: child`, and `horizon-sdv.io/module-manager-managed: "true"` so disable/drain can find and delete children.
- Keep `spec.source.repoURL` / `targetRevision` on `.Values.repo.*`.
- Destination namespaces should use `config.namespacePrefix` for platform namespaces (`workflows`, `argo-events`) and your dedicated namespace for hello-world.

Rename `helloWorldNamespace` (and the hello-world subchart resources) if you keep that app so they do not collide with `sample-hard`.

#### 4. Overview HTML and in-cluster Service

1. Edit `portal/overview.html` (title and body for **example**). Module Manager does not read this file from Git at runtime.
2. Keep `templates/module-overview-http.yaml`. It packages the HTML into a ConfigMap, runs nginx, and exposes a ClusterIP Service on port **80** path `/`.
3. Catalog `overviewService` / `overviewServiceNamespace` must match that Service name and namespace. Module Manager always GETs `http://<service>.<namespace>.svc.cluster.local:80/`.

If either catalog field is missing, **Overview** in the portal shows a configuration message and `GET /modules/{name}/overview` returns 404.

#### 5. Workflows and Argo Events sensors

Skip this if the module has no user-facing pipelines.

1. Under `argo-workflows/templates/workflowtemplates.yaml`, set `metadata.name` uniquely (cluster-wide in the workflows namespace). Set label `horizon-sdv.io/module: example`.
2. To list the template in the Developer Portal and Horizon API catalog, add `horizon-sdv.io/expose: "true"`. See [Expose label](#expose-label-catalog-and-portal-links). Internal templates (like `sample-hard-smoke-test`) omit expose.
3. Duplicate the Sensor in `argo-workflows/templates/sensors.yaml`. The EventSource filter `body.workflowTemplateName` **must** equal the WorkflowTemplate name. Map submit body fields onto Workflow parameters the same way the sample sensors do (`horizonSubmittedFrom`, your parameters). Horizon API only POSTs to the Argo Events webhook; without a matching Sensor, submit returns success from the API's point of view of dispatch but no Workflow is created.
4. Point the child Application `path` at `gitops/modules/example-module/argo-workflows` and pass `workflowNamespace` / `eventsNamespace` as in the sample.

#### 6. Register the module in ModuleCatalog

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

Required: `name`, `path` (repo-relative chart path).

Optional:

| Field | Effect |
|---|---|
| `hardDependencies` | Those modules enable first; they cannot be disabled while this module is on |
| `softDependencies` | Optional extras; parent still enables without them |
| `autoDisableWhenUnused` | Disable this module when no enabled module lists it as a hard or soft dependency |
| `applications` | Extra portal links (`id`, `url`; `url` may be site-relative `/...`) |
| `softFeaturesPropagation` / `softFeaturesConfigMapNamespaces` | How soft-dep flags reach workloads (see `sample`) |

Catalog name `example` must match labels and Sensor/template ownership. Path must be the parent chart directory, not the `argo-workflows` subchart.

#### 7. Ship, sync, and enable

1. Commit and push to the branch Argo CD syncs (`repo.revision` / platform branch).
2. Sync the **module-manager** Application so the `ModuleCatalog` CR updates. The module appears in Administration even before it is enabled.
3. In the Developer Portal: **Administration → Modules → Enable** on `example`. Wait until status is READY (parent and children healthy). Do not reload the browser while enable is in progress.
4. Confirm a sidebar tab for `example`, Overview HTML, and (if exposed) Workflow Templates.

To iterate on Git content after enable: pin or follow the platform ref as in [Change git ref in developer portal](#change-git-ref-in-developer-portal).

#### Verify

- `kubectl get modulecatalog cluster -n module-manager -o yaml` lists `example`.
- After enable: `kubectl get application -n <prefix>argocd` shows `mod-example` and children `mod-example-*`.
- Portal Overview loads; with `expose: "true"`, Horizon CLI `horizon catalog get` includes the template.

#### Checklist

- [ ] Chart copied; `Chart.yaml` `name` unique
- [ ] Child Application `spec.source.path` values point at `gitops/modules/example-module/...`
- [ ] Child labels include `horizon-sdv.io/module-manager-managed: "true"`
- [ ] Overview Service name/namespace match the catalog
- [ ] WorkflowTemplate names unique; Sensor filter matches; `expose` set only if the template should be submittable
- [ ] `ModuleCatalog` entry added and synced
- [ ] Enable in portal reaches READY

### Workflows via Developer Portal

#### Submit

The module must be enabled and READY to run the workflow.

Open **sample** in the developer-portal sidebar and go to **Workflow Templates**. There are 2 workflows available, each with its own parameters. Click **Configure & submit** to open the form.

| Workflow | Parameters |
|---|---|
| *sample-smoke-test* | horizonSubmittedFrom\*, sampleEnv\*, sampleBuildId\*, sampleNote\*, plus `gcsBucket` and the `largeUpload*` settings, which have defaults |
| *sample-smoke-test-alt* | horizonSubmittedFrom\*, releaseChannel\*, ticketId\*, owner\* |

Fields marked `*` have no default, but the portal does not require them. Leave **horizonSubmittedFrom** empty to record the run as submitted from `developer-portal`; a value you type overrides it.

After submitting, the page switches to the **Running Workflows** tab. Click the run's row (it shows the template name) to open its details. From here you can follow the logs, download artifacts (**Download**) or open them in Cloud Storage (**GCS**). See [Module pages](horizon-portal.md#module-pages) for the details dialog, logs and artifacts.

#### History

When the workflow is completed it moves to the **History** tab, with its finish time and **Result**. Runs stay for 1 day after success and 3 days after failure, after which they are removed from the cluster. The **Workflows visibility** setting under **Administration → Settings** can hide runs from both lists.

The run can also be opened from **Open in Argo Workflows** in its details dialog. The templates themselves are in Argo Workflows under **Workflow Templates**, namespace `workflows`.

The artifacts can also be located and downloaded from GCP "Cloud Storage" → "Buckets" → "*-argo-workflows".

#### How to abort/delete workflows

To abort a workflow, open the running workflow in Developer Portal, press **Abort** in the top-right corner and confirm. The run stops gracefully, so it can take a moment to finish. When it has, **Abort** is replaced by **Delete** and the status usually shows *Aborted*. The run then appears in the **History** tab.

To delete a finished workflow, open its details from **Running Workflows** or **History**, press **Delete** and confirm. Deletion is permanent and also removes the run's artifacts from Cloud Storage; the run shows *Deletion in progress* until it is gone.

#### Hello-world application

*Hello World* application can be found in **sample → Applications** in the Developer Portal sidebar, where **Open** opens it in a new tab. It should show *SOFT_FEATURE_ENABLED_SAMPLE_SOFT* flag with a correct value:

- true — if sample-soft is enabled
- false — if sample-soft is disabled

### Workflows via Horizon CLI

See [Horizon CLI](horizon-cli.md).

### Expose label (catalog and portal links)

The Horizon platform supports controlled visibility of **WorkflowTemplates** and child applications in the Developer Portal using annotations. These annotations determine whether resources are displayed in the catalog, portal navigation, and Module Manager views.

#### 1. WorkflowTemplate Visibility in the Developer Portal

WorkflowTemplates can be exposed in the Developer Portal by adding the following annotation to the WorkflowTemplate manifest:

```yaml
horizon-sdv.io/expose: "true"
```

When this annotation is enabled and the corresponding module is active, the WorkflowTemplate becomes visible in the **Workflow Templates** section of the Developer Portal.

If the annotation is removed or set to `"false"`, the WorkflowTemplate is hidden from the portal UI. The underlying Kubernetes or Argo resource may still exist in the cluster, but it will no longer be accessible from the portal.

##### Updating WorkflowTemplate Visibility

1. Open the WorkflowTemplate manifest in Argo Workflows or the GitOps source repository.
2. **Add** or **update** the annotation:

   ```yaml
   horizon-sdv.io/expose: "true"
   ```

3. Save the changes and sync the application.

To **hide** the WorkflowTemplate, either remove the annotation or change the value to:

```yaml
horizon-sdv.io/expose: "false"
```

After synchronization, refresh the Developer Portal to verify the visibility changes.

#### 2. Exposing Child Applications in the Portal Navigation

Child applications can be displayed in the Developer Portal navigation and Module Manager merged applications list using the following annotations:

```yaml
horizon-sdv.io/expose: "true"
horizon-sdv.io/portal-url: "<valid-url>"
```

Both annotations are required for the application link to appear in the portal.

- `horizon-sdv.io/expose` controls whether the application is eligible for portal visibility.
- `horizon-sdv.io/portal-url` defines the navigation link displayed in the portal.

##### Configuring Application Visibility

1. Open the child application in Argo CD.
2. Navigate to **Details → Annotations.**
3. Add or verify the following annotations:

   ```yaml
   horizon-sdv.io/expose: "true"
   horizon-sdv.io/portal-url: "https://<application-url>"
   ```

4. Save the changes and sync the application.

Once synchronization is complete, the application becomes visible in:

- Developer Portal navigation
- Module Manager merged applications list

##### Removing Application Visibility

To remove the application link from the portal:

- Remove `horizon-sdv.io/portal-url`
- OR set:

```yaml
horizon-sdv.io/expose: "false"
```

After synchronization and portal refresh, the application link is removed from the UI.

#### 3. Visibility Restrictions Based on Module Enablement

WorkflowTemplate visibility also depends on module or soft-feature availability.

Even when the following annotation is configured:

```yaml
horizon-sdv.io/expose: "true"
```

the WorkflowTemplate is only accessible if the related module or feature is enabled and in a READY state.

If the module or feature is disabled:

- The WorkflowTemplate is removed from the Developer Portal UI
- Workflow submission requests are blocked
- Access attempts may return responses such as: `404 Not Found`

This behavior ensures that exposed resources are only accessible when the owning module or feature is enabled and available to users.

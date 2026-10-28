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

# Modularization Architecture

This page describes how the module components interact. For the concepts (modules, hard and soft dependencies, pinning), start with [Horizon modules overview](01_overview.md). The endpoints and the soft-feature contract are listed in [Interfaces](06_interfaces.md).

## Table of Contents

- [Component Overview](#component-overview)
- [Key Mechanics](#key-mechanics)
- [Enabling a module](#enabling-a-module)
- [API call and workflow run](#api-call-and-workflow-run)
- [Live workflow output streaming](#live-workflow-output-streaming)
  - [Security of the streaming path](#security-of-the-streaming-path)
- [Pinning and Resetting module Git refs](#pinning-and-resetting-module-git-refs)
- [Disabling a module](#disabling-a-module)
- [Tearing down the platform](#tearing-down-the-platform)

## Component Overview

The platform is composed of:

- **Developer Portal**, a Go proxy plus an SPA. It proxies user actions to Module Manager (enable/disable, module list) and to the Horizon API (catalog, workflow submit, logs).
- **Module Manager**, an API and a Kubernetes controller.
- **Horizon API**, also used directly by the `horizon` CLI.
- **Supporting services**: Argo CD, Argo Events, Argo Workflows, Keycloak and GCS.

All requests are authenticated through Keycloak.

```mermaid
flowchart TB
    user([User / browser])
    cli([horizon CLI])
    subgraph portal[Developer Portal]
        spa[SPA UI] --> proxy[Go proxy]
    end
    keycloak[(Keycloak / OIDC)]
    api["Horizon API<br/>/api/v1 :8082"]
    mm["Module Manager<br/>API :8082 + controller"]
    subgraph crds[Module Manager CRDs]
        state["ModuleManagerState CR<br/>enabledModules, revisions"]
        catalog["ModuleCatalog CR<br/>modules, deps, apps"]
    end
    events["Argo Events<br/>EventSource + Sensor"]
    wf["Argo Workflows<br/>WorkflowTemplates + Workflows"]
    argocd["Argo CD<br/>app-of-apps"]
    git[("GitOps repo<br/>gitops/modules/*")]
    gcs[(GCS artifact bucket)]

    user --> spa
    proxy -- token --> keycloak
    proxy -- "catalog, submit, logs" --> api
    cli --> api
    proxy -- "enable/disable, list" --> mm
    api -- "reads enabled modules" --> state
    api -- "webhook POST" --> events
    events -- "create Workflow" --> wf
    api -- "logs / status" --> wf
    api -- "signed URLs" --> gcs
    wf -- artifacts --> gcs
    mm -- "updates state" --> state
    mm -- "reads catalog" --> catalog
    mm -- "create/delete Application" --> argocd
    argocd -- sync --> git
    argocd -- "deploy module workloads" --> wf
```

## Key Mechanics

- **Module Manager is the source of truth** via two CRs: `ModuleCatalog` (what exists plus dependencies and apps) and `ModuleManagerState` (`status.enabledModules`, `moduleIds`, `moduleTargetRevisions`, `workflowsVisibility`).
- **Enable/disable is realized through Argo CD.** Module Manager creates or deletes one parent Argo CD Application per module; Argo CD then deploys or prunes the module's workloads from `gitops/modules/*`.
- **Visibility is recomputed, never static.** The Horizon API builds the catalog from enabled modules plus the `horizon-sdv.io/expose` label, and a resource must also be `READY`. Disable the module and the same endpoints return `404` or block the request.
- **Workflow submission is decoupled.** The Horizon API only POSTs to the Argo Events webhook; the Sensor materializes the Workflow, so the API stays stateless about execution and just reads back status, logs, and artifacts.
- **Log streaming is relayed, not stored by the API.** The Horizon API proxies a live stream from the Argo Workflows server while pods run. Once the pods are gone, the archived `main.log` in GCS is the source. The portal and Horizon API backends raise the load-balancer timeout (`timeoutSec: 3600`) to keep these long-lived streams open.

## Enabling a module

Enabling a module in the portal's Administration tab sends a request to Module Manager, which then:

1. Resolves hard dependencies first.
2. Updates the `ModuleManagerState` CR.
3. Creates the module's parent Argo CD Application, `mod-<module>`.

The parent deploys the module chart, which usually creates child Applications for the module's workloads. The module's tab appears in the portal once the workloads are `READY`, and its workflow templates once they also carry the `horizon-sdv.io/expose` label.

```mermaid
sequenceDiagram
    participant User
    participant Proxy as Portal proxy
    participant MM as Module Manager
    participant CRs as State/Catalog CRs
    participant ArgoCD as Argo CD
    participant WL as Module workloads

    User->>Proxy: Enable module X
    Proxy->>MM: POST /modules/X/enable (Keycloak token)
    MM->>CRs: Read catalog (dependencies of X)
    Note over MM,CRs: Resolve hard dependencies first<br/>(recursively enable required modules)
    MM->>CRs: Add X and its hard dependencies to enabledModules,<br/>assign moduleId and target revision
    MM->>ArgoCD: Create parent Application mod-X<br/>(gitops/modules/X at revision)
    MM-->>Proxy: 200 enabled
    ArgoCD->>WL: Sync and deploy (child Applications, WorkflowTemplates)
    WL-->>ArgoCD: Healthy / READY
    Proxy-->>User: Module tab visible
    Note over User,Proxy: The tab appears once workloads are READY.<br/>Templates also need the horizon-sdv.io/expose label.
```

## API call and workflow run

1. When a user opens a module's Workflow Templates, the Horizon API recomputes the catalog from the enabled modules and the `expose` label, returning only visible templates.
2. On submit, the Horizon API validates the OIDC token and the module/template state, then POSTs an event to the Argo Events webhook.
3. The Argo Events Sensor creates a Workflow from the template, Argo Workflows runs it, and artifacts land in GCS.
4. Status, logs, and signed download URLs are read back through the Horizon API.

```mermaid
sequenceDiagram
    participant User
    participant Proxy as Portal proxy
    participant API as Horizon API
    participant SC as State + Catalog
    participant Events as Argo Events
    participant WF as Argo Workflows
    participant GCS as GCS bucket

    User->>Proxy: Open Workflow Templates
    Proxy->>API: GET /api/v1/catalog (token)
    API->>SC: Read enabledModules
    Note over API: Keep templates with the expose label
    API-->>Proxy: Only exposed templates of enabled modules
    User->>Proxy: Submit workflow + parameters
    Proxy->>API: POST submit (X-Horizon-Submitted-From)
    Note over API: Validate OIDC token<br/>Check module enabled and template exposed
    API->>Events: Webhook POST (event)
    Events->>WF: Sensor creates Workflow from template
    WF->>WF: Run pods (workflows namespace)
    WF->>GCS: Upload artifacts and archived logs
    User->>Proxy: View Running / History / logs
    Proxy->>API: GET running and history, stream logs
    API->>WF: Read status + stream logs
    API->>GCS: Generate signed artifact URLs
    API-->>Proxy: Status, logs, download links
```

## Live workflow output streaming

Once a workflow is running, the portal streams its logs live rather than polling for a final blob:

1. The SPA opens a long-lived streaming request, which the Go proxy forwards to the Horizon API.
2. The Horizon API connects to the Argo Workflows server (`argo-workflows-server.<ns>.svc.cluster.local:2746`, set via `--argo-base-url`).
3. The Horizon API relays log lines back through the proxy to the browser as they are produced.

The stream is newline-delimited JSON:

- Each line carries stage, timestamp, and message.
- Heartbeat lines keep the connection alive.
- The stream terminates with an explicit end marker when the workflow finishes.

The source of the logs depends on pod lifecycle:

- **While pods are running**, logs come directly from the Argo Workflows server's in-cluster log stream (near real time).
- **After pods are garbage-collected** (podGC) or the workflow has completed, the in-cluster stream is empty, so logs are served from the archived `main.log` that Argo uploads to the GCS artifact repository (`archiveLogs: true`). The portal loads each step's archived log through the Horizon API's artifact download endpoint.

Because these are long-lived connections, the default GKE load-balancer backend timeout (~15s) would cut the stream with a `504`. The portal and Horizon API backends therefore each set a `GCPBackendPolicy` with `timeoutSec: 3600` so catalog calls and log streams stay open.

```mermaid
sequenceDiagram
    participant User
    participant SPA as Portal SPA
    participant Proxy as Portal Go proxy
    participant API as Horizon API
    participant Argo as Argo Workflows server (:2746)
    participant GCS as GCS artifact repo

    User->>SPA: Open running workflow logs
    alt Pods still running
        SPA->>Proxy: Open streaming request (token)
        Proxy->>API: Forward stream (long-lived, timeoutSec 3600)
        API->>Argo: Connect to in-cluster log stream
        loop While workflow active
            Argo-->>API: Log line
            API-->>Proxy: NDJSON line (stage, timestamp, message) or heartbeat
            Proxy-->>SPA: Append line (live)
        end
        API-->>SPA: End-of-stream marker on completion
    else Completed / pods garbage-collected
        SPA->>Proxy: Download each step's main.log
        Proxy->>API: Artifact download endpoint
        API->>GCS: Read archived main.log
        GCS-->>API: Historical log content
        API-->>SPA: Archived log
    end
```

### Security of the streaming path

The live-stream path is protected by several layers, all rooted in Keycloak/OIDC plus Kubernetes controls.

1. **Browser to Portal (edge auth + TLS).** The SPA requires a Keycloak login (public client `horizon-dev-portal`). TLS is terminated at the GKE Gateway (`secure: false` internally means TLS is handled at the edge, not disabled end-to-user).
2. **Portal Go proxy to Horizon API (confidential client).** The proxy authenticates to the Horizon API using a confidential Keycloak client (`horizon-api-ci`), with `HORIZON_API_CI_CLIENT_SECRET` injected from a Secret. The streaming request reaching the API carries a valid bearer token.
3. **Horizon API (OIDC validation).** The API validates tokens against Keycloak via `--oidc-issuer-url` (signature, issuer, expiry). Audience/client-id enforcement is optional and off by default (`oidcSkipClientIdCheck: true`, `oidcClientId: ""`); unless configured, it does not restrict `aud`/`azp`, only that the token is a valid Keycloak-issued one.
4. **Horizon API to Argo Workflows server (SA bearer + RBAC).** The Argo server runs with `--auth-mode=client` (SSO for humans). The Horizon API connects using its ServiceAccount bearer token, scoped by a Role in the workflows namespace that grants `argoproj.io` workflows, `pods` and `pods/log` (`horizon-api/templates/rbac.yaml`). Its ClusterRole (`rbac-cluster.yaml`) is read-only, for the module CRs and workflow templates. The log API is authenticated and authorized, not open.
5. **Network isolation.** The portal namespace has `default-deny-all` with explicit allows: DNS, HTTPS and HTTP egress (excluding the metadata server), Module Manager on `8082`, and the `horizon-api` namespace. The Argo server only accepts ingress on `2746`. These NetworkPolicies are applied when `enableNetworkPolicies` is true, which is the default.
6. **Archived logs (completed workflows).** Fallback historical logs live in the GCS artifact bucket, reached via Workload Identity (Google IAM), and artifacts are handed out as short-lived signed URLs, not public objects.

## Pinning and Resetting module Git refs

Each module's workloads are deployed by an Argo CD Application whose `targetRevision` decides which Git ref (branch, tag, or SHA) is synced. Module Manager tracks these per module and lets you either follow the platform branch or pin a module to a specific ref from **Administration → Modules**.

**Where the ref lives.** Module Manager starts with `--target-revision` (from `repo.revision`, i.e. the platform branch / `scm_repo_branch`). Per-module overrides are recorded in `ModuleManagerState.status.moduleTargetRevisions` (a map of module name → Git ref). The effective ref written to each module's parent Argo CD Application is: the per-module pin if one exists, otherwise the platform ref.

**Pin to specific ref.** Changing a module's Git ref in the portal:

1. Writes that ref into `moduleTargetRevisions[module]`.
2. Updates the module's parent Argo CD Application (`mod-<module>`) to match: both its `targetRevision` and the `repo.revision` value it passes to the chart, so the child Applications follow.
3. Argo CD re-syncs the module to the chosen ref.

The module now shows **Pinned**, and, importantly, the startup auto-sync **skips pinned modules**, so a pinned module will not be moved off its ref by a later platform upgrade. For a disabled module, **Pin ref before install** passes the ref with the enable request.

**Reset to platform branch.** Resetting a module in the portal:

1. Clears the per-module entry from `moduleTargetRevisions`, so the module falls back to the platform `--target-revision`.
2. Sets the parent Application back to the platform ref.
3. Argo CD re-syncs.

The module then shows **Following platform**.

**Upgrade behavior.** When `module-manager` restarts (e.g. after a platform upgrade ships new charts/images), its startup auto-sync re-aligns every module that is **Following platform** to the new platform ref, and leaves **Pinned** modules untouched. (Legacy note: modules enabled under 4.0.0 recorded a ref per enable, which 4.1.0+ reads as an explicit pin, so after an in-place upgrade they appear **Pinned** on the old ref until you reset them.)

```mermaid
flowchart TD
    start["Module in Administration → Modules"] --> action{Action}
    action -- "Pin to specific ref" --> pin1["Write moduleTargetRevisions[module] = ref"]
    pin1 --> pin2["Set parent App mod-module<br/>targetRevision and repo.revision = ref"]
    pin2 --> pin3[Argo CD syncs module to ref]
    pin3 --> pin4["Status: Pinned<br/>startup auto-sync skips it"]
    action -- "Reset to platform branch" --> reset1[Remove moduleTargetRevisions entry]
    reset1 --> reset2["Set parent App mod-module<br/>targetRevision and repo.revision = platform ref (--target-revision)"]
    reset2 --> reset3[Argo CD syncs module to platform ref]
    reset3 --> reset4["Status: Following platform<br/>auto-aligned on module-manager restart"]
```

## Disabling a module

Disabling a module is guarded by hard-dependency checks: if the module is still required by an enabled parent, the Module Manager returns `409` with the blocking modules in `hardDependents`, and the portal shows "Cannot disable: required by: ...". When disable is allowed:

1. The module is removed from `enabledModules`.
2. Its parent Argo CD Application is deleted.
3. Argo CD cascades the deletion to the child Applications and prunes the workloads.

For `workloads-android`, `workloads-common` and `remotive-topology`, Module Manager also tears down the child Application and waits for their Config Connector resources to be removed.

Afterwards:

- The Horizon API drops the module's templates from the catalog.
- Submissions are blocked, and previously exposed paths return `404`.
- The module's tab disappears from the portal.

`cloud-workstations` is drained before anything else changes, because only its controller can delete the Google Cloud resources behind it:

1. Module Manager pauses the module's core Application, `mod-cloud-workstations-core`, so Argo CD doesn't recreate what is being deleted.
2. It deletes the workstations, then the workstation configurations, then the workstation cluster. Before each next step it waits up to 20 minutes until both the Horizon objects and their Config Connector objects are gone.
3. After a successful drain, the disable continues as above.
4. Once the parent Application is gone, Module Manager deletes the module's three CRDs.

If a step of the drain times out, the disable fails and nothing else changes: the module stays enabled, the core Application resumes, and you can retry.

If a module sets `autoDisableWhenUnused`, Module Manager checks after every enable and disable whether the last enabled module that depends on it (hard or soft) has gone, and disables it if so.

```mermaid
sequenceDiagram
    participant User
    participant Proxy as Portal proxy
    participant MM as Module Manager
    participant CRs as State/Catalog CRs
    participant ArgoCD as Argo CD
    participant API as Horizon API

    User->>Proxy: Disable module X
    Proxy->>MM: DELETE /modules/X/disable
    MM->>CRs: Check hard dependents of X
    alt X still required by an enabled module
        MM-->>Proxy: 409 hardDependents: [parent]
        Proxy-->>User: Cannot disable: required by: parent
    else Disable allowed
        opt X is cloud-workstations
            Note over MM,ArgoCD: Pause core Application, delete workstations,<br/>then configurations, then the cluster.<br/>A timeout fails the disable and X stays enabled.
        end
        MM->>CRs: Remove X from enabledModules
        MM->>ArgoCD: Delete parent Application mod-X
        ArgoCD->>ArgoCD: Cascade to child Applications, prune workloads
        Note over MM: Optional auto-disable of now-unused modules
        MM-->>Proxy: 200 disabled
    end
    API->>CRs: Read enabledModules (X absent)
    Note over Proxy,API: Templates dropped from catalog<br/>Submissions blocked, exposed paths return 404<br/>Module tab removed from portal
```

## Tearing down the platform

`terraform destroy` disables every enabled module before the rest of the platform goes:

1. When the root Argo CD Application is deleted, Argo CD removes its resources in reverse sync-wave order and waits for each wave to be gone before starting the next.
2. The `platform-drain-gate` ConfigMap (`gitops/templates/platform-drain-gate.yaml`) sits in the highest wave and carries a finalizer, so it is deleted first and holds back everything else.
3. While Argo CD, Config Connector and the module controllers are still running, Module Manager disables the enabled modules one at a time, each time picking a module that no other enabled module hard-depends on. Each module is disabled the same way as from the portal, including the Cloud Workstations drain.
4. Module Manager removes the gate's finalizer, and Argo CD deletes the rest.

While the root Application is being deleted, Module Manager refuses new enables with `409`. Terraform waits up to 120 minutes for the root Application to go. If the teardown is stuck, check the `module-manager` logs; the [deployment guide](../deployment_guide.md) gives the command that releases the gate by hand.

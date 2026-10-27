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

# Cuttlefish Instance Template Pipeline

## Table of contents
- [Introduction](#introduction)
- [Packer and Startup Files](#packer-and-startup-files)
- [Packer version pinning](#packer-version-pinning)
- [Orphan Packer boot disks (zonal cleanup)](#orphan-packer-boot-disks-zonal-cleanup)
- [Prerequisites](#prerequisites)
- [Config Connector mode (`ConfigConnectorContext`)](#config-connector-mode-configconnectorcontext)
- [Portal / Module Manager disable (`workloads-android`)](#portal--module-manager-disable-workloads-android)
- [Module disable vs Packer disks (not removed by disable)](#module-disable-vs-packer-disks-not-removed-by-disable)
- [KCC RBAC: Argo workflow pods vs publisherIdentity](#kcc-rbac-workflow-pods-vs-publisheridentity)
- [Environment Variables/Parameters](#environment-variables)
- [Private repo and branch (`horizon/main`)](#private-repo-and-branch-eg-horizonmain--artifacts-and-jenkins-gce)
- [Example Usage](#examples)
- [System variables (Jenkins)](#system-variables)

## Introduction <a name="introduction"></a>

This pipeline creates or deletes **x86_64** and **ARM64** Cuttlefish **Google Compute Engine (GCE)** instance templates. Those templates back **Jenkins** GCE test agents and related flows; the same scripts also run under **Argo Workflows** (see the `cf-instance-template` Helm chart). Resulting **virtual machines (VMs)** are Cuttlefish-ready for the **Compatibility Test Suite (CTS)** and **Cuttlefish Virtual Device (CVD)** workflows.

**Machine type:** use a standard **`MACHINE_TYPE`**, or leave it empty and set the custom fields:

- `CUSTOM_VM_TYPE`
- `CUSTOM_CPUS`
- `CUSTOM_MEMORY`

**Artifacts:** Packer bakes a **disk image**; the **instance template** is applied as a **`ComputeInstanceTemplate`** custom resource via **Config Connector (KCC)**, not `gcloud compute instance-templates create`.

**Two different max-run clocks (do not confuse them):** **`MAX_RUN_DURATION`** applies to **VMs created from the published Cuttlefish instance template** (Jenkins GCE test agents, long-lived). It defaults to **`12h`** in **`cf_create_instance_template.sh`**, Jenkins **`job.groovy` / `job_arm.groovy`**, and Argo **`cf-instance-template`** workflow defaults (`maxRunDuration`). **`PACKER_BUILD_MAX_RUN_DURATION`** applies **only** to the **ephemeral Packer builder VM** used for the image bake; it defaults to **`4h`** when unset and does **not** inherit **`MAX_RUN_DURATION`**. Changing one does **not** change the other unless you set both explicitly to the same value.

**Example resource names** (typical defaults):

| Role | Example name |
|------|----------------|
| Logical / VM prefix | `cuttlefish-vm-main` |
| Disk image | `image-cuttlefish-vm-main` |
| GCE instance template | `instance-template-cuttlefish-vm-main` |

**Inspect in Google Cloud Platform (GCP) and Kubernetes:**

```bash
gcloud compute instance-templates list | grep cuttlefish-vm
gcloud compute instances list | grep cuttlefish-vm
kubectl get computeinstancetemplates.compute.cnrm.cloud.google.com -n <namespacePrefix>workflows
```

**Concurrency:** do not run two publishes of this pipeline at once; they share transient Packer and naming state and will clash.

### Pipeline execution stages

The script command interface uses three primary stages:

- `1`: Build image with Packer and create/update the instance template **via KCC** (applies a `ComputeInstanceTemplate`).
- `2`: Refresh SSH key metadata on a **new** instance template revision (no Packer image rebuild; template resource is recreated with updated `jenkins-authorized-key` / related metadata). Before applying, **`cf_compute_rest.py get-global-image`** (metadata token) asserts the Packer disk image still exists in GCP.
- `3`: Delete generated artifacts: **`kubectl delete`** the KCC `ComputeInstanceTemplate` for this target, then remove the global disk image and instances (**Representational State Transfer (REST)** **`DELETE`** on the image with a metadata-server token when available; **`gcloud compute images delete`** fallback).

### References <a name="references"></a>

- [Cuttlefish Virtual Devices](https://source.android.com/docs/devices/cuttlefish) for use with [Compatibility Test Suite (CTS)](https://source.android.com/docs/compatibility/cts) and emulators.
- [Virtual Device for Android host-side utilities](https://github.com/google/android-cuttlefish)
- [Compatibility Test Suite downloads](https://source.android.com/docs/compatibility/cts/downloads)
- [Compute Instance Templates](https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create)

## Packer and Startup Files <a name="packer-and-startup-files"></a>

The Cuttlefish image/template flow uses the following files together:

- `workloads/android/pipelines/environment/cf_instance_template/packer/cuttlefish.pkr.hcl`
  - Main Packer template.
  - Defines the temporary build VM source image/machine/network.
  - Copies provisioning scripts and runs provisioning steps.
  - Produces the final GCE disk image used by the CF instance template.

- `workloads/android/pipelines/environment/cf_instance_template/cf_create_instance_template.sh`
  - Orchestrates **`packer init` / `packer build`**, instance template creation, and optional delete/SSH-refresh stages.
  - Immediately before **`packer init`**, removes **all OS Login SSH keys** for the **current** workload identity (**metadata** OAuth token with **cloud-platform** scope—the same token used for Compute REST) via **`cf_compute_rest.py prune-os-login-ssh-keys`**, so the **32 KiB** OS Login profile limit does not block Packer’s temporary key import.
  - After each **`packer build`** (success or failure) and on **SIGINT**, best-effort cleanup of **orphan** zonal **`packer-*`** boot disks via **`cf_compute_rest.py`** and a metadata-server token (see [Orphan Packer boot disks](#orphan-packer-boot-disks-zonal-cleanup)). **Abrupt stop/terminate** (workflow kill, pod **`SIGKILL`**, eviction) may skip this path—see the **Warning** in that section.

- `workloads/android/pipelines/environment/cf_instance_template/packer/provision_cf_host.sh`
  - Script executed by Packer on the temporary build VM.
  - Calls `cf_host_initialise.sh` to install/configure Cuttlefish host tooling.
  - Ensures default user SSH bootstrap content is present during image bake.

- `workloads/android/pipelines/environment/cf_instance_template/cf_host_initialise.sh`
  - Performs host-side setup used by the Packer provisioning phase.
  - Installs dependencies, builds/install Cuttlefish packages, and prepares host runtime.
  - Installs **Google Cloud Ops Agent** with a **systemd_journald** receiver so ephemeral CVD/CTS can use **`spec.guestLogSink: cloud`** (see [cvd_argo_gce/README.md](../../../../workloads/android/pipelines/tests/cvd_argo_gce/README.md)). **Live Argo logs default to serial** port 2; Ops Agent is not required for serial-only runs. Rebuild instance templates after changing Ops Agent config.

- `workloads/android/pipelines/environment/cf_instance_template/helm/files/refresh_authorized_keys.sh` (read from the Horizon checkout at publish time)
  - Runtime startup script attached via instance template metadata.
  - Runs when a VM boots from the template and rewrites **`authorized_keys` for the VM login user** (see below).
  - Reads the public key from instance metadata attribute `jenkins-authorized-key` on every boot.
  - Reads the target account from instance metadata attribute **`jenkins-user`** (historical name). If `jenkins-user` is empty, the script defaults to `jenkins`.
  - The file updated is always **`/home/<jenkins-user value>/.ssh/authorized_keys`** — **not** necessarily `/home/jenkins/...` if the template was built with a different `DEFAULT_USER`.

**VM SSH user (Cuttlefish GCE agent) vs Docker image user:** Jenkins **configuration as code (CasC)** (`values-jenkins.yaml` / `jenkins-init.yaml`) configures the GCE plugin to SSH as **`jenkins`** with `remoteFs` `/home/jenkins`. The Packer/template pipeline therefore defaults **`DEFAULT_USER` to `jenkins`** in `cf_create_instance_template.sh` so the baked image, instance metadata (`jenkins-user`), and Jenkins agree. That is **independent** of the non-root user in the **Android Automotive OS (AAOS)** **Docker** image (`docker_image_template/Dockerfile`, typically `builder`), which only applies inside Kubernetes build pods.

In short: **Packer files create the immutable image**, while the **startup script updates SSH key material at boot** so key rotation does not require rebuilding the image.

**Source of truth for the startup script:** `workloads/android/pipelines/environment/cf_instance_template/helm/files/refresh_authorized_keys.sh`. The pipeline embeds that file’s content into the KCC `ComputeInstanceTemplate` **`metadataStartupScript`** field.

**SSH key rotation:** set `UPDATE_SSH_AUTHORIZED_KEYS=true` to republish metadata (including `jenkins-authorized-key`) **without** a Packer bake. The implementation **recreates** the instance template resource; it is not an in-place metadata patch on the GCP **application programming interface (API)** object.

## Packer version pinning <a name="packer-version-pinning"></a>

Two independent versions are pinned in different places. Changing one does not affect the other.

| What | Pinned where | How to change | Reported in job log |
| --- | --- | --- | --- |
| **Packer binary** | `docker_image_template/Dockerfile`, `ARG PACKER_VERSION` (default `1.15.0`, installed from the HashiCorp **advanced package tool (APT)** repo) | Rebuild the builder image with a different `PACKER_VERSION` build arg | `PACKER_VERSION=Packer v1.15.0` |
| **`googlecompute` plugin** | `PACKER_GOOGLECOMPUTE_VERSION_MIN` / `PACKER_GOOGLECOMPUTE_VERSION_MAX`. Script defaults in `cf_create_instance_template.sh`, Helm defaults in `helm/values.yaml` | Set it per run on either interface. **Jenkins:** job parameter. **Argo:** workflow parameter `packerGooglecomputeVersionMin` / `packerGooglecomputeVersionMax` on the submit form; `helm/values.yaml` only sets the default that form starts with | `PACKER_GOOGLECOMPUTE_VERSION_MIN=…` / `…_MAX=…` |

**Defaults live in two places:** the script fallback in `cf_create_instance_template.sh` and the Argo submit-form default in `helm/values.yaml`. Keep them aligned when you change the product default; if they drift, which one applies depends on how the run was started.

The plugin bounds are **not** declared in `packer/cuttlefish.pkr.hcl`. Packer resolves `required_plugins` during **`packer init`**, before variables are evaluated, so the constraint cannot be an ordinary **HashiCorp configuration language (HCL)** variable. Instead `generate_packer_plugins_fragment()` renders a `plugins.pkr.hcl` fragment into the writable work directory next to a copy of the template, and Packer runs against that directory.

### Pin policy <a name="pin-policy"></a>

Current constraint: **`>= 1.2.3, < 1.2.5`**.

- **Lower bound `>= 1.2.3`** — required for `max_run_duration_in_seconds` and `instance_termination_action` on the ephemeral builder **virtual machine (VM)**. Do **not** lower it; without those fields a stuck build leaks a running VM.
- **Upper bound `< 1.2.5`** — **workaround, not a preference.** On Argo (read-only `/workspace` working directory) plugin **v1.2.5** dies with **SIGSEGV** in `StepImportOSLoginSSHKey`; Packer then reports `unexpected EOF` and the workflow fails at stage 1.
- The registry only ships a **1.2.x** line for `github.com/hashicorp/googlecompute` — there is no 1.5.x line. An earlier `>= 1.5.0` constraint failed with *no matching version*.

**When to revisit:** on each new `googlecompute` release **>= 1.2.5**. Re-test by raising `PACKER_GOOGLECOMPUTE_VERSION_MAX` on a single Argo run (`cf-instance-template-x86`) and checking that stage 1 completes. It is a parameter on both interfaces, so the test run needs no merge and no redeploy. If the run fails, restore the default. Remove the cap only after a successful **Argo** run. A Jenkins-only validation is insufficient because the crash is specific to Argo's read-only working directory.

**Do not** re-introduce a hard-coded `required_plugins` block in the HCL; it would silently override the parameters.

## Orphan Packer boot disks (zonal cleanup) <a name="orphan-packer-boot-disks-zonal-cleanup"></a>

The Packer **googlecompute** builder typically creates a **zonal** boot disk whose name starts with **`packer-`**. The plugin normally deletes that disk after the image is created. If the build **fails**, the **client disconnects**, or the job is **interrupted**, the disk can remain **unattached** in **`ZONE`** and accrue cost.

**Warning — stop, terminate, or hard kill:** The risk is the **ephemeral Packer builder resources in GCP**, not the Kubernetes pod label: Packer’s **googlecompute** builder creates a **temporary zonal VM** and a **zonal boot disk** (name typically **`packer-*`**). If an operator **stops** or **terminates** the CI run (Jenkins **Stop build**, Argo **Terminate** workflow, **`kubectl delete pod`**, node **drain/eviction**, **`SIGKILL`**, or similar), the **shell in the build pod** may exit **without** running **post-`packer build`** disk cleanup or the **`SIGINT`** handler. The **temporary VM** may be stopped by the platform; the **`packer-*` disk** can still be left **unattached** in **`PROJECT`** / **`ZONE`** and accrue cost. The pipeline’s best-effort cleanup does **not** cover every abrupt shutdown path. **Manually delete leftover `packer-*` disks** (Google Cloud console **Compute Engine → Disks**, **`gcloud compute disks delete`**, or the script’s **`orphan-disks`** path when **`PROJECT`**, **`ZONE`**, and token are valid—see **`cf_create_instance_template.sh` `orphan-disks`** and **`cf_compute_rest.py cleanup-orphan-packer-disks`** below).

**Packer builder max runtime (cost guard):** The ephemeral **Packer builder VM** is configured with **`max_run_duration`** (GCE **limit-vm-runtime** / auto-delete). `cf_create_instance_template.sh` passes **`packer_max_run_duration_seconds`** from **`PACKER_BUILD_MAX_RUN_DURATION`**, which defaults to **`4h`** when unset (it does **not** inherit **`MAX_RUN_DURATION`**). If that value parses to **0**, the script uses **`4h`** for the builder. **`MAX_RUN_DURATION`** (default **`12h`** for **Cuttlefish instances from the template**—Jenkins / Argo aligned) is **only** for the published instance template, not the Packer builder. See the script header and **`packer_max_run_duration_seconds`** in **`packer/cuttlefish.pkr.hcl`**.

`cf_create_instance_template.sh` calls **`cf_compute_rest.py cleanup-orphan-packer-disks`** (Compute Engine **v1 REST**, same **`CF_COMPUTE_REST_TOKEN`** pattern as global image / zonal instance helpers):

- **When:** immediately after **`packer build`** returns (any exit code), and from the **SIGINT** handler using the same boot **`disk_size_gb`** as the Packer template.
- **What is removed:** disks in **`PROJECT`** / **`ZONE`** where the name matches **`^packer-`**, **`users`** is empty (nothing is attached), and **`sizeGb`** equals the configured Packer boot disk size—so only typical orphan builder disks are targeted, not arbitrary data disks.

**IAM:** the workload identity (or other identity) that supplies the metadata **`cloud-platform`** token must be allowed **`compute.disks.list`** and **`compute.disks.delete`** on that zone (in addition to permissions already required for images, templates, and instances). If the token is missing (for example local runs without the GCE metadata server), cleanup logs a warning and skips.

**Module disable:** Disabling **`workloads-android`** does **not** remove **Packer** zonal **`packer-*`** disks; it only tears down **Kubernetes / KCC** resources under **`{namespacePrefix}workflows`**. Orphan builder disks still need this pipeline’s post-**`packer build`** cleanup, **`orphan-disks`**, **manual** zonal deletes, or a **separate audit job**—see [Module disable vs Packer disks](#module-disable-vs-packer-disks-not-removed-by-disable).

## Prerequisites <a name="prerequisites"></a>

- **Docker image template:** run **Android Workflows → Environment → Docker Image Template** once so the **Android Automotive OS (AAOS)** builder image this job expects exists.
- **GCE provisioning delay:** `gitops/workloads/values-jenkins.yaml` sets **`noDelayProvisioning: false`** so Jenkins does not start many VMs at once (cost control). Expect slightly slower VM availability when agents scale up.

## Config Connector mode (`ConfigConnectorContext`) <a name="config-connector-mode-configconnectorcontext"></a>

Horizon platform GitOps installs Config Connector in **cluster mode** (`gitops/templates/config-connector.yaml`, `spec.mode: cluster`). In that mode the operator **rejects** per-namespace **`ConfigConnectorContext` (CCC)** (`…does not serve any purpose and should be removed`), which previously left the **`workloads-android`** Argo Application **Degraded**.

The **`cf-instance-template`** chart therefore gates CCC behind Helm value **`kcc.instanceTemplates.createConfigConnectorContext`**:

| Value | When to use |
|-------|-------------|
| **`false`** (default) | Horizon / **cluster-mode** KCC — do **not** sync CCC. `ComputeInstanceTemplate` CRs still use the cluster-wide Config Connector. |
| **`true`** | **Namespaced-mode** Config Connector only — chart renders CCC in **`{namespacePrefix}workflows`**. |

`gitops/modules/workloads-android` sets **`createConfigConnectorContext: false`** on the CF Helm source. Set **`true`** only if your cluster runs namespaced-mode KCC and you intentionally need CCC.

## Portal / Module Manager disable (`workloads-android`) <a name="portal--module-manager-disable-workloads-android"></a>

**Problem:** Disabling **`workloads-android`** removes Helm objects in **`{namespacePrefix}workflows`**. When CCC was installed (**namespaced-mode** / `createConfigConnectorContext: true`), that includes **`ConfigConnectorContext` (CCC)**. CCC cannot finish until **every** `ComputeInstanceTemplate` **custom resource (CR)** in that namespace is gone—including other CRs created by pipeline runs (x86, arm64, or custom names), not only objects defined in static YAML. On **Horizon cluster-mode** (default), CCC is usually **absent**; PreDelete and Module Manager still clear **`ComputeInstanceTemplate`** CRs, and Module Manager’s wait for CCC absent is a no-op if none exists.

**Normal disable (Argo sync + prune):** The **`cf-instance-template`** chart installs a **PreDelete** `Job` that runs **before** CCC is pruned (when CCC is in the release). In order, it:

1. Deletes CRs labeled **`horizon-sdv.io/cuttlefish-kcc-template=true`** (the label `cf_create_instance_template.sh` sets on apply).
2. Deletes any remaining CRs whose **`metadata.name`** starts with **`cf-it-`** (pipeline naming convention).
3. Runs **`kubectl delete computeinstancetemplates … --all`** in the workflows namespace so one stray CR cannot block CCC finalization.

The hook uses a long **`argocd.argoproj.io/hook-timeout`** so slow KCC deletes are less likely to abort mid-uninstall. No manual **`kubectl`** is required when uninstall goes through this path.

**Developer Portal / Module Manager:** After the child **`workloads-android`** Argo **`Application`** CR is gone, Module Manager also deletes **all** `ComputeInstanceTemplate` CRs in **`{namespacePrefix}workflows`** and waits for **`ConfigConnectorContext`** to be absent if one remains (covers stuck uninstalls, leftover CCC from older releases, and paths where PreDelete never ran). **Packer** zonal **`packer-*`** disks are **not** part of that teardown—see [Module disable vs Packer disks](#module-disable-vs-packer-disks-not-removed-by-disable). **Global disk images** in GCP are also **not** removed by disable alone; optionally run the CF pipeline **delete** path (**`DELETE=true`** / **`delete=true`**) while **`workloads-android`** is still **enabled** if you want those images removed before disable (same subsection).

**Prefixed child Application (`{prefix}workloads-android`):** GitOps sets **`spec.syncPolicy.automated.selfHeal: false`** (with **`prune: true`**) so Argo does not start a **repair sync** while the Application is still **Deleting** under **`resources-finalizer.argocd.argoproj.io`**—a common wedge after **disable → enable → pipeline → disable** when empty **`automated: {}`** let version-dependent defaults treat drift during prune as “fix me,” surfacing as **Deleting → Sync** and stuck. Module Manager still clears **`automated`** entirely before deleting the child; parent **`mod-*`** Applications use the same explicit flags when created.

**Argo CD and ConfigConnectorContext health:** Disable/sync can sit on **waiting for healthy state** of **`core.cnrm.cloud.google.com/ConfigConnectorContext`** when Argo’s resource health script treats CCC as **Degraded** during normal CNRM churn (for example **`status.healthy: false`** after many pipeline **`ComputeInstanceTemplate`** CRs). Horizon defines **`health.lua`** for CCC under **`terraform/modules/sdv-gke-apps/argocd-values.yaml.tpl`** (into **`argocd-cm`** **`resource.customizations`**): terminating CCC (**`metadata.deletionTimestamp`** set) reports **Healthy**, and **`status.healthy: false`** without operator **`status.errors`** reports **Progressing** so prune/disable is less likely to wedge. **Redeploy** that Terraform (or your Argo Helm layer) so the cluster picks up **`argocd-cm`**. If CCC is still stuck, rely on the PreDelete path or the table below, then **`kubectl describe configconnectorcontext.core.cnrm.cloud.google.com -n <namespacePrefix>workflows`** for finalizers and **`status.errors`**.

| If this applies… | Then… |
|------------------|--------|
| CRs **predate** the label | Delete them once by hand, or republish so the label is applied; steps 2–3 still catch typical Cuttlefish names. |
| You need a blunt instrument | `kubectl delete computeinstancetemplates.compute.cnrm.cloud.google.com -n <namespacePrefix>workflows --all` is still valid for odd names. |
| The **Application** was removed **without** a prune sync | PreDelete may never run—**Module Manager** (Portal disable) still clears **`ComputeInstanceTemplate`** CRs and **CCC** after the child **`Application`** is gone; if both fail, use manual cleanup. |

### Module disable vs Packer disks (not removed by disable) <a name="module-disable-vs-packer-disks-not-removed-by-disable"></a>

**GCE instance templates:** Deleting the KCC **`ComputeInstanceTemplate`** CRs (via the [PreDelete steps](#portal--module-manager-disable-workloads-android) when Argo prunes, and/or **Module Manager** after the child Application is removed) causes Config Connector to remove the corresponding **GCE instance templates** in GCP—not only in-cluster objects.

**Global disk images (Packer-published):** That teardown removes the **instance template** in GCP; it does **not** run the CF pipeline’s **stage 3** image delete. **Global machine images** the pipeline created (for example **`image-cuttlefish-vm-*`** derived from **`ANDROID_CUTTLEFISH_REVISION`**) can **remain** in **Compute Engine → Images** and continue to incur storage cost until deleted explicitly.

**Optional — remove templates + global images before Portal disable:** While **`workloads-android`** is still **enabled** (so **`{namespacePrefix}workflows`** and the pipeline’s **kubectl** / workload identity still work; CCC only if namespaced-mode), you can run the CF instance template **delete** path for each **revision** and **architecture** you want gone. That runs **stage 3**: delete the KCC **`ComputeInstanceTemplate`** CR, then remove the **global disk image** (REST with metadata token when possible, **`gcloud compute images delete`** fallback) and related instances—see [`DELETE`](#delete) below.

| Channel | What to set | Then |
|---------|-------------|------|
| **Jenkins** | **`DELETE=true`** (and **`ANDROID_CUTTLEFISH_REVISION`** / **`CUTTLEFISH_INSTANCE_NAME`** matching the template to remove; **`UPDATE_SSH_AUTHORIZED_KEYS=false`**) | **Build** and wait for success. |
| **Argo Workflows** | Submit **`cf-instance-template-x86`** or **`cf-instance-template-arm64`** with **`delete=true`** and matching parameters | Wait for **Succeeded**. |
| **Portal** | Submit the same CF workflow with **delete** turned **on** | Wait for completion. |

Run **x86** and **arm64** (and any custom **`CUTTLEFISH_INSTANCE_NAME`**) jobs separately if you published multiple images. After these runs, **Disable workloads-android** in Portal only needs to remove chart objects (and **CCC** when it was installed)—not leftover global images you already purged. If you **skip** this step, expect global images (and **`packer-*`** zonal disks below) to remain until cleaned by other means.

**Packer zonal boot disks (`packer-*`):** Disable and the **stage 3** delete path above do **not** guarantee removal of every **zonal** disk created by the Packer **googlecompute** builder in **`PROJECT` / `ZONE`** (see [Orphan Packer boot disks](#orphan-packer-boot-disks-zonal-cleanup)). They are **not** namespaced **`workflows`** resources. After disable (or alongside the optional delete runs) you may still need to:

- **Manually** delete leftover **`packer-*`** disks (Google Cloud console **Compute Engine → Disks**, or **`gcloud compute disks delete`** in the bake zone), and/or
- Run **`./cf_create_instance_template.sh orphan-disks`** when **`PROJECT`** and **`ZONE`** (and token, where required) match the environment, and/or
- Add a **separate** scheduled or on-demand job (for example **CronJob**, **Workflow**, or ops automation) that **audits** the zone for **unattached** **`packer-*`** disks and deletes them using the same **`cf_compute_rest.py cleanup-orphan-packer-disks`** pattern and **IAM** as the pipeline.

## KCC RBAC: Argo workflow pods vs `publisherIdentity` <a name="kcc-rbac-workflow-pods-vs-publisheridentity"></a>

**Role-based access control (RBAC)** controls which identities may **`kubectl apply` / `kubectl delete`** `ComputeInstanceTemplate` in **`{namespacePrefix}workflows`**. That depends on **where the pod runs**:

| Identity | Where it is defined | Purpose |
|----------|---------------------|---------|
| **`workflow-executor`** / **`workflow-executor-elevated`** | Chart **`spec.serviceAccountName`** / **`spec.useElevatedWorkflowIam`**; RBAC in **`gitops/templates/argo-workflows-init.yaml`** | In-cluster identity when the pipeline runs as an **Argo Workflow** (namespaced **Role** + **ClusterRole** for elevated). |
| **`publisherIdentity`** (Helm) | **`kcc.instanceTemplates.publisherIdentity`** | **RoleBinding** subject only: a **service account (SA)** in **another** namespace (default **`{namespacePrefix}jenkins`**, name **`jenkins-sa`**) used e.g. by **Jenkins Kubernetes agents**. **Not** the Argo pod SA—do not point this at **`workflow-executor-elevated`** unless that SA actually exists in the namespace you set. |
| **SSH Secret namespace** | Same default namespace as publisher unless overridden | **`NAMESPACE`** env for **`cf_create_instance_template.sh`** (where the Cuttlefish SSH key Secret is read). Independent of **Google Cloud Workload Identity** on the Argo pod. |

## Environment Variables/Parameters <a name="environment-variables"></a>

Parameters for **Jenkins** jobs are defined in **`groovy/job.groovy`** and **`groovy/job_arm.groovy`**. Argo Workflows use the same script with **workflow parameters** (see the **`cf-instance-template`** Helm chart). The subsections below describe the main environment / job fields; naming may differ slightly between Jenkins **user interface (UI)** and Argo.

### `WORKFLOWS_NAMESPACE`

Kubernetes namespace where KCC **`ComputeInstanceTemplate`** objects are applied and deleted. On **namespaced-mode** KCC this must match the namespace that has **`ConfigConnectorContext`** (when `createConfigConnectorContext: true`). On **Horizon cluster-mode** there is no per-namespace CCC; templates still live in this namespace and are reconciled by the cluster-wide Config Connector. The chart’s **`cuttlefish-kcc-publisher`** **`RoleBinding`** grants the **`publisherIdentity`** ServiceAccount **create, read, update, and delete (CRUD)** permission on those **custom resources (CRs)** in **`{namespacePrefix}workflows`** (see [KCC RBAC: Argo workflow pods vs publisherIdentity](#kcc-rbac-workflow-pods-vs-publisheridentity)); the **`NAMESPACE`** env for SSH secrets defaults to **`{namespacePrefix}jenkins`** when **`publisherIdentity.namespace`** is empty. Default **`WORKFLOWS_NAMESPACE`**: **`workflows`** (prefixed: e.g. **`sdv-workflows`**).

### `COMPUTE_IMAGE_DELETE_DEBUG`

Optional environment variable read by `cf_create_instance_template.sh` (not added as a default Jenkins string parameter). Set to **`true`** on the build agent environment—for example **Inject environment variables** on the job, **configuration as code (CasC)** env for that job, or an explicit `environment { }` block in the Declarative pipeline—so the script logs the workload identity **email** (via Google **`tokeninfo`**) immediately before attempting **REST** deletion of a global disk image. Use only when diagnosing permission or identity mismatches on image delete; leave unset in normal CI to avoid extra logging and tokeninfo calls.

### `ANDROID_CUTTLEFISH_REPO_URL`

This defines which repository will be used to create the cuttlefish instance from. Users may choose to use the standard Google repository, or their own fork and revisions. This allows users to fix issues in android-cuttlefish builds from their own repository versions.

If you use a **private** repository, set **`REPO_USERNAME`** and **`REPO_PASSWORD`**.

### `ANDROID_CUTTLEFISH_REVISION`

This defines the branch/tag to use from `ANDROID_CUTTLEFISH_REPO_URL`, e.g.

- `main` - the main working branch of `android-cuttlefish`
- `v1.55.3` - the latest tagged version.
- `horizon/main` - a private repository fork of `main`
- `horizon/v1.55.3` - a private fork of tag `v1.55.3`

User may define any valid version so long as that version contains `tools/buildutils/build_packages.sh` which is a dependency for these scripts.

### `CUTTLEFISH_INSTANCE_NAME`

Optional name for a **custom** instance template (development / testing). Must match GCE naming: lowercase, regex `(?:[a-z](?:[-a-z0-9]{0,61}[a-z0-9])?)`.

**If empty:** names derive from **`ANDROID_CUTTLEFISH_REVISION`** (e.g. `cuttlefish-vm-main` → instance template **`instance-template-cuttlefish-vm-main`**, image **`image-cuttlefish-vm-main`**). Special characters in the revision are sanitized (`/` → `-`, `.` removed) for GCE.

**If set:** use a name starting with **`cuttlefish-vm`**, and add a matching **`computeEngine`** entry in Jenkins **configuration as code (CasC)** (`values-jenkins.yaml`)—copy the **`cuttlefish-vm-main`** pattern: sensible **`cloudName`**, align **`template`** self-link, then set test jobs’ **`JENKINS_GCE_CLOUD_LABEL`** to that cloud.

### `DELETE` <a name="delete"></a>

Deletes the instance template, its **global disk image**, and related VMs. Stage **`3`** removes the KCC **`ComputeInstanceTemplate`** **custom resource (CR)** first, then the image (**REST** + metadata token when possible, **`gcloud`** fallback), then instances.

| Scenario | Set these | Then |
|----------|-----------|------|
| Standard template (auto-derived name) | **`ANDROID_CUTTLEFISH_REVISION`** = version to remove, **`DELETE`** = true | **Build** |
| Custom name (see **`CUTTLEFISH_INSTANCE_NAME`**) | **`CUTTLEFISH_INSTANCE_NAME`** = same value as at create, **`DELETE`** = true | **Build** |

### `UPDATE_SSH_AUTHORIZED_KEYS`

Republishes the instance template with an updated `jenkins-authorized-key` (and related fields) using the public key derived from `SSH_PRIVATE_KEY_NAME`. The implementation **recreates** the instance template; it does **not** run Packer.

Use this when rotating the Jenkins SSH key and you need new VMs created from the template to pick up the new key without rebuilding the Packer image.

- `UPDATE_SSH_AUTHORIZED_KEYS`: set to `true`
- `DELETE`: keep `false`
- `ANDROID_CUTTLEFISH_REVISION` or `CUTTLEFISH_INSTANCE_NAME`: set to target template
- `Build`: triggers stage 2 (template republish; no image bake)

### `SSH_PRIVATE_KEY_NAME`

Jenkins credential name of the private SSH key used by the pipeline.

The pipeline derives the matching public key and injects it into instance template metadata (`jenkins-authorized-key`).
At VM boot, the startup script writes that key to **`/home/<jenkins-user>/.ssh/authorized_keys`**, where **`jenkins-user`** instance metadata is set from **`DEFAULT_USER`** when the template is created (default **`jenkins`** in `cf_create_instance_template.sh`). If you change `DEFAULT_USER`, you must align Jenkins **configuration as code (CasC)** (`runAsUser`, `remoteFs`, and the SSH credential **username**) and rebuild the Packer image so the account exists on disk.

### `DEFAULT_USER`

Unix account created on the Cuttlefish VM during the Packer bake and propagated to instance template metadata as **`jenkins-user`**. Default in `cf_create_instance_template.sh` is **`jenkins`**, matching the Cuttlefish GCE cloud configuration in GitOps. Override only when you intentionally change **configuration as code (CasC)** and the SSH credential to the same username; it is **not** tied to the Docker image `ARG USER` used by the CF Instance Template build pod.

### `REPO_USERNAME`

Required if using a private repository defined in `ANDROID_CUTTLEFISH_REPO_URL`.

### `REPO_PASSWORD`

Required if using a private repository defined in `ANDROID_CUTTLEFISH_REPO_URL`.

### `ANDROID_CUTTLEFISH_POST_COMMAND`

Command to run in the `ANDROID_CUTTLEFISH_REPO_URL` defined repo. e.g.
- To fix the netsimd build issues with cxxbridge:
  - `git cherry-pick 78b66377`
- Replace stale repos cuttlefish may be using, such as old kernel.org repos that have been deleted:
  - `sed -i 's|https://git.kernel.org/pub/scm/linux/kernel/git/jaegeuk/f2fs-tools|https://github.com/jaegeuk/f2fs-tools|g' ./base/cvd/build_external/f2fs_tools/f2fs_tools.MODULE.bazel`

### `MACHINE_TYPE`

The machine type to be used for the VM instance. For x86, the default is `n1-standard-64`. Whereas ARM64 currently only `c4a-highmem-96-metal` is available.

Defines the [`--machine-type`](https://cloud.google.com/compute/docs/general-purpose-machines) parameter.

To create a custom machine type, do not define `MACHINE_TYPE` and instead define the 3 `CUSTOM` options which will specify the machine type.

### `CUSTOM_VM_TYPE`

Specifies a custom machine type.

Defines the [`--custom-vm-type`](https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create) parameter.

### `CUSTOM_CPU`

Specifies the number of cores needed for custom machine type.

Defines the [`--custom-cpu`](https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create) parameter.

### `CUSTOM_MEMORY`

Specifies the memory needed for custom machine type.

Defines the [`--custom-memory`](https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create) parameter.

### `BOOT_DISK_SIZE`

A boot disk is required to create the instance, therefore define the size of disk required.

### `BOOT_DISK_TYPE`

Define the Boot disk type. Typically:
- x86_64: `pd-balanced`
- ARM64: `hyperdisk-balanced`

### `MAX_RUN_DURATION` (Cuttlefish **instance template** runtime — not Packer)

This parameter sets **`max_run_duration`** on **VMs launched from the published Cuttlefish instance template** (for example Jenkins GCE agents). It does **not** set how long the **ephemeral Packer builder VM** may run; that is **`PACKER_BUILD_MAX_RUN_DURATION`** (default **`4h`**, see [Orphan Packer boot disks](#orphan-packer-boot-disks-zonal-cleanup) and the script header).

**Default:** **`12h`** in **`cf_create_instance_template.sh`**, Jenkins jobs, and Argo **`maxRunDuration`** (`workloads/android/pipelines/environment/cf_instance_template/helm/values.yaml`). Keep these entrypoints aligned when you change the product default.

User may disable by setting the value to 0, but they must be aware of any costs that they may incur to their project.  Setting to 0 is useful when creating development test instances so users can connect directly to the VM instance.

### `JAVA_VERSION`

Exact **Debian/Ubuntu apt package name** for the **Java Development Kit (JDK)** (no hidden fallbacks). Match the Java major to your Jenkins controller (e.g. **2.555+** agents need **21**).

- **Debian** GCE images (`debian-cloud`, bookworm, etc.): use **`temurin-21-jdk`** (Eclipse Temurin / Adoptium JDK 21). Names starting with **`temurin-`** cause provisioning to add the **Adoptium** apt repo, then **`apt install ${JAVA_VERSION}`**.
- **Ubuntu** (default for x86 and ARM64 jobs today): use **`openjdk-21-jdk-headless`** from Ubuntu main unless you intentionally want Temurin, in which case set **`temurin-21-jdk`** the same way as on Debian.

Examples: **`openjdk-21-jdk-headless`** (distro OpenJDK), **`temurin-21-jdk`** (Eclipse Temurin).

Build VMs need outbound **Hypertext Transfer Protocol Secure (HTTPS)** to distro mirrors and, for Temurin, **packages.adoptium.net**.

### `OS_VERSION`

Override the OS image version string. Images age out; refresh periodically.

- **x86_64:** Ubuntu **Jammy** amd64 family on **`ubuntu-os-cloud`** (same generation as ARM64 Jammy; use **`gcloud compute images list --project=ubuntu-os-cloud`**).
- **ARM64:** Ubuntu ARM64 family (see **`ubuntu-os-cloud`** / Jammy variants).

Use **`gcloud compute images list`** to list current names.

### `OS_PROJECT`

Disk image project.

Refer to `gcloud compute images list` for the project names based on family and OS version.

### `CURL_UPDATE_COMMAND`

Command provided to upgrade Curl during the Packer bake. **Optional:** leave empty on Ubuntu Jammy unless you need a specific curl build.

When x86 used **Debian bookworm**, a typical value was **`sudo apt install -t bookworm-backports -y curl libcurl4`**.

### `NODEJS_VERSION`

**MediaTek (MTK) Connect** requires NodeJS; this option allows you to update the version to install on the instance template.

### `PACKER_GOOGLECOMPUTE_VERSION_MIN` / `PACKER_GOOGLECOMPUTE_VERSION_MAX`

Version bounds for the HashiCorp **`googlecompute`** Packer plugin, rendered at build time into the generated `plugins.pkr.hcl` as `version = ">= MIN, < MAX"`. `MIN` is **inclusive**, `MAX` is **exclusive**. Defaults **`1.2.3`** and **`1.2.5`**, i.e. the effective plugin line is **1.2.3 – 1.2.4**.

Both accept a plain version only (`1.2.3`, `1.3`); anything else — a range, an operator, `latest` — is rejected before Packer runs. They exist so the cap can be raised once upstream fixes the crash, without editing the Packer **HashiCorp configuration language (HCL)**. Both interfaces expose them the same way: a job parameter on Jenkins, a workflow parameter on Argo (`packerGooglecomputeVersionMin` / `packerGooglecomputeVersionMax` on `cf-instance-template-x86` and `cf-instance-template-arm64`).

Read [Pin policy](#pin-policy) before changing either value: the lower bound is a hard requirement and the upper bound is an Argo crash workaround, not a preference. The **Packer binary** version is a separate pin — see [Packer version pinning](#packer-version-pinning).

### `CTS_ANDROID_<14|15|16>_URL`

Defines the URL where to retrieve and install the **Compatibility Test Suite (CTS)** Android test harness. Leave blank if not required, or override the
current default using your own version, e.g. from bucket storage.

During the Packer bake, each non-empty URL is unpacked under **`/opt/android-cts_<version>/android-cts/`** (for example **`/opt/android-cts_15/android-cts/tools`**). **CTS Execution** and **CVD** test jobs symlink **`/opt/android-cts`** to the matching version at runtime (`cts_initialise.sh`); rebake instance templates after changing this install location.

### `ARM64`-only parameters

ARM64 Cuttlefish is region- and shape-constrained (often **`us-central1`**). Override only if your project differs.

**Packer IAP / SSH timeouts (ARM64)**

- Image builds use IAP-tunneled SSH with long waits for **`c4a-*-metal`** (Argo: **`packerSshTimeout` 15m**, **`packerIapTunnelLaunchWait` 300s** in the CF Helm chart; Jenkins **`Jenkinsfile`** matches).
- Metal availability and **time-to-SSH vary by region and zone** — some zones routinely take longer than others.
- If IAP or SSH timeout errors persist after increasing those values:
  - Set **`arm64_region`** and **`arm64_zone`** (and matching subnet/range names when applicable) in **`terraform/env/terraform.tfvars`**.
  - Run **`terraform apply`**, then sync **`workloads-android`** so cluster **`ARM64_REGION`** / **`ARM64_ZONE`** match.
  - Re-run **`cf-instance-template-arm64`** (Argo) or **CF Instance Template ARM64** (Jenkins).
- See [upgrade guide §2b — ARM64 placement](../../../guides/upgrade_guide_4_0_0_to_4_1_0.md#section-2b---arm64-cuttlefish-placement).

#### `ADDITIONAL_NETWORKING`

ARM64 bare metal typically needs **`nic-type=IDPF`** in the template networking options.

#### `SUBNET`

Subnet for ARM64 instances, or leave **blank** for the platform default.

#### `REGION`

**Google Cloud Platform (GCP)** region for the ARM64 instance. Leave **blank** to use the platform default.

#### `ZONE`

GCP zone for the ARM64 instance. Leave **blank** to use the platform default.

## Private repo and branch (e.g. `horizon/main`) <a name="private-repo-and-branch-eg-horizonmain--artifacts-and-jenkins-gce"></a>

Use this flow when **`ANDROID_CUTTLEFISH_REPO_URL`** is a **private** fork and **`ANDROID_CUTTLEFISH_REVISION`** is a branch such as **`horizon/main`**.

### 1. Jenkins job parameters (CF Instance Template)

| Parameter | Example | Purpose |
|-----------|---------|---------|
| `ANDROID_CUTTLEFISH_REPO_URL` | `https://github.com/your-org/android-cuttlefish.git` | Clone URL for Cuttlefish sources (HTTPS is typical for `REPO_*` credentials). |
| `ANDROID_CUTTLEFISH_REVISION` | `horizon/main` | Branch or tag to `git checkout` during the image bake. |
| `REPO_USERNAME` | service account or **personal access token (PAT)** user | Required for **private** HTTPS clones. |
| `REPO_PASSWORD` | PAT or password | Use a credential with **read** access to the repo. |
| `CUTTLEFISH_INSTANCE_NAME` | *(empty)* | Leave empty to **derive** the VM/template name from the revision (see below), or set an explicit name starting with `cuttlefish-vm-`. |

Run stage **`1`** (normal build) with **`DELETE=false`** unless you are deleting artifacts.

### 2. What gets created in GCP (auto-derived name from `horizon/main`)

Naming follows `cf_create_instance_template.sh`: the revision string is sanitized for GCE (`.` removed, **`/` → `-`**), then prefixed with `cuttlefish-vm-` when `CUTTLEFISH_INSTANCE_NAME` is left at the default `cuttlefish-vm`.

For **`ANDROID_CUTTLEFISH_REVISION=horizon/main`** and default instance naming:

| Resource | Name |
|----------|------|
| Cuttlefish “version” token | `horizon-main` (from `horizon/main`) |
| Logical / VM prefix | **`cuttlefish-vm-horizon-main`** |
| Disk image | **`image-cuttlefish-vm-horizon-main`** |
| Instance template | **`instance-template-cuttlefish-vm-horizon-main`** |

Verify after the job:

```bash
gcloud compute instance-templates list | grep cuttlefish-vm-horizon-main
gcloud compute images list | grep image-cuttlefish-vm-horizon-main
```

### 3. Wire Jenkins GCE Cloud (GitOps — preferred)

Test jobs (**Cuttlefish Virtual Device (CVD)** Launcher, **Compatibility Test Suite (CTS)** Execution, etc.) provision agents via the **Google Compute Engine (GCE)** plugin using a **label** that must match a **cloud** whose **`template`** URL points at your new instance template.

1. Open **`gitops/workloads/values-jenkins.yaml`** (**configuration as code (CasC)** for Jenkins).
2. Under **`jenkins:` → `controller:` → `JCasC` → `configScripts:`** (or equivalent), find the **`clouds:`** list and the existing **`computeEngine`** entries (e.g. `cuttlefish-vm-main`).
3. **Add a new** `- computeEngine:` block by **copying** an existing Cuttlefish entry and changing only what identifies the template and labels:
   - **`cloudName`**: e.g. **`cuttlefish-vm-horizon-main`** (this is the **cloud id** in Jenkins).
   - **`labelString`** / **`labels`** / **`namePrefix`**: use the **same** string as `cloudName` (e.g. **`cuttlefish-vm-horizon-main`**), consistent with existing entries.
   - **`template`**: set to the full instance-template self-link for **`instance-template-cuttlefish-vm-horizon-main`** in your project (same pattern as siblings — only the template **name suffix** changes).
   - Keep **`zone`**, **`region`**, **`projectId`**, **`credentialsId`**, **`sshConfiguration`**, **`remoteFs`**, **`runAsUser`** aligned with other Cuttlefish clouds unless you intentionally differ.
4. Merge and deploy via your normal **GitOps** process so CasC reapplies Jenkins configuration.

After sync, **Manage Jenkins → Clouds** should list the new cloud (e.g. `cuttlefish-vm-horizon-main`).

### 4. Wire Jenkins GCE Cloud (**user interface (UI)** only — not source of truth)

You can **verify** or **prototype** under **Manage Jenkins → Clouds →** (Google Compute Engine plugin):

- Add or edit a cloud so the **Instance template** points at `instance-template-cuttlefish-vm-horizon-main` and the **labels** match what test jobs use.

**Caution:** Manual UI edits are **overwritten** on the next CasC sync unless the same settings exist in **`values-jenkins.yaml`**. Treat GitOps as authoritative for production.

### 5. Point test jobs at the new cloud

Jobs that run on Cuttlefish VMs expose **`JENKINS_GCE_CLOUD_LABEL`** (see job **domain-specific language (DSL)** under `workloads/android/pipelines/tests/*/groovy/job.groovy`). Set it to the **label** that matches the cloud — typically the same as **`cloudName`** / **`labelString`**, e.g.:

- **`JENKINS_GCE_CLOUD_LABEL=cuttlefish-vm-horizon-main`**

Default parameter values may still reference `cuttlefish-vm-main`; change per run or update the job default in **Jenkins** / **Seed job** / **CasC** if this template becomes the platform standard.

## Example Usage <a name="examples"></a>

**Create a disposable test template and VM (Jenkins):**

| Step | Parameters |
|------|------------|
| Build | **`ANDROID_CUTTLEFISH_REVISION`** = desired Cuttlefish ref; **`CUTTLEFISH_INSTANCE_NAME`** = e.g. `cuttlefish-vm-test-instance-v110` (must start with `cuttlefish-vm-`); **`MAX_RUN_DURATION`** = **`0`** so the VM is not auto-terminated; run **Build**. |
| Tear down | Same **`CUTTLEFISH_INSTANCE_NAME`**; **`DELETE`** = **true**; **Build** — removes KCC **custom resource (CR)**, disk image, and VM. |

## System variables (Jenkins) <a name="system-variables"></a>

Platform-wide Jenkins environment variables come from Jenkins **configuration as code (CasC)** in **`gitops/workloads/values-jenkins.yaml`**. Inspect them in **Manage Jenkins → System → Global properties → Environment variables**.

| Variable | Role |
|----------|------|
| `ANDROID_BUILD_DOCKER_ARTIFACT_PATH_NAME` | Artifact Registry path for build/test Docker images |
| `CLOUD_PROJECT` | GCP project ID (buckets, registries, compute) |
| `CLOUD_REGION` | Default region |
| `CLOUD_ZONE` | Default zone |
| `HORIZON_DOMAIN` | Horizon site / API domain for job logic |
| `HORIZON_SCM_URL` | Horizon SDV Git repository URL |
| `HORIZON_SCM_BRANCH` | Branch checked out for `HORIZON_SCM_URL` |
| `JENKINS_SERVICE_ACCOUNT` | Service account for Jenkins pipelines; align with GCP **identity and access management (IAM)** roles |

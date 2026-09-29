# Standalone ABFS Deployment Troubleshooting Guide
This field guide documents real-world hurdles, organization policies, and infrastructure race conditions encountered during the standalone ABFS deployment, alongside certified resolution playbooks.

---

## 🔍 Table of Contents
1. [Google Cloud Organization Policies](#1-google-cloud-organization-policies)
2. [Config Connector (KCC) API Enablement Latency](#2-config-connector-kcc-api-enablement-latency)
3. [Cloud Spanner Schema Decoding & Cost Tuning](#3-cloud-spanner-schema-decoding--cost-tuning)
4. [GKE Private Cluster DNS and Google API Isolation](#4-gke-private-cluster-dns-and-google-api-isolation)
5. [Private Node Verification & SSH Blocker](#5-private-node-verification--ssh-blocker)
6. [StatefulSet VolumeClaimTemplate Immutability & OOM Recovery](#6-statefulset-volumeclaimtemplate-immutability--oom-recovery)
7. [Git-Pusher CrashLoopBackOff: Invalid Project Error](#7-git-pusher-crashloopbackoff-invalid-project-error)
8. [Standalone GCE VM Troubleshooting & Operations Reference](#8-standalone-gce-vm-troubleshooting--operations-reference)

---

## 1. Google Cloud Organization Policies

In hardened enterprise GCP landing zones, several default organization policies will clash with GKE Standard and standalone ABFS configurations.

### A. Shielded VM Enforcement (`constraints/compute.requireShieldedVm`)
*   **Symptom**: GKE node pool creation fails with:
    ```
    ERROR: (gcloud.compute.instances.create) Constraint constraints/compute.requireShieldedVm violated.
    ```
*   **Root Cause**: The organization enforces that all compute instances must run as Shielded VMs (Secure Boot + Integrity Monitoring).
*   **Remediation**: Pass the explicit Shielded Nodes flags during the cluster or node-pool creation commands:
    ```bash
    gcloud container node-pools create abfs-data \
      ... \
      --shielded-secure-boot \
      --shielded-integrity-monitoring
    ```

### B. VM External IP Restriction (`constraints/compute.vmExternalIpAccess`)
*   **Symptom**: GKE node creation succeeds but nodes remain in `NotReady` or fail to register with the control plane.
*   **Root Cause**: Node instances are being assigned public IPs which are blocked by organization policies.
*   **Remediation**: Always deploy GKE nodes as private nodes inside custom subnets:
    1. Pass `--enable-private-nodes` to `gcloud container clusters create`.
    2. Provision **Cloud NAT** and **Cloud Router** inside the VPC subnet to handle secure, NAT-translated outbound egress for image pulling and GKE control plane communication.

### C. Service Account Key Creation Blocked (`constraints/iam.disableServiceAccountKeyCreation`)
*   **Symptom**: Declarative deployment or manual setup fails when attempting to create static JSON keys for the runtime service account.
*   **Root Cause**: Static credential creation is locked down by organization security policies to prevent token leakage.
*   **Remediation**: **ABFS Metadata-Identity Bypass**. Do not use Workload Identity or static JSON service account keys for the `abfs-data` pool:
    1. Provision the node pool with `--workload-metadata=GCE_METADATA` and associate the runtime service account directly using `--service-account=abfs-runtime@YOUR_PROJECT_ID.iam.gserviceaccount.com`.
    2. The ABFS server container will read node-level credentials natively from the local GCE Metadata Server, eliminating static key files completely.

---

## 2. Config Connector (KCC) API Enablement Latency

*   **Symptom**: Initial deployment of infrastructure manifests (like Spanner or Secret Manager) fails with:
    ```
    STATUS: UpdateFailed
    MESSAGE: Google Cloud API has not been used in project [...] before or is disabled.
    ```
*   **Root Cause**: When a KCC `Service` resource (such as `spanner.googleapis.com`) is applied, GCP enables the API. However, GCP's IAM and API registry takes up to 2-3 minutes to propagate this enablement globally. During this propagation window, KCC resource creation requests fail with `403 SERVICE_DISABLED`.
*   **Remediation**:
    *   **Automated**: KCC's reconciliation engine automatically retries the operation. Allow 5 minutes, and KCC will self-heal and adopt the resource.
    *   **Manual Force**: Re-apply the manifest to force KCC to re-evaluate the API state immediately:
        ```bash
        kubectl apply -k rendered/standalone/infra/overlays/sandbox
        ```

---

## 3. Cloud Spanner Schema Decoding & Cost Tuning

*   **Symptom**: Applying `20-spanner.yaml` to GKE throws a CRD decoding error:
    ```
    error: error validating "20-spanner.yaml": error validating data: ValidationError(SpannerInstance.spec): unknown field "edition" / "defaultBackupScheduleType" / "autoscalingConfig"
    ```
*   **Root Cause**: Many standard Kubernetes Config Connector installations contain slightly older `v1beta1` Spanner schemas. Newer fields introduced by GCP (such as enterprise editions or autoscaling configs) are rejected by strict API validation.
*   **Remediation**: Replace newer advanced properties with standard processing unit declarations. This is also highly recommended to keep costs down in sandbox environments:
    ```yaml
    # Use this sandbox-friendly schema
    spec:
      config: regional-europe-west3
      processingUnits: 100 # Translates to 0.1 nodes; bypasses advanced fields
    ```

---

## 4. GKE Private Cluster DNS and Google API Isolation

*   **Symptom**: Pods fail to connect to Cloud Spanner or Google Cloud Storage, throwing connection timeout errors or resolving public endpoints to black holes.
*   **Root Cause**: Private nodes do not have external public IPs and cannot resolve public Google API endpoints directly over the internet.
*   **Remediation**: Setup Private Google Access DNS zones.
    1. Create a Private DNS Zone for `googleapis.com` pointing to the restricted Google IP range (`199.36.153.8/30`).
    2. Ensure that your GKE cluster subnet has `privateIpGoogleAccess: true` enabled. KCC automatically provisions these DNS records during our infrastructure phase:
        ```bash
        kubectl get dnsrecordset -n abfs
        ```

---

## 5. Private Node Verification & SSH Blocker

*   **Symptom**: SSH-ing to GKE nodes via `gcloud compute ssh --tunnel-through-iap` fails with:
    ```
    Error while connecting [4003: 'failed to connect to backend']. (Failed to connect to port 22)
    ```
*   **Root Cause**: GKE nodes block ingress to TCP port 22 (SSH) by default for security, preventing IAP tunnel establishment.
*   **Remediation**: Bypass SSH port 22. Instead, use standard **Kubernetes API Tunneling** via a temporary privileged Pod to execute host diagnostics (such as loading `modprobe casfs` or checking kernel versions):

    ```yaml
    # Save as scratch-verify.yaml
    apiVersion: v1
    kind: Pod
    metadata:
      name: casfs-verify
      namespace: default
    spec:
      nodeName: YOUR_DATA_NODE_NAME
      hostPID: true
      containers:
      - name: verify
        image: alpine
        securityContext:
          privileged: true
        command: ["sh", "-c", "nsenter -t 1 -m -u -i -n modprobe casfs && ls -d /sys/module/casfs"]
      restartPolicy: Never
      tolerations:
      - key: abfs.dev/dedicated
        operator: Equal
        value: "true"
        effect: NoSchedule
    ```
    ```bash
    kubectl apply -f scratch-verify.yaml
    kubectl logs casfs-verify
    kubectl delete -f scratch-verify.yaml
    ```

---

## 6. StatefulSet VolumeClaimTemplate Immutability & OOM Recovery

*   **Symptom**: 
    1. One or more `abfs-gerrit-uploader` pods enter `CrashLoopBackOff` or are repeatedly restarted. Describing the pod shows:
       ```
       Last State:     Terminated
         Reason:       OOMKilled
         Exit Code:    137
       ```
    2. Attempting to deploy higher memory limits or customized disk layouts via Helm fails with:
       ```
       Error: UPGRADE FAILED: cannot patch "abfs-gerrit-uploader" with kind StatefulSet: StatefulSet.apps "abfs-gerrit-uploader" is invalid: spec: Forbidden: updates to statefulset spec for fields other than 'replicas', 'ordinals', 'template', ... are forbidden
       ```

*   **Root Cause**: 
    *   **OOMKilled (Exit Code 137)**: Heavy git repositories containing massive git packs (such as core AOSP platform prebuilts or frameworks) require more memory than the default `32Gi` allocation during multi-threaded object index validation.
    *   **Forbidden StatefulSet updates**: Kubernetes enforces that the `volumeClaimTemplates` field inside StatefulSets is completely immutable. When you modify storage values (such as downsizing/upsizing disk specifications or altering storage classes) in your `values-sandbox.yaml`, Helm's `kubectl patch` call is rejected by the Kubernetes API.

*   **Remediation Playbook**:
    Perform a **Kubernetes Orphan Deletion** of the StatefulSet controller. This deletes the StatefulSet resource definition but **leaves your active pods and data volumes untouched and running**. You can then safely run `helm upgrade`, which recreates the StatefulSet controller with the new specifications and adopts the pods back seamlessly with zero downtime:

    ```bash
    # 1. Orphan-delete the StatefulSet controller (safely keeps pods and PVCs intact)
    kubectl delete statefulset abfs-gerrit-uploader -n abfs --cascade=orphan

    # 2. Deploy your updated configurations
    helm upgrade --install abfs ./rendered/standalone/chart/abfs \
      -f values-sandbox.yaml \
      -f values-local.yaml \
      -n abfs

    # 3. Confirm all uploader shards are successfully updated and running
    kubectl get pods -n abfs -l app.kubernetes.io/name=abfs-gerrit-uploader
    ```

---

## 7. Git-Pusher CrashLoopBackOff: Invalid Project Error

*   **Symptom**: One or all `abfs-gerrit-uploader` pods enter `CrashLoopBackOff`. Inspecting their logs (`kubectl logs -n abfs abfs-gerrit-uploader-0`) reveals a fatal configuration error:
    ```
    ignoring invalid *gitpusher.GitPusherPoolConfig config hash ...: invalid repo /: invalid project with no url, server or path
    ```
*   **Root Cause**: The generated `abfs-pusher-config` ConfigMap contains a redundant or malformed explicitly defined `- project:` block for the manifest repository. The `git-pusher` binary automatically clones the manifest and parses it; adding it again as a raw project block without strict `server` and `path` syntax crashes the Go YAML unmarshaler.
*   **Remediation Playbook**:
    1. Remove the redundant `- project:` definition block entirely from `rendered/standalone/chart/abfs/templates/configmap-pusher.yaml`. Rely solely on the `- manifest:` block.
    2. Re-run your `helm upgrade` command. The `post-upgrade` bootstrap Job will automatically detect the change, generate a valid configuration, and push it to the server. The crashing uploaders will instantly pick up the new configuration hash and enter a healthy state.

---

## 8. Standalone GCE VM Troubleshooting & Operations Reference

When operating an ABFS client on a standalone Ubuntu VM (as documented in [ABFS_CLIENT_VM_BUILD_GUIDE.md](ABFS_CLIENT_VM_BUILD_GUIDE.md)), use the following debugging and operational procedures:

*   **Check CASFS Module Registration**:
    ```bash
    lsmod | grep casfs
    sudo modinfo casfs
    ```
*   **Inspect Kernel Module Diagnostics**:
    ```bash
    sudo dmesg -w | grep -i casfs
    ```
*   **Tail Cacheman Logs**:
    ```bash
    abfs cacheman tail
    # Or inspect log directory:
    tail -f ~/.abfs/logs/*
    ```
*   **Run Diagnostic Health Check**:
    ```bash
    abfs doctor
    ```

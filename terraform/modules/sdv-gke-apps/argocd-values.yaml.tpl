# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

crds:
  # Allow Helm to delete CRDs during uninstall (prevents warning message)
  keep: false

configs:
  params:
    server.insecure: true
    server.rootpath: /argocd
    # GitHub HTTPS from the cluster can be slow or stall; defaults (15s git, 60s
    # server/controller → repo-server) surface as UI "Connection Failed" and
    # `context deadline exceeded` in repo-server while apps may still use cache.
    reposerver.git.request.timeout: "120s"
    server.repo.server.timeout.seconds: "180"
    controller.repo.server.timeout.seconds: "180"
  secret:
    createSecret: false
  cm:
    # Must match server.rootpath (/argocd) and the public Gateway prefix or Keycloak rejects
    # redirect_uri (OAuth callback must be under this URL).
    url: https://${subdomain_name}.${domain_name}/argocd
    resource.customizations: |
      Secret:
        ignoreDifferences: |
          jsonPointers:
          - /metadata/annotations/argocd.argoproj.io~1tracking-id
      external-secrets.io/ExternalSecret:
        ignoreDifferences: |
          jsonPointers:
          - /status
    # storage-gcs-module: GCSBucket health — operator sets status.phase.
    resource.customizations.health.gcsmanager.horizon.io_GCSBucket: |
      hs = {}
      if obj.status ~= nil and obj.status.phase ~= nil then
        if obj.status.phase == "Ready" then
          hs.status = "Healthy"
          if obj.status.message ~= nil then
            hs.message = obj.status.message
          else
            hs.message = "Ready"
          end
        elseif obj.status.phase == "Failed" or obj.status.phase == "DeletionBlocked" then
          hs.status = "Degraded"
          if obj.status.message ~= nil then
            hs.message = obj.status.message
          else
            hs.message = obj.status.phase
          end
        elseif obj.status.phase == "Terminating" then
          hs.status = "Progressing"
          hs.message = "Terminating"
        else
          hs.status = "Progressing"
          if obj.status.message ~= nil then
            hs.message = obj.status.message
          else
            hs.message = obj.status.phase
          end
        end
      else
        hs.status = "Progressing"
        hs.message = "Waiting for storage-gcs-module to reconcile GCSBucket"
      end
      return hs
    # Config Connector context: operator sets status.healthy (kubebuilder CommonStatus).
    # Without this, Argo treats the CR as Progressing forever and sync waves never complete.
    resource.customizations.health.core.cnrm.cloud.google.com_ConfigConnectorContext: |
      hs = {}
      if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil and obj.metadata.deletionTimestamp ~= "" then
        hs.status = "Healthy"
        hs.message = "Deleting (Config Connector context finalizing)"
        return hs
      end
      hs.status = "Progressing"
      hs.message = "Waiting for Config Connector operator (status.healthy unset)"
      if obj.status ~= nil and obj.status.healthy ~= nil then
        if obj.status.healthy == true then
          hs.status = "Healthy"
          hs.message = ""
          return hs
        end
        if obj.status.healthy == false then
          if obj.status.errors ~= nil and obj.status.errors[1] ~= nil and obj.status.errors[1] ~= "" then
            hs.status = "Degraded"
            hs.message = obj.status.errors[1]
            return hs
          end
          hs.status = "Progressing"
          hs.message = "status.healthy is false (CNRM reconciling; describe CCC if stuck)"
          return hs
        end
      end
      return hs
    # Cluster-mode Config Connector singleton (same status.healthy contract as ConfigConnectorContext above).
    # Without this, the configconnector-operator child Application (sync-wave -1) reports Healthy as soon as
    # the ConfigConnector CR is applied, before the operator has actually finished bootstrapping (creating
    # cnrm-system, registering storage.cnrm.* CRDs), letting the root sync race ahead into later waves.
    resource.customizations.health.core.cnrm.cloud.google.com_ConfigConnector: |
      hs = {}
      if obj.metadata ~= nil and obj.metadata.deletionTimestamp ~= nil and obj.metadata.deletionTimestamp ~= "" then
        hs.status = "Healthy"
        hs.message = "Deleting (Config Connector finalizing)"
        return hs
      end
      hs.status = "Progressing"
      hs.message = "Waiting for Config Connector operator (status.healthy unset)"
      if obj.status ~= nil and obj.status.healthy ~= nil then
        if obj.status.healthy == true then
          hs.status = "Healthy"
          hs.message = ""
          return hs
        end
        if obj.status.healthy == false then
          if obj.status.errors ~= nil and obj.status.errors[1] ~= nil and obj.status.errors[1] ~= "" then
            hs.status = "Degraded"
            hs.message = obj.status.errors[1]
            return hs
          end
          hs.status = "Progressing"
          hs.message = "status.healthy is false (CNRM reconciling; describe ConfigConnector if stuck)"
          return hs
        end
      end
      return hs
  # policy.csv + scopes: keep identical to keycloak-post-argocd/configure.sh (POLICY_CSV + rbac-cm patch).
  # Why here: argo-helm resets argocd-rbac-cm on upgrade if omitted. scopes '[roles]' matches Keycloak JWT roles claim; `list` lines satisfy Argo CD 3.x UI list APIs.
  rbac:
    policy.default: role:readonly
    policy.matchMode: glob
    scopes: '[roles]'
    policy.csv: |
      p, role:readonly, applications, get, */*, allow
      p, role:readonly, applications, list, */*, allow
      p, role:readonly, applicationsets, get, */*, allow
      p, role:readonly, applicationsets, list, */*, allow
      p, role:readonly, certificates, get, *, allow
      p, role:readonly, clusters, get, *, allow
      p, role:readonly, clusters, list, *, allow
      p, role:readonly, repositories, get, *, allow
      p, role:readonly, write-repositories, get, *, allow
      p, role:readonly, projects, get, *, allow
      p, role:readonly, projects, list, *, allow
      p, role:readonly, accounts, get, *, allow
      p, role:readonly, gpgkeys, get, *, allow
      p, role:readonly, logs, get, */*, allow
      p, role:admin, applications, create, */*, allow
      p, role:admin, applications, update, */*, allow
      p, role:admin, applications, update/*, */*, allow
      p, role:admin, applications, delete, */*, allow
      p, role:admin, applications, delete/*, */*, allow
      p, role:admin, applications, sync, */*, allow
      p, role:admin, applications, override, */*, allow
      p, role:admin, applications, action/*, */*, allow
      p, role:admin, applications, list, */*, allow
      p, role:admin, applicationsets, get, */*, allow
      p, role:admin, applicationsets, list, */*, allow
      p, role:admin, applicationsets, create, */*, allow
      p, role:admin, applicationsets, update, */*, allow
      p, role:admin, applicationsets, delete, */*, allow
      p, role:admin, certificates, create, *, allow
      p, role:admin, certificates, update, *, allow
      p, role:admin, certificates, delete, *, allow
      p, role:admin, clusters, create, *, allow
      p, role:admin, clusters, update, *, allow
      p, role:admin, clusters, delete, *, allow
      p, role:admin, clusters, list, *, allow
      p, role:admin, repositories, create, *, allow
      p, role:admin, repositories, update, *, allow
      p, role:admin, repositories, delete, *, allow
      p, role:admin, write-repositories, create, *, allow
      p, role:admin, write-repositories, update, *, allow
      p, role:admin, write-repositories, delete, *, allow
      p, role:admin, projects, create, *, allow
      p, role:admin, projects, update, *, allow
      p, role:admin, projects, delete, *, allow
      p, role:admin, projects, list, *, allow
      p, role:admin, accounts, update, *, allow
      p, role:admin, gpgkeys, create, *, allow
      p, role:admin, gpgkeys, delete, *, allow
      p, role:admin, exec, create, */*, allow
      g, role:admin, role:readonly
      g, administrators, role:admin
      g, viewers, role:readonly
global:
  domain: ${subdomain_name}.${domain_name}

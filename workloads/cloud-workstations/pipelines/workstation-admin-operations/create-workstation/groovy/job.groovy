// Copyright (c) 2024-2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Description:
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes create-workstation operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Workstation-Admin-Operations/Create New Workstation') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Create a New GCP Cloud Workstation</h3>
    <p>This job provisions a new Cloud Workstation instance based on a specified configuration.</p>

    <h4 style="margin-bottom: 10px;">Preset Properties (Non-configurable):</h4>
    <p>Values for these properties are sourced from <code>values.yaml</code> or hardcoded within the pipeline script.</p>
    <p><ul>
      <li>CLUSTER: <code>${CLOUD_WS_CLUSTER_PRESET_NAME}</code> (Workstation will be created in this cluster)</li>
      <li>REGION: <code>${CLOUD_REGION}</code></li>
      <li>NETWORK: ${CLOUD_WS_CLUSTER_PRESET_NETWORK_NAME}</li>
      <li>SUBNETWORK: ${CLOUD_WS_CLUSTER_PRESET_SUBNETWORK_NAME}</li>
      <li>PROJECT: <code>${CLOUD_PROJECT}</code></li>
    </ul></p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>This job is idempotent. Hence, if a workstation with the same name already exists, executing this job will neither create a new workstation nor update the existing one.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(7)
    numToKeep(50)
  }

  parameters {
    stringParam('WORKSTATION_NAME', '', 'REQUIRED: A unique name for the new workstation instance (e.g., "my-android-dev-ws").')
    stringParam('WORKSTATION_DISPLAY_NAME', '', 'Optional: A user-friendly display name for the workstation. Leave empty for no display name.')
    stringParam('WORKSTATION_CONFIG_NAME', '', 'REQUIRED: The name of the workstation configuration blueprint to use (e.g., "android-dev-config").')
    stringParam('INITIAL_WORKSTATION_USER_EMAILS_TO_ADD', '', 'OPTIONAL: Comma-separated list of user emails (e.g., user1@example.com,user2@example.com) to grant access to upon creation. These users will get the "workstations.user" role.')
  }

  // Block build if certain jobs are running.
  blockOn('Cloud-Workstations/.*/(Create|Update|Delete|Add|Remove|Start|Stop).*') {
    // Possible values are 'GLOBAL' and 'NODE' (default).
    blockLevel('GLOBAL')
    // Possible values are 'ALL', 'BUILDABLE' and 'DISABLED' (default).
    scanQueueFor('BUILDABLE')
  }

  definition {
    cpsScm {
      lightweight()
      scm {
        git {
          remote {
            url("${HORIZON_SCM_URL}")
            credentials('jenkins-scm-creds')
          }
          branch("*/${HORIZON_SCM_BRANCH}")
        }
      }
      scriptPath('workloads/cloud-workstations/pipelines/workstation-admin-operations/create-workstation/Jenkinsfile')
    }
  }
}
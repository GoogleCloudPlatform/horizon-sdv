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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes add-workstation-user operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Workstation-Admin-Operations/Add Users to Workstation') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Grant Access to Users for a GCP Cloud Workstation</h3>
    <p>This job adds one or more specified users to an existing Cloud Workstation, granting them access.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p><ul>
    <li>Ensure the provided user emails are valid Google Cloud identities.</li>
    <li>Adding a user who already has the specified role will have no effect.</li>
    </p></ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(7)
    numToKeep(50)
  }

  parameters {
    stringParam('WORKSTATION_NAME', '', 'REQUIRED: The exact name of the workstation to add users to.')
    stringParam('WORKSTATION_USER_EMAILS_TO_ADD', '', 'REQUIRED: Comma-separated list of user emails (e.g., user1@example.com,user2@example.com) to grant access.')
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
      scriptPath('workloads/cloud-workstations/pipelines/workstation-admin-operations/add-workstation-user/Jenkinsfile')
    }
  }
}
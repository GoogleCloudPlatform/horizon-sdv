// Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes remove-workstation-user operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Workstation-Admin-Operations/Remove Workstation Users') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Revoke Access for Users from a GCP Cloud Workstation</h3>
    <p>This job removes access for one or more specified users from an existing Cloud Workstation.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p><ul>
    <li>Ensure the provided user emails are valid Google Cloud identities.</li>
    <li>Removing a user who already does not have workstation access will have no effect.</li>
    </p></ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('WORKSTATION_NAME', '', 'REQUIRED: The exact name of the workstation to remove users from.')
    stringParam('WORKSTATION_USER_EMAILS_TO_REMOVE', '', 'REQUIRED: Comma-separated list of user emails (e.g., user1@example.com,user2@example.com) to revoke access for.')
  }

  definition {
    cpsScm {
      lightweight()
      scm {
        git {
          remote {
            url("${HORIZON_GITHUB_URL}")
            credentials('jenkins-github-creds')
          }
          branch("*/${HORIZON_GITHUB_BRANCH}")
        }
      }
      scriptPath('workloads/cloud-workstations/pipelines/workstation-admin-operations/remove-workstation-user/Jenkinsfile')
    }
  }
}
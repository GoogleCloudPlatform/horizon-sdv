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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes delete-workstation operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Workstation-Admin-Operations/Delete Existing Workstation') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Delete an Existing GCP Cloud Workstation</h3>
    <p><b>WARNING: This job permanently deletes the specified Cloud Workstation instance.</b></p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>This operation is irreversible. Ensure no critical work is in progress on the workstation before proceeding.</p>
    <p>Deleting a workstation also deletes its associated persistent disk unless a reclaim policy of 'RETAIN' was set on the configuration.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('WORKSTATION_NAME', '', 'REQUIRED: The exact name of the workstation to delete.')
    booleanParam('CONFIRM_DELETE', false, '<b>REQUIRED: Check this box to confirm deletion. This action is irreversible.</b>')
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
      scriptPath('workloads/cloud-workstations/pipelines/workstation-admin-operations/delete-workstation/Jenkinsfile')
    }
  }
}
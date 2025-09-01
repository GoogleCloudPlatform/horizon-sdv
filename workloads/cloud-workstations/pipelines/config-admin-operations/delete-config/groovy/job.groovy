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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes delete-config operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Config-Admin-Operations/Delete Existing Configuration') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Delete an Existing Configuration of GCP Cloud Workstations</h3>
    <p><strong>WARNING</strong>: This action is IRREVERSIBLE and will permanently DELETE the specified Cloud Workstation Configuration and, any associated workstations.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>Ensure you have selected the correct configuration when confirming the deletion.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('CLOUD_WS_CONFIG_NAME', '', '<strong>REQUIRED</strong>: Name of the workstation Configuration to delete.')
    booleanParam('CONFIRM_DELETE', false, '<strong>REQUIRED</strong>: Check this box to confirm deletion. (Warning: This is irreversible and will delete associated workstations as well).')
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
      scriptPath('workloads/cloud-workstations/pipelines/config-admin-operations/delete-config/Jenkinsfile')
    }
  }
}
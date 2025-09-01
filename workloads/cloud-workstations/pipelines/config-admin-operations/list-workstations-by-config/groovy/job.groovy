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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes list-workstations-by-config operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Config-Admin-Operations/List Workstations by Configuration') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">List Active Workstations for a Specific Configuration</h3>
    <p>This job retrieves and displays a list of active Cloud Workstations that were created using a specific Workstation Configuration.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>This is a read-only operation and does not modify any Cloud Workstation resources.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('CLOUD_WS_CONFIG_NAME', '', '<strong>REQUIRED</strong>: Enter the exact name of the workstation configuration to list its associated workstations.')
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
      scriptPath('workloads/cloud-workstations/pipelines/config-admin-operations/list-workstations-by-config/Jenkinsfile')
    }
  }
}
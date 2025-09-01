// Copyright (c) 2024-2025 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Description:
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes get-workstation (details) operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Workstation-User-Operations/Get Workstation Details') {
  displayName('Get Workstation Details')
  description('''
    <br/><h3 style="margin-bottom: 10px;">Get Details of a Specific GCP Cloud Workstation</h3>
    <p>This job retrieves and displays the full details and current status for a specific Cloud Workstation that current user has access to.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>This is a read-only operation and does not modify any Cloud Workstation resources.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  parameters {
    stringParam('CLOUD_WS_WORKSTATION_NAME', '', '<strong>REQUIRED</strong>: The exact name of the workstation for which to retrieve details.')
  }

  logRotator {
    daysToKeep(60)
    numToKeep(200)
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
      scriptPath('workloads/cloud-workstations/pipelines/workstation-user-operations/get-workstation/Jenkinsfile')
    }
  }
}
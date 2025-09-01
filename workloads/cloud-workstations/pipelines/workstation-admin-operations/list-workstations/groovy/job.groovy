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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes list-workstations operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Workstation-Admin-Operations/List Workstations') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">List All GCP Cloud Workstations</h3>
    <p>This job retrieves and displays a list of Cloud Workstations</p>

    <h4 style="margin-bottom: 10px;">Filter Options:</h4>
    <p>You can optionally provide a wildcard pattern to filter the list of configurations by name, assigned user, or associated configuration.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>This is a read-only operation and does not modify any Cloud Workstation resources.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('WORKSTATION_NAME_PATTERN', '.*', 'Optional: Filter by workstation name (regex, e.g., "dev-.*" or "ws[0-9]+").')
    stringParam('WORKSTATION_USER_EMAIL_PATTERN', '.*', 'Optional: Filter by user email (regex, e.g., ".*@example\\.com" or "joe|jane").')
    stringParam('WORKSTATION_CONFIG_NAME_PATTERN', '.*', 'Optional: Filter by config name (regex, e.g., "config-.*" or "code-.*").')
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
      scriptPath('workloads/cloud-workstations/pipelines/workstation-admin-operations/list-workstations/Jenkinsfile')
    }
  }
}
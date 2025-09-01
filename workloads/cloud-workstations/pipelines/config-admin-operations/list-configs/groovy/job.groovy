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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes list-configs operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Config-Admin-Operations/List Configurations') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">List GCP Cloud Workstation Configurations</h3>
    <p>This job retrieves and displays a list of all existing Configurations of Cloud Workstations.</p>

    <h4 style="margin-bottom: 10px;">Filter Options:</h4>
    <p>You can optionally provide a valid regex pattern to filter the list of configurations by name.</p>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <p>This is a read-only operation and does not modify any Cloud Workstation resources.</p>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('CLOUD_WS_CONFIG_NAME_REGEX_PATTERN', '', 'Optional: Enter a valid regex pattern to filter configuration names.<br>Leave empty to get list of ALL cloud workstation Configs.<br>Example: <code>^my-config-.*$</code>')
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
      scriptPath('workloads/cloud-workstations/pipelines/config-admin-operations/list-configs/Jenkinsfile')
    }
  }
}
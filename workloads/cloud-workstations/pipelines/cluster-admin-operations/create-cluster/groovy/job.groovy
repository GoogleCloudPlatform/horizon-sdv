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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes create-cluster operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Cluster-Admin-Operations/Create New Cluster') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Create a New Cluster for GCP Cloud Workstations</h3>
    <p>This job creates a new Cluster for GCP Cloud Workstations in your existing GCP project.</p>

    <h4 style="margin-bottom: 10px;">Preset Properties (Non-configurable):</h4>
    <ul>
      <li>NAME: <code>${CLOUD_WS_CLUSTER_PRESET_NAME}</code></li>
      <li>REGION: <code>${CLOUD_REGION}</code></li>
      <li>NETWORK: <code>${CLOUD_WS_CLUSTER_PRESET_NETWORK_NAME}</code></li>
      <li>SUBNETWORK: <code>${CLOUD_WS_CLUSTER_PRESET_SUBNETWORK_NAME}</code></li>
      <li>PROJECT: <code>${CLOUD_PROJECT}</code></li>
      <li>PRIVATE_CLUSTER: <code>${CLOUD_WS_CLUSTER_PRESET_PRIVATE_CLUSTER}</code></li>
    </ul>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <ul>
      <li>For a single GCP project, there can be <b>no more than one cluster</b> at any time.</li>
      <li>This job is idempotent. Hence, if a cluster already exists, executing this job will neither create a new cluster nor update existing one.</li>
    </ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

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
      scriptPath('workloads/cloud-workstations/pipelines/cluster-admin-operations/create-cluster/Jenkinsfile')
    }
  }
}
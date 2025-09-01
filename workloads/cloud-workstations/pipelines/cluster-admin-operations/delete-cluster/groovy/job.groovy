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
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes delete-cluster operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Cluster-Admin-Operations/Delete Existing Cluster') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Delete an Existing Cluster of GCP Cloud Workstations</h3>
    <p><strong>WARNING</strong>: This action is IRREVERSIBLE and will permanently DELETE the Cloud Workstations Cluster and all associated resources in your GCP project.</p>

    <h4 style="margin-bottom: 10px;">Cluster to be deleted has the following preset properties:</h4>
    <ul>
      <li>NAME: <code>${CLOUD_WS_CLUSTER_PRESET_NAME}</code></li>
      <li>REGION: <code>${CLOUD_REGION}</code></li>
      <li>NETWORK: <code>${CLOUD_WS_CLUSTER_PRESET_NETWORK_NAME}</code></li>
      <li>SUBNETWORK: <code>${CLOUD_WS_CLUSTER_PRESET_SUBNETWORK_NAME}</code></li>
      <li>PROJECT: <code>${CLOUD_PROJECT}</code></li>
      <li>PRIVATE_CLUSTER: <code>${CLOUD_WS_CLUSTER_PRESET_PRIVATE_CLUSTER}</code></li>
    </ul>

    <h4 style="margin-bottom: 10px;">Steps to Delete Cluster</h4>
    <p>Follow below steps in order to delete Cloud Workstation Cluster:</p>
    <ol>
      <li>
        Delete All Workstations
        <ol>
          <li>
            Run the job <strong><code>'Workstation Admin Operations > List Workstations'</code></strong>
            to get the list of all existing workstations.
          </li>
          <li>
            Run the job <strong><code>'Workstation Admin Operations > Delete Existing Workstation'</code></strong>
            for every Cloud Workstation present in the list you got in the previous step.
          </li>
        </ol>
      </li>
      <li>
        Delete All Configurations
        <ol>
          <li>
            Run the job <strong><code>'Config Admin Operations > List Configurations'</code></strong>
            to get the list of all existing configurations.
          </li>
          <li>
            Run the job <strong><code>'Config Admin Operations > Delete Existing Configuration'</code></strong>
            for every configuration present in the list you got in the previous step.
          </li>
        </ol>
      </li>
      <li>
        Delete Cluster by running this pipeline, finally.
      </li>
    </ol>
    
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
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
      scriptPath('workloads/cloud-workstations/pipelines/cluster-admin-operations/delete-cluster/Jenkinsfile')
    }
  }
}
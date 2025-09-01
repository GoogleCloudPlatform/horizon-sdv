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
// This groovy job is used by the Seed Workloads Pipeline to create folder structure for GCP Cloud Workstation pipelines
//
// References:
//

folder('Cloud-Workstations') {
  displayName('Cloud Workstations')
  description('''
    <br/><h3 style="margin-bottom: 10px;">Manage and Work with GCP Cloud Workstations</h3>

    <p>Follow below steps in order to provision and start using Cloud Workstations:</p>
    <ol>
      <li>
        Run the job <strong><code>'Environment > Docker Image Template'</code></strong> to setup the environment to be used by Cloud Workstation operation pipelines.
      </li>
      <li>
        Run any job of your choice under <strong><code>'Workstation Images'</code></strong> folder to build and publish pre-defined container images to be used by Cloud Workstations.
      </li>
      <li>
        Run the job <strong><code>'Cluster Admin Operations > Create Cluster'</code></strong> to create the Workstation Cluster with pre-set properties.
      </li>
      <li>
        Run the job <strong><code>'Config Admin Operations > Create New Configuration'</code></strong> to create a Workstation Configuration.
      </li>
      <li>
        Run the job <strong><code>'Workstation Admin Operations > Create New Workstation'</code></strong> to create a Workstation using Config created in previous step.
      </li>
      <li>
        Run the job <strong><code>'Workstation User Operations > Start Workstation'</code></strong> to start the Workstation created in previous step.
      </li>
    </ol>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')
}

folder('Cloud-Workstations/Cluster-Admin-Operations') {
  displayName('Cluster Admin Operations')
  description('<p>This folder contains jobs that run operations on Clusters in GCP Cloud Workstations service as a Workstation Admin.</p>')
}

folder('Cloud-Workstations/Config-Admin-Operations') {
  displayName('Config Admin Operations')
  description('<p>This folder contains jobs that run operations on Configs in GCP Cloud Workstations service as a Workstation Admin.</p>')
}

folder('Cloud-Workstations/Workstation-Admin-Operations') {
  displayName('Workstation Admin Operations')
  description('<p>This folder contains jobs that run operations on Workstations in GCP Cloud Workstations service as a Workstation Admin.</p>')
}

folder('Cloud-Workstations/Workstation-User-Operations') {
  displayName('Workstation User Operations')
  description('<p>This folder contains jobs that run operations on Workstations in GCP Cloud Workstations service as a Workstation User.</p>')
}

folder('Cloud-Workstations/Environment') {
  displayName('Environment')
  description('<p>This folder contains a job that builds Docker image which will be used as environment for GCP Cloud Workstations pipeline jobs.</p>')
}

folder('Cloud-Workstations/Workstation-Images') {
  displayName('Workstation Images')
  description('<p>This folder contains a job that builds Docker image for the Code OSS (open-source VS Code) IDE for use in Cloud Workstations.</p>')
}

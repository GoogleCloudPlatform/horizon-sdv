// Copyright (c) 2025 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
folder('Android') {
  displayName('Android Workflows')
  description('<p>This folder contains pipelines and jobs related to environment administration, building, testing, and deploying Android applications.<br/>It includes workflows for administration, building and test tasks.</p>')
}
folder('Android/Builds') {
  displayName('Builds')
  description('<p>This folder contains jobs to build Android targets.</p>')
}
folder('Android/Environment') {
  displayName('Environment')
  description('''<p>This folder contains environment administrative jobs related to supporting Android workflows.</p>
    For AAOS, execute the following to ensure the environment is correctly provisioned:<br/>
    <ol><li><b>Docker Image Template:</b> create the Docker build container used for standard builds.</li>
    <li><b>CF Instance Template:</b> Create the Cuttlefish VM instance templates required for test jobs (dependent on Docker Image Template)</li>
    <li><b>ABFS:</b> If using ABFS then ensure those jobs in this folder are executed accordingly.</li></ol>''')
}
folder('Android/Environment/ABFS') {
  displayName('ABFS')
  description('''<p>This folder contains environment administrative jobs related to supporting Android Build File System (ABFS) workflows.</p>
    Ensure you execute in the following order to ensure ABFS environment is correctly provisioned.<br/>
    <ol><li><b>Docker Infra Image Template:</b> create the Docker infrastructure container for ABFS Server and Uploader jobs.</li>
    <li><b>Server:</b> Create the ABFS server (dependent on Docker Infra Image Template) </li>
    <li><b>Uploader:</b> Create the ABFS uploaders (dependent on Docker Infra Image Template) </li>
    <li><b>Docker Image Template:</b> create the Docker build container used for ABFS builds.</li></ol>
    <p>Refer to <i>docs/workloads/android/abfs.md</> for additional details.</p>''')

}
folder('Android/Tests') {
  displayName('Tests')
  description('<p>This folder contains jobs used to help test and validate Android builds.</p>')
}

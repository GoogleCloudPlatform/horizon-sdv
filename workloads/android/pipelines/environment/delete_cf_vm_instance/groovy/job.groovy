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
pipelineJob('Android/Environment/Delete Cuttlefish VM Instance') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Cuttlefish VM Cleanup Job</h3>
    <p>This job allows developers to delete Cuttlefish VM instances that may have been left running after testing.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>Only VM instances with the prefix <code>cuttlefish-vm</code> can be deleted, preventing accidental deletion of non-Cuttlefish instances.</p>
    <h4 style="margin-bottom: 10px;">Viewing running instances:</h4>
    <ul>
      <li><a href="http://${HORIZON_DOMAIN}/jenkins/manage/computer/" target="_blank">Jenkins Computer Management</a></li>
      <li><a href="https://console.cloud.google.com/compute/instances?project=${CLOUD_PROJECT}" target="_blank">Google Cloud Compute Instances</a></li></ul>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('VM_INSTANCE_NAME')
      defaultValue('')
      description('''<p>The name of the instance to delete, e.g.</p>
        <i>cuttlefish-vm-main-hgie4b</i></p>''')
      trim(true)
    }
  }

  // Block build if certain jobs are running.
  blockOn('Android*.*Delete.*Cuttlefish.*') {
    // Possible values are 'GLOBAL' and 'NODE' (default).
    blockLevel('GLOBAL')
    // Possible values are 'ALL', 'BUILDABLE' and 'DISABLED' (default).
    scanQueueFor('BUILDABLE')
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
      scriptPath('workloads/android/pipelines/environment/delete_cf_vm_instance/Jenkinsfile')
    }
  }
}

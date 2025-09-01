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
pipelineJob('Android/Environment/Development Test Instance') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Development Test Instance Creation Job</h3>
    <p>This job allows creation of a temporary GCE VM instance that can be used to aid development of test instances.<br/>
    <h4 style="margin-bottom: 10px;">Instance Details</h4>
    <p>Instances can be expensive and therefore there is a maximum up-time before the instance will automatically be terminated.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>Users are responsible for saving their own work to persistent storage before expiry.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('JENKINS_GCE_CLOUD_LABEL')
      defaultValue("${JENKINS_GCE_CLOUD_LABEL}")
      description('''<p>The Jenkins GCE Clouds label for the VM instance template, e.g.<br/></p>
        <ul>
          <li>cuttlefish-vm-main</li>
          <li>cuttlefish-vm-v1180</li>
        </ul>''')
      trim(true)
    }

    choiceParam {
      name('INSTANCE_MAX_UPTIME')
      choices(['1', '2', '4', '8'])
      description('''<p>Time in hours to keep instance alive.</p>''')
    }
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
      scriptPath('workloads/android/pipelines/environment/dev_instance_test/Jenkinsfile')
    }
  }
}

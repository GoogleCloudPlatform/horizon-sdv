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
pipelineJob('Android/Environment/Delete MTK Connect Testbench') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">MTK Connect Testbench Cleanup Job</h3>
    <p>This job allows developers to delete offline MTK Connect testbenches that may have been left running after testing.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>The test pipelines show the name of the Testbench in console logs, e.g. <code>MTK_CONNECT_TESTBENCH=Android/Tests/CVD_Launcher-4</code></p>
    <h4 style="margin-bottom: 10px;">Viewing Testbenches</h4> <p><a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank">MTK Connect Testbenches</a> provides details of online and offline testbenches.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('MTK_CONNECT_TESTBENCH')
      defaultValue('')
      description('''<p>Provide the name of the testbench to remove from MTK Connect, e.g. <br/>
        <ul>
          <li>Android/Tests/CTS_Execution-1</li>
          <li>Android/Tests/CVD_Launcher-1</il>
        </ul><b>WARNING:</b> Take care when using this option!</p>''')
      trim(true)
    }
  }

  // Block build if certain jobs are running.
  blockOn('Android*.Delete.*MTK*') {
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
      scriptPath('workloads/android/pipelines/environment/delete_mtkc_testbench/Jenkinsfile')
    }
  }
}

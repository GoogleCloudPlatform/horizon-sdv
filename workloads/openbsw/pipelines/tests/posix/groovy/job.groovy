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

// Description:
// Groovy file for defining a Jenkins Pipeline Job for testing the OpenBSW
// POSIX application.
pipelineJob('OpenBSW/Tests/POSIX') {
  description("""
    <br/><h2 style="margin-bottom: 10px;">OpenBSW POSIX Test Job</h2>
    <p>This job allows the user to access the OpenBSW platform to test a prior build of the POSIX application.</p>
    <h3 style="margin-bottom: 10px;">Job Overview</h3>
    <p>Devices are initialized and remain active for a specified period, allowing users to interact with them via <a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank">MTK Connect</a>.<br/>
    After the <code>POSIX_KEEP_ALIVE_TIME</code> period expires, the devices, testbenches, and test instance are terminated in a controlled manner.</p>
    <h3 style="margin-bottom: 10px;">Mandatory Parameters</h3>
    <ul>
      <li><code>OPENBSW_DOWNLOAD_URL</code>: The URL of the user's POISX test binaries to install and run.</li>
    </ul>
    <h3 style="margin-bottom: 10px;">POSIX Application Test Execution Guide</h3>
    <p>Use this concise guide to bring up networking, launch the reference app, and run tests.</p>
    <h4 style="margin-bottom: 10px;">One-Time Setup</h4>
    <p>Run these once per machine boot (or when networking state is reset):</p>
    <pre><code class="language-bash"># Bring up Ethernet
./posix/tools/enet/bring-up-ethernet.sh
# Bring up virtual CAN on vcan0
./posix/tools/can/bring-up-vcan0.sh</code></pre>
    <h4 style="margin-bottom: 10px;">Launch the Reference Application:</h4>
    <p>Starts the POSIX reference application console:</p>
    <b>posix-freertos</b><br/>
    <pre><code class="language-bash">./posix/build/posix-freertos/executables/referenceApp/application/Release/app.referenceApp.elf</code></pre>
    <b>posix-threadx</b><br/>
    <pre><code class="language-bash">./posix/build/posix-threadx/executables/referenceApp/application/Release/app.referenceApp.elf</code></pre>
    <ul>
      <li>Keep this running while testing.</li>
      <li>Stop with Ctrl+C when done.</li>
    </ul>
    <h4 style="margin-bottom: 10px;">Run POSIX pyTest:</h4>
    <p>Execute pyTests targeting the POSIX build::</p></br><br/>
    <b>posix-freertos</b><br/>
    <pre><code>cd posix/test/pyTest/ && pytest --target=posix --app=freertos</code></pre>
    <b>posix-threadx</b><br/>
    <pre><code>cd posix/test/pyTest/ && pytest --target=posix --app=threadx</code></pre>
    <h3>Reference documentation:</h3>
    <ul>
      <li><a href="https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/console/index.html" target="_blank">Application Console.</a></li>
    </ul>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('OPENBSW_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Storage URL pointing to the location of the test image, e.g.<br/>gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Builds/BSW_Builder/&lt;BUILD_NUMBER&gt;/posix/<br/><br/>
        <b>Note:</b>
          <ul><li>if build number is less than 2 digits, then zero pad , i.e. 1 to 9 must be 01 to 09.</li></ul)</p>""")
      trim(true)
    }

    stringParam {
      name('IMAGE_TAG')
      defaultValue("${OPENBSW_IMAGE_TAG}")
      description('''<p>Docker image template to use.<p>
        <p>Note: tag may only contain 'abcdefghijklmnopqrstuvwxyz0123456789_-./'</p>''')
      trim(true)
    }

    choiceParam {
      name('POSIX_KEEP_ALIVE_TIME')
      choices(['5', '15', '30', '60', '90', '120', '180'])
      description('''<p>Time in minutes, to keep host instance alive before stopping.</p>''')
    }

    stringParam {
      name('NUM_HOST_INSTANCES')
      defaultValue('2')
      description('''<p>Number of host instances to create.<p>
        <p>i.e. the number of devices to create in MTK Connect testbench.</p>''')
      trim(true)
    }

    booleanParam {
      name('MTK_CONNECT_PUBLIC')
      defaultValue(false)
      description('''<p>When checked, the MTK Connect testbench is visible to everyone.<br/>
        By default, testbenches are private and only visible to their creator and MTK Connect administrators.</p>''')
    }
  }

  logRotator {
    artifactDaysToKeep(60)
    artifactNumToKeep(100)
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
      scriptPath('workloads/openbsw/pipelines/tests/posix/Jenkinsfile')
    }
  }
}

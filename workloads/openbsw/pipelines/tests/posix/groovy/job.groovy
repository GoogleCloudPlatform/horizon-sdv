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
    <br/><h3 style="margin-bottom: 10px;">OpenBSW POSIX Test Job</h3>
    <p>This job allows the user to access the OpenBSW platform to test a prior build of the POSIX application.</p>
    <h4 style="margin-bottom: 10px;">Job Overview</h4>
    <p>Devices are initialized and remain active for a specified period, allowing users to interact with them via <a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank">MTK Connect</a>.<br/>
    After the <code>POSIX_KEEP_ALIVE_TIME</code> period expires, the devices, testbenches, and test instance are terminated in a controlled manner.</p>
    <h4 style="margin-bottom: 10px;">Mandatory Parameters</h4>
    <ul>
      <li><code>OPENBSW_DOWNLOAD_URL</code>: The URL of the user's POISX test binaries to install and run.</li>
    </ul>
    <h4>Reference documentation:</h4>
    <ul>
      <li><a href="https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/learning/console/index.html" target="_blank">Application Console.</a></li>
    </ul>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('OPENBSW_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Storage URL pointing to the location of the test image, e.g.<br/>gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Builds/BSW_Builder/&lt;BUILD_NUMBER&gt;/posix/</p>""")
      trim(true)
    }

    stringParam {
      name('LAUNCH_APPLICATION_NAME')
      defaultValue('app.referenceApp.elf')
      description("""<p>Name of the application to launch, or empty to manually launch.</p>""")
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

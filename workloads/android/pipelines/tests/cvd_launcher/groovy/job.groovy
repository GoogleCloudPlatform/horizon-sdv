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
pipelineJob('Android/Tests/CVD Launcher') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Cuttlefish Virtual Device Test Job</h3>
    <p>This job allows the user to test <a href="https://source.android.com/docs/devices/cuttlefish" target="_blank" title="Cuttlefish Virtual Device">CVD</a> images by configuring, the following mandatory parameters:</p>
    <h4 style="margin-bottom: 10px;">Job Overview</h4>
    <p>Virtual devices are initialized and remain active for a specified period, allowing users to interact with them via <a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank">MTK Connect</a>.<br/>
    The number of devices initialized is determined by the <code>NUM_INSTANCES</code> setting.<br/>
    After the <code>CUTTLEFISH_KEEP_ALIVE_TIME</code> period expires, the devices, testbenches, and VM instance are terminated in a controlled manner.</p>
    <h4 style="margin-bottom: 10px;">Mandatory Parameters</h4>
    <ul>
      <li><code>JENKINS_GCE_CLOUD_LABEL</code>: The label name of the cuttlefish instance to provision the virtual devices on.</li>
      <li><code>CUTTLEFISH_DOWNLOAD_URL</code>: The URL of the user's virtual device images to install and launch.</li>
    </ul>
    <p>Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>Users are responsible for specifying a valid cuttlefish instance - the job will block if the specified instance does not exist.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('JENKINS_GCE_CLOUD_LABEL')
      defaultValue("${JENKINS_GCE_CLOUD_LABEL}")
      description('''<p>The Jenkins GCE Clouds label for the Cuttlefish instance template, e.g.<br/></p>
        <ul>
          <li>cuttlefish-vm-main</li>
          <li>cuttlefish-vm-v1180</li>
        </ul>''')
      trim(true)
    }

    stringParam {
      name('CUTTLEFISH_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Storage URL pointing to the location of the Cuttlefish Virtual Device images and host packages, e.g.<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;</p>""")
      trim(true)
    }

    booleanParam {
      name('CUTTLEFISH_INSTALL_WIFI')
      defaultValue(false)
      description('''<p>Enable if wishing to install Wifi on the Cuttlefish Virtual Devices.<br/><br/>
        <b>Note:</b>
        <ul><li>Feature is experimental, impacts on performance and results differ per revision of Android.</li>
        <li>Refer to <code>wifi_connection_status.log</code> artifact to check device connectivity.</li></ul></p>''')
    }

    stringParam {
      name('CUTTLEFISH_MAX_BOOT_TIME')
      defaultValue('180')
      description('''<p>Android Cuttlefish max boot time in seconds.<br/>
         Wait on VIRTUAL_DEVICE_BOOT_COMPLETED across devices.</p>''')
      trim(true)
    }

    choiceParam {
      name('CUTTLEFISH_KEEP_ALIVE_TIME')
      choices(['0', '5', '15', '30', '60', '90', '120', '180', '240', '300', '480'])
      description('''<p>Time in minutes, to keep CVD alive before stopping.</p>''')
    }

    stringParam {
      name('NUM_INSTANCES')
      defaultValue('1')
      description('''<p>Number of guest instances to launch (num-instances option)</p>''')
      trim(true)
    }

    stringParam {
      name('VM_CPUS')
      defaultValue('16')
      description('''<p>Virtual CPU count (cpus option).</p>''')
      trim(true)
    }

    stringParam {
      name('VM_MEMORY_MB')
      defaultValue('16384')
      description('''<p>total memory available to guest (memory_mb option)</p>''')
      trim(true)
    }

    stringParam {
      name('CVD_ADDITIONAL_FLAGS')
      defaultValue('')
      description('''<p>Append additional flags to `cvd` command, e.g. --display0=width=1920,height=1080,dpi=160</p>''')
      trim(true)
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
      scriptPath('workloads/android/pipelines/tests/cvd_launcher/Jenkinsfile')
    }
  }
}

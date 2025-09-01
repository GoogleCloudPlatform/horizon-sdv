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
pipelineJob('Android/Tests/CTS Execution') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">CTS on Cuttlefish Job</h3>
    <p>This job allows users execute the <a href="https://source.android.com/docs/compatibility/cts" target="_blank">Compatibility Test Suite</a> (CTS) on their <a href="https://source.android.com/docs/devices/cuttlefish" target="_blank">Cuttlefish Virtual Device</a> (CVD) image builds. Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">Job Overview</h4>
    <p>The job runs on a cuttlefish-ready virtual machine instance (refer to the <i>CF Instance Template</i> job) together with running virtual devices (refer to <i>CVD Launcher</i> job). The Compatibility Test Suite is then executed across the virtual devices:
    <ul>
      <li><a href="https://source.android.com/docs/core/tests/tradefed" target="_blank">CTS Trade Federation</a></i> (<tt>cts-tradefed</tt>) - the test harness for CTS - can distribute / shard the tests across the multiple virtual devices </li>
      <li>The CTS version can either use the default <a href="https://source.android.com/docs/compatibility/cts/downloads" target="_blank">google-released</a> version or a test suite built by the <i>CTS Builder</i> job</i></li>
    </ul></p>
    <h4 style="margin-bottom: 10px;">Mandatory Parameters</h4>
    <ul>
      <li><code>JENKINS_GCE_CLOUD_LABEL</code>: The label name of the cuttlefish instance to provision the virtual devices on.</li>
      <li><code>CUTTLEFISH_DOWNLOAD_URL</code>: The URL of the user's virtual device images to install and launch.</li>
    </ul>
    <p>Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">MTK Connect Integration</h4>
    <p>User may choose to enable <a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank">MTK Connect</a> to allow users monitor virtual devices during testing.</p>
    <h4 style="margin-bottom: 10px;">Test Results and Debugging</h4>
    <p>Test results are stored with the job as artifacts.<br/>
    <p>Users can optionally keep the cuttlefish virtual devices alive for a finite amount of time after the CTS run has completed to facilitate debugging via MTK Connect. This option is only available when MTK Connect is enabled.</p>
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

    booleanParam {
      name('CTS_TEST_LISTS_ONLY')
      defaultValue(false)
      description('''<p>Skip tests and only generate the test plan and test module lists.<br/>
        You can use the following optional arguments to customize the listing:<br/>
        <ul><li><code>ANDROID_VERSION:</code> Specify the Android version to retrieve the correct listing.</li>
            <li><code>CTS_DOWNLOAD_URL:</code> Provide the URL for the CTS package if using your own version.</li></ul></p>''')
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

    choiceParam {
      name('ANDROID_VERSION')
      choices(['15', '14'])
      description('''<p>Select Android version: Android 15 or 14<br/>
        Essential for picking the correct test hardness</p>''')
    }

    stringParam {
      name('CTS_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Optional CTS test harness download URL.<br/>Use official CTS test harness (empty field) or one built from CTS Builder job and stored in GS Bucket, e.g.<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/CTS_Builder/&lt;BUILD_NUMBER&gt;/android-cts.zip</p>""")
      trim(true)
    }

    stringParam {
      name('CTS_TESTPLAN')
      defaultValue('cts-system-virtual')
      description('''<p>CTS Test plan to execute, e.g. cts-system-virtual (Android 15), cts-virtual-device-stable (Android 14) etc.</p>''')
      trim(true)
    }

    stringParam {
      name('CTS_MODULE')
      defaultValue('CtsDeqpTestCases')
      description('''<p>CTS module to test, or leave empty if all modules are to be tested.</p>''')
      trim(true)
    }

    stringParam {
      name('CUTTLEFISH_MAX_BOOT_TIME')
      defaultValue('180')
      description('''<p>Android Cuttlefish max boot time in seconds.<br/>
         Wait on VIRTUAL_DEVICE_BOOT_COMPLETED across devices.</p>''')
      trim(true)
    }

    stringParam {
      name('NUM_INSTANCES')
      defaultValue('10')
      description('''<p>Number of guest instances to launch (num-instances option)</p>''')
      trim(true)
    }

    stringParam {
      name('VM_CPUS')
      defaultValue('6')
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
      name('CTS_TIMEOUT')
      defaultValue('600')
      description('''<p>CTS Timeout in minutes for each test run.</p>''')
      trim(true)
    }

    booleanParam {
      name('MTK_CONNECT_ENABLE')
      defaultValue(false)
      description('''<p>Enable if wishing to use MTK Connect to view UI of CTS tests on virtual devices</p>''')
    }

    choiceParam {
      name('CUTTLEFISH_KEEP_ALIVE_TIME')
      choices(['0', '5', '15', '30', '60', '90', '120', '180'])
      description('''<p>Time in minutes, to keep CVD alive before stopping the devices and instance.</br>.
        Only applicable when <i>MTK_CONNECT_ENABLE</i> enabled so as to connect via HOST.</p>''')
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
      scriptPath('workloads/android/pipelines/tests/cts_execution/Jenkinsfile')
    }
  }
}


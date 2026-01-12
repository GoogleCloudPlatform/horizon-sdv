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
      <li>The CTS version can either use the default <a
href="https://source.android.com/docs/compatibility/cts/downloads" target="_blank">google-released</a> version or a test suite built by the <i>AAOS Builder</i> job with <i>AAOS_BUILD_CTS</i> enabled.</i></li>
    </ul></p>
    <h4 style="margin-bottom: 10px;">Mandatory Parameters</h4>
    <ul>
      <li><code>JENKINS_GCE_CLOUD_LABEL</code>: The label name of the cuttlefish instance to provision the virtual devices on.</li>
      <li><code>CUTTLEFISH_DOWNLOAD_URL</code>: The URL of the user's virtual device images to install and launch.</li>
    </ul>
    <p>Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">Resources</h4>
    <p>Ensure you select appropriate values for <code>NUM_INSTANCES</code>, <code>VM_CPUS</code>, <code>VM_MEMORY_MB</code> that align with the available resources of the VM instance used for test, defined by <code>JENKINS_GCE_CLOUD_LABEL</code>.</p>
    <p>CVD will automatically resize should users define more than the default resources CVD is configured for (10), e.g <code>NUM_INSTANCES=15</code> will resize the CVD host service to support 15 devices.</p>
    <h4 style="margin-bottom: 10px;">MTK Connect Integration</h4>
    <p>User may choose to enable <a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank">MTK Connect</a> to allow users monitor virtual devices during testing.</p>
    <h4 style="margin-bottom: 10px;">Test Results and Debugging</h4>
    <p>Test results are stored with the job as artifacts.<br/>
    <p>Users can optionally keep the cuttlefish virtual devices alive for a finite amount of time after the CTS run has completed to facilitate debugging via MTK Connect. This option is only available when MTK Connect is enabled.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p><ul><li>Users are responsible for specifying a valid cuttlefish instance - the job will block if the specified instance does not exist.</li>
           <li>If tests timeout, then create the Cuttflefish instance template with a larger run duration, see <code>MAX_RUN_DURATION</code>, increase or set to 0 to ignore max runtime.</li></ul></p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('JENKINS_GCE_CLOUD_LABEL')
      defaultValue("${JENKINS_GCE_CLOUD_LABEL}")
      description('''<p>The Jenkins GCE Clouds label for the Cuttlefish instance template, e.g.<br/></p>
        <ul>
          <li>cuttlefish-vm-v1350</li>
          <li>cuttlefish-vm-main</li>
          <li>cuttlefish-vm-v1350-arm64</li>
          <li>cuttlefish-vm-main-arm64</li>
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
      description("""<p>Mandatory: Storage URL pointing to the location of the Cuttlefish Virtual Device images and host packages, e.g.<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;<br/><br/>
        <b>Note:</b>
          <ul><li>if build number is less than 2 digits, then zero pad , i.e. 1 to 9 must be 01 to 09.</li></ul)</p>""")
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
      choices(['16', '15', '14'])
      description('''<p>Select Android version: Android 16, 15 or 14<br/>
        Essential for picking the correct test hardness</p>''')
    }

    stringParam {
      name('CTS_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Optional CTS test harness download URL.<br/>Use official CTS test harness (empty field) or one built from AAOS Builder job and stored in GCS Bucket,
e.g.<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;/android-cts.zip<br/><br/>
        <b>Note:</b>
          <ul><li>if build number is less than 2 digits, then zero pad , i.e. 1 to 9 must be 01 to 09.</li></ul)</p>""")
      trim(true)
    }

    stringParam {
      name('CTS_TESTPLAN')
      defaultValue('cts-system-virtual')
      description('''<p>CTS Test plan to execute, e.g.</p>
        <ul><li>Android 15 and later: <code>cts-system-virtual</code></li>
            <li>Android 14: <code>cts-virtual-device-stable</code></li></ul>''')
      trim(true)
    }

    stringParam {
      name('CTS_MODULE')
      defaultValue('')
      description('''<p>Optional: This defines the CTS test module that will be run, e.g.</p>
        <ul><li>Android 15 and later: <code>CtsDeqpTestCases</code></li>
            <li>Android 14: <code>CtsHostsideNumberBlockingTestCases</code></li></ul>
        <p>If left empty, all CTS test modules will be run.</p>''')
      trim(true)
    }

    stringParam {
      name('CTS_RETRY_STRATEGY')
      defaultValue('RETRY_ANY_FAILURE')
      description('''<p>CTS <a href="https://source.android.com/reference/tradefed/com/android/tradefed/retry/RetryStrategy" target="_blank">--retry-strategy</a> option.</p>''')
      trim(true)
    }

    stringParam {
      name('CTS_MAX_TESTCASE_RUN_COUNT')
      defaultValue('2')
      description('''<p>CTS <a href="https://source.android.com/docs/core/tests/tradefed/testing/through-tf/auto-retry" target="_blank">--max-testcase-run-count</a> option dependent on retry strategy.</p>''')
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
      defaultValue('7')
      description('''<p>Number of guest instances to launch (num-instances option)</p>''')
      trim(true)
    }

    stringParam {
      name('VM_CPUS')
      defaultValue('4')
      description('''<p>Virtual CPU count (cpus option).</p>''')
      trim(true)
    }

    stringParam {
      name('VM_MEMORY_MB')
      defaultValue('8192')
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

    booleanParam {
      name('MTK_CONNECT_PUBLIC')
      defaultValue(false)
      description('''<p>When checked, the MTK Connect testbench is visible to everyone.<br/>
        By default, testbenches are private and only visible to their creator and MTK Connect administrators.</p>''')
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
      description('''<p>Optional: Append additional optional flags to <code>cvd</code> command, e.g.
        <ul><li><code>--setupwizard_mode DISABLED --enable_host_bluetooth false --gpu_mode guest_swiftshader</code></li>
            <li><code>--display0=width=1920,height=1080,dpi=160</code></li>
            <li><code>--verbosity=DEBUG</code></li></ul></p>''')
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


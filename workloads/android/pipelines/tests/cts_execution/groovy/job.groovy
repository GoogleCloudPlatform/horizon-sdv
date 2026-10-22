// Copyright (c) 2025-2026 Accenture, All Rights Reserved.
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

def model = ('${GEMINI_MODEL}' && '${GEMINI_MODEL}' != 'null' && !'${GEMINI_MODEL}'.contains('${')) ? "--model ${'${GEMINI_MODEL}'} " : ""

pipelineJob('Android/Tests/CTS Execution') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">CTS on Cuttlefish Job</h3>
    <p>This job allows users to execute the <a href="https://source.android.com/docs/compatibility/cts" target="_blank" rel="noopener noreferrer">Compatibility Test Suite</a> (CTS) on their <a href="https://source.android.com/docs/devices/cuttlefish" target="_blank" rel="noopener noreferrer">Cuttlefish Virtual Device</a> (CVD) image builds. Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">Job Overview</h4>
    <p>The job runs on a cuttlefish-ready virtual machine instance (refer to the <i>CF Instance Template</i> job) together with running virtual devices (refer to <i>CVD Launcher</i> job). The Compatibility Test Suite is then executed across the virtual devices:</p>
    <ul>
      <li><a href="https://source.android.com/docs/core/tests/tradefed" target="_blank" rel="noopener noreferrer">CTS Trade Federation</a> (<tt>cts-tradefed</tt>) - the test harness for CTS - can distribute / shard the tests across the multiple virtual devices </li>
      <li>The CTS version can either use the default <a href="https://source.android.com/docs/compatibility/cts/downloads" target="_blank" rel="noopener noreferrer">google-released</a> version or a test suite built by the <i>AAOS Builder</i> job with <i>AAOS_BUILD_CTS</i> enabled.</li>
    </ul>
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
    <p>User may choose to enable <a href="http://${HORIZON_DOMAIN}/mtk-connect/portal/testbenches" target="_blank" rel="noopener noreferrer">MTK Connect</a> to allow users to monitor virtual devices during testing.</p>
    <h4 style="margin-bottom: 10px;">Test Results and Debugging</h4>
    <p>Test results are stored with the job as artifacts.</p>
    <p>Users can optionally keep the cuttlefish virtual devices alive for a finite amount of time after the CTS run has completed to facilitate debugging via MTK Connect. This option is only available when MTK Connect is enabled.</p>
    <p><b>Agentic AI (experimental):</b></p>
    <ul>
      <li>When debugging errors or failures, consider the <i>CVD Launcher</i> job for more
          detailed analysis — its optional AI Review is tailored to Cuttlefish / virtual-device
          triage.</li>
      <li>Set <code>GEMINI_ANALYSE_ON_SUCCESS=true</code> to run AI Review on <b>passing</b>
          builds too — required if you want Gemini artifacts
          (<code>gemini-assist/</code>, <code>step*_output.md</code>) archived alongside a
          successful run.</li>
    </ul>
    <p><b>Utilities — Gemini AI Assistant:</b></p>
    <ul>
      <li>Same prompts, <code>skills.yaml</code>, and artifact layout as in-pipeline AI Review —
          iterate on triage or proposed fixes <b>without</b> re-running a lengthy CTS or
          Cuttlefish job.</li>
      <li>Supply the workspace or archived <code>test-results/</code> as inputs.</li>
      <li>For <b>CVD/Cuttlefish-focused</b> review (success <b>or</b> failure), upload the
          <b>CVD Launcher</b> sequenced prompts and <code>skills.yaml</code> (<b>not</b> the CTS
          set) — their Phase 0 boot preflight auto-routes between boot-failure and runtime-issue
          lanes.</li>
    </ul>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <ul>
      <li>AAOS SDV support included, please ensure you specify desired values for CUTTLEFISH_DOWNLOAD_URL, CVD_COMMAND_LINE and, if required, also CTS_DOWNLOAD_URL.</li>
      <li>Users are responsible for specifying a valid cuttlefish instance - the job will block if the specified instance does not exist.</li>
      <li>If tests timeout, then create the Cuttlefish instance template with a larger run duration, see <code>MAX_RUN_DURATION</code>, increase or set to 0 to ignore max runtime.</li>
    </ul>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  environmentVariables {
    env('GEMINI_PREVIEW_FEATURES', ${GEMINI_PREVIEW_FEATURES})
    env('GEMINI_LOCATION_GLOBAL', ${GEMINI_LOCATION_GLOBAL})
    env('GEMINI_MODEL', '${GEMINI_MODEL}')
  }

  parameters {
    separator {
      name('Environment')
      sectionHeader('Environment')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('JENKINS_GCE_CLOUD_LABEL')
      defaultValue("${JENKINS_GCE_CLOUD_LABEL}")
      description('''<p>The Jenkins GCE Clouds label for the Cuttlefish instance template, e.g.<br/></p>
        <ul>
          <li>cuttlefish-vm-v1410</li>
          <li>cuttlefish-vm-main</li>
          <li>cuttlefish-vm-v1410-arm64</li>
          <li>cuttlefish-vm-main-arm64</li>
        </ul>''')
      trim(true)
    }

    separator {
      name('CTS mode')
      sectionHeader('CTS mode')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('CTS_TEST_LISTS_ONLY')
      defaultValue(false)
      description('''<p>Skip tests and only generate the test plan and test module lists.<br/>
        You can use the following optional arguments to customize the listing:<br/>
        <ul><li><code>ANDROID_VERSION:</code> Specify the Android version to retrieve the correct listing.</li>
            <li><code>CTS_DOWNLOAD_URL:</code> Provide the URL for the CTS package if using your own version.</li></ul></p>''')
    }

    separator {
      name('Cuttlefish image & boot')
      sectionHeader('Cuttlefish image & boot')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('CUTTLEFISH_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Mandatory: Storage URL pointing to the location of the Cuttlefish Virtual Device images and host packages,<br/><br/>
        Example for AAOS:<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;<br/><br/>
        Example for AAOS SDV:<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_SDV_Builder/&lt;BUILD_NUMBER&gt;<br/><br/>
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

    stringParam {
      name('CUTTLEFISH_MAX_BOOT_TIME')
      defaultValue('180')
      description('''<p>Android Cuttlefish max boot time in seconds.<br/>
         Wait on VIRTUAL_DEVICE_BOOT_COMPLETED across devices.</p>''')
      trim(true)
    }

    separator {
      name('CTS harness & plan')
      sectionHeader('CTS harness & plan')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
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
      description("""<p>Optional CTS test harness download URL.<br/>Use official CTS test harness (empty field) or one built from AAOS (SDV) Builder job and stored in GCS Bucket,<br/><br/>
        Example for AAOS:<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;/android-cts.zip<br/><br/>
        Example for AAOS SDV:<br/>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_SDV_Builder/&lt;BUILD_NUMBER&gt;/android-cts.zip<br/><br/>
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

    separator {
      name('Virtual devices & CTS runtime')
      sectionHeader('Virtual devices & CTS runtime')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
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

    separator {
      name('MTK Connect')
      sectionHeader('MTK Connect')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
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

    stringParam {
      name('MTK_CONNECT_TUNNEL_PORT')
      defaultValue('8555')
      description('''<p>ADB tunnel <code>caller.port</code> passed to MTK Connect <code>create-testbench.js</code> (see <code>workloads/common/mtk-connect</code>). Override if the port conflicts on the agent.</p>''')
      trim(true)
    }

    choiceParam {
      name('CUTTLEFISH_KEEP_ALIVE_TIME')
      choices(['0', '5', '15', '30', '60', '90', '120', '180'])
      description('''<p>Time in minutes, to keep CVD alive before stopping the devices and instance.<br/>
        Only applicable when <i>MTK_CONNECT_ENABLE</i> enabled so as to connect via HOST.</p>''')
    }

    separator {
      name('CVD command line')
      sectionHeader('CVD command line')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('CVD_COMMAND_LINE')
      defaultValue('/usr/bin/cvd create --noresume -config=auto -report_anonymous_usage_stats=no --num_instances="${NUM_INSTANCES}" --cpus="${VM_CPUS}" --memory_mb="${VM_MEMORY_MB}" --console=true --setupwizard_mode DISABLED --enable_host_bluetooth false --gpu_mode guest_swiftshader')
      description('''<p>Full <code>/usr/bin/cvd</code> command line. The default skips the setup wizard, disables host Bluetooth, and uses guest SwiftShader (software rendering) for typical CI agents without GPU passthrough; edit to suit your hosts. <code>NUM_INSTANCES</code>, <code>VM_CPUS</code>, and <code>VM_MEMORY_MB</code> in the default derive from the respective parameters in the job automatically.<br/><br/>
        For AAOS SDV image, use one of the options below:<br/><br/>
        Option 1: single instance with default configurations (ignores: NUM_INSTANCES, VM_CPUS, VM_MEMORY_MB):<br/>
        <code>./sdv-cf create --instance_name=instance1</code><br/><br/>
        Option 2: multiple instances or specific demands (as specified with: NUM_INSTANCES, VM_CPUS, VM_MEMORY_MB):<br/>
        <code>/usr/bin/cvd create -guest_enforce_security="true" -report_anonymous_usage_stats=y -extra_bootconfig_args="androidboot.sdv.instance_name=instance1 androidboot.sdv.boot_mode=locked androidboot.virt.address=3 androidboot.sdv.init_open_dice.sample_file=dice_handover_instance1 androidboot.sdv.vvmfactorytrust=c779a73d0595a6814ba0414a419e99ad4026ad0feb603f2ad80ee6a9e4d1adb7 androidboot.sdv.keymint.rpc.hbk=799da7577efd41d5b27810c5952fcec0291cbcfd687e77ac9a6cec8370651b1d androidboot.sdv.authz.enable=true androidboot.sdv.preprovisioned_vvmtruststore=img androidboot.sdv.ignore_avb_state=true androidboot.sdv.someip.enable=true" -default_vvmtruststore_file_name="vvmtruststore_preprovisioned_1.img" --enable_sandbox="true" --num_instances="${NUM_INSTANCES}" --cpus="${VM_CPUS}" --memory_mb="${VM_MEMORY_MB}"</code></p>''')
      trim(true)
    }

    separator {
      name('Agentic AI: Configuration (Experimental)')
      sectionHeader('Agentic AI: Configuration (Experimental)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('ENABLE_GEMINI_AI_ASSISTANT')
      defaultValue(${ENABLE_GEMINI_AI_ASSISTANT})
      description('''<p>Enable Gemini AI to support in diagnosis of test failures.</p>''')
    }

    choiceParam {
      name('AI_REVIEW_RUNTIME')
      description('''<p>AI Review runtime: <code>gemini-cli</code> (CLI, default) or <code>agy</code> (Antigravity SDK).</p>''')
      choices(['gemini-cli', 'agy'])
    }

    choiceParam {
      name('GEMINI_AI_EXECUTION_TIMEOUT_MINUTES')
      description('''
        <p>Maximum duration in minutes allowed for the Gemini assistant to run before automatic termination.</p>
        <p><i>Note: This safety limit prevents hung processes and optimizes resource usage.</i></p>''')
      choices(['60', '45', '30', '20', '10', '90', '120'])
    }

    booleanParam {
      name('GEMINI_ANALYSE_ON_SUCCESS')
      defaultValue(false)
      description('''<p>Optional: also run the AI Review stage when the job result is <b>SUCCESS</b> (default is failure-only).<br/> Use this to surface hidden issues in CTS/CVD logs on otherwise-green runs.</p>''')
    }

    stringParam {
      name('GEMINI_COMMAND_LINE')
      defaultValue("gemini ${model} --yolo --output-format json")
      description('''<p>Interface for the headless <a href="https://geminicli.com/docs/cli/headless/" target="_blank">gemini-cli</a>.</p>
        <p>Use this to specify settings such as the <a href="https://ai.google.dev/gemini-api/docs/models" target="_blank">Gemini model</a>.</p>
        <p><b>Note:</b> Prompts are piped via <b>stdin</b> and output is redirected to a JSON file.</p>''')
      trim(true)
    }

    separator {
      name('Agentic AI: Storage Options')
      sectionHeader('Agentic AI: Storage Options')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('CTS_ARTIFACT_STORAGE_SOLUTION')
      defaultValue('GCS_BUCKET')
      description('''<p>CTS Artifact Storage:<br/>
        <ul><li>GCS_BUCKET will store to cloud bucket storage</li>
        <li>Empty will result in nothing stored</li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('STORAGE_BUCKET_DESTINATION')
      defaultValue('')
      description('''<p>Storage bucket destination:<br/>
        Leave empty for build to create default, e.g. gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Tests/CTS_Execution/<BUILD_NUMBER><br/>
        Alternatively, override path, e.g gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Releases/010129</p>''')
      trim(true)
    }

    stringParam {
      name('STORAGE_LABELS')
      defaultValue('')
      description('''<p>Optional, list one or more labels to be applied to the artifacts being uploaded to storage.
      <br>Use spaces or commas to seperate. Neither keys nor values should contain spaces. (e.g. Release=X.Y.Z,Workload=Android)</p>''')
      trim(true)
    }
  }

  properties {
    copyArtifactPermission {
      projectNames('/Android/Tests/CTS Execution')
    }
  }

  logRotator {
    artifactDaysToKeep(60)
    artifactNumToKeep(100)
    daysToKeep(7)
    numToKeep(50)
  }

  definition {
    cpsScm {
      lightweight()
      scm {
        git {
          remote {
            url("${HORIZON_SCM_URL}")
            credentials('jenkins-scm-creds')
          }
          branch("*/${HORIZON_SCM_BRANCH}")
        }
      }
      scriptPath('workloads/android/pipelines/tests/cts_execution/Jenkinsfile')
    }
  }
}


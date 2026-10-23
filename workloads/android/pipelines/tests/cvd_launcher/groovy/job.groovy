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

def cvdJobFullName = 'Android/Tests/CVD Launcher'

pipelineJob(cvdJobFullName) {
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
    <ul>
      <li>AAOS SDV support included, please ensure you specify desired values for CUTTLEFISH_DOWNLOAD_URL and CVD_COMMAND_LINE.</li>
      <li>Users are responsible for specifying a valid cuttlefish instance - the job will block if the specified instance does not exist.</li>
    </ul>
    <p><b>Agentic AI (experimental):</b></p>
    <ul>
      <li>When <i>Agentic AI</i> is enabled, this job runs a sequenced triage over Cuttlefish /
          CVD logs (guest <code>kernel.log</code>, <code>launcher.log</code>, logcat, host
          <code>cvd-*.log</code>) after a failed run.</li>
      <li>Experimental: non-deterministic models can produce slightly different summaries on
          repeat runs — re-run Gemini or tighten diagnostics collection if output looks
          incomplete.</li>
      <li>Set <code>GEMINI_ANALYSE_ON_SUCCESS=true</code> to run AI Review on <b>passing</b>
          builds too — required if you want Gemini artifacts
          (<code>gemini-assist/</code>, <code>step*_output.md</code>) archived alongside a
          successful run.</li>
    </ul>
    <p><b>Utilities — Gemini AI Assistant:</b></p>
    <ul>
      <li>Same prompts, <code>skills.yaml</code>, and artifact layout as AI Review on this
          pipeline — iterate on triage or proposed fixes <b>without</b> re-running a lengthy
          CVD or CTS job.</li>
      <li>Supply the workspace or archived <code>test-results/</code> as inputs.</li>
      <li>The Phase 0 boot preflight in the CVD <code>skills.yaml</code> auto-routes between
          boot-failure and runtime-issue lanes — same prompts cover success <b>and</b>
          failure.</li>
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
      name('Cuttlefish image & boot')
      sectionHeader('Cuttlefish image & boot')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('CUTTLEFISH_DOWNLOAD_URL')
      defaultValue('')
      description("""<p>Mandatory: Storage URL pointing to the location of the Cuttlefish Virtual Device images and host packages for it.<br/><br/>
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

    choiceParam {
      name('CUTTLEFISH_KEEP_ALIVE_TIME')
      choices(['0', '5', '15', '30', '60', '90', '120', '180', '240', '300', '480'])
      description('''<p>Time in minutes, to keep CVD alive before stopping.</p>''')
    }

    separator {
      name('Virtual device sizing')
      sectionHeader('Virtual device sizing')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
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
      defaultValue('8192')
      description('''<p>total memory available to guest (memory_mb option)</p>''')
      trim(true)
    }

    separator {
      name('MTK Connect')
      sectionHeader('MTK Connect')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
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
      description('''<p>Enable <b>Gemini AI Assistant</b> triage on failure (same pipeline stage pattern as CTS Execution). Uses sequenced prompts under <code>cvd_launcher/prompt/sequenced</code> (guest <code>kernel.log</code>, <code>launcher.log</code>, userspace logcat under <code>**/logs/</code>, host <code>cvd</code> logs). <b>Experimental:</b> output may vary slightly between runs; you may <b>re-run</b> the job to get another pass over the same artifacts. Not a substitute for engineering review of images and bootconfig.</p>''')
    }

    choiceParam {
      name('AI_REVIEW_RUNTIME')
      description('''<p>AI Review runtime: <code>agy</code> (Antigravity SDK, default) or <code>gemini-cli</code> (CLI).</p>''')
      choices(['agy', 'gemini-cli'])
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
      description('''<p>Headless <a href="https://geminicli.com/docs/cli/headless/" target="_blank">gemini-cli</a> invocation.</p>''')
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
      description('''<p>Gemini output storage: GCS_BUCKET or empty to skip upload.</p>''')
      trim(true)
    }

    stringParam {
      name('STORAGE_BUCKET_DESTINATION')
      defaultValue('')
      description('''<p>Optional override for Gemini artifact destination (see CTS Execution).</p>''')
      trim(true)
    }

    stringParam {
      name('STORAGE_LABELS')
      defaultValue('')
      description('''<p>Optional GCS object metadata labels for stored Gemini artifacts.</p>''')
      trim(true)
    }
  }

  properties {
    copyArtifactPermission {
      // Match CTS Execution pattern so self-copy for AI Review resolves (full name with leading slash).
      projectNames('/Android/Tests/CVD Launcher')
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
      scriptPath('workloads/android/pipelines/tests/cvd_launcher/Jenkinsfile')
    }
  }
}

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
pipelineJob('Android/Environment/Development Build Instance') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Development Build Instance Creation Job</h3>
    <p>This job allows creation of a temporary build instance that can be used to aid development and testing of builds.<br/>
    <h4 style="margin-bottom: 10px;">Instance Details</h4>
    <p>Instances can be expensive and therefore there is a maximum up-time before the instance will automatically be terminated.</p>
    <h4 style="margin-bottom: 10px;">Accessing the Instance</h4>
    <p>Access the instance via <code>kubectl</code> command line tool. Example command:</p>
    <p><code>kubectl exec -it -n jenkins &lt;pod name&gt; -- bash</code></p>
    <p>Reference <a href="https://docs.cloud.google.com/kubernetes-engine/enterprise/multicluster-management/gateway" target="_blank">Fleet management</a> to fetch credentials for a fleet-registered cluster to be used in Connect Gateway, e.g. <br/>
    <ul><li><code>gcloud container fleet memberships list</code></li>
        <li><code>gcloud container fleet memberships get-credentials sdv-cluster</code></li></ul></p>
    <p>Alternatively access Host via MTK Connect by enabling MTK_CONNECT_ENABLE.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>Users are responsible for saving their own work to persistent storage before expiry.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    choiceParam {
      name('ANDROID_VOLUME')
      description('''<p>Android disk pool to use for the build cache:</p>
          <ul>
            <li>16: Use the Android 16 disk pool.</li>
            <li>15: Use the Android 15 disk pool.</li>
            <li>14: Use the Android 14 disk pool.</li>
            <li>16-rpi: Use the Android 16 RPi disk pool.</li>
            <li>15-rpi: Use the Android 15 RPi disk pool.</li>
            <li>14-rpi: Use the Android 14 RPi disk pool.</li>
            <li>abfs: Select when using ABFS builds with persisted cache.</li>
          </ul>
        <p>For ABFS build instances you select the <code>abfs</code> version to mount the ABFS cache persistent volume.</p>''')
      choices(['16', '15', '14', '16-rpi', '15-rpi', '14-rpi', 'abfs'])
    }

    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<p>Image tag for the builder image.</p>''')
      trim(true)
    }

    choiceParam {
      name('INSTANCE_MAX_UPTIME')
      choices(['1', '2', '4', '8'])
      description('''<p>Time in hours to keep instance alive.</p>''')
    }

    booleanParam {
      name('MTK_CONNECT_ENABLE')
      defaultValue(false)
      description('''<p>Enable if wishing to use MTK Connect to connect to the host instance.</p>''')
    }

    booleanParam {
      name('MTK_CONNECT_PUBLIC')
      defaultValue(false)
      description('''<p>When checked, the MTK Connect testbench is visible to everyone.<br/>
        By default, testbenches are private and only visible to their creator and MTK Connect administrators.</p>''')
    }

    stringParam {
      name('NUM_HOST_INSTANCES')
      defaultValue('1')
      description('''<p>Number of host instances to create.<p>
        <p>i.e. the number of devices to create in MTK Connect testbench.</p>''')
      trim(true)
    }

    separator {
      name('AOSP Mirror Parameters')
      sectionHeader('AOSP Mirror Parameters')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('USE_LOCAL_AOSP_MIRROR')
      defaultValue(${USE_LOCAL_AOSP_MIRROR})
      description('''<p>If checked, the instance will mount the AOSP Mirror setup in your GCP project to fetch Android source code during <i>repo sync</i>.<br/>
        <b>Note:</b> The AOSP Mirror must be setup prior to running this job. If not setup, the job will fail.<br> The setup jobs are in folder <i>Android Workflows > Environment > Mirror</i>.<br/><br/></p>''')
    }

    stringParam {
      name('AOSP_MIRROR_DIR_NAME')
      defaultValue("${AOSP_MIRROR_DIR_NAME}")
      description('''<p>The directory name on the Filestore volume where the Mirror is located.<br/>
        <b>Note:</b> This is required if <code>USE_LOCAL_AOSP_MIRROR</code> is checked.</p>
        <b>Example:</b> If you provided '<i><code>my-mirror</code></i>' when creating the mirror, provide the same value here.<br/><br/></p>''')
      trim(true)
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
      scriptPath('workloads/android/pipelines/environment/dev_instance_build/Jenkinsfile')
    }
  }
}

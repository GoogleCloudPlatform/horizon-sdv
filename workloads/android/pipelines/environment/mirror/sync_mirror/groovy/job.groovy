// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Description:
// This groovy job is used by the Seed Workloads Pipeline to define template
// and parameters for pipeline that executes sync_mirror operation of Mirror
// setup.

pipelineJob('Android/Environment/Mirror/Sync Mirror') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Sync existing Mirror or Create new one</h3>

    <p>This job syncs an existing mirror or creates a new one (if not present) in specified directory on the NFS volume, with specified remote manifest repository via parameters.</p>

    <h4 style="margin-bottom: 10px;">Periodic Sync</h4>
    <p>To keep the Mirror up-to-date with remote, you can set a schedule for this job to run periodically by following below steps:</p>
    <ol>
      <li>Click on the <strong><code>Configure</code></strong> option in the left-hand menu of this job.</li>
      <li>Scroll down to the <strong><code>Parameters</code></strong> section under checkbox <strong><code>'This project is parameterized'</code></strong>.</li>
      <li>Set the parameters as per your requirements. <b>Note:</b> You can set the schedule either for a single mirror or for all mirrors (by selecting parameter <strong><code>'SYNC_ALL_EXISTING_MIRRORS'</code></strong>).</li>
      <li>Scroll down to the <strong><code>Triggers</code></strong> section.</li>
      <li>Check the box for <strong><code>Build periodically</code></strong>.</li>
      <li>In the <strong><code>Schedule</code></strong> field, enter a cron expression that defines how often you want the job to run. For example, to run the job daily at midnight, you would enter: <i><code>H H * * *</code></i></li>
      <li>See the <a href="https://www.jenkins.io/doc/book/pipeline/syntax/#triggers">Jenkins cron syntax documentation</a> for more details.</li>
      <li>Click the <strong><code>Save</code></strong> button at the bottom of the page to apply your changes.</li>
    </ol>

    <h4 style="margin-bottom: 10px;">Preset Properties (Non-configurable):</h4>
    <ul>
      <li>DISK_NAME: <i><code>${AOSP_MIRROR_PRESET_FILESTORE_PVC_NAME}</code></i></li>
      <li>DISK_MOUNT_PATH_IN_CONTAINER: <i><code>${AOSP_MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER}</code></i></li>
      <li>MIRROR_ROOT_SUBDIRECTORY_IN_CONTAINER (All mirrors live inside this directory): <i><code>${AOSP_MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER}/${AOSP_MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME}</code></i></li>
      <li>REGION: <i><code>${CLOUD_REGION}</code></i></li>
      <li>NETWORK: <i><code>${AOSP_MIRROR_PRESET_NETWORK_NAME}</code></i></li>
      <li>SUBNETWORK: <i><code>${AOSP_MIRROR_PRESET_SUBNETWORK_NAME}</code></i></li>
      <li>PROJECT: <i><code>${CLOUD_PROJECT}</code></i></li>
    </ul>

    <h4 style="margin-bottom: 10px;">Notes</h4>
    <ul>
      <li>If the mirror PVC `<i><code>${AOSP_MIRROR_PRESET_FILESTORE_PVC_NAME}</code></i>` does NOT exists, this job will fail. Run the 'Mirror > Create Mirror Infra' pipeline prior to this job.</li>
      <li>Multiple mirrors can be created within the same NFS-based volume, but each mirror must have a unique directory name.</li>
      <li>If the mirror with same directory name already exists, executing this job will just update the existing mirror.</li>
      <li>Once the Mirror Manifest URL is set, it cannot be changed without recreating the Mirror.</li>
    </ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  parameters {
    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<strong>REQUIRED:</strong> The Docker image tag to be used for the execution environment.<br/>
        <b>Note:</b> Ensure you have executed that image build job prior to running this job, so that the required Docker image is available in your GCP project.
      ''')
      trim(true)
    }

    booleanParam {
      name('SYNC_ALL_EXISTING_MIRRORS')
      defaultValue(false)
      description('''Optional: If checked, ALL existing mirrors within the NFS volume will be synced/updated based on their parameter values with which they were last executed (stored in metadata file).<br/>
        <b>Note:</b>
        <ul>
          <li>This will ignore all other parameters related to single mirror setup.</li>
          <li>Ideal for scheduled periodic updates of all managed mirrors.</li>
        </ul>
      ''')
    }

    separator {
      name('SINGLE_MIRROR_PARAMETERS')
      sectionHeader("Single Mirror Parameters (Required if 'SYNC_ALL_EXISTING_MIRRORS' is NOT selected)")
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('MIRROR_DIR')
      defaultValue('')
      description('''The directory name on the Filestore NFS volume where the Mirror is located (or will be created).<br/>
        <b>Example:</b> '<i><code>my-mirror</code></i>'<br/>
        <b>Note:</b> If you provide '<i><code>my-mirror</code></i>' as value, the absolute container path of the mirror will be '<i><code>${AOSP_MIRROR_PRESET_FILESTORE_PVC_MOUNT_PATH_IN_CONTAINER}/${AOSP_MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME}/my-mirror</code></i>', where '<i><code>${AOSP_MIRROR_PRESET_MIRROR_ROOT_SUBDIR_NAME}</code>.</i>' is the root subdirectory for all mirrors.
      ''')
      trim(true)
    }

    stringParam {
      name('MIRROR_MANIFEST_URL')
      defaultValue('https://android.googlesource.com/platform/manifest')
      description('''The URL of the manifest repository to be used for the Mirror.<br/>
        <b>Note:</b> Once set for a mirror, this value cannot be changed without recreating the mirror.
      ''')
      trim(true)
    }

    stringParam {
      name('MIRROR_MANIFEST_REF')
      defaultValue('android-16.0.0_r3')
      description('''The manifest branch or tag to be used for the Mirror.<br/>
        <b>Note:</b> This value can be updated in subsequent syncs to point to a different branch or tag.
      ''')
      trim(true)
    }

    stringParam {
      name('MIRROR_MANIFEST_FILE')
      defaultValue('default.xml')
      description('''The manifest file name to be used for the Mirror.<br/>
        <b>Note:</b> This value can be updated in subsequent syncs to point to a different manifest file.
      ''')
      trim(true)
    }

    stringParam {
      name('REPO_SYNC_JOBS')
      defaultValue("${REPO_SYNC_JOBS}")
      description('''Number of parallel sync jobs for <i>repo sync</i>.<br/>
        <b>Note:</b>
        <ul>
          <li>Default value is defined by the Android Seed job.</li>
          <li>Max recommended value for mirror is 4 due to rate-limiting constraints set by Google.</li>
        </ul>
      ''')
      trim(true)
    }
  }

  // Block build if certain jobs are running.
  blockOn('Android/Environment/Mirror/.*(Create|Delete|Sync).*') {
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
      scriptPath('workloads/android/pipelines/environment/mirror/sync_mirror/Jenkinsfile')
    }
  }
}

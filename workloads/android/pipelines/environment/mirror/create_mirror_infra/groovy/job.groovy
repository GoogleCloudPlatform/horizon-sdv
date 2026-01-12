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
// and parameters for pipeline that executes create_mirror_infra operation of
// an NFS-based Mirror setup.

pipelineJob('Android/Environment/Mirror/Create Mirror Infra') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Provision Mirror Infrastructure</h3>

    <p>This job provisions the resources required for an NFS-based Mirror setup in your existing GCP project. Also, allows you to specify size of the mirror volume.</p>

    <p>It executes the following steps:</p>
    <ol>
      <li>Creates a Filestore instance in the same region this platform is running.</li>
      <li>Creates a Persistent Volume (PV) and Persistent Volume Claim (PVC) using the Filestore instance.</li>
      <li>The size of the mirror volume is determined by the <strong><code>MIRROR_VOLUME_CAPACITY_GB</code></strong> parameter. Minimum size is 1024Gi (1Ti).</li>
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
      <li><b>Multiple mirrors</b> can be created within the same NFS-based mirror volume, but each mirror must have a unique directory name.</li>
      <li>To create a new mirror or update an existing one, execute the job `<i><code>Mirror > Sync Mirror</code></i>`.</li>
    </ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  parameters {
    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<strong>REQUIRED:</strong> The image tag for the Docker image to be used as environment for this job.<br>
      <b>Note:</b> Ensure you have executed that image build job prior to running this job, so that the required Docker image is available in your GCP project.''')
      trim(true)
    }

    stringParam {
      name('MIRROR_VOLUME_CAPACITY_GB')
      defaultValue('2048')
      description('''<strong>REQUIRED:</strong> Size of the mirror volume to be created in GiB.<br>
      <b>Note:</b>
      <ul>
        <li>Minimum size is 1024Gi (1Ti).</li>
        <li>This size is for the entire Filestore NFS volume, which can host multiple mirrors (each in its own unique directory).</li>
        <li>Size CANNOT be changed once created. You will need to delete the existing volume and create a new one with the desired size.</li>
        <li>Example: A full AOSP Mirror consumes around 1946Gi (1.9Ti) of storage. So 2048Gi of total volume capacity is recommended.</li>
      </ul>''')
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
      scriptPath('workloads/android/pipelines/environment/mirror/create_mirror_infra/Jenkinsfile')
    }
  }
}

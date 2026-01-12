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
// and parameters for pipeline that executes delete-mirror operation of the
// NFS-based Mirror setup.

pipelineJob('Android/Environment/Mirror/Delete Mirror') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Delete Existing Mirror</h3>

    <p><strong>WARNING</strong>: This action is IRREVERSIBLE and will permanently DELETE the specified mirror (or entire setup, if specified) for the Mirror in your existing GCP project.</p>

    <p>It executes the following steps:</p>
    <ol>
      <li>
        <strong>If a specific mirror directory is provided via the <code>MIRROR_DIR_TO_DELETE</code> parameter:</strong> Deletes only that specified mirror directory and all its contents in the mirror storage volume.
      </li>
      <li>
        <strong>If the <code>DELETE_ENTIRE_MIRROR_SETUP</code> parameter is set to true:</strong> Deletes the entire Mirror setup (all mirrors), including the root mirror subdirectory, Filestore instance, and associated PV/PVC.
      </li>
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
      <li>If the mirror PVC `<i><code>${AOSP_MIRROR_PRESET_FILESTORE_PVC_NAME}</code></i>` does NOT exists, this job will fail.</li>
      <li>If the specified mirror directory does not exist, this job will fail.</li>
      <li>The periodic schedule for job `<i><code>Mirror > Sync Mirror</code></i>`, if set, must be manually edited or removed via that job configuration.</li>
    </ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  parameters {
    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<strong>REQUIRED:</strong> The image tag for the Docker image to be used as environment for this job.<br/>
      <b>Note</b>: Ensure you have executed that image build job prior to running this job, so that the required Docker image is available in your GCP project.''')
      trim(true)
    }

    booleanParam {
      name('CONFIRM_DELETE')
      defaultValue(false)
      description('<strong>REQUIRED:</strong> Check this box to confirm deletion. This action is irreversible.')
    }

    separator {
      name('SINGLE_MIRROR_DELETION_PARAMETERS')
      sectionHeader("Single Mirror Deletion Parameters")
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('MIRROR_DIR_TO_DELETE')
      defaultValue('')
      description('''Optional: The specific mirror directory to delete.<br/>
      Example: If you provided '<i><code>my-mirror</code></i>' when creating the mirror, provide the same value here to delete that specific mirror.<br/>
      <b>Note</b>: When <code>DELETE_ENTIRE_MIRROR_SETUP</code> is set to true, this parameter is ignored and the entire setup is deleted.<br/>''')
      trim(true)
    }

    separator {
      name('DELETE_ENTIRE_MIRROR_SETUP_SEPARATOR')
      sectionHeader("[CAUTION] DELETE ENTIRE MIRROR SETUP")
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('DELETE_ENTIRE_MIRROR_SETUP')
      defaultValue(false)
      description('''Optional: <strong>[CAUTION] If set to true, deletes the entire Mirror setup including all infrastructure and data.</strong><br/>
      If set to false, only the specified mirror directory will be deleted from the Filestore instance, rest will remain intact.<br/>
      <b>Note:</b> When set to true, the <code>MIRROR_DIR_TO_DELETE</code> parameter is ignored.''')
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
      scriptPath('workloads/android/pipelines/environment/mirror/delete_mirror/Jenkinsfile')
    }
  }
}

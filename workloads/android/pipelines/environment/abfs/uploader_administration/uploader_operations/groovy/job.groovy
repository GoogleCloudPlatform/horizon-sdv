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
pipelineJob('Android/Environment/ABFS/Uploader Administration/Uploader Operations') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">ABFS Uploader Operations</h3>
    <p>This job manages ABFS uploader VM lifecycle and seeding behavior for ABFS source/cache population.<br/>
    Use this job to create/update uploader instances, adjust branch seeding configuration, or perform runtime lifecycle operations.</p>
    <h4 style="margin-bottom: 10px;">Prerequisites</h4>
    <ul>
      <li><b>Service account</b>: <code>abfs-server</code> service account exists in the target GCP project.</li>
      <li><b>ABFS server ready</b>: server has been provisioned and is reachable before uploader seeding.</li>
      <li><b>ABFS license</b>: provide <code>ABFS_LICENSE_B64</code> for <code>APPLY</code> operations.</li>
      <li><b>Infra image</b>: Docker Infra Image Template job has run successfully and image is available.</li>
    </ul>
    <h4 style="margin-bottom: 10px;">Recommended operation order</h4>
    <ol>
      <li>Run server <code>APPLY</code> and verify with <i>Get Server Details</i>.</li>
      <li>Run uploader <code>APPLY</code> with required branch/manifest inputs.</li>
      <li>Run <i>Get Uploader Details</i> to verify process liveness and branch coverage. If any uploader reports ABFS not alive, run this job with <code>DESTROY</code> then <code>APPLY</code> again (same parameters and license), then run <i>Get Uploader Details</i> again. Repeat until all uploaders return liveness.</li>
      <li>Use <code>STOP</code>/<code>START</code>/<code>RESTART</code> for runtime operations; <code>DESTROY</code> for teardown.</li>
    </ol>
    <h4 style="margin-bottom: 10px;">Upgrading ABFS version</h4>
    <p>Upgrade the server first (Server Operations with new <code>ABFS_VERSION</code>). For uploaders, run this job with <code>DESTROY</code>, then with <code>APPLY</code> and the same new <code>ABFS_VERSION</code> and license. Apply alone does not update all uploader instances (only index 0); DESTROY then APPLY ensures every uploader is recreated with the new image. Re-seeding will run again. See <i>docs/workloads/android/abfs.md</i>, section Upgrading ABFS version.</p>
    <p>Tip: keep uploader branch changes explicit (for example JSON branch lists), because branch list changes drive seeding updates.</p>""")

  parameters {
    separator {
      name('1) Action and Safety')
      sectionHeader('1) Action and Safety')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    choiceParam {
      name('ABFS_TERRAFORM_ACTION')
      choices(['APPLY', 'DESTROY', 'START', 'STOP', 'RESTART'])
      description('''<p>Uploader operation to run.<br/>
        Use <code>APPLY</code> for create/update, <code>DESTROY</code> for teardown, and <code>STOP</code>/<code>START</code>/<code>RESTART</code> for runtime lifecycle.</p>''')
    }

    nonStoredPassword {
      name('ABFS_LICENSE_B64')
      description('''<p><b>Mandatory:</b> Base64-encoded ABFS license file (required for <code>APPLY</code> actions).</p>''')
    }

    stringParam {
      name('UPLOADER_POST_APPLY_DELAY_SECONDS')
      defaultValue('300')
      description('''<p>After <code>APPLY</code>, wait this many seconds before the job completes. Gives instances time to boot and start ABFS before you run <i>Get Uploader Details</i>. Set to <code>0</code> to skip. Only used when <code>ABFS_TERRAFORM_ACTION=APPLY</code>.</p>''')
      trim(true)
    }

    separator {
      name('2) Core Image and Module Version')
      sectionHeader('2) Core Image and Module Version')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('INFRA_IMAGE_TAG')
      defaultValue('latest')
      description('''<p>Image tag for the ABFS infra docker image used for server creation.</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_VERSION')
      defaultValue("${ABFS_VERSION}")
      description('''<p>ABFS version, e.g. 0.0.33-2-ge59ffbc, latest.</p>''')
      trim(true)
    }

    stringParam {
      name('DOCKER_REGISTRY_NAME')
      defaultValue('europe-docker.pkg.dev/abfs-binaries/abfs-containers-alpha/abfs-alpha:${ABFS_VERSION}')
      description('''<p>ABFS docker registry.</p>''')
      trim(true)
    }

    stringParam {
      name('GOOGLE_ABFS_TERRAFORM_GIT_URL')
      defaultValue('https://github.com/terraform-google-modules/terraform-google-abfs.git')
      description('''<p>ABFS Terraform Git repo.</p>''')
      trim(true)
    }

    stringParam {
      name('GOOGLE_ABFS_TERRAFORM_VERSION')
      defaultValue('v0.10.0')
      description('''<p>ABFS Terraform Git repo tag or sha1 version.</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_COS_IMAGE_REF')
      defaultValue("${ABFS_COS_IMAGE_REF}")
      description('''<p>ABFS Containerized OS images used on server and uploader instances.</p>''')
      trim(true)
    }

    separator {
      name('3) Capacity and Sizing')
      sectionHeader('3) Capacity and Sizing')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('UPLOADER_COUNT')
      defaultValue('3')
      description('''<p>Number of ABFS uploader instances to seeding the android version on the Server.</p>''')
      trim(true)
    }

    stringParam {
      name('UPLOADER_MACHINE_TYPE')
      defaultValue('n2d-standard-48')
      description('''<p>Machine type for ABFS uploaders.</p>''')
      trim(true)
    }

    stringParam {
      name('UPLOADER_DATADISK_SIZE_GB')
      defaultValue('1024')
      description('''<p>Disk size for uploader instances.</p>''')
      trim(true)
    }

    separator {
      name('4) Source and Seed Behavior')
      sectionHeader('4) Source and Seed Behavior')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('UPLOADER_MANIFEST_SERVER')
      defaultValue('android.googlesource.com')
      description('''<p>Gerrit manifest server to seed from.</p>''')
      trim(true)
    }

    stringParam {
      name('UPLOADER_MANIFEST_FILE')
      defaultValue('default.xml')
      description('''<p>Gerrit manifest file to seed from.</p>''')
      trim(true)
    }

    stringParam {
      name('UPLOADER_GIT_BRANCH')
      defaultValue('["android-15.0.0_r36"]')
      description('''<p>JSON array of Gerrit branches/tags to seed.<br/>
        Single branch example: <code>["android-15.0.0_r36"]</code><br/>
        Multiple branches example: <code>["android-15.0.0_r36","android-16.0.0_r3"]</code><br/>
        Keep values explicit: removing a branch/tag from this list removes it from desired seeding state.</p>''')
      trim(true)
    }

    stringParam {
      name('UPLOADER_MANIFEST_SCHEME')
      defaultValue('https')
      description('''<p>Gerrit manifest scheme used by uploader instances.</p>''')
      trim(true)
    }

    separator {
      name('5) Advanced and Optional')
      sectionHeader('5) Advanced and Optional')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('ABFS_EXTRA_PARAMS')
      defaultValue('[]')
      description('''<p>JSON array of extra ABFS command parameters for uploader instances.</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_GERRIT_UPLOADER_EXTRA_PARAMS')
      defaultValue('[]')
      description('''<p>JSON array of extra gerrit upload-daemon parameters.</p>''')
      trim(true)
    }

    choiceParam {
      name('ABFS_ENABLE_GIT_LFS')
      choices(['false', 'true'])
      description('''<p>Enable Git LFS support on uploader instances.</p>''')
    }

    stringParam {
      name('PRE_START_HOOKS')
      defaultValue('')
      description('''<p>Optional absolute path to pre-start hook scripts for uploaders.</p>''')
      trim(true)
    }

  }

  // Block build if certain jobs are running.
  blockOn('Android*.*ABFS*.*Uploader.*') {
    // Possible values are 'GLOBAL' and 'NODE' (default).
    blockLevel('GLOBAL')
    // Possible values are 'ALL', 'BUILDABLE' and 'DISABLED' (default).
    scanQueueFor('BUILDABLE')
  }

  logRotator {
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
      scriptPath('workloads/android/pipelines/environment/abfs/uploader_administration/uploader_operations/Jenkinsfile')
    }
  }
}

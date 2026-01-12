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
    <br/><h3 style="margin-bottom: 10px;">ABFS Uploaders</h3>
    <p>This job creates a virtual machine (VM) instance for the ABFS Uploaders, which is required for the ABFS build job to mount the ABFS source(cache).<br/>
    The ABFS Uploader VM instances will be start to seed the ABFS server with the requested Android revision.<br/>
    Use <i>Get Uploader Details</i> to check the state of the uploader instances.</p>
    <h4 style="margin-bottom: 10px;">Prerequisites</h4>
    <p>Before creating the ABFS Server VM instance, the following dependencies must be met:</p>
    <ul><li><b>Service Account Creation</b>: The abfs-server service account must be created in the GCP project.</li>
        <li><b>ABFS License Deployment</b>: The ABFS license provided by Google must be deployed on the platform using GitHub environment secrets and Terraform workflow.</li>
        <li><b>Docker Infra Image Template Job</b>:The Docker Infra Image Template job must be run, and the Docker image must be available in the registry.</li>
        <li><b>ABFS Server</b>: The ABFS server must have been created and started for uploader to seed the server.</li>
    </ul>
    <p>By meeting these prerequisites, you can ensure a successful creation of the ABFS Uploader VM instances, which is essential for the ABFS build job to use the source/cache from the server.</p>""")

  parameters {

    choiceParam {
      name('ABFS_TERRAFORM_ACTION')
      choices(['APPLY', 'DESTROY', 'START', 'STOP', 'RESTART'])
      description('''<p>The action to perform to create, destroy, stop, start, restart server.<br/>
        Use `APPLY` to create the server or update based on any changes made below.</p>''')
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
      description('''<p>Gerrit branches/tags to seed from. Remove branch/tag and seed will be removed.</p>''')
      trim(true)
    }

    stringParam {
      name('TERRAFORM_GITHUB_URL')
      defaultValue('https://github.com/terraform-google-modules/terraform-google-abfs.git')
      description('''<p>ABFS Terraform GitHub repo.</p>''')
      trim(true)
    }

    stringParam {
      name('TERRAFORM_GITHUB_VERSION')
      defaultValue('961f5aa3c3be87a242597cbd4bc08821f28a7085')
      description('''<p>ABFS Terraform GitHub repo sha1 version.</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_LICENSE_B64')
      defaultValue('')
      description('''<p>Optional: Base64 encoded version of the ABFS license file.</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_COS_IMAGE_REF')
      defaultValue("${ABFS_COS_IMAGE_REF}")
      description('''<p>ABFS Containerized OS images used on server and uploader instances.</p>''')
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
      scriptPath('workloads/android/pipelines/environment/abfs/uploader_administration/uploader_operations/Jenkinsfile')
    }
  }
}

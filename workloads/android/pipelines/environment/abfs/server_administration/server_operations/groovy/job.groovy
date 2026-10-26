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
pipelineJob('Android/Environment/ABFS/Server Administration/Server Operations') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">ABFS Server Operations</h3>
      <p>This job manages the ABFS Server VM lifecycle and Terraform-backed infrastructure for ABFS server components.<br/>
      Use this job to create/update, start/stop, restart, or destroy the ABFS server resources.</p>
    <h4 style="margin-bottom: 10px;">Prerequisites</h4>
      <ul>
        <li><b>Service account</b>: <code>abfs-server</code> service account exists in the target GCP project.</li>
        <li><b>ABFS license</b>: provide <code>ABFS_LICENSE_B64</code> for <code>APPLY</code> operations.</li>
        <li><b>Infra image</b>: Docker Infra Image Template job has run successfully and image is available.</li>
      </ul>
    <h4 style="margin-bottom: 10px;">Recommended operation order</h4>
      <ol>
        <li>Run <code>APPLY</code> to create/update server infrastructure.</li>
        <li>Run <i>Get Server Details</i> to verify instance and process readiness.</li>
        <li>Use <code>STOP</code>/<code>START</code>/<code>RESTART</code> for runtime operations as needed.</li>
        <li>Use <code>DESTROY</code> only for teardown scenarios.</li>
      </ol>
    <h4 style="margin-bottom: 10px;">Spanner guidance</h4>
      <p>Set <code>ABFS_SPANNER_DATABASE_CREATE_TABLES=true</code> only when provisioning a new ABFS Spanner DB.<br/>
      For upgrades or existing legacy DBs, keep it <code>false</code>.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
    """)

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
      description('''<p>Server operation to run.<br/>
        Use <code>APPLY</code> for create/update, <code>DESTROY</code> for teardown, and <code>STOP</code>/<code>START</code>/<code>RESTART</code> for runtime lifecycle.</p>''')
    }

    nonStoredPassword {
      name('ABFS_LICENSE_B64')
      description('''<p><b>Mandatory:</b> Base64-encoded ABFS license file (required for <code>APPLY</code> actions).</p>''')
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
      name('SERVER_MACHINE_TYPE')
      defaultValue('n2-highmem-64')
      description('''<p>Machine type for ABFS server.</p>''')
      trim(true)
    }

    separator {
      name('4) Source and Seed Behavior')
      sectionHeader('4) Source and Seed Behavior')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
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

    separator {
      name('5) Spanner Controls')
      sectionHeader('5) Spanner Controls')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('ABFS_SPANNER_INSTANCE_MIN_NODES')
      defaultValue('1')
      description('''<p>Minimum number of Spanner nodes for the ABFS instance.</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_SPANNER_INSTANCE_MAX_NODES')
      defaultValue('10')
      description('''<p>Maximum number of Spanner nodes for the ABFS instance.</p>''')
      trim(true)
    }

    choiceParam {
      name('ABFS_SPANNER_DATABASE_CREATE_TABLES')
      choices(['true', 'false'])
      description('''<p>Mandatory. Set <code>false</code> for upgrades/legacy DBs. Set <code>true</code> only for creating a new ABFS Spanner DB; the pipeline will fail if an ABFS DB already exists.</p>''')
    }

    stringParam {
      name('ABFS_SPANNER_DATABASE_SCHEMA_VERSION')
      defaultValue('0.0.31')
      description('''<p>Schema version used only when <code>ABFS_SPANNER_DATABASE_CREATE_TABLES=true</code>.</p>''')
      trim(true)
    }

    separator {
      name('6) Advanced and Optional')
      sectionHeader('6) Advanced and Optional')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('ABFS_EXTRA_PARAMS')
      defaultValue('[]')
      description('''<p>JSON array of extra ABFS server command parameters.</p>''')
      trim(true)
    }

    stringParam {
      name('EXISTING_BUCKET_NAME')
      defaultValue('')
      description('''<p>Optional existing GCS bucket name for ABFS server data.</p>''')
      trim(true)
    }

  }

  // Block build if certain jobs are running.
  blockOn('Android*.*ABFS*.*Server.*') {
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
      scriptPath('workloads/android/pipelines/environment/abfs/server_administration/server_operations/Jenkinsfile')
    }
  }
}

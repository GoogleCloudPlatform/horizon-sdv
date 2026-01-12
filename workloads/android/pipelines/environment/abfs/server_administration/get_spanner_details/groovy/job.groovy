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
pipelineJob('Android/Environment/ABFS/Server Administration/Get Spanner Details') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">ABFS Server Details</h3>
    <p>This job returns details on the spanner instances, bucket storage, backups and backup schedules.<br/>
    <br/>Refer to the console log, and artifacts, for details of the instance and state.</p>""")

  parameters {
    stringParam {
      name('ABFS_DB_NAME')
      defaultValue('abfs')
      description('''<p><b>Optional:</b> Enter specific DB name if known, e.g <code>abfs</code>, else leave blank to show potential instances.</p>''')
      trim(true)
    }
    stringParam {
      name('ABFS_DB_BUCKET_NAME')
      defaultValue('')
      description('''<p><b>Optional:</b> Enter specific DB bucket name if known, e.g <code>abfs-1234</code>, else leave blank to show potential buckets associated.</p>''')
      trim(true)
    }
    stringParam {
      name('INFRA_IMAGE_TAG')
      defaultValue('latest')
      description('''<p>Image tag for the ABFS infra docker image.</p>''')
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
      scriptPath('workloads/android/pipelines/environment/abfs/server_administration/get_spanner_details/Jenkinsfile')
    }
  }
}

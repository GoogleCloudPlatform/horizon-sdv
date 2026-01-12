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
pipelineJob('Android/Environment/ABFS/Server Administration/Destroy Spanner Instance') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Destroy Spanner Instance</h3>
    <p>This job allows destroying the spanner instance:.<br/>
    Ensure the ABFS server has already been destroyed,<br/>
    Backups must be destroyed first and then the instance will be destroyed.<br/>
    Bucket and objects remain in place, so best to run BUCKET deletion once instance is deleted, .</p>""")

  parameters {
    choiceParam {
      name('DESTROY_ACTION')
      choices(['INSTANCE', 'BUCKET'])
      description('''<p>The action to perform..<br/>
        <b>INSTANCE</b>: Delete backups and instance. Backups must be removed before an instance can be deleted.<br/>
        <b>BUCKET</b>: Delete bucket (objects are left after instance is deleted).</p>''')
    }
    stringParam {
      name('ABFS_DB_NAME')
      defaultValue('')
      description('''<p><b>Mandatory:</b> Must be defined for instance deletion. Enter specific DB name, e.g <code>abfs</code>. </p>''')
      trim(true)
    }
    stringParam {
      name('ABFS_DB_BUCKET_NAME')
      defaultValue('')
      description('''<p><b>Mandatory:</b> Must be defined for bucket deletion. Enter specific DB bucket name if known, e.g <code>abfs-1234</code>, else leave blank to show potential buckets associated.</p>''')
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
      scriptPath('workloads/android/pipelines/environment/abfs/server_administration/destroy_spanner_instance/Jenkinsfile')
    }
  }
}

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
pipelineJob('Android/Environment/ABFS/Server Administration/Update Spanner Backups') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">ABFS Spanner DB Backup Schedule</h3>
    <p>This job allows retrieval or alteration of the Spanner DB backup schedule.<br/>
    <br/>Refer to the console log for details of the job.</p>""")

  parameters {
    choiceParam {
      name('BACKUP_SCHEDULE_ACTION')
      choices(['DETAILS', 'CREATE', 'DELETE', 'UPDATE'])
      description('''<p>The action to perform to create, delete, update or fetch schedule details.<br/></p>''')
    }
    stringParam {
      name('BACKUP_SCHEDULE_ID')
      defaultValue('')
      description('''<p>Leave empty for <code>BACKUP_SCHEDULE_ACTION=DETAILS</code> so as to retrieve all backup schedules.<br/>
                     For <code>BACKUP_SCHEDULE_ACTION=CREATE|DELETE|UPDATE</code>, define a backup schedule id, e.g.<br/>
                     <code>default_daily_full_backup_schedule</code> or new schedule name.</p>''')
      trim(true)
    }
    stringParam {
      name('CRON')
      defaultValue('0 23 * * *')
      description('''<p>Enter backup schedule in crontab format. Default is daily at 11pm</p>''')
      trim(true)
    }
    stringParam {
      name('RETENTION_DURATION')
      defaultValue('604800')
      description('''<p>Retention time in seconds, default is 7days.</p>''')
      trim(true)
    }
    stringParam {
      name('DATABASE')
      defaultValue('abfs')
      description('''<p>ABFS Database name.</p>''')
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
      scriptPath('workloads/android/pipelines/environment/abfs/server_administration/update_spanner_backups/Jenkinsfile')
    }
  }
}

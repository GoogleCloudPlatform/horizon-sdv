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
pipelineJob('Android/Environment/ABFS/Uploader Administration/Get Uploader Details') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">ABFS Uploader Details</h3>
    <p>This job retrieves the current status and configuration of Uploader instances.<br/>
    It also reports ABFS liveness, confirming whether ABFS has been provisioned successfully.<br/>
    If this check fails, consider destroying the instance (DESTROY) and recreating it (APPLY) under <i>Uploader Operations</i>.<br/>
    Refer to the console log and artifacts for detailed instance information and state.</p>""")

  parameters {
    stringParam {
      name('UPLOADER_INSTANCE_PREFIX')
      defaultValue('abfs-gerrit-uploader')
      description('''<p>Prefix of the ABFS Uploader instance name.</p>''')
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
      scriptPath('workloads/android/pipelines/environment/abfs/uploader_administration/get_uploader_details/Jenkinsfile')
    }
  }
}

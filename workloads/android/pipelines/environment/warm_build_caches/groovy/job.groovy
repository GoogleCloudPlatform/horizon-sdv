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
pipelineJob('Android/Environment/Warm Build Caches') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Warm Build Caches Job</h3>
    <p>This job is designed to accelerate build times by pre-populating the build caches, specifically the persistent volumes, in advance. It achieves this by executing a series of standard builds based on the selected manifest and revision.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <ul><li>Ensure all Build PVs are deleted before running this job.</li>
    <li>Gerrit manifest and revision can be changed, together with targets, but refer to <code>Jenkinsfile</code> for details as to how these are managed.</li>
    <li>Plan to run the jobs in parallel to create sufficient PVs (note: k8s cap of 20)</li></ul>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('AAOS_GERRIT_MANIFEST_URL')
      defaultValue("https://${HORIZON_DOMAIN}/gerrit/android/platform/manifest")
      description('''<p>Android Manifest URL.</p>''')
      trim(true)
    }

    stringParam {
      name('AAOS_REVISION')
      defaultValue('horizon/android-16.0.0_r3')
      description('''<p>Android revision tag/branch name.</p>''')
      trim(true)
    }

    choiceParam {
      name('ANDROID_VERSION')
      description('''<p>Version of disk pool to use for the build cache, select from one of the following options:</p>
          <ul>
            <li>default: let job determine pool.</li>
            <li>16: Use the Android 16 disk pool.</li>
            <li>15: Use the Android 15 disk pool.</li>
            <li>14: Use the Android 14 disk pool.</li>
          </ul>''')
      choices(['default', '16', '15', '14'])
    }

    stringParam {
      name('GERRIT_REPO_SYNC_JOBS')
      defaultValue("${REPO_SYNC_JOBS}")
      description('''<p>Number of parallel sync jobs for <i>repo sync</i>.<br/>
        Default value is defined by the Android Seed job</p>''')
      trim(true)
    }

    booleanParam {
      name('ARCHIVE_ARTIFACTS')
      defaultValue(false)
      description('''<p>Enable if wishing to store build artifacts to bucket</p>''')
    }

    separator {
      name('AOSP Mirror Parameters')
      sectionHeader('AOSP Mirror Parameters')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('USE_LOCAL_AOSP_MIRROR')
      defaultValue(${USE_LOCAL_AOSP_MIRROR})
      description('''<p>If checked, the build will use the AOSP Mirror setup in your GCP project to fetch Android source code during <i>repo sync</i>.<br/>
        <b>Note:</b> The AOSP Mirror must be setup prior to running this job. If not setup, the job will fail.<br> The setup jobs are in folder <i>Android Workflows > Environment > Mirror</i>.<br/><br/></p>''')
    }

    stringParam {
      name('AOSP_MIRROR_DIR_NAME')
      defaultValue("${AOSP_MIRROR_DIR_NAME}")
      description('''<p>The directory name on the Filestore volume where the Mirror is located.<br/>
        <b>Note:</b> This is required if <code>USE_LOCAL_AOSP_MIRROR</code> is checked.</p>
        <b>Example:</b> If you provided '<i><code>my-mirror</code></i>' when creating the mirror, provide the same value here.<br/><br/></p>''')
      trim(true)
    }
  }

  logRotator {
    artifactDaysToKeep(60)
    artifactNumToKeep(100)
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
      scriptPath('workloads/android/pipelines/environment/warm_build_caches/Jenkinsfile')
    }
  }
}

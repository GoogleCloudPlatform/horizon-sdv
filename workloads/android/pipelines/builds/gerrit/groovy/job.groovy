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
pipelineJob('Android/Builds/Gerrit') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Android Gerrit Pipeline Builder</h3>
    <p>This job is triggered by a Gerrit patchset change. Its purpose is to verify the integrity of the patchset change
by performing builds on that patchset and providing the user with a vote to their patchset in <a href="https://${HORIZON_DOMAIN}/gerrit/" target="_blank">Gerrit.</p>
    <h4 style="margin-bottom: 10px;">Supported Builds</h4>
    <p>Currently, this job supports the standard set of Android Automotive virtual devices and a platform target.</p>
    <h4 style="margin-bottom: 10px;">Build Outputs</h4>
    <p>Build outputs are stored in a Google Cloud Storage bucket (refer to build artifacts for location).</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <ul><li>This build job serves only to demonstrate the pipeline with Gerrit</li>
    <li>It includes a single CTS test job also for demonstration purposes, but this has no impact on the vote.</li></ul>
    <h4 style="margin-bottom: 10px;">Viewing Artifacts on Google Cloud</h4>
    <p><a href="https://cloud.google.com/docs/authentication/gcloud" target="_blank">Sign in to Google Cloud</a> and run the following command: <br/><code>gcloud storage ls gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/Gerrit/&lt;BUILD_NUMBER&gt;</code></p>
    <br/><br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  properties{
    pipelineTriggers{
      triggers{
        gerrit{
          gerritProjects{
            gerritProject{
              compareType('REG_EXP')
              pattern('^android\\/(?!.*\\/manifest$).*')
              branches{
                branch{
                  compareType('ANT')
                  pattern('**/horizon/*')
                }
              }
              disableStrictForbiddenFileVerification(true)
            }
          }
          triggerOnEvents{
            patchsetCreated()
          }
        }
      }
    }
  }

  environmentVariables {
    env('GERRIT_REPO_SYNC_JOBS', "${REPO_SYNC_JOBS}")
    env('JENKINS_GCE_CLOUD_LABEL', "${JENKINS_GCE_CLOUD_LABEL}")
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
      scriptPath('workloads/android/pipelines/builds/gerrit/Jenkinsfile')
    }
  }
}

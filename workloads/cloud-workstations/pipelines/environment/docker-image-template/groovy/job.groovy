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

pipelineJob('Cloud-Workstations/Environment/Docker Image Template') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Container Image Builder</h3>
    <p>This job builds the container image that serves as a dependency (execution environment) for all Cloud Workstations jobs.</p>
    <h4 style="margin-bottom: 10px;">Image Configuration</h4>
    <p>The Dockerfile specifies the installed packages and tools required by these jobs.</p>
    <h4 style="margin-bottom: 10px;">Pushing Changes to the Registry</h4>
    <p>To push changes to the registry, set the parameter <code>NO_PUSH=false</code>.</p>
    <p>The image will be pushed to <code>${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${CLOUD_WS_WORKLOADS_ENV_IMAGE_NAME}</code></p>
    <h4 style="margin-bottom: 10px;">Verifying Changes</h4>
    <p>When working with new Dockerfile updates, it's recommended to set <code>NO_PUSH=true</code> to verify the changes before pushing the image to the registry.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>This job need only be run once, or when there are updates to be applied based on Dockerfile changes..</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  """)

  parameters {
    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<p>Image tag for the builder image.</p>''')
      trim(true)
    }
    booleanParam {
      name('NO_PUSH')
      defaultValue(true)
      description('''<p>Build only, do not push to registry.</p>''')
    }
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
      scriptPath('workloads/cloud-workstations/pipelines/environment/docker-image-template/Jenkinsfile')
    }
  }
}

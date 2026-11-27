// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

pipelineJob('Sample/Environment/Docker Image Template') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Sample · Container image builder</h3>
    <p>Builds the <b>Horizon API workflow</b> Jenkins agent image (Alpine: curl, jq, python3).</p>
    <p>Push path: ${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${SAMPLE_BUILD_DOCKER_ARTIFACT_PATH_NAME}</p>
    <p>Set <code>NO_PUSH=false</code> to publish. Use <code>NO_PUSH=true</code> when validating Dockerfile changes.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    booleanParam {
      name('NO_PUSH')
      defaultValue(true)
      description('<p>Build only, do not push to registry.</p>')
    }
    stringParam {
      name('IMAGE_TAG')
      defaultValue("${SAMPLE_IMAGE_TAG}")
      description('<p>Image tag (e.g. latest or a pin).</p>')
      trim(true)
    }
    stringParam {
      name('LINUX_DISTRIBUTION')
      defaultValue('alpine:3.24')
      description('<p>Base image for Dockerfile ARG LINUX_DISTRIBUTION.</p>')
      trim(true)
    }

    separator {
      name('Common Parameters: Docker templates')
      sectionHeader('Common Parameters: Docker templates')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('BUILDKIT_RELEASE_TAG')
      defaultValue("${BUILDKIT_RELEASE_TAG}")
      description('<p>BuildKit image tag (<a target="_blank" href="https://hub.docker.com/r/moby/buildkit">releases</a>).</p>')
      trim(true)
    }
    stringParam {
      name('DOCKER_CREDENTIALS_URL')
      defaultValue("${DOCKER_CREDENTIALS_URL}")
      description('<p>docker-credential-gcr tarball URL.</p>')
      trim(true)
    }
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
      scriptPath('workloads/sample/pipelines/environment/docker_image_template/Jenkinsfile')
    }
  }
}

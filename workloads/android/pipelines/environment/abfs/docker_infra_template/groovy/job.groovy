// Copyright (c) 2025-2026 Accenture, All Rights Reserved.
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
pipelineJob('Android/Environment/ABFS/Docker Infra Image Template') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Infrastructure Container Builder</h3>
    <p>This job builds the container image that serves as a dependency for creating ABFS Server and Uplodaer VM instances.</p>
    <h4 style="margin-bottom: 10px;">Image Configuration</h4>
    <p>The Dockerfile specifies the installed packages and tools required by these jobs.</p>
    <h4 style="margin-bottom: 10px;">Pushing Changes to the Registry</h4>
    <p>To push changes to the registry, set the parameter <code>NO_PUSH=false</code>.</p>
    <p>The image will be pushed to ${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/horizon-sdv/${INFRA_DOCKER_ARTIFACT_PATH_NAME}</p>
    <h4 style="margin-bottom: 10px;">Verifying Changes</h4>
    <p>When working with new Dockerfile updates, it's recommended to set <code>NO_PUSH=true</code> to verify the changes before pushing the image to the registry.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>This job need only be run once, or when there are updates to be applied based on Dockerfile changes..</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<p><b>Mandatory:</b> Image tag for the builder image.</p>''')
      trim(true)
    }
    stringParam {
      name('LINUX_DISTRIBUTION')
      defaultValue('debian:12')
      description('''<p>Define the Linux distribution to use, e.g.</p></br>
        <ul><li>debian:12</li>
            <li>ubuntu:22.04</li>
            <li>ubuntu:jammy-20251203</li></ul>''')
      trim(true)
    }
    stringParam {
      name('TERRAFORM_VERSION')
      defaultValue('main')
      description('''<p>Define the Hashicorp Terraform version.</p>''')
      trim(true)
    }
    booleanParam {
      name('NO_PUSH')
      defaultValue(true)
      description('''<p>Build only, do not push to registry.</p>''')
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
      description('''<p>BuildKit tag, see <a target="_blank"  href=https://hub.docker.com/r/moby/buildkit>buildkit releases</a>.</p>''')
      trim(true)
    }
    stringParam {
      name('DOCKER_CREDENTIALS_URL')
      defaultValue("${DOCKER_CREDENTIALS_URL}")
      description('''<p>Docker credentials helper URL, e.g. <a target="_blank" href=https://cloud.google.com/artifact-registry/docs/docker/authentication#standalone-helper>credentials helper</a>.</p>''')
      trim(true)
    }
    stringParam {
      name('GCLOUD_CLI_VERSION')
      defaultValue("${GCLOUD_CLI_VERSION}")
      description('''<p>Version of <a target="_blank" https://docs.cloud.google.com/sdk/docs/release-notes>Google Cloud CLI</a>.<br/>Note: Define <code>latest</code> if wishing to use the latest available version.</p>''')
      trim(true)
    }
    stringParam {
      name('KUBECTL_VERSION')
      defaultValue("${KUBECTL_VERSION}")
      description('''<p>Version of <code>kubectl</code>. Typically based on <a target="_blank" https://docs.cloud.google.com/sdk/docs/release-notes>Google Cloud CLI</a><br/>Note: Define <code>latest</code> if wishing to use the latest available version.</p>''')
      trim(true)
    }
    stringParam {
      name('NODEJS_VERSION')
      defaultValue("${NODEJS_VERSION}")
      description('''<p>NodeJS version.<br/>
        This is installed using <i>nvm</i> on the instance template to be compatible with other tooling.</p>''')
      trim(true)
    }
    separator {
      name('Agentic AI: Configuration (Experimental)')
      sectionHeader('Agentic AI: Configuration (Experimental)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    booleanParam {
      name('ENABLE_GEMINI_AI_ASSISTANT')
      defaultValue(${ENABLE_GEMINI_AI_ASSISTANT})
      description('''<p>Enable Gemini AI to support in diagnosis of build and test failures.</p>''')
    }
    stringParam {
      name('GEMINI_CLI_VERSION')
      defaultValue("${GEMINI_CLI_VERSION}")
      description('''<p>The version of gemini-cli to be installed.<br/>
        <b>Default</b>: Latest available version.<br/>
        <b>Reference</b>: Run <code>npm view @google/gemini-cli versions</code> for a full list of valid versions.</p>''')
      trim(true)
    }
  }

  // Block build if certain jobs are running.
  blockOn('Android*.*Docker.*') {
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
      scriptPath('workloads/android/pipelines/environment/abfs/docker_infra_template/Jenkinsfile')
    }
  }
}

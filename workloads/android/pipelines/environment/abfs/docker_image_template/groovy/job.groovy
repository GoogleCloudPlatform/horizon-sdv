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
pipelineJob('Android/Environment/ABFS/Docker Image Template') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Container Image Builder</h3>
    <p>This job builds the container image that serves as a dependency for ABFS build jobs.</p>
    <h4 style="margin-bottom: 10px;">Ubuntu vs COS templates</h4>
    <p>Enable one or both template flavors:</p>
    <ul>
      <li><b>Ubuntu</b> — for ABFS client builds on Ubuntu client pools where CASFS is loaded on the node. Default tag <code>latest</code>.</li>
      <li><b>COS</b> — for ABFS client builds on COS client pools where CASFS is embedded in the node image (GKE ≥ 1.36). Default tag <code>cos-latest</code>. Seed <code>ABFS_COS_IMAGE_REF</code> (<code>${ABFS_COS_IMAGE_REF}</code>) is the GCE COS node family — printed on the COS build.</li>
    </ul>
    <h4 style="margin-bottom: 10px;">Image Configuration</h4>
    <p>The Dockerfile specifies the installed packages and tools required by these jobs.</p>
    <h4 style="margin-bottom: 10px;">Pushing Changes to the Registry</h4>
    <p>To push changes to the registry, set the parameter <code>NO_PUSH=false</code>.</p>
    <p>The image will be pushed to ${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${ABFS_BUILD_DOCKER_ARTIFACT_PATH_NAME}</p>
    <h4 style="margin-bottom: 10px;">Verifying Changes</h4>
    <p>When working with new Dockerfile updates, it's recommended to set <code>NO_PUSH=true</code> to verify the changes before pushing the image to the registry.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>This job need only be run once, or when there are updates to be applied based on Dockerfile changes..</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  environmentVariables {
    env('ABFS_COS_IMAGE_REF', '${ABFS_COS_IMAGE_REF}')
  }

  parameters {
    booleanParam {
      name('NO_PUSH')
      defaultValue(true)
      description('''<p>Build only, do not push to registry.</p>''')
    }

    separator {
      name('Template flavor (Ubuntu / COS)')
      sectionHeader('Template flavor (Ubuntu / COS)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('ENABLE_UBUNTU_TEMPLATE')
      defaultValue(true)
      description('''<p>Build the <b>Ubuntu</b> ABFS builder image for Ubuntu client pools where CASFS is loaded on the node.</p>''')
    }
    stringParam {
      name('IMAGE_TAG_UBUNTU')
      defaultValue('latest')
      description('''<p><b>Mandatory:</b> Registry image tag for the builder image for Ubuntu build node pool.</p>''')
      trim(true)
    }
    stringParam {
      name('LINUX_DISTRIBUTION')
      defaultValue('ubuntu:22.04')
      description('''<p>Define the Linux distribution to use, e.g.</p></br>
        <ul><li>ubuntu:22.04</li>
            <li>ubuntu:jammy-20251203</li></ul>''')
      trim(true)
    }

    booleanParam {
      name('ENABLE_COS_TEMPLATE')
      defaultValue(true)
      description('''<p>Build the <b>COS</b> ABFS builder image for COS client pools where CASFS is embedded in the node image (GKE ≥ 1.36).</p>''')
    }
    stringParam {
      name('IMAGE_TAG_COS')
      defaultValue('cos-latest')
      description('''<p><b>Mandatory:</b> Registry image tag for the builder image for COS build node pool. Default <code>cos-latest</code> (does not overwrite Ubuntu <code>latest</code>).</p>''')
      trim(true)
    }
    stringParam {
      name('COS_LINUX_DISTRIBUTION')
      defaultValue('ubuntu:22.04')
      description('''<p>Define the Linux distribution to use for the COS-tagged image, e.g.</p></br>
        <ul><li>ubuntu:22.04</li>
            <li>ubuntu:jammy-20251203</li></ul>''')
      trim(true)
    }

    separator {
      name('ABFS apt / version alignment')
      sectionHeader('ABFS apt / version alignment')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('ABFS_REPOSITORY')
      defaultValue("${ABFS_REPOSITORY}")
      description('''<p>ABFS Artifact Registry apt repository name from Seed Workloads (e.g. <code>abfs-apt-alpha-public</code>).
        Used to compose <code>ABFS_DISTRIBUTION_REGISTRY</code> when that override is left empty.
        Update Seed <code>ABFS_REPOSITORY</code> when Google publishes a new ABFS apt channel.</p>''')
      trim(true)
    }
    stringParam {
      name('ABFS_VERSION')
      defaultValue("${ABFS_VERSION}")
      description('''<p>ABFS client version from Seed (informational for this image build).
        The image enables ABFS apt transport + <code>kmod</code>; <code>abfs-client</code> / CASFS packages are installed at AAOS ABFS Builder runtime using this Seed value.</p>''')
      trim(true)
    }
    stringParam {
      name('GOOGLE_DISTRIBUTION_REGISTRY')
      defaultValue('http://packages.cloud.google.com/apt apt-transport-artifact-registry-stable main')
      description('''<p>Google distribution registry URL and component (apt-transport-artifact-registry).</p>''')
      trim(true)
    }
    stringParam {
      name('ABFS_DISTRIBUTION_REGISTRY')
      defaultValue('')
      description('''<p>Optional full ABFS apt sources line. Leave empty to use
        <code>ar+https://us-apt.pkg.dev/projects/abfs-binaries ${ABFS_REPOSITORY} main</code>.
        Set only to override the composed default.</p>''')
      trim(true)
    }
    stringParam {
      name('NODEJS_VERSION')
      defaultValue("${NODEJS_VERSION}")
      description('''<p>NodeJS version.<br/>
        This is installed using <i>nvm</i> on the instance template to be compatible with other tooling.</p>''')
      trim(true)
    }
    stringParam {
      name('PACKER_VERSION')
      defaultValue('1.15.0')
      description('''<p>Version of <a target="_blank" href="https://github.com/hashicorp/packer/releases">Packer</a> to install in the builder image (same Dockerfile as Android Docker Image Template).<br/>Use a pinned version (recommended) or <code>latest</code>.</p>''')
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
    stringParam {
      name('ANTIGRAVITY_SDK_VERSION')
      defaultValue("${ANTIGRAVITY_SDK_VERSION}")
      description('''<p>The version of google-antigravity SDK to be installed.<br/>
        <b>Default</b>: 0.1.16.<br/>
        <b>Reference</b>: Run <code>pip index versions google-antigravity</code> for a full list of valid versions.</p>''')
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
      scriptPath('workloads/android/pipelines/environment/abfs/docker_image_template/Jenkinsfile')
    }
  }
}

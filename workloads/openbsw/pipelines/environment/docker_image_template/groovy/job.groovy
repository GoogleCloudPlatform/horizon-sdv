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

// Description:
// Groovy file for defining a Jenkins Pipeline Job for creating a
// the Docker image template that is used by other pipeline jobs
// in the OpenBSW project.
pipelineJob('OpenBSW/Environment/Docker Image Template') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Container Image Builder</h3>
    <p>This job builds the container image that serves as a dependency for other pipeline jobs.</p>
    <h4 style="margin-bottom: 10px;">Image Configuration</h4>
    <p>The Dockerfile specifies the installed packages and tools required by these jobs.<br/>
    Parameters are provided to support customization of OpenBSW build environment/tools.</p>
    <h4 style="margin-bottom: 10px;">Pushing Changes to the Registry</h4>
    <p>To push changes to the registry, set the parameter <code>NO_PUSH=false</code>.</p>
    <p>The image will be pushed to ${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${OPENBSW_BUILD_DOCKER_ARTIFACT_PATH_NAME}</p>
    <h4 style="margin-bottom: 10px;">Verifying Changes</h4>
    <p>When working with new Dockerfile updates, it's recommended to set <code>NO_PUSH=true</code> to verify the changes before pushing the image to the registry.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>This job need only be run once, or when there are updates to be applied based on Dockerfile changes..</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    booleanParam {
      name('NO_PUSH')
      defaultValue(true)
      description('''<p>Build only, do not push to registry.</p>''')
    }
    stringParam {
      name('IMAGE_TAG')
      defaultValue("${OPENBSW_IMAGE_TAG}")
      description('''<p><b>Mandatory:</b> Image tag for the builder image.</p>
        <p>Note: tag may only contain 'abcdefghijklmnopqrstuvwxyz0123456789_-./'</p>''')
      trim(true)
    }
    separator {
      name('OpenBSW Version')
      sectionHeader('OpenBSW Version')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    stringParam {
      name('OPENBSW_GIT_URL')
      defaultValue("${OPENBSW_GIT_URL}")
      description('''<p>OpenBSW Git URL.</p>''')
      trim(true)
    }
    stringParam {
      name('OPENBSW_GIT_BRANCH')
      defaultValue("${OPENBSW_GIT_BRANCH}")
      description('''<p>OpenBSW revision tag/branch name.</p>''')
      trim(true)
    }
    stringParam {
      name('POST_GIT_CLONE_COMMAND')
      defaultValue('git checkout 1c1450e')
      description('''<p>Optional additional commands post git clone and prior to build/make.<br/>
        <b>Note: </b>Single command line only, use logical operators to execute subsequent commands.<br/></p>''')
      trim(true)
    }
    stringParam {
      name('LINUX_DISTRIBUTION')
      defaultValue('ubuntu:noble-20251001')
      description('''<p>Define the Linux distribution to use, e.g.</p></br>
        <ul><li>ubuntu:22.04</li>
            <li>ubuntu:jammy-20251203</li>
            <li>ubuntu:noble-20251001</li></ul>''')
      trim(true)
    }
    separator {
      name('OpenBSW Toolchains')
      sectionHeader('OpenBSW Toolchains')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    stringParam {
      name('ARM_TOOLCHAIN_URL')
      defaultValue('https://developer.arm.com/-/media/Files/downloads/gnu/14.3.rel1/binrel/arm-gnu-toolchain-14.3.rel1-x86_64-arm-none-eabi.tar.xz')
      description('''<p>ARM GNU toolchain archive URL.</p>''')
      trim(true)
    }
    stringParam {
      name('LLVM_ARM_TOOLCHAIN_URL')
      defaultValue('https://github.com/ARM-software/LLVM-embedded-toolchain-for-Arm/releases/download/release-19.1.1/LLVM-ET-Arm-19.1.1-Linux-x86_64.tar.xz')
      description('''<p>LLVM Embedded Toolchain for Arm.</p>''')
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
      name('PLANTUML_URL')
      defaultValue('https://github.com/plantuml/plantuml/releases/download/v1.2025.10/plantuml.jar')
      description('''<p>PlantUML Java archive URL.</p>''')
      trim(true)
    }
    stringParam {
      name('PYTHON_VERSION')
      defaultValue('3.12')
      description('''<p>Python version to install.</p>''')
      trim(true)
    }
    stringParam {
      name('TREEFMT_URL')
      defaultValue('https://github.com/numtide/treefmt/releases/download/v2.1.0/treefmt_2.1.0_linux_amd64.tar.gz')
      description('''<p>Treefmt archive URL.</p>''')
      trim(true)
    }
    stringParam {
      name('RUST_DEFAULT_TOOLCHAIN')
      defaultValue('1.90.0')
      description('''<p>Rust toolchain channel/version passed to <code>rustup</code> <code>--default-toolchain</code> (e.g. <code>1.90.0</code>, <code>stable</code>).</p>''')
      trim(true)
    }
    stringParam {
      name('RUSTUP_PROFILE')
      defaultValue('minimal')
      description('''<p><code>rustup</code> install profile (e.g. <code>minimal</code>, <code>default</code>).</p>''')
      trim(true)
    }
    stringParam {
      name('RUST_EMBEDDED_TARGET')
      defaultValue('thumbv7em-none-eabihf')
      description('''<p>Extra Rust target triple added via <code>rustup target add</code> for S32K148-class embedded builds (must match OpenBSW CMake when cross-compiling Rust).</p>''')
      trim(true)
    }
    stringParam {
      name('CBINDGEN_VERSION')
      defaultValue('0.27.0')
      description('''<p><code>cbindgen</code> version installed with <code>cargo install --locked</code> (pin OpenBSW-compatible releases).</p>''')
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
  blockOn('OpenBSW*.*Docker.*') {
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
      scriptPath('workloads/openbsw/pipelines/environment/docker_image_template/Jenkinsfile')
    }
  }
}

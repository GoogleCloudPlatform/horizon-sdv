// Copyright (C) 2025 The Qt Company Ltd.
// All rights reserved.
//
// This file is part of Squish.
//
// Licensees holding a valid Squish License Agreement may use this
// file in accordance with the Squish License Agreement provided with
// the Software.
//
// This file is provided AS IS with NO WARRANTY OF ANY KIND, INCLUDING THE
// WARRANTY OF DESIGN, MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.
//
// See the LICENSE file in this directory.
//
// Contact qatools.support@qt.io if any conditions of this licensing are
// not clear to you.

pipelineJob('Cloud-Workstations/Workstation-Images/Horizon Squish for Android') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Workstation Image Builder</h3>
    <p>This job builds the container image for Squish for Android for use in Cloud Workstations.</p>
    <h4 style="margin-bottom: 10px;">Image Configuration</h4>
    <p>The Dockerfile uses a minimal Google Cloud Workstation base image and installs essential packages and tools including Android Command Line Tools and SDKs, GNOME desktop environment (with terminal), noVNC and TigerVNC for browser-based remote desktop access, the standard Android emulator, Google Chrome browser, and all necessary system services for a complete Squish experience.</p>
    <h4 style="margin-bottom: 10px;">Pushing Changes to the Registry</h4>
    <p>To push changes to the registry, set the parameter <code>NO_PUSH=false</code>.</p>
    <p>The image will be pushed to <code>${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${CLOUD_WS_HORIZON_SQUISH_FOR_ANDROID_IMAGE_NAME}</code></p>
    <h4 style="margin-bottom: 10px;">Verifying Changes</h4>
    <p>When working with new Dockerfile updates, it's recommended to set <code>NO_PUSH=true</code> to verify the changes before pushing the image to the registry.</p>
    <h4 style="margin-bottom: 10px;">Important Notes</h4>
    <p>This job need only be run once, or when there are updates to be applied based on Dockerfile changes.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  """)

  parameters {
    stringParam {
      name('IMAGE_TAG')
      defaultValue('latest')
      description('''<p>Image tag for the Workstation image.</p>''')
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
      scriptPath('workloads/cloud-workstations/pipelines/workstation-images/horizon-squish-for-android/Jenkinsfile')
    }
  }
}

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
pipelineJob('Android/Builds/AAOS Builder') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Android Automotive Virtual Devices and Platform Targets Builder</h3>
    <p>This job is used to build Android Automotive virtual devices and platform targets from the provided source manifest.</p>
    <h4 style="margin-bottom: 10px;">Supported Builds</h4>
    <ul>
      <li><a href="https://source.android.com/docs/automotive/start/avd/android_virtual_device" target="_blank">Android Virtual Devices</a> for use with <a href="https://source.android.com/docs/automotive/start/avd/android_virtual_device#share-an-avd-image-with-android-studio-users" target="_blank">Android Studio</a></li> 
      <li><a href="https://source.android.com/docs/devices/cuttlefish" target="_blank">Cuttlefish Virtual Devices</a> for use with <a href="https://source.android.com/docs/compatibility/cts" target="_blank">CTS</a></li>
      <li>Reference hardware platforms such as <a href="https://github.com/raspberry-vanilla/android_local_manifest" target="_blank">RPi</a> and <a href="https://source.android.com/docs/automotive/start/pixelxl" target="_blank">Pixel Tablets</a></li>
    </ul>
    <h4 style="margin-bottom: 10px;">Build Outputs</h4>
    <p>Build outputs are stored in a Google Cloud Storage bucket (refer to build artifact for location).</p>
    <h4 style="margin-bottom: 10px;">Viewing Artifacts on Google Cloud</h4>
    <p><a href="https://cloud.google.com/docs/authentication/gcloud" target="_blank">Sign in to Google Cloud</a> and run the following command: <br/><code>gcloud storage ls gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;</code></p>
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
      defaultValue('horizon/android-15.0.0_r36')
      description('''<p>Android revision tag/branch name.</p>''')
      trim(true)
    }

    stringParam {
      name('AAOS_LUNCH_TARGET')
      defaultValue('')
      description('''<p>Build Android cuttlefish, virtual devices, Pixel and RPi target to build.</p>''')
      trim(true)
    }

    choiceParam {
      name('ANDROID_VERSION')
      description('''<p>Version of disk pool to use for the build cache, select from one of the following options:</p>
          <ul>
            <li>default: let job determine pool.</li>
            <li>15: Use the Android 15 disk pool, if target is for RPi the Android 15 RPi pool will be used.</li>
            <li>14: Use the Android 14 disk pool, if target is for RPi the Android 14 RPi pool will be used.</li>
          </ul>''')
      choices(['default', '15', '14'])
    }

    stringParam {
      name('POST_REPO_INITIALISE_COMMAND')
      defaultValue('')
      description('''<p>Optional additional commands post repo init and prior to repo sync, build/make.<br/>
        <b>Note: </b>Single command line only, use logical operators to execute subsequent commands.<br/><br/></p>''')
      trim(true)
    }

    stringParam {
      name('POST_REPO_COMMAND')
      defaultValue('')
      description('''<p>Optional additional commands post repo sync/fetch, git clone and prior to build/make.<br/>
        <b>Note: </b>Single command line only, use logical operators to execute subsequent commands.<br/><br/></p>''')
      trim(true)
    }

    stringParam {
      name('OVERRIDE_MAKE_COMMAND')
      defaultValue('')
      description('''<p>Optional override default make command.<br/>
        <b>Note: </b>Single command line only, use logical operators to execute subsequent commands.</p>''')
      trim(true)
    }

    choiceParam {
      name('AAOS_CLEAN')
      description('''<p>Clean build or cache directories, e.g.
        <ul>
          <li>NO_CLEAN : do not clean</li>
          <li>CLEAN_BUILD : this will clean the build target output directory</li>
          <li>CLEAN_ALL : this will clear the whole cache including source</li>
        </ul>
        <b>Warning:</b> Only use when necessary.</p>''')
      choices(['NO_CLEAN', 'CLEAN_BUILD', 'CLEAN_ALL'])
    }

    stringParam {
      name('GERRIT_REPO_SYNC_JOBS')
      defaultValue("${REPO_SYNC_JOBS}")
      description('''<p>Number of parallel sync jobs for <i>repo sync</i>.<br/>
        Default value is defined by the Android Seed job</p>''')
      trim(true)
    }

    choiceParam {
      name('INSTANCE_RETENTION_TIME')
      description('''<p>Time in minutes to retain the instance after build completion.<br/>
        Useful for debugging build issues, reviewing target outputs etc.</p>''')
      choices(['0', '15', '30', '45', '60', '120', '180'])
    }

    stringParam {
      name('AAOS_ARTIFACT_STORAGE_SOLUTION')
      defaultValue('GCS_BUCKET')
      description('''<p>Android Artifact Storage:<br/>
        <ul><li>GCS_BUCKET will store to cloud bucket storage</li>
        <li>Empty will result in nothing stored</li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('GERRIT_PROJECT')
      defaultValue('')
      description('''<p>Optional, define Gerrit Project with open review.</p>''')
      trim(true)
    }

    stringParam {
      name('GERRIT_CHANGE_NUMBER')
      defaultValue('')
      description('''<p>Optional, define Gerrit review item change number.</p>''')
      trim(true)
    }

    stringParam {
      name('GERRIT_PATCHSET_NUMBER')
      defaultValue('')
      description('''<p>Optional, define Gerrit review item patchset number.</p>''')
      trim(true)
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
      scriptPath('workloads/android/pipelines/builds/aaos_builder/Jenkinsfile')
    }
  }
}

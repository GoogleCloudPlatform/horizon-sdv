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
pipelineJob('Android/Builds/AAOS Builder ABFS') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Android Build Filesystem Builder</h3>
    <p>This job is used to build Android Automotive virtual devices and platform targets using the source and build caches from the Android Build Filesystem.</p>
    <h4 style="margin-bottom: 10px;">Supported Builds</h4>
    <ul>
      <li><a href="https://source.android.com/docs/automotive/start/avd/android_virtual_device" target="_blank">Android Virtual Devices</a> for use with <a href="https://source.android.com/docs/automotive/start/avd/android_virtual_device#share-an-avd-image-with-android-studio-users" target="_blank">Android Studio</a></li>
      <li><a href="https://source.android.com/docs/devices/cuttlefish" target="_blank">Cuttlefish Virtual Devices</a> for use with <a href="https://source.android.com/docs/compatibility/cts" target="_blank">CTS</a></li>
      <li>Reference hardware platforms such as <a href="https://source.android.com/docs/automotive/start/pixelxl" target="_blank">Pixel Tablets</a></li>
    </ul>
    <p>Users have the ability to retain the ABFS cache and ABFS source mount point in persistent storage, this may improve build times. Simply enable <code>ABFS_CACHED_BUILD</code> and a persistent volume will be created to store the cache and source mount path.</p>
    <h4 style="margin-bottom: 10px;">Build Outputs</h4>
    <p>Build outputs are stored in a Google Cloud Storage bucket (refer to build artifact for location).</p>
    <h4 style="margin-bottom: 10px;">Viewing Artifacts on Google Cloud</h4>
    <p><a href="https://cloud.google.com/docs/authentication/gcloud" target="_blank">Sign in to Google Cloud</a> and run the following command: <br/>
    <code>gcloud storage ls gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder_ABFS/&lt;BUILD_NUMBER&gt;</code></p>
    <h4 style="margin-bottom: 10px;">Important</h4>
    <p>Regardless of the build outcome, the <code>abfs_repository_list.txt</code> file will be generated. This file is
crucial for correlating <code>ABFS_VERSION</code> and <code>ABFS_CASFS_VERSION</code> with the build instance kernel revision.<br/>
    Please review the output of this file and update the <code>Seed Workloads</code> values for ABFS versions accordingly. This ensures you utilize the latest versions provided by Google, as they are subject to updates.</p>
    <h4 style="margin-bottom: 10px;">Prerequisites</h4>
    <p>Refer to abfs.md for setting up ABFS for the GCP project.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('AAOS_REVISION')
      defaultValue('android-15.0.0_r36')
      description('''<p>Android revision tag/branch name<br/>
      <b>Note:</b> ensure the ABFS uploader has been run on your branch.</p>''')
      trim(true)
    }

    stringParam {
      name('AAOS_LUNCH_TARGET')
      defaultValue('aosp_cf_x86_64_auto-bp1a-userdebug')
      description('''<p>Build Android cuttlefish, virtual devices and Pixel target.<br/>
      <b>Note:</b> RPi not supported with ABFS.</p>''')
      trim(true)
    }

    choiceParam {
      name('ANDROID_VERSION')
      description('''<p>Version of Android required for SDK generation of addons and devices.</p>''')
      choices(['15', '14'])
    }

    booleanParam {
      name('ABFS_CACHED_BUILD')
      defaultValue(false)
      description('''<p>The ABFS cache and source mount path will be stored in a persistent volume for other builds to use.<br>
        Used in conjunction with <code>ABFS_CACHEMAN_TIMEOUT</code> and may improve future build times.</p>''')
    }

    stringParam {
      name('ABFS_CACHEMAN_TIMEOUT')
      defaultValue('180')
      description('''<p>Cacheman timeout in seconds. Only applicable if <code>ABFS_CACHED_BUILD</code>.</p>''')
      trim(true)
    }

    booleanParam {
      name('ABFS_CLEAN_CACHE')
      defaultValue(false)
      description('''<p>Clean the ABFS cache directory</p>''')
    }

    stringParam {
      name('POST_REPO_COMMAND')
      defaultValue('''
        cd build/soong ; \
git fetch https://android.googlesource.com/platform/build/soong refs/changes/90/3619490/1 && git cherry-pick FETCH_HEAD; \
git fetch https://android.googlesource.com/platform/build/soong refs/changes/91/3619491/1 && git cherry-pick FETCH_HEAD; \
git fetch https://android.googlesource.com/platform/build/soong refs/changes/92/3619492/1 && git cherry-pick FETCH_HEAD; cd -''')
      description('''<p>Optional additional commands post repo sync/fetch, git clone and prior to build/make.<br/>
        The values here are for ABFS Android 15.<br/>
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

    stringParam {
      name('AAOS_GERRIT_MANIFEST_URL')
      defaultValue("https://${HORIZON_DOMAIN}/gerrit/android/platform/manifest")
      description('''<p>Gerrit manifest URL for patchset.<br>
        Manifest is required so project can be matched to path within the source tree in order to fetch the change.</p>''')
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

    choiceParam {
      name('INSTANCE_RETENTION_TIME')
      description('''<p>Time in minutes to retain the instance after build completion.<br/>
        Useful for debugging build issues, reviewing target outputs etc.</p>''')
      choices(['0', '15', '30', '45', '60', '120', '180'])
    }

    stringParam {
      name('ABFS_VERSION')
      defaultValue("${ABFS_VERSION}")
      description('''<p>ABFS version, e.g. 0.0.33-2-ge59ffbc</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_CASFS_VERSION')
      defaultValue("${ABFS_CASFS_VERSION}")
      description('''<p>ABFS CASFS version, if differs from ABFS version, e.g. 0.0.33-10-g654e659</p>''')
      trim(true)
    }

    stringParam {
      name('ABFS_REPOSITORY')
      defaultValue("${ABFS_REPOSITORY}")
      description('''<p>ABFS aptitude repository, e.g. abfs-apt-alpha-public. </p>''')
      trim(true)
    }

    stringParam {
      name('UPLOADER_MANIFEST_SERVER')
      defaultValue("${UPLOADER_MANIFEST_SERVER}")
      description('''<p>Gerrit manifest server.</p>''')
      trim(true)
    }

    stringParam {
      name('AAOS_ARTIFACT_STORAGE_SOLUTION')
      defaultValue('GCS_BUCKET')
      description('''<p>Android Artifact Storage:<br/>
        <ul><li>GCS_BUCKET will store to cloud bucket storage</li>
        <li>Empty will result in nothing stored</li></ul></p>''')
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
      scriptPath('workloads/android/pipelines/builds/aaos_abfs_builder/Jenkinsfile')
    }
  }
}

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
pipelineJob('Android/Environment/CF Instance Template') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">GCE x86_64 Instance Template Creation Job</h3>
    <p>This job creates the GCE instance templates used by test pipelines to spin up cuttlefish-ready and CTS-ready cloud instances, which are then used to launch <a href="https://source.android.com/docs/devices/cuttlefish" target="_blank" title="Cuttlefish Virtual Device">CVD</a> and run <a href="https://source.android.com/docs/compatibility/cts" target="_blank" title="Compatibility Test Suite">CTS</a> tests. Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">Instance Template Naming</h4>
    <p>The name for the created instance template can either be auto-generated or user-provided
(<code>CUTTLEFISH_INSTANCE_NAME</code>). The resulting artifact will be <code>instance-template-&lt;name&gt;</code>. If a user-defined name is defined, or non-standard revisions are used, the Jenkins CasC (<code>values-jenkins.yaml</code>) must be updated with a new <code>computeEngine</code> entry for the template.</p>
    <h4 style="margin-bottom: 10px;">Machine Type </h4>
    <p>Users may choose to create the VM instance template from a standard machine type or define based on custom options.</p>
    <p>Set <code>MACHINE_TYPE</code> parameter to an empty string and populate the custom options, i.e.:<br/>
    <ul><li><code>CUSTOM_VM_TYPE</code></li>
        <li><code>CUSTOM_CPU</code></li>
        <li><code>CUSTOM_MEMORY</code></li></ul>
    <h4 style="margin-bottom: 10px;">Updating and Deleting Outdated Instances</h4>
    <p>This job can also be used to update and replace existing instances or delete outdated instances and associated artifacts.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    separator {
      name('Git Repository Details')
      sectionHeader('Git Repository details')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('ANDROID_CUTTLEFISH_REPO_URL')
      defaultValue('https://github.com/google/android-cuttlefish.git')
      description('''<p>Users may provide their own URL to a mirror, forked version but if private then you must provide
the repo credentials, i.e.
       <ul><li><code>REPO_USERNAME</code></li>
           <li><code>REPO_PASSWORD</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('ANDROID_CUTTLEFISH_REVISION')
      defaultValue('')
      description('''<p>The branch/tag version of Android Cuttlefish to use, e.g.</p>
        <ul>
          <li>v1.35.0</li>
          <li>main</li>
          <li>horizon/main</li>
          <li>horizon/v1.35.0</li>
        </ul>
        <p>Mandatory for instance creation, not applicable for deletion if <code>CUTTLEFISH_INSTANCE_NAME</code> is defined.</p>
        <p>Reference: <a href="https://github.com/google/android-cuttlefish.git" target="_blank">android-cuttlefish.git</a></p>''')
      trim(true)
    }

    stringParam {
      name('CUTTLEFISH_INSTANCE_NAME')
      defaultValue('')
      description('''<p>Optional parameter to define the unique name used for the instance template, e.g.  <i>cuttlefish-vm-instance-test-v1350</i><br/>
        Name must start with <i>cuttlefish-vm</i>, refer to docs for details on regex requirements for name.<br/>
        Default: The name will be automatically derived from <code>ANDROID_CUTTLEFISH_REVISION</code>, e.g. <i>cuttlefish-vm-v1350</i><br/><br/></p>''')
      trim(true)
    }

    booleanParam {
      name('DELETE')
      defaultValue(false)
      description('''<p>Delete existing templates, skip creation steps.<br/>
        Useful for removing old instances to reduce costs.<br/>
        <b>Note:</b>
          <ul><li>Define the <code>CUTTLEFISH_INSTANCE_NAME</code> parameter if non-standard instance is to be deleted</li>
              <li>Define the <code>ANDROID_CUTTLEFISH_REVISION</code> revision for standard instance deletion.</li></ul></p>''')
    }

    stringParam {
      name('REPO_USERNAME')
      defaultValue('')
      description('''<p>Optional username credential when using private repos: <code>ANDROID_CUTTLEFISH_REPO_URL</code>.</p>''')
      trim(true)
    }

    nonStoredPassword {
      name('REPO_PASSWORD')
      description('''<p>Optional password credential when using private repos: <code>ANDROID_CUTTLEFISH_REPO_URL</code>.</p>''')
    }

    stringParam {
      name('ANDROID_CUTTLEFISH_POST_COMMAND')
      defaultValue('')
      description('''<p>Command to run in the <code>ANDROID_CUTTLEFISH_REPO_URL</code> repository</a> to workaround issues etc,  e.g.
        <ul><li>Cherry pick: <code>git cherry-pick 655de58f</code></li>
            <li>Checkout commit: <code>git checkout 655de58f</code></li></ul></p>''')
      trim(true)
    }

    booleanParam {
      name('ANDROID_CUTTLEFISH_PREBUILT')
      defaultValue(false)
      description('''<p>Use Google Cuttlefish prebuilt packages.<br/>
       <p>Choose whether to download and install Google prebuilt version instead of building from the <code>ANDROID_CUTTLEFISH_REPO_URL</code> repository.</p>
       <p>If disabled, cuttlefish is built and installed, if enabled and versions exist, then cuttlefish prebuilt packages are installed.</p>
       <p><b>Note:</b> this is only applicable to <code>ANDROID_CUTTLEFISH_REVISION=main</code>. If packages are not available, cuttlefish will be built from scratch.<br/></p>''')
    }

    booleanParam {
      name('VM_INSTANCE_CREATE')
      defaultValue(false)
      description('''<p>If enabled, job will create a Cuttlefish VM instance in a stopped state, created from final instance template.<br/></p>''')
    }

    separator {
      name('Custom Machine Type')
      sectionHeader('Custom Machine Type')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('MACHINE_TYPE')
      defaultValue('n2-standard-32')
      description('''<p><strong>Optional:</strong> The machine type to use when creating the instance, e.g.</p>
        <ul>
          <li>n2-standard-32</li>
          <li>n1-standard-64</li>
        </ul>
        <p>Leave empty if creating custom machine type using options below.</p>
        <p>Refer to <a href="https://cloud.google.com/compute/docs/general-purpose-machines" target="_blank">General-purpose machine family for Compute Engine</a> for additional details, i.e. <code>--machine-type=MACHINE_TYPE</code></li></uL></p>''')
      trim(true)
    }

    stringParam {
      name('CUSTOM_VM_TYPE')
      defaultValue('')
      description('''<p><strong>Optional:</strong> Specifies a custom machine type, e.g.<br/>
        <ul>
          <li>n2</li>
          <li>n1</li>
        </ul>
        <p><b>Note:</b>
        <ul><li>Option is only valid when <code>MACHINE_TYPE</code> is undefined.</li>
            <li>Refer to <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">Create Instance Template</a> for additional details, i.e.  <code>--custom-vm-type</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('CUSTOM_CPU')
      defaultValue('32')
      description('''<p><strong>Optional:</strong> Specifies the number of cores needed for custom machine type. e.g. <br/>
        <ul>
          <li>32</li>
          <li>64</li>
        </ul>
        <p><b>Note:</b>
        <ul><li>Option must be specified when <code>CUSTOM_VM_TYPE</code> is defined.</li>
            <li>Refer to  <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">Create Instance Template</a>, for additional details, i.e.  <code>--custom-cpu</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('CUSTOM_MEMORY')
      defaultValue('64GB')
      description('''<p><strong>Optional:</strong> Specifies the memory needed for custom machine type. e.g. <br/>
        <ul>
          <li>64GB</li>
          <li>96GB</li>
        </ul>
        <p><b>Note:</b>
        <ul><li>Option must be specified when <code>CUSTOM_VM_TYPE</code> is defined.</li>
            <li>Refer to  <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">Create Instance Template</a>, for additional details, i.e. <code>--custom-memory</code></li>
            <li>A size unit should be provided (eg. 3072MB or 9GB) - if no units are specified, GB is assumed</li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('BOOT_DISK_TYPE')
      defaultValue("pd-balanced")
      description('''<p>Boot disk type.</p>''')
      trim(true)
    }

    stringParam {
      name('BOOT_DISK_SIZE')
      defaultValue('250GB')
      description('''<p>The boot disk size for the instance template image, e.g.</p>
        <ul>
          <li>250GB</li>
          <li>500GB</li>
        </ul>
        <p>Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk=[PROPERTY=VALUE,…]</i></p>''')
      trim(true)
    }

    stringParam {
      name('MAX_RUN_DURATION')
      defaultValue('12h')
      description('''<p>Limits how long this VM instance can run.<br/>
        Useful to avoid excessive costs. Set to 0 to disable limit.<br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instances/create" target="_blank">gcloud compute instances create</a>, i.e. <i>--max-run-duration=MAX_RUN_DURATION</i></p>''')
      trim(true)
    }

    separator {
      name('Software Versions')
      sectionHeader('Software Versions')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('JAVA_VERSION')
      defaultValue('openjdk-17-jdk-headless')
      description('''<p>OpenJDK Java version to install.<br/>
        Use <code>headless</code> to avoid issues with installing in various operating system versions.</p>''')
      trim(true)
    }

    stringParam {
      name('OS_VERSION')
      defaultValue('debian-12-bookworm-v20251209')
      description('''<p>Disk image OS version.<br/>
        Select the OS version name based on project and family, e.g <code>`gcloud compute images list</code>`<br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk</i></p>''')
      trim(true)
    }

    stringParam {
      name('OS_PROJECT')
      defaultValue('debian-cloud')
      description('''<p>Disk image project.<br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk</i></p>''')
      trim(true)
    }

    stringParam {
      name('CURL_UPDATE_COMMAND')
      defaultValue('sudo apt install -t bookworm-backports -y curl libcurl4')
      description('''<p>Update Curl from debian backports.<br/>
        Users may choose to tailor the installation command to suit their requirements.</p>''')
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
      name('CTS Download URLs')
      sectionHeader('CTS Versions to Install')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('CTS_ANDROID_16_URL')
      defaultValue("https://dl.google.com/dl/android/cts/android-cts-16_r3-linux_x86-x86.zip")
      description('''<p>Leave blank if the version is not needed, or specify your preferred version.<br/>
      Either download from official site, or from a local bucket if stored locally to improve download times, e.g.
      <ul><li>Official downloads: <code>https://dl.google.com/dl/android/cts/android-cts-16_r3-linux_x86-x86.zip/code></li>
          <li>Local GCS bucket download: <code>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/CTS/android-cts-16_r3-linux_x86-x86.zip</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('CTS_ANDROID_15_URL')
      defaultValue("https://dl.google.com/dl/android/cts/android-cts-15_r6-linux_x86-x86.zip")
      description('''<p>Leave blank if the version is not needed, or specify your preferred version.<br/>
      Either download from official site, or from a local bucket if stored locally to improve download times, e.g.
      <ul><li>Official downloads: <code>https://dl.google.com/dl/android/cts/android-cts-15_r6-linux_x86-x86.zip/code></li>
          <li>Local GCS bucket download: <code>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/CTS/android-cts-15_r6-linux_x86-x86.zip</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('CTS_ANDROID_14_URL')
      defaultValue("https://dl.google.com/dl/android/cts/android-cts-14_r10-linux_x86-x86.zip")
      description('''<p>Leave blank if the version is not needed, or specify your preferred version.<br/>
      Either download from official site, or from a local bucket if stored locally to improve download times, e.g.
      <ul><li>Official downloads: <code>https://dl.google.com/dl/android/cts/android-cts-14_r10-linux_x86-x86.zip/code></li>
          <li>Local GCS bucket download: <code>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/CTS/android-cts-14_r10-linux_x86-x86.zip</code></li></ul></p>''')
      trim(true)
    }
  }

  // Block build if certain jobs are running.
  blockOn('Android*.*Template.*') {
    // Possible values are 'GLOBAL' and 'NODE' (default).
    blockLevel('GLOBAL')
    // Possible values are 'ALL', 'BUILDABLE' and 'DISABLED' (default).
    scanQueueFor('BUILDABLE')
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
      scriptPath('workloads/android/pipelines/environment/cf_instance_template/Jenkinsfile')
    }
  }
}


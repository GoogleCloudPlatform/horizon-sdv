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
    <br/><h3 style="margin-bottom: 10px;">GCE Instance Template Creation Job</h3>
    <p>This job creates the GCE instance templates used by test pipelines to spin up cuttlefish-ready and CTS-ready cloud instances, which are then used to launch <a href="https://source.android.com/docs/devices/cuttlefish" target="_blank" title="Cuttlefish Virtual Device">CVD</a> and run <a href="https://source.android.com/docs/compatibility/cts" target="_blank" title="Compatibility Test Suite">CTS</a> tests. Refer to the README.md in the respective repository for further details.</p>
    <h4 style="margin-bottom: 10px;">Instance Template Naming</h4>
    <p>The name for the created instance template can either be auto-generated or user-provided (<code>CUTTLEFISH_INSTANCE_UNIQUE_NAME</code>). The resulting artifact will be <code>instance-template-&lt;name&gt;</code>. If a user-defined name is used, the Jenkins CasC (<code>jenkins.yaml</code>) must be updated with a new <code>computeEngine</code> entry for the template.</p>
    <h4 style="margin-bottom: 10px;">Updating and Deleting Outdated Instances</h4>
    <p>This job can also be used to update and replace existing instances or delete outdated instances and associated artifacts.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('ANDROID_CUTTLEFISH_REVISION')
      defaultValue('')
      description('''<p>The branch/tag version of Android Cuttlefish to use, e.g.</p>
        <ul>
          <li>main</li>
          <li>v1.18.0</li>
        </ul>
        <p>Reference: <a href="https://github.com/google/android-cuttlefish.git" target="_blank">android-cuttlefish.git</a></p>''')
      trim(true)
    }

    stringParam {
      name('CUTTLEFISH_INSTANCE_UNIQUE_NAME')
      defaultValue('')
      description('''<p>Optional parameter to define the unique name used for the instance template, e.g.  <i>cuttlefish-vm-instance-test-v1180</i><br/>
        Name must start with <i>cuttlefish-vm</i>, refer to docs for details on regex requirements for name.<br/>
        Default: The name will be automatically derived from ANDROID_CUTTLEFISH_REVISION., e.g. <i>cuttlefish-vm-v1180</i><br/><br/></p>''')
      trim(true)
    }

    stringParam {
      name('MACHINE_TYPE')
      defaultValue('n1-standard-64')
      description('''<p>The machine type to use when creating the instance, e.g..</p>
        <ul>
          <li>n1-standard-64</li>
          <li>n1-standard-32</li>
          <li>n1-standard-16</li>
          <li>n1-standard-8</li>
        </ul>
        <p>Reference: <a href="https://cloud.google.com/compute/docs/general-purpose-machines" target="_blank">General-purpose machine family for Compute Engine</a> i.e. <i>--machine-type=MACHINE_TYPE</i></p>''')
      trim(true)
    }

    stringParam {
      name('BOOT_DISK_SIZE')
      defaultValue('200GB')
      description('''<p>The boot disk size for the instance template image, e.g..</p>
        <ul>
          <li>200GB</li>
          <li>150GB</li>
        </ul>
        <p>Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk=[PROPERTY=VALUE,…]</i></p>''')
      trim(true)
    }

    stringParam {
      name('MAX_RUN_DURATION')
      defaultValue('10h')
      description('''<p>Limits how long this VM instance can run.<br/>
        Useful to avoid excessive costs. Set to 0 to disable limit.<br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instances/create" target="_blank">gcloud compute instances create</a>, i.e. <i>--max-run-duration=MAX_RUN_DURATION</i></p>''')
      trim(true)
    }

    stringParam {
      name('DEBIAN_OS_VERSION')
      defaultValue('debian-12-bookworm-v20250812')
      description('''<p>Disk image OS version.<br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk</i></p>''')
      trim(true)
    }

    stringParam {
      name('NODEJS_VERSION')
      defaultValue("${NODEJS_VERSION}")
      description('''<p>NodeJS version.<br/>
        This is installed using <i>nvm</i> on the instance template to be compatible with other tooling.</p>''')
      trim(true)
    }

    booleanParam {
      name('DELETE')
      defaultValue(false)
      description('''<p>Delete existing templates, skip creation steps.<br/>
        Useful for removing old instances to reduce costs.<br/>
        <b>Note:</b> Define the CUTTLEFISH_INSTANCE_UNIQUE_NAME if non-standard instance is to be deleted, else simply define the version in ANDROID_CUTTLEFISH_REVISION field.</p>''')
    }

    booleanParam {
      name('VM_INSTANCE_CREATE')
      defaultValue(false)
      description('''<p>If enabled, job will create a Cuttlefish VM instance in a stopped state, using the final instance template.</p>''')
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


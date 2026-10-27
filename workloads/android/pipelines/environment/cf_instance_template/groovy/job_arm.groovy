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
pipelineJob('Android/Environment/CF Instance Template ARM64') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">GCE ARM64 Instance Template Creation Job</h3>
    <p>Use this job to build and manage the ARM64 Cuttlefish instance template consumed by test pipelines (CVD Launcher, CTS, Gerrit test stage, etc.).</p>

    <h4 style="margin-bottom: 10px;">How to use this job</h4>
    <ol>
      <li><b>Build or update template (normal path)</b>
        <ul>
          <li>Set <code>DELETE=false</code></li>
          <li>Set <code>UPDATE_SSH_AUTHORIZED_KEYS=false</code></li>
          <li>Provide <code>ANDROID_CUTTLEFISH_REVISION</code> (or a custom <code>CUTTLEFISH_INSTANCE_NAME</code>)</li>
          <li>Run build (creates/updates image + instance template)</li>
        </ul>
      </li>
      <li><b>Refresh SSH key metadata only (no image rebuild)</b>
        <ul>
          <li>Set <code>DELETE=false</code></li>
          <li>Set <code>UPDATE_SSH_AUTHORIZED_KEYS=true</code></li>
          <li>Set target using <code>ANDROID_CUTTLEFISH_REVISION</code> or <code>CUTTLEFISH_INSTANCE_NAME</code></li>
          <li>Run build (republishes template metadata for startup key refresh)</li>
        </ul>
      </li>
      <li><b>Delete artifacts</b>
        <ul>
          <li>Set <code>DELETE=true</code></li>
          <li>Set target via <code>ANDROID_CUTTLEFISH_REVISION</code> or <code>CUTTLEFISH_INSTANCE_NAME</code></li>
          <li>Run build (deletes template/image/related resources)</li>
        </ul>
      </li>
    </ol>

    <h4 style="margin-bottom: 10px;">Naming and machine type</h4>
    <p>If <code>CUTTLEFISH_INSTANCE_NAME</code> is empty, name is derived from <code>ANDROID_CUTTLEFISH_REVISION</code>. Name must start with <code>cuttlefish-vm</code>.</p>
    <p>Use a standard shape via <code>MACHINE_TYPE</code> or leave it empty and set custom values:
    <code>CUSTOM_VM_TYPE</code>, <code>CUSTOM_CPU</code>, <code>CUSTOM_MEMORY</code>.</p>

    <h4 style="margin-bottom: 10px;">SSH key behavior</h4>
    <p>The public key is derived from <code>SSH_PRIVATE_KEY_NAME</code> and injected into template metadata.
    On VM boot, startup script rewrites <code>/home/jenkins/.ssh/authorized_keys</code> from metadata.</p>

    <h4 style="margin-bottom: 10px;">References</h4>
    <ul>
      <li><a href="https://source.android.com/docs/devices/cuttlefish" target="_blank" title="Cuttlefish Virtual Device">Cuttlefish Virtual Device (CVD)</a></li>
      <li><a href="https://github.com/google/android-cuttlefish" target="_blank" title="android-cuttlefish repository">android-cuttlefish repository</a></li>
      <li><a href="https://source.android.com/docs/compatibility/cts/downloads" target="_blank" title="Compatibility Test Suite downloads">CTS downloads</a></li>
      <li><a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank" title="gcloud compute instance-templates create">GCE instance template creation command</a></li>
    </ul>

    <p>See docs for full parameter details and examples.</p>
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
          <li>v1.55.3</li>
          <li>main</li>
          <li>horizon/main</li>
          <li>horizon/v1.55.3</li>
        </ul>
        <p>Mandatory for instance creation, not applicable for deletion if <code>CUTTLEFISH_INSTANCE_NAME</code> is defined.</p>
        <p>Reference: <a href="https://github.com/google/android-cuttlefish.git" target="_blank">android-cuttlefish.git</a></p>''')
      trim(true)
    }

    stringParam {
      name('CUTTLEFISH_INSTANCE_NAME')
      defaultValue('')
      description('''<p>Optional parameter to define the unique name used for the instance template, e.g.  <i>cuttlefish-vm-instance-test-v1553-arm64</i><br/>
        Name must start with <i>cuttlefish-vm</i>, refer to docs for details on regex requirements for name.<br/>
        Default: The name will be automatically derived from <code>ANDROID_CUTTLEFISH_REVISION</code>, e.g.  <i>cuttlefish-vm-v1553-arm64</i><br/><br/></p>''')
      trim(true)
    }

    booleanParam {
      name('DELETE')
      defaultValue(false)
      description('''<p>Delete existing templates, skip creation steps.<br/>
        Useful for removing old instances to reduce costs.<br/>
        <b>Note:</b>
          <ul><li>Define the <code>CUTTLEFISH_INSTANCE_NAME</code> parameter if non-standard instance is to be deleted</li>
              <li>Define the <code>ANDROID_CUTTLEFISH_REVISION</code> revision for standard instance deletion.</li></ul></p>
        <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>''')
    }

    booleanParam {
      name('UPDATE_SSH_AUTHORIZED_KEYS')
      defaultValue(false)
      description('''<p>Refresh SSH authorized_keys metadata on the existing instance template without rebuilding the Packer image.<br/>
        Use this after rotating the Jenkins SSH key (<code>SSH_PRIVATE_KEY_NAME</code>).<br/>
        Keep <code>DELETE=false</code> when using this option.</p>
        <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>''')
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
      defaultValue('grep -q rust.toolchain base/cvd/toolchain/rust.MODULE.bazel || printf \'\\nrust = use_extension(\\042@rules_rust//rust:extensions.bzl\\042, \\042rust\\042)\\nrust.toolchain(edition = \\0422021\\042, versions = [\\0421.89.0\\042])\\n\' >> base/cvd/toolchain/rust.MODULE.bazel')
      description('''<p>Command to run in the <code>ANDROID_CUTTLEFISH_REPO_URL</code> repository</a> to workaround issues etc,  e.g.
        <ul><li>Cherry pick: <code>git cherry-pick b3e4bd9</code></li>
            <li>Checkout commit: <code>git checkout 655de58f</code></li></ul></p>''')
      trim(true)
    }

    separator {
      name('Custom Machine Type')
      sectionHeader('Custom Machine Type')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('MACHINE_TYPE')
      defaultValue('c4a-highmem-96-metal')
      description('''<p>The ARM64 machine type to use when creating the instance, e.g.</p>
        <ul>
          <li>c4a-highmem-96-metal</li>
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
          <li>n1</li>
          <li>n2</li>
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
      defaultValue("hyperdisk-balanced")
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
      name('WORKFLOWS_NAMESPACE')
      defaultValue('workflows')
      description('''<p>Kubernetes namespace where the KCC <code>ComputeInstanceTemplate</code> resources are applied (namespaced Config Connector).<br/>
        Default: <code>workflows</code>.</p>''')
      trim(true)
    }

    stringParam {
      name('JAVA_VERSION')
      defaultValue('openjdk-21-jdk-headless')
      description('''<p>Apt package for the JDK (ARM64 / Ubuntu). Default <code>openjdk-21-jdk-headless</code>.<br/>
        Temurin is optional: set <code>temurin-21-jdk</code> if you want Adoptium (script adds Adoptium apt when the name starts with <code>temurin-</code>).</p>''')
      trim(true)
    }

    stringParam {
      name('OS_VERSION')
      defaultValue('ubuntu-2204-jammy-arm64-v20251212')
      description('''<p>ARM64 Disk image OS version.<br/>
        Select the OS version name based on project and family, e.g. <code>gcloud compute images list</code><br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk</i></p>''')
      trim(true)
    }

    stringParam {
      name('OS_PROJECT')
      defaultValue('ubuntu-os-cloud')
      description('''<p>Disk image project.<br/>
        Reference: <a href="https://cloud.google.com/sdk/gcloud/reference/compute/instance-templates/create" target="_blank">gcloud compute instance-templates create</a>, i.e. <i>--create-disk</i></p>''')
      trim(true)
    }

    stringParam {
      name('CURL_UPDATE_COMMAND')
      defaultValue("")
      description('''<p>Update Curl command.<br/>
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
      defaultValue("https://dl.google.com/dl/android/cts/android-cts-16_r5-linux_x86-arm.zip")
      description('''<p>Leave blank if the version is not needed, or specify your preferred version.<br/>
      Either download from official site, or from a local bucket if stored locally to improve download times, e.g.
      <ul><li>Official downloads: <code>https://dl.google.com/dl/android/cts/android-cts-16_r5-linux_x86-arm.zip</code></li>
          <li>Local GCS bucket download: <code>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/CTS/android-cts-16_r5-linux_x86-arm.zip</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('CTS_ANDROID_15_URL')
      defaultValue("https://dl.google.com/dl/android/cts/android-cts-15_r8-linux_x86-arm.zip")
      description('''<p>Leave blank if the version is not needed, or specify your preferred version.<br/>
      Either download from official site, or from a local bucket if stored locally to improve download times, e.g.
      <ul><li>Official downloads: <code>https://dl.google.com/dl/android/cts/android-cts-15_r8-linux_x86-arm.zip</code></li>
          <li>Local GCS bucket download: <code>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/CTS/android-cts-15_r8-linux_x86-arm.zip</code></li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('CTS_ANDROID_14_URL')
      defaultValue("https://dl.google.com/dl/android/cts/android-cts-14_r12-linux_x86-arm.zip")
      description('''<p>Leave blank if the version is not needed, or specify your preferred version.<br/>
      Either download from official site, or from a local bucket if stored locally to improve download times, e.g.
      <ul><li>Official downloads: <code>https://dl.google.com/dl/android/cts/android-cts-14_r12-linux_x86-arm.zip</code></li>
          <li>Local GCS bucket download: <code>gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/CTS/android-cts-14_r12-linux_x86-arm.zip</code></li></ul></p>''')
      trim(true)
    }

    separator {
      name('ARM64 Unique Configuration')
      sectionHeader('ARM64 Locations')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('ADDITIONAL_NETWORKING')
      defaultValue("nic-type=IDPF")
      description('''<p>ARM64 require IDPF network interface.</p>''')
      trim(true)
    }

    stringParam {
      name('SUBNET')
      defaultValue("${ARM64_SUBNETWORK}")
      description('''<p>ARM64 subnet (<code>ARM64_SUBNETWORK</code> from Terraform / Jenkins global env). Clear to fall back to script env.</p>''')
      trim(true)
    }

    stringParam {
      name('REGION')
      defaultValue("${ARM64_REGION}")
      description('''<p>ARM64 GCP region (from Terraform / Jenkins <code>ARM64_REGION</code>). Clear to fall back to script env.</p>''')
      trim(true)
    }

    stringParam {
      name('ZONE')
      defaultValue("${ARM64_ZONE}")
      description('''<p>ARM64 GCP zone (from Terraform / Jenkins <code>ARM64_ZONE</code>). Clear to fall back to script env.</p>''')
      trim(true)
    }

    separator {
      name('Packer Plugin')
      sectionHeader('Packer googlecompute Plugin Version')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('PACKER_GOOGLECOMPUTE_VERSION_MIN')
      defaultValue('1.2.3')
      description('''<p>Minimum hashicorp/googlecompute Packer plugin version (inclusive).<br/>
        Requires &ge; 1.2.3 for <code>max_run_duration_in_seconds</code> / <code>instance_termination_action</code> on the ephemeral builder VM.</p>''')
      trim(true)
    }

    stringParam {
      name('PACKER_GOOGLECOMPUTE_VERSION_MAX')
      defaultValue('1.2.5')
      description('''<p>Exclusive upper bound for the hashicorp/googlecompute Packer plugin version.<br/>
        Capped below 1.2.5: v1.2.5 SIGSEGVs in <code>StepImportOSLoginSSHKey</code> on Argo read-only <code>/workspace</code>.
        Raise this once upstream fixes it &mdash; no code change needed.</p>''')
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
            url("${HORIZON_SCM_URL}")
            credentials('jenkins-scm-creds')
          }
          branch("*/${HORIZON_SCM_BRANCH}")
        }
      }
      scriptPath('workloads/android/pipelines/environment/cf_instance_template/Jenkinsfile')
    }
  }
}


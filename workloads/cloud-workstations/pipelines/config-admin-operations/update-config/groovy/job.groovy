// Copyright (c) 2024-2025 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Description:
// This groovy job is used by the Seed Workloads Pipeline to define template and parameters for pipeline that executes update-config operation of GCP Cloud Workstations
//
// References:
//

pipelineJob('Cloud-Workstations/Config-Admin-Operations/Update Existing Configuration') {
  description('''
    <br/><h3 style="margin-bottom: 10px;">Update an Existing Configuration of GCP Cloud Workstation</h3>
    <p>This job modifies properties of an existing Cloud Workstation Configuration.</p>

    <h4 style="margin-bottom: 10px;">Immutable Properties</h4>
    <ul>
      <li><code>CLOUD_WS_CONFIG_NAME: </code>Name of the workstation Config.</li>
      <li><code>WS_REPLICA_ZONES: </code>List of Replica Zones for workstations created using a Config.</li>
    </ul>

    <h4 style="margin-bottom: 10px;">Important Notes (CAUTION)</h4>
    <ul>
      <li>This pipeline does NOT retain your original config properties, i.e. whatever you set here, IS what the config will be. (Like create operation, but for an existing config)</li>
      <li>Leave a parameter empty, only if its "Optional" or has a "Default" value - and NOT because you think it will retain its original value - it will NOT.</li>
      <li>Please read each parameter description carefully.</li>
    </ul>

    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>
  ''')

  logRotator {
    daysToKeep(60)
    numToKeep(200)
  }

  parameters {
    stringParam('CLOUD_WS_CONFIG_NAME', '', '<strong>REQUIRED</strong>: Name of the workstation Configuration to update')

    // Timeouts
    separator {
      name('TIMEOUTS')
      sectionHeader('Machine Timeouts (Effective immediately on all workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    stringParam('WS_IDLE_TIMEOUT', '1200', 'Optional: Idle Timeout in seconds.<br>Default: 1200 = 20 mins')
    stringParam('WS_RUNNING_TIMEOUT', '43200', 'Optional: Running Timeout in seconds.<br>Default: 43200 = 12 hrs')

    // Host config
    separator {
      name('HOST_CONFIG')
      sectionHeader('Host Configuration (Effective on new and restarted workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    stringParam('HOST_MACHINE_TYPE', 'e2-standard-4', 'Optional: GCP Compute Engine Machine type for the host VM.<br>Default: <code>e2-standard-4</code>')
    stringParam('HOST_QUICK_START_POOL_SIZE', '0', 'Optional: Pool size of pre-created host VMs (0 means none and low cost).<br>Default: 0')
    stringParam('HOST_BOOT_DISK_SIZE', '30', 'Optional: Boot disk size (GB) for host VM (min: 30GB).<br>Default: 30 (GB)')
    booleanParam('HOST_DISABLE_PUBLIC_IP_ADDRESSES', true, 'Optional: If selected, your workstation will NOT have a public IP.<br>Note: Enabling public IP addresses might be restricted in certain GCP projects by admin.')
    booleanParam('HOST_DISABLE_SSH', true, 'Optional: If selected, your workstation will NOT have SSH enabled.<br>Note: Enabling SSH connections might be restricted in certain GCP projects by admin.')
    booleanParam('HOST_ENABLE_NESTED_VIRTUALIZATION', false, '''
      Optional: If selected, your workstation VMs will have nested virtualization enabled - which is generally needed for running Android emulators.<br>
      Note:<br>
      <ul>
        <li>Nested virtualization can ONLY be enabled on configurations that specify a HOST_MACHINE_TYPE in the N1 or N2 machine series.</li>
        <li>This feature might be restricted in certain GCP projects by admin.</li>
      </ul>
    ''')

    // Persistent Disk (PD) config
    separator {
      name('PD_CONFIG')
      sectionHeader('Persistent Disk Configuration (Effective only on new workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    booleanParam('PD_REQUIRED', false, '''
      Optional: If selected, your new workstations using this config will include a mounted persistent disk (PD) and its details can be filled below.<br>
      Note: Existing workstations will NOT be affected with this update.
    ''')
    stringParam('PD_MOUNT_PATH', '/home', 'Optional: Mount path for persistent disk.<br>Default (if PD_REQUIRED selected): <code>/home</code>')
    stringParam('PD_FS_TYPE', 'ext4', 'Optional: Filesystem type (e.g., ext4, xfs).<br>Default (if PD_REQUIRED selected): <code>ext4</code>')
    choiceParam('PD_DISK_TYPE',
      ['pd-balanced', 'pd-ssd', 'pd-standard', 'pd-extreme'],
      'Persistent Disk type.<br>Default (if PD_REQUIRED selected): <code>pd-balanced</code><br>Note: If PD size is less than 200 GB, disk type must be `pd-balanced` or `pd-ssd`.'
    )
    choiceParam('PD_SIZE_GB',
      ['10', '50', '100', '200', '500', '1000'],
      'Disk size in GB.<br>Default (if PD_REQUIRED selected): 10 (GB)'
    )
    choiceParam('PD_RECLAIM_POLICY',
      ['DELETE', 'RETAIN'],
      'Disk Reclaim policy.<br>Default (if PD_REQUIRED selected): <code>DELETE</code>'
    )
    stringParam('PD_SOURCE_SNAPSHOT', '', '''
      Optional: Source snapshot name<br>
      Note:
      <ul>
        <li>Do NOT prefix with full path, just provide name.</li>
        <li>If PD_SOURCE_SNAPSHOT is set then PD_FS_TYPE or PD_SIZE_GB - CANNOT be specified.</li>
      </ul>
    ''')

    // Ephemeral Disk (ED) config
    separator {
      name('ED_CONFIG')
      sectionHeader('Ephemeral Disk Configuration (Effective on new and restarted workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    booleanParam('ED_REQUIRED', false, '''
      Optional: If selected, your new and restarted workstations will include a temporary ephemeral disk (ED) and its details must be filled below.<br>
      Note: 
      <ul>
        <li>Only either of ED_SOURCE_SNAPSHOT or ED_SOURCE_IMAGE must be specified, but NOT together.</li>
        <li>If ED_SOURCE_SNAPSHOT is set then ED_READ_ONLY must be selected and vice-versa.</li>
      </ul>
    ''')
    stringParam('ED_MOUNT_PATH', '/tmp', 'Optional: Mount path for ephemeral disk.<br>Default (if ED_REQUIRED selected): <code>/tmp</code>')
    choiceParam('ED_DISK_TYPE',
      ['pd-standard', 'pd-ssd', 'pd-balanced', 'pd-extreme'],
      'Temporary Persistent Disk type.<br>Default (if ED_REQUIRED selected): <code>pd-standard</code>'
    )
    stringParam('ED_SOURCE_SNAPSHOT', '', 'REQUIRED (if ED_REQUIRED selected): Source snapshot for ephemeral disk.<br>Note: CANNOT be set together with ED_SOURCE_IMAGE')
    stringParam('ED_SOURCE_IMAGE', '', 'REQUIRED (if ED_REQUIRED selected): Source image for ephemeral disk.<br>Note: CANNOT be set together with ED_SOURCE_SNAPSHOT')
    booleanParam('ED_READ_ONLY', false, 'Optional: If selected, ephemeral disk will be mounted as read-only.<br>CANNOT be UN-selected if ED_SOURCE_SNAPSHOT is set')

    // Container Config
    separator {
      name('CONTAINER_CONFIG')
      sectionHeader('Container Configuration (Effective on new and restarted workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    stringParam('CONTAINER_IMAGE', "${CLOUD_REGION}-docker.pkg.dev/${CLOUD_PROJECT}/${CLOUD_WS_HORIZON_CODE_OSS_IMAGE_NAME}:latest", 'Optional: Container image URI.<br>Default: Full URI of the <code>horizon-code-oss</code> image.')
    stringParam('CONTAINER_ENTRYPOINT_COMMANDS', '', '''
      Optional: Comma separated list of Entrypoint commands for the container.<br>
      Example: <code>"sh", "-c", "echo", "ls -al"</code>
    ''')
    stringParam('CONTAINER_ENTRYPOINT_ARGS', '', 'Optional: Comma separated list of Command arguments for the container.<br>Example: <code>arg1, arg2</code>')
    stringParam('CONTAINER_WORKING_DIR', '', 'Optional: Working directory inside container.')
    textParam('CONTAINER_ENV_VARS', '', '''
      Optional: JSON string objects for container env vars.<br>
      Example: <code>{"ENV1":"val1", "ENV2":"val2"}</code>
    ''')
    stringParam('CONTAINER_USER', '', 'Optional: User to run container as.')

    // Allowed Ports
    separator {
      name('PORTS_CONFIG')
      sectionHeader('Allowed Ports (Effective on new and restarted workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    textParam('WS_ALLOWED_PORTS', '', '''
      Optional: List of port JSON objects, enclosed in square brackets '[ ]'<br>
      Default: <code>[{"first":80, "last":80}, {"first":1024, "last":65535}]</code><br>
      Note: The strings "first" and "last" are keys that must be specified AS IT IS.
    ''')

    // List of Config admins
    separator {
      name('IAM_CONFIG')
      sectionHeader('Workstation Admin IAM Configuration (Effective immediately on all workstations)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }
    stringParam('WS_ADMIN_IAM_MEMBERS', '', '''
      <strong>REQUIRED</strong>: Comma-separated list of new user emails to GRANT "Workstation Admin" privileges.<br>
      <strong>[CAUTION]</strong>: Strict Allow list required - Existing WS Admins, if not specified, will have their access REVOKED for all workstations.<br>
      Example: user1@example.com, user2@example.com
    ''')
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
      scriptPath('workloads/cloud-workstations/pipelines/config-admin-operations/update-config/Jenkinsfile')
    }
  }
}
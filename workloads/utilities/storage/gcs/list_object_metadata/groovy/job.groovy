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

pipelineJob('Utilities/Storage/GCS/Object - List Metadata') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">List Object Metadata</h3>
    <p>This job allows the user to inspect the metadata of objects stored in a GCS bucket.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('URL_PATH')
      defaultValue('')
      description('''<p>path to the desired object - e.g. gs://bucketname/path/objectname)
      <br>or path to folder (can contain any number of wildcard characters) - e.g. gs://bucketname/path/ or gs://bucketname/subpath/*x86*</p>''')
      trim(true)
    }
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
      scriptPath('workloads/utilities/storage/gcs/list_object_metadata/Jenkinsfile')
    }
  }
}

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

pipelineJob('Utilities/Storage/GCS/Filtered Objects - Update Metadata') {
  description("""<br/><h3 style="margin-bottom: 10px;">Update Metadata on Filtered Objects</h3>
    <p>This job allows the user to find all objects in a bucket path which have the specified metadata set
    and update the value of that custom metadata item.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('BUCKET_PATH')
      defaultValue('')
      description('''<p>path to query (ending with / or /*) <br>e.g. gs://bucketname/path/*</p>''')
      trim(true)
    }
    stringParam {
      name('KEY_OR_KEYVALUE_PAIR')
      defaultValue('')
      description('''<p>key or key/value pair that will be used to select objects for update
      <br>e.g. "key" or "key1=1"
      </p>''')
      trim(true)
    }
    stringParam {
      name('UPDATE_VALUE')
      defaultValue('')
      description('''<p>new value to be set for the key defined previously
      <br>e.g. "1" or "release"
      </p>''')
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
      scriptPath('workloads/utilities/storage/gcs/update_metadata_filtered_objects/Jenkinsfile')
    }
  }
}

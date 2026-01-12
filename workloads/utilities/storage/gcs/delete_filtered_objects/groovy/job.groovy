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

pipelineJob('Utilities/Storage/GCS/Filtered Objects - Delete') {
  description("""<br/><h3 style="margin-bottom: 10px;">Filtered Objects - Delete</h3>
    <p>This job allows the user to find all objects in a bucket path which have the specified metadata set
    and delete those objects.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam {
      name('BUCKET_PATH')
      defaultValue('')
      description('''<p>path to query (ending with / or /*) <br>e.g. gs://bucketname/path/*</p>''')
      trim(true)
    }
    choiceParam {
      name('FILTER_TYPE')
      description('''<p>Filter objects with specific metadata, any metadata or no metadata</p>''')
      choices(['Specific Metadata','Any Metadata','No Metadata'])
    }
    stringParam {
      name('KEYVALUE_PAIRS')
      defaultValue('')
      description('''<p>Applicable only if 'Specific Metadata' filter type is selected.
      <br>List keys and/or key/value pairs to filter the output
      <br>(i.e. only objects whose metadata includes the specified keys/values will be selected)
      <br>Note: if left blank, no objects are selected.
      <br>e.g. key1=1 key2=2 key5 key6
      </p>''')
      trim(true)
    }
    booleanParam {
      name('DRY_RUN_ONLY')
      defaultValue(true)
      description('''<p>Untick if you are sure you want to delete the filtered objects. This action is irreversible.</p>''')
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
      scriptPath('workloads/utilities/storage/gcs/delete_filtered_objects/Jenkinsfile')
    }
  }
}

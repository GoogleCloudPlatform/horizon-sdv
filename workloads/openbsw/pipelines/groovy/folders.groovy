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

// Description:
// Groovy file for creating folders in Jenkins for organizing OpenBSW
// pipelines and jobs.
folder('OpenBSW') {
  displayName('OpenBSW Workflows')
  description('<p>This folder contains pipelines and jobs related to environment administration, building, testing, and deploying the Eclipse Foundation OpenBSW applications.<br/>It includes workflows for administration, building and test tasks.</p>')
}
folder('OpenBSW/Builds') {
  displayName('Builds')
  description('<p>This folder contains jobs to build OpenBSW targets.</p>')
}
folder('OpenBSW/Environment') {
  displayName('Environment')
  description('<p>This folder contains environment administrative jobs related to supporting OpenBSW workflows.</p>')
}
folder('OpenBSW/Tests') {
  displayName('Tests')
  description('<p>This folder contains jobs used to help test and validate OpenBSW builds.</p>')
}

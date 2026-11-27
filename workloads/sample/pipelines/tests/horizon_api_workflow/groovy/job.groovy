// Copyright (c) 2026 Accenture, All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
//
//         http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

pipelineJob('Sample/Tests/Horizon API Workflow') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Horizon API · sample workflow</h3>
    <p>Submits <b>WORKFLOW_MODULE</b> / <b>WORKFLOW_TEMPLATE</b> (defaults <code>sample</code> / <code>sample-smoke-test</code>) via the same Horizon REST <code>/v1</code> routes the <code>horizon</code> CLI uses. Captures <code>workflowName</code> after submit, runs wait/logs, then show for <code>archivedLogs</code> / <code>outputArtifacts</code>. No <code>WORKFLOW_STATE_DIR</code>.</p>
    <p><b>HORIZON_WORKFLOW_MODE:</b> <code>Horizon CLI</code> — <code>horizon</code> binary (<code>X-Horizon-Submitted-From: horizon-cli</code>). <code>RestAPI (bash)</code> — <code>horizon-workflow-via-rest.sh</code> (curl+jq). <code>RestAPI (Python)</code> — <code>horizon-workflow-via-rest.py</code> (stdlib). Both REST paths use <code>X-Horizon-Submitted-From: rest-api</code> (recorded as <code>api</code>), <code>GET /v1/catalog</code> merge, and the same wait/logs/show flow.</p>
    <p>The agent image must include the <code>horizon</code> binary and <code>curl</code>/<code>jq</code>/<code>python3</code> (build <b>Sample/Environment/Docker Image Template</b> from repo root context). Override API URL via <code>HORIZON_DOMAIN</code> / <code>HORIZON_API_BASE_URL</code> or optional <code>~/.config/horizon/config.yaml</code> (<code>horizon config init</code>). Image tag: <code>SAMPLE_IMAGE_TAG</code>.</p>
    <p>Authentication: <code>KEYCLOAK_CLIENT_SECRET</code> from Jenkins credentials (default credential id <code>jenkins-horizon-api-ci-secret</code>), or <code>KEYCLOAK_CREDENTIALS_ID=-</code> with <code>HORIZON_ACCESS_TOKEN</code>. Optional <code>KEYCLOAK_CLIENT_ID</code> env.</p>
    <p>Catalog module and JSON examples for parameters are documented on the <b>WORKFLOW_MODULE</b> and <b>WORKFLOW_PARAMETERS_JSON</b> build fields.</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  parameters {
    stringParam('WORKFLOW_MODULE', 'sample', '''
      <p>Horizon catalog <b>module</b> passed to <code>horizon workflow submit --module</code> and REST submit paths (<code>WORKFLOW_MODULE</code> env). Must match the module that exposes <b>WORKFLOW_TEMPLATE</b> (<code>GET /v1/catalog</code>).</p>
      <p>Typical values: <code>sample</code> (default) for <code>sample-smoke-test</code>; <code>workloads-android</code> for <code>aaos-builder</code>. When this field is empty or whitespace, the pipeline uses folder/global env <code>WORKFLOW_MODULE</code> if set, otherwise <code>sample</code>.</p>''')

    choiceParam('HORIZON_WORKFLOW_MODE', ['horizon-cli', 'rest-bash', 'rest-python'], '''
      <p><b>Horizon CLI</b> — <code>horizon workflow …</code> (default). <b>RestAPI (bash)</b> — <code>horizon-workflow-via-rest.sh</code>. <b>RestAPI (Python)</b> — <code>horizon-workflow-via-rest.py</code> (Python 3 stdlib only).</p>''')

    stringParam('WORKFLOW_TEMPLATE', 'sample-smoke-test', '''
      <p>Workflow template name from <code>GET /v1/catalog</code> for the selected <b>WORKFLOW_MODULE</b> — must match <code>templateName</code> exactly.</p>
      <p>Examples: <code>sample-smoke-test</code> (module <code>sample</code>); <code>aaos-builder</code> (module <code>workloads-android</code>).</p>''')

    textParam('WORKFLOW_PARAMETERS_JSON', '{"horizonSubmittedFrom":"","sampleEnv":"jenkins","sampleBuildId":"","sampleNote":""}', '''
      <p>JSON <b>object</b> for submit <code>parameters</code>. Keys must match the catalog entry for <code>WORKFLOW_TEMPLATE</code> (CLI / REST merge with defaults).</p>
      <p><b>Copy-paste one-liners</b> (same compact style; all values are strings for the REST submit body). First key <code>horizonSubmittedFrom</code> is optional (omit or leave empty to use HTTP header / API default); when set it is stored on the workflow and passed into the Argo template.</p>
      <ul style="margin-top:4px;line-height:1.55;">
        <li><code>sample-smoke-test</code>:<br/><code style="white-space:pre-wrap;word-break:break-all;">{"horizonSubmittedFrom":"","sampleEnv":"jenkins","sampleBuildId":"","sampleNote":""}</code></li>
        <li><code>aaos-builder</code> (chart defaults; set <b>WORKFLOW_MODULE</b> to <code>workloads-android</code>; override <code>pipelineRepoUrl</code> / <code>pipelineRepoRevision</code> for your repo):<br/><code style="white-space:pre-wrap;word-break:break-all;">{"horizonSubmittedFrom":"","lunchTarget":"aosp_cf_x86_64_auto-cp2a-userdebug","androidRevision":"android-17.0.0_r1","manifestUrl":"https://android.googlesource.com/platform/manifest","cleanBuild":"NO_CLEAN","buildCtsOnly":"false","forceImageBuild":"false","storageLabels":"env=dev","enableGeminiAiAssistant":"true","parallelSyncJobs":"3","pipelineRepoUrl":"","pipelineRepoRevision":""}</code></li>
      </ul>
      <p>Built-in values for <code>horizonSubmittedFrom</code> include <code>api</code>, <code>developer-portal</code>, <code>horizon-cli</code>; REST clients default to <code>api</code> unless you override via this field or HTTP headers <code>X-Horizon-Submitted-From</code> / <code>X-Horizon-Submitted-From-Detail</code> (see Horizon API docs).</p>''')
  }

  logRotator {
    daysToKeep(14)
    numToKeep(100)
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
      scriptPath('workloads/sample/pipelines/tests/horizon_api_workflow/Jenkinsfile')
    }
  }
}

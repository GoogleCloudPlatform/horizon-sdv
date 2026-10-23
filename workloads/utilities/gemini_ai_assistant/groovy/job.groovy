// Copyright (c) 2026 Accenture, All Rights Reserved.
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

def model = ('${GEMINI_MODEL}' && '${GEMINI_MODEL}' != 'null' && !'${GEMINI_MODEL}'.contains('${')) ? "--model ${'${GEMINI_MODEL}'} " : ""

pipelineJob('Utilities/Gemini AI Assistant') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">Gemini AI Assistant Utility</h3>
    <p>This Jenkins job allows users to execute the Gemini AI assistant CLI tool against selected artifacts that will be
       downloaded to the workspace. Users can provide a custom prompt to guide the AI's analysis, such as build and test
       failure analysis.<br/><br/>
    <b>Parameters:</b></br/>
    <ul><li><b>GEMINI_PROMPT_FILE:</b> A text prompt file (step 1) to send to the Gemini AI model (upload).</li>
        <li><b>GEMINI_PROMPT_FILE_2, GEMINI_PROMPT_FILE_3:</b> Optional step 2 and step 3 prompt files (upload) for sequenced analysis (order matters).</li>
        <li><b>GEMINI_ARTIFACTS_COMMAND:</b> A command to run to download the artifacts to be analysed.</li></ul>
    <b>Usage:</b><br/>
    <ul><li>Specify the command to retrieve the artifacts to process.</li>
        <li>Select your prompt file (step 1). Optionally add step 2 and/or step 3 for sequenced analysis (order matters).</li>
        <li>Run the job to execute the Gemini CLI command.</li>
        <li>Review the generated artifacts (gemini-assist/*, step*_output.md) stored with the job.</li></ul></p>
    <p><b>Offline analysis &amp; prompt/skill iteration:</b> use this job against archived artifacts from any pipeline — no re-run of the original job needed.</p>
    <ul>
      <li><b>Build, CVD or CTS log analysis:</b> point <code>GEMINI_ARTIFACTS_COMMAND</code> at the archived workspace or <code>test-results/</code> for the run you want to investigate. Works for passes <i>and</i> failures.</li>
      <li><b>Pick the matching sequenced prompts</b> from the relevant pipeline's <code>prompt/sequenced/</code> directory (build / CVD Launcher / CTS Execution) and upload them as <code>GEMINI_PROMPT_FILE</code>, <code>_2</code>, <code>_3</code> with their <code>skills.yaml</code> (<code>GEMINI_SKILLS_YAML</code>).</li>
      <li><b>For Cuttlefish / virtual-device focus,</b> always use the <b>CVD Launcher</b> prompts — the Phase 0 boot preflight auto-routes between boot-failure and runtime-health lanes, so the same prompts cover both successful and failed boots (including CVD logs captured by CTS Execution).</li>
      <li><b>Prompt &amp; skill development:</b> ideal sandbox for iterating on prompts or <code>skills.yaml</code> against real artifacts before promoting changes back into the pipeline.</li>
    </ul>
        <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")


  environmentVariables {
    env('GEMINI_PREVIEW_FEATURES', ${GEMINI_PREVIEW_FEATURES})
    env('GEMINI_LOCATION_GLOBAL', ${GEMINI_LOCATION_GLOBAL})
    env('GEMINI_MODEL', '${GEMINI_MODEL}')
  }

  parameters {
    separator {
      name('Agentic AI: Configuration (Experimental)')
      sectionHeader('Agentic AI: Configuration (Experimental)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    base64File {
      name('GEMINI_PROMPT_FILE')
      description('''<p>Required for gemini-cli: Select a prompt file for the Gemini AI assistant (step 1). Prompts are provided only via upload.</p>''')
    }

    base64File {
      name('GEMINI_PROMPT_FILE_2')
      description('''<p>Optional: Step 2 prompt file (upload). For sequenced analysis; when set with step 1, analysis runs in two or three steps (order matters). We do not ship single-prompt files—use step 2 (and optionally step 3) for sequenced runs.</p>''')
    }

    base64File {
      name('GEMINI_PROMPT_FILE_3')
      description('''<p>Optional: Step 3 prompt file (upload). For sequenced analysis; when set with step 1 (and optionally step 2), analysis runs in three steps (order matters).</p>''')
    }

    base64File {
      name('GEMINI_SKILLS_YAML')
      description('''<p>Optional: Upload <code>skills.yaml</code> (same format as repo). When provided, the job decodes it and converts to <code>.gemini/skills/*/SKILL.md</code>. Use when prompts are uploaded so the job cannot auto-detect the prompt dir.</p>''')
    }

    stringParam {
      name('GEMINI_ARTIFACTS_COMMAND')
      defaultValue('')
      description("""<p>Mandatory: Command to copy content to be used for analysis, e.g.<br/>
        <ul><li><code>gcloud storage cp -r gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Tests/CTS_Execution/&lt;BUILD_NUMBER&gt;/test-results/ .</code></li>
            <li><code>gcloud storage cp gs://${ANDROID_BUILD_BUCKET_ROOT_NAME}/Android/Builds/AAOS_Builder/&lt;BUILD_NUMBER&gt;/aaos* .</code></li></ul></p>""")
      trim(true)
    }

    choiceParam {
      name('AI_REVIEW_RUNTIME')
      description('''<p>AI Review runtime: <code>agy</code> (Antigravity SDK, default) or <code>gemini-cli</code> (CLI).</p>''')
      choices(['agy', 'gemini-cli'])
    }

    stringParam {
      name('GEMINI_COMMAND_LINE')
      defaultValue("gemini ${model} --yolo --output-format json")
      description('''<p>Interface for the headless <a href="https://geminicli.com/docs/cli/headless/" target="_blank">gemini-cli</a>.</p>
        <p>Use this to specify settings such as the <a href="https://ai.google.dev/gemini-api/docs/models" target="_blank">Gemini model</a>.</p>
        <p><b>Note:</b> Prompts are piped via <b>stdin</b> and output is redirected to a JSON file.</p>''')
      trim(true)
    }

    choiceParam {
      name('GEMINI_AI_EXECUTION_TIMEOUT_MINUTES')
      description('''
        <p>Maximum duration in minutes allowed for the Gemini assistant to run before automatic termination.</p>
        <p><i>Note: This safety limit prevents hung processes and optimizes resource usage.</i></p>''')
      choices(['60', '45', '30', '20', '10', '90', '120'])
    }

  }

  logRotator {
    artifactDaysToKeep(60)
    artifactNumToKeep(100)
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
      scriptPath('workloads/utilities/gemini_ai_assistant/Jenkinsfile')
    }
  }
}


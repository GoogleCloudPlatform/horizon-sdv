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

// Description:
// Groovy file for defining a Jenkins Pipeline Job for building the Eclipse
// Foundation OpenBSW project.

def model = ('${GEMINI_MODEL}' && '${GEMINI_MODEL}' != 'null' && !'${GEMINI_MODEL}'.contains('${')) ? "--model ${'${GEMINI_MODEL}'} " : ""

pipelineJob('OpenBSW/Builds/BSW Builder') {
  description("""
    <br/><h3 style="margin-bottom: 10px;">OpenBSW Build Job</h3>
    <p>This job is used to build the <a href="https://github.com/eclipse-openbsw/openbsw/tree/main" target="_blank">Eclipse Foundation OpenBSW.</a>.</p>
    <h4 style="margin-bottom: 10px;">Reference documentation:</h4>
    <ul>
      <li><a href="https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/index.html" target="_blank">Welcome to Eclipse OpenBSW.</a></li>
      <li><a href="https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/learning/unit_tests/index.html" target="_blank">Building and Running Unit Tests.</a></li>
      <li><a href="https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/learning/setup/setup_posix_build.html#setup-posix-build" target="_blank">POSIX Platform.</a></li>
      <li><a href="https://eclipse-openbsw.github.io/openbsw/sphinx_docs/doc/dev/learning/setup/setup_s32k148_ubuntu_build.html" target="_blank">S32K148 Platform.</a></li>
    </ul>
    <p>For <b>Rust</b> reference builds (<code>RTOS_PLATFORM=rust</code>, presets <code>posix-rust</code> / <code>s32k148-rust-gcc</code>), follow the numbered guide <b>Rust reference build (step by step)</b> in <code>workloads/openbsw/pipelines/builds/bsw_builder/README.md</code> (also under docs/workloads/openbsw/builds).</p>
    <br/><div style="border-top: 1px solid #ccc; width: 100%;"></div><br/>""")

  environmentVariables {
    env('GEMINI_PREVIEW_FEATURES', ${GEMINI_PREVIEW_FEATURES})
    env('GEMINI_LOCATION_GLOBAL', ${GEMINI_LOCATION_GLOBAL})
    env('GEMINI_MODEL', '${GEMINI_MODEL}')
  }

  parameters {
    separator {
      name('Common Parameters')
      sectionHeader('Common Parameters')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('OPENBSW_GIT_URL')
      defaultValue("${OPENBSW_GIT_URL}")
      description('''<p>OpenBSW Git URL.</p>''')
      trim(true)
    }

    stringParam {
      name('OPENBSW_GIT_BRANCH')
      defaultValue("${OPENBSW_GIT_BRANCH}")
      description('''<p>OpenBSW revision tag/branch name.</p>''')
      trim(true)
    }

    stringParam {
      name('POST_GIT_CLONE_COMMAND')
      defaultValue('cd openbsw && git checkout 1c1450e && cd -')
      description('''<p>Optional additional commands post git clone and prior to build/make.<br/>
        <b>Note: </b>Single command line only, use logical operators to execute subsequent commands.<br/></p>''')
      trim(true)
    }

    choiceParam {
      name('RTOS_PLATFORM')
      choices(['freertos', 'threadx', 'rust'])
      description('''<p>Select the <b>RTOS Kernel</b> implementation (or Rust-enabled reference build). <br/>
        <code>rust</code>: POSIX uses CMake preset <code>posix-rust</code> (FreeRTOS + Rust). S32K148 uses <code>s32k148-rust-gcc</code> only (GCC; Clang is ignored).<br/>
        Note: Ensure the selected toolchain supports the specific kernel port.</p>''')
    }

    choiceParam {
      name('BUILD_CONFIG')
      choices(['debug', 'release'])
      description('''<p>Select the compilation profile.</p>''')
    }

    choiceParam {
      name('TOOLCHAIN')
      choices(['gcc', 'clang'])
      description('''<p>Select the <b>Compiler Toolchain</b> for the build. GCC is standard; Clang provides enhanced static analysis.</p>''')
    }

    stringParam {
      name('IMAGE_TAG')
      defaultValue("${OPENBSW_IMAGE_TAG}")
      description('''<p>Docker image template to use.<p>
        <p>Note: tag may only contain 'abcdefghijklmnopqrstuvwxyz0123456789_-./'</p>''')
      trim(true)
    }

    stringParam {
      name('CMAKE_SYNC_JOBS')
      defaultValue('7')
      description('''<p>Number of parallel sync jobs for <i>cmake</i>.<br/>
        If undefined, defaults to -j.</p>''')
      trim(true)
    }

    separator {
      name('Documentation')
      sectionHeader('Documentation')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('BUILD_DOCUMENTATION')
      defaultValue(false)
      description('''<p>Create OpenBSW doxygen documentation and coverage report.</p>''')
    }

    separator {
      name('Unit Tests')
      sectionHeader('Unit Tests')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('CODE_COVERAGE')
      defaultValue(false)
      description('''<p>Enable code coverage metrics. BUILD_UNIT_TESTS and RUN_UNIT_TESTS must been enabled.</p>''')
    }

    booleanParam {
      name('LIST_UNIT_TESTS')
      defaultValue(false)
      description('''<p>List of available Unit Tests.</p>''')
    }

    stringParam {
      name('LIST_UNIT_TESTS_CMDLINE')
      defaultValue('cmake --preset tests-posix-${BUILD_CONFIG} && cmake --build --preset tests-posix-${BUILD_CONFIG} --target help -j${CMAKE_SYNC_JOBS}')
      description('''<p>Default List Unit Test build command line.<br/><br/>
      <b>Options:</b> <ul><li><code>tests-posix-debug</code></li><li><code>tests-posix-release</code></li>
                          <li><code>tests-s32k1xx-debug</code></li><li><code>tests-s32k1xx-release</code></li></ul></p>''')
      trim(true)
    }

    booleanParam {
      name('BUILD_UNIT_TESTS')
      defaultValue(true)
      description('''<p>Build Unit Tests.</p>''')
    }

    stringParam {
      name('UNIT_TEST_TARGET')
      defaultValue('all')
      description('''<p>Build specific Unit Test target, or all tests.</p>''')
      trim(true)
    }

    stringParam {
      name('UNIT_TESTS_CMDLINE')
      defaultValue('cmake --preset tests-posix-${BUILD_CONFIG} && cmake --build --preset tests-posix-${BUILD_CONFIG} --target ${UNIT_TEST_TARGET} -j${CMAKE_SYNC_JOBS}')
      description('''<p>Default Unit Test build command line.<br/><br/>
      <b>Options:</b> <ul><li><code>tests-posix-debug</code></li><li><code>tests-posix-release</code></li>
                          <li><code>tests-s32k1xx-debug</code></li><li><code>tests-s32k1xx-release</code></li></ul></p>''')
      trim(true)
    }

    booleanParam {
      name('RUN_UNIT_TESTS')
      defaultValue(true)
      description('''<p>Run Unit Tests.</p>''')
    }

    stringParam {
      name('RUN_UNIT_TESTS_CMDLINE')
      defaultValue('ctest --preset tests-posix-${BUILD_CONFIG} --parallel ${CMAKE_SYNC_JOBS}')
      description('''<p>Default Unit Test execution command line.</p>
      <p>If running a single unit test, ensure use of <code>--test-dir</code> and point to the correct test path, e.g.  bspTest, or /bspIoTest:<br/>
      <ul><li><code>ctest --test-dir build/tests/posix/Debug/libs/bsw/bsp/test --parallel ${CMAKE_SYNC_JOBS}</code></li>
          <li><code>ctest --test-dir build/tests/posix/Release/libs/bsw/bsp/test --parallel ${CMAKE_SYNC_JOBS}</code></li>
          <li><code>ctest --test-dir build/tests/s32k1xx/Debug/platforms/s32k1xx/bsp/bspIo/test --parallel ${CMAKE_SYNC_JOBS}</code></li>
          <li><code>ctest --test-dir build/tests/s32k1xx/Release/platforms/s32k1xx/bsp/bspIo/test --parallel ${CMAKE_SYNC_JOBS}</code></li></ul><br/>
      <b>Options:</b> <ul><li><code>tests-posix-debug</code></li><li><code>tests-posix-release</code></li>
                          <li><code>tests-s32k1xx-debug</code></li><li><code>tests-s32k1xx-release</code></li></ul></p>''')
      trim(true)
    }

    separator {
      name('POSIX Target')
      sectionHeader('POSIX Target')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('BUILD_POSIX')
      defaultValue(true)
      description('''<p>Build POSIX Target application.<br/>
      This will upload the reference application, to artifact registry for use by the OpenBSW POSIX test job.</p>''')
    }

    stringParam {
      name('POSIX_BUILD_CMDLINE')
      defaultValue('cmake --preset posix-${RTOS_PLATFORM} && cmake --build --preset posix-${RTOS_PLATFORM} -j${CMAKE_SYNC_JOBS}')
      description('''<p>Default POSIX build command line<br/><br/>
      <b>Options:</b><ul><li><code>posix-freertos</code></li><li><code>posix-threadx</code></li><li><code>posix-rust</code> (set RTOS_PLATFORM to <code>rust</code>)</li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('POSIX_ARTIFACT')
      defaultValue('build/posix-${RTOS_PLATFORM}/executables/referenceApp/application/Release/app.referenceApp.elf')
      description('''<p>Default POSIX artifact.<br/><br/>
      <b>Options:</b><ul><li><code>build/posix-freertos/...</code></li><li><code>build/posix-threadx/...</code></li><li><code>build/posix-rust/...</code></li></ul></p>''')
      trim(true)
    }

    separator {
      name('POSIX pyTest')
      sectionHeader('POSIX pyTest')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('POSIX_PYTEST')
      defaultValue(false)
      description('''<p>Run pyTest on POSIX target application. Only applicable when <code>BUILD_POSIX</code> is selected.</p>''')
    }

    stringParam {
      name('POSIX_PYTEST_CMDLINE')
      defaultValue('./tools/enet/bring-up-ethernet.sh && ./tools/can/bring-up-vcan0.sh && cd test/pyTest/ && pytest --target=posix --app=${RTOS_PLATFORM}')
      description('''<p>Default POSIX pyTest command line<br/><br/>
      <b>Options:</b><ul><li><code>--app=freertos</code></li><li><code>--app=threadx</code></li><li><code>--app=rust</code> when <code>RTOS_PLATFORM=rust</code></li></ul>
      <b>Rust + POSIX tests:</b> OpenBSW’s test file does not yet list Rust; when <code>RTOS_PLATFORM=rust</code>, this job pastes in the missing Rust entry so automated tests know which executable to start. That happens after the POSIX build and again right before pyTest (Jenkins runs pyTest in its own stage). Technical detail: <code>target_posix_rust.fragment.toml</code> → <code>test/pyTest/target_posix.toml</code> as <code>[rust.target_process]</code>.</p>''')
      trim(true)
    }

    separator {
      name('NXP Hardware Target')
      sectionHeader('NXP Hardware Target')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('BUILD_NXP_S32K148')
      defaultValue(true)
      description('''<p>Build NXP S32K148 Target.</p>''')
    }

    stringParam {
      name('NXP_S32K148_BUILD_CMDLINE')
      defaultValue('cmake --preset s32k148-${RTOS_PLATFORM}-${TOOLCHAIN} && cmake --build --preset s32k148-${RTOS_PLATFORM}-${TOOLCHAIN} -j${CMAKE_SYNC_JOBS}')
      description('''<p>Default NXP S32K148 build command line.<br/><br/>
      <b>Options:</b><ul><li><code>s32k148-freertos-gcc</code></li><li><code>s32k148-threadx-gcc</code></li>
                         <li><code>s32k148-freertos-clang</code></li><li><code>s32k148-threadx-clang</code></li>
                         <li><code>s32k148-rust-gcc</code> when <code>RTOS_PLATFORM</code> is <code>rust</code> (preset fixed; not <code>s32k148-rust-clang</code>)</li></ul><br/>
      <b>Note:</b></p>''')
      trim(true)
    }

    stringParam {
      name('NXP_S32K148_ARTIFACT')
      defaultValue('build/s32k148-${RTOS_PLATFORM}-${TOOLCHAIN}/executables/referenceApp/application/RelWithDebInfo/app.referenceApp.elf')
      description('''<p>Default NXP S32K148 artifact.<br/><br/>
      <b>Options:</b><ul><li><code>build/s32k148-freertos-gcc/...</code></li><li><code>build/s32k148-threadx-gcc...</code></li>
                         <li><code>build/s32k148-freertos-clang/...</code></li><li><code>build/s32k148-threadx-clang/...</code></li>
                         <li><code>build/s32k148-rust-gcc/...</code></li></ul></p>''')
      trim(true)
    }

    separator {
      name('Miscellaneous Options')
      sectionHeader('Miscellaneous Options')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    choiceParam {
      name('INSTANCE_RETENTION_TIME')
      description('''<p>Time in minutes to retain the instance after build completion.<br/>
        Useful for debugging build issues, reviewing target outputs etc.</p>''')
      choices(['0', '15', '30', '45', '60', '120', '180'])
    }

    separator {
      name('Agentic AI: Configuration (Experimental)')
      sectionHeader('Agentic AI: Configuration (Experimental)')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    booleanParam {
      name('ENABLE_GEMINI_AI_ASSISTANT')
      defaultValue(true)
      description('''<p>Enable Gemini AI assistant to provide potential solutions to any build failures.</p>''')
    }

    choiceParam {
      name('AI_REVIEW_RUNTIME')
      description('''<p>AI Review runtime: <code>agy</code> (Antigravity SDK, default) or <code>gemini-cli</code> (CLI).</p>''')
      choices(['agy', 'gemini-cli'])
    }

    choiceParam {
      name('GEMINI_AI_EXECUTION_TIMEOUT_MINUTES')
      description('''
        <p>Maximum duration in minutes allowed for the Gemini assistant to run before automatic termination.</p>
        <p><i>Note: This safety limit prevents hung processes and optimizes resource usage.</i></p>''')
      choices(['60', '45', '30', '20', '10', '90', '120'])
    }

    stringParam {
      name('GEMINI_COMMAND_LINE')
      defaultValue("gemini ${model} --yolo --output-format json")
      description('''<p>The headless <a href="https://geminicli.com/docs/cli/headless/" target="_blank">gemini cli</a>.</p>
        <p>Use this to define such things as <a href="https://ai.google.dev/gemini-api/docs/models" target="_blank">Gemini model</a> to use.</p>
        <p><b>Note:</b> We use <b>stdin</b> to pipe the prompt to gemini-cli and redirect output to json file.</p>''')
      trim(true)
    }

    separator {
      name('Storage Options')
      sectionHeader('Storage Options')
      sectionHeaderStyle("${HEADER_STYLE}")
      separatorStyle("${SEPARATOR_STYLE}")
    }

    stringParam {
      name('OPENBSW_ARTIFACT_STORAGE_SOLUTION')
      defaultValue('GCS_BUCKET')
      description('''<p>OpenBSW Artifact Storage:<br/>
        <ul><li>GCS_BUCKET will store to cloud bucket storage</li>
        <li>Empty will result in nothing stored</li></ul></p>''')
      trim(true)
    }

    stringParam {
      name('STORAGE_BUCKET_DESTINATION')
      defaultValue('')
      description('''<p>OpenBSW Bucket Storage destination:<br/>
        Leave empty for build to create default, e.g. gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Builds/BSW_Builder/<BUILD_NUMBER><br/>
        Alternatively, override path, e.g gs://${OPENBSW_BUILD_BUCKET_ROOT_NAME}/OpenBSW/Releases/010129</p>''')
      trim(true)
    }

    stringParam {
      name('STORAGE_LABELS')
      defaultValue('')
      description('''<p>Optional, list one or more labels to be applied to the artifacts being uploaded to storage.
      <br>Use spaces or commas to seperate. Neither keys nor values should contain spaces. (e.g. Release=X.Y.Z,Workload=OpenBSW)</p>''')
      trim(true)
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
      scriptPath('workloads/openbsw/pipelines/builds/bsw_builder/Jenkinsfile')
    }
  }
}

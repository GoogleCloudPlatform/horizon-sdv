<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# **How to Contribute**

We would love to accept your patches and contributions to the Horizon SDV project.

## **Before you begin**

### **Sign our Contributor License Agreement**

Contributions to this project must be accompanied by a [Contributor License Agreement](https://cla.developers.google.com/about) (CLA). You (or your employer) retain the copyright to your contribution; this simply gives us permission to use and redistribute your contributions as part of the project.

If you or your current employer have already signed the Google CLA (even if it was for a different project), you probably don’t need to do it again.

Visit [https://cla.developers.google.com/](https://cla.developers.google.com/) to see your current agreements or to sign a new one.

### **Review our Community Guidelines**

This project follows [Google’s Open Source Community Guidelines](https://opensource.google/conduct/).

## **Prerequisites: Self-Deployment**

Before submitting any code, documentation, or configuration changes, you must have a functional, self-deployed instance of Horizon SDV running on your own Google Cloud Platform (GCP) project.

This requirement ensures that contributors have first-hand experience with the deployment workflow, can test their changes in a live environment, and verify that their contributions do not break existing platform functionality.

* **Follow the Deployment Guide:** Refer to the [Deployment Guide](deployment_guide.md) to set up your own environment.
* **Verify your Instance:** Ensure your deployment is healthy and that you can access the core services before beginning your development work.
* **Test on your Instance:** Use your deployed instance to test your changes. We generally require proof that your changes have been successfully applied and verified in a live environment. For a change to a single module, you can pin that module to your branch from **Administration → Modules** in the Developer Portal; see [Change git ref in developer portal](architecture/05_creating-modules.md#change-git-ref-in-developer-portal).

## **Branches and Pull Requests**

Horizon SDV uses a single permanent branch, `main`. All changes, however small, are developed on a branch and merged to `main` through a reviewed Pull Request. The full process is described in the [Git Branching Strategy](branching_strategy.md); the rules that apply to external contributors are summarised here.

* **Fork:** Unless you have write access to the Horizon SDV repository, [fork it](https://docs.github.com/pull-requests/collaborating-with-pull-requests/working-with-forks/fork-a-repo) and work on a branch in your fork.
* **Target branch:** Create your branch from an up-to-date `main` and raise your Pull Request against `main` of the Horizon SDV repository.
* **Branch name:** Use `contrib/feature/<title>` for features and `contrib/fix/<title>` for fixes, for example `contrib/fix/BuildManifestError`. For a change with multiple parts, separate the part with `__`, for example `contrib/feature/CustomBuildManifest__ParamCreation`. Use these names in your fork too; the Horizon SDV repository rejects any other branch name.
* **Trivial changes:** Typo and documentation fixes follow the same process, using a `contrib/fix/<title>` branch.
* **Signed commits:** All commits must be signed.
* **Rebase:** Rebase your branch against `main` before final testing and again if `main` moves on before approval, so that what you tested is exactly what will land on `main`.
* **Draft first:** Open the Pull Request as a draft early, and mark it *Ready for review* once testing is complete.

### **Pull Request contents**

The Pull Request title and description become the commit message on `main` (changes are squash merged), so complete the [Pull Request template](../.github/pull_request_template.md) carefully:

* **Title:** names the feature or fix.
* **Purpose and Concise Description:** why the change is needed and what it does.
* **Dependencies:** any hard or soft dependencies, or N/A.
* **Open Source Component Changes:** any open source components added, updated or modified, with the reason, or N/A.
* **Suggested Release Note:** wording for the release notes.
* **Testing:** logs, screenshots, or testing steps performed on your self-deployed instance, posted as a PR comment. Alternatively, a virtual meet can be scheduled to walk through your changes.

### **Review and release cycle**

* **Reviewers:** Each Pull Request is reviewed by at least one Code Owner, one Release Manager, and one reviewer who checks the functioning of the change.
* **Target release:** Our maintainers will determine the appropriate release version for your changes and communicate it in the PR thread.
* **Release freeze:** While a release branch (`rel/<X>.<Y>.<Z>`) is open, new changes are not merged to `main`. A Pull Request raised during a release may wait until the release is complete and need another rebase before approval.

### **Copyright headers**

Every new source file needs a copyright header naming you or your employer, followed by the Apache 2.0 licence notice. Match the comment style of the file type, for example:

```yaml
# Copyright (c) 2026 <Your Name or Company>, All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#         http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
```

## **Contribution Checklist**

When submitting a Pull Request, confirm the following:

* [ ] I have deployed Horizon SDV on my own GCP project.
* [ ] I have tested my changes on my deployed instance, after rebasing on `main`.
* [ ] My branch is named `contrib/feature/<title>` or `contrib/fix/<title>` and targets `main`.
* [ ] All my commits are signed.
* [ ] I have completed the Pull Request template, and posted testing evidence as a comment.
* [ ] New files carry a copyright header and the Apache 2.0 licence notice.
* [ ] I understand that my PR will be reviewed and assigned to a specific Horizon SDV release cycle.

## **Getting Help**

If you encounter issues during your initial deployment or have questions about the development workflow, please:

* Review the [Troubleshooting](deployment_guide.md#section-6---troubleshooting) section in the documentation.
* Reach out to the team at [horizon-sdv@google.com](mailto:horizon-sdv@google.com) for consultation regarding setup or specific technical blockers.

# **Partner Contributions**

To further enhance Horizon project capabilities and foster a collaborative ecosystem, partners can contribute external tools, components, and integrations to the Horizon SDV repository. Partner contributions follow the same branch and Pull Request process as any other contribution.

## **Where contributions go**

* **Modules:** New capabilities are contributed as Horizon modules. A module is a Helm chart under `gitops/modules/<name>/`, registered in the `ModuleCatalog` (`gitops/apps/module-manager/templates/module-catalog.yaml`), that can be enabled and disabled from **Administration → Modules** without affecting the rest of the platform. Follow [How to create a new module](architecture/05_creating-modules.md), including its checklist. For the concepts behind modules, see the [Horizon modules overview](architecture/01_overview.md).
* **Workload pipelines:** Workflow definitions, scripts, and images that a module deploys live under `workloads/<area>/`, for example `workloads/android/pipelines`.
* **Third-party licences:** The `third_party/` directory is reserved for dependencies that cannot be licensed under Apache 2.0. Put their licence terms, and any trial licence files needed before deployment (for example `license.json`), in `third_party/<vendor>/<component>/`.

## **Key Integration Principles**

### **Clear and Comprehensive Documentation**

Every contribution must include thorough and concise README.md files within its respective subfolder. Documentation should detail the component’s purpose, prerequisites, usage instructions, and clearly identify its integration points within the broader Horizon SDV framework. A module also provides an overview page, shown on its tab in the Developer Portal.

### **Modularity and Self-Containment**

Contributions should be designed as modular, self-contained units that can be enabled, disabled, and updated without breaking core functionalities of Horizon core. Declare dependencies on other modules in the `ModuleCatalog` rather than assuming they are present, and expose only the workflow templates and applications intended for users.

### **Prioritize Configuration over Custom Code**

Whenever feasible, contributions should leverage declarative configuration rather than requiring extensive, custom code changes.

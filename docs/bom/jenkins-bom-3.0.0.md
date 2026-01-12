## Jenkins BOM (3.0.0)

| BOM Item | Version (latest) |
|---------|------------------|
| [HELM](https://github.com/jenkinsci/helm-charts/releases) | `5.8.31` → `5.8.114` |
| [Jenkins](https://www.jenkins.io/changelog-stable/) | `jenkins:2.528.3-jdk21` |

Jenkins plugin versions are defined in
[`values-jenkins.yaml`](../../gitops/env/stage2/workloads/values-jenkins.yaml),
under the `installPlugins` and `additionalPlugins` sections.

### Disable Warnings:

Jenkins update monitor is set to disable warning of Jenkins and plugins upgrades being available

**System → Administrative monitors:**

**Jenkins Update Notification:**  

When a new release of Jenkins has been released, this informs administrators about it.

```
disabledAdministrativeMonitors:
- "hudson.model.UpdateCenter$CoreUpdateMonitor"
```

### Update Site WarningsSecurity

This warning informs administrators about active security warnings for installed Jenkins core or plugin versions. Rather than disable this warning entirely (including any future issues), consider disabling specific messages in the global security configuration.

```
disabledAdministrativeMonitors:
- "jenkins.security.UpdateSiteWarningsMonitor"
- "hudson.util.DoubleLaunchChecker"
```

Additional warning removals:

#### Built-in Node With Agents Executors Configured MonitorSecurity

Shows a warning when the built-in node has executors despite agents (static or clouds) being configured.

```
disabledAdministrativeMonitors:
- "jenkins.diagnostics.ControllerExecutorsAgents"
```
Removing because Seed job needs to use the default built-in-node to seed the workloads so the k8s pods/container images can be created. Chicken and egg issue.

#### Role-Based Naming Strategy not enabled

Shows a warning when the Role-based Authorization strategy is enabled but the Role-based project naming strategy is not used.

```
disabledAdministrativeMonitors:
- "org.jenkinsci.plugins.rolestrategy.NamingStrategyAdministrativeMonitor"
```
Disabling currently because RBAC and Role Base Auth strategy is only provided as an example.

We can temporarily enable in system when we wish to reinstate, for now disable in Jenkins YAML. The latter 2 may warrant additional review but for now, disable.

---

### R2.0.0: Jenkins Update  

Reference: TAA-900  

| BOM Item | Version (latest) |
|----------|------------------|
| [HELM ](https://github.com/jenkinsci/helm-charts/releases)| `5.8.2` → `5.8.31` <br> `5.8.31 == jenkins:2.492.3-jdk17` |
| [Jenkins](https://www.jenkins.io/changelog-stable/) | `jenkins:2.492.3-jdk17` |

---

### R2.0.1: Jenkins Update  

Reference: TAA-1002

| BOM Item       | Version (latest) |
|----------------|------------------|
| [Ansicolor plugin](https://updates.jenkins.io/download/plugins/ansicolor/) | ansicolor:1.0.6 |
| JCasC config   | `ansiColorBuildWrapper:`<br>&nbsp;&nbsp;`globalColorMapName: "xterm"` |
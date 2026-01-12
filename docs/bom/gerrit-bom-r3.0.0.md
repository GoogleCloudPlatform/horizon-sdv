## Gerrit BOM (R3.0.0)

This page documents Horizon SDV Gerrit application component versions so that we can track updates per release.

- TAA-1018: [7] GitOps applications/Gerrit, Gerrit Operator and Gerrit plugins security updates

  - [Summary](#summary)
  - [References](#references)
  - [Components](#components)
    - [k8sgerrit](#k8sgerrit)
    - [k8sgerrit/gerrit-operator:](#k8sgerritgerrit-operator)
    - [k8sgerrit/gerrit:](#k8sgerritgerrit)
    - [gerrit-oauth-provider](#gerrit-oauth-provider)
    - [gerrit-post](#gerrit-post)
    - [plugins](#plugins)
  - [How to Upgrade](#how-to-upgrade)
  - [Problems](#problems)
  - [RefDB issue:](#refdb-issue)
      - [Zookeeper Plugin updates](#zookeeper-plugin-updates)
      - [R3.0.0](#r300)
      - [R2.0.1](#r201)

### Summary

We do need to assess the changes around zookeeper plugin licensing and support if we move to 3.12 and later, to ensure we have no issues. This will be deferred until R4.0.0 because currently there is an bug causing `gerrit-init` and `refdb` that has yet to be backported to versions maintained.

Refer to RefDB issue in Problems below, which describes the problem and identifies the fix, yet to be backported to a compatible release for us to upgrade which describes the problem and identifies the fix, yet to be backported to a compatible release for us to upgrade.

Version 3.13 requires changes to Zookeeper plugin as licensing has changed, this must now be built, refer to Problems below.

### References

- [Gerrit Code Review | Gerrit Code Review](https://www.gerritcodereview.com/)
  - [Gerrit Code Review Releases | Gerrit Code Review ](https://www.gerritcodereview.com/releases-readme.html)
    - [Gerrit Code Review - Releases](https://gerrit-releases.storage.googleapis.com/index.html)

    - [ gerrit - Git at Google ](https://gerrit.googlesource.com/gerrit)
  - https://hub.docker.com/r/gerritcodereview/gerrit/tags  
  - Horizon-SDV Gerrit: [Gerrit Code Review ](https://connected-products.atlassian.net/wiki/spaces/GAC/pages/1704624146)
  - [GerritForge/zookeeper-refdb](https://github.com/GerritForge/zookeeper-refdb)
  - DEPRECATION: [plugins/zookeeper-refdb - Git at Google](https://github.com/GerritForge/zookeeper-refdb)
- [Gerrit Operator](https://gerrit.googlesource.com/k8s-gerrit/+/HEAD/Documentation/operator.md)
  -  [7c0185d9f1c72fafde047bcb70d6943671fe7f06 - k8s-gerrit - Git at Google](https://gerrit.googlesource.com/k8s-gerrit/+/7c0185d9f1c72fafde047bcb70d6943671fe7f06)
  -  [helm-charts/gerrit-operator - k8s-gerrit - Git at Google ](https://gerrit.googlesource.com/k8s-gerrit/+/master/helm-charts/gerrit-operator/)
  -  https://hub.docker.com/r/k8sgerrit/gerrit-operator/tags  
- [GitHub - davido/gerrit-oauth-provider ](https://github.com/davido/gerrit-oauth-provider)

### Components

#### k8sgerrit

Repo: [k8s-gerrit - Git at Google  ](https://gerrit.googlesource.com/k8s-gerrit)

#### k8sgerrit/gerrit-operator:

Refer to https://hub.docker.com/r/k8sgerrit/gerrit-operator/tags for Docker images and  [k8s-gerrit - Git at Google](https://gerrit.googlesource.com/k8s-gerrit) for repo.

The tags match to shas in the repo and you can determine relationship from that tag, e.g.:
- Use `git describe --always --dirty --abbrev=10` to relate docker images (operator, image, init etc)

- `git show <tag>` shows the change (sha1) the tag relates to.
- R2.0.1 Operator: `v0.1-767-g7c0185d9` == `sha1 7c0185d9` and contains the change Horizon SDV required to allow gerrit url to use a path prefix.

Naming convention: `v0.1-767-g7c0185d9`

- `v0.1:` tag  
- `g7c0185d9:` 'g' indicates Git, and the rest is the sha1  

  -   eg. `git checkout 7c0185d9 ; git describe --always --dirty --abbrev=10` 
`v0.1-767-g7c0185d9f1`
- 767: has something to do with commits since some prior tag but they not marry up! Ignore, benign you can diff/compare any way!

#### k8sgerrit/gerrit:

The Gerrit client version is defined from https://hub.docker.com/r/k8sgerrit/gerrit/tags and [`k8s-gerrit - Git at Google `](https://gerrit.googlesource.com/k8s-gerrit)for repo. This client will start gerrit-init (https://hub.docker.com/r/k8sgerrit/gerrit-init/tags) and version aligns with the client version.

-   R2.0.1 Client: `v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91`

Naming convention:[`gerrit:v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91`](https://hub.docker.com/layers/k8sgerrit/gerrit/v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91/images/sha256-5a81bf7dbb7b81def224a4af4946b332fdd7a8b72aafa521f5a1735d84a5d6ec)

- [v0.1-764-g2f33a537b8](https://hub.docker.com/layers/k8sgerrit/gerrit/v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91/images/sha256-5a81bf7dbb7b81def224a4af4946b332fdd7a8b72aafa521f5a1735d84a5d6ec): gerrit operator compatible version  
- [3.11.1-489-g177cc22f91](https://hub.docker.com/layers/k8sgerrit/gerrit/v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91/images/sha256-5a81bf7dbb7b81def224a4af4946b332fdd7a8b72aafa521f5a1735d84a5d6ec): Gerrit client/application version  
  - `3.11.1` : client version  
  - `g77cc22f91`: 'g' indicates Git, and the rest is the sha1  
  - eg.`git checkout 77cc22f91 ; git describe`  
   ` v3.11.1-489-g177cc22f91`
   -` 489`: has something to do with commits since some prior tag but they not marry up! Ignore, benign you can diff/compare any way!
-   v0.1-767-g7c0185d9f1-3.11.1-657-g998b07ccfc
    -   To compare delta:
        -   `git log --pretty=oneline 177cc22f91...998b07ccfc`

 **Note :**
  - We have to take care not to move client too far because of changes that have been made around zookeeper refdb based on hosting and licensing updates.
  - Also, the docker images for 3.11.x only go to 3.11.1-657 and again, if we require download the Gerrit client then that too an R4.0.0 activity to accurately plan the upgrade process.

#### gerrit-oauth-provider

Refer to [GitHub - davido/gerrit-oauth-provider](https://github.com/davido/gerrit-oauth-provider)

This is already on latest:

- Tag: [3.5.0.1](https://github.com/davido/gerrit-oauth-provider/releases/tag/v3.5.1)  
- `name: gerrit-oauth-provider`  
- `url: <https://github.com/davido/gerrit-oauth-provider/releases/download/v3.5.1/gerrit-oauth-provider.jar> ` 
- `sha1: 8e3903ec02900fdec20208242959724cc10f240c`  

#### gerrit-post

This is an internal Horizon SDV component `terraform/modules/sdv-container-images/gerrit/gerrit-post` to initialise gerrit (eg. accesss, All-Users/All-Projects)

#### plugins

Most plugins are defined by Gerrit client needs, but additional for Horizon are defined in `gitops/apps/gerrit/templates/gerrit.yaml`.

As such https://dev.horizon-sdv.com/gerrit/admin/plugins will show more, such as healthcheck and zookeeper-refdb (based on Gerrit DB selection).

#### How to Upgrade

Gerrit operator is defined within `gitops/templates/gerrit-operator.yaml` .  
The sha1 revision can be determined from the tagged version name in Docker hub.

Gerrit client is defined within `gitops/apps/gerrit/templates/gerrit.yaml`. The problem with the client application sync within ArgoCD reverts to env/dev branch and usage of a feature branch is difficult to retain. Setting horizon-sdv target branch has no bearing on the application and gerrit resets the branch back to the original. For now you have to experiment on env/dev and not feature branches.

- If deleting Gerrit applications in ArgoCD, always delete gerrit first and then gerrit-operator.  
You may also wish to delete the gerrit-shared-git storage 1st for full clean install.

Upgrading is generally safe for **patch or minor versions**.  
However, for **major upgrades**, a full reset is required, which involves deleting:

- PVC: `gerrit-shared-git`  
- Application: `gerrit`  
- Application: `gerrit-operator`

**Note:** Deleting Gerrit can take a long time, and with shared persistent volumes, **all stored data may be lost**.

Due to these risks, the upgrade is deferred to **R4.0.0**, not only because of existing Gerrit issues that block an immediate upgrade, but also because a **properly defined upgrade plan/procedure is needed**.  

### Problems

#### RefDB issue:

k8sgerrit/gerrit: v0.1-767-g7c0185d9f1-3.11.1-657-g998b07ccfc:

`gerrit-init` fails when client is aligned with `gerrit-operator` `v0.1-767-g7c0185d9f1`, ie `v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91` to `v0.1-767-g7c0185d9f1-3.11.1-657-g998b07ccfc`

See `git log --pretty=oneline 177cc22f91...998b07ccfc` for changes and gerrit-init errors in logs below:

```
$ kubectl logs gerrit-0 -c gerrit-init -n gerrit
[2025-11-16 11:15:47,929] INFO Requiring plugins (ClusterMode: HIGH_AVAILABILITY): ['healthcheck', 'zookeeper-refdb']
[2025-11-16 11:15:47,931] INFO Requiring libs (ClusterMode: HIGH_AVAILABILITY): ['global-refdb']
[2025-11-16 11:15:47,936] INFO Removed plugin gerrit-oauth-provider.jar
[2025-11-16 11:15:47,936] INFO Removed plugin gitiles.jar
[2025-11-16 11:15:47,936] INFO Removed plugin delete-project.jar
[2025-11-16 11:15:47,937] INFO Removed plugin download-commands.jar
[2025-11-16 11:15:47,942] INFO Installing plugin healthcheck from container to /var/gerrit/plugins/healthcheck.jar.
[2025-11-16 11:15:47,942] DEBUG SHA1 of file '/var/plugins/healthcheck.jar' is 9f1f45952a412ad9445192f11fd7f474278fea65
[2025-11-16 11:15:47,944] DEBUG SHA1 of file '/var/gerrit/plugins/healthcheck.jar' is 9f1f45952a412ad9445192f11fd7f474278fea65
[2025-11-16 11:15:47,947] INFO Installing plugin zookeeper-refdb from container to /var/gerrit/plugins/zookeeper-refdb.jar.
[2025-11-16 11:15:47,969] DEBUG SHA1 of file '/var/plugins/zookeeper-refdb.jar' is ad4d66dddaf4cc524c21e1049af06fb8609c12e4
[2025-11-16 11:15:47,998] DEBUG SHA1 of file '/var/gerrit/plugins/zookeeper-refdb.jar' is ad4d66dddaf4cc524c21e1049af06fb8609c12e4
[2025-11-16 11:15:48,005] INFO Installing plugin global-refdb from container to /var/gerrit/lib/global-refdb.jar.
[2025-11-16 11:15:48,006] DEBUG SHA1 of file '/var/plugins/global-refdb.jar' is f35976daad3cfd8111afe76e5b534aff6f468483
[2025-11-16 11:15:48,007] DEBUG SHA1 of file '/var/gerrit/lib/global-refdb.jar' is f35976daad3cfd8111afe76e5b534aff6f468483
[2025-11-16 11:15:48,007] INFO Installing packaged plugin download-commands.
[2025-11-16 11:15:48,013] INFO Installing packaged plugin delete-project.
[2025-11-16 11:15:48,021] INFO Installing packaged plugin gitiles.
[2025-11-16 11:15:48,041] INFO Downloading gerrit-oauth-provider plugin to /var/gerrit/plugins/gerrit-oauth-provider.jar
[2025-11-16 11:15:48,218] DEBUG SHA1 of file '/var/gerrit/plugins/gerrit-oauth-provider.jar' is 8e3903ec02900fdec20208242959724cc10f240c
[2025-11-16 11:15:52,236] INFO Installed Gerrit version: gerrit version 3.11.1-657-g998b07ccfc; Provided Gerrit version: gerrit version 3.11.1-657-g998b07ccfc).
[2025-11-16 11:15:52,236] INFO Plugins were installed or updated. Initializing.
[2025-11-16 11:15:52,236] INFO Existing gerrit.config found.
Exception in thread "main" com.google.inject.ProvisionException: Unable to provision, see the following errors:
1) [Guice/NullInjectedIntoNonNullable]: null returned by binding at BaseInit$1.configure(BaseInit.java:280)
 but the 2nd parameter repoManager of NoteDbSchemaVersionManager.<init>(NoteDbSchemaVersionManager.java:39) is not @Nullable
  at BaseInit$1.configure(BaseInit.java:280)
  at NoteDbSchemaVersionManager.<init>(NoteDbSchemaVersionManager.java:39)
      \_ for 2nd parameter repoManager
  at ZkInit.versionManager(ZkInit.java:56)
      \_ for field versionManager
  while locating ZkInit
Learn more:
  https://github.com/google/guice/wiki/NULL_INJECTED_INTO_NON_NULLABLE
1 error
======================
Full classname legend:
======================
BaseInit$1:                 "com.google.gerrit.pgm.init.BaseInit$1"
NoteDbSchemaVersionManager: "com.google.gerrit.server.schema.NoteDbSchemaVersionManager"
ZkInit:                     "com.googlesource.gerrit.plugins.validation.dfsrefdb.zookeeper.ZkInit"
========================
End of classname legend:
========================
	at com.google.inject.internal.InternalProvisionException.toProvisionException(InternalProvisionException.java:251)
	at com.google.inject.internal.MembersInjectorImpl.injectMembers(MembersInjectorImpl.java:74)
	at com.google.inject.internal.InjectorImpl.injectMembers(InjectorImpl.java:1109)
	at com.googlesource.gerrit.plugins.validation.dfsrefdb.zookeeper.ZkInit.run(ZkInit.java:72)
	at com.google.gerrit.pgm.init.InitPlugins.initPlugins(InitPlugins.java:180)
	at com.google.gerrit.pgm.init.InitPlugins.run(InitPlugins.java:99)
	at com.google.gerrit.pgm.init.SitePathInitializer.run(SitePathInitializer.java:92)
	at com.google.gerrit.pgm.init.BaseInit.run(BaseInit.java:115)
	at com.google.gerrit.pgm.util.AbstractProgram.main(AbstractProgram.java:62)
	at java.base/jdk.internal.reflect.DirectMethodHandleAccessor.invoke(DirectMethodHandleAccessor.java:103)
	at java.base/java.lang.reflect.Method.invoke(Method.java:580)
	at com.google.gerrit.launcher.GerritLauncher.invokeProgram(GerritLauncher.java:251)
	at com.google.gerrit.launcher.GerritLauncher.mainImpl(GerritLauncher.java:147)
	at com.google.gerrit.launcher.GerritLauncher.main(GerritLauncher.java:92)
	at Main.main(Main.java:30)
Traceback (most recent call last):
  File "<frozen runpy>", line 198, in _run_module_as_main
  File "<frozen runpy>", line 88, in _run_code
  File "/var/tools/gerrit-initializer/__main__.py", line 18, in <module>
    main()
  File "/var/tools/gerrit-initializer/main.py", line 96, in main
    args.func(args)
  File "/var/tools/gerrit-initializer/main.py", line 43, in _run_init
    GerritInitHA(_parse_gerrit_config(), config).execute()
  File "/var/tools/gerrit-initializer/initializer/tasks/init.py", line 188, in execute
    init_process = subprocess.run(
                   ^^^^^^^^^^^^^^^
  File "/usr/lib/python3.12/subprocess.py", line 573, in run
    raise CalledProcessError(retcode, process.args,
subprocess.CalledProcessError: Command '['java', '-jar', '/var/war/gerrit.war', 'init', '--no-auto-start', '--batch', '-d', '/var/gerrit']' returned non-zero exit status 1.
```
Analysis has found the issue is a real Gerrit bug, see:

-   See https://issues.gerritcodereview.com/issues/440874371 , we are using zookeeper and not installing as per bug description but failure and cause are identical (see Gerrit change exposing the issue: https://gerrit-review.googlesource.com/c/gerrit/+/449321)  
See also below, change is in `998b07ccfc` but not `177cc22f91`

```
dave.m.smith in ~/Horizon/source/gerrit/java/com/google/gerrit/pgm ((HEAD detached at 998b07ccfc))
$ git blame init/BaseInit.java | grep GitRepositoryManager.class
406bf5ee48a java/com/google/gerrit/pgm/init/BaseInit.java                     (Daniele Sassoli    2025-01-21 14:39:04 +0000 280)             bind(GitRepositoryManager.class).toProvider(Providers.of(null));
```
-   Fixed in: 1c85cd3f89d5971be0121f2258f987f38e909399 but needs to be assessed on where this has been backported and whether the image is hosted in docker hub k8sgerrit.

-   The fix has only been patched in 3.12.3 and 3.13.0-rcX (release candidate) and the docker versions are not available on k8sgerrit/gerrit and thus we will not upgrade unless these versions become available. No plan to use release candidates only official.
```
Bug: Issue 440874371  
Release-Notes: Avoid hacking the GitRepositoryManager on init, which broke the initialisation of a Gerrit site with Zookeeper.
```
#### Zookeeper Plugin updates

Update 3.13.x is too far, the zookeeper refdb plugin is no longer supported. Changes in license etc, requires this now to be built. If you don’t build, gerrit-init fails as follows:

```
# Gerrit Operator: v0.1-767-g7c0185d9
# Gerrit Client: v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91

$ kubectl logs gerrit-0 -c gerrit-init -n gerrit
[2025-11-16 18:49:25,756] INFO Requiring plugins (ClusterMode: HIGH_AVAILABILITY): ['healthcheck', 'zookeeper-refdb']
[2025-11-16 18:49:25,759] INFO Requiring libs (ClusterMode: HIGH_AVAILABILITY): ['global-refdb']
[2025-11-16 18:49:25,770] INFO Installing plugin healthcheck from container to /var/gerrit/plugins/healthcheck.jar.
[2025-11-16 18:49:25,770] DEBUG SHA1 of file '/var/plugins/healthcheck.jar' is 151866d90cf4a256db91d4560969fcdcda508fb1
[2025-11-16 18:49:25,771] DEBUG SHA1 of file '/var/gerrit/plugins/healthcheck.jar' is 151866d90cf4a256db91d4560969fcdcda508fb1
[2025-11-16 18:49:25,776] INFO Installing plugin zookeeper-refdb from container to /var/gerrit/plugins/zookeeper-refdb.jar.
Traceback (most recent call last):
  File "/var/tools/gerrit-initializer/initializer/tasks/download_plugins.py", line 150, in _install_required_jar
    self._install_jar_from_container(jar, target_dir)
  File "/var/tools/gerrit-initializer/initializer/tasks/download_plugins.py", line 163, in _install_jar_from_container
    raise FileNotFoundError(
FileNotFoundError: Unable to find required plugin in container: zookeeper-refdb

During handling of the above exception, another exception occurred:

Traceback (most recent call last):
  File "<frozen runpy>", line 198, in _run_module_as_main
  File "<frozen runpy>", line 88, in _run_code
  File "/var/tools/gerrit-initializer/__main__.py", line 18, in <module>
    main()
  File "/var/tools/gerrit-initializer/main.py", line 96, in main
    args.func(args)
  File "/var/tools/gerrit-initializer/main.py", line 43, in _run_init
    GerritInitHA(_parse_gerrit_config(), config).execute()
  File "/var/tools/gerrit-initializer/initializer/tasks/init.py", line 163, in execute
    self.plugin_installer.execute()
  File "/var/tools/gerrit-initializer/initializer/tasks/download_plugins.py", line 259, in execute
    self._install_required_plugins()
  File "/var/tools/gerrit-initializer/initializer/tasks/download_plugins.py", line 133, in _install_required_plugins
    self._install_required_jar(plugin, SITE_PLUGIN_PATH)
  File "/var/tools/gerrit-initializer/initializer/tasks/download_plugins.py", line 152, in _install_required_jar
    raise MissingRequiredPluginException(f"Required jar {jar} was not found.")
initializer.tasks.download_plugins.MissingRequiredPluginException: Required jar zookeeper-refdb was not found.
```
 
> **Note :** Need to check whether 3.12.x requires this change also. But 3.12 is blocked on the RefDB regression.

#### R3.0.0

- Same as R2.0.1, upgrades blocked on Zookeeper RefDB issue and lack of compatible versions, other than 3.13.0-rcX on k8sgerrit/gerrit. No plan to use a release candidate for 3.13 and also we would need to assess Zookeeper licensing updates. Remain on 3.11.1 for R3.0.0.

**Gerrit Code Review:** `3.11.1-489-g177cc22f91 ` 
**Docker Image:** [k8sgerrit/gerrit:v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91](https://hub.docker.com/layers/k8sgerrit/gerrit/v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91/images/sha256-5a81bf7dbb7b81def224a4af4946b332fdd7a8b72aafa521f5a1735d84a5d6ec)   
**Gerrit Operator:**  `v0.1-767-g7c0185d9`

```yaml
Gerrit Operator:

repoURL: https://gerrit.googlesource.com/k8s-gerrit
targetRevision: 7c0185d9f1c72fafde047bcb70d6943671fe7f06
path: helm-charts/gerrit-operator
helm:
  values: |
    image:
      tag: v0.1-767-g7c0185d9
```

| Plugin | Version | API Version |
|------|--------|-------------|
| [delete-project](https://dev.horizon-sdv.com/gerrit/plugins/delete-project/Documentation/index.html) | `v3.11.1-5-g437c4ddafd` | `3.12.0-SNAPSHOT` |
| [download-commands](https://dev.horizon-sdv.com/gerrit/plugins/download-commands/Documentation/index.html) | `v3.11.1` | `3.12.0-SNAPSHOT` |
| [gerrit-oauth-provider](https://dev.horizon-sdv.com/gerrit/plugins/gerrit-oauth-provider/Documentation/index.html) | `d21d172<br>name: gerrit-oauth-provider<br>url: <https://github.com/davido/gerrit-oauth-provider/releases/download/v3.5.1/gerrit-oauth-provider.jar><br>sha1: 8e3903ec02900fdec20208242959724cc10f240c` | [3.5.0.1](https://github.com/davido/gerrit-oauth-provider/releases/tag/v3.5.1) |
| [gitiles](https://dev.horizon-sdv.com/gerrit/plugins/gitiles/) | `v3.11.1-1-g7cb549065f` | `3.12.0-SNAPSHOT` |
| [healthcheck](https://dev.horizon-sdv.com/gerrit/plugins/healthcheck/Documentation/index.html) | `v3.5.6-49-g8739ef95c7` | `3.12.0-SNAPSHOT` |
| [zookeeper-refdb](https://dev.horizon-sdv.com/gerrit/plugins/zookeeper-refdb/Documentation/index.html) | `v3.3.0-40-g97923cbd6a` | `3.11.0-SNAPSHOT` |

#### R2.0.1

**Gerrit Code Review:** `3.11.1-489-g177cc22f91`   
**Docker Image:** [k8sgerrit/gerrit:v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91](https://hub.docker.com/layers/k8sgerrit/gerrit/v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91/images/sha256-5a81bf7dbb7b81def224a4af4946b332fdd7a8b72aafa521f5a1735d84a5d6ec)   

**Gerrit Operator:** 
```yaml
    repoURL: https://gerrit.googlesource.com/k8s-gerrit
    targetRevision: 7c0185d9f1c72fafde047bcb70d6943671fe7f06
    path: helm-charts/gerrit-operator
    helm:
      values: |
        image:
          tag: v0.1-767-g7c0185d9
```
| Plugin | Version | API Version |
|--------|---------|-------------|
| [delete-project](https://dev.horizon-sdv.com/gerrit/plugins/delete-project/Documentation/index.html) | `v3.11.1-5-g437c4ddafd` | `3.12.0-SNAPSHOT` |
| [download-commands](https://dev.horizon-sdv.com/gerrit/plugins/download-commands/Documentation/index.html) | `v3.11.1` | `3.12.0-SNAPSHOT` |
| [gerrit-oauth-provider](https://dev.horizon-sdv.com/gerrit/plugins/gerrit-oauth-provider/Documentation/index.html) | `d21d172<br>name: gerrit-oauth-provider<br>url: <https://github.com/davido/gerrit-oauth-provider/releases/download/v3.5.1/gerrit-oauth-provider.jar><br>sha1: 8e3903ec02900fdec20208242959724cc10f240c` | [3.5.0.1](https://github.com/davido/gerrit-oauth-provider/releases/tag/v3.5.1) |
| [gitiles](https://dev.horizon-sdv.com/gerrit/plugins/gitiles/) | `v3.11.1-1-g7cb549065f` | `3.12.0-SNAPSHOT` |
| [healthcheck](https://dev.horizon-sdv.com/gerrit/plugins/healthcheck/Documentation/index.html) | `v3.5.6-49-g8739ef95c7` | `3.12.0-SNAPSHOT` |
| [zookeeper-refdb](https://dev.horizon-sdv.com/gerrit/plugins/zookeeper-refdb/Documentation/index.html) | `v3.3.0-40-g97923cbd6a` | `3.11.0-SNAPSHOT` |
## Horizon SDV Rel.3.0.0 Open Source Modules

- [Horizon SDV Rel.3.0.0 Open Source Modules](#horizon-sdv-rel300-open-source-modules)
  - [Components](#components)
    - [Applications (Open Source)](#applications-open-source)
    - [Applications (Closed Source)](#applications-closed-source)
    - [Jenkins Plugins](#jenkins-plugins)
    - [Gerrit Plugins](#gerrit-plugins)
    - [Terraform](#terraform)
    - [Deployment script](#deployment-script)
    - [Post Jobs](#post-jobs)
    - [Horizon Apps](#horizon-apps)

### Components

All components installed in terraform, deployment script and GitOps operations are listed below. All of them are provided with carefully selected versions. Some components are helm charts based, some other are container based. Sometimes, version numbers aren’t managed correctly, but even that - it is possible to pin down each single version for each component. Each component contains a link to the location in Horizon SDV project when actually, the component and its version are referenced.

Internal/External column gives an information if such a component is explicitly referenced in Horizon SDV project or it’s an indirect dependency of other component, additionally - in some scenarios even if it is indirectly referenced - it can still be force pinned to a given version.

---

#### Applications (Open Source)

This list contains all Open Source application included in Horizon SDV project.

| # | Applications (Open Source) | Current app version | Helm Chart | Internal/External | Description | Location |
|---|---------------------------|---------------------|------------|-------------------|-------------|----------|
| 1 | Gerrit Operator | [7c0185d9f1c72fafde047bcb70d6943671fe7f06](https://gerrit.googlesource.com/k8s-gerrit/+/7c0185d9f1c72fafde047bcb70d6943671fe7f06/) | [0.1.0](https://gerrit.googlesource.com/k8s-gerrit/+/7c0185d9f1c72fafde047bcb70d6943671fe7f06/helm-charts/gerrit-operator/Chart.yaml) | External | Gerrit Operator - installs and manages Gerrit instances | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/env/stage2/templates/gerrit-operator.yaml) |
| 2 | Gerrit | [Image Layer Details - k8sgerrit/gerrit:v0.1-767-g7c0185d9f1-3.11.1-657-g998b07ccfc \| Docker Hub](https://hub.docker.com/layers/k8sgerrit/gerrit/v0.1-764-g2f33a537b8-3.11.1-489-g177cc22f91/images/sha256-5a81bf7dbb7b81def224a4af4946b332fdd7a8b72aafa521f5a1735d84a5d6ec) | N/A | External | GerritCluster resource of Gerrit Operator | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/templates/gerrit.yaml) |
| 3 | Zookeeper | [3.9.3](https://github.com/apache/zookeeper/tree/release-3.9.3) | [13.8.7](https://github.com/bitnami/charts/blob/main/bitnami/zookeeper/Chart.yaml) | External | Centralized service used by Gerrit for maintaining configuration information, naming, providing distributed synchronization, and providing group services. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/templates/zookeeper.yaml) |
| 4 | ArgoCD | [2.12.6](https://github.com/argoproj/argo-cd/tree/v2.12.6) | [9.1.4](https://artifacthub.io/packages/helm/argo/argo-cd/9.1.4) | External | Declarative GitOps continuous delivery tool for Kubernetes implemented as a Kubernetes controller which continuously monitors running applications and compares the current, live state against the desired target state (as specified in the Git repo). | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/sdv-gke-apps/main.tf) |
| 5 | Jenkins | [2.528.3](https://www.jenkins.io/changelog-stable/) | [5.8.110](https://github.com/jenkinsci/helm-charts/releases) | External | Automation server which automates the parts of software development process related to building, testing, and deploying, facilitating continuous integration, and continuous delivery. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/templates/jenkins.yaml)|
| 6 | Keycloakx | [26.0.7](https://github.com/keycloak/keycloak/tree/26.0.7) | [7.1.4](https://github.com/codecentric/helm-charts/blob/master/charts/keycloakx/Chart.yaml) | External | Identity and Access Management which adds authentication to applications and secure services with SSO support. Keycloak provides user federation, strong authentication, user management, fine-grained authorization, and role based access control. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/templates/keycloak.yaml) |
| 7 | PostgreSQL | [17.0.0](https://github.com/bitnami/charts/blob/7b3f2bb7a65a78cba10fe8dfe87fd47b55dd8ec0/bitnami/postgresql/Chart.yaml#L15) (bitnami build) | [17.0.1](https://github.com/bitnami/charts/blob/main/bitnami/postgresql/Chart.yaml) | External | Relational database for Keycloak | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/env/stage2/values.yaml) |
| 8 | dynamic-pvc-provisioner | [0.2.3](https://github.com/plumber-cd/kubernetes-dynamic-reclaimable-pvc-controllers/tree/v0.2.3) | [0.1.1](https://github.com/plumber-cd/helm/tree/dynamic-pvc-provisioner-0.1.1) | External | Persistent Volume support. Dynamic PVC provisioner for Pods requesting it via annotations. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/templates/dynpvc-provisioner.yaml)|
| 9 | reclaimable-pv-releaser | [0.2.3](https://github.com/plumber-cd/kubernetes-dynamic-reclaimable-pvc-controllers/tree/v0.2.3) | [0.1.1](https://github.com/plumber-cd/helm/tree/reclaimable-pv-releaser-0.1.1) | External | Automatic PV releaser. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/templates/dynpvc-releaser.yaml)|
|10 | external-secrets | [0.10.4](https://github.com/external-secrets/external-secrets/tree/v0.10.4) | [0.10.4](https://github.com/external-secrets/external-secrets/tree/v0.10.4/deploy/charts/external-secrets) | External | Handles secret sharing between GCP Secret Manager and K8S Secrets. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/sdv-gke-apps/main.tf) |
|11 | Landing Page | [0.0.1](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/b20f93e03c7428cc8d865e6dcef644dc34423f45/gitops/env/stage2/apps/landingpage/Chart.yaml#L20) | [0.0.1](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/b20f93e03c7428cc8d865e6dcef644dc34423f45/gitops/env/stage2/apps/landingpage/Chart.yaml#L18) | External | Horizon Landing Page. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/apps/landingpage/templates/landingpage.yaml) |
|12 | Kubescape-operator | [1.29.12](https://github.com/kubescape/helm-charts/blob/main/charts/kubescape-operator/Chart.yaml) | [1.29.11](https://github.com/kubescape/helm-charts/blob/main/charts/kubescape-operator/Chart.yaml) | External | Automated Kubernetes security monitoring | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/env/stage2/values.yaml) |

---

#### Applications (Closed Source)

This list contains all closed source applications included in Horizon SDV project.

| # | Applications (Closed Source) | Current app version | Helm Chart | Internal/External | Description | Location |
|---|-----------------------------|---------------------|------------|-------------------|-------------|----------|
| 1 | MTK Connect | [1.10.0](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/dc6130c267b1994a816eeeabae55da91a0047c05/gitops/env/stage2/apps/mtk-connect/Chart.yaml#L20) | [1.10.0](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/dc6130c267b1994a816eeeabae55da91a0047c05/gitops/env/stage2/apps/mtk-connect/Chart.yaml#L18) | External | MTK Connect provides connectivity to remote devices for automated and manual testing. |[ LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/gitops/apps/mtk-connect) |

---

#### Jenkins Plugins

This list contains all Jenkins plugins installed. “Internal” plugins are installed together with main Jenkins application, but are pinned down to the versions mentioned down. External plugins and added beyond that. Detailed list of Jenkins plugins and its versions is available in  
[Jenkins BOM (3.0.0)](jenkins-bom-3.0.0.md)

---

#### Gerrit Plugins

This list contains all Gerrit plugins. Most of them are added after the installation, but healthcheck and zookeeper-refdb are Gerrit Operator internal and cannot be controlled explicitly.

| # | Gerrit Plugins | Version | API Version | Internal/External | Description | Location |
|---|---------------|---------|-------------|-------------------|-------------|----------|
| 1 | delete-project | v3.11.1-5-g437c4ddafd | 3.12.0-SNAPSHOT | External | Allows deleting projects in Gerrit. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/apps/gerrit/templates/gerrit.yaml#L88) |
| 2 | download-commands | v3.11.1 | 3.12.0-SNAPSHOT | External | Adds “Download” commands to Gerrit. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/apps/gerrit/templates/gerrit.yaml#L87) |
| 3 | gerrit-oauth-provider | d21d172 | 3.5.0.1 | External | OAUTH2 proxy to Keycloak. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/apps/gerrit/templates/gerrit.yaml#L90) |
| 4 | gitiles | v3.11.1-1-g7cb549065f | 3.12.0-SNAPSHOT | External | Gitiles git repository browser. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/gitops/apps/gerrit/templates/gerrit.yaml#L89) |
| 5 | healthcheck | v3.5.6-49-g8739ef95c7 | 3.12.0-SNAPSHOT | Internal | Internal plugin: just healthcheck - more important for multisite installation. | N/A |
| 6 | zookeeper-refdb | v3.3.0-40-g97923cbd6a | 3.11.0-SNAPSHOT | Internal | Internal plugin: Manages connection to Zookeeper RefDB. | N/A |

---

#### Terraform

Terraform allows provision and manage cloud infastucture based on Infrastructure as Code (IaC) concept.Horizon terraform required_version >= 1.9.6 . Current version Terraform code contains 3 levels of versions:

- provider (whole Google Cloud Platform provider)
- module (selected GCP module versions)
- resource (selected resource versions)

| # | Terraform | Uses | Version | Internal/External | Description | Location |
|---|-----------|------|---------|-------------------|-------------|----------|
| 1 | Terraform |  | 1.14.2 | External |  |  |
| 2 | provider/google | hashicorp/google | 7.12.0 | External | Terraform provider: “google”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 3 | provider/google-beta | hashicorp/google-beta | 7.12.0 | External | Terraform provider: “google-beta”. |[ LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 4 | provider/helm | hashicorp/helm | ~> 3.0 | External | Terraform provider: “helm”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 5 | provider/kubernetes | hashicorp/kubernetes | ~> 2.0 | External | Terraform provider: “kubernetes”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 6 | provider/docker | kreuzwerker/docker | 3.6.2 | External | Terraform provider: “docker”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 7 | provider/kubectl | gavinbunney/kubectl | ~> 1.14 | External | Terraform provider: “kubectl”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 8 | provider/tls | hashicorp/tls | >= 4.0 | External | Terraform provider: “tls”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
| 9 | provider/local | hashicorp/local | >= 2.4.0 | External | Terraform provider: “local”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
|10 | provider/external | hashicorp/external | >= 2.3.0 | External | Terraform provider: “external”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/base/version.tf) |
|11 | module/sdv-network | terraform-google-modules/network/google | ~>13.1 | External | Terraform module: “vpc”. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/env/sbx/terraform/modules/sdv-network/main.tf) |

#### Deployment script

List below contains all dependencies installed during the [deployment script](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) .

| # | Deployment script component | Version | Internal/External | Description | Location |
|---|-----------------------------|---------|-------------------|-------------|----------|
| 1 | git | Latest | External | git client | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |
| 2 | [Docker: Accelerated Container Application Development](http://docker.io/) | 29.1.3 | External | docker client | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile)|
| 3 | kubectl | 1.34.3 | External | kubectl client | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |
| 4 | gcloud | 549.0.1 | External | kubectl cli | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |
| 5 | curl | N/A | External | Linux Ubuntu 22.04 tool | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |
| 6 | unzip | Latest | External | Linux Ubuntu 22.04 compression tool | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |
| 7 | gnupg | Latest | External | Linux Ubuntu 22.04 tool for pgp encryption | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |
| 8 | ca-certificates | Latest | External | Linux Ubuntu 22.04 tool for root certificates | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/tools/scripts/deployment/container/Dockerfile) |

#### Post Jobs

Post Jobs are structured on top of a Base OS. Internal dependencies are included in Base OS, external ones are downloaded per need.

| # | Post Job | Component | Version | Internal/External | Description | Location |
|---|---------|-----------|---------|-------------------|-------------|----------|
| 1 | gerrit-post | debian | 12.9 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 2 | gerrit-post | tzdata | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 3 | gerrit-post | vim | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 4 | gerrit-post | curl | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 5 | gerrit-post | jq | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 6 | gerrit-post | yq | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 7 | gerrit-post | openssh-client | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 8 | gerrit-post | git | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 9 | gerrit-post | procps | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 10 | gerrit-post | kubectl | Latest | External | Kubectl. |[LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit/gerrit-post/Dockerfile) |
| 11 | gerrit-mcp-server | python:3.12-slim-bookworm | N/A | Internal | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit-mcp-server/gerrit-mcp-server-app/Dockerfile) |
| 12 | gerrit-mcp-server | curl | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit-mcp-server/gerrit-mcp-server-app/Dockerfile) |
| 13 | gerrit-mcp-server | vim | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit-mcp-server/gerrit-mcp-server-app/Dockerfile) |
| 14 | gerrit-mcp-server | git | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/gerrit-mcp-server/gerrit-mcp-server-app/Dockerfile) |
| 15 | keycloak-post-gerrit | node | 22.13.0 | Internal | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-gerrit/Dockerfile) |
| 16 | keycloak-post-gerrit | vim | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-gerrit/Dockerfile) |
| 17 | keycloak-post-gerrit | curl | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-gerrit/Dockerfile) |
| 18 | keycloak-post-gerrit | jq | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-gerrit/Dockerfile) |
| 19 | keycloak-post-jenkins | node | 22.13.0 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-jenkins/Dockerfile) |
| 20 | keycloak-post-jenkins | vim | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-jenkins/Dockerfile) |
| 21 | keycloak-post-jenkins | curl | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-jenkins/Dockerfile) |
| 22 | keycloak-post-jenkins | jq | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-jenkins/Dockerfile) |
| 23 | keycloak-post-mtk-connect | node | 22.13.0 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-mtk-connect/Dockerfile) |
| 24 | keycloak-post-mtk-connect | vim | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-mtk-connect/Dockerfile) |
| 25 | keycloak-post-mtk-connect | kubectl | Latest | External | Kubectl. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-mtk-connect/Dockerfile) |
| 26 | keycloak-post | node | 22.13.0 | External | Base OS. |[ LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile) |
| 27 | mtk-connect-post-key | python | 3.9-slim | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile) |
| 28 | mtk-connect-post-key | requests | 2.32.3 | External | Python dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile) |
| 29 | mtk-connect-post-key | python-dateutil | 2.9.0.post0 | External | Python dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile) |
| 30 | mtk-connect-post-key | kubernetes | 32.0.1 | External | Python dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post-key/Dockerfile) |
| 31 | mtk-connect-post | debian | 12.9 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post/Dockerfile) |
| 32 | mtk-connect-post | vim | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post/Dockerfile) |
| 33 | mtk-connect-post | curl | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post/Dockerfile) |
| 34 | mtk-connect-post | skopeo | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post/Dockerfile) |
| 35 | mtk-connect-post | jq | N/A | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post/Dockerfile) |
| 36 | mtk-connect-post | kubectl | Latest | External | Kubectl. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/mtk-connect/mtk-connect-post/Dockerfile) |
| 37 | keycloak-post-argocd | node | 22.13.0 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-argocd/Dockerfile) |
| 38 | keycloak-post-argocd | vim | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-argocd/Dockerfile) |
| 39 | keycloak-post-argocd | curl | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-argocd/Dockerfile) |
| 40 | keycloak-post-argocd | jq | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-argocd/Dockerfile) |
| 41 | keycloak-post-headlamp | node | 22.13.0 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-headlamp/Dockerfile) |
| 42 | keycloak-post-headlamp | vim | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-headlamp/Dockerfile) |
| 43 | keycloak-post-headlamp | curl | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-headlamp/Dockerfile) |
| 44 | keycloak-post-headlamp | jq | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-headlamp/Dockerfile) |
| 45 | keycloak-post-grafana | node | 22.13.0 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-grafana/Dockerfile) |
| 46 | keycloak-post-grafana | vim | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-grafana/Dockerfile) |
| 47 | keycloak-post-grafana | curl | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-grafana/Dockerfile) |
| 48 | keycloak-post-grafana | jq | Latest | Internal | OS dependency. | [LIN](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/keycloak/keycloak-post-grafana/Dockerfile)K |
| 49 | grafana-post | node | 22.13.0 | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/grafana/grafana-post/Dockerfile) |
| 50 | grafana-post | vim | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/grafana/grafana-post/Dockerfile) |
| 51 | grafana-post | curl | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/grafana/grafana-post/Dockerfile) |
| 52 | grafana-post | jq | Latest | Internal | OS dependency. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/tree/env/sbx/terraform/modules/sdv-container-images/images/grafana/grafana-post/Dockerfile) |

---

#### Horizon Apps

List below contains a Base OS for Horizon SDV home page.

| Horizon App | Component | Version | Internal/External | Description | Location |
|------------|-----------|---------|-------------------|-------------|----------|
| landingpage-app | nginx | 1.28.0-alpine | External | Base OS. | [LINK](https://github.com/AGBG-ASG/acn-horizon-sdv/blob/dc6130c267b1994a816eeeabae55da91a0047c05/gitops/env/stage2/configs/landingpage/landingpage-app/Dockerfile#L15) |
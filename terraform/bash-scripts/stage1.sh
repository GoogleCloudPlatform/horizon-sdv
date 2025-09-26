#!/usr/bin/env bash

# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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
#
# Description:
# This script is executed post successful setup of the GKE cluster and
# performs various initial operations.
#
# The script performs below actions:
# 1. Retrieve information of various tools present on the bastion host such
#    as helm, kubectl, docker and so on.
# 2. Establish connection to the GKE cluster.
# 3. Setup docker with required permission and perform a docker pull check.
# 4. Clone the GitHub repository for further setup.

CLUSTER_NAME="sdv-cluster"
REPOSITORY=github.com/${GITHUB_REPO_NAME}

touch ~/terraform-log.log
echo $(date) >>~/terraform-log.log
cat ~/terraform-log.log

echo ""
echo "Updating Debian APT"
export DEBIAN_FRONTEND=noninteractive
sudo apt update && sudo apt upgrade -y

echo ""
echo "Updating Debian APT"
sudo apt install -y git docker.io kubectl google-cloud-cli-gke-gcloud-auth-plugin
curl -fsSL https://packages.buildkite.com/helm-linux/helm-debian/gpgkey | gpg --dearmor | sudo tee /usr/share/keyrings/helm.gpg > /dev/null
sudo apt-get install apt-transport-https --yes
echo "deb [signed-by=/usr/share/keyrings/helm.gpg] https://packages.buildkite.com/helm-linux/helm-debian/any/ any main" | sudo tee /etc/apt/sources.list.d/helm-stable-debian.list
sudo apt-get update
sudo apt install -y helm
gcloud container clusters get-credentials sdv-cluster --region ${GCP_CLOUD_REGION}

echo ""
echo "Helm Version"
helm version

echo ""
echo "Install Helm Diff plugin"
helm plugin install https://github.com/databus23/helm-diff

echo ""
echo "Kubectl Version"
kubectl version

echo ""
echo "docker version"
sudo docker version

echo ""
echo "Gcloud Info"
gcloud info

echo ""
echo "Connecting to Kubernetes"
gcloud container clusters get-credentials ${CLUSTER_NAME} --region ${GCP_CLOUD_REGION}

echo ""
echo "List $SDV_CLUSTER_NAME nodes"
kubectl get nodes

echo ""
echo "Adding the current user to the docker group"
echo "sudo usermod -aG docker $USER"
sudo usermod -aG docker $USER

echo ""
echo "Docker configurations"
gcloud auth configure-docker ${GCP_CLOUD_REGION}-docker.pkg.dev --quiet
sudo gcloud auth configure-docker ${GCP_CLOUD_REGION}-docker.pkg.dev --quiet

echo ""
echo "Removing old project"
rm -rf ~/horizon-sdv

echo ""
echo "Cloning github project"
git clone https://x-access-token:${GITHUB_ACCESS_TOKEN}@${REPOSITORY} ~/horizon-sdv

echo ""
echo "List current branch and remote"
cd ~/horizon-sdv
git checkout -t origin/env/${GITHUB_ENV_NAME}

echo ""
echo "Build config post jobs"
cd ~/horizon-sdv/gitops/env/stage2/configs
chmod +x ./build.sh
sudo -s GCP_CLOUD_REGION=$GCP_CLOUD_REGION GCP_PROJECT_ID=$GCP_PROJECT_ID ./build.sh

echo ""
echo "Run stage1 deployment"
cd ~/horizon-sdv/gitops/env/stage1
chmod +x ./deploy.sh
./deploy.sh

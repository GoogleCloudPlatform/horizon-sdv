#!/bin/sh
# Copyright (c) 2026 Accenture, All Rights Reserved.
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
# Ensures the secondary region networking (subnet, Cloud Router, Cloud NAT) and
# Cloud Workstations cluster exist so dual-region GPU capacity can be used.

set -eu

: "${PROJECT:?PROJECT is required}"
: "${VPC_NETWORK:?VPC_NETWORK is required}"
: "${SECONDARY_REGION:?SECONDARY_REGION is required}"
: "${SECONDARY_CLUSTER:?SECONDARY_CLUSTER is required}"
: "${SECONDARY_SUBNET:?SECONDARY_SUBNET is required}"
: "${SECONDARY_SUBNET_CIDR:?SECONDARY_SUBNET_CIDR is required}"
: "${SECONDARY_ROUTER:?SECONDARY_ROUTER is required}"
: "${SECONDARY_NAT:?SECONDARY_NAT is required}"

ensure_subnet() {
  if gcloud compute networks subnets describe "${SECONDARY_SUBNET}" \
      --project="${PROJECT}" --region="${SECONDARY_REGION}" >/dev/null 2>&1; then
    echo "Subnet ${SECONDARY_SUBNET} already exists in ${SECONDARY_REGION}."
    return 0
  fi

  echo "Creating subnet ${SECONDARY_SUBNET} (${SECONDARY_SUBNET_CIDR}) in ${SECONDARY_REGION}..."
  gcloud compute networks subnets create "${SECONDARY_SUBNET}" \
      --project="${PROJECT}" \
      --region="${SECONDARY_REGION}" \
      --network="${VPC_NETWORK}" \
      --range="${SECONDARY_SUBNET_CIDR}" \
      --enable-private-ip-google-access \
      --enable-flow-logs
}

ensure_router_and_nat() {
  if ! gcloud compute routers describe "${SECONDARY_ROUTER}" \
      --project="${PROJECT}" --region="${SECONDARY_REGION}" >/dev/null 2>&1; then
    echo "Creating Cloud Router ${SECONDARY_ROUTER} in ${SECONDARY_REGION}..."
    gcloud compute routers create "${SECONDARY_ROUTER}" \
        --project="${PROJECT}" \
        --region="${SECONDARY_REGION}" \
        --network="${VPC_NETWORK}"
  else
    echo "Cloud Router ${SECONDARY_ROUTER} already exists in ${SECONDARY_REGION}."
  fi

  if gcloud compute routers nats describe "${SECONDARY_NAT}" \
      --router="${SECONDARY_ROUTER}" \
      --project="${PROJECT}" --region="${SECONDARY_REGION}" >/dev/null 2>&1; then
    echo "Cloud NAT ${SECONDARY_NAT} already exists on ${SECONDARY_ROUTER}."
    return 0
  fi

  echo "Creating Cloud NAT ${SECONDARY_NAT} on ${SECONDARY_ROUTER}..."
  gcloud compute routers nats create "${SECONDARY_NAT}" \
      --project="${PROJECT}" \
      --region="${SECONDARY_REGION}" \
      --router="${SECONDARY_ROUTER}" \
      --nat-custom-subnet-ip-ranges="${SECONDARY_SUBNET}" \
      --auto-allocate-nat-external-ips \
      --enable-dynamic-port-allocation \
      --min-ports-per-vm=64 \
      --max-ports-per-vm=4096 \
      --enable-logging \
      --log-filter=TRANSLATIONS_ONLY
}

ensure_workstation_cluster() {
  if gcloud workstations clusters describe "${SECONDARY_CLUSTER}" \
      --project="${PROJECT}" --region="${SECONDARY_REGION}" >/dev/null 2>&1; then
    echo "Cloud Workstations cluster ${SECONDARY_CLUSTER} already exists in ${SECONDARY_REGION}."
    return 0
  fi

  echo "Starting creation of Cloud Workstations cluster ${SECONDARY_CLUSTER} in ${SECONDARY_REGION}..."
  gcloud workstations clusters create "${SECONDARY_CLUSTER}" \
      --project="${PROJECT}" \
      --region="${SECONDARY_REGION}" \
      --network="projects/${PROJECT}/global/networks/${VPC_NETWORK}" \
      --subnetwork="projects/${PROJECT}/regions/${SECONDARY_REGION}/subnetworks/${SECONDARY_SUBNET}" \
      --async
}

ensure_subnet
ensure_router_and_nat
ensure_workstation_cluster

# Ensure additional multi-region event subnets, Cloud Routers, Cloud NATs, and CWS clusters.
ADDITIONAL_INFRA="
europe-west4:10.4.0.0/24:sdv-cloud-ws-cluster-euw4
europe-west1:10.5.0.0/24:sdv-cloud-ws-cluster-euw1
europe-west2:10.6.0.0/24:sdv-cloud-ws-cluster-euw2
us-east4:10.7.0.0/24:sdv-cloud-ws-cluster-use4
us-east1:10.8.0.0/24:sdv-cloud-ws-cluster-use1
us-central1:10.9.0.0/24:sdv-cloud-ws-cluster-usc1
us-west1:10.11.0.0/24:sdv-cloud-ws-cluster-usw1
"
for entry in ${ADDITIONAL_INFRA}; do
  SECONDARY_REGION="${entry%%:*}"
  REST="${entry#*:}"
  SECONDARY_SUBNET_CIDR="${REST%%:*}"
  SECONDARY_CLUSTER="${REST#*:}"
  SECONDARY_SUBNET="sdv-subnet-${SECONDARY_REGION}"
  SECONDARY_ROUTER="${VPC_NETWORK}-${SECONDARY_REGION}-nat-router"
  SECONDARY_NAT="${VPC_NETWORK}-${SECONDARY_REGION}-egress-nat"
  ensure_subnet
  ensure_router_and_nat
  ensure_workstation_cluster
done

echo "Multi-region infrastructure check complete."

#!/usr/bin/env bash

# Copyright 2026 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# setup-abfs-vm.sh — Automated ABFS Client Setup with Kernel Pinning (6.14.0-1021-gcp)
#
# This script configures a standalone Google Compute Engine (GCE) Ubuntu 24.04 VM
# with the precompiled casfs-kmod-6.14.0-1021-gcp kernel module (version 0.1.14),
# ensuring exact symbol correlation with abfs-client (version 0.1.14).
#
set -euo pipefail

PINNED_KERNEL_VER="6.14.0-1021-gcp"
PINNED_CASFS_PKG="casfs-kmod-${PINNED_KERNEL_VER}"

echo "=== 1. Configuring Google Cloud APT & Artifact Registry Repositories ==="
sudo apt-get update -qq
sudo apt-get install -y -qq curl gnupg libcap2-bin kmod

# Acquire public keys
curl -fsSL https://packages.cloud.google.com/apt/doc/apt-key.gpg | sudo gpg --dearmor --yes -o /usr/share/keyrings/cloud.google.gpg
echo 'deb [signed-by=/usr/share/keyrings/cloud.google.gpg] https://packages.cloud.google.com/apt apt-transport-artifact-registry-stable main' | sudo tee /etc/apt/sources.list.d/artifact-registry.list > /dev/null
sudo apt-get update -qq
sudo apt-get install -y -qq apt-transport-artifact-registry

# Add abfs-binaries artifact registry
echo 'deb [signed-by=/usr/share/keyrings/cloud.google.gpg] ar+https://us-apt.pkg.dev/projects/abfs-binaries abfs-apt-alpha-public main' | sudo tee -a /etc/apt/sources.list.d/artifact-registry.list > /dev/null
sudo apt-get update -qq --allow-insecure-repositories || true

echo "=== 2. Installing Pinned Linux Kernel (${PINNED_KERNEL_VER}) & ABFS Client ==="
sudo apt-get install -y --allow-unauthenticated \
    linux-image-${PINNED_KERNEL_VER} \
    linux-headers-${PINNED_KERNEL_VER} \
    linux-modules-${PINNED_KERNEL_VER} \
    abfs-client \
    repo \
    git \
    python3 \
    unzip \
    zip \
    build-essential \
    rsync \
    bc \
    libelf-dev \
    bison \
    flex \
    libcap2-bin \
    ${PINNED_CASFS_PKG}

echo "=== 3. Relaxing AppArmor for Unprivileged User Namespaces ==="
sudo sysctl -w kernel.apparmor_restrict_unprivileged_unconfined=0 || true
sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0 || true
echo "kernel.apparmor_restrict_unprivileged_unconfined=0" | sudo tee /etc/sysctl.d/99-abfs-apparmor.conf > /dev/null
echo "kernel.apparmor_restrict_unprivileged_userns=0" | sudo tee -a /etc/sysctl.d/99-abfs-apparmor.conf > /dev/null

CURRENT_KERNEL=$(uname -r)
if [ "${CURRENT_KERNEL}" != "${PINNED_KERNEL_VER}" ]; then
    echo "========================================================================"
    echo "WARNING: Current booted kernel (${CURRENT_KERNEL}) differs from pinned (${PINNED_KERNEL_VER})."
    echo "Configuring GRUB default boot entry..."
    echo "========================================================================"
    # Update GRUB to default to the newly installed pinned kernel
    sudo sed -i "s/^GRUB_DEFAULT=.*/GRUB_DEFAULT=\"Advanced options for Ubuntu>Ubuntu, with Linux ${PINNED_KERNEL_VER}\"/" /etc/default/grub || true
    sudo update-grub || true
    sudo grub-reboot "Advanced options for Ubuntu>Ubuntu, with Linux ${PINNED_KERNEL_VER}" || true
    echo ""
    echo "========================================================================"
    echo "IMPORTANT: Please reboot this VM now by running:"
    echo "    sudo reboot"
    echo "After rebooting, SSH back in and re-run this script to insert casfs.ko."
    echo "========================================================================"
    exit 0
fi

echo "=== 4. Inserting CASFS Kernel Module (version 0.1.14) ==="
sudo depmod -a
sudo modprobe casfs

echo "=== 5. Verification ==="
echo "Loaded kernel module: $(lsmod | grep casfs || echo 'ERROR: casfs not loaded')"
sudo modinfo casfs | grep -E 'version|filename' || true
echo ""
echo "SUCCESS! ABFS client and casfs.ko (${PINNED_KERNEL_VER}) are installed and ready."
echo "You can now configure abfs and run 'abfs mount'."

#!/bin/bash
#
# Copyright 2024 Google Inc. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

echo "Persisting Android SDKs"

# Provide Android SDKs via overlay mount containing both User Downloaded SDKs and SDKs bundled with the image.
image_sdk=/opt/google/Android/Sdk
user_sdk=/home/.workstation/UserSdk
overlay_work_dir=/home/.workstation/OverlayWorkDir
sdk_dir=/home/user/Android/Sdk

# If we are not using a persistent home directory, just symlink the SDK
# directory as any downloaded packages will not be persisted anyway.
if [[ ! $(grep "/dev/disk/by-id/google-" /proc/mounts | grep "/home") ]]; then
  echo "No persistent disk mounted, user downloaded SDKs will not be persisted."
  mkdir -p "$(dirname ${sdk_dir})"
  chown -R user:user $(dirname "${sdk_dir}")
  ln -s "${image_sdk}" "${sdk_dir}"
  exit 0
fi

# User has opted out of using the overlay mount.
if [[ ! -d "${user_sdk}" && -d "${sdk_dir}" ]]; then
  echo "Android SDK cannot be mounted."
  echo "To recieve the latest SDK updates, please delete ${sdk_dir} and restart your workstation."
else
    # create requisite directory structure to persist user downloaded SDKs via overlay mount
  if [[ ! -d "${user_sdk}" ||  ! -d "${overlay_work_dir}" || ! -d "${sdk_dir}" ]]; then
    mkdir -p "${user_sdk}" "${overlay_work_dir}" "${sdk_dir}"
    chown -R user:user $(dirname "${user_sdk}")
    chown -R user:user $(dirname "${sdk_dir}")
  fi

  echo "User downloaded SDKs will be persisted under ${user_sdk}"
  mount overlay -t overlay -o lowerdir="${image_sdk}",upperdir="${user_sdk}",workdir="${overlay_work_dir}" "${sdk_dir}"
fi

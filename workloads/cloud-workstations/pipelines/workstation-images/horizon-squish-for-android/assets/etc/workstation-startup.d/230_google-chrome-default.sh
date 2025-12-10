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

if [[ "${EUID:-$(id -u)}" -eq 0 ]]; then
    exec runuser user "${BASH_SOURCE[0]}"
fi

if [[ ! -f /home/user/.config/mimeapps.list ]]; then
  echo "Setting google-chrome as default browser for first run."
  xdg-mime default google-chrome.desktop text/html
  xdg-mime default google-chrome.desktop x-scheme-handler/http
  xdg-mime default google-chrome.desktop x-scheme-handler/https
  xdg-mime default google-chrome.desktop x-scheme-handler/about
  xdg-settings set default-web-browser google-chrome.desktop
fi

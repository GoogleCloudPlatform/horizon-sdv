#!/bin/sh

# Copyright (c) 2026 Accenture, All Rights Reserved.
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

# Build the git tree the pusher-config Job stores in the ABFS ref "config".
# Prints the tree hash. The Job and the uploader startup check both call this
# so the two hashes are comparable.

set -eu

work="$1"
: "${PUSHER_CONFIG:?}"
: "${SEED_REPOS:?}"
: "${CONCURRENT_UPLOADS:?}"
: "${GIT_EMAIL:?}"

rm -rf "$work"
mkdir -p "$work"
cd "$work"
git init -q
mkdir -p git-pusher clients
cp "$PUSHER_CONFIG" git-pusher/default
printf '\n' >> git-pusher/default
cat "$SEED_REPOS" >> git-pusher/default
printf 'concurrent-uploads: %s\n' "$CONCURRENT_UPLOADS" > clients/defaults
git add git-pusher/default clients/defaults
git -c user.email="$GIT_EMAIL" -c user.name="ABFS Bootstrap Job" commit -q -m "Creating the default pusher config."
git rev-parse 'HEAD^{tree}'

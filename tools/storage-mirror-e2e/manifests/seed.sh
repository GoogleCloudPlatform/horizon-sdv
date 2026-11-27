#!/usr/bin/env bash

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

# Seeds the e2e Gitea: a repo manifest with three projects (plus refs/changes/* to exercise prune-refs) and one
# single repository with an LFS object, and a private repository (credentials scenario). MODE=update adds one commit to every project (for the sync scenarios).
# Runs in the storage-mirror-sync image (git, git-lfs).

set -euo pipefail

BASE="https://${GITEA_USER}:${GITEA_PASSWORD}@gitea.e2e.svc.cluster.local:3000/${GITEA_USER}"
PUBLIC="https://gitea.e2e.svc.cluster.local:3000/${GITEA_USER}"
export GIT_SSL_NO_VERIFY=true GIT_TERMINAL_PROMPT=0
export HOME="$(mktemp -d)"
git config --global user.email e2e@example.com
git config --global user.name e2e
git config --global init.defaultBranch main
git lfs install

work="$(mktemp -d)"
cd "${work}"

commit_project() {
  local name=$1
  if [[ "${MODE:-create}" == "create" ]]; then
    mkdir "${name}" && cd "${name}" && git init -q
  else
    git clone -q "${BASE}/${name}.git" "${name}" && cd "${name}"
  fi
  date +%s%N >"${name}.txt"
  git add . && git commit -qm "${name} $(date)"
  git push -q -f "${BASE}/${name}.git" HEAD:refs/heads/main
  if [[ "${MODE:-create}" == "create" ]]; then
    git push -q -f "${BASE}/${name}.git" HEAD:refs/changes/01/1/1 || echo "note: gitea refused refs/changes (prune-refs check skipped)"
    git push -q -f "${BASE}/${name}.git" HEAD:refs/heads/release
  fi
  cd "${work}"
}

for p in p1 p2 p3; do
  commit_project "${p}"
done

if [[ "${MODE:-create}" == "create" ]]; then
  mkdir manifest && cd manifest && git init -q
  cat >default.xml <<EOF
<?xml version="1.0" encoding="UTF-8"?>
<manifest>
  <remote name="e2e" fetch="${PUBLIC}/"/>
  <default remote="e2e" revision="main" sync-j="2"/>
  <project name="p1" path="p1"/>
  <project name="p2" path="p2"/>
  <project name="p3" path="sub/p3"/>
</manifest>
EOF
  git add . && git commit -qm manifest
  git push -q -f "${BASE}/manifest.git" HEAD:refs/heads/main
  cd "${work}"

  mkdir lfsrepo && cd lfsrepo && git init -q
  git lfs track "*.bin"
  head -c 2097152 /dev/urandom >blob.bin
  echo plain >README.md
  git add . && git commit -qm "lfs payload"
  # Gitea creates repositories on the first push; the LFS pre-push upload would run before that and get 404, so
  # push the refs without the hook first, then the objects.
  git config lfs.locksverify false
  git push -q -f --no-verify "${BASE}/lfsrepo.git" HEAD:refs/heads/main
  git lfs push --all "${BASE}/lfsrepo.git"
  cd "${work}"

  # Private repository for the git credentials scenario; anonymous reads must fail.
  api="https://gitea.e2e.svc.cluster.local:3000/api/v1"
  curl -fsSk -u "${GITEA_USER}:${GITEA_PASSWORD}" -H 'Content-Type: application/json' \
    -d '{"name":"private-repo","private":true}' "${api}/user/repos" >/dev/null
  mkdir private-repo && cd private-repo && git init -q
  echo secret >README.md
  git add . && git commit -qm private
  git push -q -f "${BASE}/private-repo.git" HEAD:refs/heads/main
  if git ls-remote "${PUBLIC}/private-repo.git" >/dev/null 2>&1; then
    echo "private-repo is readable without credentials" >&2
    exit 1
  fi
  cd "${work}"
fi

echo "seed ${MODE:-create} done"

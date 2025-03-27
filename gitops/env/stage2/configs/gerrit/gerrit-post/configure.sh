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

# Important delay ! Actions below cannot be done while Init stage is ongoing
sleep 30

APISERVER=https://kubernetes.default.svc
SERVICEACCOUNT=/var/run/secrets/kubernetes.io/serviceaccount
NAMESPACE=$(cat ${SERVICEACCOUNT}/namespace)
TOKEN=$(cat ${SERVICEACCOUNT}/token)
CACERT=${SERVICEACCOUNT}/ca.crt

STAGE1_COMPLETED=false
STAGE2_COMPLETED=false
STAGE3_COMPLETED=false
STAGE4_COMPLETED=false
STAGE5_COMPLETED=false

HTTP_PASSWORD=""
retVal=""

function initialize() {
  git config --global user.email "gerrit@gerrit"
  git config --global user.name "Gerrit Gerrit"
}

function gerrit-test-connection() {
  retVal="RETVAL_NOK"
  echo "Testing the SSH connection to Gerrit."
  n=1
  until [ "$n" -ge 30 ]; do
    ERR_MSG=$(ssh -o LogLevel=ERROR -o ConnectTimeout=1 -o BatchMode=yes -o UserKnownHostsFile=/dev/null -o StrictHostKeychecking=no -p 29418 -i /root/.ssh/privatekey gerrit-admin@gerrit-service gerrit version 2>&1)
    if [[ $ERR_MSG == *"gerrit version"* ]]; then
      echo "SSH connection worked, no need to craft All-Users repository. Just retrieve HTTP PASSWORD"
      HTTP_PASSWORD=$(kubectl get secrets -n gerrit gerrit-http-password -o json | jq -r ".data[]" | base64 -d)
      if [ -z "${HTTP_PASSWORD}" ]; then
        echo "ERROR: HTTP_PASSWORD is empty."
        retVal="RETVAL_OK"
        return
      else
        retVal="RETVAL_OK"
        return
      fi
    else
      echo "No SSH connection, retrying... #$n"
      n=$((n + 1))
      sleep 1
    fi
  done
  retVal="RETVAL_OK"
  return
}

function gerrit-craft-all-users() {
  retVal="RETVAL_NOK"
  rm -rf /mnt/git/tmp
  mkdir /mnt/git/tmp
  cd /mnt/git

  git config --global --add safe.directory /mnt/git/All-Users.git
  git clone All-Users.git /mnt/git/tmp/All-Users
  cd /mnt/git/tmp/All-Users

  RES=$(git ls-remote origin | grep "refs/users/00/1000000")
  if [[ "${RES}" != *"refs/users/00/1000000"* ]]; then
    echo "File does not exists. Adding gerrit-admin user to the repository."
    git checkout -b refs/users/00/1000000
    rm -f *
    cp /root/account.config .

    ssh-keygen -f /root/.ssh/privatekey -y >./authorized_keys
    git add .
    git commit -m "gerrit-admin"
    git push -f origin refs/users/00/1000000:refs/users/00/1000000

    STAGE1_COMPLETED=true
  else
    echo "File exists. Checking if authorized_keys are there. If not - adding."
    git fetch origin refs/users/00/1000000:refs/users/00/1000000
    git checkout refs/users/00/1000000

    F1=account.config
    if [[ -f "$F1" ]]; then
      if grep -q "fullName = Gerrit Gerrit" $F1; then
        echo "Contents of account.config is correct."
      else
        echo "ERROR: Contents of account.config is incorrect."
        retVal="RETVAL_NOK"
        return
      fi
    fi

    F2=authorized_keys
    if [[ -f "$F2" ]]; then
      if grep -q gerrit-admin $F2; then
        echo "gerrit-admin ssh public key exists and contents is correct."
      else
        echo "File exists but missing gerrit-admin public key."
        cat /root/$F2 >>$F2
        git add $F2
        git commit -m "gerrit-admin"
        git push origin refs/users/00/1000000:refs/users/00/1000000
      fi
    else
      cp /root/$F2 .
      git add $F2
      git commit -m "gerrit-admin"
      git push origin refs/users/00/1000000:refs/users/00/1000000
    fi

    STAGE1_COMPLETED=true
  fi

  RES=$(git ls-remote origin | grep "refs/meta/external-ids")
  if [[ "${RES}" != *"refs/meta/external-ids"* ]]; then
    echo "File does not exists. Adding external-ids to the repository."
    git checkout -b refs/meta/external-ids
    rm -f *
    F1=$(echo -n "username:gerrit-admin" | sha1sum | cut -f 1 -d ' ')
    F2=$(echo -n "keycloak-oauth:gerrit-admin" | sha1sum | cut -f 1 -d ' ')
    cp /root/externalId-username-gerrit-admin ./$F1
    cp /root/externalId-keycloak-oauth-gerrit-admin ./$F2
    git add .
    git commit -m "gerrit-admin"
    git push origin refs/meta/external-ids:refs/meta/external-ids

    STAGE2_COMPLETED=true
  else
    echo "ExternalIds are there. Checking if externalIds are correct. If not - error."
    git fetch origin refs/meta/external-ids:refs/meta/external-ids
    git checkout refs/meta/external-ids
    F1=$(echo -n "username:gerrit-admin" | sha1sum | cut -f 1 -d ' ')
    F2=$(echo -n "keycloak-oauth:gerrit-admin" | sha1sum | cut -f 1 -d ' ')
    if [[ -f "$F1" ]]; then
      if grep -q "username:gerrit-admin" $F1; then
        echo "$F1 is correct."
      else
        echo "ERROR $F1 is incorrect."
        retVal="RETVAL_NOK"
        return
      fi
    else
      echo "ERROR: $F1 doesn't exist."
      retVal="RETVAL_NOK"
      return
    fi
    if [[ -f "$F2" ]]; then
      if grep -q "keycloak-oauth:gerrit-admin" $F2; then
        echo "$F2 is correct."
      else
        echo "ERROR $F2 is incorrect."
        retVal="RETVAL_NOK"
        return
      fi
    else
      echo "ERROR: $F2 doesn't exist."
      retVal="RETVAL_NOK"
      return
    fi

    STAGE2_COMPLETED=true
  fi

  RES=$(git ls-remote origin | grep "refs/meta/group-names")
  if [[ "${RES}" != *"refs/meta/group-names"* ]]; then
    echo "ERROR: group-names must exist by default"
    retVal="RETVAL_NOK"
    return
  else
    echo "group-names are there. Changing membership."
    git fetch origin refs/meta/group-names:refs/meta/group-names
    git checkout refs/meta/group-names
    FILE=$(grep "name = Administrators" * | awk -F':' '{print $1}')
    UUID=$(cat ${FILE} | grep "uuid" | awk '{print $3}')
    UUID_SHORT=$(echo ${UUID} | cut -c1-2)
    git fetch origin refs/groups/${UUID_SHORT}/${UUID}:refs/groups/${UUID_SHORT}/${UUID}
    git checkout refs/groups/${UUID_SHORT}/${UUID}
    if [ -f ./members ]; then
      if ! grep -q "1000000" ./members; then
        echo "1000000" >>members
      fi
    else
      echo "1000000" >members
    fi
    git add members
    git commit -m "Updating members"
    git push origin HEAD:refs/groups/${UUID_SHORT}/${UUID}

    STAGE3_COMPLETED=true
  fi

  if [[ "$STAGE1_COMPLETED" != true || "$STAGE2_COMPLETED" != true || "$STAGE3_COMPLETED" != true ]]; then
    echo "ERROR: STAGE 1 or 2 or 3 failed"
    retVal="RETVAL_NOK"
    return
  fi

  rm -rf /mnt/git/tmp
  echo "Testing SSH connection..."

  n=1
  until [ "$n" -ge 400 ]; do
    ERR_MSG=$(ssh -o LogLevel=ERROR -o ConnectTimeout=1 -o BatchMode=yes -o UserKnownHostsFile=/dev/null -o StrictHostKeychecking=no -p 29418 -i /root/.ssh/privatekey gerrit-admin@gerrit-service gerrit version 2>&1)
    if [[ $ERR_MSG == *"gerrit version"* ]]; then
      echo "SSH connection worked !!!"
      STAGE4_COMPLETED=true
      break
    else
      echo "No SSH connection, retrying... #$n"
      n=$((n + 1))
      sleep 1
    fi
  done

  if [[ "$STAGE4_COMPLETED" != true ]]; then
    # Restart gerrit to refresh external-ids and make it possible to generate the HTTP token, getting SSH to work again requires around 300 seconds.
    echo "Restarting Gerrit..."
    kubectl delete pod gerrit-0 -n gerrit
    echo "Testing SSH connection again (after restart)..."

    n=1
    until [ "$n" -ge 400 ]; do
      ERR_MSG=$(ssh -o LogLevel=ERROR -o ConnectTimeout=1 -o BatchMode=yes -o UserKnownHostsFile=/dev/null -o StrictHostKeychecking=no -p 29418 -i /root/.ssh/privatekey gerrit-admin@gerrit-service gerrit version 2>&1)
      if [[ $ERR_MSG == *"gerrit version"* ]]; then
        echo "SSH connection worked !!!"
        STAGE5_COMPLETED=true
        break
      else
        echo "No SSH connection, retrying... #$n"
        n=$((n + 1))
        sleep 1
      fi
    done
  else
    echo "No need to restart Gerrit, SSH connection works."
    STAGE5_COMPLETED=true
  fi

  if [[ "$STAGE5_COMPLETED" != true ]]; then
    echo "ERROR: Can't connect to SSH server..."
    retVal="RETVAL_NOK"
    return
  else
    HTTP_PASSWORD=$(cat /root/.ssh/privatekey | head -2 | tail -1 | cut -c1-30)
    ERR_MSG=$(ssh -q -o LogLevel=ERROR -o BatchMode=yes -o UserKnownHostsFile=/dev/null -o StrictHostKeychecking=no -p 29418 -i /root/.ssh/privatekey gerrit-admin@gerrit-service gerrit set-account gerrit-admin --http-password ${HTTP_PASSWORD})
    if [ -z "${HTTP_PASSWORD}" ]; then
      echo "ERROR: HTTP_PASSWORD is empty."
      retVal="RETVAL_NOK"
      return
    else
      cd /root
      HTTP_PASSWORD_BASE64=$(echo -n $HTTP_PASSWORD | base64 -w0)
      sed -i "s/##HTTP_PASSWORD##/${HTTP_PASSWORD_BASE64}/g" ./secret.json

      curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -X DELETE ${APISERVER}/api/v1/namespaces/jenkins/secrets/jenkins-gerrit-http-password
      curl --cacert ${CACERT} --header "Authorization: Bearer ${TOKEN}" -H 'Accept: application/json' -H 'Content-Type: application/json' -X POST ${APISERVER}/api/v1/namespaces/jenkins/secrets -d @secret.json
      echo "HTTP PASSWORD saved into secrets"
      retVal="RETVAL_OK"
      return
    fi
  fi
}

function gerrit-setup-all-projects() {
  retVal="RETVAL_NOK"
  cd /root
  mkdir -p /root/tmp
  cd /root/tmp
  git config --global --add safe.directory /mnt/git/All-Projects.git
  git clone /mnt/git/All-Projects.git
  cd All-Projects
  if grep -q "Anonymous-Users" groups; then
    echo "Removing anonymous access"
    sed -i '/read = group Anonymous Users$/d' project.config
    sed -i '/\[access \"refs\/meta\/version\"\]$/d' project.config
    sed -i '/^global:Anonymous-Users/d' groups
  else
    echo "Anonymous access is already disabled"
  fi
  if ! grep -q "label-Verified" ./project.config; then
    echo "Adding Verified label"
    sed -i '/label-Code-Review = -1..+1 group Registered Users/a\        label-Verified = -1..+1 group Administrators\n\        label-Verified = -1..+1 group Project Owners\n\        label-Verified = -1..+1 group Registered Users' ./project.config
    sed -i '/copyCondition = changekind:NO_CHANGE OR changekind:TRIVIAL_REBASE OR is:MIN/a\[label "Verified"]\n\        function = NoBlock\n\        defaultValue = 0\n\        value = -1 Fails\n\        value = 0 No score\n\        value = +1 Verified\n\        copyCondition = changekind:NO_CODE_CHANGE' ./project.config
  else
    echo "Verified label is already added"
  fi
  git add .
  git commit -m "Disable anonymous access, add Verified label"
  git push origin HEAD:refs/meta/config

  cd /root
  rm -rf /root/tmp
  retVal="RETVAL_OK"
  return
}

function main() {
  initialize

  gerrit-test-connection
  if [[ "${retVal}" == "RETVAL_NOK" ]]; then
    echo "gerrit-test-connection failed"
    exit 1
  fi

  gerrit-craft-all-users
  if [[ "${retVal}" == "RETVAL_NOK" ]]; then
    echo "gerrit-craft-all-users failed"
    exit 1
  fi

  gerrit-setup-all-projects
  if [[ "${retVal}" == "RETVAL_NOK" ]]; then
    echo "gerrit-setup-all-projects failed"
    exit 1
  fi
}

main

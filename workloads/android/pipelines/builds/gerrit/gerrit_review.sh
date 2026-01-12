#!/usr/bin/env bash

# Copyright (c) 2025 Accenture, All Rights Reserved.
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

# Description:
# Gerrit verified label vote on build success/fail.

# Include common functions and variables.
# shellcheck disable=SC1091
source "$(dirname "${BASH_SOURCE[0]}")"/../aaos_builder/aaos_environment.sh "$0"

GERRIT_SERVER_URL=${GERRIT_SERVER_URL:-}
GERRIT_USERNAME=${GERRIT_USERNAME:-}
GERRIT_PASSWORD=${GERRIT_PASSWORD:-}

VOTE=${VOTE:-}
MESSAGE=${MESSAGE:-}

# Loop through all changes to send message / vote.
while IFS="" read -r CHANGE_ID; do
    echo "Processing change: $CHANGE_ID"

    # The curl command to perform the POST request
    if [ -n "$VOTE" ]; then
        curl -s -X POST \
            "$GERRIT_SERVER_URL/a/changes/$CHANGE_ID/revisions/current/review" \
            -u "$GERRIT_USERNAME:$GERRIT_PASSWORD" \
            -H "Content-Type: application/json" \
            -d "{
                  \"message\": \"$MESSAGE\",
                  \"labels\": {
                    \"Verified\": \"$VOTE\"
                  }
                }"
        echo ""
        echo "Comment $MESSAGE and VOTE=$VOTE sent for $CHANGE_ID."
    else
        curl -s -X POST \
            "$GERRIT_SERVER_URL/a/changes/$CHANGE_ID/revisions/current/review" \
            -u "$GERRIT_USERNAME:$GERRIT_PASSWORD" \
            -H "Content-Type: application/json" \
            -d "{
                  \"message\": \"$MESSAGE\"
                }"
        echo ""
        echo "Comment $MESSAGE sent for $CHANGE_ID."
    fi

    # Add a newline for cleaner output after each curl response
done < "${GERRIT_CHANGES_FILE}"

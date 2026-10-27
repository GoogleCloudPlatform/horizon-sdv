#!/usr/bin/env python3

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

# Verify an ABFS licence before import or change stores it.
#
# Accepts JSON or a JWT. Stops on the first failure, while validating:
# 1. audience against 'abfs-user'
# 2. allowed_service_accounts contains the Unique ID of abfs-license
#      dedicated SA
# 3. project_number matches current GCP project number
# 4. expiry_threshold is still in the future

import base64
import json
import os
import sys
from datetime import datetime, timezone


def fail(msg):
    print("ERROR: " + msg, file=sys.stderr)
    sys.exit(1)


def b64url(data):
    pad = "=" * (-len(data) % 4)
    return base64.urlsafe_b64decode(data + pad)


def load_claims(raw):
    raw = raw.strip()
    if raw.count(".") == 2 and not raw.startswith("{"):
        try:
            parsed = json.loads(b64url(raw.split(".")[1]))
        except (ValueError, json.JSONDecodeError):
            return None
        return parsed if isinstance(parsed, dict) else None

    for decoder in (json.loads, lambda s: json.loads(b64url(s))):
        try:
            parsed = decoder(raw)
        except (ValueError, json.JSONDecodeError):
            continue
        if isinstance(parsed, dict):
            return parsed
    return None


def as_ids(accounts):
    if accounts is None or accounts == "":
        return []
    if isinstance(accounts, list):
        ids = []
        for item in accounts:
            ids.extend(as_ids(item))
        return ids
    if isinstance(accounts, bool):
        return []
    if isinstance(accounts, (str, int)):
        text = str(accounts).strip()
        return [text] if text else []
    if isinstance(accounts, dict):
        for key in ("unique_id", "uniqueId", "id"):
            if accounts.get(key) not in (None, ""):
                return [str(accounts[key]).strip()]
        # Google licence: {"<uniqueId>": "<email>", ...}
        return [str(key).strip() for key in accounts if str(key).strip()]
    return []


def project_numbers(claims):
    found = []
    for key in ("project_number", "projectNumber"):
        value = claims.get(key)
        if value not in (None, "") and not isinstance(value, bool):
            text = str(value).strip()
            if text:
                found.append(text)
    allowed = claims.get(
        "allowed_project_numbers",
        claims.get("allowedProjectNumbers"),
    )
    if isinstance(allowed, dict):
        found.extend(str(key).strip() for key in allowed if str(key).strip())
    elif isinstance(allowed, list):
        for item in allowed:
            if item not in (None, "") and not isinstance(item, bool):
                text = str(item).strip()
                if text:
                    found.append(text)
    elif isinstance(allowed, (str, int)) and not isinstance(allowed, bool):
        text = str(allowed).strip()
        if text:
            found.append(text)
    return found


def parse_expiry(value):
    if isinstance(value, bool) or value is None:
        return None
    if isinstance(value, (int, float)) or (
        isinstance(value, str) and value.strip().lstrip("-").isdigit()
    ):
        try:
            n = float(value)
            if n > 1e12:
                n = n / 1000.0
            return datetime.fromtimestamp(n, tz=timezone.utc)
        except (TypeError, ValueError, OverflowError):
            return None
    if isinstance(value, str):
        text = value.strip()
        if text.endswith("Z"):
            text = text[:-1] + "+00:00"
        try:
            dt = datetime.fromisoformat(text)
        except (TypeError, ValueError, OverflowError):
            return None
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt
    return None


def main():
    claims = load_claims(os.environ.get("LICENSE_PAYLOAD", ""))
    if not isinstance(claims, dict):
        fail("license type unknown")

    aud = claims.get("audience", claims.get("aud"))
    if isinstance(aud, list):
        aud_ok = "abfs-user" in [str(item).strip() for item in aud]
    else:
        aud_ok = str(aud).strip() == "abfs-user"
    if not aud_ok:
        fail("license type unknown")

    expected_uid = os.environ.get("EXPECTED_UNIQUE_ID", "").strip()
    ids = as_ids(
        claims.get(
            "allowed_service_accounts",
            claims.get("allowedServiceAccounts"),
        )
    )
    if expected_uid not in ids:
        fail("license service account incorrect")

    expected_pn = os.environ.get("EXPECTED_PROJECT_NUMBER", "").strip()
    if expected_pn not in project_numbers(claims):
        fail("license not applicable to this project")

    expiry = parse_expiry(
        claims.get("expiry_threshold", claims.get("expiryThreshold"))
    )
    if expiry is None or expiry < datetime.now(timezone.utc):
        fail("license already expired")

    print("License verification passed.")


if __name__ == "__main__":
    main()

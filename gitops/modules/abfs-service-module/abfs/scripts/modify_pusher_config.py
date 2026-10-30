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

# TABLE_PATH is the uploader-config table. PUSHER_CONFIG is the service pusher
# ConfigMap. SEED_OUT is the repos file the pusher tree is built from.
# The active row supplies url, branch, file, and fetch-ref-spec.
# workers comes from the pusher list and must be the same on every pusher.
# When TABLE_PATH is missing, empty, unreadable or has no single valid active
# row, FALLBACK_TABLE_PATH (rendered from abfs-service values) is used instead
# and a WARNING is printed.

import os
import sys

import yaml


class TableError(Exception):
    """The uploader-config table cannot provide a single valid active row."""


def fail(reason):
    """Print the reason and stop the init container."""
    print("modify_pusher: %s" % reason, file=sys.stderr)
    sys.exit(1)


def load_yaml(path):
    """Read one YAML file. Fail when the file cannot be parsed."""
    try:
        with open(path) as src:
            return yaml.safe_load(src.read())
    except Exception as exc:
        fail("cannot read %s (%s)" % (path, exc))


def text(value):
    """Return a stripped string, or None when the value is blank."""
    if isinstance(value, str) and value.strip():
        return value.strip()
    return None


def load_table(path):
    """Read the table YAML. Raise TableError when it is missing or unparsable."""
    try:
        with open(path) as src:
            return yaml.safe_load(src.read())
    except Exception as exc:
        raise TableError("cannot read %s (%s)" % (path, exc))


def find_active_row(entries):
    """Return the single table row marked active."""
    if not isinstance(entries, list) or not entries:
        raise TableError("table is empty")
    active = [entry for entry in entries if isinstance(entry, dict) and entry.get("active") is True]
    if len(active) != 1:
        raise TableError("expected exactly one active row, found %d" % len(active))
    print("active_row: %s" % active[0].get("name"))
    return active[0]


def read_repo_fields(row):
    """Read url, branch, file, and fetch-ref-spec from the active row."""
    name = row.get("name")
    repo = row.get("repo") if isinstance(row.get("repo"), dict) else None
    url = text(repo.get("url")) if repo else None
    branch = text(repo.get("branch")) if repo else None
    file_name = text(repo.get("file")) if repo else None
    specs = repo.get("fetch-ref-spec") if repo else None
    if not url or not branch or not file_name:
        raise TableError("active row %s is missing url, branch, or file" % name)
    if not isinstance(specs, list) or not specs or any(text(item) is None for item in specs):
        raise TableError("active row %s fetch-ref-spec is missing" % name)
    cleaned = [text(item) for item in specs]
    print("repo_fields: branch=%s file=%s" % (branch, file_name))
    return url, branch, file_name, cleaned


def worker_count(item):
    """Positive workers value from one pusher, or None."""
    value = item.get("workers") if isinstance(item, dict) else None
    if isinstance(value, int) and not isinstance(value, bool) and value > 0:
        return value
    return None


def shared_workers(pusher_config):
    """The one workers value shared by every pusher."""
    pushers = pusher_config.get("pushers") if isinstance(pusher_config, dict) else None
    workers = [worker_count(item) for item in pushers] if isinstance(pushers, list) else []
    if not workers or any(value is None for value in workers) or len(set(workers)) != 1:
        fail("pushers need one shared positive integer workers value")
    print("shared_workers: %s" % workers[0])
    return workers[0]


def seed_document(url, branch, file_name, specs, workers):
    """Repos block stored in the pusher seed."""
    return {
        "repos": [{
            "manifest": {"url": url, "files": [{"branch": branch, "file": file_name}]},
            "project-defaults": {"workers": workers, "fetch-ref-spec": specs},
        }]
    }


def write_seed(path, document):
    """Write the repos seed YAML."""
    with open(path, "w") as dst:
        dst.write(yaml.safe_dump(document, sort_keys=False))
    print("write_seed: wrote %s" % path)


def print_current_config(row, url, branch, file_name, specs):
    """Write the active repo to the job log."""
    name = row.get("name")
    kind = "custom" if name == "AndroidCustom" else "predefined"
    shown = {
        "name": name,
        "repo": {
            "url": url,
            "branch": branch,
            "file": file_name,
            "fetch-ref-spec": specs,
        },
    }
    print("===== current config (%s) =====" % kind)
    print(yaml.safe_dump(shown, sort_keys=False).rstrip())


def active_repo(path):
    """Active row and its repo fields from one table file."""
    row = find_active_row(load_table(path))
    return row, read_repo_fields(row)


def main():
    try:
        row, (url, branch, file_name, specs) = active_repo(os.environ["TABLE_PATH"])
    except TableError as exc:
        print("WARNING: modify_pusher: uploader-config table not usable (%s); "
              "using the abfs-service fallback row" % exc, file=sys.stderr)
        try:
            row, (url, branch, file_name, specs) = active_repo(os.environ["FALLBACK_TABLE_PATH"])
        except TableError as fallback_exc:
            fail("fallback table not usable either (%s)" % fallback_exc)
    workers = shared_workers(load_yaml(os.environ["PUSHER_CONFIG"]))
    print_current_config(row, url, branch, file_name, specs)
    write_seed(os.environ["SEED_OUT"], seed_document(url, branch, file_name, specs, workers))
    print("modify_pusher: %s branch=%s workers=%s" % (row.get("name"), branch, workers))


if __name__ == "__main__":
    main()

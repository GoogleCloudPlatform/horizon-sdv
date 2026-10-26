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

# Set one uploader-config row active and write a kubectl patch for that table.
#
# The workflow saves ConfigMap key "table" to /tmp/table.yaml, then runs this
# script. SELECTION names the row that becomes active and synced.
# CUSTOM_CONFIG is applied only when that name is AndroidCustom; an empty paste uses
# DEFAULT_CUSTOM_CONFIG. Every other row is active false and synced false.
# The patch is written to /tmp/uploader-config-patch.json.

import json
import os

import yaml

REPO_KEYS = ("url", "branch", "file", "fetch-ref-spec")
TABLE_PATH = "/tmp/table.yaml"
PATCH_PATH = "/tmp/uploader-config-patch.json"


def row_name(selection):
    """Android15_r36 (android-15.0.0_r36) is row Android15_r36."""
    selection = selection.strip()
    marker = " ("
    if marker in selection:
        selection = selection.split(marker, 1)[0].strip()
    return selection


def read_inputs():
    """Row name from SELECTION. Custom text falls back to the form example."""
    selection = row_name(os.environ.get("SELECTION", ""))
    custom_text = os.environ.get("CUSTOM_CONFIG", "").strip() or os.environ.get("DEFAULT_CUSTOM_CONFIG", "")
    if not selection:
        raise SystemExit("ERROR: selection is required")
    print("read_inputs: selection=%r custom_text_chars=%d" % (selection, len(custom_text)))
    return selection, custom_text


def load_table():
    """Return the list of rows from the ConfigMap table."""
    entries = yaml.safe_load(open(TABLE_PATH))
    if not isinstance(entries, list) or not entries:
        raise SystemExit("ERROR: table is empty")
    print("load_table: %d rows %s" % (len(entries), [entry.get("name") for entry in entries]))
    return entries


def repo_of(node):
    """Accept a full row or a bare repo block, and check the four required fields."""
    repo = node.get("repo") if isinstance(node, dict) and "repo" in node else node
    if not isinstance(repo, dict):
        raise SystemExit("ERROR: AndroidCustom config must contain a repo block")
    missing = [key for key in REPO_KEYS if not repo.get(key)]
    if missing:
        raise SystemExit("ERROR: repo missing %s" % ", ".join(missing))
    spec = repo["fetch-ref-spec"]
    if not isinstance(spec, list) or not all(isinstance(item, str) and item.strip() for item in spec):
        raise SystemExit("ERROR: repo.fetch-ref-spec must be a non-empty list of strings")
    cleaned = {
        "url": str(repo["url"]).strip(),
        "branch": str(repo["branch"]).strip(),
        "file": str(repo["file"]).strip(),
        "fetch-ref-spec": [item.strip() for item in spec],
    }
    print("repo_of: branch=%s file=%s" % (cleaned["branch"], cleaned["file"]))
    return cleaned


def custom_repo(custom_text):
    """Parse the AndroidCustom config entry into a repo dict."""
    print("custom_repo: parsing %d characters" % len(custom_text or ""))
    try:
        pasted = yaml.safe_load(custom_text)
    except yaml.YAMLError as exc:
        mark = getattr(exc, "problem_mark", None)
        where = "line %d, column %d: " % (mark.line + 1, mark.column + 1) if mark else ""
        raise SystemExit("ERROR: AndroidCustom config is not valid YAML (%s%s)" % (where, getattr(exc, "problem", None) or exc))
    if isinstance(pasted, list):
        if len(pasted) != 1:
            raise SystemExit("ERROR: Add one AndroidCustom entry")
        pasted = pasted[0]
    return repo_of(pasted)


def apply_selection(entries, selection, custom_repo_value):
    """Set active and synced on the chosen row. Replace that row's repo only for AndroidCustom."""
    names = [entry.get("name") for entry in entries if isinstance(entry, dict)]
    if selection not in names:
        raise SystemExit("ERROR: unknown selection %r; expected one of: %s" % (selection, ", ".join(names)))
    chosen = None
    for entry in entries:
        selected = entry.get("name") == selection
        entry["active"] = selected
        entry["synced"] = selected
        if selected:
            chosen = entry
            if custom_repo_value is not None:
                entry["repo"] = custom_repo_value
    if chosen is None:
        print("apply_selection: no row matched %r" % selection)
    else:
        print(
            "apply_selection: active=%s synced=true replaced_repo=%s"
            % (chosen["name"], custom_repo_value is not None)
        )
    return chosen


def write_patch(entries):
    """Write a patch for kubectl merge to replace data.table."""
    json.dump(
        {"data": {"table": yaml.safe_dump(entries, sort_keys=False)}},
        open(PATCH_PATH, "w"),
    )
    print("write_patch: wrote %s" % PATCH_PATH)


def already_active(entries, name):
    """True when this row name is already the active row."""
    for entry in entries:
        if isinstance(entry, dict) and entry.get("name") == name and entry.get("active") is True:
            return True
    return False


def custom_repo_changed(entries, new_repo):
    """True when the pasted AndroidCustom configuration is not the one stored on that row."""
    for entry in entries:
        if isinstance(entry, dict) and entry.get("name") == "AndroidCustom":
            return repo_of(entry) != new_repo
    return True


def write_restart_flag(restart):
    """Tell the workflow whether the uploader StatefulSet should restart."""
    with open("/tmp/uploader-restart", "w") as flag:
        flag.write("true\n" if restart else "false\n")
    print("restart_uploaders: %s" % ("true" if restart else "false"))


def main():
    selection, custom_text = read_inputs()
    entries = load_table()
    same_row = already_active(entries, selection)
    repo_changed = False
    if selection == "AndroidCustom":
        custom_repo_value = custom_repo(custom_text)
        repo_changed = custom_repo_changed(entries, custom_repo_value)
    else:
        custom_repo_value = None
        print("main: selection is not AndroidCustom, leaving repo blocks unchanged")
    chosen = apply_selection(entries, selection, custom_repo_value)
    write_patch(entries)
    write_restart_flag((not same_row) or repo_changed)
    print("Active row: %s branch=%s" % (chosen["name"], chosen["repo"]["branch"]))


if __name__ == "__main__":
    main()

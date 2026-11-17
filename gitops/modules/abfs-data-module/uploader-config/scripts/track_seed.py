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

# Wait until the uploaders have seeded the active uploader-config row, then set
# synced true on that row.
#
# The git-pusher has no "seed finished" line and never exits: it pushes every
# project again about a minute after the previous push. What it does log is
#   Starting pusher loop for manifest <m>   once, when the process starts
#   Starting pusher loop for project <p>    once per project, right after it
#   Pushing project https://<p>             at the start of every push
#   Project push <p> succeeded | failed     at the end of every push
# A project is seeded after its first "succeeded". The row is seeded when every
# uploader is Ready, every project of the row has succeeded at least once, and
# that holds with the same project count for SEED_STABLE_POLLS polls in a row.
#
# "Every project of the row" needs the full project list. The uploader log
# rotates within minutes, so the logs are read every SEED_POLL_SECONDS from the
# last line seen and the project names are kept in memory:
#   check "full"          the start lines of every uploader were seen, so the
#                         list is complete. Normal after a restart by the workflow.
#   check "steady-state"  the start lines were already rotated away. Only
#                         projects pushed while watching are known, and a project
#                         still in its first upload is invisible. The seed is
#                         confirmed only when the count equals fullTotal, the
#                         count of the last full check of this row. Without
#                         that count the result is "unconfirmed", never seeded.
#
# synced is set true only here, after such a confirmation. A flag that is true
# without a recorded "seeded" result for the row is cleared when a check starts.
# Progress is written to key seed-status of the uploader-config ConfigMap as
# JSON. States: seeding, seeded, and without synced: timeout (not finished
# after SEED_TIMEOUT_SECONDS), unconfirmed (see above), incomplete (the only
# projects left have failed SEED_FAIL_LIMIT pushes or more without a success;
# they are named). The module overview page shows it.
# The tracker also stops with an error when another row becomes active or a
# newer workflow takes over. It also runs, without an uploader restart, when
# the active row is selected again; a row confirmed before keeps synced during
# that check. Table writes carry the ConfigMap resourceVersion, so a selection
# made in between is not overwritten.

import json
import os
import re
import subprocess
import sys
import time
from datetime import datetime, timezone

import yaml

ABFS_NS = os.environ["ABFS_NAMESPACE"]
STATEFULSET = os.environ["UPLOADER_STATEFULSET"]
CONTAINER = os.environ.get("UPLOADER_CONTAINER", "abfs-uploader")
CONFIG_NS = os.environ["HORIZON_NAMESPACE"]
CONFIGMAP = os.environ["UPLOADER_CONFIGMAP"]
ROW = os.environ["SEED_ROW"].strip()
WORKFLOW = os.environ.get("WORKFLOW_NAME", "")
STARTED_AT = os.environ.get("SEED_STARTED_AT", "").strip()
TIMEOUT = int(os.environ.get("SEED_TIMEOUT_SECONDS", "10800"))
POLL = int(os.environ.get("SEED_POLL_SECONDS", "60"))
STABLE = int(os.environ.get("SEED_STABLE_POLLS", "3"))
FAIL_LIMIT = int(os.environ.get("SEED_FAIL_LIMIT", "10"))
STATUS_KEY = "seed-status"
PATCH_PATH = "/tmp/seed-status-patch.json"

PROCESS_START = " Starting pusher loop for manifest "
LOOP_RE = re.compile(r" Starting pusher loop for project (\S+)")
PUSHING_RE = re.compile(r" Pushing project (?:https?://)?(\S+)")
RESULT_RE = re.compile(r" Project push (\S+) (succeeded|failed)")


def now():
    """Current UTC time, to the second."""
    return datetime.now(timezone.utc).replace(microsecond=0)


def stamp(moment):
    """2026-01-31T12:00:00Z."""
    return moment.strftime("%Y-%m-%dT%H:%M:%SZ")


def kubectl(*args):
    """Run kubectl. Return (exit code, stdout); stderr goes to the workflow log on failure."""
    result = subprocess.run(("kubectl",) + args, capture_output=True, text=True, errors="replace")
    if result.returncode != 0:
        print("kubectl %s: %s" % (" ".join(args[:4]), result.stderr.strip()[:300]), flush=True)
    return result.returncode, result.stdout


def uploader_state():
    """(replicas, ready replicas) of the uploader StatefulSet, or (0, 0) when it cannot be read."""
    code, out = kubectl(
        "get", "statefulset", STATEFULSET, "-n", ABFS_NS,
        "-o", "jsonpath={.spec.replicas} {.status.readyReplicas}",
    )
    parts = out.split()
    if code != 0 or not parts:
        return 0, 0
    return int(parts[0]), int(parts[1]) if len(parts) > 1 else 0


def read_config():
    """Return (table rows, seed-status dict, resourceVersion), or (None, {}, None) when it cannot be read."""
    code, out = kubectl("get", "configmap", CONFIGMAP, "-n", CONFIG_NS, "-o", "json")
    if code != 0:
        return None, {}, None
    obj = json.loads(out)
    data = obj.get("data") or {}
    try:
        rows = yaml.safe_load(data.get("table", ""))
        status = json.loads(data.get(STATUS_KEY) or "{}")
    except (yaml.YAMLError, ValueError):
        return None, {}, None
    version = (obj.get("metadata") or {}).get("resourceVersion")
    return (rows if isinstance(rows, list) else None), (status if isinstance(status, dict) else {}), version


def active_row(rows):
    """Name of the active row, or None."""
    for row in rows or []:
        if isinstance(row, dict) and row.get("active") is True:
            return row.get("name")
    return None


def patch_config(data, version=None):
    """Merge-patch ConfigMap data. With a resourceVersion the patch only applies to that version."""
    patch = {"data": data}
    if version:
        patch["metadata"] = {"resourceVersion": version}
    with open(PATCH_PATH, "w") as out:
        json.dump(patch, out)
    code, _ = kubectl(
        "patch", "configmap", CONFIGMAP, "-n", CONFIG_NS, "--type", "merge", "--patch-file", PATCH_PATH,
    )
    return code == 0


def write_status(status):
    """Write seed-status only. The table is not touched."""
    return patch_config({STATUS_KEY: json.dumps(status, sort_keys=True)})


def write_synced(status, synced):
    """Set synced on the active row (false on every other row) together with seed-status.

    The table is read again and the patch carries its resourceVersion, so a
    selection made by someone else in between is never overwritten: the patch
    is refused, and the next try sees the new table. Return False when ROW is
    no longer the active row or the write keeps failing.
    """
    for _ in range(5):
        rows, _stored, version = read_config()
        if rows is None or version is None:
            time.sleep(2)
            continue
        if active_row(rows) != ROW:
            return False
        for row in rows:
            if isinstance(row, dict):
                row["synced"] = synced and row.get("name") == ROW
        data = {STATUS_KEY: json.dumps(status, sort_keys=True), "table": yaml.safe_dump(rows, sort_keys=False)}
        if patch_config(data, version):
            return True
        time.sleep(2)
    return False


class Uploader:
    """What one uploader pod has logged so far."""

    def __init__(self, pod):
        self.pod = pod
        self.started = set()
        self.done = set()
        self.fails = {}
        self.saw_start_lines = False
        self.in_header = False
        self.since = None

    def read(self):
        """Read the log lines since the last call. Return False when the log cannot be read."""
        args = ["logs", self.pod, "-n", ABFS_NS, "-c", CONTAINER, "--timestamps"]
        if self.since:
            args.append("--since-time=%s" % self.since)
        code, out = kubectl(*args)
        if code != 0:
            return False
        last = None
        for line in out.splitlines():
            head = line[:20]
            if len(head) == 20 and head[4] == "-" and head[10] == "T":
                last = head[:19] + "Z"
            if PROCESS_START in line:
                # A new git-pusher process (pod or container restart): what the
                # previous one pushed may belong to another row, so start again.
                if not self.in_header:
                    self.started.clear()
                    self.done.clear()
                    self.fails.clear()
                    self.saw_start_lines = False
                self.in_header = True
                continue
            match = RESULT_RE.search(line)
            if match:
                self.in_header = False
                project = match.group(1)
                self.started.add(project)
                if match.group(2) == "succeeded":
                    self.done.add(project)
                    self.fails.pop(project, None)
                else:
                    self.fails[project] = self.fails.get(project, 0) + 1
                continue
            match = PUSHING_RE.search(line)
            if match:
                self.in_header = False
                self.started.add(match.group(1))
                continue
            match = LOOP_RE.search(line)
            if match:
                self.in_header = False
                self.started.add(match.group(1))
                self.saw_start_lines = True
        if last:
            self.since = last
        return True


def stop(status, state, message):
    """End without a confirmed seed: write the state, make sure the row is not synced, exit 1."""
    status["state"] = state
    if not write_synced(status, False):
        write_status(status)
    print("ERROR: %s" % message, file=sys.stderr, flush=True)
    raise SystemExit(1)


def main():
    if not ROW:
        raise SystemExit("ERROR: SEED_ROW is empty")
    began = now()
    deadline = time.time() + TIMEOUT
    rows, stored, _version = read_config()
    previous = stored if stored.get("row") == ROW else {}
    # Project count of the last run for this row that saw the full list.
    full_total = previous.get("fullTotal") if isinstance(previous.get("fullTotal"), int) else None
    status = {
        "row": ROW,
        "state": "seeding",
        "workflow": WORKFLOW,
        "startedAt": STARTED_AT or None,
        "trackingSince": stamp(began),
        "projectsDone": 0,
        "projectsTotal": 0,
        "updatedAt": stamp(began),
    }
    if full_total:
        status["fullTotal"] = full_total
    print("track_seed: row=%s timeout=%ss poll=%ss stable_polls=%s known_total=%s"
          % (ROW, TIMEOUT, POLL, STABLE, full_total or "none"), flush=True)
    # synced may only be true when this tracker has confirmed the seed. A flag
    # that is true without a "seeded" result for this row (written by an older
    # chart at selection time) is cleared before the check starts.
    row_synced = any(isinstance(row, dict) and row.get("name") == ROW and row.get("synced") is True for row in rows or [])
    if row_synced and previous.get("state") != "seeded":
        print("row %s is marked synced but no finished seed is recorded for it; clearing the flag until it is confirmed" % ROW, flush=True)
        write_synced(status, False)
    else:
        write_status(status)
    uploaders = {}
    stable = 0
    stuck = 0
    blind_polls = 0
    last_total = -1
    while True:
        rows, stored, _version = read_config()
        if rows is not None:
            if active_row(rows) != ROW:
                raise SystemExit("ERROR: row %s is no longer active (now %s); seed tracking stopped" % (ROW, active_row(rows)))
            # Two runs can track the same row for a moment. The one started last keeps going.
            if (stored.get("row") == ROW and stored.get("workflow") not in (None, "", WORKFLOW)
                    and str(stored.get("trackingSince") or "") > status["trackingSince"]):
                raise SystemExit("ERROR: workflow %s now tracks row %s; this run stopped" % (stored.get("workflow"), ROW))

        replicas, ready = uploader_state()
        pods = ["%s-%d" % (STATEFULSET, index) for index in range(replicas)]
        readable = True
        for pod in pods:
            readable = uploaders.setdefault(pod, Uploader(pod)).read() and readable
        current = [uploaders[pod] for pod in pods]
        started = set().union(*[item.started for item in current]) if current else set()
        done = set().union(*[item.done for item in current]) if current else set()
        fails = {}
        for item in current:
            fails.update(item.fails)
        pending = sorted(started - done)
        full = bool(current) and all(item.saw_start_lines for item in current)
        healthy = replicas > 0 and ready == replicas and readable and all(item.started for item in current)
        # Without the start lines only projects pushed while watching are known.
        # The count of the last full run tells whether that is all of them.
        all_known = full or (full_total is not None and len(started) >= full_total)
        complete = healthy and not pending and all_known
        if complete and len(started) == last_total:
            stable += 1
        else:
            stable = 1 if complete else 0
        failing = [project for project in pending if fails.get(project, 0) >= FAIL_LIMIT]
        only_failing = healthy and pending and len(failing) == len(pending) and len(started) == last_total
        stuck = stuck + 1 if only_failing else 0
        blind = healthy and not pending and not full and full_total is None and len(started) == last_total
        blind_polls = blind_polls + 1 if blind else 0
        last_total = len(started)

        moment = now()
        status.update({
            "projectsDone": len(started & done),
            "projectsTotal": len(started),
            "pending": pending[:10],
            "uploadersReady": "%d/%d" % (ready, replicas),
            "check": "full" if full else "steady-state",
            "updatedAt": stamp(moment),
        })
        print(
            "%s seed %s: %d/%d projects, uploaders %s, %d not done%s"
            % (stamp(moment), ROW, status["projectsDone"], status["projectsTotal"], status["uploadersReady"],
               len(pending), (" (e.g. %s)" % ", ".join(pending[:3])) if pending else ""),
            flush=True,
        )

        if complete and stable >= STABLE:
            if not full and len(started) != full_total:
                stop(status, "unconfirmed",
                     "row %s: %d projects are pushed, but the last full check counted %d; the project list changed, so the seed "
                     "cannot be confirmed without an uploader restart (select another row and this row again)"
                     % (ROW, len(started), full_total))
            start = STARTED_AT or status["trackingSince"]
            seconds = int((moment - datetime.strptime(start, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)).total_seconds())
            status.update({"state": "seeded", "finishedAt": stamp(moment), "durationSeconds": seconds, "fullTotal": len(started)})
            if not write_synced(status, True):
                raise SystemExit("ERROR: cannot write synced=true for row %s (row no longer active, or the ConfigMap keeps changing)" % ROW)
            print(
                "SEED FINISHED row=%s projects=%d check=%s duration=%ds (%s since %s)"
                % (ROW, status["projectsTotal"], status["check"], seconds,
                   "uploader restart" if STARTED_AT else "tracking start", start),
                flush=True,
            )
            return

        if blind_polls >= STABLE:
            stop(status, "unconfirmed",
                 "row %s: all %d projects seen in the uploader logs are pushed, but the logs no longer show the full project "
                 "list and no earlier full check exists for this row, so projects still in their first upload cannot be ruled "
                 "out. To confirm, let the workflow restart the uploaders: select another row and then this row again"
                 % (ROW, len(started)))

        if stuck >= STABLE:
            status["failing"] = failing[:10]
            stop(status, "incomplete",
                 "row %s: %d of %d projects are pushed; %d keep failing (%d or more failed pushes each, none succeeded): %s. "
                 "The row is not marked synced. See the uploader logs for the reason"
                 % (ROW, status["projectsDone"], status["projectsTotal"], len(failing), FAIL_LIMIT, ", ".join(failing[:10])))

        if time.time() >= deadline:
            stop(status, "timeout",
                 "seed of row %s not finished after %ss: %d/%d projects, uploaders %s; not done: %s"
                 % (ROW, TIMEOUT, status["projectsDone"], status["projectsTotal"], status["uploadersReady"],
                    ", ".join(pending[:10]) or "-"))
        write_status(status)
        time.sleep(POLL)


if __name__ == "__main__":
    main()

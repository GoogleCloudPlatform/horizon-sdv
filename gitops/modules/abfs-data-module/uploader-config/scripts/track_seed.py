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
# uploader is Ready, every project an uploader has started has succeeded at
# least once, and that holds with the same project count for SEED_STABLE_POLLS
# polls in a row.
#
# The uploader log rotates within minutes, so the logs are read every
# SEED_POLL_SECONDS from the last line seen and the project names are kept in
# memory. When the start lines of an uploader were already rotated away (the
# tracker was started long after the uploaders), only projects pushed since
# then are known; the result is then marked check "steady-state", not "full".
#
# Progress is written to key seed-status of the uploader-config ConfigMap as
# JSON (state seeding, seeded or timeout). The module overview page shows it.
# The tracker stops with an error when another row becomes active, when a newer
# workflow takes over, or after SEED_TIMEOUT_SECONDS.

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
    """Return (table rows, seed-status dict) from the ConfigMap, or (None, {}) when it cannot be read."""
    code, out = kubectl("get", "configmap", CONFIGMAP, "-n", CONFIG_NS, "-o", "json")
    if code != 0:
        return None, {}
    data = json.loads(out).get("data") or {}
    try:
        rows = yaml.safe_load(data.get("table", ""))
        status = json.loads(data.get(STATUS_KEY) or "{}")
    except (yaml.YAMLError, ValueError):
        return None, {}
    return (rows if isinstance(rows, list) else None), (status if isinstance(status, dict) else {})


def active_row(rows):
    """Name of the active row, or None."""
    for row in rows or []:
        if isinstance(row, dict) and row.get("active") is True:
            return row.get("name")
    return None


def write_config(status, rows=None):
    """Merge-patch seed-status, and the table when rows is given. Return True on success."""
    data = {STATUS_KEY: json.dumps(status, sort_keys=True)}
    if rows is not None:
        data["table"] = yaml.safe_dump(rows, sort_keys=False)
    with open(PATCH_PATH, "w") as patch:
        json.dump({"data": data}, patch)
    code, _ = kubectl(
        "patch", "configmap", CONFIGMAP, "-n", CONFIG_NS, "--type", "merge", "--patch-file", PATCH_PATH,
    )
    return code == 0


class Uploader:
    """What one uploader pod has logged so far."""

    def __init__(self, pod):
        self.pod = pod
        self.started = set()
        self.done = set()
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
                    self.saw_start_lines = False
                self.in_header = True
                continue
            match = RESULT_RE.search(line)
            if match:
                self.in_header = False
                self.started.add(match.group(1))
                if match.group(2) == "succeeded":
                    self.done.add(match.group(1))
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


def main():
    if not ROW:
        raise SystemExit("ERROR: SEED_ROW is empty")
    began = now()
    deadline = time.time() + TIMEOUT
    status = {
        "row": ROW,
        "state": "seeding",
        "workflow": WORKFLOW,
        "startedAt": STARTED_AT or None,
        "trackingSince": stamp(began),
        "projectsDone": 0,
        "projectsTotal": 0,
    }
    print("track_seed: row=%s timeout=%ss poll=%ss stable_polls=%s" % (ROW, TIMEOUT, POLL, STABLE), flush=True)
    write_config(dict(status, updatedAt=stamp(began)))
    uploaders = {}
    stable = 0
    last_total = -1
    while True:
        rows, stored = read_config()
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
        pending = sorted(started - done)
        complete = (
            replicas > 0 and ready == replicas and readable
            and all(item.started for item in current) and not pending
        )
        if complete and len(started) == last_total:
            stable += 1
        else:
            stable = 1 if complete else 0
        last_total = len(started)

        moment = now()
        status.update({
            "projectsDone": len(started & done),
            "projectsTotal": len(started),
            "pending": pending[:10],
            "uploadersReady": "%d/%d" % (ready, replicas),
            "check": "full" if current and all(item.saw_start_lines for item in current) else "steady-state",
            "updatedAt": stamp(moment),
        })
        print(
            "%s seed %s: %d/%d projects, uploaders %s, %d not done%s"
            % (stamp(moment), ROW, status["projectsDone"], status["projectsTotal"], status["uploadersReady"],
               len(pending), (" (e.g. %s)" % ", ".join(pending[:3])) if pending else ""),
            flush=True,
        )

        if complete and stable >= STABLE:
            start = STARTED_AT or status["trackingSince"]
            seconds = int((moment - datetime.strptime(start, "%Y-%m-%dT%H:%M:%SZ").replace(tzinfo=timezone.utc)).total_seconds())
            status.update({"state": "seeded", "finishedAt": stamp(moment), "durationSeconds": seconds})
            rows, _ = read_config()
            if rows is None or active_row(rows) != ROW:
                raise SystemExit("ERROR: cannot confirm that row %s is still active; synced not changed" % ROW)
            for row in rows:
                if isinstance(row, dict):
                    row["synced"] = row.get("name") == ROW
            if not write_config(status, rows):
                raise SystemExit("ERROR: cannot write synced=true to %s/%s" % (CONFIG_NS, CONFIGMAP))
            print(
                "SEED FINISHED row=%s projects=%d check=%s duration=%ds (%s since %s)"
                % (ROW, status["projectsTotal"], status["check"], seconds,
                   "uploader restart" if STARTED_AT else "tracking start", start),
                flush=True,
            )
            return

        if time.time() >= deadline:
            status["state"] = "timeout"
            write_config(status)
            print("ERROR: seed of row %s not finished after %ss: %d/%d projects, uploaders %s; not done: %s"
                  % (ROW, TIMEOUT, status["projectsDone"], status["projectsTotal"], status["uploadersReady"],
                     ", ".join(pending[:10]) or "-"), file=sys.stderr, flush=True)
            raise SystemExit(1)
        write_config(status)
        time.sleep(POLL)


if __name__ == "__main__":
    main()

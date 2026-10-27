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

# Copy Job abfs-pusher-config into a Job the uploader can run again.
#
# Uploader pod 0 exports the Helm Job, runs this script, and creates the result.
# The new Job keeps that pod spec, including the compare-and-exit seed, and uses
# its own name so Argo CD keeps owning abfs-pusher-config.

import json
import os

SOURCE_PATH = os.environ["SOURCE_PATH"]
OUTPUT_PATH = os.environ["OUTPUT_PATH"]
SYNC_JOB_NAME = os.environ["SYNC_JOB_NAME"]
OLD_POD_LABELS = (
    "controller-uid",
    "batch.kubernetes.io/controller-uid",
    "job-name",
    "batch.kubernetes.io/job-name",
)


def load_job():
    """Read the Helm Job exported with kubectl."""
    job = json.load(open(SOURCE_PATH))
    print("load_job: %s/%s" % (job["metadata"].get("namespace"), job["metadata"]["name"]))
    return job


def clear_old_identity(job):
    """Drop status, selector, and Argo labels so this Job can be created again."""
    namespace = job["metadata"].get("namespace")
    job["metadata"] = {
        "name": SYNC_JOB_NAME,
        "labels": {"app.kubernetes.io/component": "pusher-config"},
    }
    if namespace:
        job["metadata"]["namespace"] = namespace
    job.pop("status", None)
    job["spec"].pop("selector", None)
    job["spec"].pop("manualSelector", None)
    # This Job is not the Helm Job. A short TTL is safe because Argo does not
    # watch it. The Helm Job must not set a TTL.
    job["spec"]["ttlSecondsAfterFinished"] = 600
    labels = job["spec"]["template"]["metadata"].setdefault("labels", {})
    for key in OLD_POD_LABELS:
        labels.pop(key, None)
    print("clear_old_identity: %s" % SYNC_JOB_NAME)
    return job


def write_job(job):
    """Write the manifest passed to kubectl create."""
    json.dump(job, open(OUTPUT_PATH, "w"))
    print("write_job: wrote %s" % OUTPUT_PATH)


def main():
    job = load_job()
    job = clear_old_identity(job)
    write_job(job)


if __name__ == "__main__":
    main()

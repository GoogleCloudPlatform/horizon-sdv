#!/usr/bin/env python3

# Copyright 2026 Google LLC
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

# Real-time Standalone ABFS Seeding Monitor & Completion Tracker

import sys
import re
import time
import subprocess
import json
from collections import defaultdict

# ANSI escape codes for stunning terminal aesthetics
CLEAR_SCREEN = "\033[2J\033[H"
RESET = "\033[0m"
BOLD = "\033[1m"
GREEN = "\033[32m"
CYAN = "\033[36m"
YELLOW = "\033[33m"
RED = "\033[31m"
BLUE = "\033[34m"

def run_cmd(cmd):
    try:
        res = subprocess.run(cmd, shell=True, capture_output=True, text=True, timeout=10)
        if res.returncode == 0:
            return res.stdout
        return ""
    except Exception:
        return ""

def sum_submap(submap_str):
    # Parses strings like "blob:12 map:0 object:4 ref:0 tree:10" and returns the sum of values
    total = 0
    matches = re.findall(r"(\w+):(\d+)", submap_str)
    for key, val in matches:
        if key in ["blob", "object", "tree", "ref", "map"]:
            total += int(val)
    return total

def monitor_loop():
    print(f"{CYAN}Initializing Standalone ABFS Seeding Monitor...{RESET}")
    
    # Verify kubectl context
    nodes = run_cmd("kubectl get nodes")
    if not nodes:
        print(f"{RED}ERROR: Cannot communicate with GKE cluster. Make sure your kubectl context is active!{RESET}")
        sys.exit(1)

    try:
        while True:
            # 1. Fetch uploader pods
            pods_raw = run_cmd("kubectl get pods -n abfs -l app.kubernetes.io/component=uploader -o jsonpath='{.items[*].metadata.name}'")
            uploader_pods = pods_raw.strip().split()
            
            if not uploader_pods:
                print(f"{YELLOW}Waiting for uploader pods to become ready...{RESET}")
                time.sleep(5)
                continue

            # 2. Monitor pod health to detect OOMs or CrashLoopBackOff
            pod_details_raw = run_cmd("kubectl get pods -n abfs -l app.kubernetes.io/component=uploader -o json")
            pod_list = {}
            if pod_details_raw:
                try:
                    pod_list = json.loads(pod_details_raw)
                    for pod in pod_list.get("items", []):
                        name = pod["metadata"]["name"]
                        statuses = pod.get("status", {}).get("containerStatuses", [])
                        for status in statuses:
                            state = status.get("state", {})
                            waiting = state.get("waiting", {})
                            terminated = state.get("terminated", {})
                            
                            if waiting and waiting.get("reason") == "CrashLoopBackOff":
                                print(f"{RED}ERROR: Pod {name} is in CrashLoopBackOff! It may have OOM'd.{RESET}")
                                sys.exit(1)
                            if terminated and terminated.get("reason") == "OOMKilled" or terminated.get("exitCode") == 137:
                                print(f"{RED}ERROR: Pod {name} was OOMKilled! Increase memory limits.{RESET}")
                                sys.exit(1)
                except Exception as e:
                    pass

            active_repos = {}
            total_queued_items = 0
            total_running_items = 0
            total_failed_items = 0

            # 3. Query logs and parse current states
            for pod in uploader_pods:
                logs = run_cmd(f"kubectl logs {pod} -n abfs --since=60s --tail=200")
                if not logs:
                    continue

                # Parse lines like: "android.googlesource.com/... still uploading. Current states: map[...]"
                lines = logs.split("\n")
                for line in lines:
                    if "still uploading. Current states:" in line:
                        # Extract repo name and the full map content
                        match = re.search(r"(\S+) still uploading\. Current states: map\[(.+)\]", line)
                        if match:
                            repo_name = match.group(1).replace("android.googlesource.com/", "")
                            map_content = match.group(2)
                            
                            # Extract queued, running, failed, blocked maps
                            queued_match = re.search(r"queued:map\[([^\]]+)\]", map_content)
                            running_match = re.search(r"running:map\[([^\]]+)\]", map_content)
                            failed_match = re.search(r"failed:map\[([^\]]+)\]", map_content)
                            blocked_match = re.search(r"blocked:map\[([^\]]+)\]", map_content)

                            queued = sum_submap(queued_match.group(1)) if queued_match else 0
                            running = sum_submap(running_match.group(1)) if running_match else 0
                            failed = sum_submap(failed_match.group(1)) if failed_match else 0
                            blocked = sum_submap(blocked_match.group(1)) if blocked_match else 0

                            # Track repos with any pending or failed activity
                            if queued > 0 or running > 0 or failed > 0 or blocked > 0:
                                active_repos[repo_name] = {
                                    "pod": pod,
                                    "queued": queued,
                                    "running": running,
                                    "failed": failed,
                                    "blocked": blocked
                                }
                                total_queued_items += queued
                                total_running_items += running
                                total_failed_items += failed
                    else:
                        # Fallback for newer ABFS log formats
                        blobs_match = re.search(r"(\S+) blobs: (\d+) found, (\d+) needed", line)
                        if blobs_match:
                            repo_name = blobs_match.group(1).replace("android.googlesource.com/", "")
                            running = int(blobs_match.group(3))
                            queued = int(blobs_match.group(2)) - running
                            
                            if queued > 0 or running > 0:
                                active_repos[repo_name] = {
                                    "pod": pod,
                                    "queued": queued,
                                    "running": running,
                                    "failed": 0,
                                    "blocked": 0
                                }
                                total_queued_items += queued
                                total_running_items += running

                    # Check for fatal errors that crash the uploader or indicate resource exhaustion
                    lower_line = line.lower()
                    if "ignoring invalid" in line or "invalid repo" in line or "FATAL" in line or "panic" in line or "no space left on device" in lower_line or "connection refused" in lower_line:
                        print(f"\n{RED}ERROR in {pod} logs:{RESET}")
                        print(f"{RED}{line}{RESET}")
                        sys.exit(1)

            # Check if pods are ready
            ready_pods = 0
            if pod_list:
                for pod in pod_list.get("items", []):
                    conditions = pod.get("status", {}).get("conditions", [])
                    for cond in conditions:
                        if cond.get("type") == "Ready" and cond.get("status") == "True":
                            ready_pods += 1

            # 4. Render Dashboard
            import os
            os.system('clear')
            print(f"{BOLD}{BLUE}======================================================================={RESET}")
            print(f"{BOLD}{CYAN}             STANDALONE ABFS SEEDING MONITOR & PROGRESS                {RESET}")
            print(f"{BOLD}{BLUE}======================================================================={RESET}")
            print(f"Current Local Time: {YELLOW}{time.strftime('%Y-%m-%d %H:%M:%S')}{RESET}")
            print(f"Active Uploaders   : {GREEN}{len(uploader_pods)} Replicas Online{RESET}")
            print(f"Active Repos Syncing: {YELLOW}{len(active_repos)}{RESET}")
            print(f"Pending Items      : Queued = {CYAN}{total_queued_items}{RESET} | Transferring = {GREEN}{total_running_items}{RESET} | Failed = {RED}{total_failed_items}{RESET}")
            print(f"{BLUE}-----------------------------------------------------------------------{RESET}")

            if active_repos:
                print(f"{BOLD}{'AOSP REPOSITORY PATH':<65} {'UPLOADER':<20} {'QUEUED':<8} {'SYNCING':<8} {'FAILED':<8}{RESET}")
                print(f"{BLUE}-----------------------------------------------------------------------{RESET}")
                # Print top 15 active repos to avoid terminal overflow
                for i, (repo, data) in enumerate(sorted(active_repos.items(), key=lambda x: x[1]['queued'] + x[1]['running'] + x[1]['failed'], reverse=True)):
                    if i < 15:
                        failed_str = f"{RED}{data['failed']}{RESET}" if data['failed'] > 0 else f"{data['failed']}"
                        print(f"{repo:<65} {data['pod']:<20} {CYAN}{data['queued']:<8}{RESET} {GREEN}{data['running']:<8}{RESET} {failed_str:<8}")
                    else:
                        remaining = len(active_repos) - 15
                        print(f"... and {remaining} more active repository streams in progress.")
                        break

                if total_failed_items > 0:
                    print(f"\n{RED}ERROR: Seeding encountered failures. Check the dashboard for failing items.{RESET}")
                    sys.exit(1)
            else:
                if ready_pods < len(uploader_pods):
                    # Pods are not fully ready yet, they haven't started.
                    print("\n")
                    print(f"{YELLOW}Uploaders are booting up and syncing initial cache. Please wait...{RESET}")
                    print(f"({ready_pods}/{len(uploader_pods)} Uploaders Ready)")
                    print("\n")
                else:
                    # 5. Seeding Completion Check
                    print("\n")
                    print(f"{BOLD}{GREEN}🎉🎉🎉 SEEDING HAS COMPLETED SUCCESSFULLY! 🎉🎉🎉{RESET}")
                    print(f"{GREEN}All AOSP repositories have finished initial replication to your standalone server.{RESET}")
                    print(f"{GREEN}All uploader queues are empty, and the client caches are ready for build pipelines.{RESET}\a") # Terminal bell notification
                    print("\n")
                    print(f"Monitoring will remain active. If AOSP releases new commits, they will appear here automatically.")

            print(f"{BLUE}======================================================================={RESET}")
            print(f"Refreshing in 5 seconds... Press {RED}Ctrl+C{RESET} to exit monitor.")
            time.sleep(5)

    except KeyboardInterrupt:
        print(f"\n{YELLOW}Monitoring stopped.{RESET}")

if __name__ == "__main__":
    monitor_loop()

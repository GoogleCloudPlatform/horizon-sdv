# Copyright (c) 2026 Google LLC, All Rights Reserved.
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

"""Firebase Device Streaming client for Android Virtual Devices (AAOS / OEM IVI).

Allocates and streams ADB connections to virtual automotive devices
(e.g., VOLVO_ARMAPP_IVI, AUTOMOTIVE_CF) via Google Cloud's Device Streaming API.
"""

import argparse
import os
import sys
import time
from queue import Queue
import google.auth
from google.auth.transport.requests import Request
import grpc
import requests

from google.cloud.devicestreaming.v1 import adb_service_pb2 as adb_pb
from google.cloud.devicestreaming.v1 import service_pb2 as service_pb
from google.cloud.devicestreaming.v1 import service_pb2_grpc as service_grpc

API_ENDPOINT = "devicestreaming.googleapis.com"
CLOUD_SCOPE = "https://www.googleapis.com/auth/cloud-platform"


def get_authenticated_credentials():
  """Retrieves Application Default Credentials with cloud-platform scope."""
  credentials, project = google.auth.default(scopes=[CLOUD_SCOPE])
  credentials.refresh(Request())
  return credentials, project


def create_device_session(
    project_id: str, model_id: str, version_id: str, token: str
) -> str:
  """Requests a new device session via the REST API."""
  url = f"https://{API_ENDPOINT}/v1/projects/{project_id}/deviceSessions"
  headers = {
      "Authorization": f"Bearer {token}",
      "Content-Type": "application/json",
  }
  payload = {
      "android_device": {
          "android_model_id": model_id,
          "android_version_id": version_id,
      }
  }
  response = requests.post(url, headers=headers, json=payload)
  response.raise_for_status()
  session = response.json()
  session_name = session["name"]
  print(f"[+] Successfully requested session: {session_name}")
  return session_name


def wait_for_active_session(
    session_name: str, token: str, timeout_seconds: int = 600
):
  """Polls the session until it reaches ACTIVE state."""
  url = f"https://{API_ENDPOINT}/v1/{session_name}"
  headers = {"Authorization": f"Bearer {token}"}
  start_time = time.time()
  print("[*] Waiting for virtual device to boot and become ACTIVE...")

  while time.time() - start_time < timeout_seconds:
    response = requests.get(url, headers=headers)
    response.raise_for_status()
    session_data = response.json()
    state = session_data.get("state")
    print(f"[*] Current session state: {state}")

    if state == "ACTIVE":
      print("[+] Device session is ACTIVE and ready for ADB streaming!")
      return session_data
    elif state in ("EXPIRED", "FINISHED", "UNAVAILABLE", "ERROR"):
      raise RuntimeError(f"Session terminated unexpectedly with state: {state}")

    time.sleep(10)

  raise TimeoutError(
      f"Timed out after {timeout_seconds}s waiting for session to become ACTIVE."
  )


def execute_adb_shell_command(
    session_name: str, project_id: str, command: str, credentials
) -> str:
  """Executes a one-off shell command over the bidirectional gRPC AdbConnect stream."""
  call_credentials = grpc.access_token_call_credentials(credentials.token)
  composite_credentials = grpc.composite_channel_credentials(
      grpc.ssl_channel_credentials(), call_credentials
  )
  channel = grpc.secure_channel(f"{API_ENDPOINT}:443", composite_credentials)
  stub = service_grpc.DirectAccessServiceStub(channel)

  metadata = [
      ("x-omnilab-session-name", session_name),
      ("x-goog-user-project", project_id),
  ]

  request_queue = Queue()
  request_queue.put(
      adb_pb.AdbMessage(
          open=adb_pb.Open(stream_id=1, service=f"shell:{command}")
      )
  )

  def generate_requests():
    while True:
      msg = request_queue.get()
      if msg is None:
        break
      yield msg

  responses = stub.AdbConnect(generate_requests(), metadata=metadata)
  output_data = b""

  for response in responses:
    case = response.WhichOneof("contents")
    if case == "stream_data":
      if response.stream_data.HasField("data"):
        output_data += response.stream_data.data
      if response.stream_data.HasField("close"):
        break

  request_queue.put(None)
  return output_data.decode("utf-8", errors="replace").strip()


def parse_args():
  parser = argparse.ArgumentParser(
      description="Connect and stream ADB from Google Cloud Device Streaming."
  )
  parser.add_argument(
      "--project",
      type=str,
      default=os.environ.get("CLOUD_PROJECT"),
      help="Google Cloud project ID (defaults to CLOUD_PROJECT env var)",
  )
  parser.add_argument(
      "--model",
      type=str,
      default="VOLVO_ARMAPP_IVI",
      help="Virtual device model ID (e.g. VOLVO_ARMAPP_IVI, AUTOMOTIVE_CF)",
  )
  parser.add_argument(
      "--api-version",
      type=str,
      default="34",
      help="Android API level (default: 34)",
  )
  parser.add_argument(
      "--cmd",
      type=str,
      default="getprop ro.product.model",
      help="Shell command to run on device",
  )
  return parser.parse_args()


def main():
  args = parse_args()
  if not args.project:
    print(
        "ERROR: --project must be specified or CLOUD_PROJECT env var set.",
        file=sys.stderr,
    )
    sys.exit(1)

  print(f"[1/4] Authenticating with Google Cloud...")
  credentials, detected_project = get_authenticated_credentials()
  project_id = args.project or detected_project

  print(
      f"[2/4] Allocating device session: model={args.model},"
      f" api={args.api_version}..."
  )
  session_name = create_device_session(
      project_id, args.model, args.api_version, credentials.token
  )

  print(f"[3/4] Waiting for device readiness...")
  wait_for_active_session(session_name, credentials.token)

  print(f"[4/4] Executing command: '{args.cmd}'...")
  output = execute_adb_shell_command(
      session_name, project_id, args.cmd, credentials
  )
  print(f"\n--- [Command Output] ---")
  print(output)
  print(f"------------------------\n")


if __name__ == "__main__":
  main()

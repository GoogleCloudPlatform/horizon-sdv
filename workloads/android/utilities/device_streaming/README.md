<!-- Copyright (c) 2026 Google LLC, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# Virtual Device Streaming Client

This utility provides a standalone, public Google Cloud Device Streaming client to allocate and interact with Android Automotive OS (AAOS) and OEM IVI virtual devices (such as Volvo IVI and reference Cuttlefish) via ADB over gRPC.

## Prerequisites

1. Google Cloud SDK installed and authenticated:
   ```bash
   gcloud auth application-default login
   ```
2. Python 3.10+
3. Install dependencies:
   ```bash
   pip install -r requirements.txt
   ```
4. Compile protobuf stubs:
   ```bash
   python -m grpc_tools.protoc \
     -I. \
     --python_out=. \
     --grpc_python_out=. \
     google/cloud/devicestreaming/v1/*.proto
   ```

## Usage

### Run a command on a virtual Volvo IVI device

```bash
python stream_device.py \
  --project=<YOUR_GCP_PROJECT_ID> \
  --model=VOLVO_ARMAPP_IVI \
  --api-version=34 \
  --cmd="getprop ro.product.model"
```

### Run on Reference AAOS Cuttlefish

```bash
python stream_device.py \
  --project=<YOUR_GCP_PROJECT_ID> \
  --model=AUTOMOTIVE_CF \
  --api-version=34 \
  --cmd="dumpsys car_service"
```

# Copyright (c) 2024-2026 Accenture, All Rights Reserved.
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

# requests>=2.33.0 requires Python >=3.10 (see PyPI Requires-Python).
# Python 3.13 to satisfy ISD-recommended fix for CVE-2025-13836 (>= 3.13.11).
FROM --platform=linux/amd64 python:3.13-slim-bookworm

# Add security repository and upgrade all packages to fix CVEs
RUN echo "deb http://security.debian.org/debian-security bookworm-security main" > /etc/apt/sources.list.d/security.list && \
    apt update -y && apt upgrade -y && \
    apt clean && rm -rf /var/lib/apt/lists/*

RUN pip install --upgrade pip

WORKDIR /app

# Install dependencies as root (system site-packages is not writable for non-root).
COPY requirements.txt /app/
RUN pip install --no-cache-dir -r requirements.txt

COPY configure-key.py /app/

RUN useradd -m appuser && chown -R appuser:appuser /app
USER appuser
ENTRYPOINT ["python", "/app/configure-key.py"]

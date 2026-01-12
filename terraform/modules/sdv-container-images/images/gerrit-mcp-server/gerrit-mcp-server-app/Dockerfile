# Copyright (c) 2024-2025 Accenture, All Rights Reserved.
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

# --- Stage 1: Build Stage (for installing dependencies) ---
FROM python:3.12-slim-bookworm AS build

# Install necessary tools
RUN apt-get update && apt-get install -y \
    curl \
    git \
    build-essential \
    && rm -rf /var/lib/apt/lists/*

# Set working directory
WORKDIR /app

# Copy the configuration script from host
COPY configure.sh .

# Run the configuration script to clone the repository, install dependencies and the server app
RUN chmod +x configure.sh &&\
    ./configure.sh

# --- Stage 2: Final Runtime Image ---
FROM python:3.12-slim-bookworm AS final

# Install required system tools (curl for REST API calls)
RUN apt-get update && apt-get install -y \
    curl \
    vim \
    && rm -rf /var/lib/apt/lists/*

# Create a non-root user for security
RUN useradd --create-home --shell /bin/bash appuser

# Copy installed packages from the build stage
COPY --from=build /usr/local/lib/python3.12/site-packages /usr/local/lib/python3.12/site-packages
COPY --from=build /usr/local/bin/ /usr/local/bin/

# Copy the startup script to working directory and set ownership
COPY --chown=appuser:appuser startup.sh /app/gerrit-mcp-server/startup.sh

# Create a log file (needed by app logic) in to installed packages dir and
# set permissions for the non-root user for the entire site-packages directory
RUN touch /usr/local/lib/python3.12/site-packages/server.log && \
    chown -R appuser:appuser /usr/local/lib/python3.12/site-packages

# Switch to the non-root user
USER appuser

# Set the working directory
WORKDIR /app/gerrit-mcp-server/

# Define the startup script as the container entrypoint
CMD ["/bin/bash", "startup.sh"]

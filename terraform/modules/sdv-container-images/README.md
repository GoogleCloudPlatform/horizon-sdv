<!-- Copyright (c) 2026 Accenture, All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

        http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License. -->

# sdv-container-images

Terraform builds selected container images and pushes them to Google Artifact Registry via the `kreuzwerker/docker` provider.

## Module inputs

See [`variables.tf`](variables.tf). Optional per image:

- `context_path` — absolute path to Docker build context
- `dockerfile_path` — absolute path to Dockerfile (when not `Dockerfile` inside the context)
- `platform` — e.g. `linux/amd64` for GKE-compatible builds from Apple Silicon

Default images use `images/<directory>/<image_name>/` as context and `Dockerfile` in that folder.
Images that use the shared Go library `images/shared/horizonauth` (`horizonauth_image_keys` in `main.tf`) are built with `images/` as context and the `Dockerfile` in their own folder, so their Dockerfiles copy paths relative to `images/`.

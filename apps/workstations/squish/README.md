<!--
Copyright 2026 Google LLC

Copyright (C) 2026 The Qt Company Ltd.
All rights reserved.

This file is provided AS IS with NO WARRANTY OF ANY KIND, INCLUDING THE
WARRANTY OF DESIGN, MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE.
-->

# Squish Custom Image for Cloud Workstations

The [CICD-Foundation](https://github.com/GoogleCloudPlatform/cicd-foundation)
[Blueprint for Cloud Workstations](https://github.com/GoogleCloudPlatform/cicd-foundation/tree/main/infra/blueprints/workstations)
automates the deployment of
[Cloud Workstations](https://docs.cloud.google.com/workstations/docs/overview)
using this custom image example for [Squish](https://www.qt.io/quality-assurance/squish).
It is designed for a self-service model where developers and testers can create their own
Cloud Workstation instances.

The **Squish Custom Image for Cloud Workstations** is a specialized image layer built on top of the [GNOME Workstation Blueprint](../gnome/README.md). It is designed to provide a highly productive, pre-configured desktop environment for Google Cloud Workstations, specifically tailored for advanced testing.

## 🚀 Key Features

- **Foundation-First**: Seamlessly integrates with the [cicd-foundation](https://github.com/GoogleCloudPlatform/cicd-foundation) workstations blueprint.
- **Headless Excellence**: Leverages the base blueprint's headless Wayland and RDP/Guacamole stack for low-latency browser-based access.
- **Testing-Ready**: Includes the professional automated GUI testing framework for testing GUI apps.

## 🏗️ Architecture

This image uses a **multi-layered build strategy**:

1.  **Base Layer**: [GNOME Workstation](../gnome/Dockerfile) - Handles the core OS (Ubuntu 24.04), systemd, GNOME Shell 46, and remote access protocols.
2.  **Squish Layer**: [Dockerfile](./Dockerfile) - Injects specialized post-build-time hooks (e.g., `01_install_squish.sh`), custom assets, and tools to layer on top of the foundation.

## 🛠️ Build Arguments

This image supports and propagates all base arguments, including:

| Argument                    | Default               | Description                                      |
| :-------------------------- | :-------------------- | :----------------------------------------------- |
| `SQUISH_GCP_LICENSE_SECRET` | `squish-license-key`  | Name of the GCP Secret Manager secret used to retrieve the Squish license during image build. |

## 📖 Documentation

- **[Design Document](./docs/design.md)**: Deep-dive into the thin-layer architecture, hook-based integration logic, and declarative packaging.
- **[System Overview](../../../docs/system_overview.md)**: High-level map of the entire Cloud Workstations Custom Image blueprint stack.
- **[Base Blueprint Docs](../gnome/docs/design.md)**: Deep-dives into the underlying systemd orchestrations and networking handover logic.
- **[Squish Doc](https://doc.qt.io/squish/)**: Official reference documentation for the Squish GUI test automation tool.

## 🔒 Secret manager
You can use GCP Secret Manager to supply the Squish license. By default, the secret is expected under the name `squish-license-key`, which is configured via the `SQUISH_GCP_LICENSE_SECRET` build argument. The secret value is fetched during the image build.

Make sure the following service accounts used during the build have been granted the Secret Manager Secret Accessor role (read permission) on this secret — otherwise the build will fail to retrieve the license:

- `cloudbuild@<PROJECT_ID>.iam.gserviceaccount.com`
- `workstations@<PROJECT_ID>.iam.gserviceaccount.com`

(where `<PROJECT_ID>` is your GCP project ID)

## Getting Started

1.  **Clone the CICD-Foundation**:
    ```bash
    git clone https://github.com/GoogleCloudPlatform/cicd-foundation.git
    cd cicd-foundation/infra/blueprints/workstations
    ```
2.  **Configure**: Create a `terraform.tfvars` file such as:

    ```yml
    project_id = "YOUR_GCP_PROJECT_ID"

    # Squish Custom Image Build
    cws_custom_images = {
      "squish" : {
        git_repo = {
          url    = "https://github.com/GoogleCloudPlatform/cicd-foundation.git"
          branch = "main"
        }
        build = {
          skaffold_path = "apps/workstations/squish/"
          machine_type  = "E2_HIGHCPU_32"
          # Pass environment variables to Skaffold to configure the Preflight UI source, Base Image or Squish license
          env = {
            CWS_BASE_IMAGE_TAG = "latest"
            GCP_REGION         = "us-central1"
            PREFLIGHT_WEB_REPO = "https://github.com/GoogleCloudPlatform/cicd-foundation.git"
            PREFLIGHT_WEB_DIR  = "apps/workstations/preflight-web"
            # Name of the GCP Secret Manager secret containing the Squish license. Fetched at build time.
            # SQUISH_GCP_LICENSE_SECRET = "squish-license-key"
          }
        }
      }
    }

    # Workstation Cluster
    cws_clusters = {
      "workstations" = {
        network    = "workstations"
        region     = "us-central1"
        subnetwork = "primary"
      }
    }

    # Workstation Configuration
    cws_configs = {
      "custom" = {
        cws_cluster = "workstations"

        # Self-service: Grant permissions to a group or specific users to create their own instances
        #creators = ["group:developers@example.com"]

        # Reference the custom image defined above
        custom_image_names = ["squish"]

        # Best user experience: Keep >0 instance(s) ready in the pool for instant startup
        pool_size = 1

        # Hardware Specs
        machine_type                 = "e2-standard-8"
        enable_nested_virtualization = false
        persistent_disk_size_gb      = 500
        persistent_disk_type         = "pd-balanced"
      }
    }
    ```

3.  **Deploy**:
    ```bash
    terraform init
    terraform plan
    terraform apply
    ```

For more information have a look at the **[Infrastructure & Deployment Guide](../docs/deployment_guide.md)**.
